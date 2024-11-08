package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	raplPath                   = "/sys/devices/virtual/powercap/intel-rapl"
	constraint0PowerLimitFile0 = raplPath + "/intel-rapl:0/constraint_0_power_limit_uw"
	constraint1PowerLimitFile0 = raplPath + "/intel-rapl:0/constraint_1_power_limit_uw"
	constraint0PowerLimitFile1 = raplPath + "/intel-rapl:1/constraint_0_power_limit_uw"
	constraint1PowerLimitFile1 = raplPath + "/intel-rapl:1/constraint_1_power_limit_uw"
	nodeEnv                    = "NODE_NAME"
	timeToSleep                = 60 * time.Second
	minSource                  = 10000000.0
	maxSource                  = 200000000.0
)

func init() {
	// Set log output format to include date and time
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
}

func getKubeClient() (*kubernetes.Clientset, error) {
	log.Println("Attempting to get in-cluster config")
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Printf("Error getting in-cluster config: %v", err)
		return nil, fmt.Errorf("error getting in-cluster config: %w", err)
	}

	log.Println("Attempting to create Kubernetes clientset")
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Printf("Error creating Kubernetes clientset: %v", err)
		return nil, fmt.Errorf("error creating kubernetes clientset: %w", err)
	}
	log.Println("Successfully created Kubernetes clientset")

	return clientset, nil
}

func getNodeName() (string, error) {
	nodeName := os.Getenv(nodeEnv)
	if nodeName == "" {
		log.Println("NODE_NAME environment variable is not set")
		return "", fmt.Errorf("no node name found in environment variable %s", nodeEnv)
	}
	log.Printf("NODE_NAME environment variable is set to %s", nodeName)
	return nodeName, nil
}

func getNode(clientset *kubernetes.Clientset, nodeName string) (*v1.Node, error) {
	log.Printf("Attempting to get node %s", nodeName)
	node, err := clientset.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
	if err != nil {
		log.Printf("Error getting node %s: %v", nodeName, err)
		return nil, fmt.Errorf("error getting node %s: %w", nodeName, err)
	}
	log.Printf("Successfully retrieved node %s", nodeName)
	return node, nil
}

// getPowerLimits retrieves power limits from predefined constraint files.
// It returns four power limit values as strings and an error if any occurs during reading the files.
// If an error occurs while reading a file, the corresponding power limit is set to "0".
//
// Returns:
//   - string: Power limit from constraint0PowerLimitFile0
//   - string: Power limit from constraint1PowerLimitFile0
//   - string: Power limit from constraint0PowerLimitFile1
//   - string: Power limit from constraint1PowerLimitFile1
//   - error: Error encountered during reading the power limit files, if any
func getPowerLimits() (string, string, string, string) {
	constraints := []string{
		constraint0PowerLimitFile0,
		constraint1PowerLimitFile0,
		constraint0PowerLimitFile1,
		constraint1PowerLimitFile1,
	}

	limits := make([]string, len(constraints))
	for i, filePath := range constraints {
		limit, err := readPowerLimit(filePath)
		if err != nil {
			log.Printf("Error reading power limit from %s: %v", filePath, err)
			limits[i] = "0"
		} else {
			log.Printf("Successfully read power limit from %s: %s", filePath, limit)
			limits[i] = limit
		}
	}

	return limits[0], limits[1], limits[2], limits[3]
}

func readPowerLimit(filePath string) (string, error) {

	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("Failed to read file %s: %v", filePath, err)
		return "", fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	return strings.TrimSpace(string(data)), nil

	// min := 23000000
	// max := 110000000
	// randomNumber := rand.Intn(max-min+1) + min
	// str := strconv.Itoa(randomNumber)
	// return str, nil
}

// initNodePowerLimits initializes the power limits for RAPL (Running Average Power Limit) domains
// on a specified Kubernetes node by setting appropriate labels.
//
// Parameters:
// - clientset: A Kubernetes clientset to interact with the Kubernetes API.
// - nodeName: The name of the node to set the power limits on.
// - constraint0PowerLimit0: Power limit for RAPL domain 0, constraint 0.
// - constraint1PowerLimit0: Power limit for RAPL domain 0, constraint 1.
// - constraint0PowerLimit1: Power limit for RAPL domain 1, constraint 0.
// - constraint1PowerLimit1: Power limit for RAPL domain 1, constraint 1.
func initNodePowerLimits(clientset *kubernetes.Clientset,
	nodeName string,
	constraint0PowerLimit0,
	constraint1PowerLimit0,
	constraint0PowerLimit1,
	constraint1PowerLimit1 string) error {

	node, err := getNode(clientset, nodeName)
	if err != nil {
		log.Printf("Error retrieving node %s: %v", nodeName, err)
		return err
	}
	if node.Labels == nil {
		node.Labels = make(map[string]string)
	}

	// Initialize the RAPL domains with the power limits
	labels := map[string]string{
		"rapl0/constraint-0-power-limit-uw":  constraint0PowerLimit0,
		"rapl0/constraint-1-power-limit-uw":  constraint1PowerLimit0,
		"rapl1/constraint-0-power-limit-uw":  constraint0PowerLimit1,
		"rapl1/constraint-1-power-limit-uw":  constraint1PowerLimit1,
		"crapl0/constraint-0-power-limit-uw": constraint0PowerLimit0,
		"crapl0/constraint-1-power-limit-uw": constraint1PowerLimit0,
		"crapl1/constraint-0-power-limit-uw": constraint0PowerLimit1,
		"crapl1/constraint-1-power-limit-uw": constraint1PowerLimit1,
	}

	for key, value := range labels {
		// Check if the label key exists in the node's labels map and if the key starts with "c"(constant or constructor value).
		// If the key does not exist in the node's labels or does not start with "c",
		// add or update the label in the node's labels map with the provided value.
		if _, ok := node.Labels[key]; !ok || !strings.HasPrefix(key, "c") {
			node.Labels[key] = value
		}
	}

	_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
	if err != nil {
		log.Printf("Error updating node %s: %v", nodeName, err)
		return fmt.Errorf("error updating node %s: %w", nodeName, err)
	}
	return nil
}

func getNodeLabelValue(clientset *kubernetes.Clientset, nodeName, label string) (string, error) {
	node, err := getNode(clientset, nodeName)
	if err != nil {
		return "", err
	}

	value, ok := node.Labels[label]
	if !ok || value == "" {
		log.Printf("Label %s not found on node %s", label, nodeName)
		return "", fmt.Errorf("label %s not found on node %s", label, nodeName)
	}
	return strings.TrimSpace(value), nil
}

func getSourcePower() (int64, error) {
	// Placeholder implementation

	source := rand.NewSource(time.Now().UnixNano())
	r := rand.New(source)

	// Générer un nombre aléatoire exponentiel
	lambda := 1.0 // le taux pour la distribution exponentielle
	expRandom := r.ExpFloat64()

	// Échelle de la distribution exponentielle pour qu'elle corresponde aux bornes spécifiées
	scaledExpRandom := minSource + (maxSource-minSource)*(1-math.Exp(-lambda*expRandom))
	log.Printf("Generated source power: %d", int64(math.Round(scaledExpRandom)))

	return int64(math.Round(scaledExpRandom)), nil
}

// powerCap adjusts the power limits of a Kubernetes node based on the source power available.
// It retrieves the current power limits from the node's labels, calculates the new power limits
// based on the ratio of the power limit to the source power, and updates the node's labels and
// corresponding files if the new power limit percentage is greater than or equal to 60%.
//
// Parameters:
// - clientset: A Kubernetes clientset to interact with the Kubernetes API.
// - nodeName: The name of the node to adjust the power limits for.
//
// Returns:
// - An error if any step in the process fails, otherwise nil.
func powerCap(clientset *kubernetes.Clientset, nodeName string) error {
	node, err := getNode(clientset, nodeName)
	if err != nil {
		log.Printf("Error retrieving node %s: %v", nodeName, err)
		return err
	}

	labels := []string{
		"rapl0/constraint-0-power-limit-uw",
		"rapl0/constraint-1-power-limit-uw",
		"rapl1/constraint-0-power-limit-uw",
		"rapl1/constraint-1-power-limit-uw",
	}

	powerLimits := make([]int64, len(labels))
	for i, label := range labels {
		value, err := getNodeLabelValue(clientset, nodeName, label)
		if err != nil {
			log.Printf("Error getting node label value for %s: %v", label, err)
			return err
		}
		powerLimits[i], err = strconv.ParseInt(value, 10, 64)
		if err != nil {
			log.Printf("Error parsing power limit for label %s: %v", label, err)
			return fmt.Errorf("error parsing power limit for label %s: %w", label, err)
		}
	}

	sourcePower, err := getSourcePower()
	if err != nil || sourcePower == 0 {
		log.Printf("Error getting source power for node %s: %v", nodeName, err)
		return fmt.Errorf("source power not found for node %s", nodeName)
	}

	r := float64(powerLimits[1]) / float64(sourcePower)
	if r < 1 {
		pc := r * 100
		if pc >= 60 {
			for i, filePath := range []string{
				constraint0PowerLimitFile0,
				constraint1PowerLimitFile0,
				constraint0PowerLimitFile1,
				constraint1PowerLimitFile1,
			} {
				newPowerLimit := int64(float64(powerLimits[i]) * r)
				err = os.WriteFile(filePath, []byte(strconv.FormatInt(newPowerLimit, 10)), 0644)
				if err == nil {
					node.Labels[labels[i]] = strconv.FormatInt(newPowerLimit, 10)
				} else {
					log.Printf("Error writing power limit to file %s: %v", filePath, err)
				}
			}

			_, err = clientset.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Error updating node labels: %v", err)
				return fmt.Errorf("error updating node labels: %w", err)
			}
		}
	}

	return nil
}

/*
	func modifyPowerLimit(newConstraint0PowerLimit, newConstraint1PowerLimit int64) error {
		cmd0 := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo %d > %s", newConstraint0PowerLimit, constraint0PowerLimitFile))
		if err := cmd0.Run(); err != nil {
			return fmt.Errorf("failed to set constraint0 power limit: %w", err)
		}

		cmd1 := exec.Command("sudo", "sh", "-c", fmt.Sprintf("echo %d > %s", newConstraint1PowerLimit, constraint1PowerLimitFile))
		if err := cmd1.Run(); err != nil {
			return fmt.Errorf("failed to set constraint1 power limit: %w", err)
		}

		return nil
	}
*/
func main() {
	rand.NewSource(time.Now().UnixNano())

	log.Println("Starting power capping application...")

	clientset, err := getKubeClient()
	if err != nil {
		log.Fatalf("failed to get kubernetes client: %v", err)
	}

	nodeName, err := getNodeName()
	if err != nil {
		log.Fatalf("failed to get node name: %v", err)
	}

	constraint0PowerLimit0, constraint1PowerLimit0, constraint0PowerLimit1, constraint1PowerLimit1 := getPowerLimits()

	err = initNodePowerLimits(clientset, nodeName, constraint0PowerLimit0, constraint1PowerLimit0, constraint0PowerLimit1, constraint1PowerLimit1)
	if err != nil {
		log.Fatalf("failed to init node power limits: %v", err)
	}

	// for {
	// 	err = powerCap(clientset, nodeName)
	// 	if err != nil {
	// 		log.Printf("error during power capping: %v", err)
	// 	}

	// 	time.Sleep(timeToSleep) // Sleep to avoid tight loop
	// }

	ticker := time.NewTicker(timeToSleep)
	defer ticker.Stop()

	for range ticker.C {
		if err := powerCap(clientset, nodeName); err != nil {
			log.Printf("Error during power capping: %v", err)
		}
	}
}
