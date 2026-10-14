# Development

jenklod-batman is Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea). Go's version comes from `go.mod` (the repository's `mise.toml` pins it too).

```sh
make build        # bin/jenklod-batman and its jk link, stamped with git describe
make test         # go test ./...
make vet
```

On Linux, building needs cgo and the ALSA headers for the [theme music](/guide/music): `sudo apt install libasound2-dev` (or `alsa-lib-devel`). macOS needs nothing extra.

## Layout

| Path | What |
|---|---|
| `main.go`, `cli.go` | Flags and the `run` subcommand |
| `internal/jenkins` | The Jenkins REST client |
| `internal/config` | `config.toml`: pins, macros, per-controller scoping |
| `internal/macro` | The macro runner shared by the TUI and the CLI |
| `internal/ui` | The TUI: model, views, search, macro editor |
| `internal/music`, `cmd/music-sample` | The theme: its score, the SID-style synthesizer, the player, and the renderer for the docs' sample |
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

It also plays GitHub for the branch picker: its builds check out `wayne-enterprises/gotham`, whose fictional branches it serves on `/api/graphql`. Point the app there with `JB_GITHUB_API`; then it sends no token and doesn't ask `gh`:

```sh
JB_GITHUB_API=http://127.0.0.1:8765/api/graphql JENKINS_TOKEN=demo go run . --config /tmp/demo.toml
```

## Docs

```sh
cd docs
npm ci
npm run dev       # http://localhost:5173/
```

The screenshots and recordings in `docs/public/media` come from the [vhs](https://github.com/charmbracelet/vhs) scripts in `docs/tapes`, played against the demo Jenkins. Regenerate them after a UI change:

```sh
brew install vhs
make docs-media   # or: docs/tapes/record.sh tour macros
```

The music page's sample, `docs/public/media/gotham.ogg`, is the theme rendered offline. Re-render it after changing the score or the synthesizer (needs ffmpeg with libopus):

```sh
make music-sample
```

Each recording starts a fresh demo server and a copy of `docs/tapes/demo.toml`. The outputs are committed, and CI only builds and publishes the site. Pushing to `main` with changes under `docs/` or to `CHANGELOG.md` publishes it.

vhs takes a `Screenshot` on the next frame, after the following command has run, so put a `Sleep` right after each one.
