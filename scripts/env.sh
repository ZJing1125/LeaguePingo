#!/usr/bin/env bash
# Source after task_root is set. Optional config is trusted executable Bash,
# never download/copy it from another host without review.
if [[ -f "$task_root/.leaguepingo/environment.sh" ]]; then
  source "$task_root/.leaguepingo/environment.sh"
fi
if [[ -z "${LEAGUEPINGO_BASH:-}" ]]; then
  if command -v cygpath >/dev/null 2>&1; then
    export LEAGUEPINGO_BASH="$(cygpath -m "$BASH")"
  else
    export LEAGUEPINGO_BASH="$BASH"
  fi
fi
