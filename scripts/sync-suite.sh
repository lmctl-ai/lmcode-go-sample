#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "$0")/.." && pwd)"
source="${1:-$root/../lmcode-cpp}"
exec python3 "$root/scripts/sync-suite.py" "$source"
