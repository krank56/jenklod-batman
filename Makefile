VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Fixed install dir: mise sets GOBIN inside its versioned Go install.
INSTALL_DIR ?= $(HOME)/go/bin

.PHONY: build install test vet run clean dist

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

# Release archives for the GitHub release and the Homebrew tap.
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
dist:
	rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=jenklod-batman_$(VERSION)_$${os}_$${arch}; \
		mkdir -p dist/$$name && \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$$name/jenklod-batman . && \
		cp LICENSE README.md dist/$$name/ && \
		tar -C dist -czf dist/$$name.tar.gz $$name && rm -rf dist/$$name || exit 1; \
	done
	cd dist && shasum -a 256 *.tar.gz > checksums.txt

clean:
	rm -rf bin dist
