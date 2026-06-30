#!/bin/bash
set -euo pipefail

# Platformctl Monitoring Script
# Provides health checks and monitoring for deployed services

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
ENVIRONMENT="${ENVIRONMENT:-development}"
NAMESPACE="${NAMESPACE:-}"
WATCH="${WATCH:-false}"
INTERVAL="${INTERVAL:-30}"
OUTPUT_FORMAT="${OUTPUT_FORMAT:-table}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Logging functions
log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# Help function
show_help() {
    cat << EOF
Platformctl Monitoring Script

Usage: $0 [OPTIONS] [COMMAND]

OPTIONS:
    -h, --help              Show this help message
    -e, --environment ENV   Target environment (development|staging|production)
    -n, --namespace NAME    Target namespace (optional)
    -w, --watch             Watch mode - continuously monitor
    -i, --interval SEC      Watch interval in seconds (default: 30)
    -f, --format FORMAT     Output format (table|json|yaml) (default: table)

COMMANDS:
    status      Show overall system status (default)
    pods        Show pod status and health
    services    Show service endpoints and connectivity
    metrics     Show metrics and performance data
    logs        Show recent logs from all services
    health      Perform comprehensive health checks

EXAMPLES:
    $0                              # Show status for development
    $0 -e production status         # Show production status
    $0 -w -i 10 pods               # Watch pod status every 10 seconds
    $0 -e staging health           # Perform health checks for staging
    $0 logs -e production          # Show production logs

EOF
}

# Parse arguments
COMMAND="status"
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -e|--environment)
            ENVIRONMENT="$2"
            shift 2
            ;;
        -n|--namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        -w|--watch)
            WATCH=true
            shift
            ;;
        -i|--interval)
            INTERVAL="$2"
            shift 2
            ;;
        -f|--format)
            OUTPUT_FORMAT="$2"
            shift 2
            ;;
        status|pods|services|metrics|logs|health)
            COMMAND="$1"
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Set namespace if not provided
if [[ -z "$NAMESPACE" ]]; then
    case $ENVIRONMENT in
        development) NAMESPACE="platformctl-dev" ;;
        staging) NAMESPACE="platformctl-staging" ;;
        production) NAMESPACE="platformctl-prod" ;;
        *) NAMESPACE="platformctl" ;;
    esac
fi

# Check prerequisites
check_prerequisites() {
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required but not installed"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace $NAMESPACE does not exist"
        exit 1
    fi
}

# Show overall system status
show_status() {
    log_info "Platformctl System Status - $ENVIRONMENT"
    log_info "Namespace: $NAMESPACE"
    echo
    
    # Deployment status
    echo "=== Deployments ==="
    kubectl get deployments -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o wide
    echo
    
    # Service status
    echo "=== Services ==="
    kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o wide
    echo
    
    # Pod summary
    echo "=== Pod Summary ==="
    local total_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers | wc -l)
    local running_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --field-selector=status.phase=Running --no-headers | wc -l)
    local pending_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --field-selector=status.phase=Pending --no-headers | wc -l)
    local failed_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --field-selector=status.phase=Failed --no-headers | wc -l)
    
    echo "Total: $total_pods | Running: $running_pods | Pending: $pending_pods | Failed: $failed_pods"
    
    if [[ $failed_pods -gt 0 ]]; then
        echo
        echo "=== Failed Pods ==="
        kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --field-selector=status.phase=Failed
    fi
}

# Show detailed pod status
show_pods() {
    log_info "Pod Status - $ENVIRONMENT"
    echo
    
    case $OUTPUT_FORMAT in
        json)
            kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o json
            ;;
        yaml)
            kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o yaml
            ;;
        *)
            kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o wide
            
            # Show resource usage if metrics-server is available
            if kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform &> /dev/null; then
                echo
                echo "=== Resource Usage ==="
                kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform
            fi
            ;;
    esac
}

# Show service information
show_services() {
    log_info "Service Information - $ENVIRONMENT"
    echo
    
    case $OUTPUT_FORMAT in
        json)
            kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o json
            ;;
        yaml)
            kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o yaml
            ;;
        *)
            kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o wide
            echo
            
            # Show endpoints
            echo "=== Service Endpoints ==="
            kubectl get endpoints -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform
            ;;
    esac
}

# Show metrics information
show_metrics() {
    log_info "Metrics Information - $ENVIRONMENT"
    echo
    
    # Check if metrics-server is available
    if kubectl top nodes &> /dev/null; then
        echo "=== Node Metrics ==="
        kubectl top nodes
        echo
        
        echo "=== Pod Metrics ==="
        kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --sort-by=cpu
    else
        log_warn "Metrics server not available"
    fi
    
    # Show HPA status if available
    local hpas=$(kubectl get hpa -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
    if [[ $hpas -gt 0 ]]; then
        echo
        echo "=== Horizontal Pod Autoscalers ==="
        kubectl get hpa -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform
    fi
}

# Show recent logs
show_logs() {
    log_info "Recent Logs - $ENVIRONMENT"
    echo
    
    # Get all pods
    local pods=($(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o jsonpath='{.items[*].metadata.name}'))
    
    if [[ ${#pods[@]} -eq 0 ]]; then
        log_warn "No Platformctl pods found"
        return 0
    fi
    
    for pod in "${pods[@]}"; do
        echo "=== Logs from $pod ==="
        kubectl logs "$pod" -n "$NAMESPACE" --tail=20 --timestamps
        echo
    done
}

# Perform comprehensive health checks
perform_health_checks() {
    log_info "Performing Health Checks - $ENVIRONMENT"
    echo
    
    local health_issues=0
    
    # Check pod health
    echo "=== Pod Health Check ==="
    local unhealthy_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{" "}{.status.containerStatuses[0].ready}{"\n"}{end}' | grep -v "Running true" | wc -l)
    
    if [[ $unhealthy_pods -eq 0 ]]; then
        log_success "All pods are healthy"
    else
        log_error "$unhealthy_pods pods are unhealthy"
        kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform | grep -v "Running"
        health_issues=$((health_issues + 1))
    fi
    
    # Check service endpoints
    echo
    echo "=== Service Endpoint Check ==="
    local services_without_endpoints=$(kubectl get endpoints -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.subsets[*].addresses[*].ip}{"\n"}{end}' | grep -c "^[^ ]* *$" || true)
    
    if [[ $services_without_endpoints -eq 0 ]]; then
        log_success "All services have healthy endpoints"
    else
        log_error "$services_without_endpoints services have no endpoints"
        health_issues=$((health_issues + 1))
    fi
    
    # Test gateway health endpoint if possible
    echo
    echo "=== Gateway Health Check ==="
    local gateway_pod=$(kubectl get pods -n "$NAMESPACE" -l app=gateway -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
    
    if [[ -n "$gateway_pod" ]]; then
        if kubectl exec "$gateway_pod" -n "$NAMESPACE" -- curl -s http://localhost:8081/health > /dev/null 2>&1; then
            log_success "Gateway health endpoint is responding"
        else
            log_warn "Gateway health endpoint is not responding"
            health_issues=$((health_issues + 1))
        fi
    else
        log_warn "Gateway pod not found"
        health_issues=$((health_issues + 1))
    fi
    
    # Check resource usage
    echo
    echo "=== Resource Usage Check ==="
    if kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform &> /dev/null; then
        local high_cpu_pods=$(kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers | awk '{if($2 ~ /[0-9]+m/ && $2+0 > 800) print $1}' | wc -l)
        local high_memory_pods=$(kubectl top pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers | awk '{if($3 ~ /[0-9]+Mi/ && $3+0 > 800) print $1}' | wc -l)
        
        if [[ $high_cpu_pods -eq 0 && $high_memory_pods -eq 0 ]]; then
            log_success "Resource usage is within normal limits"
        else
            log_warn "High resource usage detected (CPU: $high_cpu_pods pods, Memory: $high_memory_pods pods)"
        fi
    else
        log_warn "Resource metrics not available"
    fi
    
    # Overall health summary
    echo
    echo "=== Health Summary ==="
    if [[ $health_issues -eq 0 ]]; then
        log_success "All health checks passed!"
    else
        log_error "$health_issues health issues detected"
        exit 1
    fi
}

# Watch mode execution
run_watch_mode() {
    log_info "Starting watch mode (interval: ${INTERVAL}s)"
    log_info "Press Ctrl+C to exit"
    
    while true; do
        clear
        echo "Platformctl Monitoring - $(date)"
        echo "Environment: $ENVIRONMENT | Namespace: $NAMESPACE"
        echo "========================================"
        
        case $COMMAND in
            status) show_status ;;
            pods) show_pods ;;
            services) show_services ;;
            metrics) show_metrics ;;
            health) perform_health_checks ;;
        esac
        
        echo
        echo "Next update in ${INTERVAL}s... (Press Ctrl+C to exit)"
        sleep "$INTERVAL"
    done
}

# Main execution
main() {
    check_prerequisites
    
    if [[ "$WATCH" == "true" ]]; then
        run_watch_mode
    else
        case $COMMAND in
            status) show_status ;;
            pods) show_pods ;;
            services) show_services ;;
            metrics) show_metrics ;;
            logs) show_logs ;;
            health) perform_health_checks ;;
            *) 
                log_error "Unknown command: $COMMAND"
                show_help
                exit 1
                ;;
        esac
    fi
}

# Run main function
main "$@"