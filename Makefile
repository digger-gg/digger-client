GO ?= go

# Bake the relay's signup URL into the binary so friends can run `./digger`
# with zero setup.
#   make tui SIGNUP=http://my.relay.host:7778/
#   make tui JOIN=pl1://abcd-1234@my.relay.host:7777
#
# Without either, the binary launches with the manual paste screen.

SIGNUP ?=
JOIN ?=
LDFLAGS  = -X 'main.defaultSignup=$(SIGNUP)' -X 'main.defaultJoin=$(JOIN)'

OUT ?= digger

.PHONY: tui all archives bare clean linux darwin-arm64 darwin-amd64 windows \
        archive-linux-amd64 archive-darwin-arm64 archive-darwin-amd64 archive-windows-amd64

tui:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/digger

# `make all` produces both bare binaries (for the curl|sh installer)
# and platform-appropriate archives (for the website's download buttons,
# so browsers don't render the binary as text).
all: bare archives

bare: linux darwin-arm64 darwin-amd64 windows
archives: archive-linux-amd64 archive-darwin-arm64 archive-darwin-amd64 archive-windows-amd64

linux:
	GOOS=linux   GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-linux-amd64   ./cmd/digger
darwin-arm64:
	GOOS=darwin  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-darwin-arm64  ./cmd/digger
darwin-amd64:
	GOOS=darwin  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-darwin-amd64  ./cmd/digger
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-windows-amd64.exe ./cmd/digger

# Each archive contains a single file named `digger` (or `digger.exe`)
# so that what users extract is exactly what they want to run.
# tar preserves the executable bit, so no chmod step needed.
archive-linux-amd64: linux
	@cd dist && cp digger-linux-amd64 digger && tar -czf digger-linux-amd64.tar.gz digger && rm digger
archive-darwin-arm64: darwin-arm64
	@cd dist && cp digger-darwin-arm64 digger && tar -czf digger-darwin-arm64.tar.gz digger && rm digger
archive-darwin-amd64: darwin-amd64
	@cd dist && cp digger-darwin-amd64 digger && tar -czf digger-darwin-amd64.tar.gz digger && rm digger
archive-windows-amd64: windows
	@cd dist && cp digger-windows-amd64.exe digger.exe && zip -q digger-windows-amd64.zip digger.exe && rm digger.exe

clean:
	rm -rf $(OUT) dist/
