#!/bin/bash

echo "🔍 ApplicationSet Query Commands via ContextOps and Kubernetes APIs"
echo "=================================================================="
echo

# First, let's test what endpoints are available
echo "📋 1. CONTEXTOPS API QUERIES"
echo "-----------------------------"
echo

echo "1.1 ContextOps System Health (includes ArgoCD integration status):"
echo "curl -s -u admin:admin http://45.55.118.17/health"
curl -s -u admin:admin http://45.55.118.17/health | jq .
echo

echo "1.2 List all contexts (may show related GitOps contexts):"
echo "curl -s -u admin:admin http://45.55.118.17/api/v1/contexts"
echo "# Response (showing first context):"
curl -s -u admin:admin http://45.55.118.17/api/v1/contexts | jq '.contexts[0]' 2>/dev/null || echo "API response available but needs proper formatting"
echo

echo "1.3 Try GitOps status endpoints:"
echo "curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/health/overview"
curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/health/overview 2>/dev/null || echo "Note: May require customer context"
echo

echo "1.4 Test GitOps contexts status:"
echo "curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/contexts/status"
curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/contexts/status 2>/dev/null || echo "Note: May require customer context"
echo

echo "📋 2. KUBERNETES API QUERIES (via kubectl/proxy)"
echo "-----------------------------------------------"
echo

echo "2.1 Get ApplicationSet basic info:"
echo "kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.metadata.labels'"
kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.metadata.labels'
echo

echo "2.2 Get ApplicationSet status and conditions:"
echo "kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.status'"
kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.status'
echo

echo "2.3 Get ApplicationSet generator configuration:"
echo "kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.spec.generators'"
kubectl get applicationsets -n contextops contextops-sample-apps -o json | jq '.spec.generators'
echo

echo "2.4 Get generated Applications from ApplicationSet:"
echo "kubectl get applications -n contextops -l contextops.io/applicationset=contextops-sample-apps -o json | jq '.items[] | {name: .metadata.name, customer: .metadata.labels.\"contextops.io/customer\", environment: .metadata.labels.\"contextops.io/environment\"}'"
kubectl get applications -n contextops -l contextops.io/applicationset=contextops-sample-apps -o json | jq '.items[] | {name: .metadata.name, customer: .metadata.labels."contextops.io/customer", environment: .metadata.labels."contextops.io/environment"}'
echo

echo "📋 3. KUBERNETES API VIA CURL (if kubectl proxy is running)"
echo "----------------------------------------------------------"
echo

echo "3.1 Start kubectl proxy (run in separate terminal):"
echo "kubectl proxy --port=8080 &"
echo

echo "3.2 Query ApplicationSet via Kubernetes REST API:"
echo "curl -s http://localhost:8080/apis/argoproj.io/v1alpha1/namespaces/contextops/applicationsets/contextops-sample-apps | jq '.metadata.labels'"
echo

echo "3.3 Query generated Applications:"
echo "curl -s 'http://localhost:8080/apis/argoproj.io/v1alpha1/namespaces/contextops/applications?labelSelector=contextops.io/applicationset=contextops-sample-apps' | jq '.items[].metadata.name'"
echo

echo "📋 4. ARGOCD SERVICE QUERIES (if ArgoCD server was running)"
echo "---------------------------------------------------------"
echo

echo "4.1 ArgoCD server health (via kubectl port-forward):"
echo "kubectl port-forward -n contextops svc/contextops-argocd-server 8080:80 &"
echo "curl -s http://localhost:8080/api/v1/applications"
echo

echo "📋 5. CONTEXTOPS INTEGRATION STATUS QUERIES"
echo "-------------------------------------------"
echo

echo "5.1 Check ArgoCD integration environment variables:"
echo "kubectl exec -n contextops contextops-gateway-64d6fddc7b-54kbr -- env | grep ARGOCD"
kubectl exec -n contextops $(kubectl get pods -n contextops -l app=contextops-gateway -o jsonpath='{.items[0].metadata.name}') -- env | grep ARGOCD
echo

echo "5.2 Check integration configuration:"
echo "kubectl get configmap contextops-integration-config -n contextops -o json | jq '.data | with_entries(select(.key | contains(\"argocd\")))'"
kubectl get configmap contextops-integration-config -n contextops -o json | jq '.data | with_entries(select(.key | contains("argocd")))'
echo

echo "🎯 SUMMARY: Key Commands for ApplicationSet Queries"
echo "=================================================="
echo "• ApplicationSet Status: kubectl get applicationsets -n contextops contextops-sample-apps"
echo "• Generated Applications: kubectl get applications -n contextops -l contextops.io/applicationset=contextops-sample-apps"
echo "• ContextOps Health: curl -s -u admin:admin http://45.55.118.17/health"
echo "• Integration Config: kubectl get configmap contextops-integration-config -n contextops"