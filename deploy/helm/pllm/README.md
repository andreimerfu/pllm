# PLLM Helm Chart

A Helm chart for deploying PLLM (Proxy for LLM) on Kubernetes with all dependencies.

## Features

- **High Availability**: Multi-replica deployments with auto-scaling
- **Dependencies Management**: Integrated PostgreSQL, Redis, and Dex
- **Security**: Pod security contexts, non-root containers, secret management
- **Monitoring**: ServiceMonitor for Prometheus integration
- **Ingress**: Configurable ingress with TLS support

## Prerequisites

- Kubernetes 1.19+
- Helm 3.8+
- PV provisioner support in the underlying infrastructure

## Installing the Chart

### From Source

```bash
# Clone the repository
git clone https://github.com/amerfu/pllm.git
cd pllm/deploy/helm

# Update dependencies
helm dependency update pllm

# Install the chart
helm install pllm ./pllm
```

### From Registry (when published)

```bash
# Add the repository
helm repo add pllm https://amerfu.github.io/pllm
helm repo update

# Install the chart
helm install pllm pllm/pllm
```

## Configuration

### Basic Configuration

Create a `values.yaml` file with your specific configuration:

```yaml
# API Keys (required)
pllm:
  secrets:
    jwtSecret: "your-jwt-secret-here"
    masterKey: "sk-master-your-master-key"
    openaiApiKey: "sk-your-openai-key"
    anthropicApiKey: "sk-ant-your-anthropic-key"

# Ingress configuration
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: pllm.example.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: pllm-tls
      hosts:
        - pllm.example.com

# High availability
replicaCount: 3
autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
```

Then install with your values:

```bash
helm install pllm pllm/pllm -f values.yaml
```

### Configuration Parameters

#### PLLM Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of PLLM pods | `1` |
| `image.repository` | PLLM image repository | `amerfu/pllm` |
| `image.tag` | PLLM image tag | `""` (uses appVersion) |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

#### Secrets Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `pllm.secrets.jwtSecret` | JWT secret for token signing | `""` (required) |
| `pllm.secrets.masterKey` | Master key for admin access | `""` (required) |
| `pllm.secrets.openaiApiKey` | OpenAI API key | `""` |
| `pllm.secrets.anthropicApiKey` | Anthropic API key | `""` |
| `pllm.secrets.azureApiKey` | Azure OpenAI API key | `""` |
| `pllm.secrets.bedrockAccessKeyId` | AWS Bedrock access key ID | `""` |
| `pllm.secrets.bedrockSecretAccessKey` | AWS Bedrock secret access key | `""` |

#### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Main API port | `8080` |
| `service.adminPort` | Admin API port | `8081` |
| `service.metricsPort` | Metrics port | `9090` |

#### Ingress Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `ingress.enabled` | Enable ingress | `false` |
| `ingress.className` | Ingress class name | `""` |
| `ingress.hosts` | Ingress hosts configuration | `[]` |
| `ingress.tls` | Ingress TLS configuration | `[]` |

#### Autoscaling Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `autoscaling.enabled` | Enable autoscaling | `false` |
| `autoscaling.minReplicas` | Minimum replicas | `1` |
| `autoscaling.maxReplicas` | Maximum replicas | `100` |
| `autoscaling.targetCPUUtilizationPercentage` | CPU target percentage | `80` |

#### Database Configuration (PostgreSQL)

| Parameter | Description | Default |
|-----------|-------------|---------|
| `postgresql.enabled` | Enable PostgreSQL dependency | `true` |
| `postgresql.auth.username` | Database username | `pllm` |
| `postgresql.auth.password` | Database password | `pllm-password` |
| `postgresql.auth.database` | Database name | `pllm` |

#### Redis Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `redis.enabled` | Enable Redis dependency | `true` |
| `redis.auth.enabled` | Enable Redis authentication | `true` |
| `redis.auth.password` | Redis password | `pllm-redis` |

#### Dex Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `dex.enabled` | Enable Dex SSO dependency | `true` |
| `dex.config.issuer` | Dex issuer URL | Auto-configured |
| `dex.config.staticClients` | Static client configuration | Pre-configured for PLLM |

## Examples

### Minimal Production Setup

```yaml
pllm:
  secrets:
    jwtSecret: "your-super-secret-jwt-key-here"
    masterKey: "sk-master-production-key"
    openaiApiKey: "sk-your-openai-production-key"

replicaCount: 2

ingress:
  enabled: true
  className: nginx
  hosts:
    - host: pllm.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
```

### High Availability Setup

```yaml
pllm:
  secrets:
    jwtSecret: "your-super-secret-jwt-key-here"
    masterKey: "sk-master-production-key"
    openaiApiKey: "sk-your-openai-production-key"
    anthropicApiKey: "sk-ant-your-anthropic-key"

replicaCount: 3

autoscaling:
  enabled: true
  minReplicas: 3
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

ingress:
  enabled: true
  className: nginx
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/rate-limit: "100"
  hosts:
    - host: pllm.yourdomain.com
      paths:
        - path: /
          pathType: Prefix
  tls:
    - secretName: pllm-tls
      hosts:
        - pllm.yourdomain.com

serviceMonitor:
  enabled: true
  labels:
    prometheus: kube-prometheus
```

### External Dependencies Setup

```yaml
# Disable internal dependencies
postgresql:
  enabled: false

redis:
  enabled: false

dex:
  enabled: false

# Configure external services
pllm:
  config:
    database:
      host: "your-postgres-host.amazonaws.com"
      port: 5432
      name: pllm
      user: pllm
      sslMode: require
    redis:
      host: "your-redis-cluster.cache.amazonaws.com"
      port: 6379
    auth:
      dex:
        issuer: "https://your-dex-instance.com/dex"

  secrets:
    databasePassword: "your-database-password"
    redisPassword: "your-redis-password"
    jwtSecret: "your-jwt-secret"
    masterKey: "sk-master-your-key"
    dexClientSecret: "your-dex-client-secret"
```

## Upgrading

### Upgrade the Chart

```bash
# Update repository
helm repo update

# Upgrade release
helm upgrade pllm pllm/pllm -f values.yaml
```

### Major Version Upgrades

Check the [CHANGELOG.md](CHANGELOG.md) for breaking changes before upgrading major versions.

## Uninstalling

```bash
# Uninstall the release
helm uninstall pllm

# Remove PVCs (optional, will delete data)
kubectl delete pvc -l app.kubernetes.io/instance=pllm
```

## Monitoring

The chart includes a ServiceMonitor resource for Prometheus integration:

```yaml
serviceMonitor:
  enabled: true
  labels:
    prometheus: kube-prometheus
  interval: 30s
  scrapeTimeout: 10s
```

Metrics are available at `/metrics` endpoint on port 9090.

## Security

The chart follows security best practices:

- Non-root containers
- Read-only root filesystem where possible
- Pod security contexts
- Secret management for sensitive data
- Network policies (optional)

## Troubleshooting

### Common Issues

1. **Pod stuck in Pending**: Check PVC provisioning and node resources
2. **Database connection failed**: Verify PostgreSQL is running and credentials are correct
3. **Dex authentication not working**: Check Dex configuration and client settings

### Debug Commands

```bash
# Check pod status
kubectl get pods -l app.kubernetes.io/name=pllm

# View pod logs
kubectl logs -l app.kubernetes.io/name=pllm

# Check services
kubectl get svc -l app.kubernetes.io/name=pllm

# Describe pod for events
kubectl describe pod <pod-name>
```

## Contributing

1. Fork the repository
2. Create your feature branch
3. Make your changes
4. Test with `helm template` and `helm lint`
5. Submit a pull request

## License

This chart is licensed under the same license as the PLLM project.