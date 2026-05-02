#!/usr/bin/env bash
#
# demo-bank.sh — thin wrapper around `pllm seed bank` for muscle memory
# parity with the older Phase-1+2 scripts/demo-seed.sh wrapper. All
# flags pass through:
#
#   scripts/demo-bank.sh                            # seed
#   scripts/demo-bank.sh --cleanup                  # remove
#   scripts/demo-bank.sh --llm-route my-route       # use a different route
#   scripts/demo-bank.sh --with-deployments         # also publish live MCP entries
#
# Prefers a `pllm` on $PATH; falls back to building the CLI on the fly
# under ./bin/pllm when run from a fresh checkout.

set -euo pipefail

cd "$(dirname "$0")/.."

if command -v pllm >/dev/null 2>&1; then
  exec pllm seed bank "$@"
fi

if [ ! -x ./bin/pllm ]; then
  echo "building ./bin/pllm..." >&2
  go build -o ./bin/pllm ./cmd/cli
fi

exec ./bin/pllm seed bank "$@"
