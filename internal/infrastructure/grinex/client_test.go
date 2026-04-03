package grinex_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"usdt-rates/internal/infrastructure/grinex"
)

func TestClient_Fetch_Success_LegacyFormat(t *testing.T) {
	// Mock server returning legacy format [[price, qty], ...]
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"bids": [["100.5", "1.0"], ["100.0", "2.5"]],
			"asks": [["101.0", "1.5"], ["101.5", "3.0"]],
			"timestamp": 1672531200
		}`))
	}))
	defer server.Close()

	logger := zap.NewNop()
	client, err := grinex.NewClient(2*time.Second, server.URL, logger)
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
		_, _ = w.Write([]byte(`{
			"bids": [{"price": "90.5", "volume": "10"}, {"price": 90.0, "volume": "20"}],
			"asks": [{"price": "91.0", "volume": "15"}, {"price": 91.5, "volume": "25"}],
			"ts": 1672531205
		}`))
	}))
	defer server.Close()

	logger := zap.NewNop()
	client, err := grinex.NewClient(2*time.Second, server.URL, logger)
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
		_, _ = w.Write([]byte(`{
			"bids": [["90.5", "10"], ["90.0", "20"]],
			"asks": [],
			"ts_ms": 1672531205000
		}`))
	}))
	defer server.Close()

	logger := zap.NewNop()
	client, err := grinex.NewClient(2*time.Second, server.URL, logger)
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
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer server.Close()

	logger := zap.NewNop()
	// Set very short timeout to fail fast
	client, err := grinex.NewClient(10*time.Millisecond, server.URL, logger)
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
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	logger := zap.NewNop()
	client, err := grinex.NewClient(2*time.Second, server.URL, logger)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = client.Fetch(ctx)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "decode depth")
}

func TestNewClient_InvalidURL(t *testing.T) {
	logger := zap.NewNop()
	_, err := grinex.NewClient(2*time.Second, "::invalid-url", logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse depth url")
}

func TestNewClient_MissingHost(t *testing.T) {
	logger := zap.NewNop()
	_, err := grinex.NewClient(2*time.Second, "/relative/path", logger)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid depth url")
}
