# ---- config ----
BIN       		:= veriflow
PKG       		:= ./main.go
VERSION   		:= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOOS      		?= $(shell go env GOOS)
GOARCH    		?= $(shell go env GOARCH)
OUT       		:= bin/$(BIN)-$(GOOS)-$(GOARCH)-debug
RELEASE_OUT     := bin/$(BIN)-$(GOOS)-$(GOARCH)
LDFLAGS   		:= -s -w
VERSION_FLAGS	:= -X main.version=$(VERSION)
GOFLAGS   		:= -trimpath

.DEFAULT_GOAL 	:= build
.PHONY: build run test fmt clean help

# ---- tasks ----
build:
	@mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build -ldflags "$(VERSION_FLAGS)" -o $(OUT) $(PKG)
	@echo "built $(OUT)"

release:
	@mkdir -p bin
	GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(GOFLAGS) -ldflags "$(LDFLAGS) $(VERSION_FLAGS)" -o $(RELEASE_OUT) $(PKG)
	@echo "built $(RELEASE_OUT)"

run: build
	@$(OUT) $(ARGS)

test:
	go test ./...

fmt:
	go fmt ./...

clean:
	rm -rf bin

help:
	@echo "make [build|run ARGS='... '|test|fmt|clean]"