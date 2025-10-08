package power

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kcas/new/internal/config"
	"kcas/new/internal/datastore"
	"kcas/new/internal/rapl"
	"kcas/new/pkg/providers"
)

const (
	InitializationAnnotation = "power-manager/initialized"
)

// Manager handles power management operations
type Manager struct {
	clientset  *kubernetes.Clientset
	config     *config.Config
	logger     *log.Logger
	raplMgr    *rapl.Manager
	dataStore  datastore.DataStore
	calculator datastore.PowerCalculator
	ctx        context.Context
}

// NewManager creates and initializes a new power Manager
func NewManager(ctx context.Context, logger *log.Logger) (*Manager, error) {
	logger.Println("🚀 Initializing PowerCap Manager...")

	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	logger.Printf("✅ Configuration loaded successfully")
	logger.Printf("   - Node Name: %s", cfg.NodeName)
	logger.Printf("   - Data Provider: %s", cfg.DataProvider)
	logger.Printf("   - Provider URL: %s", cfg.ProviderURL)
	logger.Printf("   - Stabilisation Time: %v", cfg.StabilisationTime)
	logger.Printf("   - RAPL Min Power: %d µW (%.1f W)", cfg.RaplLimit, float64(cfg.RaplLimit)/1000000)

	logger.Println("🔌 Creating Kubernetes client...")
	clientset, err := createKubernetesClient()
	if err != nil {
		logger.Printf("❌ Failed to create Kubernetes client: %v", err)
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	logger.Printf("✅ Kubernetes client created successfully")

	logger.Println("⚡ Discovering RAPL domains...")
	raplMgr := rapl.NewManager(logger)
	if err := raplMgr.DiscoverDomains(); err != nil {
		logger.Printf("❌ Failed to discover RAPL domains: %v", err)
		return nil, fmt.Errorf("failed to discover RAPL domains: %w", err)
	}
	logger.Printf("✅ Discovered %d RAPL domains", len(raplMgr.GetDomains()))

	// Initialize data store and calculator
	logger.Println("📊 Initializing data store and calculator...")
	dataStore := datastore.NewCSVDataStore(logger)
	calculator := datastore.NewMarketBasedCalculator()

	// Create and configure provider using factory
	logger.Println("🏭 Setting up market data provider...")
	factory := providers.NewProviderFactory()
	if err := factory.ValidateProviderConfig(cfg); err != nil {
		logger.Printf("❌ Invalid provider configuration: %v", err)
		return nil, fmt.Errorf("invalid provider configuration: %w", err)
	}

	provider, err := factory.CreateProvider(cfg)
	if err != nil {
		logger.Printf("❌ Failed to create provider: %v", err)
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	dataStore.SetProvider(provider)
	logger.Printf("✅ Configured data provider: %s", provider.GetName())

	logger.Printf("✅ PowerCap Manager initialized successfully with %d RAPL domains", len(raplMgr.GetDomains()))

	return &Manager{
		clientset:  clientset,
		config:     cfg,
		logger:     logger,
		raplMgr:    raplMgr,
		dataStore:  dataStore,
		calculator: calculator,
		ctx:        ctx,
	}, nil
}

// SetDataProvider sets the market data provider (deprecated - use config instead)
func (pm *Manager) SetDataProvider(provider datastore.MarketDataProvider) {
	pm.logger.Printf("Warning: SetDataProvider is deprecated. Use configuration instead.")
	pm.dataStore.SetProvider(provider)
}

// LoadData loads market data for the given date
func (pm *Manager) LoadData(date time.Time) error {
	pm.logger.Printf("📥 Loading market data for %s...", date.Format("2006-01-02"))

	data, err := pm.dataStore.LoadData(date)
	if err != nil {
		pm.logger.Printf("❌ Failed to load market data for %s: %v", date.Format("2006-01-02"), err)
		return fmt.Errorf("failed to load market data: %w", err)
	}

	pm.logger.Printf("✅ Successfully loaded %d market data points for %s", len(data), date.Format("2006-01-02"))

	// Log sample data for debugging
	if len(data) > 0 {
		pm.logger.Printf("   📊 Sample data points:")
		sampleCount := 3
		if len(data) < sampleCount {
			sampleCount = len(data)
		}
		for i := 0; i < sampleCount; i++ {
			pm.logger.Printf("      %s: %.1f MWh @ %.2f €/MWh",
				data[i].Period, data[i].Volume, data[i].Price)
		}
		if len(data) > sampleCount {
			pm.logger.Printf("      ... and %d more data points", len(data)-sampleCount)
		}
	}

	return nil
}

// InitializeNode initializes the Kubernetes node with RAPL information
func (pm *Manager) InitializeNode() error {
	pm.logger.Printf("🔧 Initializing Kubernetes node '%s'...", pm.config.NodeName)

	node, err := pm.getNode()
	if err != nil {
		pm.logger.Printf("❌ Failed to get node '%s': %v", pm.config.NodeName, err)
		return fmt.Errorf("failed to get node: %w", err)
	}
	pm.logger.Printf("✅ Successfully retrieved node '%s'", node.Name)

	// Check if the node is already initialized
	if pm.isNodeInitialized(node) {
		pm.logger.Printf("ℹ️  Node '%s' already initialized, skipping initialization", node.Name)
		return nil
	}

	pm.logger.Printf("🚀 Node '%s' not initialized, proceeding with initialization...", node.Name)

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
		pm.logger.Printf("📝 Created new annotations map for node '%s'", node.Name)
	}

	// Find the maximum power value across all domains and constraints
	pm.logger.Printf("⚡ Finding maximum power value from RAPL domains...")
	maxPower, err := pm.raplMgr.FindMaxPowerValue()
	if err != nil {
		pm.logger.Printf("❌ Failed to find max power value: %v", err)
		return fmt.Errorf("failed to find max power value: %w", err)
	}
	pm.logger.Printf("✅ Found maximum power value: %d µW (%.1f W)", maxPower, float64(maxPower)/1000000)

	// Store a single value for the node
	maxPowerValue := strconv.FormatInt(maxPower, 10)
	pm.logger.Printf("📝 Setting node annotations...")
	node.Annotations["rapl/max_power_uw"] = maxPowerValue
	node.Annotations["rapl/pmax"] = maxPowerValue
	node.Annotations["rapl/provider"] = pm.config.DataProvider
	pm.logger.Printf("   - rapl/max_power_uw: %s", maxPowerValue)
	pm.logger.Printf("   - rapl/pmax: %s", maxPowerValue)
	pm.logger.Printf("   - rapl/provider: %s", pm.config.DataProvider)

	// Mark the node as initialized
	pm.logger.Printf("🏷️  Marking node as initialized...")
	if err := pm.markNodeAsInitialized(node); err != nil {
		pm.logger.Printf("❌ Failed to mark node as initialized: %v", err)
		return fmt.Errorf("failed to mark node as initialized: %w", err)
	}

	pm.logger.Printf("✅ Node '%s' initialized successfully with max power: %s µW (%.1f W)",
		node.Name, maxPowerValue, float64(maxPower)/1000000)
	return nil
}

// AdjustPowerCap adjusts the power cap based on current market data
func (pm *Manager) AdjustPowerCap() error {
	pm.logger.Printf("🔄 Starting power cap adjustment cycle...")

	node, err := pm.getNode()
	if err != nil {
		pm.logger.Printf("❌ Failed to get node: %v", err)
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Calculate source power using market data
	currentTime := time.Now()
	currentPeriod := pm.calculator.GetCurrentPeriod(currentTime)
	pm.logger.Printf("⏰ Current time: %s (period: %s)", currentTime.Format("15:04:05"), currentPeriod)

	data := pm.dataStore.GetCurrentData()
	maxVolume := pm.dataStore.GetMaxVolume()
	pm.logger.Printf("📊 Market data: %d points available, max volume: %.1f MWh", len(data), maxVolume)

	// Get the maximum hardware power limit from RAPL
	pm.logger.Printf("⚡ Retrieving RAPL max power...")
	maxPower, err := pm.getMaxPowerValue(node)
	if err != nil {
		pm.logger.Printf("❌ Failed to get max power value: %v", err)
		return fmt.Errorf("failed to get max power value: %w", err)
	}
	pm.logger.Printf("✅ RAPL max power: %d µW (%.1f W)", maxPower, float64(maxPower)/1000000)

	// Use RAPL max power as the reference for rule of three calculation
	pm.logger.Printf("🧮 Calculating source power using market data...")
	sourcePower := pm.calculator.CalculatePower(float64(maxPower), maxVolume, currentTime, data)

	if sourcePower == 0 {
		pm.logger.Printf("⚠️  No market data found for period %s, using minimum power fallback", currentPeriod)
		sourcePower = pm.config.RaplLimit
		pm.logger.Printf("   Fallback source power: %d µW (%.1f W)", sourcePower, float64(sourcePower)/1000000)
	} else {
		pm.logger.Printf("✅ Calculated source power: %d µW (%.1f W)", sourcePower, float64(sourcePower)/1000000)
	}

	// Determine the power limit to apply
	pm.logger.Printf("🎯 Determining final power limit to apply...")
	var pmax int64 = pm.config.RaplLimit
	pm.logger.Printf("   Starting with minimum: %d µW (%.1f W)", pmax, float64(pmax)/1000000)

	if sourcePower > maxPower {
		pmax = maxPower
		pm.logger.Printf("   ⬆️  Source power exceeds max hardware limit")
		pm.logger.Printf("   🔒 Capped to hardware max: %d µW (%.1f W)", pmax, float64(pmax)/1000000)
	} else if sourcePower > pm.config.RaplLimit {
		pmax = sourcePower
		pm.logger.Printf("   ✅ Using calculated source power: %d µW (%.1f W)", pmax, float64(pmax)/1000000)
	} else {
		pm.logger.Printf("   ⬇️  Source power below minimum threshold")
		pm.logger.Printf("   🔒 Using minimum limit: %d µW (%.1f W)", pmax, float64(pmax)/1000000)
	}

	// Log the calculation details
	pm.logger.Printf("📋 Power calculation summary:")
	pm.logger.Printf("   - Period: %s", currentPeriod)
	pm.logger.Printf("   - Source Power: %d µW (%.1f W)", sourcePower, float64(sourcePower)/1000000)
	pm.logger.Printf("   - Max Hardware: %d µW (%.1f W)", maxPower, float64(maxPower)/1000000)
	pm.logger.Printf("   - Min Threshold: %d µW (%.1f W)", pm.config.RaplLimit, float64(pm.config.RaplLimit)/1000000)
	pm.logger.Printf("   - Applied Limit: %d µW (%.1f W)", pmax, float64(pmax)/1000000)

	pm.logger.Printf("⚡ Applying power limits to RAPL domains...")
	return pm.applyPowerLimits(node, pmax)
}

// Run starts the power management cycle
func (pm *Manager) Run() {
	pm.logger.Println("Starting power management cycle...")

	ticker := time.NewTicker(pm.config.StabilisationTime)
	defer ticker.Stop()

	// Schedule daily data refresh at midnight
	dailyTicker := pm.scheduleDailyDataRefresh()
	defer dailyTicker.Stop()

	// Do an initial adjustment
	if err := pm.AdjustPowerCap(); err != nil {
		pm.logger.Printf("Initial power cap adjustment failed: %v", err)
	}

	// Main event loop
	for {
		select {
		case <-ticker.C:
			if err := pm.AdjustPowerCap(); err != nil {
				pm.logger.Printf("Failed to adjust power cap: %v", err)
			}
		case <-pm.ctx.Done():
			pm.logger.Println("Power manager shutting down...")
			return
		}
	}
}

// RefreshData manually refreshes market data
func (pm *Manager) RefreshData(date time.Time) error {
	return pm.dataStore.RefreshData(context.Background(), date)
}

// scheduleDailyDataRefresh sets up automatic data refresh at midnight
func (pm *Manager) scheduleDailyDataRefresh() *time.Ticker {
	now := time.Now()
	nextMidnight := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
	timeUntilMidnight := nextMidnight.Sub(now)

	pm.logger.Printf("Next data refresh scheduled in %v (at %v)",
		timeUntilMidnight, nextMidnight.Format("2006-01-02 15:04:05"))

	ticker := time.NewTicker(24 * time.Hour)

	go func() {
		time.Sleep(timeUntilMidnight)
		pm.logger.Println("Midnight reached - triggering data refresh...")

		today := time.Now()
		if err := pm.dataStore.RefreshData(context.Background(), today); err != nil {
			pm.logger.Printf("Failed to refresh data at midnight: %v", err)
		} else {
			pm.logger.Println("Midnight data refresh completed successfully")
		}
	}()

	return ticker
}

// Helper methods

func (pm *Manager) getNode() (*v1.Node, error) {
	return pm.clientset.CoreV1().Nodes().Get(pm.ctx, pm.config.NodeName, metav1.GetOptions{})
}

func (pm *Manager) updateNode(node *v1.Node) error {
	_, err := pm.clientset.CoreV1().Nodes().Update(pm.ctx, node, metav1.UpdateOptions{})
	return err
}

func (pm *Manager) isNodeInitialized(node *v1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	_, exists := node.Annotations[InitializationAnnotation]
	return exists
}

func (pm *Manager) markNodeAsInitialized(node *v1.Node) error {
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[InitializationAnnotation] = "kcas-power-manager"
	return pm.updateNode(node)
}

func (pm *Manager) getMaxPowerValue(node *v1.Node) (int64, error) {
	if node.Annotations == nil {
		return 0, errors.New("node has no annotations")
	}

	annotation := "rapl/max_power_uw"
	value, ok := node.Annotations[annotation]
	if !ok {
		return 0, fmt.Errorf("max power annotation not found: %s", annotation)
	}

	maxPower, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid max power value: %w", err)
	}

	return maxPower, nil
}

func (pm *Manager) applyPowerLimits(node *v1.Node, pmax int64) error {
	// Update node annotations with detailed power information
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	// Core power information
	node.Annotations["rapl/pmax"] = strconv.FormatInt(pmax, 10)
	node.Annotations["rapl/last-update"] = time.Now().Format(time.RFC3339)
	node.Annotations["rapl/provider"] = pm.config.DataProvider

	// Get current market data for additional context
	data := pm.dataStore.GetCurrentData()
	if len(data) > 0 {
		currentTime := time.Now()
		currentPeriod := pm.calculator.GetCurrentPeriod(currentTime)

		// Find current period data
		for _, point := range data {
			if point.Period == currentPeriod {
				node.Annotations["rapl/market-period"] = currentPeriod
				node.Annotations["rapl/market-volume"] = fmt.Sprintf("%.1f", point.Volume)
				node.Annotations["rapl/market-price"] = fmt.Sprintf("%.2f", point.Price)
				break
			}
		}
	}

	// Apply this limit to all power_limit_uw files in all domains
	if errs := pm.raplMgr.ApplyPowerLimits(pmax); len(errs) > 0 {
		var errStrs []string
		for _, err := range errs {
			errStrs = append(errStrs, err.Error())
		}
		pm.logger.Printf("Errors applying power limits: %s", strings.Join(errStrs, "; "))
	}

	return pm.updateNode(node)
}

func createKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to get in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return clientset, nil
}
