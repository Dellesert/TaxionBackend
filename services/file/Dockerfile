# Multi-stage build for File Service
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

# Download dependencies with cache mount
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy shared dependencies first (for better layer caching)
COPY shared/ ./shared/

# Copy file service source code
COPY services/file/ ./services/file/

# Set working directory to file service
WORKDIR /build/services/file

# Build the application with cache mounts
# CGO_ENABLED=0 for static binary
# GOOS=linux for Linux target
# Removed -a flag to allow Go to use cached builds
# -installsuffix cgo for static linking
# -ldflags for reducing binary size
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
    -installsuffix cgo \
    -ldflags='-w -s -extldflags "-static"' \
    -o file-service \
    main.go

# Runtime stage
FROM alpine:3.19

# Install ca-certificates, timezone data, libheif for HEIC conversion,
# and ImageMagick with format support for EXIF orientation fix (auto-orient)
# Note: Alpine 3.19+ requires separate packages for each image format
RUN apk --no-cache add ca-certificates tzdata libheif-tools \
    imagemagick imagemagick-jpeg imagemagick-webp imagemagick-heic

# Create a non-root user
RUN addgroup -g 1001 appgroup && \
    adduser -u 1001 -G appgroup -s /bin/sh -D appuser

# Set working directory
WORKDIR /app

# Copy CA certificates from builder
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Copy the binary from builder stage
COPY --from=builder /build/services/file/file-service .

# Create uploads directory
RUN mkdir -p /app/uploads && chown -R appuser:appgroup /app

# Change ownership of the application to non-root user
RUN chown -R appuser:appgroup /app

# Switch to non-root user
USER appuser

# Expose port
EXPOSE 8084

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8084/health || exit 1

# Set environment variables
ENV GIN_MODE=release
ENV TZ=UTC
ENV UPLOAD_DIR=/app/uploads

# Run the application
CMD ["./file-service"]
