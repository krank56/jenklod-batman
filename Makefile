VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
# Fixed install dir: mise sets GOBIN inside its versioned Go install.
INSTALL_DIR ?= $(HOME)/go/bin

.PHONY: build install test vet run clean dist docs docs-media music-sample

# Each also links jk, the short name, next to the binary.
build:
	go build -ldflags '$(LDFLAGS)' -o bin/jenklod-batman .
	ln -sf jenklod-batman bin/jk

install:
	GOBIN=$(INSTALL_DIR) go install -ldflags '$(LDFLAGS)' .
	ln -sf jenklod-batman $(INSTALL_DIR)/jk

test:
	go test ./...

vet:
	go vet ./...

run: build
	./bin/jenklod-batman

# Release archives for the GitHub release and the Homebrew tap.
# Linux builds need cgo for the music (ALSA), so only the host's Linux
# platform builds here: the release workflow runs one Linux runner per
# architecture. macOS needs no cgo and builds anywhere.
HOST_OS := $(shell go env GOOS)
HOST_ARCH := $(shell go env GOARCH)
PLATFORMS ?= darwin/amd64 darwin/arm64 $(if $(filter linux,$(HOST_OS)),linux/$(HOST_ARCH))
dist:
	rm -rf dist && mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; name=jenklod-batman_$(VERSION)_$${os}_$${arch}; \
		cgo=0; [ $$os = linux ] && cgo=1; \
		mkdir -p dist/$$name && \
		CGO_ENABLED=$$cgo GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o dist/$$name/jenklod-batman . && \
		ln -s jenklod-batman dist/$$name/jk && \
		cp LICENSE README.md dist/$$name/ && \
		tar -C dist -czf dist/$$name.tar.gz $$name && rm -rf dist/$$name || exit 1; \
	done
	cd dist && shasum -a 256 *.tar.gz > checksums.txt

# The docs site (docs/). docs-media re-records its screenshots and demos
# with vhs against the simulated Jenkins in cmd/demo-jenkins.
docs:
	cd docs && npm ci && npm run build

docs-media:
	docs/tapes/record.sh tour search macros input cli rebuild branches

# The docs' audio sample: one pass of the theme, rendered offline (needs
# ffmpeg with libopus).
music-sample:
	mkdir -p dist
	go run ./cmd/music-sample dist/gotham.wav
	ffmpeg -loglevel error -y -i dist/gotham.wav -c:a libopus -b:a 96k docs/public/media/gotham.ogg
	rm dist/gotham.wav

clean:
	rm -rf bin dist
