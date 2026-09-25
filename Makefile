VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Fixed install dir: mise sets GOBIN inside its versioned Go install.
INSTALL_DIR ?= $(HOME)/go/bin

.PHONY: build install test vet run clean

build:
	go build -ldflags '$(LDFLAGS)' -o bin/jenklod-batman .

install:
	GOBIN=$(INSTALL_DIR) go install -ldflags '$(LDFLAGS)' .

test:
	go test ./...

vet:
	go vet ./...

run: build
	./bin/jenklod-batman

clean:
	rm -rf bin
