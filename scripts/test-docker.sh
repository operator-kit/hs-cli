#!/usr/bin/env sh
set -eu

repo_root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

exec docker run --rm \
  -v "${repo_root}:/workspace" \
  -v "hs-cli-go-mod-cache:/go/pkg/mod" \
  -v "hs-cli-go-build-cache:/tmp/go-cache" \
  -w /workspace \
  -e HOME=/tmp/hs-test-home \
  -e USERPROFILE=/tmp/hs-test-home \
  -e XDG_CONFIG_HOME=/tmp/hs-test-home/.config \
  -e APPDATA=/tmp/hs-test-home/AppData \
  -e GOCACHE=/tmp/go-cache \
  -e GOMODCACHE=/go/pkg/mod \
  -e GOTELEMETRY=off \
  golang:1.25 \
  go test "$@"
