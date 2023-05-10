default: install

generate:
	go generate ./...

install:
	go install ./cmd/terraform-provider-crusoe.go

test:
	go test -count=1 -parallel=4 ./...

lint:
	golangci-lint run --fix
