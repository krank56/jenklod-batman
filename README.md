# jenklod-batman 🦇

A terminal UI for managing Jenkins jobs. It has a bat-signal splash screen and a cat that lives in the status bar.

## Install

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
| `/` | filter jobs in the current folder |
| `b` | build (a form opens for parameterized jobs; always asks y/n) |
| `x` | abort a running build (asks y/n) |
| `i` | answer a paused pipeline `input` step, e.g. "Deploy to prod?" |
| `w` | watch or unwatch a job |
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

Watched jobs (`w`, shown with ◆) are saved in the config under `pinned`. While the TUI is open they are polled every `poll_seconds` (default 20). When a build finishes, or pauses on an input step, you get a desktop notification:

- **macOS:** Notification Center, through `osascript`. If nothing shows up, allow notifications for *Script Editor* under System Settings → Notifications.
- **Linux:** `notify-send`, which works with GNOME, KDE, dunst, mako and others. Install it with `apt install libnotify-bin`, `dnf install libnotify` or `pacman -S libnotify`.

If notifications can't be shown, the status bar says so once. Events still appear in the app.

```toml
url = "https://jenkins.example.com"
user = "bruce.wayne"
poll_seconds = 20
pinned = ["gotham/deploy"]
```

## Development

```sh
make test   # Jenkins client and a UI flow against a fake Jenkins
JB_SNAPSHOTS=/tmp/snap go test ./internal/ui/   # dump rendered screens
```
