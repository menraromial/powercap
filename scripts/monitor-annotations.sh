#!/bin/bash
# Script to monitor PowerCap annotations on Kubernetes nodes

echo "🔍 PowerCap Manager - Node Annotations Monitor"
echo "=============================================="

# Function to display annotations in a readable format
display_annotations() {
    local node=$1
    echo "📊 Node: $node"
    echo "├─ Power Information:"
    
    # Get power-related annotations
    local pmax=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/pmax}' 2>/dev/null)
    local last_update=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/last-update}' 2>/dev/null)
    local provider=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/provider}' 2>/dev/null)
    local max_power=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/max_power_uw}' 2>/dev/null)
    
    # Market data annotations
    local market_period=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/market-period}' 2>/dev/null)
    local market_volume=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/market-volume}' 2>/dev/null)
    local market_price=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.rapl/market-price}' 2>/dev/null)
    
    # Status annotations
    local initialized=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations.power-manager/initialized}' 2>/dev/null)
    
    if [[ -n "$pmax" ]]; then
        local power_watts=$((pmax / 1000000))
        echo "│  ├─ Current Power Limit: ${pmax} µW (${power_watts} W)"
    else
        echo "│  ├─ Current Power Limit: Not set"
    fi
    
    if [[ -n "$max_power" ]]; then
        local max_watts=$((max_power / 1000000))
        echo "│  ├─ Maximum Power: ${max_power} µW (${max_watts} W)"
    fi
    
    if [[ -n "$provider" ]]; then
        echo "│  ├─ Data Provider: ${provider}"
    fi
    
    if [[ -n "$last_update" ]]; then
        echo "│  ├─ Last Update: ${last_update}"
    fi
    
    if [[ -n "$initialized" ]]; then
        echo "│  ├─ Status: Initialized ✅"
    else
        echo "│  ├─ Status: Not initialized ❌"
    fi
    
    echo "├─ Market Information:"
    if [[ -n "$market_period" ]]; then
        echo "│  ├─ Current Period: ${market_period}"
        if [[ -n "$market_volume" ]]; then
            echo "│  ├─ Market Volume: ${market_volume} MWh"
        fi
        if [[ -n "$market_price" ]]; then
            echo "│  ├─ Market Price: ${market_price} €/MWh"
        fi
    else
        echo "│  └─ No market data available"
    fi
    
    echo "└─"
}

# Function to monitor annotations in real-time
monitor_annotations() {
    echo "🔄 Starting real-time monitoring (Ctrl+C to stop)..."
    echo ""
    
    while true; do
        clear
        echo "🔍 PowerCap Manager - Real-time Monitor"
        echo "======================================="
        echo "$(date '+%Y-%m-%d %H:%M:%S')"
        echo ""
        
        # Get all nodes with PowerCap annotations
        local nodes=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}')
        
        for node in $nodes; do
            # Check if node has PowerCap annotations
            local has_powercap=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations}' 2>/dev/null | grep -q "rapl/" && echo "yes" || echo "no")
            
            if [[ "$has_powercap" == "yes" ]]; then
                display_annotations "$node"
                echo ""
            fi
        done
        
        if [[ -z "$(echo $nodes)" ]]; then
            echo "❌ No nodes with PowerCap annotations found"
            echo ""
        fi
        
        echo "Next update in 10 seconds..."
        sleep 10
    done
}

# Function to export annotations to JSON
export_annotations() {
    echo "📤 Exporting PowerCap annotations to JSON..."
    
    local output_file="powercap-annotations-$(date +%Y%m%d-%H%M%S).json"
    
    kubectl get nodes -o json | jq '.items[] | {
        name: .metadata.name,
        annotations: (.metadata.annotations // {} | with_entries(select(.key | startswith("rapl/") or startswith("power-manager/"))))
    } | select(.annotations != {})' > "$output_file"
    
    echo "✅ Annotations exported to: $output_file"
    
    # Display summary
    local node_count=$(jq 'select(.annotations != {}) | .name' "$output_file" | wc -l)
    echo "📊 Found $node_count nodes with PowerCap annotations"
}

# Function to show help
show_help() {
    echo "PowerCap Manager - Node Annotations Monitor"
    echo ""
    echo "Usage: $0 [COMMAND]"
    echo ""
    echo "Commands:"
    echo "  list       List all nodes with PowerCap annotations (default)"
    echo "  monitor    Real-time monitoring of annotations"
    echo "  export     Export annotations to JSON file"
    echo "  help       Show this help message"
    echo ""
    echo "Examples:"
    echo "  $0              # List current annotations"
    echo "  $0 monitor      # Start real-time monitoring"
    echo "  $0 export       # Export to JSON"
    echo ""
}

# Main execution
case "${1:-list}" in
    "list")
        echo ""
        nodes=$(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name}{" "}{end}')
        
        found_powercap=false
        for node in $nodes; do
            has_powercap=$(kubectl get node "$node" -o jsonpath='{.metadata.annotations}' 2>/dev/null | grep -q "rapl/" && echo "yes" || echo "no")
            
            if [[ "$has_powercap" == "yes" ]]; then
                display_annotations "$node"
                echo ""
                found_powercap=true
            fi
        done
        
        if [[ "$found_powercap" == "false" ]]; then
            echo "❌ No nodes with PowerCap annotations found"
            echo ""
            echo "💡 Make sure PowerCap Manager is running on your nodes"
        fi
        ;;
    "monitor")
        monitor_annotations
        ;;
    "export")
        export_annotations
        ;;
    "help"|"-h"|"--help")
        show_help
        ;;
    *)
        echo "❌ Unknown command: $1"
        echo ""
        show_help
        exit 1
        ;;
esac