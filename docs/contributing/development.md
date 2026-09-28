# Development

jenklod-batman is Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Go's version comes from `go.mod` (the repository's `mise.toml` pins it too).

```sh
make build        # bin/jenklod-batman, stamped with git describe
make test         # go test ./...
make vet
```

## Layout

| Path | What |
|---|---|
| `main.go`, `cli.go` | Flags and the `run` subcommand |
| `internal/jenkins` | The Jenkins REST client |
| `internal/config` | `config.toml`: pins, macros, per-controller scoping |
| `internal/macro` | The macro runner shared by the TUI and the CLI |
| `internal/ui` | The TUI: model, views, search, macro editor |
| `internal/demo`, `cmd/demo-jenkins` | The simulated Jenkins used for the docs and for trying the app |
| `docs/` | This site (VitePress) |

## Tests

The UI tests drive the model against a fake Jenkins, without a terminal. To look at the rendered screens:

```sh
JB_SNAPSHOTS=/tmp/snap go test ./internal/ui/
```

## The demo Jenkins

`go run ./cmd/demo-jenkins` serves a fictional Gotham on `127.0.0.1:8765`: nested folders, pipelines with parameters, a deploy that pauses on *Deploy to prod?*, and some history. Triggered builds wait briefly in the queue, then log progress and finish. Any token works:

```sh
go run ./cmd/demo-jenkins &
cp docs/tapes/demo.toml /tmp/demo.toml
JENKINS_TOKEN=demo go run . --config /tmp/demo.toml
```

## Docs

```sh
cd docs
npm ci
npm run dev       # http://localhost:5173/jenklod-batman/
```

The screenshots and recordings in `docs/public/media` come from the [vhs](https://github.com/charmbracelet/vhs) scripts in `docs/tapes`, played against the demo Jenkins. Regenerate them after a UI change:

```sh
brew install vhs
make docs-media   # or: docs/tapes/record.sh tour macros
```

Each recording starts a fresh demo server and a copy of `docs/tapes/demo.toml`. The outputs are committed, and CI only builds and publishes the site. Pushing to `main` with changes under `docs/` publishes it.
