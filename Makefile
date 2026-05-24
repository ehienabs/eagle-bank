# Eagle Bank Makefile
# Production-ready build automation

.PHONY: all build run test lint fmt vet clean docker docker-build docker-push \
        helm-lint helm-template helm-install helm-upgrade helm-uninstall \
        migrate-up migrate-down migrate-create \
        dev dev-up dev-down dev-logs \
        proto mocks coverage help

# Variables
APP_NAME := eagle-bank
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Docker
DOCKER_REGISTRY ?= ghcr.io
DOCKER_REPO ?= $(DOCKER_REGISTRY)/ehienabs/$(APP_NAME)
DOCKER_TAG ?= $(VERSION)

# Go
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOGET := $(GOCMD) get
GOMOD := $(GOCMD) mod
GOFMT := gofmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags "-w -s \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.buildTime=$(BUILD_TIME)"

# Directories
BIN_DIR := bin
COVERAGE_DIR := coverage

# Default target
all: lint test build

## Build
build: ## Build the application
	@echo "Building $(APP_NAME) $(VERSION)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME) ./cmd/api

build-linux: ## Build for Linux
	@echo "Building $(APP_NAME) for Linux..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-linux-amd64 ./cmd/api

build-darwin: ## Build for macOS
	@echo "Building $(APP_NAME) for macOS..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-darwin-amd64 ./cmd/api
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(APP_NAME)-darwin-arm64 ./cmd/api

## Run
run: build ## Run the application locally
	@echo "Running $(APP_NAME)..."
	./$(BIN_DIR)/$(APP_NAME)

run-dev: ## Run with hot reload (requires air)
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	air

## Test
test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race -cover ./...

test-short: ## Run short tests
	@echo "Running short tests..."
	$(GOTEST) -v -short ./...

test-integration: ## Run integration tests
	@echo "Running integration tests..."
	$(GOTEST) -v -tags=integration ./...

coverage: ## Generate coverage report
	@echo "Generating coverage report..."
	@mkdir -p $(COVERAGE_DIR)
	$(GOTEST) -v -race -coverprofile=$(COVERAGE_DIR)/coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=$(COVERAGE_DIR)/coverage.out -o $(COVERAGE_DIR)/coverage.html
	@echo "Coverage report: $(COVERAGE_DIR)/coverage.html"

benchmark: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

## Lint
lint: ## Run linters
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) -s -w .

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

## Dependencies
deps: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

deps-update: ## Update dependencies
	@echo "Updating dependencies..."
	$(GOMOD) tidy
	$(GOGET) -u ./...
	$(GOMOD) tidy

deps-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify

## Clean
clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -rf $(COVERAGE_DIR)

## Docker
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t $(DOCKER_REPO):$(DOCKER_TAG) \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT=$(COMMIT) \
		--build-arg BUILD_TIME=$(BUILD_TIME) \
		.
	docker tag $(DOCKER_REPO):$(DOCKER_TAG) $(DOCKER_REPO):latest

docker-push: ## Push Docker image
	@echo "Pushing Docker image..."
	docker push $(DOCKER_REPO):$(DOCKER_TAG)
	docker push $(DOCKER_REPO):latest

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 8080:8080 \
		-e APP_ENVIRONMENT=development \
		$(DOCKER_REPO):$(DOCKER_TAG)

## Development Environment
dev-up: ## Start development environment
	@echo "Starting development environment..."
	docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 10
	@echo "Development environment is ready!"
	@echo "  API: http://localhost:8080"
	@echo "  Kafka UI: http://localhost:8090"
	@echo "  Jaeger: http://localhost:16686"
	@echo "  Prometheus: http://localhost:9090"
	@echo "  Grafana: http://localhost:3000 (admin/admin)"

dev-down: ## Stop development environment
	@echo "Stopping development environment..."
	docker-compose down

dev-logs: ## View development logs
	docker-compose logs -f api

dev-restart: ## Restart development environment
	docker-compose restart api

dev-clean: ## Clean development environment (removes volumes)
	@echo "Cleaning development environment..."
	docker-compose down -v --remove-orphans

## Database Migrations
migrate-up: ## Run database migrations
	@echo "Running migrations..."
	docker-compose run --rm migrate

migrate-down: ## Rollback last migration
	@echo "Rolling back migration..."
	docker run --rm -v $(PWD)/migrations:/migrations \
		--network eagle-bank_eagle-bank-network \
		migrate/migrate:v4.17.0 \
		-path /migrations \
		-database "postgres://eaglebank:eaglebank123@postgres:5432/eaglebank?sslmode=disable" \
		down 1

migrate-create: ## Create new migration (usage: make migrate-create NAME=migration_name)
	@if [ -z "$(NAME)" ]; then echo "Error: NAME is required. Usage: make migrate-create NAME=migration_name"; exit 1; fi
	@echo "Creating migration: $(NAME)"
	@NEXT_NUM=$$(ls -1 migrations/*.up.sql 2>/dev/null | wc -l | tr -d ' '); \
	NEXT_NUM=$$((NEXT_NUM + 1)); \
	PADDED=$$(printf "%06d" $$NEXT_NUM); \
	touch migrations/$${PADDED}_$(NAME).up.sql migrations/$${PADDED}_$(NAME).down.sql
	@echo "Created migrations/$${PADDED}_$(NAME).{up,down}.sql"

migrate-status: ## Check migration status
	@echo "Checking migration status..."
	docker run --rm -v $(PWD)/migrations:/migrations \
		--network eagle-bank_eagle-bank-network \
		migrate/migrate:v4.17.0 \
		-path /migrations \
		-database "postgres://eaglebank:eaglebank123@postgres:5432/eaglebank?sslmode=disable" \
		version

## Helm
helm-lint: ## Lint Helm chart
	@echo "Linting Helm chart..."
	helm lint helm/eagle-bank

helm-template: ## Render Helm templates
	@echo "Rendering Helm templates..."
	helm template eagle-bank helm/eagle-bank

helm-template-dev: ## Render Helm templates for dev
	@echo "Rendering Helm templates for dev..."
	helm template eagle-bank-dev helm/eagle-bank \
		--set config.app.environment=development \
		--set replicaCount=1

helm-install: ## Install Helm chart
	@echo "Installing Helm chart..."
	helm install eagle-bank helm/eagle-bank \
		--namespace eagle-bank \
		--create-namespace

helm-upgrade: ## Upgrade Helm release
	@echo "Upgrading Helm release..."
	helm upgrade eagle-bank helm/eagle-bank \
		--namespace eagle-bank

helm-uninstall: ## Uninstall Helm release
	@echo "Uninstalling Helm release..."
	helm uninstall eagle-bank --namespace eagle-bank

helm-package: ## Package Helm chart
	@echo "Packaging Helm chart..."
	helm package helm/eagle-bank

## Code Generation
generate: ## Run go generate
	@echo "Running go generate..."
	$(GOCMD) generate ./...

mocks: ## Generate mocks (requires mockgen)
	@echo "Generating mocks..."
	@which mockgen > /dev/null || (echo "Installing mockgen..." && go install github.com/golang/mock/mockgen@latest)
	mockgen -source=internal/repository/user_repository.go -destination=internal/repository/mocks/user_repository_mock.go
	mockgen -source=internal/repository/account_repository.go -destination=internal/repository/mocks/account_repository_mock.go
	mockgen -source=internal/repository/transaction_repository.go -destination=internal/repository/mocks/transaction_repository_mock.go

## Security
security-scan: ## Run security scan (requires gosec)
	@echo "Running security scan..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	gosec -fmt=json -out=security-report.json ./...

vuln-check: ## Check for vulnerabilities (requires govulncheck)
	@echo "Checking for vulnerabilities..."
	@which govulncheck > /dev/null || (echo "Installing govulncheck..." && go install golang.org/x/vuln/cmd/govulncheck@latest)
	govulncheck ./...

## CI/CD
ci: deps lint test build ## Run CI pipeline

cd: docker-build docker-push ## Run CD pipeline

## Info
version: ## Show version information
	@echo "App: $(APP_NAME)"
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"
	@echo "Go Version: $(GO_VERSION)"

## Help
help: ## Show this help
	@echo "Eagle Bank - Available Commands:"
	@echo ""
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'
