SWAG_CMD=swag
SWAG_OUT=docs
SWAG_MAIN=cmd/main.go

.PHONY: docs

docs:
	@echo "🔄 Generating Swagger docs..."
	$(SWAG_CMD) init --parseDependency --parseInternal -g $(SWAG_MAIN)
	@echo "✅ Swagger docs generated in ./$(SWAG_OUT)"