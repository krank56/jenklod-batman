# First run

The first launch opens a setup form. It asks for:

- **Jenkins URL**, for example `https://jenkins.example.com`.
- **User**: your Jenkins user ID.
- **API token**: create one in Jenkins under *your name → Security → API Token → Add new token*.

Before saving anything, the app checks the credentials against Jenkins, so a typo never ends up in the config or the keyring.

![The splash screen: a bat signal over the Gotham skyline](/media/splash.png)

## Where things are stored

| What | Where |
|---|---|
| URL, user, preferences, pins, macros | `~/.config/jenklod-batman/config.toml` (mode `0600`; `$XDG_CONFIG_HOME` is honoured) |
| API token | The system keyring, service `jenklod-batman`, account `<user>@<url>`: Keychain on macOS, the Secret Service (gnome-keyring, KWallet) on Linux |

The token is never written to disk. `JENKINS_TOKEN` in the environment overrides the keyring, which helps on headless Linux machines with no keyring daemon.

## Useful flags

```sh
jenklod-batman --setup          # enter URL / user / token again
jenklod-batman --forget-token   # remove the token from the keyring
jenklod-batman --no-anim        # skip the splash and keep the cat still
```

All of them are listed in [Command line](/reference/cli).

## Trying it without a Jenkins

The repository includes a simulated Jenkins, with folders, running pipelines and an input step. The docs recordings use it.

```sh
git clone https://github.com/krank56/jenklod-batman && cd jenklod-batman
go run ./cmd/demo-jenkins &
cp docs/tapes/demo.toml /tmp/demo.toml   # the app saves pins and macros to it
JENKINS_TOKEN=demo go run . --config /tmp/demo.toml
```

Next: [Browsing & builds](./browsing).
