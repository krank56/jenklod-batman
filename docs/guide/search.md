# Search

<kbd>/</kbd> searches the whole controller, not only the folder you're in.

![Searching "hook" finds a job three folders deep](/media/search.png)

- **Fuzzy:** matching is on the full path, like fzf. `gdep` finds `gotham/deploy`, and `hook` finds `wayne-enterprises/applied-sciences/grappling-hook-ci`. Consecutive letters and word starts rank higher.
- **Nearest first:** matches in the current folder come first, then the rest with their folder shown.
- **Jumping:** <kbd>enter</kbd> keeps the results so you can move through them with the arrow keys. <kbd>enter</kbd> again opens the match. A match from another folder takes you into that folder, so <kbd>h</kbd> goes back to its parent.
- **Actions:** <kbd>b</kbd>, <kbd>L</kbd>, <kbd>x</kbd>, <kbd>w</kbd> and <kbd>o</kbd> work on results too.
- <kbd>esc</kbd> clears the search.

## How it works

Every job and folder is loaded in one request when the app starts, using a nested `tree=` query 8 levels deep, and kept for the session. <kbd>r</kbd> on the jobs screen reloads it. If loading fails, the search box says so and still matches the current folder.
