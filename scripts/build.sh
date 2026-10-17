#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment
node scripts/workflow.mjs check
node scripts/artifacts.mjs audit --allow-empty
task_commit=$(git rev-parse --short HEAD)
if [[ -n "$(git status --porcelain)" ]]; then task_commit+=-dirty; fi
task_time=$(date -u +%Y-%m-%dT%H:%M:%SZ)
output=${1:-bin/leaguepingo-dev-$(date -u +%Y%m%d-%H%M%S)-$task_commit.exe}
# The manager validates the path and refuses overwrite before verification/build.
# All normal builds share a lock; stale locks require inspection, not force removal.
task_lock="$task_root/bin/.build-lock"
node scripts/artifacts.mjs check-output --file "$output"
mkdir -p "$task_root/bin"
mkdir "$task_lock" || { echo 'Another build/uncleared build lock exists' >&2; exit 1; }
task_prepared=0
task_registered=0
finish_build() {
  task_status=$?
  if [[ "$task_prepared" == 1 && "$task_registered" == 0 ]]; then
    node scripts/artifacts.mjs discard --file "$output" --apply || echo 'Failed-build leftovers preserved; inspect before cleanup' >&2
  fi
  rmdir "$task_lock"
  return "$task_status"
}
trap finish_build EXIT
node scripts/artifacts.mjs prepare --file "$output" --apply
task_prepared=1
task_id=$(basename "$output" .exe)
export UI_SCREENSHOT_PREFIX="$task_root/artifacts/$task_id/ui"
export WORKSPACE_SCREENSHOT="$task_root/artifacts/$task_id/workspace.png"
export OVERLAY_SCREENSHOT="$task_root/artifacts/$task_id/overlay.png"
bash scripts/verify.sh
go build -tags production -trimpath -buildvcs=false -ldflags="-H windowsgui -X main.Commit=$task_commit -X main.BuildTime=$task_time" -o "$output" .
sha256sum "$output"
node scripts/artifacts.mjs register --file "$output" --commit "$task_commit" --apply
task_registered=1
node scripts/artifacts.mjs clean --apply
node scripts/artifacts.mjs audit
