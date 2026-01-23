# Multi-stage Dockerfile for ContextOps GitOps Monitoring Platform
# Optimized for production deployment with minimal attack surface

# Stage 1: Build environment
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    ca-certificates \
    make \
    gcc \
    musl-dev

# Set working directory
WORKDIR /build

# Copy go mod files and download dependencies
COPY go.mod go.sum ./

# Set Go proxy for faster downloads and enable module caching
ENV GOPROXY=https://proxy.golang.org,direct \
    GOSUMDB=sum.golang.org \
    GOCACHE=/tmp/gocache \
    GOMODCACHE=/tmp/gomodcache

# Download and verify dependencies with timeout
RUN go mod download && go mod verify

# Copy source code
COPY . .

# Build services - using build args to determine which service to build
ARG SERVICE_NAME
ARG VERSION=dev
ARG COMMIT_SHA=unknown
ARG BUILD_DATE=unknown

# Validate SERVICE_NAME argument
RUN if [ -z "$SERVICE_NAME" ]; then \
        echo "SERVICE_NAME build arg is required"; \
        exit 1; \
    fi

# Set build flags for optimization and version info
ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Build the specific service
RUN go build \
    -ldflags="-w -s" \
    -a -installsuffix cgo \
    -o /build/bin/contextops \
    ./cmd/${SERVICE_NAME}/

# Create health check script
RUN echo '#!/bin/sh' > /build/bin/healthcheck && \
    echo 'exec /app/contextops healthcheck' >> /build/bin/healthcheck && \
    chmod +x /build/bin/healthcheck

# Stage 2: Runtime environment
FROM alpine:3.19

# Install runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    && update-ca-certificates

# Create non-root user for security
RUN addgroup -g 1000 -S contextops && \
    adduser -u 1000 -S contextops -G contextops -h /app

# Set working directory
WORKDIR /app

# Copy binary and health check script from builder
COPY --from=builder --chown=contextops:contextops /build/bin/contextops /app/contextops
COPY --from=builder --chown=contextops:contextops /build/bin/healthcheck /app/healthcheck

# Copy database migrations if they exist
COPY --from=builder --chown=contextops:contextops /build/migrations ./migrations

# Create directories for logs and data
RUN mkdir -p /app/logs /app/data && \
    chown -R contextops:contextops /app

# Switch to non-root user
USER contextops:contextops

# Set environment variables
ENV PATH="/app:${PATH}" \
    SERVICE_VERSION=${VERSION:-dev} \
    LOG_LEVEL=info \
    LOG_FORMAT=json \
    METRICS_ENABLED=true \
    HEALTH_CHECK_PORT=8081

# Health check configuration
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD ["/app/healthcheck"]

# Default command - will be overridden by Kubernetes deployment
CMD ["/app/contextops"]

# Labels for container metadata
LABEL \
    org.opencontainers.image.title="ContextOps" \
    org.opencontainers.image.description="GitOps-optimized application monitoring platform" \
    org.opencontainers.image.vendor="ContextOps" \
    org.opencontainers.image.version="${VERSION}" \
    org.opencontainers.image.created="${BUILD_DATE}" \
    org.opencontainers.image.revision="${COMMIT_SHA}" \
    org.opencontainers.image.source="https://github.com/contextops/platformctl" \
    org.opencontainers.image.licenses="MIT"

# Expose common ports (will be overridden by Kubernetes service definitions)
EXPOSE 8080 8081 9090