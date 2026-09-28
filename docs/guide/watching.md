# Watching jobs

Press <kbd>w</kbd> on a job to watch it (<kbd>w</kbd> again stops). Watched jobs are listed at the top of the root screen with their last build: number, result, age, duration or a live progress bar, and what triggered it.

![The Watched section: a deploy waiting for input, a backup running, a failed audit](/media/watched.png)

On a watched row:

| Key | Action |
|---|---|
| <kbd>enter</kbd> | Open the job as if you had browsed to it, so <kbd>h</kbd> goes back to its folder |
| <kbd>b</kbd> | Build |
| <kbd>L</kbd> | Open the last build's log |
| <kbd>x</kbd> | Abort the running build (asks first) |
| <kbd>w</kbd> | Stop watching |
| <kbd>o</kbd> | Open in the browser |
| <kbd>J</kbd>/<kbd>K</kbd> | Move it down or up; the order is saved |

<Demo name="search-watch" caption="Find a job in a nested folder, watch it, and move it to the top." />

If a watched job can't be polled (deleted, renamed, or no longer visible to you), its row shows `✗` and the reason until you stop watching it.

Watched jobs are saved per Jenkins URL, so switching controllers with `--setup` doesn't mix them.

## Polling

While the TUI is open, watched jobs are polled every 10 seconds. <kbd>p</kbd> cycles through 5, 10, 20, 30 and 60 seconds. The header shows the current interval (`⟳10s`), and it's saved as `poll_seconds`.

## Notifications

You get a desktop notification when a watched job's build finishes, and once when it starts waiting for input.

- **macOS:** Notification Center, through `osascript`. If nothing shows up, allow notifications for *Script Editor* under System Settings → Notifications.
- **Linux:** `notify-send`, which works with GNOME, KDE, dunst, mako and others. Install it with `apt install libnotify-bin`, `dnf install libnotify` or `pacman -S libnotify`.

If notifications can't be shown, the status bar says so once. Events still appear in the app.
