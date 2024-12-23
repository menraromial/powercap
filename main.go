package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"os"
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
	envMaxHour		   = "MAX_HOUR"
	
	// Default values
	defaultMaxSource         = "40000000"
	defaultStabilisationTime = "300"
	defaultAlpha            = "4"
	defaultRaplLimit        = "60"
	defaultMaxHour		   = "4"
)

// PowerConstraint represents a RAPL power constraint configuration
type PowerConstraint struct {
	Path  string
	Value string
}

// Config holds the application configuration
type Config struct {
	MaxSource         float64
	Alpha            float64
	StabilisationTime time.Duration
	RaplLimit        float64
	MaxHour		   int
	NodeName         string
}

// PowerManager handles power management operations
type PowerManager struct {
	clientset *kubernetes.Clientset
	config    *Config
	logger    *log.Logger
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

	return &PowerManager{
		clientset: clientset,
		config:    config,
		logger:    logger,
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

	maxHour, err := strconv.Atoi(getEnvOrDefault(envMaxHour, defaultMaxHour))
	if err != nil {
		return nil, fmt.Errorf("invalid max hour: %w", err)
	}

	return &Config{
		MaxSource:         maxSource,
		Alpha:            alpha,
		StabilisationTime: stabilisationTime,
		RaplLimit:        raplLimit,
		MaxHour:		   maxHour,
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

// getPowerConstraints returns all RAPL power constraints
func (pm *PowerManager) getPowerConstraints() ([]PowerConstraint, error) {
	constraints := []PowerConstraint{
		{Path: fmt.Sprintf("%s/intel-rapl:0/constraint_0_power_limit_uw", raplBasePath)},
		{Path: fmt.Sprintf("%s/intel-rapl:0/constraint_1_power_limit_uw", raplBasePath)},
		{Path: fmt.Sprintf("%s/intel-rapl:1/constraint_0_power_limit_uw", raplBasePath)},
		{Path: fmt.Sprintf("%s/intel-rapl:1/constraint_1_power_limit_uw", raplBasePath)},
	}

	for i := range constraints {
		value, err := readPowerLimit(constraints[i].Path)
		if err != nil {
			constraints[i].Value = "0"
			//return nil, fmt.Errorf("failed to read power limit: %w", err)
		}
		constraints[i].Value = value
	}

	return constraints, nil
}

func readPowerLimit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}

// initializeNode initializes the node with power constraints
func (pm *PowerManager) initializeNode(constraints []PowerConstraint) error {
	node, err := pm.getNode()
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	// Initialize labels with power constraints
	for i, constraint := range constraints {
		raplPrefix := fmt.Sprintf("rapl%d/constraint_%d_power_limit_uw", i/2, i%2)
		craplPrefix := fmt.Sprintf("crapl%d/constraint_%d_power_limit_uw", i/2, i%2)
		
		node.Labels[raplPrefix] = constraint.Value
		node.Labels[craplPrefix] = constraint.Value
	}

	return pm.updateNode(node)
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
	power := pm.config.MaxSource * math.Pow(math.Sin((math.Pi/16)*(t-float64(pm.config.MaxHour))), pm.config.Alpha)
	
	if power < 0 {
		return 0
	}
	
	return int64(math.Round(power))
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

	// Trouver la valeur maximale non nulle
	maxNonZeroPowerLimit := int64(0)
	for _, limit := range powerLimits {
		if limit > maxNonZeroPowerLimit {
			maxNonZeroPowerLimit = limit
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

func (pm *PowerManager) getCurrentPowerLimits(node *v1.Node) ([]int64, error) {
	var powerLimits []int64
	
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			label := fmt.Sprintf("crapl%d/constraint_%d_power_limit_uw", i, j)
			value, ok := node.Labels[label]
			if !ok {
				return nil, fmt.Errorf("power limit label not found: %s", label)
			}
			
			limit, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("invalid power limit value: %w", err)
			}
			
			powerLimits = append(powerLimits, limit)
		}
	}
	
	return powerLimits, nil
}

func (pm *PowerManager) applyPowerLimits(node *v1.Node, currentLimits []int64, factor float64) error {
	constraints, err := pm.getPowerConstraints()
	if err != nil {
		return fmt.Errorf("failed to get power constraints: %w", err)
	}

	for i, constraint := range constraints {
		newLimit := int64(float64(currentLimits[i]) * factor)
		
		// Update RAPL file
		if err := os.WriteFile(constraint.Path, []byte(strconv.FormatInt(newLimit, 10)), 0644); err != nil {
			pm.logger.Printf("Failed to write power limit to %s: %v", constraint.Path, err)
			continue
		}
		
		// Update node label
		label := fmt.Sprintf("rapl%d/constraint_%d_power_limit_uw", i/2, i%2)
		node.Labels[label] = strconv.FormatInt(newLimit, 10)
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

	constraints, err := pm.getPowerConstraints()
	if err != nil {
		logger.Fatalf("Failed to get power constraints: %v", err)
	}

	if err := pm.initializeNode(constraints); err != nil {
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