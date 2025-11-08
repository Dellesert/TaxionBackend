.PHONY: build run test clean

SERVICE_NAME=analytics-service

build:
	@echo "Building $(SERVICE_NAME)..."
	go build -o $(SERVICE_NAME) .

run:
	@echo "Running $(SERVICE_NAME)..."
	go run .

test:
	@echo "Running tests..."
	go test -v ./...

clean:
	@echo "Cleaning..."
	rm -f $(SERVICE_NAME)
	go clean

deps:
	@echo "Installing dependencies..."
	go mod download
	go mod tidy

fmt:
	@echo "Formatting code..."
	go fmt ./...
