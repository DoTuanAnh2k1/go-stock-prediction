package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAllFixtures(t *testing.T) {
	fixtures, err := LoadAllFixtures()
	require.NoError(t, err)

	assert.Len(t, fixtures.Exchanges, 2, "should load 2 exchanges")
	assert.Len(t, fixtures.Stocks, 5, "should load 5 stocks")
	assert.Greater(t, len(fixtures.StockPrices), 0, "should load stock prices")
	assert.Greater(t, len(fixtures.Predictions), 0, "should load predictions")
	assert.Greater(t, len(fixtures.GoldPrices), 0, "should load gold prices")

	// Verify stock data integrity
	assert.Equal(t, "VCB", fixtures.Stocks[0].Symbol)
	assert.Equal(t, "FPT", fixtures.Stocks[1].Symbol)
	assert.Equal(t, "HOSE", fixtures.Exchanges[0].Code)
}

func TestLoadFixtures_SingleFile(t *testing.T) {
	var stocks []struct {
		Symbol string `json:"symbol"`
	}
	err := LoadFixtures("stocks.json", &stocks)
	require.NoError(t, err)
	assert.Len(t, stocks, 5)
}

func TestLoadFixtures_FileNotFound(t *testing.T) {
	var data []interface{}
	err := LoadFixtures("nonexistent.json", &data)
	assert.Error(t, err)
}
