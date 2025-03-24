
# Power Manager for Kubernetes Nodes

## Overview
This project is a Kubernetes-based power management tool designed to monitor and control power consumption on cluster nodes using Intel RAPL (Running Average Power Limit) domains. The Power Manager interacts with the Kubernetes API and dynamically adjusts power constraints to optimize energy usage.

## Features
- Discovers available RAPL power domains on the system
- Reads and manages power constraints
- Configures power limits based on environmental variables
- Integrates with Kubernetes to apply node-specific configurations
- Logs power adjustments for monitoring

## Requirements
- Kubernetes cluster with accessible API server
- Intel RAPL support in the node hardware
- Go 1.18+ installed
- Client-go library for Kubernetes

## Installation
1. Clone the repository:
   ```sh
   git clone <repository_url>
   cd <repository_folder>
   ```
2. Build the binary:
   ```sh
   go build -o power-manager main.go
   ```
3. Run the application:
   ```sh
   ./power-manager
   ```

## Environment Variables
The application relies on the following environment variables:

| Variable              | Description                                  | Default Value |
|----------------------|----------------------------------|------------------|
| NODE_NAME           | Kubernetes node name               | (Required)      |
| MAX_SOURCE         | Maximum power source in µW       | 40000000        |
| STABILISATION_TIME | Stabilization time in seconds     | 300             |
| ALPHA              | Adjustment factor                 | 4               |
| RAPL_MIN_POWER     | Minimum power limit in µW        | 10000000        |

## Usage
1. Set up environment variables:
   ```sh
   export NODE_NAME="node-1"
   export MAX_SOURCE="40000000"
   export STABILISATION_TIME="300"
   export ALPHA="4"
   export RAPL_MIN_POWER="10000000"
   ```
2. Start the power manager:
   ```sh
   ./power-manager
   ```

## How It Works
- The Power Manager discovers available RAPL power domains on the node.
- It reads the current power constraints from the system.
- Based on the configuration, it adjusts power limits dynamically.
- It communicates with the Kubernetes API to ensure proper integration and logging.

## Logging
The application provides logs for:
- Detected RAPL power domains
- Power limit changes
- Kubernetes API interactions

## Future Improvements
- Support for additional hardware architectures
- Integration with Prometheus for monitoring
- Dynamic adjustment strategies based on workload analysis

## License
This project is licensed under the MIT License.

