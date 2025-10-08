package rapl

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	// RaplBasePath is the base path for RAPL domains
	RaplBasePath = "/sys/devices/virtual/powercap/intel-rapl"
)

// PowerConstraint represents a RAPL power constraint configuration
type PowerConstraint struct {
	ID    int    // constraint number (0, 1, etc.)
	Path  string // full path to the constraint file
	Value string // current power limit value
}

// Domain represents a RAPL domain with its constraints
type Domain struct {
	ID             string // e.g., "intel-rapl:0"
	Constraints    []PowerConstraint
	ConstraintsMax []PowerConstraint
}

// Manager handles RAPL domain operations
type Manager struct {
	domains []Domain
	logger  *log.Logger
}

// NewManager creates a new RAPL manager
func NewManager(logger *log.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// DiscoverDomains finds all RAPL domains and their constraints in the system
func (m *Manager) DiscoverDomains() error {
	m.logger.Printf("🔍 Discovering RAPL domains in %s...", RaplBasePath)
	var domains []Domain

	// List all RAPL domains
	entries, err := os.ReadDir(RaplBasePath)
	if err != nil {
		m.logger.Printf("❌ Failed to read RAPL base path %s: %v", RaplBasePath, err)
		return fmt.Errorf("failed to read RAPL base path: %w", err)
	}
	m.logger.Printf("📁 Found %d entries in RAPL directory", len(entries))

	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "intel-rapl:") {
			m.logger.Printf("   ⏭️  Skipping non-RAPL entry: %s", entry.Name())
			continue
		}

		m.logger.Printf("⚡ Processing RAPL domain: %s", entry.Name())
		domain := Domain{
			ID: entry.Name(),
		}

		// Read only direct constraint files in this domain
		domainPath := filepath.Join(RaplBasePath, entry.Name())
		constraintEntries, err := os.ReadDir(domainPath)
		if err != nil {
			return fmt.Errorf("failed to read domain directory %s: %w", domainPath, err)
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
				m.logger.Printf("Warning: Invalid constraint number in %s: %v", name, err)
				continue
			}

			path := filepath.Join(domainPath, name)

			// Process max power constraints
			if strings.HasSuffix(name, "_max_power_uw") {
				value, err := readPowerLimit(path)
				if err != nil {
					m.logger.Printf("Warning: Failed to read max power at %s: %v", path, err)
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
					m.logger.Printf("Warning: Failed to read power limit at %s: %v", path, err)
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
			m.logger.Printf("   ✅ Added domain %s with %d constraints and %d max constraints",
				domain.ID, len(domain.Constraints), len(domain.ConstraintsMax))
			domains = append(domains, domain)
		} else {
			m.logger.Printf("   ⚠️  Skipped domain %s (no constraints found)", domain.ID)
		}
	}

	m.domains = domains
	m.logger.Printf("✅ Domain discovery completed: found %d valid RAPL domains", len(domains))

	// Log summary of discovered domains
	for _, domain := range domains {
		m.logger.Printf("   📊 Domain %s: %d power constraints, %d max constraints",
			domain.ID, len(domain.Constraints), len(domain.ConstraintsMax))
	}

	return nil
}

// GetDomains returns the discovered RAPL domains
func (m *Manager) GetDomains() []Domain {
	return m.domains
}

// FindMaxPowerValue finds the maximum power value across all domains and constraints
func (m *Manager) FindMaxPowerValue() (int64, error) {
	m.logger.Printf("🔍 Searching for maximum power value across %d RAPL domains...", len(m.domains))
	var maxPower int64
	var maxPowerSource string

	for _, domain := range m.domains {
		m.logger.Printf("   📊 Checking domain %s...", domain.ID)

		// Check Constraints
		for _, constraint := range domain.Constraints {
			value, err := strconv.ParseInt(constraint.Value, 10, 64)
			if err == nil && value > maxPower {
				m.logger.Printf("      🔋 Found higher power constraint: %d µW (%.1f W) from %s",
					value, float64(value)/1000000, constraint.Path)
				maxPower = value
				maxPowerSource = constraint.Path
			} else if err != nil {
				m.logger.Printf("      ⚠️  Invalid constraint value '%s' at %s: %v",
					constraint.Value, constraint.Path, err)
			}
		}

		// Check ConstraintsMax
		for _, constraint := range domain.ConstraintsMax {
			value, err := strconv.ParseInt(constraint.Value, 10, 64)
			if err == nil && value > maxPower {
				m.logger.Printf("      🔋 Found higher max constraint: %d µW (%.1f W) from %s",
					value, float64(value)/1000000, constraint.Path)
				maxPower = value
				maxPowerSource = constraint.Path
			} else if err != nil {
				m.logger.Printf("      ⚠️  Invalid max constraint value '%s' at %s: %v",
					constraint.Value, constraint.Path, err)
			}
		}
	}

	if maxPower == 0 {
		m.logger.Printf("❌ No valid max power values found in any RAPL domain")
		return 0, fmt.Errorf("no valid max power values found")
	}

	m.logger.Printf("✅ Maximum power value determined: %d µW (%.1f W) from %s",
		maxPower, float64(maxPower)/1000000, maxPowerSource)
	return maxPower, nil
}

// ApplyPowerLimits applies the given power limit to all power_limit_uw files
func (m *Manager) ApplyPowerLimits(pmax int64) []error {
	pmaxStr := strconv.FormatInt(pmax, 10)
	var errors []error

	for _, domain := range m.domains {
		for _, constraint := range domain.Constraints {
			if err := os.WriteFile(constraint.Path, []byte(pmaxStr), 0644); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", constraint.Path, err))
			}
		}
	}

	return errors
}

// readPowerLimit reads power limit from a file
func readPowerLimit(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return strings.TrimSpace(string(data)), nil
}
