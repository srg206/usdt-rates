package grinex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/sony/gobreaker"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"

	"usdt-rates/internal/models/domain"
	"usdt-rates/pkg/circuitbreaker"
)

var grinexTracer = otel.Tracer("usdt-rates/grinex")

const (
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
	retryCount       = 3
)

type depthEnvelope struct {
	Bids      json.RawMessage `json:"bids"`
	Asks      json.RawMessage `json:"asks"`
	Timestamp int64           `json:"timestamp"`
	Time      *int64          `json:"time"`
	Ts        *int64          `json:"ts"`
	TsMs      *int64          `json:"ts_ms"`
}

type Client struct {
	fetchSem    *semaphore.Weighted
	breaker     *gobreaker.CircuitBreaker
	hc          *resty.Client
	url         string
	warmupURL   string
	jsonOrigin  string
	jsonReferer string
	log         *zap.Logger
}

// NewClient builds a Grinex HTTP client. maxConcurrent limits how many Fetch calls may run
// concurrent HTTP traffic to the exchange at once (shared cookie jar and resty client).
// cb wraps exchange I/O in a circuit breaker when cb.Enabled is true.
// warmupURL, jsonOrigin, and jsonReferer must be set explicitly (e.g. via env); they are not derived from depthURL.
// HTTP(S) shape of these values is validated in config.Load.
func NewClient(timeout time.Duration, depthURL, warmupURL, jsonOrigin, jsonReferer string, maxConcurrent int, cb circuitbreaker.Settings, log *zap.Logger) (*Client, error) {
	if maxConcurrent < 1 {
		return nil, fmt.Errorf("maxConcurrent must be >= 1, got %d", maxConcurrent)
	}
	if log == nil {
		log = zap.NewNop()
	}

	breaker, err := circuitbreaker.New(log, cb)
	if err != nil {
		return nil, err
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}

	hc := resty.New().
		SetTimeout(timeout).
		SetCookieJar(jar).
		SetRetryCount(retryCount).
		SetRetryWaitTime(500*time.Millisecond).
		SetRetryMaxWaitTime(3*time.Second).
		SetHeader("User-Agent", defaultUserAgent).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			if r == nil {
				return false
			}
			code := r.StatusCode()
			return code == http.StatusTooManyRequests || code >= 500
		})

	return &Client{
		fetchSem:    semaphore.NewWeighted(int64(maxConcurrent)),
		breaker:     breaker,
		hc:          hc,
		url:         depthURL,
		warmupURL:   warmupURL,
		jsonOrigin:  jsonOrigin,
		jsonReferer: jsonReferer,
		log:         log,
	}, nil
}

// Fetch runs the depth flow with semaphore limiting and optional circuit breaker.
func (c *Client) Fetch(ctx context.Context) (domain.OrderBook, error) {
	if c.breaker == nil {
		return c.fetchWithConcurrencyLimit(ctx)
	}
	v, err := c.breaker.Execute(func() (interface{}, error) {
		return c.fetchWithConcurrencyLimit(ctx)
	})
	if err != nil {
		var empty domain.OrderBook
		if errors.Is(err, gobreaker.ErrOpenState) {
			return empty, fmt.Errorf("grinex circuit breaker open: %w", err)
		}
		return empty, err
	}
	return v.(domain.OrderBook), nil
}

func (c *Client) fetchWithConcurrencyLimit(ctx context.Context) (domain.OrderBook, error) {
	var empty domain.OrderBook
	if err := c.fetchSem.Acquire(ctx, 1); err != nil {
		return empty, err
	}
	defer c.fetchSem.Release(1)
	return c.fetchOrderBook(ctx)
}

func (c *Client) fetchOrderBook(ctx context.Context) (domain.OrderBook, error) {
	var empty domain.OrderBook

	// Warmup request for cookies / WAF
	{
		traceCtx, sp := grinexTracer.Start(ctx, "HTTP GET grinex warmup")
		sp.SetAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", c.warmupURL),
		)
		_, err := c.hc.R().
			SetContext(traceCtx).
			SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
			Get(c.warmupURL)
		if err != nil {
			sp.RecordError(err)
			sp.SetStatus(codes.Error, err.Error())
			sp.End()
			return empty, fmt.Errorf("grinex warmup: %w", err)
		}
		sp.End()
	}

	var resp *resty.Response
	{
		traceCtx, sp := grinexTracer.Start(ctx, "HTTP GET grinex depth")
		sp.SetAttributes(
			attribute.String("http.method", "GET"),
			attribute.String("http.url", c.url),
		)
		var err error
		resp, err = c.hc.R().
			SetContext(traceCtx).
			SetHeader("Accept", "application/json, text/plain, */*").
			SetHeader("Origin", c.jsonOrigin).
			SetHeader("Referer", c.jsonReferer).
			Get(c.url)
		if err != nil {
			sp.RecordError(err)
			sp.SetStatus(codes.Error, err.Error())
			sp.End()
			return empty, fmt.Errorf("grinex fetch: %w", err)
		}
		if resp.IsError() {
			httpErr := fmt.Errorf("grinex http %d: %s", resp.StatusCode(), string(resp.Body()))
			sp.RecordError(httpErr)
			sp.SetStatus(codes.Error, httpErr.Error())
			sp.SetAttributes(attribute.Int("http.status_code", resp.StatusCode()))
			sp.End()
			return empty, httpErr
		}
		sp.SetAttributes(attribute.Int("http.status_code", resp.StatusCode()))
		sp.End()
	}

	var env depthEnvelope
	if err := json.Unmarshal(resp.Body(), &env); err != nil {
		return empty, fmt.Errorf("decode depth: %w", err)
	}

	bids, err := parseSidePrices(env.Bids)
	if err != nil {
		return empty, fmt.Errorf("bids: %w", err)
	}

	asks, err := parseSidePrices(env.Asks)
	if err != nil {
		return empty, fmt.Errorf("asks: %w", err)
	}

	if len(bids) == 0 && len(asks) == 0 {
		return empty, fmt.Errorf("empty order book")
	}

	// Fallback for thin / broken book
	if len(asks) == 0 && len(bids) > 0 {
		asks = append([]float64(nil), bids...)
	}
	if len(bids) == 0 && len(asks) > 0 {
		bids = append([]float64(nil), asks...)
	}

	book := domain.OrderBook{
		Bids: bids,
		Asks: asks,
	}

	switch {
	case env.Timestamp > 0:
		book.ExchangeTime = time.Unix(env.Timestamp, 0).UTC()
		book.HasServerTime = true
	case env.Ts != nil && *env.Ts > 0:
		book.ExchangeTime = time.Unix(*env.Ts, 0).UTC()
		book.HasServerTime = true
	case env.Time != nil && *env.Time > 0:
		book.ExchangeTime = time.Unix(*env.Time, 0).UTC()
		book.HasServerTime = true
	case env.TsMs != nil && *env.TsMs > 0:
		book.ExchangeTime = time.UnixMilli(*env.TsMs).UTC()
		book.HasServerTime = true
	default:
		book.ExchangeTime = time.Now().UTC()
	}

	c.log.Debug("grinex depth parsed",
		zap.Int("bid_levels", len(book.Bids)),
		zap.Int("ask_levels", len(book.Asks)),
	)

	return book, nil
}

func parseSidePrices(data json.RawMessage) ([]float64, error) {
	if len(data) == 0 || string(data) == "null" {
		return nil, nil
	}

	// Legacy format: [[price, qty], ...]
	var matrix [][]any
	if err := json.Unmarshal(data, &matrix); err == nil && len(matrix) > 0 {
		return parseMatrix(matrix)
	}

	// Current format: [{"price":"80.0","volume":"..."}, ...]
	var objs []map[string]any
	if err := json.Unmarshal(data, &objs); err != nil {
		return nil, err
	}

	out := make([]float64, 0, len(objs))
	for _, item := range objs {
		raw, ok := item["price"]
		if !ok {
			return nil, fmt.Errorf("missing price")
		}

		price, err := parseScalar(raw)
		if err != nil {
			return nil, err
		}
		out = append(out, price)
	}

	return out, nil
}

func parseMatrix(levels [][]any) ([]float64, error) {
	out := make([]float64, 0, len(levels))

	for _, row := range levels {
		if len(row) == 0 {
			continue
		}

		price, err := parseScalar(row[0])
		if err != nil {
			return nil, err
		}
		out = append(out, price)
	}

	if len(out) == 0 {
		return nil, fmt.Errorf("empty side")
	}

	return out, nil
}

func parseScalar(v any) (float64, error) {
	switch t := v.(type) {
	case string:
		return strconv.ParseFloat(t, 64)
	case float64:
		return t, nil
	case json.Number:
		return t.Float64()
	default:
		return 0, fmt.Errorf("unsupported price type %T", v)
	}
}
