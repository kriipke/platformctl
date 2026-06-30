#!/bin/bash
set -euo pipefail

# Platformctl Docker Image Build Script
# Builds all service containers with proper versioning and metadata

# Configuration
REGISTRY="${REGISTRY:-platformctl}"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_DATE="${BUILD_DATE:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
PARALLEL="${PARALLEL:-4}"

# Services to build
SERVICES=(
    "gateway"
    "gitops-aggregator" 
    "app-sync-svc"
    "environment-validation-svc"
    "context-correlation-svc"
    "multi-environment-kube-svc"
    "customer-git-branch-svc"
)

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
Platformctl Docker Image Build Script

Usage: $0 [OPTIONS] [SERVICE...]

OPTIONS:
    -h, --help          Show this help message
    -r, --registry      Set container registry (default: platformctl)
    -v, --version       Set image version (default: git describe)
    -p, --push          Push images to registry after building
    -f, --force         Force rebuild without cache
    -j, --parallel      Number of parallel builds (default: 4)
    --no-latest         Don't tag images as 'latest'
    --dry-run           Show what would be built without building

SERVICES:
    If no services are specified, all services will be built:
    ${SERVICES[*]}

EXAMPLES:
    $0                          # Build all services
    $0 gateway aggregator       # Build only gateway and aggregator
    $0 -p -v 1.0.0              # Build all and push with version 1.0.0
    $0 --force gateway          # Force rebuild gateway without cache

EOF
}

# Parse command line arguments
PUSH=false
FORCE=false
TAG_LATEST=true
DRY_RUN=false
SERVICES_TO_BUILD=()

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -r|--registry)
            REGISTRY="$2"
            shift 2
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -p|--push)
            PUSH=true
            shift
            ;;
        -f|--force)
            FORCE=true
            shift
            ;;
        -j|--parallel)
            PARALLEL="$2"
            shift 2
            ;;
        --no-latest)
            TAG_LATEST=false
            shift
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        -*)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
        *)
            SERVICES_TO_BUILD+=("$1")
            shift
            ;;
    esac
done

# If no specific services provided, build all
if [[ ${#SERVICES_TO_BUILD[@]} -eq 0 ]]; then
    SERVICES_TO_BUILD=("${SERVICES[@]}")
fi

# Validate services
for service in "${SERVICES_TO_BUILD[@]}"; do
    if [[ ! " ${SERVICES[*]} " =~ " ${service} " ]]; then
        log_error "Unknown service: $service"
        log_info "Available services: ${SERVICES[*]}"
        exit 1
    fi
    
    if [[ ! -d "cmd/$service" ]]; then
        log_error "Service directory not found: cmd/$service"
        exit 1
    fi
done

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v docker &> /dev/null; then
        log_error "Docker is required but not installed"
        exit 1
    fi
    
    if ! docker info &> /dev/null; then
        log_error "Docker daemon is not running"
        exit 1
    fi
    
    if [[ ! -f "Dockerfile" ]]; then
        log_error "Dockerfile not found in current directory"
        exit 1
    fi
    
    if [[ ! -f "go.mod" ]]; then
        log_error "go.mod not found - are you in the project root?"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Build single service
build_service() {
    local service=$1
    local temp_log="/tmp/docker-build-${service}.log"
    
    log_info "Building $service..."
    
    # Prepare build arguments
    local build_args=(
        "--build-arg" "SERVICE_NAME=$service"
        "--build-arg" "VERSION=$VERSION"
        "--build-arg" "COMMIT_SHA=$COMMIT_SHA"
        "--build-arg" "BUILD_DATE=$BUILD_DATE"
    )
    
    # Add cache options
    if [[ "$FORCE" == "true" ]]; then
        build_args+=("--no-cache")
    fi
    
    # Add tags
    local tags=(
        "-t" "$REGISTRY/$service:$VERSION"
    )
    
    if [[ "$TAG_LATEST" == "true" ]]; then
        tags+=("-t" "$REGISTRY/$service:latest")
    fi
    
    # Build command
    local cmd=(
        "docker" "build"
        "${build_args[@]}"
        "${tags[@]}"
        "."
    )
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would run: ${cmd[*]}"
        return 0
    fi
    
    # Execute build
    if "${cmd[@]}" > "$temp_log" 2>&1; then
        log_success "Built $service:$VERSION"
        
        # Show image size
        local size=$(docker images "$REGISTRY/$service:$VERSION" --format "table {{.Size}}" | tail -n1)
        log_info "Image size: $size"
        
        return 0
    else
        log_error "Failed to build $service"
        log_error "Build log:"
        cat "$temp_log"
        return 1
    fi
}

# Push single service
push_service() {
    local service=$1
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_info "Would push: $REGISTRY/$service:$VERSION"
        if [[ "$TAG_LATEST" == "true" ]]; then
            log_info "Would push: $REGISTRY/$service:latest"
        fi
        return 0
    fi
    
    log_info "Pushing $service:$VERSION..."
    
    if docker push "$REGISTRY/$service:$VERSION"; then
        log_success "Pushed $service:$VERSION"
        
        if [[ "$TAG_LATEST" == "true" ]]; then
            if docker push "$REGISTRY/$service:latest"; then
                log_success "Pushed $service:latest"
            else
                log_error "Failed to push $service:latest"
                return 1
            fi
        fi
        
        return 0
    else
        log_error "Failed to push $service:$VERSION"
        return 1
    fi
}

# Main execution
main() {
    log_info "Platformctl Docker Build Script"
    log_info "Registry: $REGISTRY"
    log_info "Version: $VERSION"
    log_info "Commit: $COMMIT_SHA"
    log_info "Build Date: $BUILD_DATE"
    log_info "Services: ${SERVICES_TO_BUILD[*]}"
    log_info "Parallel builds: $PARALLEL"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        log_warn "DRY RUN MODE - No actual building will occur"
    fi
    
    check_prerequisites
    
    # Build services
    log_info "Building ${#SERVICES_TO_BUILD[@]} services..."
    
    local failed_builds=()
    local successful_builds=()
    
    # Export functions for parallel execution
    export -f build_service log_info log_success log_error log_warn
    export REGISTRY VERSION COMMIT_SHA BUILD_DATE FORCE TAG_LATEST DRY_RUN
    export RED GREEN YELLOW BLUE NC
    
    # Build in parallel
    printf '%s\n' "${SERVICES_TO_BUILD[@]}" | xargs -P "$PARALLEL" -I {} bash -c 'build_service "$@"' _ {}
    
    # Check results
    for service in "${SERVICES_TO_BUILD[@]}"; do
        if docker images "$REGISTRY/$service:$VERSION" --format "{{.Repository}}" | grep -q "$REGISTRY/$service"; then
            successful_builds+=("$service")
        else
            failed_builds+=("$service")
        fi
    done
    
    # Report results
    if [[ ${#successful_builds[@]} -gt 0 ]]; then
        log_success "Successfully built: ${successful_builds[*]}"
    fi
    
    if [[ ${#failed_builds[@]} -gt 0 ]]; then
        log_error "Failed builds: ${failed_builds[*]}"
    fi
    
    # Push if requested
    if [[ "$PUSH" == "true" && ${#successful_builds[@]} -gt 0 ]]; then
        log_info "Pushing images..."
        
        for service in "${successful_builds[@]}"; do
            if ! push_service "$service"; then
                log_error "Failed to push $service"
            fi
        done
    fi
    
    # Summary
    log_info "Build Summary:"
    log_info "- Total services: ${#SERVICES_TO_BUILD[@]}"
    log_info "- Successful: ${#successful_builds[@]}"
    log_info "- Failed: ${#failed_builds[@]}"
    
    if [[ ${#failed_builds[@]} -gt 0 ]]; then
        exit 1
    fi
    
    log_success "All builds completed successfully!"
}

# Run main function
main "$@"