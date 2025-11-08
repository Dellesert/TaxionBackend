# Multi-stage build for Backup Service with PostgreSQL client tools

# Stage 1: Build stage
FROM golang:1.23-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Create non-root user for building
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared code
COPY shared/ ./shared/

# Copy backup service code
COPY services/backup/ ./services/backup/

# Change to service directory
WORKDIR /build/services/backup

# Build the application with cache mounts
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o backup-service \
    main.go

# Stage 2: Final stage with PostgreSQL client
FROM alpine:3.19

# Install runtime dependencies including PostgreSQL client
RUN apk --no-cache add ca-certificates tzdata postgresql16-client

# Create non-root user
RUN addgroup -g 1009 appgroup && \
    adduser -u 1009 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/services/backup/backup-service .

# Create backup directory and set permissions
RUN mkdir -p /app/backups && \
    chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8089

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8089/health || exit 1

# Run the application
CMD ["./backup-service"]
