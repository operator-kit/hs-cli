#!/usr/bin/env sh
set -eu

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to run the isolated test suite." >&2
  exit 1
fi

if ! docker info >/dev/null 2>&1; then
  echo "The Docker engine is unavailable. Start Docker Desktop or Docker Engine and retry." >&2
  exit 1
fi

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
  golang:1.25.9-bookworm@sha256:298734aec230b5f3e8cee450ce6d7eccc39f1797ba548ee90d57e9803030c6c3 \
  go test "$@"
