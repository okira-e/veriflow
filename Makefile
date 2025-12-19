APP := veriflow

build:
	go build -o bin/$(APP) .

release:
	GOOS=$(GOOS) GOARCH=$(GOARCH) \
	go build -ldflags="-s -w" -trimpath -o bin/$(APP) .

test:
	go test ./...

fmt:
	go fmt ./...