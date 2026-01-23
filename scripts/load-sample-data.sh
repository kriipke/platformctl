#!/bin/bash

# ContextOps Sample Data Loading Script
# Loads comprehensive sample data into the ContextOps database

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Default database connection parameters
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-contextops}"
DB_NAME="${DB_NAME:-contextops}"
DB_PASSWORD="${DB_PASSWORD:-contextops}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SQL_FILE="$SCRIPT_DIR/generate-sample-data.sql"

echo -e "${BLUE}ContextOps Sample Data Loader${NC}"
echo "=============================="
echo

# Check if SQL file exists
if [[ ! -f "$SQL_FILE" ]]; then
    echo -e "${RED}Error: SQL file not found at $SQL_FILE${NC}"
    exit 1
fi

# Function to run SQL with psql
run_sql() {
    if command -v psql >/dev/null 2>&1; then
        echo -e "${BLUE}Loading sample data using psql...${NC}"
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$SQL_FILE"
    else
        echo -e "${RED}Error: psql command not found. Please install PostgreSQL client.${NC}"
        exit 1
    fi
}

# Function to run SQL with kubectl exec (for Kubernetes deployment)
run_sql_k8s() {
    local pod_name="$1"
    local namespace="${2:-contextops}"
    
    echo -e "${BLUE}Loading sample data using kubectl exec...${NC}"
    kubectl exec -n "$namespace" "$pod_name" -- psql -h localhost -U "$DB_USER" -d "$DB_NAME" -f - < "$SQL_FILE"
}

# Parse command line arguments
KUBERNETES_MODE=false
KUBERNETES_POD=""
KUBERNETES_NAMESPACE="contextops"
CLEAR_EXISTING=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --kubernetes|-k)
            KUBERNETES_MODE=true
            shift
            ;;
        --pod|-p)
            KUBERNETES_POD="$2"
            shift 2
            ;;
        --namespace|-n)
            KUBERNETES_NAMESPACE="$2"
            shift 2
            ;;
        --clear-existing)
            CLEAR_EXISTING=true
            shift
            ;;
        --host)
            DB_HOST="$2"
            shift 2
            ;;
        --port)
            DB_PORT="$2"
            shift 2
            ;;
        --user|-u)
            DB_USER="$2"
            shift 2
            ;;
        --database|-d)
            DB_NAME="$2"
            shift 2
            ;;
        --password)
            DB_PASSWORD="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo
            echo "Options:"
            echo "  -k, --kubernetes          Use kubectl exec instead of direct psql"
            echo "  -p, --pod POD_NAME        Kubernetes pod name (required with --kubernetes)"
            echo "  -n, --namespace NAMESPACE Kubernetes namespace (default: contextops)"
            echo "  --clear-existing          Clear existing sample data first (DANGEROUS)"
            echo "  --host HOST               Database host (default: localhost)"
            echo "  --port PORT               Database port (default: 5432)"
            echo "  -u, --user USER           Database user (default: contextops)"
            echo "  -d, --database DB         Database name (default: contextops)"
            echo "  --password PASSWORD       Database password (default: contextops)"
            echo "  -h, --help                Show this help message"
            echo
            echo "Examples:"
            echo "  $0                        # Load using local psql"
            echo "  $0 -k -p postgres-pod     # Load using kubectl exec"
            echo "  $0 --host db.example.com  # Load to remote database"
            echo
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Validate Kubernetes options
if [[ "$KUBERNETES_MODE" == true ]] && [[ -z "$KUBERNETES_POD" ]]; then
    echo -e "${RED}Error: --pod is required when using --kubernetes mode${NC}"
    exit 1
fi

# Display configuration
echo -e "${YELLOW}Configuration:${NC}"
if [[ "$KUBERNETES_MODE" == true ]]; then
    echo "  Mode: Kubernetes (kubectl exec)"
    echo "  Pod: $KUBERNETES_POD"
    echo "  Namespace: $KUBERNETES_NAMESPACE"
else
    echo "  Mode: Direct connection (psql)"
    echo "  Host: $DB_HOST:$DB_PORT"
    echo "  Database: $DB_NAME"
    echo "  User: $DB_USER"
fi
echo

# Warning about clearing existing data
if [[ "$CLEAR_EXISTING" == true ]]; then
    echo -e "${YELLOW}⚠️  WARNING: This will clear existing sample data first!${NC}"
    echo -e "${YELLOW}   Only enable this if you want to reset the sample data.${NC}"
    echo
    read -p "Are you sure you want to continue? (y/N): " -r
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Aborted."
        exit 1
    fi
fi

# Test database connection
echo -e "${BLUE}Testing database connection...${NC}"
if [[ "$KUBERNETES_MODE" == true ]]; then
    if ! kubectl get pod -n "$KUBERNETES_NAMESPACE" "$KUBERNETES_POD" >/dev/null 2>&1; then
        echo -e "${RED}Error: Pod $KUBERNETES_POD not found in namespace $KUBERNETES_NAMESPACE${NC}"
        exit 1
    fi
    
    if ! kubectl exec -n "$KUBERNETES_NAMESPACE" "$KUBERNETES_POD" -- psql -h localhost -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        echo -e "${RED}Error: Cannot connect to database via kubectl exec${NC}"
        exit 1
    fi
else
    if ! PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" >/dev/null 2>&1; then
        echo -e "${RED}Error: Cannot connect to database${NC}"
        echo "Check your connection parameters or ensure the database is running."
        exit 1
    fi
fi
echo -e "${GREEN}✓ Database connection successful${NC}"
echo

# Load the sample data
echo -e "${BLUE}Loading ContextOps sample data...${NC}"
echo "This will create:"
echo "  • 3 customer organizations (ACME Corp, TechStart.io, Global Bank)"
echo "  • 9 environments across all customers"
echo "  • 11 applications with multi-environment deployments"
echo "  • 18 contexts (app-environment combinations)"
echo "  • 30+ Vault secrets with validation status"
echo "  • GitOps metadata (ApplicationSets, Helm sources, etc.)"
echo

if [[ "$KUBERNETES_MODE" == true ]]; then
    run_sql_k8s "$KUBERNETES_POD" "$KUBERNETES_NAMESPACE"
else
    run_sql
fi

if [[ $? -eq 0 ]]; then
    echo
    echo -e "${GREEN}✅ Sample data loaded successfully!${NC}"
    echo
    echo -e "${BLUE}What was created:${NC}"
    echo "  🏢 Customer Organizations: ACME Corp, TechStart.io, Global Bank"
    echo "  🌍 Environments: dev/qa/prod, dev/staging/prod, dev/test/prod"
    echo "  📱 Applications: user-service, payment-service, web-app, api-backend, etc."
    echo "  🔐 Vault Secrets: Database credentials, JWT keys, API keys, certificates"
    echo "  🚀 Deployments: Multi-environment GitOps deployments with ArgoCD"
    echo
    echo -e "${YELLOW}Next steps:${NC}"
    echo "  1. Test the API endpoints: curl -u admin:admin http://your-gateway/api/v1/contexts"
    echo "  2. Explore multi-environment deployments for each customer"
    echo "  3. Review Vault secret synchronization across environments"
    echo "  4. Check GitOps ApplicationSet configurations"
    echo
else
    echo
    echo -e "${RED}❌ Failed to load sample data${NC}"
    echo "Check the error messages above for details."
    exit 1
fi