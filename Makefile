.PHONY: build test check-style clean deploy bundle

GO ?= go
PLUGIN_ID ?= com.klab.mattermost-command-center
BUNDLE_NAME ?= $(PLUGIN_ID).tar.gz

# Build targets
GOOS_LINUX = linux
GOOS_DARWIN = darwin
GOARCH_AMD64 = amd64
GOARCH_ARM64 = arm64

## build: Build the plugin server binary for all platforms
build:
	mkdir -p server/dist
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_AMD64) $(GO) build -o server/dist/plugin-linux-amd64 ./server
	GOOS=$(GOOS_LINUX) GOARCH=$(GOARCH_ARM64) $(GO) build -o server/dist/plugin-linux-arm64 ./server
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_AMD64) $(GO) build -o server/dist/plugin-darwin-amd64 ./server
	GOOS=$(GOOS_DARWIN) GOARCH=$(GOARCH_ARM64) $(GO) build -o server/dist/plugin-darwin-arm64 ./server

## webapp: Build the webapp plugin component
webapp:
	cd webapp && npm install --no-audit --no-fund && npm run build

## bundle: Build and package the plugin as a tar.gz bundle
bundle: build webapp
	rm -f $(BUNDLE_NAME)
	tar -czf $(BUNDLE_NAME) \
		plugin.json \
		assets/ \
		server/dist/ \
		webapp/dist/

## test: Run all tests
test:
	$(GO) test ./server/... -v -count=1

## test-short: Run tests in short mode (skip long-running tests)
test-short:
	$(GO) test ./server/... -v -count=1 -short

## check-style: Run linter
check-style:
	$(GO) vet ./server/...
	@echo "Style check passed"

## clean: Remove build artifacts
clean:
	rm -rf server/dist
	rm -f $(BUNDLE_NAME)

## deploy: Upload plugin to Mattermost server (requires MM_SERVICESETTINGS_SITEURL and MM_ADMIN_TOKEN)
deploy: bundle
	curl -sSf -X POST \
		-H "Authorization: Bearer $(MM_ADMIN_TOKEN)" \
		$(MM_SERVICESETTINGS_SITEURL)/api/v4/plugins \
		-F "plugin=@$(BUNDLE_NAME)" \
		-F "force=true"
	@echo "\nPlugin deployed successfully"

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | sort
