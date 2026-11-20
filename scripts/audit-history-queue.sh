#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
if [[ "$(go env GOHOSTOS)" != windows || "$(go env GOHOSTARCH)" != amd64 ]]; then
  echo 'Queue-history audit requires Windows amd64 and the connected national-server client.' >&2
  exit 1
fi
echo 'Read-only self-account Mayhem queue audit, idle/lobby/postgame, at most six pages in 45 seconds.'
LEAGUEPINGO_LIVE_QUEUE_AUDIT=1 go test ./internal/source -run '^TestLiveQueueHistoryAudit$' -count=1 -v
