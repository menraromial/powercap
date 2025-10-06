# PowerCap Manager - Annotations Guide

## 🏷️ Node Annotations Overview

The PowerCap Manager uses Kubernetes **annotations** (not labels) to store detailed power management information on nodes. Annotations are perfect for metadata that doesn't need to be used for object selection but provides valuable operational information.

## 📋 Annotation Schema

### **Power Information**
```yaml
metadata:
  annotations:
    # Core power data
    rapl/pmax: "36300000"                    # Current power limit in µW
    rapl/max_power_uw: "40000000"            # Maximum available power in µW
    rapl/last-update: "2025-10-06T13:22:45Z" # Last update timestamp (RFC3339)
    rapl/provider: "epex"                     # Active data provider
    
    # Market context (when available)
    rapl/market-period: "13:15-13:30"        # Current 15-minute period
    rapl/market-volume: "85.2"               # Market volume in MWh
    rapl/market-price: "45.67"               # Market price in €/MWh
    
    # System status
    power-manager/initialized: "true"         # Initialization status
```

## 🔍 Monitoring Annotations

### **Quick Node Check**
```bash
# View all PowerCap annotations for a specific node
kubectl get node <node-name> -o yaml | grep -A 20 "annotations:"

# Get current power limit
kubectl get node <node-name> -o jsonpath='{.metadata.annotations.rapl/pmax}'

# Get last update time
kubectl get node <node-name> -o jsonpath='{.metadata.annotations.rapl/last-update}'
```

### **Using the Monitoring Script**
```bash
# List all nodes with PowerCap annotations
./scripts/monitor-annotations.sh

# Real-time monitoring
./scripts/monitor-annotations.sh monitor

# Export to JSON
./scripts/monitor-annotations.sh export
```

### **Script Output Example**
```
🔍 PowerCap Manager - Node Annotations Monitor
==============================================

📊 Node: worker-node-1
├─ Power Information:
│  ├─ Current Power Limit: 36300000 µW (36 W)
│  ├─ Maximum Power: 40000000 µW (40 W)
│  ├─ Data Provider: epex
│  ├─ Last Update: 2025-10-06T13:22:45Z
│  ├─ Status: Initialized ✅
├─ Market Information:
│  ├─ Current Period: 13:15-13:30
│  ├─ Market Volume: 85.2 MWh
│  ├─ Market Price: 45.67 €/MWh
└─
```

## 📊 Prometheus Integration

### **Custom Metrics from Annotations**
```yaml
# prometheus-rules.yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: powercap-annotations
spec:
  groups:
  - name: powercap.rules
    rules:
    - record: powercap:current_power_watts
      expr: |
        label_replace(
          kube_node_annotations{annotation_rapl_pmax!=""},
          "power_watts", 
          "$1", 
          "annotation_rapl_pmax", 
          "([0-9]+)"
        ) / 1000000
        
    - record: powercap:market_volume_mwh
      expr: |
        label_replace(
          kube_node_annotations{annotation_rapl_market_volume!=""},
          "volume_mwh",
          "$1",
          "annotation_rapl_market_volume",
          "([0-9.]+)"
        )
```

### **Grafana Dashboard Query Examples**
```promql
# Current power consumption across all nodes
powercap:current_power_watts

# Market price tracking
label_replace(
  kube_node_annotations{annotation_rapl_market_price!=""},
  "price_eur_mwh",
  "$1",
  "annotation_rapl_market_price",
  "([0-9.]+)"
)

# Power efficiency (Power/Volume ratio)
powercap:current_power_watts / powercap:market_volume_mwh
```

## 🔧 Troubleshooting Annotations

### **Common Issues**

#### **Missing Annotations**
```bash
# Check if PowerCap Manager is running
kubectl get pods -l app=powercap-manager

# Check manager logs
kubectl logs -l app=powercap-manager --tail=50

# Verify RBAC permissions
kubectl auth can-i update nodes --as=system:serviceaccount:default:powercap-manager
```

#### **Stale Annotations**
```bash
# Check last update time
kubectl get nodes -o custom-columns=\
NAME:.metadata.name,\
LAST_UPDATE:.metadata.annotations.rapl/last-update,\
POWER:.metadata.annotations.rapl/pmax

# If stale, restart PowerCap Manager
kubectl rollout restart daemonset/powercap-manager
```

#### **Invalid Annotation Values**
```bash
# Validate power values are numeric
kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{": "}{.metadata.annotations.rapl/pmax}{"\n"}{end}' | \
  while read line; do
    node=$(echo $line | cut -d: -f1)
    power=$(echo $line | cut -d: -f2 | xargs)
    if ! [[ "$power" =~ ^[0-9]+$ ]]; then
      echo "❌ Invalid power value on $node: $power"
    fi
  done
```

## 🎯 Best Practices

### **Annotation Naming**
- Use `/` to create hierarchical namespaces
- Prefix with `rapl/` for power-related data
- Prefix with `power-manager/` for system metadata
- Use consistent date formats (RFC3339)

### **Monitoring Strategy**
1. **Real-time**: Monitor `rapl/last-update` for freshness
2. **Alerting**: Set up alerts for missing or stale annotations
3. **Trending**: Track `rapl/pmax` changes over time
4. **Correlation**: Compare `rapl/market-price` with power adjustments

### **Data Retention**
```bash
# Archive annotations before major updates
kubectl get nodes -o json | \
  jq '.items[] | {name: .metadata.name, annotations: .metadata.annotations}' | \
  jq 'select(.annotations | has("rapl/pmax"))' > \
  powercap-backup-$(date +%Y%m%d).json
```

## 🚀 Advanced Usage

### **Custom Annotation Processing**
```go
// Example: Custom controller watching PowerCap annotations
func (c *Controller) processPowerAnnotations(node *v1.Node) {
    if node.Annotations == nil {
        return
    }
    
    // Parse power limit
    if powerStr, exists := node.Annotations["rapl/pmax"]; exists {
        power, err := strconv.ParseInt(powerStr, 10, 64)
        if err == nil {
            c.updatePowerMetrics(node.Name, power)
        }
    }
    
    // Parse market data
    if volume, exists := node.Annotations["rapl/market-volume"]; exists {
        if price, exists := node.Annotations["rapl/market-price"]; exists {
            c.calculateEfficiency(node.Name, volume, price)
        }
    }
}
```

### **Annotation-Based Scheduling**
```yaml
# Pod that prefers nodes with specific power characteristics
apiVersion: v1
kind: Pod
spec:
  affinity:
    nodeAffinity:
      preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        preference:
          matchExpressions:
          - key: node.alpha.kubernetes.io/annotations.rapl/provider
            operator: In
            values: ["epex"]
      - weight: 50
        preference:
          matchExpressions:
          - key: node.alpha.kubernetes.io/annotations.rapl/pmax
            operator: Gt
            values: ["30000000"]  # Prefer nodes with > 30W
```

This comprehensive annotation system provides rich operational visibility while maintaining clean separation between selection criteria (labels) and operational metadata (annotations).