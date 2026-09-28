#!/usr/bin/env bash
# Records docs/tapes/<name>.tape against a fresh demo Jenkins:
#   docs/tapes/record.sh tour search macros input
set -euo pipefail
cd "$(dirname "$0")/../.."
go build -o bin/jenklod-batman .
go build -o bin/demo-jenkins ./cmd/demo-jenkins
work=$(mktemp -d)
trap 'kill $(jobs -p) 2>/dev/null || true; rm -rf "$work"' EXIT
for name in "$@"; do
	bin/demo-jenkins -addr 127.0.0.1:8765 2>/dev/null &
	pid=$!
	sleep 0.5
	# The default config location, so commands on screen need no flags.
	rm -rf "$work/jenklod-batman" && mkdir -p "$work/jenklod-batman"
	cp docs/tapes/demo.toml "$work/jenklod-batman/config.toml"
	XDG_CONFIG_HOME="$work" PATH="$PWD/bin:$PATH" vhs "docs/tapes/$name.tape"
	kill "$pid"
	wait "$pid" 2>/dev/null || true
done
