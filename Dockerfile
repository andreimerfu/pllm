# UI Build stage
FROM node:20-alpine AS ui-builder

WORKDIR /app/web

# Copy package files
COPY web/package*.json ./

# Install dependencies
RUN npm ci

# Copy UI source
COPY web/ ./

# Build arguments for Vite environment variables
ARG VITE_DEX_PUBLIC_AUTHORITY
ARG VITE_DEX_CLIENT_ID
ARG VITE_DEX_CLIENT_SECRET

# Set environment variables for build
ENV VITE_DEX_PUBLIC_AUTHORITY=$VITE_DEX_PUBLIC_AUTHORITY
ENV VITE_DEX_CLIENT_ID=$VITE_DEX_CLIENT_ID
ENV VITE_DEX_CLIENT_SECRET=$VITE_DEX_CLIENT_SECRET

# Build UI
RUN npm run build

# Docs Build stage
FROM node:20-alpine AS docs-builder

WORKDIR /app/docs

# Copy package files
COPY docs/package*.json ./

# Install dependencies
RUN npm install

# Copy docs source
COPY docs/ ./

# Build documentation
RUN npm run build

# Go Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy built UI from ui-builder stage
COPY --from=ui-builder /app/web/dist ./internal/ui/dist

# Copy built docs from docs-builder stage
COPY --from=docs-builder /app/docs/.vitepress/dist ./internal/docs/dist

# Generate Swagger documentation
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init -g cmd/server/main.go -o internal/handlers/swagger

# Build the application with embedded UI
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o pllm cmd/server/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 -S pllm && \
    adduser -u 1000 -S pllm -G pllm

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/pllm .
COPY --from=builder /app/docs ./docs

# Copy config file (if exists)
COPY --chown=pllm:pllm config.yaml* ./

# Copy pricing file
COPY --from=builder --chown=pllm:pllm /app/internal/config/model_prices_and_context_window.json ./internal/config/

# Change ownership
RUN chown -R pllm:pllm /app

# Use non-root user
USER pllm

# Expose ports
EXPOSE 8080 8081 9090

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Run the application
ENTRYPOINT ["./pllm"]