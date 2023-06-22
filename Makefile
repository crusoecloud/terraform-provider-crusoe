GOLANGCI_VERSION = v1.50.1

default: install

generate:
	go generate ./...

install:
	go install ./cmd/terraform-provider-crusoe.go

test:
	go test -count=1 -parallel=4 ./...

precommit: test
	golangci-lint run --fix

lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_VERSION)
	golangci-lint run
