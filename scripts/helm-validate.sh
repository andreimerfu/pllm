#!/bin/bash
set -e

CHART_DIR="deploy/helm/pllm"
VERSION=${1:-"0.1.0-dev"}

echo "🔍 Validating Helm chart..."

# Update dependencies
echo "📦 Updating Helm dependencies..."
cd "$CHART_DIR"
helm dependency update

# Lint the chart
echo "🔍 Linting Helm chart..."
helm lint .

# Template and validate
echo "📝 Templating chart..."
helm template pllm . --version "$VERSION" --debug --dry-run > /tmp/pllm-template.yaml

# Check if template is valid YAML
echo "✅ Validating YAML syntax..."
yq eval . /tmp/pllm-template.yaml > /dev/null

echo "🎉 Helm chart validation completed successfully!"
echo "📄 Template output saved to /tmp/pllm-template.yaml"