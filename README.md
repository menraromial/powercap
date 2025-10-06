# Power Manager for Kubernetes Nodes with EPEX Integration

## Overview
This project is a Kubernetes-based power management tool designed to monitor and control power consumption on cluster nodes using Intel RAPL (Running Average Power Limit) domains. The Power Manager integrates with EPEX market data to dynamically adjust power constraints based on real-time energy market conditions.

## 🆕 New Features
- **Automatic EPEX data scraping** at midnight every day
- **Real-time market-based power calculation** using EPEX volume data
- **Dynamic CSV generation** with daily market data
- **Rule-of-three power scaling** based on market volumes
- **Intelligent fallback** to previous day's data if current data is unavailable

## Features
- Discovers available RAPL power domains on the system
- Reads and manages power constraints
- Configures power limits based on EPEX market data and environmental variables
- Integrates with Kubernetes to apply node-specific configurations
- Automatically generates and updates EPEX data files daily
- Logs power adjustments for monitoring

## Requirements
- Kubernetes cluster with accessible API server
- Intel RAPL support in the node hardware
- Go 1.18+ installed
- Client-go library for Kubernetes

## Installation
1. Clone the repository:
   ```sh
   git clone https://github.com/menraromial/powercap.git
   cd powercap
   ```
2. Build the binary:
   ```sh
   go build -o powercap main.go
   ```
3. Run the application:
   ```sh
   ./powercap
   ```

## Environment Variables
The application relies on the following environment variables:

| Variable              | Description                                  | Default Value |
|----------------------|----------------------------------|------------------|
| NODE_NAME           | Kubernetes node name               | (Required)      |
| MAX_SOURCE         | Maximum power source in µW       | 40000000        |
| STABILISATION_TIME | Stabilization time in seconds     | 300             |
| ALPHA              | Adjustment factor (legacy)        | 4               |
| RAPL_MIN_POWER     | Minimum RAPL power limit in µW   | 10000000        |

## 🔄 EPEX Integration

### How it works
The power manager now uses real-time EPEX (European Power Exchange) market data to calculate power consumption based on energy market conditions:

1. **Data Collection**: Every day at midnight, the system automatically scrapes EPEX market data for 15-minute intervals
2. **Power Calculation**: Uses a rule-of-three calculation:
   ```
   current_power = (current_volume / max_volume_in_day) × MAX_SOURCE
   ```
3. **Dynamic Adjustment**: Power limits are updated every `STABILISATION_TIME` seconds based on the current 15-minute market period

### EPEX Data Format
The generated CSV files follow this format:
```csv
Period,Volume (MWh),Price (€/MWh)
00:00-00:15,66.3,31.91
00:15-00:30,65.3,29.39
```

### Manual EPEX Data Generation
To manually generate EPEX data for testing:
```sh
./powercap test-epex
```

### Automatic Data Management
- Files are named `epex_data_YYYY-MM-DD.csv`
- Generated automatically at 00:00 every day
- Falls back to previous day's data if current data is unavailable
- Logs all data generation and loading activities
| RAPL_MIN_POWER     | Minimum power limit in µW        | 10000000        |

## Kubernetes Deployment
To deploy the Power Manager as a DaemonSet in your Kubernetes cluster, apply the following YAML configuration:

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: powercap
  namespace: default

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: powercap
rules:
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch", "update"]

---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: powercap
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: powercap
subjects:
  - kind: ServiceAccount
    name: powercap
    namespace: default

---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: powercap
  namespace: default
spec:
  selector:
    matchLabels:
      name: powercap
  template:
    metadata:
      labels:
        name: powercap
    spec:
      serviceAccountName: powercap
      containers:
        - name: powercap
          image: ghcr.io/menraromial/powercap:sha-7d14609
          imagePullPolicy: Always
          securityContext:
            privileged: true
            runAsUser: 0
            runAsGroup: 0
            capabilities:
              add: ["SYS_ADMIN"]
          env:
            - name: MAX_SOURCE
              value: "140000000"
            - name: RAPL_MIN_POWER
              value: "11000000"
            - name: STABILISATION_TIME
              value: "120"
            - name: ALPHA
              value: "4"
            - name: NODE_NAME
              valueFrom:
                fieldRef:
                  fieldPath: spec.nodeName
          volumeMounts:
          - name: intel-rapl
            mountPath: /sys/devices/virtual/powercap/intel-rapl
      volumes:
      - name: intel-rapl
        hostPath:
          path: /sys/devices/virtual/powercap/intel-rapl
          type: Directory
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule

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

