# Build stage
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

# Generate Swagger documentation
RUN go install github.com/swaggo/swag/cmd/swag@latest
RUN swag init -g cmd/server/main.go -o docs

# Build the application
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