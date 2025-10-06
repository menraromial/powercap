#!/bin/bash
# PowerCap Manager - Comprehensive Cleanup Script
# This script cleans up all PowerCap Manager resources and data

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
BACKUP_DIR="${PROJECT_ROOT}/cleanup-backups/$(date +%Y%m%d-%H%M%S)"

# Default values
DRY_RUN=false
VERBOSE=false
BACKUP_ENABLED=true
FORCE=false
NAMESPACE="default"

# Cleanup categories
CLEANUP_KUBERNETES=true
CLEANUP_LOCAL_DATA=true
CLEANUP_RAPL=true
CLEANUP_LOGS=true
CLEANUP_DOCKER=false

# Function to print colored output
print_status() {
    local color=$1
    local message=$2
    echo -e "${color}${message}${NC}"
}

print_info() {
    print_status "$BLUE" "ℹ️  $1"
}

print_success() {
    print_status "$GREEN" "✅ $1"
}

print_warning() {
    print_status "$YELLOW" "⚠️  $1"
}

print_error() {
    print_status "$RED" "❌ $1"
}

# Function to show help
show_help() {
    cat << EOF
PowerCap Manager - Cleanup Script

Usage: $0 [OPTIONS]

OPTIONS:
    -n, --namespace NAMESPACE    Kubernetes namespace (default: default)
    -d, --dry-run               Show what would be done without executing
    -v, --verbose               Verbose output
    --no-backup                 Skip creating backups
    --force                     Force cleanup without confirmation
    --kubernetes-only           Only clean Kubernetes resources
    --local-only                Only clean local data and files
    --include-docker            Also clean Docker images and containers
    -h, --help                  Show this help message

CLEANUP CATEGORIES:
    Kubernetes Resources:
        - DaemonSets, ConfigMaps, ServiceAccounts
        - RBAC resources (ClusterRoles, ClusterRoleBindings)
        - Node annotations and labels
        
    Local Data:
        - CSV data files (epex_data_*.csv)
        - Log files
        - Temporary files
        
    RAPL Settings:
        - Reset power limits to system defaults
        - Clear custom constraints
        
    Docker (optional):
        - Remove PowerCap images
        - Clean up containers

EXAMPLES:
    $0                          # Interactive cleanup with backups
    $0 --dry-run                # Show what would be cleaned
    $0 --kubernetes-only        # Only clean K8s resources
    $0 --force --no-backup      # Force cleanup without backups

EOF
}

# Parse command line arguments
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -n|--namespace)
                NAMESPACE="$2"
                shift 2
                ;;
            -d|--dry-run)
                DRY_RUN=true
                shift
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            --no-backup)
                BACKUP_ENABLED=false
                shift
                ;;
            --force)
                FORCE=true
                shift
                ;;
            --kubernetes-only)
                CLEANUP_LOCAL_DATA=false
                CLEANUP_RAPL=false
                CLEANUP_LOGS=false
                shift
                ;;
            --local-only)
                CLEANUP_KUBERNETES=false
                shift
                ;;
            --include-docker)
                CLEANUP_DOCKER=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                show_help
                exit 1
                ;;
        esac
    done
}

# Function to execute commands with dry-run support
execute_cmd() {
    local cmd="$1"
    local description="$2"
    
    if [[ "$VERBOSE" == "true" ]]; then
        print_info "$description"
        echo "Command: $cmd"
    fi
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_info "[DRY-RUN] Would execute: $cmd"
        return 0
    fi
    
    if eval "$cmd"; then
        [[ "$VERBOSE" == "true" ]] && print_success "$description completed"
        return 0
    else
        local exit_code=$?
        print_warning "$description failed (exit code: $exit_code)"
        return $exit_code
    fi
}

# Function to create backup directory
create_backup_dir() {
    if [[ "$BACKUP_ENABLED" == "true" && "$DRY_RUN" == "false" ]]; then
        mkdir -p "$BACKUP_DIR"
        print_info "Backup directory created: $BACKUP_DIR"
    fi
}

# Function to backup Kubernetes resources
backup_kubernetes_resources() {
    if [[ "$BACKUP_ENABLED" == "false" || "$DRY_RUN" == "true" ]]; then
        return 0
    fi
    
    print_info "Backing up Kubernetes resources..."
    
    # Backup DaemonSets
    if kubectl get daemonsets -n "$NAMESPACE" -l app=powercap-manager -o yaml > "$BACKUP_DIR/daemonsets.yaml" 2>/dev/null; then
        print_success "DaemonSets backed up"
    fi
    
    # Backup ConfigMaps
    if kubectl get configmaps -n "$NAMESPACE" -l app=powercap-manager -o yaml > "$BACKUP_DIR/configmaps.yaml" 2>/dev/null; then
        print_success "ConfigMaps backed up"
    fi
    
    # Backup RBAC
    if kubectl get clusterroles,clusterrolebindings,serviceaccounts -l app=powercap-manager -o yaml > "$BACKUP_DIR/rbac.yaml" 2>/dev/null; then
        print_success "RBAC resources backed up"
    fi
    
    # Backup node annotations
    kubectl get nodes -o json | jq '.items[] | {
        name: .metadata.name,
        annotations: (.metadata.annotations // {} | with_entries(select(.key | startswith("rapl/") or startswith("power-manager/"))))
    } | select(.annotations != {})' > "$BACKUP_DIR/node-annotations.json" 2>/dev/null || true
    
    print_success "Node annotations backed up"
}

# Function to clean Kubernetes resources
cleanup_kubernetes() {
    if [[ "$CLEANUP_KUBERNETES" == "false" ]]; then
        return 0
    fi
    
    print_info "Cleaning up Kubernetes resources..."
    
    # Stop DaemonSets
    execute_cmd \
        "kubectl delete daemonsets -n '$NAMESPACE' -l app=powercap-manager --ignore-not-found=true" \
        "Deleting PowerCap DaemonSets"
    
    # Clean ConfigMaps
    execute_cmd \
        "kubectl delete configmaps -n '$NAMESPACE' -l app=powercap-manager --ignore-not-found=true" \
        "Deleting PowerCap ConfigMaps"
    
    # Clean specific ConfigMaps that might not have labels
    for cm in powercap-config powercap-config-epex-fr powercap-config-epex-de powercap-config-mock powercap-config-static; do
        execute_cmd \
            "kubectl delete configmap '$cm' -n '$NAMESPACE' --ignore-not-found=true" \
            "Deleting ConfigMap: $cm"
    done
    
    # Clean ServiceAccounts
    execute_cmd \
        "kubectl delete serviceaccounts -n '$NAMESPACE' powercap-manager --ignore-not-found=true" \
        "Deleting PowerCap ServiceAccount"
    
    # Clean RBAC resources
    execute_cmd \
        "kubectl delete clusterrole powercap-manager --ignore-not-found=true" \
        "Deleting PowerCap ClusterRole"
    
    execute_cmd \
        "kubectl delete clusterrolebinding powercap-manager --ignore-not-found=true" \
        "Deleting PowerCap ClusterRoleBinding"
    
    # Clean node annotations
    cleanup_node_annotations
    
    print_success "Kubernetes cleanup completed"
}

# Function to clean node annotations
cleanup_node_annotations() {
    print_info "Cleaning node annotations..."
    
    local nodes
    if ! nodes=$(kubectl get nodes -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); then
        print_warning "Could not get node list"
        return 1
    fi
    
    for node in $nodes; do
        # Check if node has PowerCap annotations
        local has_annotations
        has_annotations=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations}' 2>/dev/null | grep -q "rapl\|power-manager" && echo "yes" || echo "no")
        
        if [[ "$has_annotations" == "yes" ]]; then
            print_info "Cleaning annotations from node: $node"
            
            # Remove RAPL annotations
            for annotation in rapl/pmax rapl/max_power_uw rapl/last-update rapl/provider rapl/market-period rapl/market-volume rapl/market-price; do
                execute_cmd \
                    "kubectl annotate node '$node' '$annotation-' --ignore-not-found=true" \
                    "Removing annotation: $annotation"
            done
            
            # Remove PowerManager annotations
            for annotation in power-manager/initialized power-manager/version; do
                execute_cmd \
                    "kubectl annotate node '$node' '$annotation-' --ignore-not-found=true" \
                    "Removing annotation: $annotation"
            done
        fi
    done
}

# Function to clean local data files
cleanup_local_data() {
    if [[ "$CLEANUP_LOCAL_DATA" == "false" ]]; then
        return 0
    fi
    
    print_info "Cleaning up local data files..."
    
    # Backup data files if requested
    if [[ "$BACKUP_ENABLED" == "true" && "$DRY_RUN" == "false" ]]; then
        mkdir -p "$BACKUP_DIR/data"
        find "$PROJECT_ROOT" -name "epex_data_*.csv" -exec cp {} "$BACKUP_DIR/data/" \; 2>/dev/null || true
        print_info "Data files backed up to $BACKUP_DIR/data/"
    fi
    
    # Remove CSV data files
    execute_cmd \
        "find '$PROJECT_ROOT' -name 'epex_data_*.csv' -delete" \
        "Removing EPEX data files"
    
    # Remove temporary files
    execute_cmd \
        "find '$PROJECT_ROOT' -name '*.tmp' -delete" \
        "Removing temporary files"
    
    # Remove backup files
    execute_cmd \
        "find '$PROJECT_ROOT' -name '*.bak' -delete" \
        "Removing backup files"
    
    print_success "Local data cleanup completed"
}

# Function to clean log files
cleanup_logs() {
    if [[ "$CLEANUP_LOGS" == "false" ]]; then
        return 0
    fi
    
    print_info "Cleaning up log files..."
    
    # Backup logs if requested
    if [[ "$BACKUP_ENABLED" == "true" && "$DRY_RUN" == "false" ]]; then
        mkdir -p "$BACKUP_DIR/logs"
        find "$PROJECT_ROOT" -name "*.log" -exec cp {} "$BACKUP_DIR/logs/" \; 2>/dev/null || true
    fi
    
    # Remove log files
    execute_cmd \
        "find '$PROJECT_ROOT' -name '*.log' -delete" \
        "Removing log files"
    
    # Clean systemd logs for PowerCap (if any)
    execute_cmd \
        "sudo journalctl --vacuum-time=1d --unit=powercap* 2>/dev/null || true" \
        "Cleaning systemd logs"
    
    print_success "Log cleanup completed"
}

# Function to reset RAPL settings
cleanup_rapl() {
    if [[ "$CLEANUP_RAPL" == "false" ]]; then
        return 0
    fi
    
    print_info "Resetting RAPL power settings..."
    
    # Find RAPL domains
    local rapl_domains
    rapl_domains=$(find /sys/class/powercap -name "intel-rapl:*" -type d 2>/dev/null || true)
    
    if [[ -z "$rapl_domains" ]]; then
        print_warning "No RAPL domains found"
        return 0
    fi
    
    for domain in $rapl_domains; do
        if [[ -f "$domain/constraint_0_power_limit_uw" ]]; then
            # Get max power limit
            local max_power
            if max_power=$(cat "$domain/constraint_0_max_power_uw" 2>/dev/null); then
                execute_cmd \
                    "echo '$max_power' | sudo tee '$domain/constraint_0_power_limit_uw' > /dev/null" \
                    "Resetting power limit for $(basename "$domain")"
            fi
        fi
        
        # Reset time window to default (if writable)
        if [[ -w "$domain/constraint_0_time_window_us" ]]; then
            execute_cmd \
                "echo '976562' | sudo tee '$domain/constraint_0_time_window_us' > /dev/null" \
                "Resetting time window for $(basename "$domain")"
        fi
    done
    
    print_success "RAPL cleanup completed"
}

# Function to clean Docker resources
cleanup_docker() {
    if [[ "$CLEANUP_DOCKER" == "false" ]]; then
        return 0
    fi
    
    print_info "Cleaning up Docker resources..."
    
    # Stop and remove PowerCap containers
    execute_cmd \
        "docker ps -a --filter 'ancestor=powercap-manager' --format '{{.ID}}' | xargs -r docker rm -f" \
        "Removing PowerCap containers"
    
    # Remove PowerCap images
    execute_cmd \
        "docker images 'powercap-manager' --format '{{.ID}}' | xargs -r docker rmi -f" \
        "Removing PowerCap images"
    
    # Clean up dangling images
    execute_cmd \
        "docker image prune -f" \
        "Cleaning up dangling images"
    
    print_success "Docker cleanup completed"
}

# Function to show cleanup summary
show_cleanup_summary() {
    print_info "Cleanup Summary:"
    echo "=================="
    
    if [[ "$CLEANUP_KUBERNETES" == "true" ]]; then
        echo "✓ Kubernetes resources"
    fi
    
    if [[ "$CLEANUP_LOCAL_DATA" == "true" ]]; then
        echo "✓ Local data files"
    fi
    
    if [[ "$CLEANUP_RAPL" == "true" ]]; then
        echo "✓ RAPL settings reset"
    fi
    
    if [[ "$CLEANUP_LOGS" == "true" ]]; then
        echo "✓ Log files"
    fi
    
    if [[ "$CLEANUP_DOCKER" == "true" ]]; then
        echo "✓ Docker resources"
    fi
    
    if [[ "$BACKUP_ENABLED" == "true" && "$DRY_RUN" == "false" ]]; then
        echo ""
        print_info "Backups saved to: $BACKUP_DIR"
    fi
    
    echo ""
}

# Function to get user confirmation
get_confirmation() {
    if [[ "$FORCE" == "true" || "$DRY_RUN" == "true" ]]; then
        return 0
    fi
    
    echo ""
    print_warning "This will clean up PowerCap Manager resources."
    
    if [[ "$BACKUP_ENABLED" == "false" ]]; then
        print_warning "No backups will be created!"
    fi
    
    show_cleanup_summary
    
    echo ""
    read -p "Do you want to continue? (y/N): " -n 1 -r
    echo ""
    
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Cleanup cancelled by user"
        exit 0
    fi
}

# Function to check prerequisites
check_prerequisites() {
    local missing_deps=()
    
    # Check for kubectl
    if ! command -v kubectl &> /dev/null; then
        missing_deps+=("kubectl")
    fi
    
    # Check for jq (for backups)
    if [[ "$BACKUP_ENABLED" == "true" ]] && ! command -v jq &> /dev/null; then
        print_warning "jq not found - node annotation backups will be skipped"
    fi
    
    # Check for docker (if cleaning docker)
    if [[ "$CLEANUP_DOCKER" == "true" ]] && ! command -v docker &> /dev/null; then
        missing_deps+=("docker")
    fi
    
    if [[ ${#missing_deps[@]} -gt 0 ]]; then
        print_error "Missing required dependencies: ${missing_deps[*]}"
        exit 1
    fi
    
    # Check Kubernetes connectivity
    if [[ "$CLEANUP_KUBERNETES" == "true" ]]; then
        if ! kubectl cluster-info &> /dev/null; then
            print_error "Cannot connect to Kubernetes cluster"
            exit 1
        fi
    fi
}

# Function to create restore script
create_restore_script() {
    if [[ "$BACKUP_ENABLED" == "false" || "$DRY_RUN" == "true" ]]; then
        return 0
    fi
    
    local restore_script="$BACKUP_DIR/restore.sh"
    
    cat > "$restore_script" << 'EOF'
#!/bin/bash
# PowerCap Manager - Restore Script
# Generated automatically during cleanup

set -euo pipefail

BACKUP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "🔄 Restoring PowerCap Manager from backup..."
echo "Backup directory: $BACKUP_DIR"

# Restore Kubernetes resources
if [[ -f "$BACKUP_DIR/daemonsets.yaml" ]]; then
    echo "Restoring DaemonSets..."
    kubectl apply -f "$BACKUP_DIR/daemonsets.yaml"
fi

if [[ -f "$BACKUP_DIR/configmaps.yaml" ]]; then
    echo "Restoring ConfigMaps..."
    kubectl apply -f "$BACKUP_DIR/configmaps.yaml"
fi

if [[ -f "$BACKUP_DIR/rbac.yaml" ]]; then
    echo "Restoring RBAC resources..."
    kubectl apply -f "$BACKUP_DIR/rbac.yaml"
fi

# Restore node annotations
if [[ -f "$BACKUP_DIR/node-annotations.json" ]]; then
    echo "Note: Node annotations need to be restored manually"
    echo "Backup file: $BACKUP_DIR/node-annotations.json"
fi

# Restore data files
if [[ -d "$BACKUP_DIR/data" ]]; then
    echo "Data files available for restore in: $BACKUP_DIR/data"
fi

echo "✅ Restore completed!"
echo "💡 You may need to restart the PowerCap Manager DaemonSet"
EOF
    
    chmod +x "$restore_script"
    print_success "Restore script created: $restore_script"
}

# Main execution function
main() {
    parse_args "$@"
    
    # Print header
    echo ""
    print_info "PowerCap Manager - Cleanup Script"
    print_info "=================================="
    
    if [[ "$DRY_RUN" == "true" ]]; then
        print_warning "DRY RUN MODE - No changes will be made"
    fi
    
    echo ""
    
    # Check prerequisites
    check_prerequisites
    
    # Get user confirmation
    get_confirmation
    
    # Create backup directory
    create_backup_dir
    
    # Backup resources before cleanup
    backup_kubernetes_resources
    
    # Perform cleanup
    cleanup_kubernetes
    cleanup_local_data
    cleanup_logs
    cleanup_rapl
    cleanup_docker
    
    # Create restore script
    create_restore_script
    
    # Show summary
    echo ""
    print_success "PowerCap Manager cleanup completed!"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        show_cleanup_summary
    fi
}

# Run main function with all arguments
main "$@"