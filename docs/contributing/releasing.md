# Releasing

Every user-facing change adds a line under `## Unreleased` in [`CHANGELOG.md`](https://github.com/krank56/jenklod-batman/blob/main/CHANGELOG.md), in the PR that makes it.

To release, rename that heading to the version and date, merge it, then tag the merge:

```md
## v0.4.0 (2026-10-14)
```

```sh
git tag -a v0.4.0 -m v0.4.0 && git push origin v0.4.0
```

The `release` workflow then:

1. runs `make vet test`,
2. takes the tag's section of `CHANGELOG.md` as the release notes (`go run . changelog v0.4.0`), and fails if there isn't one,
3. builds the archives with `make dist` (macOS and Linux, amd64 and arm64). Linux links ALSA through cgo for the music, so each Linux architecture builds on its own runner (`ubuntu-latest`, `ubuntu-24.04-arm`); macOS cross-compiles without cgo,
4. gathers them, writes `checksums.txt`, and publishes them on the GitHub release.

Add a new `## Unreleased` heading on top for what comes next. The app embeds `CHANGELOG.md`, so a release shows its own section as "what's new" on first launch, and `jk update` prints the notes of every version it skips.

## Homebrew

The formula lives in [krank56/homebrew-tap](https://github.com/krank56/homebrew-tap). Its `bump` workflow checks for a new release every hour, and can also be run from the Actions tab. When there is one, it renders the formula with the new checksums, installs and tests it with brew on macOS, and commits it. No token crosses repositories.

## Building archives locally

```sh
make dist VERSION=v0.3.0
ls dist/
```

That builds both macOS archives, plus the host's own Linux one on Linux. Linux needs `libasound2-dev`. `PLATFORMS="darwin/arm64"` narrows it.
