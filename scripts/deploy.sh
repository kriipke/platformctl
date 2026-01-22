#!/bin/bash
set -euo pipefail

# ContextOps Deployment Script
# Automates deployment of ContextOps GitOps monitoring platform across environments

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
DEPLOYMENTS_DIR="$PROJECT_ROOT/deployments"

# Default values
ENVIRONMENT="${ENVIRONMENT:-development}"
NAMESPACE="${NAMESPACE:-}"
DRY_RUN="${DRY_RUN:-false}"
SKIP_BUILD="${SKIP_BUILD:-false}"
SKIP_TESTS="${SKIP_TESTS:-false}"
TIMEOUT="${TIMEOUT:-600s}"
FORCE="${FORCE:-false}"

# Available environments
ENVIRONMENTS=("development" "staging" "production")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
ContextOps Deployment Script

Usage: $0 [OPTIONS]

OPTIONS:
    -h, --help              Show this help message
    -e, --environment ENV   Set target environment (development|staging|production)
    -n, --namespace NAME    Override default namespace
    -d, --dry-run           Show what would be deployed without deploying
    -f, --force             Force deployment even if validations fail
    --skip-build            Skip building container images
    --skip-tests            Skip running pre-deployment tests
    --timeout DURATION      Deployment timeout (default: 600s)

EXAMPLES:
    $0                                    # Deploy to development
    $0 -e staging                         # Deploy to staging
    $0 -e production -f                   # Force deploy to production
    $0 -d -e production                   # Dry run for production
    $0 --skip-build -e development        # Deploy without rebuilding images

ENVIRONMENTS:
    development    - Development environment with debug features
    staging        - Staging environment for testing release candidates  
    production     - Production environment with full security and monitoring

EOF
}

# Parse command line arguments
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
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        --skip-build)
            SKIP_BUILD=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
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

# Validate environment
validate_environment() {
    log_info "Validating environment: $ENVIRONMENT"
    
    if [[ ! " ${ENVIRONMENTS[*]} " =~ " ${ENVIRONMENT} " ]]; then
        log_error "Invalid environment: $ENVIRONMENT"
        log_info "Available environments: ${ENVIRONMENTS[*]}"
        exit 1
    fi
    
    # Set default namespace if not provided
    if [[ -z "$NAMESPACE" ]]; then
        case $ENVIRONMENT in
            development) NAMESPACE="contextops-dev" ;;
            staging) NAMESPACE="contextops-staging" ;;
            production) NAMESPACE="contextops-prod" ;;
        esac
    fi
    
    log_success "Environment validation passed"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    # Check required tools
    local tools=("kubectl" "kustomize" "docker")
    for tool in "${tools[@]}"; do
        if ! command -v "$tool" &> /dev/null; then
            log_error "$tool is required but not installed"
            exit 1
        fi
    done
    
    # Check kubectl connection
    if ! kubectl cluster-info &> /dev/null; then
        log_error "Cannot connect to Kubernetes cluster"
        log_info "Please ensure kubectl is configured correctly"
        exit 1
    fi
    
    # Check if overlay directory exists
    if [[ ! -d "$DEPLOYMENTS_DIR/overlays/$ENVIRONMENT" ]]; then
        log_error "Deployment overlay not found: $DEPLOYMENTS_DIR/overlays/$ENVIRONMENT"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Build container images
build_images() {
    if [[ "$SKIP_BUILD" == "true" ]]; then
        log_info "Skipping container image build"
        return 0
    fi
    
    log_info "Building container images for $ENVIRONMENT..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would build container images with build script"
        return 0
    fi
    
    # Use the build script
    if [[ -x "$SCRIPT_DIR/build-images.sh" ]]; then
        "$SCRIPT_DIR/build-images.sh" --environment "$ENVIRONMENT"
    else
        log_warn "Build script not found, building with make"
        make -C "$PROJECT_ROOT" docker-build
    fi
    
    log_success "Container images built successfully"
}

# Run pre-deployment tests
run_tests() {
    if [[ "$SKIP_TESTS" == "true" ]]; then
        log_info "Skipping pre-deployment tests"
        return 0
    fi
    
    log_info "Running pre-deployment tests..."
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would run pre-deployment tests"
        return 0
    fi
    
    # Run tests if test script exists
    if [[ -x "$SCRIPT_DIR/test.sh" ]]; then
        "$SCRIPT_DIR/test.sh" --environment "$ENVIRONMENT"
    elif command -v make &> /dev/null && grep -q "test:" "$PROJECT_ROOT/Makefile"; then
        make -C "$PROJECT_ROOT" test
    else
        log_warn "No test script found, skipping tests"
    fi
    
    log_success "Pre-deployment tests passed"
}

# Validate Kubernetes manifests
validate_manifests() {
    log_info "Validating Kubernetes manifests..."
    
    # Build manifests with kustomize
    local manifests_file="/tmp/contextops-manifests-$ENVIRONMENT.yaml"
    kustomize build "$DEPLOYMENTS_DIR/overlays/$ENVIRONMENT" > "$manifests_file"
    
    # Validate with kubectl
    if ! kubectl apply --dry-run=client -f "$manifests_file" &> /dev/null; then
        log_error "Manifest validation failed"
        log_info "Run: kubectl apply --dry-run=client -f $manifests_file"
        exit 1
    fi
    
    # Additional validations
    local services=$(kubectl get -f "$manifests_file" -o jsonpath='{range .items[*]}{.kind}{"\n"}{end}' --dry-run=client | grep -c "Service" || true)
    local deployments=$(kubectl get -f "$manifests_file" -o jsonpath='{range .items[*]}{.kind}{"\n"}{end}' --dry-run=client | grep -c "Deployment" || true)
    
    log_info "Manifest summary: $deployments deployments, $services services"
    log_success "Manifest validation passed"
    
    # Clean up
    rm -f "$manifests_file"
}

# Deploy to Kubernetes
deploy_to_kubernetes() {
    log_info "Deploying ContextOps to $ENVIRONMENT environment..."
    log_info "Target namespace: $NAMESPACE"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "DRY RUN: Would deploy the following manifests:"
        kustomize build "$DEPLOYMENTS_DIR/overlays/$ENVIRONMENT"
        return 0
    fi
    
    # Apply manifests
    kustomize build "$DEPLOYMENTS_DIR/overlays/$ENVIRONMENT" | \
        kubectl apply -f - --timeout="$TIMEOUT"
    
    log_success "Manifests applied successfully"
}

# Wait for deployment to be ready
wait_for_deployment() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would wait for deployments to be ready"
        return 0
    fi
    
    log_info "Waiting for deployments to be ready..."
    
    # Get all deployments in the namespace
    local deployments=($(kubectl get deployments -n "$NAMESPACE" -o name | grep "contextops"))
    
    if [[ ${#deployments[@]} -eq 0 ]]; then
        log_warn "No ContextOps deployments found in namespace $NAMESPACE"
        return 0
    fi
    
    # Wait for each deployment
    for deployment in "${deployments[@]}"; do
        log_info "Waiting for $deployment..."
        kubectl rollout status "$deployment" -n "$NAMESPACE" --timeout="$TIMEOUT"
    done
    
    log_success "All deployments are ready"
}

# Perform post-deployment checks
post_deployment_checks() {
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would perform post-deployment health checks"
        return 0
    fi
    
    log_info "Performing post-deployment checks..."
    
    # Check pod status
    local failed_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=contextops-platform -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.status.phase}{"\n"}{end}' | grep -v "Running" | wc -l)
    
    if [[ $failed_pods -gt 0 ]]; then
        log_warn "$failed_pods pods are not running"
        kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/part-of=contextops-platform
    fi
    
    # Check service endpoints
    log_info "Checking service endpoints..."
    kubectl get services -n "$NAMESPACE" -l app.kubernetes.io/part-of=contextops-platform
    
    # Health check if possible
    if command -v curl &> /dev/null; then
        local gateway_service=$(kubectl get service -n "$NAMESPACE" -l app.kubernetes.io/component=api-gateway -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
        if [[ -n "$gateway_service" ]]; then
            log_info "Testing gateway health endpoint..."
            kubectl port-forward service/"$gateway_service" 8080:80 -n "$NAMESPACE" &
            local pf_pid=$!
            sleep 5
            
            if curl -s http://localhost:8080/health > /dev/null; then
                log_success "Gateway health check passed"
            else
                log_warn "Gateway health check failed"
            fi
            
            kill $pf_pid 2>/dev/null || true
        fi
    fi
    
    log_success "Post-deployment checks completed"
}

# Show deployment summary
show_summary() {
    log_info "Deployment Summary:"
    log_info "- Environment: $ENVIRONMENT"
    log_info "- Namespace: $NAMESPACE"
    log_info "- Dry Run: $DRY_RUN"
    
    if [[ "$DRY_RUN" == "false" ]]; then
        log_info ""
        log_info "Deployed resources:"
        kubectl get all -n "$NAMESPACE" -l app.kubernetes.io/part-of=contextops-platform
        
        log_info ""
        log_info "Access URLs:"
        case $ENVIRONMENT in
            development)
                log_info "- API: https://contextops-dev.example.com/api"
                log_info "- Metrics: https://metrics-dev.contextops.example.com"
                ;;
            staging)
                log_info "- API: https://staging.contextops.com/api"
                log_info "- Metrics: https://metrics-staging.contextops.com"
                ;;
            production)
                log_info "- API: https://api.contextops.com"
                log_info "- Metrics: https://metrics.contextops.com"
                ;;
        esac
    fi
}

# Production deployment confirmation
confirm_production_deployment() {
    if [[ "$ENVIRONMENT" == "production" && "$FORCE" != "true" && "$DRY_RUN" != "true" ]]; then
        log_warn "You are about to deploy to PRODUCTION environment!"
        read -p "Are you sure you want to continue? (yes/no): " -r
        if [[ ! $REPLY =~ ^[Yy]es$ ]]; then
            log_info "Deployment cancelled"
            exit 0
        fi
    fi
}

# Main execution
main() {
    log_info "ContextOps Deployment Script"
    log_info "Environment: $ENVIRONMENT"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "DRY RUN MODE - No actual deployment will occur"
    fi
    
    validate_environment
    check_prerequisites
    confirm_production_deployment
    build_images
    run_tests
    validate_manifests
    deploy_to_kubernetes
    wait_for_deployment
    post_deployment_checks
    show_summary
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_success "Dry run completed successfully!"
    else
        log_success "Deployment completed successfully!"
    fi
}

# Run main function
main "$@"