GOLANGCI_VERSION = v1.47.3

default: install

generate:
	go generate ./...

install:
	go install ./cmd/terraform-provider-crusoe.go

test:
	go test -count=1 -parallel=4 ./...

precommit:
	golangci-lint run --fix

lint:
	golangci-lint run
