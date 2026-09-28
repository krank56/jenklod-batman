# Releasing

```sh
git tag -a v0.3.0 -m v0.3.0 && git push origin v0.3.0
```

The `release` workflow then:

1. runs `make vet test`,
2. builds the archives with `make dist` (macOS and Linux, amd64 and arm64, plus `checksums.txt`),
3. publishes them on the GitHub release.

To write the release notes yourself, draft the release before pushing the tag; the workflow adds the files to it. Otherwise the notes are generated.

## Homebrew

The formula lives in [krank56/homebrew-tap](https://github.com/krank56/homebrew-tap). Its `bump` workflow checks for a new release every hour, and can also be run from the Actions tab. When there is one, it renders the formula with the new checksums, installs and tests it with brew on macOS, and commits it. No token crosses repositories.

## Building archives locally

```sh
make dist VERSION=v0.3.0
ls dist/
```
