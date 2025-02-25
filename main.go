package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Configuration constants
const (
	raplBasePath = "/sys/devices/virtual/powercap/intel-rapl"

	// Environment variable names
	envNodeName          = "NODE_NAME"
	envMaxSource         = "MAX_SOURCE"
	envStabilisationTime = "STABILISATION_TIME"
	envAlpha             = "ALPHA"
	envRaplLimit         = "RAPL_MIN_POWER"

	// Default values
	defaultMaxSource         = "40000000"
	defaultStabilisationTime = "300"
	defaultAlpha             = "4"
	defaultRaplLimit         = "10000000"

	initializationAnnotation = "power-manager/initialized"
)

// Configuration types
type (
	// RaplDomain represents a RAPL domain with its constraints
	RaplDomain struct {
		ID             string // e.g., "intel-rapl:0"
		Constraints    []PowerConstraint
		ConstraintsMax []PowerConstraint
	}

	// PowerConstraint represents a RAPL power constraint configuration
	PowerConstraint struct {
		ID    int    // constraint number (0, 1, etc.)
		Path  string // full path to the constraint file
		Value string // current power limit value
	}

	// Config holds the application configuration
	Config struct {
		MaxSource         float64
		Alpha             float64
		StabilisationTime time.Duration
		RaplLimit         int64
		NodeName          string
	}

	// PowerManager handles power management operations
	PowerManager struct {
		clientset   *kubernetes.Clientset
		config      *Config
		logger      *log.Logger
		raplDomains []RaplDomain
		ctx         context.Context
	}
)

// NewPowerManager creates and initializes a new PowerManager
func NewPowerManager(ctx context.Context, logger *log.Logger) (*PowerManager, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	clientset, err := createKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	domains, err := discoverRaplDomains(logger)
	if err != nil {
		return nil, fmt.Errorf("failed to discover RAPL domains: %w", err)
	}

	logger.Printf("Discovered %d RAPL domains", len(domains))

	return &PowerManager{
		clientset:   clientset,
		config:      config,
		logger:      logger,
		raplDomains: domains,
		ctx:         ctx,
	}, nil
}

// discoverRaplDomains finds all RAPL domains and their constraints in the system
func discoverRaplDomains(logger *log.Logger) ([]RaplDomain, error) {
	var domains []RaplDomain

	// List all RAPL domains
	entries, err := os.ReadDir(raplBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read RAPL base path: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "intel-rapl:") {
			continue
		}

		domain := RaplDomain{
			ID: entry.Name(),
		}

		// Read only direct constraint files in this domain
		domainPath := filepath.Join(raplBasePath, entry.Name())
		constraintEntries, err := os.ReadDir(domainPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read domain directory %s: %w", domainPath, err)
		}

		for _, constEntry := range constraintEntries {
			name := constEntry.Name()
			if constEntry.IsDir() {
				continue // Skip directories
			}

			// Process only constraint files
			if !strings.HasPrefix(name, "constraint_") {
				continue
			}

			// Extract constraint number from filename
			constraintNumStr := strings.Split(name, "_")[1]
			constraintNum, err := strconv.Atoi(constraintNumStr)
			if err != nil {
				logger.Printf("Warning: Invalid constraint number in %s: %v", name, err)
				continue
			}

			path := filepath.Join(domainPath, name)

			// Process max power constraints
			if strings.HasSuffix(name, "_max_power_uw") {
				value, err := readPowerLimit(path)
				if err != nil {
					logger.Printf("Warning: Failed to read max power at %s: %v", path, err)
					value = "0"
				}
				domain.ConstraintsMax = append(domain.ConstraintsMax, PowerConstraint{
					ID:    constraintNum,
					Path:  path,
					Value: value,
				})
			}

			// Process power limit constraints
			if strings.HasSuffix(name, "_power_limit_uw") {
				value, err := readPowerLimit(path)
				if err != nil {
					logger.Printf("Warning: Failed to read power limit at %s: %v", path, err)
					value = "0"
				}
				domain.Constraints = append(domain.Constraints, PowerConstraint{
					ID:    constraintNum,
					Path:  path,
					Value: value,
				})
			}
		}

		// Only add domains that have constraints
		if len(domain.Constraints) > 0 || len(domain.ConstraintsMax) > 0 {
			domains = append(domains, domain)
		}
	}

	return domains, nil
}

// Helper functions for configuration and setup
func loadConfig() (*Config, error) {
	nodeName := os.Getenv(envNodeName)
	if nodeName == "" {
		return nil, fmt.Errorf("%s environment variable is not set", envNodeName)
	}

	maxSource, err := strconv.ParseFloat(getEnvOrDefault(envMaxSource, defaultMaxSource), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid max source value: %w", err)
	}

	alpha, err := strconv.ParseFloat(getEnvOrDefault(envAlpha, defaultAlpha), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid alpha value: %w", err)
	}

	stabilisationTime, err := time.ParseDuration(getEnvOrDefault(envStabilisationTime, defaultStabilisationTime) + "s")
	if err != nil {
		return nil, fmt.Errorf("invalid stabilisation time: %w", err)
	}

	raplLimit, err := strconv.ParseInt(getEnvOrDefault(envRaplLimit, defaultRaplLimit), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RAPL limit: %w", err)
	}

	return &Config{
		MaxSource:         maxSource,
		Alpha:             alpha,
		StabilisationTime: stabilisationTime,
		RaplLimit:         raplLimit,
		NodeName:          nodeName,
	}, nil
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

func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists && value != "" {
		return value
	}
	return defaultValue
}

func readPowerLimit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// Node management methods
func (pm *PowerManager) getNode() (*v1.Node, error) {
	return pm.clientset.CoreV1().Nodes().Get(pm.ctx, pm.config.NodeName, metav1.GetOptions{})
}

func (pm *PowerManager) updateNode(node *v1.Node) error {
	_, err := pm.clientset.CoreV1().Nodes().Update(pm.ctx, node, metav1.UpdateOptions{})
	return err
}

func (pm *PowerManager) isNodeInitialized(node *v1.Node) bool {
	if node.Annotations == nil {
		return false
	}
	_, exists := node.Annotations[initializationAnnotation]
	return exists
}

func (pm *PowerManager) markNodeAsInitialized(node *v1.Node) error {
	if node.Annotations == nil {
		node.Annotations = make(map[string]string)
	}
	node.Annotations[initializationAnnotation] = "kcas-power-manager"
	return pm.updateNode(node)
}

// Power management methods
func (pm *PowerManager) findMaxPowerValue() (int64, error) {
	var maxPower int64

	for _, domain := range pm.raplDomains {
		for _, constraint := range domain.ConstraintsMax {
			value, err := strconv.ParseInt(constraint.Value, 10, 64)
			if err != nil {
				continue // Skip invalid values instead of failing
			}

			if value > maxPower {
				maxPower = value
			}
		}
	}

	if maxPower == 0 {
		return 0, errors.New("no valid max power values found")
	}

	return maxPower, nil
}

func (pm *PowerManager) calculateSourcePower() int64 {
	currentTime := time.Now()
	t := float64(currentTime.Hour()) + float64(currentTime.Minute())/60.0
	power := pm.config.MaxSource * math.Pow(math.Sin((math.Pi/16)*(t-4)), pm.config.Alpha)

	if power < 0 {
		return 0
	}

	return int64(math.Round(power))
}

func (pm *PowerManager) getMaxPowerValue(node *v1.Node) (int64, error) {
	if node.Labels == nil {
		return 0, errors.New("node has no labels")
	}

	label := "rapl/max_power_uw"
	value, ok := node.Labels[label]
	if !ok {
		return 0, fmt.Errorf("max power label not found: %s", label)
	}

	maxPower, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid max power value: %w", err)
	}

	return maxPower, nil
}

// Main operations
func (pm *PowerManager) initializeNode() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Check if the node is already initialized
	if pm.isNodeInitialized(node) {
		pm.logger.Println("Node already initialized, skipping initialization")
		return nil
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	// Find the maximum power value across all domains and constraints
	maxPower, err := pm.findMaxPowerValue()
	if err != nil {
		return fmt.Errorf("failed to find max power value: %w", err)
	}

	// Store a single value for the node
	maxPowerValue := strconv.FormatInt(maxPower, 10)
	node.Labels["rapl/max_power_uw"] = maxPowerValue
	node.Labels["rapl/pmax"] = maxPowerValue

	// Mark the node as initialized
	if err := pm.markNodeAsInitialized(node); err != nil {
		return fmt.Errorf("failed to mark node as initialized: %w", err)
	}

	pm.logger.Printf("Node initialized with max power: %s µW", maxPowerValue)
	return nil
}

func (pm *PowerManager) adjustPowerCap() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	sourcePower := pm.calculateSourcePower()
	if sourcePower == 0 {
		return errors.New("calculated source power is zero")
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

	// Apply the determined power limit
	pm.logger.Printf("Adjusting power cap to %d µW (source: %d µW, max: %d µW, min: %d µW)", 
		pmax, sourcePower, maxPower, pm.config.RaplLimit)
	
	return pm.applyPowerLimits(node, pmax)
}

func (pm *PowerManager) applyPowerLimits(node *v1.Node, pmax int64) error {
	// Update node label for the new calculated limit
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}
	node.Labels["rapl/pmax"] = strconv.FormatInt(pmax, 10)

	// Apply this limit to all power_limit_uw files in all domains
	pmaxStr := strconv.FormatInt(pmax, 10)
	var applyErrors []string

	for _, domain := range pm.raplDomains {
		for _, constraint := range domain.Constraints {
			if err := os.WriteFile(constraint.Path, []byte(pmaxStr), 0644); err != nil {
				applyErrors = append(applyErrors, fmt.Sprintf("%s: %v", constraint.Path, err))
			}
		}
	}

	if len(applyErrors) > 0 {
		pm.logger.Printf("Errors applying power limits: %s", strings.Join(applyErrors, "; "))
	}

	return pm.updateNode(node)
}

func (pm *PowerManager) Run() {
	pm.logger.Println("Starting power management cycle...")

	ticker := time.NewTicker(pm.config.StabilisationTime)
	defer ticker.Stop()

	// Do an initial adjustment
	if err := pm.adjustPowerCap(); err != nil {
		pm.logger.Printf("Initial power cap adjustment failed: %v", err)
	}

	// Set up signal handling for graceful shutdown
	for {
		select {
		case <-ticker.C:
			if err := pm.adjustPowerCap(); err != nil {
				pm.logger.Printf("Failed to adjust power cap: %v", err)
			}
		case <-pm.ctx.Done():
			pm.logger.Println("Power manager shutting down...")
			return
		}
	}
}

func main() {
	logger := log.New(os.Stdout, "[PowerManager] ", log.LstdFlags|log.Lmicroseconds)
	logger.Println("Starting power management system...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pm, err := NewPowerManager(ctx, logger)
	if err != nil {
		logger.Fatalf("Failed to initialize power manager: %v", err)
	}

	if err := pm.initializeNode(); err != nil {
		logger.Fatalf("Failed to initialize node: %v", err)
	}

	pm.Run() // This will block until context is cancelled
}