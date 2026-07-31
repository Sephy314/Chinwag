SERVICES := gateway shared/auth services/auth services/room services/chat/command services/chat/query

.PHONY: help fmt test test-race test-cover vet run-gateway run-auth run-room run-chat-command run-chat-query test-all

help: ## Show available commands
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*##/ {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Format Go source files in every backend service
	@gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

define run-tests
	@for service in $(SERVICES); do \
		pkgs="$$(cd $$service && go list -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}{{end}}' ./... | awk 'NF')"; \
		if [ -n "$$pkgs" ]; then \
			(cd $$service && go test $(1) $$pkgs) || exit 1; \
		fi; \
	done
endef

test: ## Run tests for every backend Go module
	$(call run-tests)

test-all: ## Run all tests for every backend Go module (race detector)
	$(call run-tests,-race)

test-race: ## Run all tests with the race detector
	$(call run-tests,-race)

test-cover: ## Run all tests and print coverage per package
	$(call run-tests,-cover)

vet: ## Run go vet for every backend Go module
	@for service in $(SERVICES); do \
		echo "==> $$service"; \
		(cd $$service && go vet ./...); \
	done

run-gateway: ## Run the API gateway
	@cd gateway && go run .

run-auth: ## Run the auth service
	@cd services/auth && go run .

run-room: ## Run the room service
	@cd services/room && go run .

run-chat-command: ## Run the chat command service
	@cd services/chat/command && go run .

run-chat-query: ## Run the chat query service
	@cd services/chat/query && go run .