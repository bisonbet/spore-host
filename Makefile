# Makefile for spore-host infrastructure
#
# The CLI tools (truffle, spawn, lagotto, spore-host-mcp) live in their own repos:
#   https://github.com/spore-host/truffle
#   https://github.com/spore-host/spawn
#   https://github.com/spore-host/lagotto
#   https://github.com/spore-host/spore-host-mcp
#
# This repo holds infrastructure: the rest-api Lambda, dashboard (web/),
# Python SDK (sdk/), deployment scripts (scripts/), AMIs (infra/), and docs.

.PHONY: build-rest-api test-rest-api docs-dev docs-build security clean help

# Build the hosted REST API Lambda
build-rest-api:
	@echo "Building rest-api Lambda..."
	@cd lambda/rest-api && GOOS=linux GOARCH=arm64 go build -o bootstrap .
	@echo "✅ lambda/rest-api/bootstrap"

# Test the rest-api Lambda
test-rest-api:
	@cd lambda/rest-api && go test ./...

# Run the VitePress docs site locally
docs-dev:
	@cd docs && npm run dev

# Build the VitePress docs site
docs-build:
	@cd docs && npm run build

# Security scans (govulncheck on the Lambda, trivy on infra)
security:
	@cd lambda/rest-api && go vet ./... && govulncheck ./... || true

clean:
	@rm -f lambda/rest-api/bootstrap lambda/rest-api/*.zip

help:
	@echo "spore-host infrastructure targets:"
	@echo "  build-rest-api  Build the hosted REST API Lambda"
	@echo "  test-rest-api   Test the rest-api Lambda"
	@echo "  docs-dev        Run the VitePress docs site locally"
	@echo "  docs-build      Build the VitePress docs site"
	@echo "  security        Run security scans"
	@echo "  clean           Remove build artifacts"
