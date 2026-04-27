GO ?= go

# Bake the relay's signup URL into the binary so friends can run `./digger`
# with zero setup. Examples:
#   make tui SIGNUP=http://my.relay.host:7778/
#   make tui JOIN=pl1://abcd-1234@my.relay.host:7777
#
# Without either, the binary launches with the manual paste screen.

SIGNUP ?=
JOIN ?=
LDFLAGS  = -X 'main.defaultSignup=$(SIGNUP)' -X 'main.defaultJoin=$(JOIN)'

OUT ?= digger

.PHONY: tui all linux darwin-arm64 darwin-amd64 windows clean

tui:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(OUT) ./cmd/digger

all: linux darwin-arm64 darwin-amd64 windows

linux:
	GOOS=linux   GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-linux-amd64   ./cmd/digger
darwin-arm64:
	GOOS=darwin  GOARCH=arm64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-darwin-arm64  ./cmd/digger
darwin-amd64:
	GOOS=darwin  GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-darwin-amd64  ./cmd/digger
windows:
	GOOS=windows GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o dist/digger-windows-amd64.exe ./cmd/digger

clean:
	rm -rf $(OUT) dist/
