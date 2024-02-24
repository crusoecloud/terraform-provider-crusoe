# Set any default go build tags
BUILDTAGS :=

GOLANGCI_VERSION = v1.50.1
TFPLUGINDOCS_VERSION = v0.18.0
GO_ACC_VERSION = latest
GOTESTSUM_VERSION = latest
GOCOVER_VERSION = latest

default: install

.PHONY: dev
dev: build-deps test lint docs

.PHONY: build-deps
build-deps:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)

.PHONY: ci
ci: test-ci build-deps lint-ci docs-ci ## Runs test, build-deps, lint and docs

.PHONY: install
install:
	go install ./cmd/terraform-provider-crusoe.go

.PHONY: test
test:
	@echo "==> $@"
	@go test -count=1 -parallel=4 ./...

.PHONY: test-ci
test-ci: ## Runs the go tests with additional options for a CI environment
	@echo "==> $@"
	@go mod tidy
	@git diff --exit-code go.mod go.sum # fail if go.mod is not tidy
	@go install github.com/ory/go-acc@${GO_ACC_VERSION}
	@go install gotest.tools/gotestsum@${GOTESTSUM_VERSION}
	@go install github.com/boumenot/gocover-cobertura@${GOCOVER_VERSION}
	@gotestsum --junitfile tests.xml --raw-command -- go-acc -o coverage.out --ignore ./... -- -json -tags "$(BUILDTAGS)" -race -v
	@go tool cover -func=coverage.out
	@gocover-cobertura < coverage.out > coverage.xml

.PHONY: precommit
precommit: test
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	@golangci-lint run --fix

.PHONY: lint
lint:
	@echo "==> $@"
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	@golangci-lint run

.PHONY: lint-ci
lint-ci: ## Verifies `golangci-lint` passes and outputs in CI-friendly format
	@echo "==> $@"
	@golangci-lint version
	@golangci-lint run ./... --out-format code-climate > golangci-lint.json

.PHONY: docs
docs: build-deps
	@echo "==> $@"
	@go generate ./...
	@git diff --exit-code docs/* || (printf '\e[1;31m%s\e[0m\n' "doc generation produced a diff, make sure to check these in" && false)

.PHONY: docs-ci
docs-ci: build-deps
	@echo "==> $@"
	@go generate ./...
	@git diff --exit-code docs/* # fail if doc autogen produces a diff
