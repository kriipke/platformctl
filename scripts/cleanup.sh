#!/bin/bash
set -euo pipefail

# Platformctl Cleanup Script
# Handles cleanup and uninstallation of Platformctl deployments

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments"

# Default values
ENVIRONMENT="${ENVIRONMENT:-development}"
NAMESPACE="${NAMESPACE:-}"
FORCE="${FORCE:-false}"
KEEP_DATA="${KEEP_DATA:-true}"
DRY_RUN="${DRY_RUN:-false}"

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
Platformctl Cleanup Script

Usage: $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -e, --environment ENV   Target environment (development|staging|production)
    -n, --namespace NAME    Target namespace (optional)
    -f, --force             Force cleanup without confirmation
    --delete-data           Delete persistent data (PVCs, secrets)
    -d, --dry-run           Show what would be deleted without deleting

EXAMPLES:
    $0 -e development               # Clean up development environment
    $0 -e staging --delete-data     # Clean up staging and delete all data
    $0 -f -e production            # Force cleanup production (dangerous!)
    $0 -d -e production            # Dry run cleanup for production

WARNING:
    This script will delete Platformctl deployments and optionally their data.
    Use with caution, especially in production environments.

EOF
}

# Parse arguments
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
        -f|--force)
            FORCE=true
            shift
            ;;
        --delete-data)
            KEEP_DATA=false
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
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
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required but not installed"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Confirm cleanup
confirm_cleanup() {
    if [[ "$FORCE" == "true" || "$DRY_RUN" == "true" ]]; then
        return 0
    fi
    
    log_warn "You are about to clean up Platformctl in $ENVIRONMENT environment!"
    log_warn "Namespace: $NAMESPACE"
    log_warn "Keep data: $KEEP_DATA"
    
    if [[ "$ENVIRONMENT" == "production" ]]; then
        log_error "PRODUCTION CLEANUP DETECTED!"
        echo -n "Type 'DELETE PRODUCTION' to confirm: "
        read -r confirmation
        if [[ "$confirmation" != "DELETE PRODUCTION" ]]; then
            log_info "Cleanup cancelled"
            exit 0
        fi
    else
        read -p "Are you sure you want to continue? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy]es$ ]]; then
            log_info "Cleanup cancelled"
            exit 0
        fi
    fi
}

# Show what will be deleted
show_cleanup_plan() {
    log_info "Cleanup plan for $ENVIRONMENT environment:"
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_warn "Namespace $NAMESPACE does not exist - nothing to clean up"
        return 0
    fi
    
    # Show deployments
    local deployments=$(kubectl get deployments -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
    log_info "- Deployments to delete: $deployments"
    
    # Show services
    local services=$(kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
    log_info "- Services to delete: $services"
    
    # Show configmaps
    local configmaps=$(kubectl get configmaps -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
    log_info "- ConfigMaps to delete: $configmaps"
    
    # Show secrets
    local secrets=$(kubectl get secrets -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
    log_info "- Secrets to delete: $secrets"
    
    if [[ "$KEEP_DATA" == "false" ]]; then
        # Show PVCs
        local pvcs=$(kubectl get pvc -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
        log_info "- PersistentVolumeClaims to delete: $pvcs"
        log_warn "  This will DELETE ALL PERSISTENT DATA!"
    else
        log_info "- PersistentVolumeClaims: KEEPING (use --delete-data to remove)"
    fi
}

# Delete Platformctl resources
delete_platformctl_resources() {
    log_info "Deleting Platformctl resources..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would delete the following resources:"
        
        if kubectl get namespace "$NAMESPACE" &> /dev/null; then
            # Show what would be deleted
            kubectl get all,cm,secrets,pvc,ingress,networkpolicies -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform 2>/dev/null || true
            
            if [[ "$KEEP_DATA" == "false" ]]; then
                log_info "Would also delete PersistentVolumeClaims"
            fi
        fi
        return 0
    fi
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace $NAMESPACE does not exist - nothing to delete"
        return 0
    fi
    
    # Delete applications first (graceful shutdown)
    log_info "Deleting deployments..."
    kubectl delete deployments -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --wait=true --timeout=300s || log_warn "Some deployments failed to delete gracefully"
    
    # Delete other resources
    log_info "Deleting services..."
    kubectl delete services -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    
    log_info "Deleting ingresses..."
    kubectl delete ingress -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    
    log_info "Deleting network policies..."
    kubectl delete networkpolicies -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    
    log_info "Deleting configmaps..."
    kubectl delete configmaps -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    
    log_info "Deleting secrets..."
    kubectl delete secrets -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    
    # Delete PVCs if requested
    if [[ "$KEEP_DATA" == "false" ]]; then
        log_warn "Deleting persistent data..."
        kubectl delete pvc -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform || true
    fi
    
    log_success "Resource deletion completed"
}

# Clean up cluster-wide resources
cleanup_cluster_resources() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would clean up cluster-wide resources"
        return 0
    fi
    
    log_info "Cleaning up cluster-wide resources..."
    
    # Remove ClusterRoleBindings
    kubectl delete clusterrolebinding -l app.kubernetes.io/part-of=platformctl-platform || true
    
    # Remove ClusterRoles (be careful not to delete system roles)
    kubectl delete clusterrole platformctl-kubernetes-reader platformctl-aggregator || true
    
    log_info "Cluster-wide cleanup completed"
}

# Remove namespace if empty
cleanup_namespace() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would check if namespace should be deleted"
        return 0
    fi
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_info "Namespace $NAMESPACE does not exist"
        return 0
    fi
    
    # Check if namespace has any remaining resources
    local remaining_resources=$(kubectl get all,cm,secrets,pvc,ingress -n "$NAMESPACE" --no-headers 2>/dev/null | wc -l)
    
    if [[ $remaining_resources -eq 0 ]]; then
        log_info "Namespace $NAMESPACE is empty, deleting..."
        kubectl delete namespace "$NAMESPACE" || log_warn "Failed to delete namespace"
    else
        log_info "Namespace $NAMESPACE still has resources, keeping it"
        log_info "Remaining resources: $remaining_resources"
    fi
}

# Show cleanup summary
show_summary() {
    log_info "Cleanup Summary:"
    log_info "- Environment: $ENVIRONMENT"
    log_info "- Namespace: $NAMESPACE"
    log_info "- Data deleted: $([ "$KEEP_DATA" == "false" ] && echo "YES" || echo "NO")"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        # Check if any Platformctl resources remain
        if kubectl get namespace "$NAMESPACE" &> /dev/null; then
            local remaining=$(kubectl get all,cm,secrets,pvc -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform --no-headers 2>/dev/null | wc -l)
            if [[ $remaining -gt 0 ]]; then
                log_warn "$remaining Platformctl resources still exist"
                kubectl get all,cm,secrets,pvc -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform 2>/dev/null || true
            else
                log_success "All Platformctl resources have been cleaned up"
            fi
        else
            log_success "Namespace $NAMESPACE has been completely removed"
        fi
    fi
}

# Main execution
main() {
    log_info "Platformctl Cleanup Script"
    log_info "Environment: $ENVIRONMENT"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "DRY RUN MODE - No actual deletion will occur"
    fi
    
    check_prerequisites
    show_cleanup_plan
    confirm_cleanup
    delete_platformctl_resources
    cleanup_cluster_resources
    cleanup_namespace
    show_summary
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_success "Dry run completed successfully!"
    else
        log_success "Cleanup completed successfully!"
    fi
}

# Run main function
main "$@"