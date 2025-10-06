package providers

import (
	"context"
	"fmt"
	"math"
	"time"

	"kcas/new/internal/datastore"
)

// MockProvider implements MarketDataProvider for testing/simulation
type MockProvider struct {
	name string
}

// NewMockProvider creates a new mock market data provider
func NewMockProvider() *MockProvider {
	return &MockProvider{
		name: "Mock",
	}
}

// GetName returns the provider name
func (p *MockProvider) GetName() string {
	return p.name
}

// GetDataPath returns the file path for the given date
func (p *MockProvider) GetDataPath(date time.Time) string {
	return fmt.Sprintf("mock_data_%s.csv", date.Format("2006-01-02"))
}

// FetchData generates mock market data for the given date
func (p *MockProvider) FetchData(ctx context.Context, date time.Time) ([]datastore.MarketDataPoint, error) {
	var data []datastore.MarketDataPoint

	// Generate 96 periods (24 hours * 4 periods per hour)
	for hour := 0; hour < 24; hour++ {
		for quarter := 0; quarter < 4; quarter++ {
			minute := quarter * 15
			nextMinute := minute + 15
			nextHour := hour

			if nextMinute >= 60 {
				nextMinute = 0
				nextHour = (hour + 1) % 24
			}

			// Generate period string
			var period string
			if nextHour != hour {
				period = fmt.Sprintf("%02d:%02d-%02d:%02d", hour, minute, nextHour, nextMinute)
			} else {
				period = fmt.Sprintf("%02d:%02d-%02d:%02d", hour, minute, hour, nextMinute)
			}

			// Handle special case for last period
			if hour == 23 && quarter == 3 {
				period = "23:45-24:00"
			}

			// Generate realistic-looking data using sine waves
			timeOfDay := float64(hour) + float64(minute)/60.0

			// Volume varies with a daily pattern (higher during day, lower at night)
			baseVolume := 70.0 + 30.0*math.Sin((timeOfDay-6)*math.Pi/12) // Peak around noon
			volumeNoise := 10.0 * math.Sin(timeOfDay*math.Pi/3)          // Add some variation
			volume := math.Max(20.0, baseVolume+volumeNoise)

			// Price generally inversely related to volume with random variation
			basePrice := 120.0 - (volume-50.0)*0.8 // Inverse relationship
			priceNoise := 20.0 * math.Sin(timeOfDay*math.Pi/2)
			price := math.Max(10.0, basePrice+priceNoise)

			data = append(data, datastore.MarketDataPoint{
				Period: period,
				Volume: math.Round(volume*10) / 10,  // Round to 1 decimal
				Price:  math.Round(price*100) / 100, // Round to 2 decimals
			})
		}
	}

	return data, nil
}
