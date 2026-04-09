package grinex

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// grinexProductionDepthSnippet mirrors the live shape from
// https://grinex.io/api/v1/spot/depth?symbol=usdta7a5 — timestamp plus asks/bids
// as objects with price, volume, and amount as decimal strings.
const grinexProductionDepthSnippet = `{
	"timestamp": 1775595759,
	"asks": [
		{"price":"80.63","volume":"20231.6218","amount":"1631275.67"},
		{"price":"80.64","volume":"3743.4357","amount":"301870.65"}
	],
	"bids": [
		{"price":"80.47","volume":"461.9718","amount":"37174.87"},
		{"price":"80.46","volume":"85.6715","amount":"6893.13"}
	]
}`

func TestParseSidePrices_GrinexProductionObjectLevels(t *testing.T) {
	var env depthEnvelope
	require.NoError(t, json.Unmarshal([]byte(grinexProductionDepthSnippet), &env))

	asks, err := parseSidePrices(env.Asks)
	require.NoError(t, err)
	assert.Equal(t, []float64{80.63, 80.64}, asks)

	bids, err := parseSidePrices(env.Bids)
	require.NoError(t, err)
	assert.Equal(t, []float64{80.47, 80.46}, bids)
}

func TestParseSidePrices_RawAsksOnlyProductionShape(t *testing.T) {
	raw := json.RawMessage(`[{"price":"80.63","volume":"20231.6218","amount":"1631275.67"}]`)
	got, err := parseSidePrices(raw)
	require.NoError(t, err)
	assert.Equal(t, []float64{80.63}, got)
}

func TestParseSidePrices_LegacyMatrixStringPrices(t *testing.T) {
	raw := json.RawMessage(`[["100.5", "1.0"], ["99.0", "2.5"]]`)
	got, err := parseSidePrices(raw)
	require.NoError(t, err)
	assert.Equal(t, []float64{100.5, 99.0}, got)
}

func TestParseSidePrices_LegacyMatrixNumericPrices(t *testing.T) {
	raw := json.RawMessage(`[[100.5, 1.0], [99.0, 2.5]]`)
	got, err := parseSidePrices(raw)
	require.NoError(t, err)
	assert.Equal(t, []float64{100.5, 99.0}, got)
}

func TestParseSidePrices_NullData(t *testing.T) {
	got, err := parseSidePrices(json.RawMessage("null"))
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseSidePrices_EmptyData(t *testing.T) {
	got, err := parseSidePrices(nil)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestParseSidePrices_MissingPriceKey(t *testing.T) {
	raw := json.RawMessage(`[{"volume":"10","amount":"100"}]`)
	_, err := parseSidePrices(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing price")
}

func TestParseSidePrices_BadScalarType(t *testing.T) {
	raw := json.RawMessage(`[{"price": true}]`)
	_, err := parseSidePrices(raw)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported price type")
}
