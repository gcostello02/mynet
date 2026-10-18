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

default_state_dir() {
  echo "${MYNET_STATE_DIR:-${HOME}/.mynet}"
}

# Other processes running this repo's mynet binary (not _daemon).
find_other_mynet_pids() {
  local repo="$1" live="$2"
  OTHER_MYNET_PIDS=()
  while IFS= read -r line; do
    [[ -z "$line" ]] && continue
    local pid rest
    pid="${line%% *}"
    rest="${line#"$pid "}"
    [[ "$rest" == *"_daemon"* ]] && continue
    OTHER_MYNET_PIDS+=("$pid")
  done < <(pgrep -fl "${repo}/${live}" 2>/dev/null || true)
}

preflight_other_mynet_procs() {
  local repo="$1" live="$2"
  find_other_mynet_pids "$repo" "$live"
  if [[ ${#OTHER_MYNET_PIDS[@]} -gt 0 ]]; then
    die "unexpected mynet processes (not _daemon): ${OTHER_MYNET_PIDS[*]} — resolve before deploy"
  fi
  info "no extra mynet CLI processes for ${repo}/${live}"
}

verify_live_not_already_candidate() {
  local live="$1"
  [[ -x "$live" ]] || return 0
  local rev
  rev="$(binary_vcs_field "$live" "vcs.revision")"
  if [[ -n "$rev" && "$rev" == "$IMPLEMENTATION_COMMIT" ]]; then
    die "live binary already at V0.2.10 implementation $IMPLEMENTATION_COMMIT (nothing to install)"
  fi
}

assert_clean_install_artifacts() {
  local live="$1"
  [[ ! -e "${live}.new" ]] || die "leftover ${live}.new from a failed deploy — remove or finish manually"
}

report_ipc_and_dataplane_state() {
  local sd
  sd="$(default_state_dir)"
  local sock="${sd}/mynetd.sock"
  local pidf="${sd}/mynetd.pid"
  if [[ -e "$sock" ]]; then
    if [[ ${#PIDS[@]} -eq 1 ]]; then
      info "daemon IPC socket present (mynet _daemon running)"
    elif python3 -c "
import socket
s=socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
s.settimeout(0.5)
try:
 s.connect('${sock}')
except OSError:
 raise SystemExit(1)
raise SystemExit(0)
" 2>/dev/null; then
      info "IPC socket accepts connections (unexpected without _daemon — investigate)"
    else
      info "stale or dead IPC socket at $sock (mynet up will RemoveStaleSocket when needed)"
    fi
  else
    info "no daemon IPC socket at $sock"
  fi
  if [[ -e "$pidf" ]]; then
    local pidtxt=""
    pidtxt="$(cat "$pidf" 2>/dev/null || true)"
    if [[ -n "$pidtxt" ]] && kill -0 "$pidtxt" 2>/dev/null; then
      info "mynetd.pid=$pidtxt references a live process"
    elif [[ -n "$pidtxt" ]]; then
      info "mynetd.pid=$pidtxt is stale (process not running)"
    else
      info "mynetd.pid present but unreadable (likely root-owned); pgrep is authoritative for daemon identity"
    fi
  else
    info "no mynetd.pid file"
  fi
  if [[ "$(uname -s)" == "Darwin" ]]; then
    if netstat -rn -f inet 2>/dev/null | grep -q '172\.31\.240'; then
      info "172.31.240 routes still present on macOS (VPN may be partially up)"
    else
      info "no 172.31.240 routes on macOS (VPN data plane down; OK if daemon stopped)"
    fi
  elif [[ "$(uname -s)" == "Linux" ]]; then
    if ip link show mynet0 &>/dev/null; then
      info "mynet0 present on Linux"
    else
      info "mynet0 absent on Linux (OK if daemon/VPN stopped)"
    fi
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
    info "no mynet _daemon process found (OK for deploy; live mode will swap binary without stopping V0.2.9)"
  fi
  preflight_other_mynet_procs "$repo" "$live"
  report_ipc_and_dataplane_state
}

assert_dataplane_quiescent_for_install() {
  find_mynet_daemon_pids "${REPO:?}" "${LIVE:?}"
  if [[ ${#PIDS[@]} -ne 0 ]]; then
    return 0
  fi
  if [[ "$(uname -s)" == "Linux" ]] && ip link show mynet0 &>/dev/null; then
    die "mynet0 still present but no mynet _daemon — run sudo ./$LIVE down or investigate before install"
  fi
  if [[ "$(uname -s)" == "Darwin" ]] && netstat -rn -f inet 2>/dev/null | grep -q '172\.31\.240'; then
    die "172.31.240 routes still present but no mynet _daemon — investigate before install"
  fi
  info "data plane quiescent (no daemon; no MyNet routes/interface)"
}
