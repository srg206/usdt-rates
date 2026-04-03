package grinex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"go.uber.org/zap"

	"usdt-rates/internal/models/domain"
)

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
	mu          sync.Mutex
	hc          *resty.Client
	url         string
	warmupURL   string
	jsonOrigin  string
	jsonReferer string
	log         *zap.Logger
}

func NewClient(timeout time.Duration, depthURL string, log *zap.Logger) (*Client, error) {
	if log == nil {
		log = zap.NewNop()
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar: %w", err)
	}

	warmupURL, jsonOrigin, jsonReferer, err := parseDepthWarmupTargets(depthURL)
	if err != nil {
		return nil, err
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
		hc:          hc,
		url:         depthURL,
		warmupURL:   warmupURL,
		jsonOrigin:  jsonOrigin,
		jsonReferer: jsonReferer,
		log:         log,
	}, nil
}

func (c *Client) Fetch(ctx context.Context) (domain.OrderBook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var empty domain.OrderBook

	// Warmup request for cookies / WAF
	if _, err := c.hc.R().
		SetContext(ctx).
		SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		Get(c.warmupURL); err != nil {
		return empty, fmt.Errorf("grinex warmup: %w", err)
	}

	resp, err := c.hc.R().
		SetContext(ctx).
		SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Origin", c.jsonOrigin).
		SetHeader("Referer", c.jsonReferer).
		Get(c.url)
	if err != nil {
		return empty, fmt.Errorf("grinex fetch: %w", err)
	}
	if resp.IsError() {
		return empty, fmt.Errorf("grinex http %d: %s", resp.StatusCode(), string(resp.Body()))
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

	c.log.Info("grinex depth parsed",
		zap.Int("bid_levels", len(book.Bids)),
		zap.Int("ask_levels", len(book.Asks)),
	)

	return book, nil
}

func parseDepthWarmupTargets(depthURL string) (warmupURL, jsonOrigin, jsonReferer string, err error) {
	u, err := url.Parse(depthURL)
	if err != nil {
		return "", "", "", fmt.Errorf("parse depth url: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", "", fmt.Errorf("invalid depth url")
	}

	apiHost := u.Hostname()
	apexHost := apiHost

	if h, ok := strings.CutPrefix(apiHost, "api."); ok && h != "" {
		apexHost = h
	}

	scheme := u.Scheme

	if apexHost != apiHost {
		warmupURL = scheme + "://" + apexHost + "/"
		jsonOrigin = scheme + "://" + apexHost
		jsonReferer = jsonOrigin + "/"
	} else {
		warmupURL = scheme + "://" + apiHost + "/"
		jsonOrigin = scheme + "://" + apiHost
		jsonReferer = jsonOrigin + "/"
	}

	return warmupURL, jsonOrigin, jsonReferer, nil
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
