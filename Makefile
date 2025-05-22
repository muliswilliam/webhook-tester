# Project variables
SWAG_CMD=swag
SWAG_OUT=docs
SWAG_MAIN=cmd/main.go
APP_NAME=webhook-tester
DOCKER_COMPOSE=docker-compose

.PHONY: help up down logs restart services docs generate coverage

coverage:
	go test -v -coverprofile=coverage.out ./... && go tool cover -html=coverage.out -o coverage.html && open coverage.html

docs:
	@echo "🔄 Generating Swagger docs..."
	$(SWAG_CMD) init --parseDependency --parseInternal -g $(SWAG_MAIN)
	@echo "✅ Swagger docs generated in ./$(SWAG_OUT)"

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