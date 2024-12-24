package main

import (
	"context"
	"fmt"
	//"io/fs"
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
	envNodeName           = "NODE_NAME"
	envMaxSource         = "MAX_SOURCE"
	envStabilisationTime = "STABILISATION_TIME"
	envAlpha             = "ALPHA"
	envRaplLimit         = "RAPL_LIMIT"
	
	// Default values
	defaultMaxSource         = "40000000"
	defaultStabilisationTime = "300"
	defaultAlpha            = "4"
	defaultRaplLimit        = "60"
)

// RaplDomain represents a RAPL domain with its constraints
type RaplDomain struct {
	ID          string // e.g., "intel-rapl:0"
	Constraints []PowerConstraint
}

// PowerConstraint represents a RAPL power constraint configuration
type PowerConstraint struct {
	ID    int    // constraint number (0, 1, etc.)
	Path  string // full path to the constraint file
	Value string // current power limit value
}

// Config holds the application configuration
type Config struct {
	MaxSource         float64
	Alpha            float64
	StabilisationTime time.Duration
	RaplLimit        float64
	NodeName         string
}

// PowerManager handles power management operations
type PowerManager struct {
	clientset   *kubernetes.Clientset
	config      *Config
	logger      *log.Logger
	raplDomains []RaplDomain
}

// discoverRaplDomains finds all RAPL domains and their constraints in the system
func discoverRaplDomains() ([]RaplDomain, error) {
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
            return nil, fmt.Errorf("failed to read domain directory: %w", err)
        }

        for _, constEntry := range constraintEntries {
            // Skip directories and non-constraint files
            if constEntry.IsDir() || !strings.HasPrefix(constEntry.Name(), "constraint_") || !strings.HasSuffix(constEntry.Name(), "_power_limit_uw") {
                continue
            }

            // Extract constraint number from filename
            constraintNum, err := strconv.Atoi(string(constEntry.Name()[11]))
            if err != nil {
                return nil, fmt.Errorf("invalid constraint number in %s: %w", constEntry.Name(), err)
            }

            path := filepath.Join(domainPath, constEntry.Name())
            value, err := readPowerLimit(path)
            log.Println("path : ", path, " value : ", value)
            if err != nil {
                return nil, fmt.Errorf("failed to read power limit: %w", err)
            }

            domain.Constraints = append(domain.Constraints, PowerConstraint{
                ID:    constraintNum,
                Path:  path,
                Value: value,
            })
        }

        domains = append(domains, domain)
    }

    return domains, nil
}

// NewPowerManager creates and initializes a new PowerManager
func NewPowerManager(logger *log.Logger) (*PowerManager, error) {
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	clientset, err := createKubernetesClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	domains, err := discoverRaplDomains()
	if err != nil {
		return nil, fmt.Errorf("failed to discover RAPL domains: %w", err)
	}

	log.Println("domains : ", domains)

	return &PowerManager{
		clientset:   clientset,
		config:      config,
		logger:      logger,
		raplDomains: domains,
	}, nil
}

func loadConfig() (*Config, error) {
	nodeName := os.Getenv(envNodeName)
	if nodeName == "" {
		return nil, fmt.Errorf("NODE_NAME environment variable is not set")
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

	raplLimit, err := strconv.ParseFloat(getEnvOrDefault(envRaplLimit, defaultRaplLimit), 64)
	if err != nil {
		return nil, fmt.Errorf("invalid RAPL limit: %w", err)
	}

	return &Config{
		MaxSource:         maxSource,
		Alpha:            alpha,
		StabilisationTime: stabilisationTime,
		RaplLimit:        raplLimit,
		NodeName:         nodeName,
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
// initializeNode initializes the node with power constraints
func (pm *PowerManager) initializeNode() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	// Initialize labels with power constraints for each domain
	for _, domain := range pm.raplDomains {
		domainID := strings.TrimPrefix(domain.ID, "intel-rapl:")
		for _, constraint := range domain.Constraints {
			raplLabel := fmt.Sprintf("rapl%s/constraint_%d_power_limit_uw", domainID, constraint.ID)
			craplLabel := fmt.Sprintf("crapl%s/constraint_%d_power_limit_uw", domainID, constraint.ID)
			
			node.Labels[raplLabel] = constraint.Value
			node.Labels[craplLabel] = constraint.Value
		}
	}

	return pm.updateNode(node)
}

func readPowerLimit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}



func (pm *PowerManager) getNode() (*v1.Node, error) {
	return pm.clientset.CoreV1().Nodes().Get(context.TODO(), pm.config.NodeName, metav1.GetOptions{})
}

func (pm *PowerManager) updateNode(node *v1.Node) error {
	_, err := pm.clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	return err
}

// calculateSourcePower calculates the current source power based on time of day
func (pm *PowerManager) calculateSourcePower() int64 {
	currentTime := time.Now()
	t := float64(currentTime.Hour()) + float64(currentTime.Minute())/60.0
	power := pm.config.MaxSource * math.Pow(math.Sin((math.Pi/16)*(t-4)), pm.config.Alpha)
	
	if power < 0 {
		return 0
	}
	
	return int64(math.Round(power))
}

// getCurrentPowerLimits gets current power limits for all domains from node labels
func (pm *PowerManager) getCurrentPowerLimits(node *v1.Node) (map[string][]int64, error) {
	powerLimits := make(map[string][]int64)
	
	for _, domain := range pm.raplDomains {
		domainID := strings.TrimPrefix(domain.ID, "intel-rapl:")
		var domainLimits []int64
		
		for _, constraint := range domain.Constraints {
			label := fmt.Sprintf("crapl%s/constraint_%d_power_limit_uw", domainID, constraint.ID)
			value, ok := node.Labels[label]
			if !ok {
				return nil, fmt.Errorf("power limit label not found: %s", label)
			}
			
			limit, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid power limit value: %w", err)
			}
			
			domainLimits = append(domainLimits, limit)
		}
		
		powerLimits[domain.ID] = domainLimits
	}
	
	return powerLimits, nil
}

// adjustPowerCap adjusts the power cap based on available source power
func (pm *PowerManager) adjustPowerCap() error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	sourcePower := pm.calculateSourcePower()
	if sourcePower == 0 {
		return fmt.Errorf("invalid source power")
	}

	powerLimits, err := pm.getCurrentPowerLimits(node)
	if err != nil {
		return fmt.Errorf("failed to get current power limits: %w", err)
	}

	// Find maximum non-zero power limit across all domains
	maxNonZeroPowerLimit := int64(0)
	for _, limits := range powerLimits {
		for _, limit := range limits {
			if limit > maxNonZeroPowerLimit {
				maxNonZeroPowerLimit = limit
			}
		}
	}

	if maxNonZeroPowerLimit == 0 {
		return fmt.Errorf("no valid power limit found")
	}

	ratio := float64(sourcePower) / float64(maxNonZeroPowerLimit)
	if ratio > 1 {
		return nil
	}

	factor := pm.config.RaplLimit / 100.0
	if (ratio * 100) >= pm.config.RaplLimit {
		factor = ratio
	}

	return pm.applyPowerLimits(node, powerLimits, factor)
}

// applyPowerLimits applies new power limits to all domains
func (pm *PowerManager) applyPowerLimits(node *v1.Node, currentLimits map[string][]int64, factor float64) error {
	for _, domain := range pm.raplDomains {
		domainLimits, ok := currentLimits[domain.ID]
		if !ok {
			continue
		}

		domainID := strings.TrimPrefix(domain.ID, "intel-rapl:")
		
		for i, constraint := range domain.Constraints {
			if i >= len(domainLimits) {
				continue
			}
			
			newLimit := int64(float64(domainLimits[i]) * factor)
			
			// Update RAPL file
			if err := os.WriteFile(constraint.Path, []byte(strconv.FormatInt(newLimit, 10)), 0644); err != nil {
				pm.logger.Printf("Failed to write power limit to %s: %v", constraint.Path, err)
				continue
			}
			
			// Update node label
			label := fmt.Sprintf("rapl%s/constraint_%d_power_limit_uw", domainID, constraint.ID)
			node.Labels[label] = strconv.FormatInt(newLimit, 10)
		}
	}

	return pm.updateNode(node)
}


func getEnvOrDefault(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	logger.Println("Starting power management system...")

	pm, err := NewPowerManager(logger)
	if err != nil {
		logger.Fatalf("Failed to initialize power manager: %v", err)
	}

	if err := pm.initializeNode(); err != nil {
		logger.Fatalf("Failed to initialize node: %v", err)
	}

	ticker := time.NewTicker(pm.config.StabilisationTime)
	defer ticker.Stop()

	for range ticker.C {
		if err := pm.adjustPowerCap(); err != nil {
			logger.Printf("Failed to adjust power cap: %v", err)
		}
	}
}