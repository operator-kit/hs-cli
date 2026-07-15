#!/usr/bin/env sh
set -eu

script_dir="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

exec sh "${script_dir}/test-docker.sh" -race "$@"
