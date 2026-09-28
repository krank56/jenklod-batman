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

Check that it's installed:

```sh
jenklod-batman --version
```

::: tip A different version than you expected?
A copy installed with a plain `go install` from inside a clone reports `dev` and can shadow the Homebrew one. `type -a jenklod-batman` lists every copy on your `PATH`, in the order your shell finds them.
:::

## Upgrading

```sh
brew upgrade jenklod-batman
```

The Homebrew formula follows new releases automatically, usually within an hour of the release.

Next: [First run](./first-run).
