#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source scripts/env.sh
if [[ "$(go env GOHOSTOS)" != windows || "$(go env GOHOSTARCH)" != amd64 || "$(go env GOOS)" != windows || "$(go env GOARCH)" != amd64 ]]; then
 echo 'NOT VERIFIED: run on Windows amd64 with the logged-in client during loading/gameplay.' >&2
 exit 1
fi
LEAGUEPINGO_GROUP_AUDIT=1 go test ./internal/source -run '^TestLiveCurrentGrouping$' -count=1 -v
