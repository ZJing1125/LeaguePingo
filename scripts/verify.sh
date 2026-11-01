#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
task_scope=${1:-windows}
case "$task_scope" in
  windows) export LEAGUEPINGO_VERIFY_SCOPE=windows ;;
  portable) export LEAGUEPINGO_VERIFY_SCOPE=portable ;;
  *) echo 'Usage: bash scripts/verify.sh [windows|portable]' >&2; exit 1 ;;
esac
for task_tool in go node npm; do
  command -v "$task_tool" >/dev/null || { echo "Missing tool: $task_tool" >&2; exit 1; }
done
if [[ "$task_scope" == windows ]]; then
  node scripts/workflow.mjs environment
  command -v wails3 >/dev/null || { echo 'Missing tool: wails3' >&2; exit 1; }
fi
echo "Verification scope: $task_scope (not real-game acceptance)"
node scripts/workflow.mjs check
node scripts/version.mjs check
node --test scripts/artifacts.test.mjs scripts/workflow.test.mjs scripts/environment.test.mjs scripts/version.test.mjs scripts/champion-portraits.test.mjs scripts/ui-assets.test.mjs scripts/app-icons.test.mjs
if [[ "$task_scope" == portable ]]; then
  npm --prefix frontend run build
  go test ./internal/...
  go vet ./internal/...
  git diff --check
  echo 'PASS: portable subset only. NOT RUN: binding generation, Wails task execution, desktop packages, Windows build/process integration, browser interaction, real client/game acceptance.'
  exit 0
fi
task_runtime=$(node -p "JSON.parse(require('fs').readFileSync('frontend/package.json','utf8')).dependencies['@wailsio/runtime']")
task_backend=$(go list -m -f '{{.Version}}' github.com/wailsapp/wails/v3)
if [[ "v$task_runtime" != "$task_backend" ]]; then
  echo 'Wails frontend runtime and backend versions differ' >&2
  exit 1
fi
wails3 generate bindings -clean=true -ts -i
npm --prefix frontend run build
wails3 generate syso -arch amd64 -icon build/windows/icon.ico -manifest build/windows/wails.exe.manifest -info build/windows/info.json -out wails_windows_amd64.syso
go test ./...
go vet ./...
if [[ -z "${BROWSER_EXECUTABLE:-}" ]]; then
  export BROWSER_EXECUTABLE="$(node scripts/environment.mjs browser)"
fi
npm --prefix frontend run test:smoke
git diff --check
echo 'PASS: Windows automated verification. NOT RUN: real client/game acceptance and FPS/network measurements.'
