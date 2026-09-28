# Command line

## `jenklod-batman`

Starts the TUI.

| Flag | Default | Description |
|---|---|---|
| `--config FILE` | `~/.config/jenklod-batman/config.toml` | Config file. `$XDG_CONFIG_HOME` replaces `~/.config`. |
| `--setup` | | Open the setup form again (URL, user, token). |
| `--forget-token` | | Remove the stored token from the keyring and exit. |
| `--no-anim` | | Skip the splash and keep the cat still. |
| `--version` | | Print the version and exit. |

## `jenklod-batman run`

Runs a [macro](/guide/macros) without the TUI.

```text
jenklod-batman run <macro> [KEY=VALUE…] [-y] [--config FILE]
jenklod-batman run --list
```

| Argument | Description |
|---|---|
| `<macro>` | The macro's name, as listed by `--list`. |
| `KEY=VALUE` | Override that parameter in every build step that has it. Repeat for several. A key that no build step uses is an error. |
| `-y`, `--yes` | Don't ask for confirmation. |
| `-l`, `--list` | List the macros of the configured Jenkins, with their steps. |
| `--config FILE` | Config file, as for the TUI. |
| `-h`, `--help` | Show the usage. |

**Behaviour:**

- **Before running,** it prints the steps and any overrides. On a terminal it asks `Run it? [y/N]`. With `-y`, or without a terminal, it doesn't ask.
- **Progress** goes to standard output as `[step/total] message`.
- **Exit codes:** `0` when every step succeeded; `1` on any error, including a failed step, an unknown macro or an unused override.
- **Interrupting:** <kbd>ctrl+c</kbd> or `SIGTERM` stops the run. Builds already started keep running.

## Environment

| Variable | Description |
|---|---|
| `JENKINS_TOKEN` | API token to use instead of the keyring's. |
| `XDG_CONFIG_HOME` | Base directory for the config (default `~/.config`). |
