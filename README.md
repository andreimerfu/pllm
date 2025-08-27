# PLLM Helm Charts

This repository hosts Helm charts for PLLM (Proxy for LLM).

## Usage

Add the repository:
```bash
helm repo add pllm https://andreimerfu.github.io/pllm
helm repo update
```

Install PLLM:
```bash
helm install my-pllm pllm/pllm
```

## Available Charts

- **pllm**: Multi-provider LLM proxy with authentication, budgeting, and analytics

For more information, visit: https://github.com/andreimerfu/pllm