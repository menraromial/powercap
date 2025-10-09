package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"kcas/new/internal/config"
	"kcas/new/internal/datastore"
	"kcas/new/internal/power"
	"kcas/new/pkg/providers"
)

func main() {
	logger := log.New(os.Stdout, "[PowerManager] ", log.LstdFlags|log.Lmicroseconds)
	logger.Println("Starting professional power management system...")

	// Load configuration first to get timezone
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Set timezone globally for all time operations
	if err := setTimezone(cfg.Timezone, logger); err != nil {
		logger.Printf("Warning: Failed to set timezone %s: %v", cfg.Timezone, err)
		logger.Println("Continuing with system timezone...")
	}

	// Check for test mode
	if len(os.Args) > 1 && os.Args[1] == "test-data" {
		runTestMode(logger)
		return
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize power manager (provider is configured via environment variables)
	pm, err := power.NewManager(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize power manager: %v", err)
	}

	// Load initial data
	today := time.Now()
	if err := pm.LoadData(today); err != nil {
		logger.Printf("Warning: Failed to load initial data: %v", err)
		logger.Println("System will attempt to generate data automatically")
	}

	// Initialize Kubernetes node
	if err := pm.InitializeNode(); err != nil {
		logger.Fatalf("Failed to initialize node: %v", err)
	}

	// Start the power management cycle
	logger.Println("Power management system ready - starting main cycle")
	pm.Run() // This will block until context is cancelled
}

func runTestMode(logger *log.Logger) {
	logger.Println("Running in test mode - full power calculation test...")

	ctx := context.Background()

	// Run complete test with CSV generation
	if len(os.Args) > 2 && os.Args[2] == "full" {
		runFullTest(logger, ctx)
		return
	}

	// Just test data fetching
	logger.Println("Testing data fetch only...")
	epexProvider := providers.NewDefaultEPEXProvider()

	today := time.Now()
	data, err := epexProvider.FetchData(ctx, today)
	if err != nil {
		logger.Fatalf("Failed to fetch test data: %v", err)
	}

	logger.Printf("Successfully fetched %d data points", len(data))

	// Save data manually for testing
	if len(data) > 0 {
		logger.Println("Sample data points:")
		for i := 0; i < 5 && i < len(data); i++ {
			logger.Printf("  %s: %.1f MWh, %.2f €/MWh",
				data[i].Period, data[i].Volume, data[i].Price)
		}
	}

	logger.Println("Test mode completed successfully")
}

func runFullTest(logger *log.Logger, ctx context.Context) {
	logger.Println("Running full power calculation and CSV generation test...")

	// Initialize components manually for local testing
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load config: %v", err)
	}

	// Create provider factory and get provider
	factory := providers.NewProviderFactory()
	provider, err := factory.CreateProvider(cfg)
	if err != nil {
		logger.Fatalf("Failed to create provider: %v", err)
	}

	// Fetch data
	today := time.Now()
	logger.Printf("Fetching data from %s provider...", cfg.DataProvider)
	data, err := provider.FetchData(ctx, today)
	if err != nil {
		logger.Fatalf("Failed to fetch data: %v", err)
	}

	logger.Printf("Successfully fetched %d data points", len(data))

	// Initialize datastore
	ds := datastore.NewCSVDataStore(logger)
	ds.SetProvider(provider)

	// Save data
	if err := ds.SaveData(today, data); err != nil {
		logger.Fatalf("Failed to store data: %v", err)
	}

	// Calculate power for each data point
	logger.Println("Calculating power consumption for each time period...")
	var totalCalculations int

	// For testing, use a default max power of 40W (typical CPU TDP)
	// In production, this comes from RAPL hardware limits
	maxSource := float64(40000000) // 40W in microwatts

	// Get max volume from datastore (calculated once during SaveData)
	maxVolume := ds.GetMaxVolume()
	logger.Printf("Using max volume: %.1f MWh", maxVolume)
	logger.Printf("Using test max power: %.1f W", maxSource/1000000)

	for i, point := range data {
		// Manual calculation since we're testing
		var powerMicroWatts int64
		if maxVolume > 0 {
			// Rule of three: currentVolume / maxVolume = currentPower / maxPower
			powerMicroWatts = int64((point.Volume / maxVolume) * maxSource)
		}

		logger.Printf("Period %s: Price=%.2f €/MWh, Volume=%.1f MWh → Power=%d µW (%.1f W)",
			point.Period, point.Price, point.Volume, powerMicroWatts, float64(powerMicroWatts)/1000000)

		totalCalculations++
		if i >= 9 { // Show first 10 calculations
			break
		}
	}

	if len(data) > 10 {
		logger.Printf("... and %d more calculations", len(data)-10)
	}

	// Generate CSV
	filename := fmt.Sprintf("epex_data_%s.csv", today.Format("2006-01-02"))
	logger.Printf("Generating CSV file: %s", filename)

	if err := ds.SaveData(today, data); err != nil {
		logger.Fatalf("Failed to generate CSV: %v", err)
	}

	logger.Printf("✅ Full test completed successfully!")
	logger.Printf("   - Fetched: %d data points", len(data))
	logger.Printf("   - Calculated: %d power values", totalCalculations)
	logger.Printf("   - Generated: %s", filename)
}

// setTimezone sets the global timezone for the application
func setTimezone(timezone string, logger *log.Logger) error {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("invalid timezone %s: %w", timezone, err)
	}

	time.Local = loc
	logger.Printf("🌍 Timezone set to: %s (current time: %s)", timezone, time.Now().Format("15:04:05 MST"))
	return nil
}
