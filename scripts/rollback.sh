#!/bin/bash
set -euo pipefail

# Platformctl Rollback Script
# Handles rolling back deployments to previous versions

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values
ENVIRONMENT="${ENVIRONMENT:-development}"
NAMESPACE="${NAMESPACE:-}"
REVISION="${REVISION:-}"
SERVICE="${SERVICE:-all}"
DRY_RUN="${DRY_RUN:-false}"
TIMEOUT="${TIMEOUT:-300s}"

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
Platformctl Rollback Script

Usage: $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -e, --environment ENV   Target environment (development|staging|production)
    -n, --namespace NAME    Target namespace (optional)
    -r, --revision REV      Specific revision number to rollback to
    -s, --service NAME      Specific service to rollback (default: all)
    -d, --dry-run           Show what would be rolled back
    --timeout DURATION      Rollback timeout (default: 300s)

EXAMPLES:
    $0 -e staging                          # Rollback all services to previous version
    $0 -e production -s gateway            # Rollback only gateway service
    $0 -e staging -r 3                     # Rollback to specific revision 3
    $0 -d -e production                    # Dry run rollback for production

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
        -r|--revision)
            REVISION="$2"
            shift 2
            ;;
        -s|--service)
            SERVICE="$2"
            shift 2
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
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

# Validate prerequisites
validate_prerequisites() {
    log_info "Validating prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required but not installed"
        exit 1
    fi
    
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        exit 1
    fi
    
    # Check if namespace exists
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace $NAMESPACE does not exist"
        exit 1
    fi
    
    log_success "Prerequisites validation passed"
}

# Get deployments to rollback
get_deployments() {
    log_info "Getting deployments in namespace $NAMESPACE..."
    
    if [[ "$SERVICE" == "all" ]]; then
        DEPLOYMENTS=($(kubectl get deployments -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o name))
    else
        # Find deployment with matching service name
        DEPLOYMENTS=($(kubectl get deployments -n "$NAMESPACE" -l app="$SERVICE" -o name))
        
        if [[ ${#DEPLOYMENTS[@]} -eq 0 ]]; then
            log_error "No deployment found for service: $SERVICE"
            exit 1
        fi
    fi
    
    if [[ ${#DEPLOYMENTS[@]} -eq 0 ]]; then
        log_error "No Platformctl deployments found in namespace $NAMESPACE"
        exit 1
    fi
    
    log_info "Found ${#DEPLOYMENTS[@]} deployments to rollback:"
    printf '%s\n' "${DEPLOYMENTS[@]}"
}

# Show rollout history
show_history() {
    log_info "Rollout history for deployments:"
    
    for deployment in "${DEPLOYMENTS[@]}"; do
        log_info "History for $deployment:"
        kubectl rollout history "$deployment" -n "$NAMESPACE"
        echo
    done
}

# Confirm rollback
confirm_rollback() {
    if [[ "$ENVIRONMENT" == "production" && "$DRY_RUN" != "true" ]]; then
        log_warn "You are about to rollback PRODUCTION environment!"
        read -p "Are you sure you want to continue? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy]es$ ]]; then
            log_info "Rollback cancelled"
            exit 0
        fi
    fi
}

# Perform rollback
perform_rollback() {
    log_info "Starting rollback process..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would rollback the following deployments:"
        for deployment in "${DEPLOYMENTS[@]}"; do
            if [[ -n "$REVISION" ]]; then
                log_info "Would rollback $deployment to revision $REVISION"
            else
                log_info "Would rollback $deployment to previous revision"
            fi
        done
        return 0
    fi
    
    local failed_rollbacks=()
    local successful_rollbacks=()
    
    for deployment in "${DEPLOYMENTS[@]}"; do
        log_info "Rolling back $deployment..."
        
        if [[ -n "$REVISION" ]]; then
            if kubectl rollout undo "$deployment" -n "$NAMESPACE" --to-revision="$REVISION"; then
                successful_rollbacks+=("$deployment")
            else
                failed_rollbacks+=("$deployment")
                log_error "Failed to rollback $deployment"
            fi
        else
            if kubectl rollout undo "$deployment" -n "$NAMESPACE"; then
                successful_rollbacks+=("$deployment")
            else
                failed_rollbacks+=("$deployment")
                log_error "Failed to rollback $deployment"
            fi
        fi
    done
    
    # Report results
    if [[ ${#successful_rollbacks[@]} -gt 0 ]]; then
        log_success "Successfully initiated rollback for: ${successful_rollbacks[*]}"
    fi
    
    if [[ ${#failed_rollbacks[@]} -gt 0 ]]; then
        log_error "Failed to rollback: ${failed_rollbacks[*]}"
        exit 1
    fi
}

# Wait for rollback completion
wait_for_rollback() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would wait for rollback to complete"
        return 0
    fi
    
    log_info "Waiting for rollback to complete..."
    
    local failed_deployments=()
    
    for deployment in "${DEPLOYMENTS[@]}"; do
        log_info "Waiting for $deployment rollback to complete..."
        
        if kubectl rollout status "$deployment" -n "$NAMESPACE" --timeout="$TIMEOUT"; then
            log_success "Rollback completed for $deployment"
        else
            failed_deployments+=("$deployment")
            log_error "Rollback timeout for $deployment"
        fi
    done
    
    if [[ ${#failed_deployments[@]} -gt 0 ]]; then
        log_error "Rollback failed or timed out for: ${failed_deployments[*]}"
        exit 1
    fi
}

# Verify rollback
verify_rollback() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would verify rollback success"
        return 0
    fi
    
    log_info "Verifying rollback success..."
    
    # Check pod status
    local unhealthy_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{"\n"}{end}' | grep -v "Running" | wc -l)
    
    if [[ $unhealthy_pods -gt 0 ]]; then
        log_warn "$unhealthy_pods pods are not running after rollback"
        kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform
    else
        log_success "All pods are running after rollback"
    fi
    
    # Show current deployment status
    log_info "Current deployment status:"
    for deployment in "${DEPLOYMENTS[@]}"; do
        kubectl get "$deployment" -n "$NAMESPACE"
    done
}

# Show summary
show_summary() {
    log_info "Rollback Summary:"
    log_info "- Environment: $ENVIRONMENT"
    log_info "- Namespace: $NAMESPACE"
    log_info "- Service: $SERVICE"
    log_info "- Revision: ${REVISION:-previous}"
    log_info "- Deployments affected: ${#DEPLOYMENTS[@]}"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        log_info ""
        log_info "Updated deployments:"
        kubectl get deployments -n "$NAMESPACE" -l app.kubernetes.io/part-of=platformctl-platform
    fi
}

# Main execution
main() {
    log_info "Platformctl Rollback Script"
    log_info "Environment: $ENVIRONMENT"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "DRY RUN MODE - No actual rollback will occur"
    fi
    
    validate_prerequisites
    get_deployments
    show_history
    confirm_rollback
    perform_rollback
    wait_for_rollback
    verify_rollback
    show_summary
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_success "Dry run completed successfully!"
    else
        log_success "Rollback completed successfully!"
    fi
}

# Run main function
main "$@"