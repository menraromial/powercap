# PowerCap Manager - Cleanup Script Documentation

## 🧹 Overview

The PowerCap Manager cleanup script provides comprehensive cleanup capabilities for all system components, with built-in backup and restore functionality.

## 🚀 Quick Start

```bash
# Basic cleanup with backups
./scripts/cleanup.sh

# See what would be cleaned (safe)
./scripts/cleanup.sh --dry-run

# Force cleanup without prompts
./scripts/cleanup.sh --force
```

## 📋 Command Reference

### **Basic Usage**
```bash
./scripts/cleanup.sh [OPTIONS]
```

### **Options**
| Option | Description | Default |
|--------|-------------|---------|
| `-n, --namespace` | Kubernetes namespace | `default` |
| `-d, --dry-run` | Show actions without executing | `false` |
| `-v, --verbose` | Detailed output | `false` |
| `--no-backup` | Skip creating backups | `false` |
| `--force` | Skip confirmation prompts | `false` |
| `--kubernetes-only` | Only clean K8s resources | `false` |
| `--local-only` | Only clean local files | `false` |
| `--include-docker` | Also clean Docker resources | `false` |
| `-h, --help` | Show help message | - |

## 🗂️ Cleanup Categories

### **1. Kubernetes Resources**
```yaml
# What gets cleaned:
- DaemonSets: powercap-manager
- ConfigMaps: powercap-config-*
- ServiceAccounts: powercap-manager
- ClusterRoles: powercap-manager
- ClusterRoleBindings: powercap-manager
- Node annotations: rapl/*, power-manager/*
```

**Commands executed:**
```bash
kubectl delete daemonsets -l app=powercap-manager
kubectl delete configmaps -l app=powercap-manager
kubectl annotate nodes rapl/pmax- rapl/provider-
```

### **2. Local Data Files**
```bash
# File patterns cleaned:
epex_data_*.csv     # Market data files
*.tmp               # Temporary files
*.bak               # Backup files
*.log               # Log files
```

**Commands executed:**
```bash
find . -name "epex_data_*.csv" -delete
find . -name "*.tmp" -delete
find . -name "*.bak" -delete
```

### **3. RAPL Settings**
```bash
# RAPL domains reset:
/sys/class/powercap/intel-rapl:*
  ├── constraint_0_power_limit_uw → reset to max
  └── constraint_0_time_window_us → reset to default
```

**Commands executed:**
```bash
echo $max_power > /sys/class/powercap/intel-rapl:0/constraint_0_power_limit_uw
echo 976562 > /sys/class/powercap/intel-rapl:0/constraint_0_time_window_us
```

### **4. Docker Resources (Optional)**
```bash
# Docker cleanup:
- Stop PowerCap containers
- Remove PowerCap images
- Clean dangling images
```

**Commands executed:**
```bash
docker ps -a --filter 'ancestor=powercap-manager' -q | xargs docker rm -f
docker images 'powercap-manager' -q | xargs docker rmi -f
```

## 💾 Backup System

### **Automatic Backups**
Every cleanup creates timestamped backups:
```
cleanup-backups/20251006-143022/
├── daemonsets.yaml          # K8s DaemonSets
├── configmaps.yaml          # K8s ConfigMaps
├── rbac.yaml                # RBAC resources
├── node-annotations.json    # Node annotations
├── data/                    # Data files
│   ├── epex_data_2025-10-06.csv
│   └── epex_data_2025-10-05.csv
├── logs/                    # Log files
│   └── powercap.log
└── restore.sh               # Restore script
```

### **Restore Process**
```bash
# Auto-generated restore script
./cleanup-backups/20251006-143022/restore.sh

# Manual restore
kubectl apply -f cleanup-backups/20251006-143022/daemonsets.yaml
kubectl apply -f cleanup-backups/20251006-143022/configmaps.yaml
```

## 📊 Usage Examples

### **Development Workflow**
```bash
# Quick development cleanup (safe)
./scripts/cleanup.sh --dry-run --local-only

# Clean test environment
./scripts/cleanup.sh --kubernetes-only --namespace=powercap-test

# Complete development reset
./scripts/cleanup.sh --force --include-docker
```

### **Production Maintenance**
```bash
# Planned maintenance with full backup
./scripts/cleanup.sh --verbose

# Emergency cleanup (careful!)
./scripts/cleanup.sh --force --no-backup

# Partial cleanup (keep data)
./scripts/cleanup.sh --kubernetes-only
```

### **CI/CD Integration**
```bash
# In CI pipeline
./scripts/cleanup.sh --force --dry-run  # Validation
./scripts/cleanup.sh --force            # Actual cleanup

# Docker-based CI
./scripts/cleanup.sh --include-docker --force
```

## 🔍 Monitoring & Validation

### **Pre-Cleanup Check**
```bash
# See what will be cleaned
./scripts/cleanup.sh --dry-run --verbose

# Check current state
kubectl get all,cm,sa -l app=powercap-manager
ls -la epex_data_*.csv
```

### **Post-Cleanup Validation**
```bash
# Verify Kubernetes cleanup
kubectl get all,cm,sa -l app=powercap-manager
# Should return: No resources found

# Verify file cleanup
ls -la epex_data_*.csv *.log *.tmp
# Should return: No such files

# Verify node annotations
kubectl get nodes -o json | jq '.items[].metadata.annotations' | grep rapl
# Should return: null or empty
```

### **Backup Verification**
```bash
# Check backup completeness
ls -la cleanup-backups/20251006-143022/
test -f cleanup-backups/20251006-143022/restore.sh
```

## ⚠️ Safety Features

### **Confirmation Prompts**
```bash
# Interactive mode shows preview
./scripts/cleanup.sh

# Output:
⚠️  This will clean up PowerCap Manager resources.
✓ Kubernetes resources
✓ Local data files
✓ RAPL settings reset
Do you want to continue? (y/N):
```

### **Dry-Run Mode**
```bash
# Safe preview mode
./scripts/cleanup.sh --dry-run

# Shows commands without execution:
ℹ️  [DRY-RUN] Would execute: kubectl delete daemonsets -l app=powercap-manager
```

### **Backup Protection**
```bash
# Backups enabled by default
./scripts/cleanup.sh                    # ✅ Creates backup

# Only disable if certain
./scripts/cleanup.sh --no-backup        # ⚠️  No backup created
```

## 🛠️ Troubleshooting

### **Common Issues**

#### **Permission Errors**
```bash
# RAPL access denied
sudo ./scripts/cleanup.sh

# Kubernetes access denied
kubectl auth can-i delete daemonsets
```

#### **Incomplete Cleanup**
```bash
# Some resources remain
kubectl get all -A | grep powercap

# Manual cleanup
kubectl delete namespace powercap-system --force
```

#### **Backup Issues**
```bash
# Backup directory permissions
chmod -R 755 cleanup-backups/

# Disk space for backups
df -h cleanup-backups/
```

### **Recovery Scenarios**

#### **Accidental Cleanup**
```bash
# Use latest backup
LATEST=$(ls -t cleanup-backups/ | head -1)
./cleanup-backups/$LATEST/restore.sh
```

#### **Partial Failure**
```bash
# Re-run specific category
./scripts/cleanup.sh --kubernetes-only
./scripts/cleanup.sh --local-only
```

#### **Corrupted Backup**
```bash
# Manual restoration
kubectl apply -f k8s/
docker build -t powercap-manager .
```

## 🔧 Configuration

### **Environment Variables**
```bash
# Override defaults
export POWERCAP_NAMESPACE="powercap-system"
export POWERCAP_BACKUP_DIR="/opt/backups"
./scripts/cleanup.sh
```

### **Configuration File**
```bash
# Edit cleanup settings
vim scripts/cleanup.conf

# Use custom config
CLEANUP_CONFIG=./my-cleanup.conf ./scripts/cleanup.sh
```

## 📈 Best Practices

### **Regular Maintenance**
```bash
# Weekly data cleanup
./scripts/cleanup.sh --local-only --force

# Monthly full cleanup
./scripts/cleanup.sh --dry-run  # Review first
./scripts/cleanup.sh            # Execute
```

### **Environment Management**
```bash
# Development
./scripts/cleanup.sh --include-docker

# Staging
./scripts/cleanup.sh --kubernetes-only

# Production
./scripts/cleanup.sh --verbose  # With review
```

### **Backup Management**
```bash
# Cleanup old backups (manual)
find cleanup-backups/ -type d -mtime +30 -exec rm -rf {} \;

# Archive important backups
tar czf powercap-backup-$(date +%Y%m).tar.gz cleanup-backups/
```

This cleanup system ensures safe, comprehensive cleanup of PowerCap Manager with full backup and restore capabilities.