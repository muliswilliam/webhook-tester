# Project variables
SWAG_CMD=swag
SWAG_OUT=docs
SWAG_MAIN=cmd/main.go
APP_NAME=webhook-tester
DOCKER_COMPOSE=docker-compose

.PHONY: help up down logs restart services docs css test test-cover

# Run the Go test suite
test:
	go test ./...

# Run the Go test suite with a statement coverage summary
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -1

docs:
	@echo "🔄 Generating Swagger docs..."
	$(SWAG_CMD) init --parseDependency --parseInternal -g $(SWAG_MAIN)
	@echo "✅ Swagger docs generated in ./$(SWAG_OUT)"

# Rebuild static/css/tailwind.css after changing templates or Tailwind classes
css:
	@echo "🎨 Building Tailwind CSS..."
	npx --yes tailwindcss@3 -i static/css/input.css -o static/css/tailwind.css --minify
	@echo "✅ CSS built to static/css/tailwind.css"

# Start all services
up:
	$(DOCKER_COMPOSE) up -d

# Stop all services
down:
	$(DOCKER_COMPOSE) down

# Build all services
build:
	$(DOCKER_COMPOSE) build --no-cache

# Status of services
ps:
	$(DOCKER_COMPOSE) ps

# Reset everything (⚠️ destructive)
reset:
	$(DOCKER_COMPOSE) down -v --remove-orphans

# Dynamic targets: make restart SERVICE=name
restart:
	@$(DOCKER_COMPOSE) restart $(SERVICE)

logs:
	@$(DOCKER_COMPOSE) logs -f $(SERVICE)

sh:
	@$(DOCKER_COMPOSE) exec $(SERVICE) sh

# List all service names from docker-compose
services:
	@echo "Available services:"
	@$(DOCKER_COMPOSE) config --services

# Example helper
help:
	@echo "Usage: make [target] [SERVICE=service_name]"
	@echo ""
	@echo "Available targets:"
	@echo "  up               Build and start the app with Docker Compose"
	@echo "  down             Stop and remove containers"
	@echo "  logs             View logs (requires SERVICE=app or SERVICE=db)"
	@echo "  restart          Restart a specific service (requires SERVICE=app or SERVICE=db)"
	@echo "  services         List available service names"
	@echo "  css              Rebuild static/css/tailwind.css from templates"
	@echo "  test             Run the Go test suite"
	@echo "  test-cover       Run the Go test suite with coverage summary"