APP := veriflow

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

VERSION := $(shell git describe --tags --dirty --always)
COMMIT  := $(shell git rev-parse --short HEAD)
BUILT   := $(shell date -u +%Y-%m-%d)

LDFLAGS := -s -w \
	-X github.com/okira-e/veriflow/app/version.Version=$(VERSION) \
	-X github.com/okira-e/veriflow/app/version.Commit=$(COMMIT) \
	-X github.com/okira-e/veriflow/app/version.Built=$(BUILT)

build:
	go build -o bin/debug/$(APP) .

release:
	@test -n "$$(git tag --points-at HEAD)" || (echo "ERROR: release must be from a git tag" && exit 1)
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 \
	go build -trimpath -ldflags "$(LDFLAGS)" \
	-o bin/release/$(APP) .

test:
	go test -v ./...

fmt:
	go fmt ./...