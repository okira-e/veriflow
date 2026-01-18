APP := veriflow

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
LDFLAGS := -s -w

build:
	go build -o bin/debug/$(APP) .

release:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 \
	go build -trimpath -ldflags "$(LDFLAGS)" \
	-o bin/release/$(APP) .

test:
	go test -v ./...

fmt:
	go fmt ./...