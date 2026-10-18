# Shared checks for V0.2.10 rollout scripts (source only; do not execute directly).
IMPLEMENTATION_COMMIT="eea206783d102c4468f092ca07f1dce0cd95b02a"
BACKUP_VCS_PREFIX="21092d3"

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

binary_vcs_field() {
  local binpath="$1" field="$2"
  go version -m "$binpath" 2>/dev/null | sed -n "s/^[[:space:]]*build[[:space:]]*${field}=//p" | head -1
}

verify_git_contains_implementation() {
  local head
  head="$(git rev-parse HEAD)"
  if git merge-base --is-ancestor "$IMPLEMENTATION_COMMIT" HEAD; then
    info "git HEAD ${head:0:12} contains implementation ${IMPLEMENTATION_COMMIT:0:12} (ancestor check OK)"
  else
    die "git HEAD $head is not a descendant of implementation $IMPLEMENTATION_COMMIT"
  fi
}

verify_backup_binary() {
  local path="$1"
  local rev
  rev="$(binary_vcs_field "$path" "vcs.revision")"
  if [[ -z "$rev" ]]; then
    go version -m "$path" 2>/dev/null | grep -q "$BACKUP_VCS_PREFIX" || \
      die "backup $path missing vcs.revision and does not match $BACKUP_VCS_PREFIX"
    info "backup $path matches V0.2.9-era revision ($BACKUP_VCS_PREFIX)"
    return
  fi
  [[ "$rev" == *"$BACKUP_VCS_PREFIX"* ]] || die "backup vcs.revision=$rev expected *$BACKUP_VCS_PREFIX*"
  info "backup vcs.revision=${rev:0:12} (V0.2.9)"
}

verify_candidate_binary() {
  local path="$1"
  local rev modified
  rev="$(binary_vcs_field "$path" "vcs.revision")"
  modified="$(binary_vcs_field "$path" "vcs.modified")"
  [[ -n "$rev" ]] || die "candidate $path has no build vcs.revision (rebuild with go1.18+)"
  [[ "$rev" == "$IMPLEMENTATION_COMMIT" ]] || \
    die "candidate vcs.revision=$rev expected $IMPLEMENTATION_COMMIT"
  [[ "$modified" == "false" ]] || \
    die "candidate vcs.modified=$modified (require false; rebuild from clean tree at $IMPLEMENTATION_COMMIT)"
  info "candidate vcs.revision=${rev:0:12} vcs.modified=false"
}

report_live_binary() {
  local path="$1"
  if [[ ! -x "$path" ]]; then
    info "live binary $path not present or not executable"
    return
  fi
  local rev modified
  rev="$(binary_vcs_field "$path" "vcs.revision")"
  modified="$(binary_vcs_field "$path" "vcs.modified")"
  if [[ -n "$rev" ]]; then
    info "live $path vcs.revision=${rev:0:12} vcs.modified=${modified:-unknown}"
  else
    go version -m "$path" 2>/dev/null | grep -E 'mod github.com/gcostello02/mynet' || true
  fi
}

find_mynet_daemon_pids() {
  local repo="$1" live="$2"
  PIDS=()
  while IFS= read -r pid; do
    [[ -n "$pid" ]] && PIDS+=("$pid")
  done < <(pgrep -f "${repo}/${live} _daemon" 2>/dev/null || true)
  if [[ ${#PIDS[@]} -eq 0 ]]; then
    while IFS= read -r pid; do
      [[ -n "$pid" ]] && PIDS+=("$pid")
    done < <(pgrep -f "mynet _daemon" 2>/dev/null || true)
  fi
}

preflight_daemon_identity() {
  local repo="$1" live="$2"
  find_mynet_daemon_pids "$repo" "$live"
  if [[ ${#PIDS[@]} -gt 1 ]]; then
    die "multiple mynet _daemon PIDs: ${PIDS[*]}"
  fi
  if [[ ${#PIDS[@]} -eq 1 ]]; then
    info "mynet _daemon PID=${PIDS[0]} (would stop via mynet daemon stop in live mode)"
    ps -p "${PIDS[0]}" -o pid,user,command 2>/dev/null || ps -p "${PIDS[0]}" -o pid,user,cmd 2>/dev/null || true
  else
    info "no mynet _daemon process found"
  fi
}
