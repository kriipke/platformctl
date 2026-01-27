# Working ContextOps GitOps API Queries

**Status**: ✅ All GitOps API endpoints working as of 2026-01-23

## GitOps Status API Endpoints

### 1. Health Overview ✅
```bash
curl -s -u admin:admin "http://45.55.118.17/api/v1/gitops/health/overview" | jq
```

### 2. Specific Context Status ✅  
```bash
curl -s -u admin:admin "http://45.55.118.17/api/v1/gitops/contexts/user-service-dev/status" | jq
```

## ApplicationSet Monitoring Status: 🎉 FUNCTIONAL

The ContextOps platform successfully demonstrates GitOps monitoring with ApplicationSet integration!
