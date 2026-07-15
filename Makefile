.PHONY: build test test-race vet lint fmt example golden package-test package-example package-pack package-registry-test dependency-test
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
	go run ./cmd/mosaic --project ./examples/operator-integrations validate
	go run ./cmd/mosaic --project ./examples/operator-integrations build dev
	go run ./cmd/mosaic --project ./examples/operator-integrations build prod
	go run ./cmd/mosaic --project ./examples/operator-integrations test
golden:
	UPDATE_GOLDEN=1 go test ./...
package-test:
	go test ./pkg/package ./pkg/packagearchive ./pkg/packagecache ./pkg/vendor ./pkg/compiler
dependency-test:
	go test ./pkg/dependency ./pkg/lockfile ./pkg/packagefs
package-example:
	go run ./cmd/mosaic package validate ./examples/packages/observability
	go run ./cmd/mosaic package validate ./examples/packages/http-service
	go run ./cmd/mosaic --project ./examples/packages/catalog-platform --cache-dir /tmp/mosaic-package-cache deps resolve
	go run ./cmd/mosaic --project ./examples/packages/catalog-platform --cache-dir /tmp/mosaic-package-cache build prod --locked --output /tmp/mosaic-package-build
	go run ./cmd/mosaic --project ./examples/packages/catalog-platform --cache-dir /tmp/mosaic-package-cache deps vendor --offline --output /tmp/mosaic-package-vendor
package-pack:
	go run ./cmd/mosaic package pack ./examples/packages/http-service --output /tmp/http-a.mosaicpkg
	go run ./cmd/mosaic package pack ./examples/packages/http-service --output /tmp/http-b.mosaicpkg
	cmp /tmp/http-a.mosaicpkg /tmp/http-b.mosaicpkg
package-registry-test:
	go test ./pkg/registry/...
