package datastore

import (
	"fmt"
	"math"
	"time"
)

// MarketBasedCalculator implements PowerCalculator using market data
type MarketBasedCalculator struct{}

// NewMarketBasedCalculator creates a new market-based power calculator
func NewMarketBasedCalculator() *MarketBasedCalculator {
	return &MarketBasedCalculator{}
}

// CalculatePower calculates power using rule of three based on market volumes
func (calc *MarketBasedCalculator) CalculatePower(maxSource float64, currentTime time.Time, data []MarketDataPoint) int64 {
	currentPeriod := calc.GetCurrentPeriod(currentTime)

	// Find current period data
	var currentVolume float64
	for _, point := range data {
		if point.Period == currentPeriod {
			currentVolume = point.Volume
			break
		}
	}

	// If no data found, return 0
	if currentVolume == 0 {
		return 0
	}

	// Find max volume in the dataset
	maxVolume := 0.0
	for _, point := range data {
		if point.Volume > maxVolume {
			maxVolume = point.Volume
		}
	}

	// Apply rule of three: if MaxSource corresponds to maxVolume, what corresponds to currentVolume?
	if maxVolume == 0 {
		return 0
	}

	power := (currentVolume / maxVolume) * maxSource
	return int64(math.Round(power))
}

// GetCurrentPeriod returns the current 15-minute market period
func (calc *MarketBasedCalculator) GetCurrentPeriod(currentTime time.Time) string {
	hour := currentTime.Hour()
	minute := currentTime.Minute()

	// Determine the 15-minute period
	periodStart := (minute / 15) * 15
	periodEnd := periodStart + 15

	if periodEnd == 60 {
		// Handle transition to next hour
		return fmt.Sprintf("%02d:%02d-%02d:00", hour, periodStart, (hour+1)%24)
	}

	periodStr := fmt.Sprintf("%02d:%02d-%02d:%02d", hour, periodStart, hour, periodEnd)

	// Handle special case for the last period of the day
	if hour == 23 && periodStart == 45 {
		return "23:45-24:00"
	}

	return periodStr
}
