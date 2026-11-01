#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment
node scripts/workflow.mjs check
node scripts/version.mjs check

task_binary=""
task_output=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --binary) task_binary=${2:-}; shift 2 ;;
    --output) task_output=${2:-}; shift 2 ;;
    *) echo 'Usage: bash scripts/package-windows.sh [--binary bin/leaguepingo*.exe] [--output artifacts/.../*.exe]' >&2; exit 1 ;;
  esac
done
task_version=$(node scripts/version.mjs value)
task_numeric=$(node scripts/version.mjs numeric)
task_commit=$(git rev-parse --short HEAD)
task_makensis=$(node scripts/environment.mjs nsis)
if [[ -z "$task_binary" ]]; then
  task_binary="bin/leaguepingo-$task_version-$task_commit.exe"
  bash scripts/build.sh "$task_binary"
else
  [[ "$task_binary" =~ ^bin/leaguepingo[A-Za-z0-9_.-]*\.exe$ && -f "$task_binary" ]] || { echo 'Existing package input must be a managed bin/leaguepingo*.exe' >&2; exit 1; }
  task_current=$(node scripts/artifacts.mjs current)
  [[ "$task_current" == "$task_binary" ]] || { echo 'Only the current audited delivery binary can be packaged' >&2; exit 1; }
fi
if [[ -z "$task_output" ]]; then
  task_output="artifacts/installers/leaguepingo-$task_version-$task_commit-amd64-installer.exe"
fi
[[ "$task_output" =~ ^artifacts/[A-Za-z0-9_./-]+\.exe$ && "$task_output" != *..* ]] || { echo 'Installer output must be a safe artifacts/*.exe path' >&2; exit 1; }
[[ ! -e "$task_output" ]] || { echo 'Installer output already exists; overwrite refused' >&2; exit 1; }
mkdir -p "$(dirname "$task_output")"
task_binary_windows=$(cygpath -w "$(realpath "$task_binary")")
task_output_windows=$(cygpath -w "$task_root/$task_output")
(
  cd build/windows/nsis
  "$task_makensis" -V3 \
    "-DARG_WAILS_AMD64_BINARY=$task_binary_windows" \
    "-DARG_INSTALLER_OUTPUT=$task_output_windows" \
    "-DINFO_PRODUCTVERSION=$task_version" \
    "-DINFO_NUMERICVERSION=$task_numeric" \
    project.nsi
)
[[ -f "$task_output" ]] || { echo 'NSIS did not create the requested installer' >&2; exit 1; }
if [[ -n "${LEAGUEPINGO_SIGN_CERT_SHA1:-}" ]]; then
  bash scripts/sign-windows.sh "$task_output"
fi
sha256sum "$task_output"
printf 'Installer: %s\n' "$task_output"
