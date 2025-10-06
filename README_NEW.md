# Professional Power Management System

A modular, enterprise-grade power management system for Kubernetes nodes with pluggable market data providers.

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────┐
│                    Main Application                         │
└─────────────────────┬───────────────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────────────┐
│                Power Manager                                │
│  • Orchestrates all components                             │
│  • Handles Kubernetes integration                          │
│  • Manages power adjustment cycles                         │
└─────┬─────────────┬─────────────────┬─────────────────────┘
      │             │                 │
┌─────▼─────┐ ┌─────▼─────┐ ┌─────────▼─────────┐
│   RAPL    │ │   Data    │ │   Market Data     │
│  Manager  │ │   Store   │ │   Providers       │
│           │ │           │ │                   │
│ • Domain  │ │ • CSV     │ │ • EPEX (live)     │
│   Discovery│ │   Storage │ │ • Mock (testing)  │
│ • Power   │ │ • Loading │ │ • Static (demo)   │
│   Control │ │ • Caching │ │ • Custom...       │
└───────────┘ └───────────┘ └───────────────────┘
```

## 📁 Project Structure

```
powercap/
├── main.go                    # Application entry point
├── internal/                  # Private application packages
│   ├── config/               # Configuration management
│   │   └── config.go         # Environment variable handling
│   ├── datastore/            # Data abstraction layer
│   │   ├── interfaces.go     # Core interfaces
│   │   ├── csv_store.go      # CSV storage implementation
│   │   └── calculator.go     # Power calculation logic
│   ├── power/                # Power management core
│   │   └── manager.go        # Main power management logic
│   └── rapl/                 # Hardware abstraction
│       └── manager.go        # RAPL domain management
└── pkg/                      # Public packages
    └── providers/            # Market data providers
        ├── epex.go          # EPEX market data provider
        ├── mock.go          # Mock provider for testing
        └── static.go        # Static data provider
```

## 🔌 Pluggable Architecture

### Market Data Providers

The system supports multiple market data sources through a common interface:

```go
type MarketDataProvider interface {
    GetName() string
    FetchData(ctx context.Context, date time.Time) ([]MarketDataPoint, error)
    GetDataPath(date time.Time) string
}
```

#### Available Providers

1. **EPEX Provider** (`pkg/providers/epex.go`)
   - Scrapes real data from EPEX market
   - Default production provider
   - Handles HTTP requests and HTML parsing

2. **Mock Provider** (`pkg/providers/mock.go`)
   - Generates realistic synthetic data
   - Perfect for development and testing
   - Uses mathematical models for data generation

3. **Static Provider** (`pkg/providers/static.go`)
   - Uses predefined static data
   - Useful for demos and controlled testing
   - Configurable data sets

### Adding Custom Providers

To add a new market data provider:

1. Create a new file in `pkg/providers/`
2. Implement the `MarketDataProvider` interface
3. Register it in your main application

Example:
```go
// pkg/providers/custom.go
type CustomProvider struct {
    // Your implementation
}

func (p *CustomProvider) GetName() string {
    return "Custom"
}

func (p *CustomProvider) FetchData(ctx context.Context, date time.Time) ([]datastore.MarketDataPoint, error) {
    // Your data fetching logic
}

func (p *CustomProvider) GetDataPath(date time.Time) string {
    return fmt.Sprintf("custom_data_%s.csv", date.Format("2006-01-02"))
}
```

## 🚀 Usage

### Standard Operation
```bash
# Build the application
go build -o powercap main.go

# Run with EPEX provider (default)
export NODE_NAME="worker-node-1"
export MAX_SOURCE="40000000"
./powercap
```

### Testing with Different Providers
```bash
# Test data generation
./powercap test-data

# The system automatically uses EPEX provider by default
# To use different providers, modify main.go:

// Use Mock provider for testing
mockProvider := providers.NewMockProvider()
pm.SetDataProvider(mockProvider)

// Use Static provider for demos
staticProvider := providers.NewStaticProviderWithDefaults()
pm.SetDataProvider(staticProvider)
```

## ⚙️ Configuration

All configuration is handled through environment variables:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `NODE_NAME` | Kubernetes node name | - | ✅ |
| `MAX_SOURCE` | Maximum power source (µW) | `40000000` | ❌ |
| `STABILISATION_TIME` | Adjustment interval (seconds) | `300` | ❌ |
| `RAPL_MIN_POWER` | Minimum power limit (µW) | `10000000` | ❌ |

## 🔄 Data Flow

1. **Initialization**
   - Load configuration from environment
   - Initialize RAPL manager and discover domains
   - Set up data store and calculator
   - Configure market data provider

2. **Data Loading**
   - Check for existing CSV file
   - If not found, fetch from provider
   - Load data into memory for fast access

3. **Power Calculation**
   - Get current 15-minute period
   - Find volume for current period
   - Calculate power using rule of three
   - Apply constraints (min/max limits)

4. **Power Application**
   - Update Kubernetes node labels
   - Write power limits to RAPL files
   - Log all operations

5. **Automatic Refresh**
   - Schedule daily data refresh at midnight
   - Reload data in memory
   - Continue operation seamlessly

## 🧪 Testing

### Unit Tests
```bash
# Run all tests
go test ./...

# Test specific package
go test ./internal/datastore
go test ./pkg/providers
```

### Integration Tests
```bash
# Test with mock provider
go run main.go test-data

# Test RAPL discovery (requires hardware)
# Set NODE_NAME=test-node before running
```

### Provider Testing
```bash
# Test different providers
go run examples/provider_test.go
```

## 🏆 Benefits of This Architecture

### 1. **Separation of Concerns**
- Data logic separated from business logic
- Hardware abstraction isolated
- Clear interfaces between components

### 2. **Extensibility**
- Easy to add new market data sources
- Simple to implement different calculation algorithms
- Modular design allows component replacement

### 3. **Testability**
- Mock providers for unit testing
- Isolated components for focused testing
- Dependency injection for test scenarios

### 4. **Maintainability**
- Clean package structure
- Well-defined interfaces
- Single responsibility principle

### 5. **Production Ready**
- Comprehensive error handling
- Detailed logging
- Graceful degradation
- Resource management

## 🔧 Development Guidelines

### Adding New Features

1. **New Data Sources**
   - Implement `MarketDataProvider` interface
   - Add to `pkg/providers/`
   - Follow existing patterns

2. **New Calculation Methods**
   - Implement `PowerCalculator` interface
   - Add to `internal/datastore/`
   - Update power manager

3. **New Storage Backends**
   - Implement `DataStore` interface
   - Add configuration options
   - Maintain backward compatibility

### Code Quality

- Follow Go best practices
- Add comprehensive tests
- Document public interfaces
- Use meaningful variable names
- Handle errors appropriately

## 📊 Monitoring and Observability

### Logging Levels
- **INFO**: Normal operations, data loading, power adjustments
- **WARN**: Recoverable errors, fallback scenarios
- **ERROR**: Critical errors, system failures

### Key Metrics to Monitor
- Power adjustment frequency
- Data refresh success rate
- RAPL operation errors
- Market data availability

### Example Log Output
```
[PowerManager] Starting professional power management system...
[PowerManager] Configured data provider: EPEX
[PowerManager] Loaded 96 market data points for 2025-10-06
[PowerManager] Discovered 2 RAPL domains
[PowerManager] Node initialized with max power: 50000000 µW
[PowerManager] Power calculation: period=14:30-14:45, source=32769231 µW, max=50000000 µW, min=10000000 µW, applied=32769231 µW
```

This professional architecture ensures scalability, maintainability, and extensibility while providing a robust foundation for enterprise power management systems.