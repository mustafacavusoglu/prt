VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/mustafacavusoglu/prt/cmd.version=$(VERSION)

.PHONY: build install test lint fmt vet cover clean

build:
	go build -ldflags "$(LDFLAGS)" -o prt .

install:
	go install -ldflags "$(LDFLAGS)" .

test:
	go test -race ./...

cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -1

fmt:
	gofmt -w .

vet:
	go vet ./...
	GOOS=darwin go vet ./...
	GOOS=windows go vet ./...

lint: vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt gerekli:"; gofmt -l .; exit 1)

clean:
	rm -rf prt prt.exe dist coverage.out
