package providers

import (
	"context"
	"fmt"
	"time"

	"kcas/new/internal/datastore"
)

// StaticProvider implements MarketDataProvider with static data
type StaticProvider struct {
	name string
	data []datastore.MarketDataPoint
}

// NewStaticProvider creates a new static market data provider
func NewStaticProvider(data []datastore.MarketDataPoint) *StaticProvider {
	return &StaticProvider{
		name: "Static",
		data: data,
	}
}

// NewStaticProviderWithDefaults creates a static provider with default test data
func NewStaticProviderWithDefaults() *StaticProvider {
	// Generate a full day of data with simple pattern
	var fullData []datastore.MarketDataPoint
	for hour := 0; hour < 24; hour++ {
		for quarter := 0; quarter < 4; quarter++ {
			minute := quarter * 15
			nextMinute := minute + 15
			nextHour := hour

			if nextMinute >= 60 {
				nextMinute = 0
				nextHour = (hour + 1) % 24
			}

			var period string
			if nextHour != hour {
				period = fmt.Sprintf("%02d:%02d-%02d:%02d", hour, minute, nextHour, nextMinute)
			} else {
				period = fmt.Sprintf("%02d:%02d-%02d:%02d", hour, minute, hour, nextMinute)
			}

			if hour == 23 && quarter == 3 {
				period = "23:45-24:00"
			}

			// Simple pattern: volume increases during day, decreases at night
			volume := 30.0 + float64(hour*2) // Increases with hour
			if hour > 12 {
				volume = 30.0 + float64((24-hour)*2) // Decreases after noon
			}

			price := 120.0 - volume // Simple inverse relationship

			fullData = append(fullData, datastore.MarketDataPoint{
				Period: period,
				Volume: volume,
				Price:  price,
			})
		}
	}

	return &StaticProvider{
		name: "Static",
		data: fullData,
	}
}

// GetName returns the provider name
func (p *StaticProvider) GetName() string {
	return p.name
}

// GetDataPath returns the file path for the given date
func (p *StaticProvider) GetDataPath(date time.Time) string {
	return fmt.Sprintf("static_data_%s.csv", date.Format("2006-01-02"))
}

// FetchData returns the static data (ignores date parameter)
func (p *StaticProvider) FetchData(ctx context.Context, date time.Time) ([]datastore.MarketDataPoint, error) {
	// Return a copy of the static data
	result := make([]datastore.MarketDataPoint, len(p.data))
	copy(result, p.data)
	return result, nil
}

// SetData allows updating the static data
func (p *StaticProvider) SetData(data []datastore.MarketDataPoint) {
	p.data = make([]datastore.MarketDataPoint, len(data))
	copy(p.data, data)
}
