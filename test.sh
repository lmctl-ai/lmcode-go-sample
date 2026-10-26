#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")" && pwd)"
cd "$root"

python3 -B scripts/sync-suite.py --check
go mod verify
mkdir -p build
binary=build/lmcode-go-sample
if [[ "$(go env GOOS)" == windows ]]; then binary+='.exe'; fi
CGO_ENABLED=0 go build -trimpath -ldflags='-buildid=' -o "$binary" .
binary="$root/$binary"
sdk_dir="$(go list -m -f '{{.Dir}}' github.com/lmctl-ai/lmcode-go)"
python3 -B sample-suite/audit.py --std "$sdk_dir/std/lmcode-std.schema.json"
bash sample-suite/run-suite.sh "$binary"
python3 -B scripts/check-bytecode.py
python3 -B sample-suite/test_runner.py
