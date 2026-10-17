#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment
node scripts/artifacts.mjs audit
task_binary=$(node scripts/artifacts.mjs current)
exec "$task_root/$task_binary"
