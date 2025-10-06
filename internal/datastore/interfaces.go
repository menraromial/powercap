package datastore

import (
	"context"
	"time"
)

// MarketDataPoint represents a single data point in the market data
type MarketDataPoint struct {
	Period string  `csv:"Period"`        // Time period (e.g., "00:00-00:15")
	Volume float64 `csv:"Volume (MWh)"`  // Volume in MWh
	Price  float64 `csv:"Price (€/MWh)"` // Price in €/MWh
}

// MarketDataProvider defines the interface for market data providers
type MarketDataProvider interface {
	// GetName returns the provider name
	GetName() string

	// FetchData fetches market data for the given date
	FetchData(ctx context.Context, date time.Time) ([]MarketDataPoint, error)

	// GetDataPath returns the file path for the given date
	GetDataPath(date time.Time) string
}

// DataStore manages market data storage and retrieval
type DataStore interface {
	// LoadData loads market data for the given date
	LoadData(date time.Time) ([]MarketDataPoint, error)

	// SaveData saves market data to storage
	SaveData(date time.Time, data []MarketDataPoint) error

	// GetCurrentData returns the currently loaded data
	GetCurrentData() []MarketDataPoint

	// RefreshData refreshes data for the given date
	RefreshData(ctx context.Context, date time.Time) error

	// SetProvider sets the market data provider
	SetProvider(provider MarketDataProvider)
}

// PowerCalculator calculates power based on market data
type PowerCalculator interface {
	// CalculatePower calculates power for the current time
	CalculatePower(maxSource float64, currentTime time.Time, data []MarketDataPoint) int64

	// GetCurrentPeriod returns the current market period
	GetCurrentPeriod(currentTime time.Time) string
}
