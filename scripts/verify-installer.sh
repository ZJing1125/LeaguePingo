#!/usr/bin/env bash
set -euo pipefail
task_root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
cd "$task_root"
source "$task_root/scripts/env.sh"
node scripts/workflow.mjs environment
task_installer=${1:-}
[[ -f "$task_installer" ]] || { echo 'Usage: bash scripts/verify-installer.sh <installer.exe>' >&2; exit 1; }
task_install_root="${RUNNER_TEMP:-$task_root/artifacts}/LeaguePingoInstallerTest-$RANDOM"
task_install_windows=$(cygpath -w "$task_install_root")
task_installer_windows=$(cygpath -w "$(realpath "$task_installer")")
"$task_installer_windows" /S "/D=$task_install_windows"
[[ -f "$task_install_root/LeaguePingo.exe" && -f "$task_install_root/uninstall.exe" ]] || { echo 'Silent NSIS installation did not create expected files' >&2; exit 1; }
"$(cygpath -w "$task_install_root/uninstall.exe")" /S
for _ in {1..40}; do
  [[ ! -e "$task_install_root/LeaguePingo.exe" ]] && break
  sleep 0.25
done
[[ ! -e "$task_install_root/LeaguePingo.exe" ]] || { echo 'Silent NSIS uninstall did not remove the application' >&2; exit 1; }
echo 'PASS: NSIS install/uninstall lifecycle. NOT RUN: clean Windows 10, WebView2-missing interaction, signed publisher trust.'
