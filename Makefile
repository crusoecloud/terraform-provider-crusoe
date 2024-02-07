GOLANGCI_VERSION = v1.50.1
TFPLUGINDOCS_VERSION = v0.18.0

default: install

.PHONY: dev
dev: build-deps lint docs ## TODO: CRUSOE-6492 add tests to this once we have fixed the test suite

.PHONY: build-deps
build-deps:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION)

.PHONY: docs
docs: build-deps
	go generate ./...
	@git diff --exit-code docs/* || (printf '\e[1;31m%s\e[0m\n' "doc generation produced a diff, make sure to check these in" && false)

.PHONY: install
install:
	go install ./cmd/terraform-provider-crusoe.go

.PHONY: test
test:
	go test -count=1 -parallel=4 ./...

.PHONY: precommit
precommit: test
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	golangci-lint run --fix

.PHONY: lint
lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	golangci-lint run
