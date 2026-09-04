MODULE  := github.com/justwaters/sitrep
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE    := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X '$(MODULE)/internal/buildinfo.Version=$(VERSION)' \
           -X '$(MODULE)/internal/buildinfo.Commit=$(COMMIT)' \
           -X '$(MODULE)/internal/buildinfo.Date=$(DATE)'

.PHONY: build test vet fmt lint linux clean

build:
	go build -ldflags "$(LDFLAGS)" -o bin/sitrep ./cmd/sitrep

# Sitrep only targets Linux at runtime (systemd, apt/dnf/pacman, iputils
# ping semantics), but development happens on whatever host you're on —
# use `make linux` to cross-compile the artifact you'll actually deploy.
linux:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/sitrep-linux-amd64 ./cmd/sitrep
	GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/sitrep-linux-arm64 ./cmd/sitrep

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l .

clean:
	rm -rf bin/
