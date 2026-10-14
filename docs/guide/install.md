# Install

jenklod-batman is a single binary for macOS and Linux, on Intel and ARM.

::: code-group

```sh [Homebrew]
brew install krank56/tap/jenklod-batman
```

```sh [Go]
go install github.com/krank56/jenklod-batman@latest
```

```sh [From a clone]
git clone https://github.com/krank56/jenklod-batman
cd jenklod-batman
make install        # into ~/go/bin (override with INSTALL_DIR=...)
```

:::

Each [release](https://github.com/krank56/jenklod-batman/releases) also has prebuilt archives (`jenklod-batman_<version>_<os>_<arch>.tar.gz`) and a `checksums.txt`.

## Linux: ALSA

On Linux, the [theme music](/guide/music) plays through ALSA, so the binary needs its library, `libasound.so.2`, even with the music off. Desktop distributions ship it; a minimal server or container may not:

```sh
sudo apt install libasound2   # Debian, Ubuntu (libasound2t64 on Ubuntu 24.04+)
sudo dnf install alsa-lib     # Fedora
```

`go install` and `make install` on Linux also build against it, with cgo, so they need its headers (`libasound2-dev` or `alsa-lib-devel`) and a C compiler. macOS needs nothing extra.

## The short name

Homebrew, the release archives and `make install` also install `jk`, a link to `jenklod-batman` named after the keys that move the cursor. These docs use it, and the app's usage lines and hints show whichever name you ran.

`go install` only installs `jenklod-batman`, and `update` doesn't add `jk` to an archive install from before it existed. A link in the binary's directory (`ln -s jenklod-batman jk`) or an alias does the same: `alias jk jenklod-batman` (fish) or `alias jk=jenklod-batman` (bash, zsh).

Check that it's installed:

```sh
jk --version
```

::: tip A different version than you expected?
A copy installed with a plain `go install` from inside a clone reports `dev` and can shadow the Homebrew one. `type -a jenklod-batman` lists every copy on your `PATH`, in the order your shell finds them.
:::

## Upgrading

```sh
jk update
```

It installs the latest release the way this copy was installed: with `brew upgrade` for Homebrew, `go install` for Go, or by downloading the release archive and checking it against `checksums.txt`. Then it prints what changed. `jk update --check` only says whether there's a newer release.

The TUI checks once a day and shows `↑ vX.Y.Z` in the header when there is one, an hour after it's released, which is when the Homebrew formula has caught up. `check_updates = false` in the [config](/reference/config) turns that off. The first launch after an upgrade shows what's new; `jk changelog` prints the whole [changelog](/changelog).

Next: [First run](./first-run).
