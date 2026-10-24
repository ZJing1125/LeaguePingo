#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
task_section=${1:-all}
case "$task_section" in
  rank|history|match|all) ;;
  *) echo 'Usage: bash scripts/audit-player-data.sh [rank|history|match|all]' >&2; exit 1 ;;
esac
command -v go >/dev/null || { echo 'Missing tool: go' >&2; exit 1; }
# This is a source test, not a desktop compiler/package/delivery entry.
if [[ "$(go env GOHOSTOS)" != windows || "$(go env GOHOSTARCH)" != amd64 ]]; then
  echo 'NOT VERIFIED: run on Windows amd64 with the logged-in Tencent client.' >&2
  exit 1
fi
if [[ "$(go env GOOS)" != windows || "$(go env GOARCH)" != amd64 ]]; then
  echo 'Cross-target overrides are not supported for live auditing.' >&2
  exit 1
fi
echo 'Read-only audit: one player, 45 seconds maximum, up to 3 history pages, one game.'
echo 'Use idle/lobby/postgame. Report states and coverage require manual client/WeGame/UI comparison.'
LEAGUEPINGO_LIVE_DATA_AUDIT=1 LEAGUEPINGO_AUDIT_SECTION="$task_section" \
  go test ./internal/source -run '^TestLivePlayerDataAudit$' -count=1 -v
