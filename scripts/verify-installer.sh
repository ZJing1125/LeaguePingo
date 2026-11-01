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
export LEAGUEPINGO_VERIFY_EXECUTABLE="$task_installer_windows"
export LEAGUEPINGO_VERIFY_INSTALL_ROOT="$task_install_windows"
powershell.exe -NoProfile -NonInteractive -Command '
  $ErrorActionPreference = "Stop"
  $process = Start-Process -FilePath $env:LEAGUEPINGO_VERIFY_EXECUTABLE -ArgumentList @("/S", "/D=$($env:LEAGUEPINGO_VERIFY_INSTALL_ROOT)") -PassThru
  if (-not $process.WaitForExit(120000)) {
    try { $process.Kill() } catch {}
    throw "NSIS installer timed out after 120 seconds"
  }
  if ($process.ExitCode -ne 0) { throw "NSIS installer exited with code $($process.ExitCode)" }
'
[[ -f "$task_install_root/LeaguePingo.exe" && -f "$task_install_root/uninstall.exe" ]] || { echo 'Silent NSIS installation did not create expected files' >&2; exit 1; }
export LEAGUEPINGO_VERIFY_EXECUTABLE
LEAGUEPINGO_VERIFY_EXECUTABLE=$(cygpath -w "$task_install_root/uninstall.exe")
powershell.exe -NoProfile -NonInteractive -Command '
  $ErrorActionPreference = "Stop"
  $process = Start-Process -FilePath $env:LEAGUEPINGO_VERIFY_EXECUTABLE -ArgumentList "/S" -PassThru
  if (-not $process.WaitForExit(60000)) {
    try { $process.Kill() } catch {}
    throw "NSIS uninstaller timed out after 60 seconds"
  }
  if ($process.ExitCode -ne 0) { throw "NSIS uninstaller exited with code $($process.ExitCode)" }
'
for _ in {1..40}; do
  [[ ! -e "$task_install_root/LeaguePingo.exe" && ! -e "$task_install_root/uninstall.exe" ]] && break
  sleep 0.25
done
[[ ! -e "$task_install_root/LeaguePingo.exe" && ! -e "$task_install_root/uninstall.exe" ]] || { echo 'Silent NSIS uninstall did not remove the application' >&2; exit 1; }
echo 'PASS: NSIS install/uninstall lifecycle. NOT RUN: clean Windows 10, WebView2-missing interaction, signed publisher trust.'
