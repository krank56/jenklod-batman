# Changelog

What changed in each release of jenklod-batman. `jk changelog` prints this file, and the first launch after an upgrade shows what's new.

## v0.5.0 (unreleased)

### Theme music
- `M` plays *Gotham by Night*, an original keygen-style chiptune in D minor, synthesized as it plays like a C64's SID chip. `M` again stops it. It's off until you turn it on, and saved as `music` in the config, so music left on plays from the next launch.
- `+` (or `=`) and `-` change its volume in steps of 10% while it plays, saved as `music_volume`.
- While it plays, the header shows `♫` and the cat dances to the beat.
- Without a sound device the status bar says *Music unavailable* and the app carries on silently.
- On Linux, the binary now needs ALSA's `libasound.so.2`, and `go install` needs `libasound2-dev` and a C compiler.

## v0.4.0 (2026-10-13)

### `jk`
- `jk` is short for `jenklod-batman`, after the keys that move the cursor. Homebrew, the release archives and `make install` put it next to the binary.
- Usage lines and hints show the name you ran: `jk ship <VERSION>` when you ran `jk`.

### Builds
- A build you start shows up in the list on its own. While it waits in the Jenkins queue it's a `QUEUED` row with Jenkins' reason; once it starts, the cursor moves to it.
- The builds list refreshes every 2s while a build is queued and every 5s while one runs. New builds no longer move the cursor off the build it was on.
- Leave the screen and the build is still followed: the status bar says when it starts, or that it was cancelled.
- Rebuilt with `R` from a log, the screen switches to the new build's log once it starts.

### Branch picker
- In the build form, `ctrl+g` on a text field lists the job's GitHub branches: the default branch first, then the most recently committed to. Typing filters, `enter` fills the field, and text that matches nothing is kept.
- Branches come from GitHub through the `gh` CLI's login. The token is never stored.
- The repository is the one the job's last build checked out. When that's unclear, the picker asks once and saves the answer under `repos` in the config.

### Macros
- A macro can take an argument: one of its build parameters, set with `A` in the editor (`arg` in the config). The word after its name fills it, so `jk ship v2.8.0` runs `ship` with `VERSION=v2.8.0`.
- `run` is optional: `jk ship` is `jk run ship`. A macro named like a command (`run`, `update`, `changelog`) still needs `run`.
- The macro list and `run --list` show a macro's argument: `ship <VERSION>`.

### Browser
- `o` opens the job's page from the builds list and from a log; `O` opens the selected build, or the log's console.

### Updates
- The TUI shows `↑ vX.Y.Z` in the header when a newer release is out, and says so once in the status bar. It checks at most once a day; `check_updates = false` in the config turns it off.
- `jk update` installs the latest release the way this one was installed: Homebrew, `go install`, or the release archive (checked against `checksums.txt`). `update --check` only says whether there's one.
- `jk changelog` prints this changelog, and the first launch after an upgrade shows what's new.

## v0.3.0 (2026-09-29)

### Rebuild
- `R` opens the build form filled in with the parameters of an earlier build: the selected one in the builds list, the one you're reading in a log, or the job's last build from the jobs list.
- Parameters that changed since are reported in the status bar: dropped, reset to their default, or new.
- Jobs with a password or file parameter can't be rebuilt, since Jenkins doesn't return those values. Use `b` instead.

### Docs and releases
- A documentation site at [jenklod-batman.ytalbi.com](https://jenklod-batman.ytalbi.com), with recorded demos against a simulated Jenkins (`go run ./cmd/demo-jenkins`).
- Releases are built by CI from `v*` tags, for macOS and Linux on amd64 and arm64.

## v0.2.0 (2026-09-28)

### Watched jobs
- The root screen opens with a **Watched** section. Each job shows its last build: number, result, age, duration or live progress, and cause.
- `enter` opens a job, and `b` / `L` / `x` / `w` / `o` act on it directly. `J`/`K` reorder the list.
- If a watched job can't be polled (deleted, renamed or forbidden), it shows `✗` and the reason.
- Watched jobs are now stored per Jenkins URL. The old list moves over automatically.
- Polling defaults to 10s. `p` cycles through 5, 10, 20, 30 and 60s.

### Search
- `/` fuzzy-searches every folder, not just the current one, so `gdep` finds `gotham/deploy`.

### Macros
- Press `m` to build step sequences in the TUI: build with saved parameters, abort, wait for success, answer input.
- Macros run in the background, and you can cancel them.
- From a shell: `jenklod-batman run <name> [KEY=VALUE…] [-y]`, or `run --list` to list them.
- An input step that follows no build of the same macro is skipped when nothing is waiting.

### Install
- `brew install krank56/tap/jenklod-batman`

## v0.1.0 (2026-09-25)

First release: a terminal UI for Jenkins, with a bat-signal splash screen and a cat in the status bar.

- Browse jobs and folders, with status colours and a filter.
- Build history per job: result, start time, duration and cause, plus live progress for running builds.
- Live console log with a follow mode.
- Trigger builds, with a form for parameterized jobs, and abort running ones; both ask for confirmation.
- Answer pipeline `input` steps ("Deploy to prod?"): proceed with parameters, or abort. This needs the Pipeline: Stage View plugin on the controller.
- Watch jobs and get desktop notifications on macOS and Linux when a build finishes or waits for input. Linux needs `notify-send`.
- The API token stays in the system keyring (macOS Keychain, or Secret Service on Linux) and is never written to disk.
