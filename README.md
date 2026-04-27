# digger

The TUI client for [digger.gg](https://digger.gg) — host a game server,
anywhere.

```
$ ./digger
```

Pick a game with `g`, share the address that appears.

## Build

```
go build -o digger ./cmd/digger
```

To bake in a relay's signup URL (so the binary auto-connects on first
launch with no paste step):

```
make tui SIGNUP=https://your.relay.host:7778/
```

Cross-compile for distribution:

```
make all SIGNUP=https://your.relay.host:7778/
# dist/digger-linux-amd64
# dist/digger-darwin-arm64
# dist/digger-darwin-amd64
# dist/digger-windows-amd64.exe
```

## Wire protocol

Length-prefixed (u32 BE) MessagePack frames. Adjacently-tagged enums:
`{ "t": "VariantName", "c": <payload> }`. See `proto/proto.go`.

The client speaks to a digger relay over a single outbound TCP connection
on port 7777. Relay source is in a separate repo.

MIT licensed.
