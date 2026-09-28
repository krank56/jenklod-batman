<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="assets/banner-dark.png" width="640">
    <img src="assets/logo-light.png" alt="jenklod-batman: a black cat with bat wings in a yellow spotlight above a >_ prompt" width="200">
  </picture>
</p>

# jenklod-batman 🦇

A terminal UI for managing Jenkins jobs. It has a bat-signal splash screen and a cat that lives in the status bar.

## Install

```sh
brew install krank56/tap/jenklod-batman
# or
go install github.com/krank56/jenklod-batman@latest
```

Prebuilt binaries for macOS and Linux (amd64 and arm64) are attached to each
[release](https://github.com/krank56/jenklod-batman/releases). From a clone:

```sh
make install        # into ~/go/bin (override with INSTALL_DIR=...)
# or
make build && ./bin/jenklod-batman
```

## First run

The first launch opens a setup form that asks for:

- **Jenkins URL**, for example `https://jenkins.example.com`
- **User**: your Jenkins user ID
- **API token**: create one in Jenkins under *your name → Security → API Token → Add new token*

Before saving anything, the app checks the credentials against Jenkins.

- The URL and user go to `~/.config/jenklod-batman/config.toml` (file mode 0600).
- The token goes to the system keyring (service `jenklod-batman`, account `<user>@<url>`). It is never written to disk.
  - **macOS:** Keychain.
  - **Linux:** the Secret Service, i.e. gnome-keyring or KWallet.
- `JENKINS_TOKEN` in the environment overrides the keyring for one-off runs. This helps on headless Linux boxes with no keyring daemon.

```sh
jenklod-batman --setup          # re-enter URL / user / token
jenklod-batman --forget-token   # remove the token from the keyring
jenklod-batman --no-anim        # skip the splash and keep the cat still
```

## Keys

| Key | Action |
|---|---|
| `j`/`k`, `↑`/`↓`, `g`/`G`, `ctrl+d`/`ctrl+u` | move |
| `enter`, `l`, `→` | open folder, then job builds, then build log |
| `h`, `←`, `esc` | back |
| `/` | search: fuzzy matches in the current folder first, then in every other folder |
| `b` | build (a form opens for parameterized jobs; always asks y/n) |
| `x` | abort a running build (asks y/n); in the jobs list, the job's last build |
| `L` | in the jobs list: open the job's last build log |
| `i` | answer a paused pipeline `input` step, e.g. "Deploy to prod?" |
| `w` | watch or unwatch a job |
| `J`/`K` | move a watched job down or up |
| `p` | cycle the watch poll interval: 5, 10, 20, 30, 60s (saved) |
| `m` | macros |
| `o` | open in browser |
| `f` | follow log output |
| `r` | refresh |
| `a` | toggle the cat animation |
| `?` | help |
| `q` | back / quit |

## Pipeline input steps

Many pipelines stop on an `input` step before the production deploy. jenklod-batman shows these pauses:

- **Builds view:** the paused build shows as `⏸ INPUT` with its message, and a banner at the top reads `#42 is waiting: Deploy to prod? — press i`.
- **Log view:** the same banner appears while you follow the console.
- **Answering (`i`):** opens the prompt with any fields the step defines. `enter` runs its proceed action (e.g. *Deploy*) and `ctrl+x` (or `x`) aborts it. Both ask y/n first. `esc` leaves it for later.
- **Watched jobs:** a desktop notification fires once when a watched job's build starts waiting for input.

Listing pending inputs uses the **Pipeline: Stage View** plugin's API (`wfapi`). Without that plugin, `i` says so, and you can use `o` to answer in the browser. Jenkins still checks `submitter` restrictions: if you aren't allowed to approve, you'll see a "forbidden" error.

## Watching

Watched jobs (`w`, shown with ◆) are listed at the top of the root screen, each with its last build: number, result, age, duration (or a progress bar while running) and what triggered it. `enter` opens one as if you had browsed to it. `b`, `L`, `x`, `w` and `o` act on it directly, and `J`/`K` reorder the list.

If a watched job can't be polled (deleted, renamed, or no longer visible to you), its row shows `✗` and the reason until you unwatch it.

While the TUI is open, watched jobs are polled every `poll_seconds` (default 10, change it with `p`). When a build finishes, or pauses on an input step, you get a desktop notification:

- **macOS:** Notification Center, through `osascript`. If nothing shows up, allow notifications for *Script Editor* under System Settings → Notifications.
- **Linux:** `notify-send`, which works with GNOME, KDE, dunst, mako and others. Install it with `apt install libnotify-bin`, `dnf install libnotify` or `pacman -S libnotify`.

If notifications can't be shown, the status bar says so once. Events still appear in the app.

## Search

`/` searches the whole controller, not only the current folder. Every job and folder is loaded in one request at startup (`r` reloads it). Matching is fuzzy on the full path, so `gdep` finds `gotham/deploy`. Matches in the current folder come first, then everything else with its folder shown. `enter` on a match from another folder takes you there, so `h` goes back to its parent folder.

## Macros

A macro is a named list of steps, built in the TUI: press `m`, then `n`.

| Step | What it does |
|---|---|
| build | Triggers a job with saved parameter values, filled in through the job's own parameter form. Password parameters are never saved; Jenkins uses their default. |
| abort | Aborts the job's running build. |
| wait | Waits for the build to finish and fails unless it succeeded. The timeout is per step, 30 minutes by default; a timeout leaves the build running. |
| input | Proceeds with the input step's default values, or aborts it. After a build step on the same job, it waits for the pipeline to reach the input (same timeout), and fails if the build ends without asking. Otherwise it checks once: if nothing is running or nothing is waiting, the step is skipped and the macro goes on. |

Abort, wait and input act on the build an earlier step of the same macro started. If there isn't one, they use the job's newest running build. The macro stops at the first step that fails.

In the TUI, `enter` runs the selected macro after a y/n. It runs in the background: the header shows `▶ name 2/4`, and `m` shows each step's progress. `c` cancels before the next step; builds already started keep running.

From a shell:

```sh
jenklod-batman run --list
jenklod-batman run ship                # shows the steps and asks y/n
jenklod-batman run ship ENV=prod -y    # override a parameter, no prompt
```

`KEY=VALUE` replaces that parameter in every build step that has one. A key no step uses is an error. Without a terminal (cron, CI), it doesn't prompt. The exit code is non-zero if a step fails.

## Config

Pins and macros are stored per Jenkins URL, so pointing `--setup` at another controller doesn't mix them. The `pinned` list from older versions moves to its controller on the next start.

```toml
url = "https://jenkins.example.com"
user = "bruce.wayne"
poll_seconds = 10

[servers."https://jenkins.example.com"]
pinned = ["gotham/deploy"]

[[servers."https://jenkins.example.com".macros]]
name = "ship"

[[servers."https://jenkins.example.com".macros.steps]]
kind = "build"
job = "gotham/deploy"
with_params = true
params = { ENV = "prod" }

[[servers."https://jenkins.example.com".macros.steps]]
kind = "wait"
job = "gotham/deploy"
timeout_minutes = 45
```

## Development

```sh
make test   # Jenkins client and a UI flow against a fake Jenkins
JB_SNAPSHOTS=/tmp/snap go test ./internal/ui/   # dump rendered screens
```

## License

[MIT](LICENSE)
