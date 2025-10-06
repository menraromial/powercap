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
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	clientset, err := createKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	raplMgr := rapl.NewManager(logger)
	if err := raplMgr.DiscoverDomains(); err != nil {
		return nil, fmt.Errorf("failed to discover RAPL domains: %w", err)
	}

	// Initialize data store and calculator
	dataStore := datastore.NewCSVDataStore(logger)
	calculator := datastore.NewMarketBasedCalculator()

	// Create and configure provider using factory
	factory := providers.NewProviderFactory()
	if err := factory.ValidateProviderConfig(cfg); err != nil {
		return nil, fmt.Errorf("invalid provider configuration: %w", err)
	}

	provider, err := factory.CreateProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	dataStore.SetProvider(provider)
	logger.Printf("Configured data provider: %s", provider.GetName())

	logger.Printf("Discovered %d RAPL domains", len(raplMgr.GetDomains()))

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
	data, err := pm.dataStore.LoadData(date)
	if err != nil {
		return fmt.Errorf("failed to load market data: %w", err)
	}

	pm.logger.Printf("Loaded %d market data points for %s", len(data), date.Format("2006-01-02"))
	return nil
}

// InitializeNode initializes the Kubernetes node with RAPL information
func (pm *Manager) InitializeNode() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Check if the node is already initialized
	if pm.isNodeInitialized(node) {
		pm.logger.Println("Node already initialized, skipping initialization")
		return nil
	}

	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}

	// Find the maximum power value across all domains and constraints
	maxPower, err := pm.raplMgr.FindMaxPowerValue()
	if err != nil {
		return fmt.Errorf("failed to find max power value: %w", err)
	}

	// Store a single value for the node
	maxPowerValue := strconv.FormatInt(maxPower, 10)
	node.Annotations["rapl/max_power_uw"] = maxPowerValue
	node.Annotations["rapl/pmax"] = maxPowerValue
	node.Annotations["rapl/provider"] = pm.config.DataProvider

	// Mark the node as initialized
	if err := pm.markNodeAsInitialized(node); err != nil {
		return fmt.Errorf("failed to mark node as initialized: %w", err)
	}

	pm.logger.Printf("Node initialized with max power: %s µW", maxPowerValue)
	return nil
}

// AdjustPowerCap adjusts the power cap based on current market data
func (pm *Manager) AdjustPowerCap() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Calculate source power using market data
	currentTime := time.Now()
	data := pm.dataStore.GetCurrentData()

	sourcePower := pm.calculator.CalculatePower(pm.config.MaxSource, currentTime, data)
	if sourcePower == 0 {
		currentPeriod := pm.calculator.GetCurrentPeriod(currentTime)
		pm.logger.Printf("No market data found for period %s, using minimum power", currentPeriod)
		sourcePower = pm.config.RaplLimit
	}

	maxPower, err := pm.getMaxPowerValue(node)
	if err != nil {
		return fmt.Errorf("failed to get max power value: %w", err)
	}

	// Determine the power limit to apply
	var pmax int64 = pm.config.RaplLimit

	if sourcePower > maxPower {
		pmax = maxPower
	} else if sourcePower > pm.config.RaplLimit {
		pmax = sourcePower
	}

	// Log the calculation details
	currentPeriod := pm.calculator.GetCurrentPeriod(currentTime)
	pm.logger.Printf("Power calculation: period=%s, source=%d µW, max=%d µW, min=%d µW, applied=%d µW",
		currentPeriod, sourcePower, maxPower, pm.config.RaplLimit, pmax)

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
