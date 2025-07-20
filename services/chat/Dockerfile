# Multi-stage build for Chat Service
# File: services/chat/Dockerfile

# Build stage
FROM golang:1.23-alpine AS builder

# Install git and ca-certificates (needed for go mod download)
RUN apk add --no-cache git ca-certificates tzdata

# Create a non-root user for building
RUN adduser -D -g '' appuser

# Set working directory
WORKDIR /build

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy shared dependencies first (for better layer caching)
COPY shared/ ./shared/

# Copy chat service source code
COPY services/chat/ ./services/chat/

# Set working directory to chat service
WORKDIR /build/services/chat

# Build the application
# CGO_ENABLED=0 for static binary
# GOOS=linux for Linux target
# -a flag forces rebuilding of packages
# -installsuffix cgo for static linking
# -ldflags for reducing binary size
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o chat-service \
    main.go

# Runtime stage
FROM alpine:3.19

# Install ca-certificates, timezone data, and wget for health checks
RUN apk --no-cache add ca-certificates tzdata wget

# Create a non-root user
RUN addgroup -g 1002 appgroup && \
    adduser -u 1002 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder stage
COPY --from=builder /build/services/chat/chat-service .

# Change ownership of the application to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8082

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8082/health || exit 1

# Set environment variables
ENV GIN_MODE=release
ENV TZ=UTC

# Run the application
CMD ["./chat-service"]