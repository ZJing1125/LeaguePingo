#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
for task_tool in go node npm wails3; do
  command -v "$task_tool" >/dev/null || { echo "Missing tool: $task_tool" >&2; exit 1; }
done
task_runtime=$(node -p "JSON.parse(require('fs').readFileSync('frontend/package.json','utf8')).dependencies['@wailsio/runtime']")
task_backend=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
if [[ "v$task_runtime" != "$task_backend" ]]; then
  echo 'Wails frontend runtime and backend versions differ' >&2
  exit 1
fi
wails3 generate bindings -clean=true -ts -i
npm --prefix frontend run build
go test ./...
go vet ./...
if [[ -z "${BROWSER_EXECUTABLE:-}" && -f '/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe' ]]; then
  export BROWSER_EXECUTABLE='C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe'
fi
npm --prefix frontend run test:smoke
git diff --check
