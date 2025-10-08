package datastore

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// CSVDataStore implements DataStore interface for CSV-based storage
type CSVDataStore struct {
	provider    MarketDataProvider
	currentData []MarketDataPoint
	maxVolume   float64 // Cached maximum volume for the current day
	logger      *log.Logger
}

// NewCSVDataStore creates a new CSV-based data store
func NewCSVDataStore(logger *log.Logger) *CSVDataStore {
	return &CSVDataStore{
		logger:      logger,
		currentData: make([]MarketDataPoint, 0),
	}
}

// SetProvider sets the market data provider
func (ds *CSVDataStore) SetProvider(provider MarketDataProvider) {
	ds.provider = provider
}

// LoadData loads market data for the given date
func (ds *CSVDataStore) LoadData(date time.Time) ([]MarketDataPoint, error) {
	if ds.provider == nil {
		return nil, fmt.Errorf("no market data provider set")
	}

	filePath := ds.provider.GetDataPath(date)

	// Check if file exists, if not try to generate it
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		ds.logger.Printf("Data file %s not found, attempting to generate...", filePath)
		if err := ds.RefreshData(context.Background(), date); err != nil {
			ds.logger.Printf("Failed to generate data: %v", err)
			// Try yesterday's file as fallback
			yesterday := date.AddDate(0, 0, -1)
			filePath = ds.provider.GetDataPath(yesterday)
			ds.logger.Printf("Trying fallback file: %s", filePath)
		}
	}

	data, err := ds.loadFromCSV(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to load data from %s: %w", filePath, err)
	}

	ds.currentData = data
	ds.updateMaxVolume(data)
	return data, nil
}

// SaveData saves market data to CSV file
func (ds *CSVDataStore) SaveData(date time.Time, data []MarketDataPoint) error {
	if ds.provider == nil {
		return fmt.Errorf("no market data provider set")
	}

	filePath := ds.provider.GetDataPath(date)
	if err := ds.saveToCSV(filePath, data); err != nil {
		return err
	}

	// Update internal state after successful save
	ds.currentData = data
	ds.updateMaxVolume(data)

	return nil
}

// GetCurrentData returns the currently loaded data
func (ds *CSVDataStore) GetCurrentData() []MarketDataPoint {
	return ds.currentData
}

// GetMaxVolume returns the cached maximum volume for the current day
func (ds *CSVDataStore) GetMaxVolume() float64 {
	return ds.maxVolume
}

// RefreshData refreshes data for the given date by fetching from provider
func (ds *CSVDataStore) RefreshData(ctx context.Context, date time.Time) error {
	if ds.provider == nil {
		ds.logger.Printf("❌ No market data provider set for refresh operation")
		return fmt.Errorf("no market data provider set")
	}

	ds.logger.Printf("🔄 Refreshing market data for %s using provider '%s'...",
		date.Format("2006-01-02"), ds.provider.GetName())

	startTime := time.Now()
	data, err := ds.provider.FetchData(ctx, date)
	fetchDuration := time.Since(startTime)

	if err != nil {
		ds.logger.Printf("❌ Failed to fetch data from provider '%s' after %v: %v",
			ds.provider.GetName(), fetchDuration, err)
		return fmt.Errorf("failed to fetch data: %w", err)
	}

	if len(data) == 0 {
		ds.logger.Printf("❌ No data retrieved from provider '%s'", ds.provider.GetName())
		return fmt.Errorf("no data retrieved from provider")
	}

	ds.logger.Printf("✅ Successfully fetched %d data points from '%s' in %v",
		len(data), ds.provider.GetName(), fetchDuration)

	// Log sample of fetched data
	if len(data) > 0 {
		ds.logger.Printf("   📊 Sample fetched data:")
		sampleCount := 3
		if len(data) < sampleCount {
			sampleCount = len(data)
		}
		for i := 0; i < sampleCount; i++ {
			ds.logger.Printf("      %s: %.1f MWh @ %.2f €/MWh",
				data[i].Period, data[i].Volume, data[i].Price)
		}
		if len(data) > sampleCount {
			ds.logger.Printf("      ... and %d more data points", len(data)-sampleCount)
		}
	}

	ds.logger.Printf("💾 Saving fetched data to CSV...")
	if err := ds.SaveData(date, data); err != nil {
		ds.logger.Printf("❌ Failed to save data: %v", err)
		return fmt.Errorf("failed to save data: %w", err)
	}

	ds.currentData = data
	ds.updateMaxVolume(data)
	ds.logger.Printf("✅ Successfully refreshed data for %s", date.Format("2006-01-02"))
	return nil
}

// updateMaxVolume calculates and caches the maximum volume from the dataset
func (ds *CSVDataStore) updateMaxVolume(data []MarketDataPoint) {
	ds.logger.Printf("📊 Calculating maximum volume from %d data points...", len(data))

	ds.maxVolume = 0.0
	var maxVolumeTime string

	for _, point := range data {
		if point.Volume > ds.maxVolume {
			ds.maxVolume = point.Volume
			maxVolumeTime = point.Period
		}
	}

	ds.logger.Printf("✅ Maximum volume calculated: %.1f MWh at period %s", ds.maxVolume, maxVolumeTime)
}

// loadFromCSV loads data from a CSV file
func (ds *CSVDataStore) loadFromCSV(filePath string) ([]MarketDataPoint, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file has insufficient data")
	}

	var data []MarketDataPoint
	// Skip header row
	for i, record := range records[1:] {
		if len(record) != 3 {
			ds.logger.Printf("Warning: Skipping malformed record at line %d", i+2)
			continue
		}

		volume, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			ds.logger.Printf("Warning: Invalid volume at line %d: %v", i+2, err)
			continue
		}

		price, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			ds.logger.Printf("Warning: Invalid price at line %d: %v", i+2, err)
			continue
		}

		data = append(data, MarketDataPoint{
			Period: record[0],
			Volume: volume,
			Price:  price,
		})
	}

	return data, nil
}

// saveToCSV saves data to a CSV file
func (ds *CSVDataStore) saveToCSV(filePath string, data []MarketDataPoint) error {
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Period", "Volume (MWh)", "Price (€/MWh)"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	// Write data
	for _, point := range data {
		row := []string{
			point.Period,
			strconv.FormatFloat(point.Volume, 'f', 1, 64),
			strconv.FormatFloat(point.Price, 'f', 2, 64),
		}
		if err := writer.Write(row); err != nil {
			return fmt.Errorf("failed to write data row: %w", err)
		}
	}

	return nil
}
