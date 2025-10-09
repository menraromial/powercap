package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Environment variable names
const (
	EnvNodeName          = "NODE_NAME"
	EnvStabilisationTime = "STABILISATION_TIME"
	EnvRaplLimit         = "RAPL_MIN_POWER"
	EnvTimezone          = "TIMEZONE"

	// Provider configuration
	EnvDataProvider    = "DATA_PROVIDER"     // epex, mock, static
	EnvProviderURL     = "PROVIDER_URL"      // Base URL for data provider
	EnvProviderParams  = "PROVIDER_PARAMS"   // Additional parameters (JSON format)
	EnvDataRefreshCron = "DATA_REFRESH_CRON" // Cron expression for data refresh
)

// Default values
const (
	DefaultStabilisationTime = "300"
	DefaultRaplLimit         = "10000000"
	DefaultTimezone          = "Europe/Paris"

	// Provider defaults
	DefaultDataProvider    = "epex"
	DefaultProviderURL     = "https://www.epexspot.com/en/market-results"
	DefaultProviderParams  = `{"market_area":"FR","auction":"IDA1","modality":"Auction","sub_modality":"Intraday"}`
	DefaultDataRefreshCron = "0 0 * * *" // Every day at midnight
)

// Config holds the application configuration
type Config struct {
	StabilisationTime time.Duration
	RaplLimit         int64
	NodeName          string
	Timezone          string // Timezone for time calculations

	// Provider configuration
	DataProvider    string            // Type of data provider
	ProviderURL     string            // Base URL for provider
	ProviderParams  map[string]string // Additional provider parameters
	DataRefreshCron string            // Cron expression for data refresh
}

// Load loads configuration from environment variables
func Load() (*Config, error) {
	// NODE_NAME is required for Kubernetes, but we can provide a default for local testing
	nodeName := os.Getenv(EnvNodeName)
	if nodeName == "" {
		// For local/Docker testing, use a default node name
		nodeName = "local-node"
	}

	stabilisationTime, err := time.ParseDuration(getEnvOrDefault(EnvStabilisationTime, DefaultStabilisationTime) + "s")
	if err != nil {
		return nil, fmt.Errorf("invalid stabilisation time: %w", err)
	}

	raplLimit, err := strconv.ParseInt(getEnvOrDefault(EnvRaplLimit, DefaultRaplLimit), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RAPL limit: %w", err)
	}

	// Load provider configuration
	providerParams, err := parseProviderParams(getEnvOrDefault(EnvProviderParams, DefaultProviderParams))
	if err != nil {
		return nil, fmt.Errorf("invalid provider params: %w", err)
	}

	return &Config{
		StabilisationTime: stabilisationTime,
		RaplLimit:         raplLimit,
		NodeName:          nodeName,
		Timezone:          getEnvOrDefault(EnvTimezone, DefaultTimezone),
		DataProvider:      getEnvOrDefault(EnvDataProvider, DefaultDataProvider),
		ProviderURL:       getEnvOrDefault(EnvProviderURL, DefaultProviderURL),
		ProviderParams:    providerParams,
		DataRefreshCron:   getEnvOrDefault(EnvDataRefreshCron, DefaultDataRefreshCron),
	}, nil
}

// parseProviderParams parses provider parameters from JSON string
func parseProviderParams(jsonStr string) (map[string]string, error) {
	var params map[string]string
	if err := json.Unmarshal([]byte(jsonStr), &params); err != nil {
		return nil, fmt.Errorf("failed to parse provider params JSON: %w", err)
	}
	return params, nil
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}
