#!/usr/bin/env bash

set -euo pipefail

GO_BIN="$(command -v go || true)"
if [[ -z "${GO_BIN}" && -x "/opt/homebrew/bin/go" ]]; then
  GO_BIN="/opt/homebrew/bin/go"
fi

if [[ -z "${GO_BIN}" ]]; then
  echo "go binary not found. Install Go or add it to PATH."
  exit 1
fi

"${GO_BIN}" run ./scripts/write_bench_with_obs.go "$@"
