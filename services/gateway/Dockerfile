# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the gateway service
RUN CGO_ENABLED=0 GOOS=linux go build -o gateway ./services/gateway

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/gateway .

# Expose port
EXPOSE 8080

# Command to run
CMD ["./gateway"]