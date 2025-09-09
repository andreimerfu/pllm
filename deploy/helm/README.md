# PLLM Helm Chart

This directory contains the Helm chart for deploying PLLM (Proxy for LLM) to Kubernetes.

## Automated Releases

The Helm chart is automatically versioned and published when using semantic-release:

1. **Version Management**: Chart version and appVersion are automatically updated with semantic-release
2. **GitHub Releases**: Packaged charts are attached to GitHub releases
3. **Helm Repository**: Charts are published to GitHub Pages at `https://<username>.github.io/<repo-name>/`

## Manual Testing

```bash
# Validate chart
make helm-validate

# Package chart
make helm-package

# Install locally (requires k8s cluster)
make helm-install

# Uninstall
make helm-uninstall
```

## Usage

### Add the Helm repository
```bash
helm repo add pllm https://<username>.github.io/<repo-name>/
helm repo update
```

### Install PLLM
```bash
helm install pllm pllm/pllm --create-namespace --namespace pllm-system
```

### Upgrade PLLM
```bash
helm upgrade pllm pllm/pllm --namespace pllm-system
```

## Configuration

See the main [values.yaml](pllm/values.yaml) file for all configuration options.

Common configurations:

```yaml
# Custom image tag
image:
  tag: "v1.2.3"

# Enable ingress
ingress:
  enabled: true
  className: "nginx"
  hosts:
    - host: pllm.example.com
      paths:
        - path: /
          pathType: Prefix

# External database
postgresql:
  enabled: false
externalDatabase:
  host: "postgres.example.com"
  port: 5432
  database: "pllm"
```

## Dependencies

The chart includes these sub-charts by default:
- **postgresql**: Database (can be disabled for external DB)
- **redis**: Caching layer
- **dex**: OIDC provider (optional)

## Troubleshooting

See [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for common issues and solutions.