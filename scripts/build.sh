#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
output=${1:-bin/leaguepingo-dev.exe}
task_commit=$(git rev-parse --short HEAD)
if [[ -n "$(git status --porcelain)" ]]; then task_commit+=-dirty; fi
task_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
bash scripts/verify.sh
go build -tags production -trimpath -buildvcs=false -ldflags="-H windowsgui -X main.Commit=$task_commit -X main.BuildTime=$task_time" -o "$output" .
sha256sum "$output"
