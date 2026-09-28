# Browsing & builds

The root screen lists your [watched jobs](./watching) first, then the top-level folders and jobs. `▸` marks a folder. A coloured dot shows a job's last result, and a spinner means it's building.

| Key | Action |
|---|---|
| <kbd>enter</kbd> <kbd>l</kbd> <kbd>→</kbd> | Open a folder, then a job's builds, then a build's log |
| <kbd>h</kbd> <kbd>←</kbd> <kbd>esc</kbd> | Back |
| <kbd>j</kbd>/<kbd>k</kbd>, <kbd>g</kbd>/<kbd>G</kbd>, <kbd>ctrl+d</kbd>/<kbd>ctrl+u</kbd> | Move |
| <kbd>r</kbd> | Refresh |
| <kbd>o</kbd> | Open in the browser |

## Builds

A job's screen lists its last 30 builds with result, start time, duration and cause. A running build shows a progress bar against Jenkins' estimate, and the list refreshes while anything is running.

![The builds of a job: one running, the rest finished](/media/builds.png)

- <kbd>b</kbd> starts a build. It always asks y/n. For a job with parameters, the form opens first: <kbd>tab</kbd> moves between fields, <kbd>←</kbd>/<kbd>→</kbd> or <kbd>space</kbd> change choices and checkboxes, and <kbd>enter</kbd> builds.
- <kbd>x</kbd> aborts the selected build if it's running, after a y/n.
- <kbd>enter</kbd> opens the build's console log.

From the jobs list, you don't have to open a job first: <kbd>b</kbd> builds the selected job, <kbd>L</kbd> opens its last build's log, and <kbd>x</kbd> aborts its last build.

## Logs

The log follows new output while the build runs (<kbd>f</kbd> toggles following). <kbd>g</kbd>/<kbd>G</kbd> jump to the top or bottom. Jenkins' hidden console annotations are stripped.

![A build log following a pipeline that is waiting for input](/media/log.png)
