# PowerCap Manager - Comprehensive System Overview

## 🎯 System Architecture & End-to-End Operation

The PowerCap Manager is a sophisticated power management system that dynamically adjusts CPU power consumption based on real-time electricity market data. Here's how it works from startup to operation.

## 🚀 System Startup Flow

### 1. **Application Bootstrap**
```bash
# System starts with environment configuration
export DATA_PROVIDER="epex"
export PROVIDER_URL="https://www.epexspot.com/en/market-results"
export PROVIDER_PARAMS='{"market_area":"FR","auction":"IDA1"}'
./powercap
```

**What happens:**
- Logger initializes with microsecond timestamps
- System checks for test mode (`test-data` argument)
- Context created for graceful shutdown handling
- Configuration loaded from environment variables

### 2. **Configuration Loading**
```go
// internal/config/config.go
type Config struct {
    DataProvider    string  // "epex", "mock", "static"
    ProviderURL     string  // Market data URL
    ProviderParams  string  // JSON parameters
    MaxSource       int64   // Maximum power in µW
    NodeName        string  // Kubernetes node name
}
```

**Environment Variables Parsed:**
- `DATA_PROVIDER`: Selects which data source to use
- `PROVIDER_URL`: API endpoint for market data
- `PROVIDER_PARAMS`: JSON configuration for provider
- `MAX_SOURCE`: Maximum CPU power limit (default: 40W = 40,000,000 µW)
- `STABILISATION_TIME`: Update interval in seconds

### 3. **Provider Factory Initialization**
```go
// pkg/providers/factory.go
factory := providers.NewProviderFactory()
provider, err := factory.CreateProvider(cfg)
```

**Available Providers:**
- **EPEX Provider**: Real market data from European Power Exchange
- **Mock Provider**: Synthetic data for development/testing
- **Static Provider**: Predefined datasets for demos

## 📊 Data Acquisition Process

### 4. **Market Data Fetching (EPEX Example)**
```go
// Real-time data fetch from EPEX SPOT
data, err := provider.FetchData(ctx, time.Now())
```

**EPEX Provider Operation:**
1. **HTTP Request** to `https://www.epexspot.com/en/market-results`
2. **Parameters sent:**
   ```json
   {
     "market_area": "FR",
     "auction": "IDA1", 
     "modality": "Auction",
     "sub_modality": "Intraday",
     "data_mode": "table"
   }
   ```
3. **HTML Parsing** of market results table
4. **Data Extraction** into structured format

**Sample Data Retrieved:**
```
Period          Volume (MWh)    Price (€/MWh)
00:00-00:15     66.3           31.91
00:15-00:30     65.3           29.39
00:30-00:45     64.0           21.13
01:00-01:15     90.5           20.32
01:15-01:30     91.8           14.87
```

### 5. **Data Storage & CSV Generation**
```go
// internal/datastore/csv_store.go
ds := datastore.NewCSVDataStore(logger)
err := ds.SaveData(today, marketData)
```

**File Generated:** `epex_data_2025-10-06.csv`
```csv
Period,Volume (MWh),Price (€/MWh)
00:00-00:15,66.3,31.91
00:15-00:30,65.3,29.39
00:30-00:45,64.0,21.13
...
```

## ⚡ Power Calculation Engine

### 6. **Rule of Three Power Calculation**
```go
// Core calculation algorithm
func CalculatePower(maxSource float64, currentVolume float64, maxVolume float64) int64 {
    // Rule of three: if MaxVolume → MaxPower, then CurrentVolume → ?
    power := (currentVolume / maxVolume) * maxSource
    return int64(power) // Result in microWatts
}
```

**Real Example from Market Data:**
- **Max Volume Found**: 93.8 MWh (highest in dataset)
- **Max Power Configured**: 40,000,000 µW (40W)
- **Current Period**: 01:15-01:30 with 91.8 MWh

**Calculation:**
```
Power = (91.8 MWh / 93.8 MWh) × 40,000,000 µW
Power = 0.9787 × 40,000,000 µW
Power = 39,148,000 µW = 39.1W
```

### 7. **Time-Based Power Application**
```go
// Get current 15-minute period
currentPeriod := GetCurrentPeriod(time.Now())
// Example: if time is 13:22, period is "13:15-13:30"

// Find matching market data
for _, point := range marketData {
    if point.Period == currentPeriod {
        newPower := CalculatePower(maxSource, point.Volume, maxVolume)
        raplManager.SetPowerLimit(newPower)
        break
    }
}
```

## 🔧 Hardware Interface (RAPL)

### 8. **RAPL Power Control**
```go
// internal/rapl/manager.go
func (r *RAPLManager) SetPowerLimit(powerMicroWatts int64) error {
    // Convert µW to RAPL format
    raplValue := powerMicroWatts / 1000000 * 1000000 // Align to µW precision
    
    // Write to hardware interface
    err := writeToFile("/sys/class/powercap/intel-rapl:0/constraint_0_power_limit_uw", 
                       fmt.Sprintf("%d", raplValue))
    
    // Update constraint time window
    err = writeToFile("/sys/class/powercap/intel-rapl:0/constraint_0_time_window_us", 
                      "976562") // ~1 second window
    
    return err
}
```

**Hardware Path Examples:**
- Power Limit: `/sys/class/powercap/intel-rapl:0/constraint_0_power_limit_uw`
- Time Window: `/sys/class/powercap/intel-rapl:0/constraint_0_time_window_us`
- Current Power: `/sys/class/powercap/intel-rapl:0/energy_uj`

## 🔄 Main Operation Loop

### 9. **Continuous Monitoring Cycle**
```go
// internal/power/manager.go
func (pm *Manager) Run() {
    ticker := time.NewTicker(time.Duration(pm.config.StabilisationTime) * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            pm.updatePowerConsumption()
        case <-pm.ctx.Done():
            return // Graceful shutdown
        }
    }
}
```

**Update Cycle (every 5 minutes default):**
1. **Check Current Time** → Determine 15-minute market period
2. **Load Market Data** → From CSV or fetch new data if needed
3. **Calculate Power** → Apply rule of three for current period
4. **Update RAPL** → Set new CPU power limit
5. **Update Kubernetes** → Set node annotation with new power value
6. **Log Activity** → Record power change with timestamp

## 🔍 Kubernetes Integration

### 10. **Node Annotation Updates**
```go
// Update Kubernetes node with current power info
func (pm *Manager) updateNodeAnnotations(powerMicroWatts int64) error {
    node, err := pm.clientset.CoreV1().Nodes().Get(ctx, pm.config.NodeName, metav1.GetOptions{})
    
    // Set RAPL power annotations
    if node.Annotations == nil {
        node.Annotations = make(map[string]string)
    }
    node.Annotations["rapl/pmax"] = fmt.Sprintf("%d", powerMicroWatts)
    node.Annotations["rapl/last-update"] = time.Now().Format(time.RFC3339)
    node.Annotations["rapl/provider"] = pm.config.DataProvider
    
    _, err = pm.clientset.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
    return err
}
```

**Kubernetes Annotations Set:**
```yaml
metadata:
  annotations:
    rapl/pmax: "39148000"                    # Current power limit in µW
    rapl/last-update: "2025-10-06T13:22:45Z" # Last update timestamp
    rapl/provider: "epex"                     # Active data provider
    power-manager/initialized: "true"         # System status
```

## 📈 Real-World Operation Example

### **Scenario: Monday 13:22 on October 6, 2025**

1. **Market Period**: 13:15-13:30
2. **EPEX Data**: Volume = 85.2 MWh, Price = 45.67 €/MWh
3. **Max Volume**: 93.8 MWh (from daily dataset)
4. **Power Calculation**: (85.2/93.8) × 40W = 36.3W
5. **RAPL Update**: Set CPU limit to 36,300,000 µW
6. **Kubernetes**: Node annotation `rapl/pmax="36300000"`
7. **Logging**: `[13:22:45] Updated power to 36.3W based on market period 13:15-13:30`

### **Daily Power Profile Example**
```
Time        Volume(MWh)  Price(€/MWh)  Power(W)  CPU Impact
06:00-06:15    72.1       18.90        30.7W    Normal load
06:15-06:30    74.6       19.93        31.9W    Slight increase  
12:00-12:15    88.3       42.15        37.7W    High performance
12:15-12:30    89.1       43.22        38.1W    Peak performance
18:00-18:15    91.8       55.30        39.2W    Maximum performance
02:00-02:15    64.0       12.30        27.3W    Power saving mode
```

## 🛡️ Error Handling & Resilience

### **Fallback Mechanisms:**
1. **Network Failure**: Use yesterday's cached data
2. **Invalid Data**: Apply safe default power limits
3. **RAPL Access Denied**: Log error, continue monitoring
4. **Kubernetes Unavailable**: Continue local power management

### **Validation Checks:**
- Power values within safe ranges (10W - 50W)
- Market data freshness (< 24 hours old)
- RAPL interface accessibility
- Configuration parameter validation

## 🧪 Testing Modes

### **Test Mode - Data Fetch Only**
```bash
./powercap test-data
# Output: Shows market data retrieval without power changes
```

### **Test Mode - Full Simulation**
```bash
./powercap test-data full  
# Output: Complete flow with power calculations and CSV generation
```

### **Real Test Output Example:**
```
[PowerManager] 2025/10/06 11:06:59 Starting professional power management system...
[PowerManager] 2025/10/06 11:06:59 Running full power calculation and CSV generation test...
[PowerManager] 2025/10/06 11:06:59 Fetching data from epex provider...
[PowerManager] 2025/10/06 11:06:59 Successfully fetched 96 data points
[PowerManager] 2025/10/06 11:06:59 Calculating power consumption for each time period...
[PowerManager] 2025/10/06 11:06:59 Period 00:00-00:15: Price=31.91 €/MWh, Volume=66.3 MWh → Power=22589437 µW (22.6 W)
[PowerManager] 2025/10/06 11:06:59 Period 00:15-00:30: Price=29.39 €/MWh, Volume=65.3 MWh → Power=22248722 µW (22.2 W)
[PowerManager] 2025/10/06 11:06:59 Period 01:15-01:30: Price=14.87 €/MWh, Volume=91.8 MWh → Power=31277683 µW (31.3 W)
[PowerManager] 2025/10/06 11:06:59 ✅ Full test completed successfully!
[PowerManager] 2025/10/06 11:06:59    - Fetched: 96 data points
[PowerManager] 2025/10/06 11:06:59    - Generated: epex_data_2025-10-06.csv
```

## 🏗️ Kubernetes Deployment

### **ConfigMap Example:**
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: powercap-config-epex-fr
data:
  DATA_PROVIDER: "epex"
  PROVIDER_URL: "https://www.epexspot.com/en/market-results"
  PROVIDER_PARAMS: |
    {
      "market_area": "FR",
      "auction": "IDA1",
      "modality": "Auction",
      "sub_modality": "Intraday"
    }
  MAX_SOURCE: "40000000"
  STABILISATION_TIME: "300"
```

### **DaemonSet Example:**
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: powercap-manager
spec:
  selector:
    matchLabels:
      app: powercap-manager
  template:
    spec:
      serviceAccountName: powercap-manager
      containers:
      - name: powercap-manager
        image: powercap-manager:latest
        securityContext:
          privileged: true  # Required for RAPL access
        envFrom:
        - configMapRef:
            name: powercap-config-epex-fr
        volumeMounts:
        - name: sys
          mountPath: /sys
          readOnly: false
      volumes:
      - name: sys
        hostPath:
          path: /sys
```

## 🔧 Configuration Management

### **Switching Providers via Kubernetes:**
```bash
# Switch to Mock provider for testing
kubectl patch daemonset powercap-manager -p '{
  "spec": {
    "template": {
      "spec": {
        "containers": [{
          "name": "powercap-manager", 
          "envFrom": [{"configMapRef": {"name": "powercap-config-mock"}}]
        }]
      }
    }
  }
}'

# Switch to German market
kubectl create configmap powercap-config-epex-de \
  --from-literal=DATA_PROVIDER=epex \
  --from-literal=PROVIDER_PARAMS='{"market_area":"DE","auction":"IDA1"}'
```

### **Multi-Environment Support:**
```bash
# Production
kubectl apply -k k8s/overlays/production

# Staging with mock data
kubectl apply -k k8s/overlays/staging

# Development with static data
kubectl apply -k k8s/overlays/development
```

## 📊 Monitoring & Observability

### **Kubernetes Annotations for Monitoring:**
```yaml
# Node annotations updated every cycle
metadata:
  annotations:
    rapl/pmax: "36300000"                    # Current power limit (µW)
    rapl/last-update: "2025-10-06T13:22:45Z" # Last update timestamp
    rapl/provider: "epex"                     # Active data provider
    rapl/market-area: "FR"                    # Market area
    rapl/volume-mwh: "85.2"                   # Current market volume
    rapl/price-eur-mwh: "45.67"               # Current market price
    power-manager/initialized: "true"         # System status
    power-manager/version: "v1.0.0"           # Manager version
```

### **Prometheus Metrics (if integrated):**
```
powercap_current_limit_watts 36.3
powercap_market_volume_mwh 85.2
powercap_market_price_eur_mwh 45.67
powercap_last_update_timestamp 1696593765
```

## 🧹 Cleanup & Maintenance

### **Complete System Cleanup**
```bash
# Interactive cleanup with backups
./scripts/cleanup.sh

# Dry-run to see what would be cleaned
./scripts/cleanup.sh --dry-run

# Force cleanup without confirmation
./scripts/cleanup.sh --force

# Clean only Kubernetes resources
./scripts/cleanup.sh --kubernetes-only

# Clean only local files (no K8s)
./scripts/cleanup.sh --local-only
```

### **Cleanup Categories**
The cleanup script handles:

- **Kubernetes Resources**: DaemonSets, ConfigMaps, RBAC, node annotations
- **Local Data**: CSV files, temporary files, logs
- **RAPL Settings**: Reset power limits to defaults
- **Docker Resources**: Images and containers (optional)

### **Backup & Restore**
```bash
# Cleanup creates automatic backups
ls cleanup-backups/20251006-143022/

# Restore from backup
./cleanup-backups/20251006-143022/restore.sh
```

This architecture provides a robust, configurable, and Kubernetes-native power management solution that dynamically adjusts CPU performance based on real-time electricity market conditions, with comprehensive testing, monitoring, and operational capabilities.