package grinex

import (
	"bytes"
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
)

const (
	// Browser profile aligned with a successful manual WAF bypass (Chrome on macOS).
	defaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
	retryCount       = 3
	retryMinWait     = 300 * time.Millisecond
	retryMaxWait     = 6 * time.Second
)

// OrderBook holds parsed bid/ask price levels (best level first per side).
type OrderBook struct {
	Bids          []float64
	Asks          []float64
	ExchangeTime  time.Time
	HasServerTime bool
}

// depthEnvelope matches Grinex spot depth JSON (object levels and/or legacy matrix).
type depthEnvelope struct {
	Bids      json.RawMessage `json:"bids"`
	Asks      json.RawMessage `json:"asks"`
	Timestamp int64           `json:"timestamp"`
	Time      *int64          `json:"time"`
	Ts        *int64          `json:"ts"`
	TsMs      *int64          `json:"ts_ms"`
}

// Client pulls the public depth endpoint using a browser-like session (one document warmup + API GET).
type Client struct {
	mu          sync.Mutex
	hc          *resty.Client
	url         string
	warmupURL   string // GET once before depth: apex / when depth is on api.*, else depth-host /
	jsonOrigin  string // Origin for the JSON GET (apex when calling api.*, same as depth host otherwise)
	jsonReferer string
	log         *zap.Logger
}

// NewClient configures Resty with cookie jar, retries, and WAF-oriented headers.
// log may be nil (no-op logger).
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
		SetHeader("User-Agent", defaultUserAgent).
		SetHeader("Sec-Ch-Ua", `"Chromium";v="146", "Not-A.Brand";v="24", "Google Chrome";v="146"`).
		SetHeader("Sec-Ch-Ua-Mobile", "?0").
		SetHeader("Sec-Ch-Ua-Platform", `"macOS"`).
		SetRetryCount(retryCount).
		SetRetryWaitTime(retryMinWait).
		SetRetryMaxWaitTime(retryMaxWait).
		SetRetryAfter(retryBackoff).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			if r == nil {
				return false
			}
			sc := r.StatusCode()
			if sc == http.StatusTooManyRequests {
				return true
			}
			if sc >= 500 {
				return true
			}
			return false
		})

	hc.AddRetryHook(func(r *resty.Response, err error) {
		if r != nil && r.Request != nil {
			log.Info("grinex retry",
				zap.Int("attempt", r.Request.Attempt),
				zap.Int("http_status", r.StatusCode()),
				zap.Error(err),
			)
			return
		}
		log.Info("grinex retry", zap.Error(err))
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

func retryBackoff(_ *resty.Client, resp *resty.Response) (time.Duration, error) {
	if resp == nil || resp.Request == nil {
		return 400 * time.Millisecond, nil
	}
	n := resp.Request.Attempt
	if n < 1 {
		n = 1
	}
	shift := n - 1
	if shift > 4 {
		shift = 4
	}
	ms := 400 << uint(shift)
	if ms > 5000 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// parseDepthWarmupTargets matches cmd/grinex_probe: one GET to collect cookies, then JSON GET.
//   - Depth on grinex.io → warmup https://grinex.io/ (same host as probe).
//   - Depth on api.grinex.io → warmup only https://grinex.io/ (do NOT GET api.* root; that often returns WAF HTML).
//     Origin/Referer for the JSON call are the apex site, like the probe.
func parseDepthWarmupTargets(depthURL string) (warmupURL, jsonOrigin, jsonReferer string, err error) {
	u, parseErr := url.Parse(depthURL)
	if parseErr != nil {
		return "", "", "", fmt.Errorf("parse depth url %q: %w", depthURL, parseErr)
	}
	if u.Scheme == "" || u.Host == "" {
		return "", "", "", fmt.Errorf("invalid depth url %q: missing scheme or host", depthURL)
	}
	apiHost := u.Hostname()
	apexHost := apiHost
	if h, ok := strings.CutPrefix(apiHost, "api."); ok && h != "" {
		apexHost = h
	}
	scheme := u.Scheme
	jsonOrigin = scheme + "://" + apiHost
	jsonReferer = jsonOrigin + "/"
	warmupURL = scheme + "://" + apiHost + "/"
	if apexHost != apiHost {
		warmupURL = scheme + "://" + apexHost + "/"
		jsonOrigin = scheme + "://" + apexHost
		jsonReferer = jsonOrigin + "/"
	}
	return warmupURL, jsonOrigin, jsonReferer, nil
}

// applyWarmupHeaders mimics the first document navigation (grinex_probe: Sec-Fetch-Site: none).
func applyWarmupHeaders(r *resty.Request) {
	r.SetHeader("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7").
		SetHeader("Accept-Language", "ru-RU,ru;q=0.9,en-US;q=0.8,en;q=0.7").
		SetHeader("Cache-Control", "max-age=0").
		SetHeader("Priority", "u=0, i").
		SetHeader("Sec-Fetch-Dest", "document").
		SetHeader("Sec-Fetch-Mode", "navigate").
		SetHeader("Sec-Fetch-Site", "none").
		SetHeader("Sec-Fetch-User", "?1").
		SetHeader("Upgrade-Insecure-Requests", "1")
}

// applyAPIHeaders matches grinex_probe second request (only Accept, Referer, Origin — no Sec-Fetch-*).
func applyAPIHeaders(r *resty.Request, origin, referer string) {
	r.SetHeader("Accept", "application/json, text/plain, */*").
		SetHeader("Referer", referer).
		SetHeader("Origin", origin)
}

// Fetch runs one warmup GET then GET depth URL (same cookie jar as probe).
func (c *Client) Fetch(ctx context.Context) (*OrderBook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	w := c.hc.R().SetContext(ctx)
	applyWarmupHeaders(w)
	if _, err := w.Get(c.warmupURL); err != nil {
		c.logUpstreamFailure(nil, err, "warmup_transport")
		return nil, fmt.Errorf("grinex warmup: %w", err)
	}

	req := c.hc.R().SetContext(ctx)
	applyAPIHeaders(req, c.jsonOrigin, c.jsonReferer)
	resp, err := req.Get(c.url)
	if err != nil {
		c.logUpstreamFailure(resp, err, "transport")
		return nil, err
	}
	if resp.IsError() {
		c.logUpstreamFailure(resp, nil, "http_error")
		return nil, fmt.Errorf("http %d: %s", resp.StatusCode(), string(resp.Body()))
	}

	body := resp.Body()
	if !looksLikeJSON(body) {
		c.logUpstreamFailure(resp, nil, "non_json_body")
		return nil, fmt.Errorf("%s (http %d, content-type %q, body_prefix=%q)",
			nonJSONReason(body),
			resp.StatusCode(),
			resp.Header().Get("Content-Type"),
			truncateForLog(body, 240),
		)
	}

	var env depthEnvelope
	if decErr := json.Unmarshal(body, &env); decErr != nil {
		c.logUpstreamFailure(resp, decErr, "json_decode")
		return nil, fmt.Errorf("decode depth: %w", decErr)
	}

	bids, err := parseSidePrices(env.Bids)
	if err != nil {
		return nil, fmt.Errorf("bids: %w", err)
	}
	asks, err := parseSidePrices(env.Asks)
	if err != nil {
		return nil, fmt.Errorf("asks: %w", err)
	}

	if len(bids) == 0 && len(asks) == 0 {
		return nil, fmt.Errorf("empty order book")
	}
	// Thin book: API may return only bids or only asks — mirror the other side so metrics/DB stay consistent (spread ~ 0).
	if len(asks) == 0 && len(bids) > 0 {
		c.log.Warn("grinex: ask side empty; mirroring bid ladder (synthetic asks, not real market ask depth)")
		asks = append([]float64(nil), bids...)
	} else if len(bids) == 0 && len(asks) > 0 {
		c.log.Warn("grinex: bid side empty; mirroring ask ladder (synthetic bids)")
		bids = append([]float64(nil), asks...)
	}

	book := &OrderBook{Bids: bids, Asks: asks}
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
		zap.String("url", c.url),
		zap.Int("bid_levels", len(book.Bids)),
		zap.Int("ask_levels", len(book.Asks)),
	)
	return book, nil
}

func (c *Client) logUpstreamFailure(resp *resty.Response, transportErr error, phase string) {
	fields := []zap.Field{
		zap.String("phase", phase),
		zap.String("hint", "WAF may require browser TLS/JS/cookies; plain net/http often gets HTML with 200 OK"),
	}
	if transportErr != nil {
		fields = append(fields, zap.Error(transportErr))
	}
	if resp != nil {
		h := resp.Header()
		setCookie := h.Get("Set-Cookie")
		if len(setCookie) > 220 {
			setCookie = setCookie[:220] + "…"
		}
		fields = append(fields,
			zap.Int("http_status", resp.StatusCode()),
			zap.String("http_status_line", resp.Status()),
			zap.String("content_type", h.Get("Content-Type")),
			zap.String("server", h.Get("Server")),
			zap.String("cf_ray", h.Get("Cf-Ray")),
			zap.String("cf_cache_status", h.Get("Cf-Cache-Status")),
			zap.String("x_request_id", h.Get("X-Request-Id")),
			zap.String("set_cookie", setCookie),
		)
		if len(resp.Body()) > 0 {
			fields = append(fields, zap.String("body_prefix", truncateForLog(resp.Body(), 500)))
		}
	}
	c.log.Warn("grinex upstream diagnostics", fields...)
}

// parseSidePrices supports:
//   - [{"price":"80.0","volume":"..."}] (Grinex spot / v1)
//   - [[price, qty], ...] (legacy matrix)
func parseSidePrices(data json.RawMessage) ([]float64, error) {
	trim := bytes.TrimSpace(data)
	if len(trim) == 0 || bytes.Equal(trim, []byte("null")) {
		return nil, nil
	}
	if len(trim) >= 2 && trim[0] == '[' && trim[1] == '[' {
		var rows [][]any
		if err := json.Unmarshal(trim, &rows); err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, nil
		}
		return ladder(rows)
	}
	var objs []map[string]any
	if err := json.Unmarshal(trim, &objs); err != nil {
		return nil, err
	}
	if len(objs) == 0 {
		return nil, nil
	}
	out := make([]float64, 0, len(objs))
	for _, m := range objs {
		p, ok := m["price"]
		if !ok {
			return nil, fmt.Errorf("order level missing price")
		}
		f, err := parseScalar(p)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func ladder(levels [][]any) ([]float64, error) {
	out := make([]float64, 0, len(levels))
	for _, row := range levels {
		if len(row) == 0 {
			continue
		}
		p, err := parseScalar(row[0])
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty side")
	}
	return out, nil
}

func looksLikeJSON(b []byte) bool {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return false
	}
	switch b[0] {
	case '{', '[':
		return true
	default:
		return false
	}
}

func nonJSONReason(b []byte) string {
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return "empty response body"
	}
	if looksLikeHTML(b) {
		return "response is HTML, not JSON (often WAF/captcha/block page)"
	}
	if b[0] == '<' {
		return "response is HTML, not JSON (often WAF/captcha/block page)"
	}
	return "response is not JSON"
}

func looksLikeHTML(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return false
	}
	n := min(256, len(trimmed))
	lower := strings.ToLower(string(trimmed[:n]))
	return strings.HasPrefix(lower, "<!doctype html") ||
		strings.HasPrefix(lower, "<html") ||
		strings.Contains(lower, "<head>") ||
		strings.Contains(lower, "<body")
}

func truncateForLog(b []byte, max int) string {
	s := string(b)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
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
