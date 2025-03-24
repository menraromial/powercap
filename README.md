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
          image: menraromial/powercap:latest
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
            - name: RAPL_LIMIT
              value: "40"
            - name: STABILISATION_TIME
              value: "120"
            - name: ALPHA
              value: "4"
            - name: PMAX_FUNC
              value: min
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

