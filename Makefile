.PHONY: build run test clean docker-build docker-up docker-down deps fmt lint

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=gofmt
BINARY_NAME=mev-engine
BINARY_PATH=./cmd/mev-engine

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)

# Run the application
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)
	./$(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

# Clean build artifacts
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Format code
fmt:
	$(GOFMT) -s -w .

# Lint code
lint:
	golangci-lint run

# Build Docker image
docker-build:
	docker build -t mev-engine:latest .

# Start development environment
docker-up:
	docker-compose up -d

# Stop development environment
docker-down:
	docker-compose down

# Restart development environment
docker-restart: docker-down docker-up

# View logs
docker-logs:
	docker-compose logs -f mev-engine

# Initialize go module
init:
	$(GOMOD) init github.com/mev-engine/l2-mev-strategy-engine

# Install development tools
install-tools:
	$(GOGET) github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Generate mocks (if using mockery)
generate-mocks:
	mockery --all --output=./mocks

# Run benchmarks
bench:
	$(GOTEST) -bench=. -benchmem ./...

# Check for security vulnerabilities
security:
	gosec ./...

# Build for multiple platforms
build-all:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-linux-amd64 $(BINARY_PATH)
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-darwin-amd64 $(BINARY_PATH)
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(BINARY_NAME)-windows-amd64.exe $(BINARY_PATH)

# Help
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  run            - Build and run the application"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  clean          - Clean build artifacts"
	@echo "  deps           - Download and tidy dependencies"
	@echo "  fmt            - Format code"
	@echo "  lint           - Lint code"
	@echo "  docker-build   - Build Docker image"
	@echo "  docker-up      - Start development environment"
	@echo "  docker-down    - Stop development environment"
	@echo "  docker-restart - Restart development environment"
	@echo "  docker-logs    - View application logs"
	@echo "  install-tools  - Install development tools"
	@echo "  bench          - Run benchmarks"
	@echo "  security       - Check for security vulnerabilities"
	@echo "  build-all      - Build for multiple platforms"