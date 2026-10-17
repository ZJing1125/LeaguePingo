#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment
case "${1:-}" in
  build)
    node scripts/workflow.mjs check
    node scripts/workflow.mjs dev-check
    mkdir -p bin artifacts/hot-reload
    # Share the delivery build lock: both modes generate bindings/frontend files.
    mkdir bin/.build-lock || { echo 'Another build/uncleared build lock exists' >&2; exit 1; }
    trap 'rmdir bin/.build-lock' EXIT
    wails3 generate bindings -clean=true -ts -i
    npm --prefix frontend run build:dev
    # Fixed scratch output: never added to the delivery manifest or promoted.
    go build -buildvcs=false -gcflags=all="-l" -o artifacts/hot-reload/leaguepingo-hot-reload.exe .
    ;;
  run)
    node scripts/workflow.mjs dev-check
    test -f artifacts/hot-reload/leaguepingo-hot-reload.exe
    exec "$task_root/artifacts/hot-reload/leaguepingo-hot-reload.exe"
    ;;
  *) echo 'Usage: bash scripts/dev.sh build|run (not a delivery)' >&2; exit 1 ;;
esac
