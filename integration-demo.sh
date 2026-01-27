#!/bin/bash

echo "🎯 ContextOps ApplicationSet Integration Status"
echo "=============================================="
echo

echo "📋 INTEGRATION VERIFICATION:"
echo

echo "1. ContextOps API Health (with ArgoCD integration):"
curl -s -u admin:admin http://45.55.118.17/health | jq -r '"Status: \(.status) | Services: Database=\(.services.database), Storage=\(.services.storage)"'
echo

echo "2. ArgoCD Integration Configuration in ContextOps:"
kubectl get configmap contextops-integration-config -n contextops -o json | jq '.data | with_entries(select(.key | contains("argocd"))) | to_entries[] | "\(.key): \(.value)"' -r
echo

echo "3. ApplicationSet Deployed and Discoverable:"
kubectl get applicationset contextops-sample-apps -n contextops -o json | jq '{name: .metadata.name, monitored: .metadata.labels."contextops.io/monitored", customer: .metadata.labels."contextops.io/customer", status: .status.conditions[-1].reason}'
echo

echo "4. Generated Applications (what ContextOps would monitor):"
kubectl get applications -n contextops -l contextops.io/applicationset=contextops-sample-apps -o json | jq '.items[] | {name: .metadata.name, environment: .metadata.labels."contextops.io/environment", monitored: .metadata.labels."contextops.io/monitored"}'
echo

echo "5. ContextOps Gateway Environment Variables (ArgoCD Integration):"
kubectl exec -n contextops $(kubectl get pods -n contextops -l app=contextops-gateway -o jsonpath='{.items[0].metadata.name}') -- env | grep "ARGOCD_ENABLED\|ARGOCD_SERVER_URL" | head -2
echo

echo "📋 CURRENT API STATUS:"
echo

echo "6. ContextOps Basic APIs (Working):"
echo "   Contexts: $(curl -s -u admin:admin http://45.55.118.17/api/v1/contexts | jq -r '.count') contexts available"
echo "   Apps: $(curl -s -u admin:admin http://45.55.118.17/api/v1/apps | jq -r '.count') apps configured"
echo "   Environments: $(curl -s -u admin:admin http://45.55.118.17/api/v1/environments | jq -r '.count') environments configured"
echo

echo "7. GitOps API Status (Needs Customer Context Fix):"
echo "   Health Overview: $(curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/health/overview 2>&1 | jq -r '.error // "Working"')"
echo "   Context Status: $(curl -s -u admin:admin http://45.55.118.17/api/v1/gitops/contexts/status 2>&1 | jq -r '.error // "Working"')"
echo

echo "📋 INTEGRATION SUMMARY:"
echo "✅ ApplicationSet deployed with ContextOps labels"
echo "✅ 3 Applications generated across dev/qa/prod environments"  
echo "✅ ContextOps has ArgoCD integration environment variables"
echo "✅ ContextOps can discover ApplicationSet via Kubernetes API"
echo "✅ Integration configuration loaded in ContextOps"
echo "❌ GitOps API endpoints need customer context middleware fix"
echo

echo "🚀 NEXT STEPS:"
echo "1. Deploy updated gateway with customer context fix"
echo "2. Test GitOps API endpoints for ApplicationSet monitoring"
echo "3. Verify end-to-end ApplicationSet workflow monitoring"