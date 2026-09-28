# config.toml

The app writes `~/.config/jenklod-batman/config.toml` itself: the setup form fills it, and pins, poll interval and macros are saved as you change them. You can also edit it by hand while the app is closed. It never contains your token.

```toml
url = "https://jenkins.example.com"
user = "bruce.wayne"
poll_seconds = 10

[servers."https://jenkins.example.com"]
pinned = ["gotham/deploy", "arkham/cell-audit"]

[[servers."https://jenkins.example.com".macros]]
name = "ship"

[[servers."https://jenkins.example.com".macros.steps]]
kind = "build"
job = "gotham/deploy"
with_params = true
params = { ENV = "prod", DRY_RUN = "false" }

[[servers."https://jenkins.example.com".macros.steps]]
kind = "input"
job = "gotham/deploy"

[[servers."https://jenkins.example.com".macros.steps]]
kind = "wait"
job = "gotham/deploy"
timeout_minutes = 45
```

## Top level

| Key | Type | Default | Description |
|---|---|---|---|
| `url` | string | | The Jenkins controller. |
| `user` | string | | Your Jenkins user ID. |
| `poll_seconds` | integer | `10` | How often watched jobs are polled. <kbd>p</kbd> cycles 5, 10, 20, 30, 60. |

## `[servers."<url>"]`

Everything that belongs to one controller, keyed by its URL without the trailing slash. Switching `url` with `--setup` shows that controller's pins and macros, and keeps the others for later.

| Key | Type | Description |
|---|---|---|
| `pinned` | list of strings | Watched jobs, as full names (`folder/sub/job`), in display order. |
| `macros` | array of tables | See below. |

::: info Older configs
Before v0.2.0, `pinned` was a top-level list. On the next start it moves under the configured `url`.
:::

## Macros

Each `[[servers."<url>".macros]]` has a `name` and an array of `steps`.

| Key | Type | Applies to | Description |
|---|---|---|---|
| `kind` | string | all | `build`, `abort`, `wait` or `input`. |
| `job` | string | all | The job's full name. |
| `with_params` | boolean | build | Use `buildWithParameters` with `params`; otherwise start the job plainly. |
| `params` | table | build | Parameter values. Password parameters are never saved. |
| `timeout_minutes` | integer | wait, input | Defaults to 30. |
| `abort` | boolean | input | Answer *Abort* instead of proceeding. |

How the steps run is described in [Macros](/guide/macros#steps).
