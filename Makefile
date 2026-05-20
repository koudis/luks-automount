BINARY  := luks-automount
PACKAGE := ./cmd/luks-automount
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -extldflags '-static'"

.PHONY: release test test-integration vet clean

release:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64 $(PACKAGE)

test:
	go test ./...

test-integration:
	go test -tags integration -timeout 30m -run TestSmoke_WorkerProtocol -v ./internal/worker

vet:
	go vet ./...

clean:
	rm -f $(BINARY)-linux-amd64