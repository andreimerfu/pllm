# pllm MCP wrapper image
# Spawns an stdio MCP server (npx or uvx) and exposes it over HTTP on :8000.
# Designed to run under the restricted SecurityContext the K8sAdapter applies
# (runAsNonRoot, readOnlyRootFilesystem=true). The caller must mount a
# writable emptyDir at /tmp and set HOME=/tmp for npm / uv cache.

# Grab the static uv binary from Astral's image (multi-arch, musl-compatible).
FROM ghcr.io/astral-sh/uv:0.5.14 AS uv

FROM node:22-alpine

# Python runtime for any uvx-launched packages.
RUN apk add --no-cache python3 ca-certificates \
 && rm -rf /var/cache/apk/*

COPY --from=uv /uv /uvx /usr/local/bin/

WORKDIR /app
COPY bridge.mjs ./

# Non-root UID must match what the K8sAdapter sets in PodSecurityContext.
# Give the user a HOME under /tmp so npm / uv caches have somewhere to go
# when the root filesystem is read-only. K8sAdapter mounts an emptyDir at
# /tmp to make this work.
RUN addgroup -g 10001 -S wrapper \
 && adduser  -u 10001 -S -G wrapper -H -h /tmp wrapper \
 && mkdir -p /tmp && chmod 1777 /tmp

USER 10001:10001

ENV PORT=8000 \
    HOME=/tmp \
    NODE_NO_WARNINGS=1

EXPOSE 8000

# Child spawns via `node bridge.mjs`. Child process (npx/uvx) inherits env.
CMD ["node", "bridge.mjs"]
