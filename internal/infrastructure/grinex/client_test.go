package grinex_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"usdt-rates/internal/infrastructure/grinex"
	"usdt-rates/pkg/circuitbreaker"
)

func testGrinexURLs(t *testing.T, srv *httptest.Server) (warmup, depth, origin, referer string) {
	t.Helper()
	depth = srv.URL
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	origin = u.Scheme + "://" + u.Host
	warmup = origin + "/"
	referer = origin + "/"
	return warmup, depth, origin, referer
}

func TestClient_Fetch_Success_LegacyFormat(t *testing.T) {
	// Mock server returning legacy format [[price, qty], ...]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"bids": [["100.5", "1.0"], ["100.0", "2.5"]],
			"asks": [["101.0", "1.5"], ["101.5", "3.0"]],
			"timestamp": 1672531200
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	client, err := grinex.NewClient(2*time.Second, depth, warmup, origin, referer, 4, circuitbreaker.Settings{Enabled: false}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	book, err := client.Fetch(ctx)

	require.NoError(t, err)
	assert.Equal(t, []float64{100.5, 100.0}, book.Bids)
	assert.Equal(t, []float64{101.0, 101.5}, book.Asks)
	assert.True(t, book.HasServerTime)
	assert.Equal(t, int64(1672531200), book.ExchangeTime.Unix())
}

func TestClient_Fetch_Success_CurrentFormat(t *testing.T) {
	// Mock server returning current format [{"price":"80.0","volume":"..."}, ...]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"bids": [{"price": "90.5", "volume": "10"}, {"price": 90.0, "volume": "20"}],
			"asks": [{"price": "91.0", "volume": "15"}, {"price": 91.5, "volume": "25"}],
			"ts": 1672531205
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	client, err := grinex.NewClient(2*time.Second, depth, warmup, origin, referer, 4, circuitbreaker.Settings{Enabled: false}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	book, err := client.Fetch(ctx)

	require.NoError(t, err)
	assert.Equal(t, []float64{90.5, 90.0}, book.Bids)
	assert.Equal(t, []float64{91.0, 91.5}, book.Asks)
	assert.True(t, book.HasServerTime)
	assert.Equal(t, int64(1672531205), book.ExchangeTime.Unix())
}

func TestClient_Fetch_EmptyBookFallback(t *testing.T) {
	// Test fallback when asks are missing but bids exist
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{
			"bids": [["90.5", "10"], ["90.0", "20"]],
			"asks": [],
			"ts_ms": 1672531205000
		}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	client, err := grinex.NewClient(2*time.Second, depth, warmup, origin, referer, 4, circuitbreaker.Settings{Enabled: false}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	book, err := client.Fetch(ctx)

	require.NoError(t, err)
	assert.Equal(t, []float64{90.5, 90.0}, book.Bids)
	// Fallback logic copies bids to asks
	assert.Equal(t, []float64{90.5, 90.0}, book.Asks)
	assert.True(t, book.HasServerTime)
}

func TestClient_Fetch_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`Internal Server Error`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	// Set very short timeout to fail fast
	client, err := grinex.NewClient(10*time.Millisecond, depth, warmup, origin, referer, 4, circuitbreaker.Settings{Enabled: false}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Fetch(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "grinex http 500")
}

func TestClient_Fetch_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(`{invalid json`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	client, err := grinex.NewClient(2*time.Second, depth, warmup, origin, referer, 4, circuitbreaker.Settings{Enabled: false}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Fetch(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode depth")
}

func TestClient_Fetch_CircuitBreakerOpens(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(`err`))
		require.NoError(t, err)
	}))
	defer server.Close()

	logger := zap.NewNop()
	warmup, depth, origin, referer := testGrinexURLs(t, server)
	client, err := grinex.NewClient(2*time.Second, depth, warmup, origin, referer, 4, circuitbreaker.Settings{
		Name:                "test",
		Enabled:             true,
		ConsecutiveFailures: 2,
		OpenTimeout:         time.Hour,
		HalfOpenMaxRequests: 1,
		Interval:            0,
	}, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Fetch(ctx)
	require.Error(t, err)
	_, err = client.Fetch(ctx)
	require.Error(t, err)
	_, err = client.Fetch(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "circuit breaker open")
}

func TestNewClient_CircuitBreakerInvalidSettings(t *testing.T) {
	logger := zap.NewNop()
	_, err := grinex.NewClient(2*time.Second, "https://example.com/depth", "https://example.com/", "https://example.com", "https://example.com/", 4, circuitbreaker.Settings{
		Enabled:             true,
		ConsecutiveFailures: 0,
		OpenTimeout:         time.Second,
		HalfOpenMaxRequests: 1,
	}, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "circuit breaker")
}

func TestNewClient_MaxConcurrentInvalid(t *testing.T) {
	logger := zap.NewNop()
	_, err := grinex.NewClient(2*time.Second, "https://example.com/depth", "https://example.com/", "https://example.com", "https://example.com/", 0, circuitbreaker.Settings{Enabled: false}, logger)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "maxConcurrent")
}
