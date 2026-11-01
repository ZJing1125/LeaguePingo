#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment

task_file=${1:-}
[[ -n "$task_file" ]] || { echo 'Usage: bash scripts/sign-windows.sh <unregistered-exe>' >&2; exit 1; }
[[ "$task_file" != *..* && "$task_file" == *.exe ]] || { echo 'Signing target must be a project-relative .exe path' >&2; exit 1; }
task_absolute=$(realpath "$task_file")
[[ "$task_absolute" == "$task_root"/* && -f "$task_absolute" ]] || { echo 'Signing target is missing or outside the project' >&2; exit 1; }
task_relative=${task_absolute#"$task_root"/}

# Registered delivery hashes are immutable. build.sh invokes this script before
# registration; installers live outside bin and can be signed after packaging.
if [[ -f bin/artifacts.json ]] && node - "$task_relative" <<'NODE'
const fs = require('fs');
const target = process.argv[2].replaceAll('\\', '/').toLowerCase();
const state = JSON.parse(fs.readFileSync('bin/artifacts.json', 'utf8'));
process.exit(state.builds.some(item => item.file.toLowerCase() === target) ? 0 : 1);
NODE
then
  echo 'Refusing to change a registered delivery binary; rebuild with signing enabled' >&2
  exit 1
fi

task_cert=${LEAGUEPINGO_SIGN_CERT_SHA1:-}
[[ "$task_cert" =~ ^[A-Fa-f0-9]{40}$ ]] || { echo 'LEAGUEPINGO_SIGN_CERT_SHA1 must name a 40-character certificate thumbprint in the Windows certificate store' >&2; exit 1; }
task_signtool=${LEAGUEPINGO_SIGNTOOL:-}
if [[ -z "$task_signtool" ]]; then
  task_signtool=$(command -v signtool.exe || command -v signtool || true)
fi
[[ -n "$task_signtool" ]] || { echo 'signtool was not found; configure LEAGUEPINGO_SIGNTOOL' >&2; exit 1; }
task_windows_file=$(cygpath -w "$task_absolute")
"$task_signtool" sign /sha1 "$task_cert" /fd SHA256 /tr https://timestamp.digicert.com /td SHA256 "$task_windows_file"
"$task_signtool" verify /pa /all "$task_windows_file"
