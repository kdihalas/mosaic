.PHONY: build test test-race vet lint fmt example golden
build:
	go build ./cmd/mosaic
test:
	go test ./...
test-race:
	go test -race ./...
vet:
	go vet ./...
lint:
	go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2 run ./...
fmt:
	test -z "$$(gofmt -l .)"
example:
	go run ./cmd/mosaic --project ./examples/catalog-platform validate
	go run ./cmd/mosaic --project ./examples/catalog-platform build dev
	go run ./cmd/mosaic --project ./examples/catalog-platform build stage
	go run ./cmd/mosaic --project ./examples/catalog-platform build prod
	go run ./cmd/mosaic --project ./examples/catalog-platform test
golden:
	UPDATE_GOLDEN=1 go test ./...
