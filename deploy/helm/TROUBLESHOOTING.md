# PLLM Helm Chart Troubleshooting

This document contains solutions to common issues when deploying PLLM with Helm.

## Dex Authentication Issues

### Problem: Issuer URL Mismatch

**Error Message:**
```
failed to initialize Dex provider: failed to create OIDC provider: oidc: issuer did not match the issuer returned by provider, expected "http://pllm-dex:5556/dex" got "http://localhost:5556/dex"
```

**Cause:** The Dex issuer URL is configured for localhost instead of the internal Kubernetes service name.

**Solution 1: Use the Fixed Makefile Commands**

The easiest way is to use the updated Makefile commands which automatically set the correct issuer:

```bash
# For demo/development
make helm-install

# For production (after creating values-prod.yaml)  
make helm-install-prod
```

**Solution 2: Manual Installation**

If installing manually, override the Dex issuer URL:

```bash
helm upgrade --install pllm ./pllm \
  --set dex.config.issuer="http://pllm-dex.pllm.svc.cluster.local:5556/dex" \
  --namespace pllm \
  --create-namespace
```

**Solution 3: Fix Existing Installation**

If you already have a failed installation, you can fix it:

```bash
# Uninstall the failed release
helm uninstall pllm -n pllm

# Reinstall with correct configuration
make helm-install
```

**Solution 4: Custom Values File**

Create a values file with the correct issuer (the chart will auto-configure it):

```yaml
# values-prod.yaml
dex:
  enabled: true
  config:
    # Leave issuer empty or set to the internal service name
    issuer: ""  # Will be auto-configured
    staticClients:
      - id: pllm
        redirectURIs:
          - 'https://your-domain.com/auth/callback'
        name: 'PLLM'
        secret: your-client-secret
```

Then install:

```bash
helm upgrade --install pllm ./pllm -f values-prod.yaml -n pllm --create-namespace
```

### Problem: Dex Pod Not Starting

**Symptoms:**
- Dex pod stuck in `Pending` or `CrashLoopBackOff`
- PLLM can't connect to Dex

**Solution:**

1. **Check Pod Status:**
   ```bash
   kubectl get pods -n pllm -l app.kubernetes.io/name=dex
   kubectl describe pod -n pllm -l app.kubernetes.io/name=dex
   ```

2. **Check Logs:**
   ```bash
   kubectl logs -n pllm -l app.kubernetes.io/name=dex
   ```

3. **Common Issues:**
   - **Resource constraints**: Increase resource limits
   - **Storage issues**: Check PVC status
   - **RBAC permissions**: Ensure Dex has proper permissions for Kubernetes storage

## Database Connection Issues

### Problem: PostgreSQL Connection Failed

**Error Message:**
```
failed to connect to database: dial tcp: lookup pllm-postgresql
```

**Solution:**

1. **Check PostgreSQL Pod:**
   ```bash
   kubectl get pods -n pllm -l app.kubernetes.io/name=postgresql
   kubectl logs -n pllm -l app.kubernetes.io/name=postgresql
   ```

2. **Verify Service:**
   ```bash
   kubectl get svc -n pllm -l app.kubernetes.io/name=postgresql
   ```

3. **Test Connection:**
   ```bash
   kubectl exec -n pllm -it deployment/pllm -- sh
   # Inside the pod:
   nc -zv pllm-postgresql 5432
   ```

## Redis Connection Issues

### Problem: Redis Connection Failed

**Error Message:**
```
failed to connect to Redis: dial tcp: lookup pllm-redis-master
```

**Solution:**

1. **Check Redis Pod:**
   ```bash
   kubectl get pods -n pllm -l app.kubernetes.io/name=redis
   kubectl logs -n pllm -l app.kubernetes.io/name=redis
   ```

2. **Verify Service:**
   ```bash
   kubectl get svc -n pllm -l app.kubernetes.io/name=redis
   ```

## Secret Management Issues

### Problem: Missing Required Secrets

**Error Message:**
```
Error: execution error at (pllm/templates/secret.yaml:X:X): required "JWT secret is required"
```

**Solution:**

Ensure all required secrets are provided:

```bash
helm upgrade --install pllm ./pllm \
  --set pllm.secrets.jwtSecret="$(openssl rand -hex 32)" \
  --set pllm.secrets.masterKey="sk-master-$(openssl rand -hex 16)" \
  --namespace pllm
```

## Ingress Issues

### Problem: Ingress Not Working

**Symptoms:**
- Can't access PLLM externally
- 404 or connection refused errors

**Solution:**

1. **Check Ingress Controller:**
   ```bash
   kubectl get pods -n ingress-nginx  # or your ingress namespace
   ```

2. **Verify Ingress Resource:**
   ```bash
   kubectl get ingress -n pllm
   kubectl describe ingress -n pllm pllm
   ```

3. **Check Service:**
   ```bash
   kubectl get svc -n pllm pllm
   ```

4. **Test Port Forward:**
   ```bash
   kubectl port-forward -n pllm svc/pllm 8080:8080
   # Test: curl http://localhost:8080/health
   ```

## Resource Issues

### Problem: Pods Stuck in Pending

**Symptoms:**
- Pods show `Pending` status
- Events show scheduling issues

**Solution:**

1. **Check Node Resources:**
   ```bash
   kubectl describe nodes
   kubectl top nodes  # if metrics-server is installed
   ```

2. **Check Pod Requirements:**
   ```bash
   kubectl describe pod -n pllm <pod-name>
   ```

3. **Reduce Resource Requirements:**
   ```yaml
   # values.yaml
   resources:
     requests:
       cpu: 100m
       memory: 128Mi
   ```

## Networking Issues

### Problem: Service-to-Service Communication Failed

**Symptoms:**
- PLLM can't reach Dex/PostgreSQL/Redis
- DNS resolution errors

**Solution:**

1. **Check DNS:**
   ```bash
   kubectl exec -n pllm -it deployment/pllm -- nslookup pllm-dex
   kubectl exec -n pllm -it deployment/pllm -- nslookup pllm-postgresql
   kubectl exec -n pllm -it deployment/pllm -- nslookup pllm-redis-master
   ```

2. **Test Connectivity:**
   ```bash
   kubectl exec -n pllm -it deployment/pllm -- nc -zv pllm-dex 5556
   kubectl exec -n pllm -it deployment/pllm -- nc -zv pllm-postgresql 5432
   kubectl exec -n pllm -it deployment/pllm -- nc -zv pllm-redis-master 6379
   ```

3. **Check Network Policies:**
   ```bash
   kubectl get networkpolicies -n pllm
   ```

## Useful Debug Commands

### Get All Resources
```bash
kubectl get all -n pllm
kubectl get events -n pllm --sort-by='.lastTimestamp'
```

### Pod Debugging
```bash
# Get pod logs
kubectl logs -n pllm -l app.kubernetes.io/name=pllm --tail=100

# Follow logs
kubectl logs -n pllm -l app.kubernetes.io/name=pllm -f

# Get previous pod logs (if pod restarted)
kubectl logs -n pllm <pod-name> --previous

# Exec into pod
kubectl exec -n pllm -it <pod-name> -- sh
```

### Service Debugging
```bash
# Test service endpoints
kubectl get endpoints -n pllm

# Port forward for testing
kubectl port-forward -n pllm svc/pllm 8080:8080
kubectl port-forward -n pllm svc/pllm-dex 5556:5556
```

### Configuration Debugging
```bash
# View ConfigMaps
kubectl get configmap -n pllm pllm -o yaml

# View Secrets (base64 encoded)
kubectl get secret -n pllm pllm -o yaml

# Decode secret
kubectl get secret -n pllm pllm -o jsonpath='{.data.master-key}' | base64 -d
```

## Getting Help

If you're still experiencing issues:

1. **Check the logs** for specific error messages
2. **Verify prerequisites** (Kubernetes version, storage class, ingress controller)
3. **Test with minimal configuration** first
4. **Open an issue** with detailed logs and configuration

## Common Configuration Examples

### Minimal Working Configuration
```yaml
pllm:
  secrets:
    jwtSecret: "your-jwt-secret"
    masterKey: "sk-master-your-key"
    openaiApiKey: "sk-your-openai-key"
```

### Production Configuration
```yaml
replicaCount: 3
pllm:
  secrets:
    jwtSecret: "your-jwt-secret"
    masterKey: "sk-master-your-key"
    openaiApiKey: "sk-your-openai-key"
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: pllm.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
```