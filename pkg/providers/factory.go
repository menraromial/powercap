package providers

import (
	"fmt"
	"strings"

	"kcas/new/internal/config"
	"kcas/new/internal/datastore"
)

// ProviderFactory creates market data providers based on configuration
type ProviderFactory struct{}

// NewProviderFactory creates a new provider factory
func NewProviderFactory() *ProviderFactory {
	return &ProviderFactory{}
}

// CreateProvider creates a provider based on configuration
func (f *ProviderFactory) CreateProvider(cfg *config.Config) (datastore.MarketDataProvider, error) {
	providerType := strings.ToLower(cfg.DataProvider)

	switch providerType {
	case "epex":
		return NewEPEXProvider(cfg.ProviderURL, cfg.ProviderParams), nil

	case "mock":
		return NewMockProvider(), nil

	case "static":
		return NewStaticProviderWithDefaults(), nil

	default:
		return nil, fmt.Errorf("unknown provider type: %s. Supported types: epex, mock, static", cfg.DataProvider)
	}
}

// GetSupportedProviders returns a list of supported provider types
func (f *ProviderFactory) GetSupportedProviders() []string {
	return []string{"epex", "mock", "static"}
}

// ValidateProviderConfig validates provider configuration
func (f *ProviderFactory) ValidateProviderConfig(cfg *config.Config) error {
	supported := f.GetSupportedProviders()
	providerType := strings.ToLower(cfg.DataProvider)

	// Check if provider type is supported
	for _, p := range supported {
		if p == providerType {
			// Provider type is valid, perform provider-specific validation
			return f.validateSpecificProvider(providerType, cfg)
		}
	}

	return fmt.Errorf("unsupported provider type: %s. Supported types: %v", cfg.DataProvider, supported)
}

// validateSpecificProvider performs provider-specific validation
func (f *ProviderFactory) validateSpecificProvider(providerType string, cfg *config.Config) error {
	switch providerType {
	case "epex":
		if cfg.ProviderURL == "" {
			return fmt.Errorf("EPEX provider requires a valid URL")
		}
		// Validate required EPEX parameters
		requiredParams := []string{"market_area", "auction", "modality", "sub_modality"}
		for _, param := range requiredParams {
			if _, exists := cfg.ProviderParams[param]; !exists {
				return fmt.Errorf("EPEX provider missing required parameter: %s", param)
			}
		}

	case "mock":
		// Mock provider doesn't require special validation

	case "static":
		// Static provider doesn't require special validation

	default:
		return fmt.Errorf("unknown provider type for validation: %s", providerType)
	}

	return nil
}
