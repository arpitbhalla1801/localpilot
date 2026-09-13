#!/usr/bin/env bash
#
# Black-box end-to-end test suite for localpilot.
#
# Builds the real binary and drives it exactly the way a shell user would:
# real listening sockets, real PIDs, piping, non-interactive stdin, JSON
# validation with jq, exit code checks. Does not touch Go internals.
#
# Usage: ./tests/e2e.sh [-v]
#   -v   keep the temp workdir and print every command's full output

set -u
set -o pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="$ROOT_DIR/tests/.bin/localpilot"
VERBOSE=0
[ "${1:-}" = "-v" ] && VERBOSE=1

PASS=0
FAIL=0
SKIP=0
FAILED_NAMES=()

# --- helpers ----------------------------------------------------------

log()  { printf '%s\n' "$*" >&2; }
note() { [ "$VERBOSE" = 1 ] && printf '    %s\n' "$*" >&2; }

pass() { PASS=$((PASS + 1)); printf '  \033[32mok\033[0m   - %s\n' "$1"; }
fail() {
  FAIL=$((FAIL + 1))
  FAILED_NAMES+=("$1")
  printf '  \033[31mFAIL\033[0m - %s\n' "$1"
  shift
  for line in "$@"; do printf '        %s\n' "$line"; done
}
skip() { SKIP=$((SKIP + 1)); printf '  \033[33mskip\033[0m - %s (%s)\n' "$1" "$2"; }

# assert_eq NAME ACTUAL EXPECTED
assert_eq() {
  if [ "$2" = "$3" ]; then pass "$1"; else fail "$1" "expected: $3" "actual:   $2"; fi
}

# assert_contains NAME HAYSTACK NEEDLE
assert_contains() {
  case "$2" in
    *"$3"*) pass "$1" ;;
    *) fail "$1" "expected output to contain: $3" "actual output:" "$2" ;;
  esac
}

assert_not_contains() {
  case "$2" in
    *"$3"*) fail "$1" "expected output NOT to contain: $3" "actual output:" "$2" ;;
    *) pass "$1" ;;
  esac
}

# free_port: ask the OS for an unused TCP port
free_port() {
  python3 -c '
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
'
}

# start_listener PORT -> prints PID, leaves it running in the background
LISTENER_PIDS=()
start_listener() {
  local port="$1"
  ( exec nc -l "$port" >/dev/null 2>&1 ) &
  local pid=$!
  LISTENER_PIDS+=("$pid")
  # wait until it's actually bound
  for _ in $(seq 1 50); do
    if lsof -nP -iTCP:"$port" -sTCP:LISTEN >/dev/null 2>&1; then
      echo "$pid"
      return 0
    fi
    sleep 0.05
  done
  echo "$pid"
}

cleanup() {
  for pid in "${LISTENER_PIDS[@]:-}"; do
    kill "$pid" >/dev/null 2>&1
  done
  wait >/dev/null 2>&1
  [ "$VERBOSE" = 1 ] || rm -rf "$ROOT_DIR/tests/.bin"
}
trap cleanup EXIT

have() { command -v "$1" >/dev/null 2>&1; }

# hangs_longer_than SECS CMD...
# Portable stand-in for GNU `timeout` (macOS ships none by default).
# Runs CMD in the background, polls up to SECS for it to finish. Echoes
# "0" if it finished in time, "1" (and kills it) if it's still running.
hangs_longer_than() {
  local secs="$1"; shift
  "$@" &
  local cmd_pid=$!
  local waited=0
  while kill -0 "$cmd_pid" 2>/dev/null; do
    if [ "$waited" -ge $((secs * 10)) ]; then
      kill -9 "$cmd_pid" 2>/dev/null
      wait "$cmd_pid" 2>/dev/null
      echo 1
      return
    fi
    sleep 0.1
    waited=$((waited + 1))
  done
  wait "$cmd_pid" 2>/dev/null
  echo 0
}

# --- build --------------------------------------------------------------

export PATH="/opt/homebrew/opt/go/bin:$PATH"
if ! have go; then
  log "go not found on PATH; cannot build localpilot for testing"
  exit 2
fi

mkdir -p "$ROOT_DIR/tests/.bin"
log "Building localpilot..."
if ! go build -o "$BIN" "$ROOT_DIR/cmd/localpilot" 2>&1 | tee /tmp/localpilot-build.log >&2; then
  log "build failed, see above"
  exit 2
fi

log ""
log "=== localpilot e2e test suite ==="
log ""

# =========================================================================
# 1. Argument parsing / validation
# =========================================================================
log "-- argument parsing --"

out=$("$BIN" port 3000abc 2>&1); code=$?
assert_contains "port: rejects trailing garbage (3000abc)" "$out" "invalid port"
assert_eq       "port: trailing garbage exits non-zero" "$code" "1"

out=$("$BIN" port +80 2>&1)
assert_contains "port: rejects leading plus sign" "$out" "invalid port"

out=$("$BIN" port 0 2>&1); code=$?
assert_contains "port: rejects 0" "$out" "invalid port"
assert_eq       "port: rejects 0 exits non-zero" "$code" "1"

out=$("$BIN" port 65536 2>&1)
assert_contains "port: rejects > 65535" "$out" "invalid port"

out=$("$BIN" port abc 2>&1); code=$?
assert_contains "port: rejects non-numeric" "$out" "invalid port"
assert_eq       "port: non-numeric exits non-zero" "$code" "1"

out=$("$BIN" inspect -5 2>&1)
assert_contains "inspect: negative PID gives clean error (not Cobra flag error)" "$out" "invalid PID"
assert_not_contains "inspect: negative PID does not leak Cobra flag error" "$out" "shorthand"

out=$("$BIN" inspect 999999999 2>&1); code=$?
assert_eq "inspect: huge nonexistent PID exits non-zero" "$code" "1"
note "output: $out"

out=$("$BIN" inspect 99999999999999999999 2>&1); code=$?
assert_eq "inspect: int64-overflowing PID exits non-zero (no panic)" "$code" "1"

out=$("$BIN" port 2>&1); code=$?
assert_eq "port: missing arg exits non-zero" "$code" "1"

out=$("$BIN" port 80 90 2>&1); code=$?
assert_eq "port: extra positional arg exits non-zero" "$code" "1"

out=$("$BIN" bogus-command 2>&1); code=$?
assert_eq "unknown subcommand exits non-zero" "$code" "1"

# =========================================================================
# 2. JSON validity
# =========================================================================
log ""
log "-- JSON output --"

if have jq; then
  out=$("$BIN" list --json 2>&1)
  # Deliberately `jq .`, not `jq -e .`: an empty ports list marshals to the
  # literal JSON `null` (Go's encoding/json for a nil slice), which is valid
  # JSON but which `-e` treats as a falsy result and flags as failure.
  if echo "$out" | jq . >/dev/null 2>&1; then
    pass "list --json: produces valid JSON (jq parses it)"
  else
    fail "list --json: NOT valid JSON" "$out"
  fi

  out=$("$BIN" port 3000abc --json 2>&1)
  # invalid input should still not print JSON to stdout that a jq pipeline would misparse
  if echo "$out" | jq . >/dev/null 2>&1; then
    fail "port --json: error path should not print JSON to stdout on invalid input" "$out"
  else
    pass "port --json: error path prints plain error, not malformed JSON"
  fi
else
  skip "JSON validity checks" "jq not installed"
fi

# free port --json shape
fp=$(free_port)
out=$("$BIN" port "$fp" --json 2>&1)
assert_contains "port --json: free port has inUse:false" "$out" '"inUse": false'
assert_not_contains "port --json: free port omits pid key" "$out" '"pid"'

# camelCase field check (regression guard for issue #10)
pid=$$
out=$("$BIN" inspect "$pid" --json 2>&1)
if echo "$out" | grep -q '"pid"'; then
  pass "inspect --json: uses camelCase 'pid' not 'PID'"
else
  fail "inspect --json: missing lowercase 'pid' field" "$out"
fi
if echo "$out" | grep -qE '"PID"|"CPUPercent"|"MemoryBytes"'; then
  fail "inspect --json: leaking PascalCase Go field names" "$out"
else
  pass "inspect --json: no PascalCase field names leaked"
fi

# =========================================================================
# 3. Real listening port / process lifecycle
# =========================================================================
log ""
log "-- live port + process behavior --"

port1=$(free_port)
pid1=$(start_listener "$port1")
sleep 0.2

out=$("$BIN" port "$port1" 2>&1)
assert_contains "port: detects a real live listener" "$out" "IN USE"
assert_contains "port: reports correct PID for live listener" "$out" "$pid1"

out=$("$BIN" list 2>&1)
assert_contains "list: shows the live port" "$out" "$port1"

out=$("$BIN" inspect "$pid1" 2>&1); code=$?
assert_eq "inspect: succeeds for a real running PID" "$code" "0"

kill "$pid1" 2>/dev/null
wait "$pid1" 2>/dev/null
sleep 0.3

out=$("$BIN" port "$port1" 2>&1); code=$?
assert_contains "port: reports FREE after process exits" "$out" "FREE"
assert_eq       "port: FREE exits 0 (not an error)" "$code" "0"

out=$("$BIN" inspect "$pid1" 2>&1); code=$?
assert_eq "inspect: exits non-zero for a PID that no longer exists" "$code" "1"

# =========================================================================
# 4. kill: confirmation / non-interactive / force
# =========================================================================
log ""
log "-- kill safety behavior --"

port2=$(free_port)
pid2=$(start_listener "$port2")
sleep 0.2

out=$("$BIN" kill "$port2" </dev/null 2>&1); code=$?
assert_contains "kill: refuses to prompt when stdin is not a tty" "$out" "not a terminal"
assert_eq       "kill: non-interactive refusal exits non-zero" "$code" "1"
if kill -0 "$pid2" 2>/dev/null; then
  pass "kill: process survives when confirmation was refused"
else
  fail "kill: process was killed despite refusing to prompt!"
fi

out=$("$BIN" kill "$port2" --force </dev/null 2>&1); code=$?
assert_eq "kill --force: succeeds non-interactively" "$code" "0"
sleep 0.3
if kill -0 "$pid2" 2>/dev/null; then
  fail "kill --force: process is still alive after kill --force"
else
  pass "kill --force: process is actually gone"
fi

# kill a PID that doesn't correspond to any port or process
out=$("$BIN" kill 999999999 --force 2>&1); code=$?
assert_eq "kill --force: nonexistent target exits non-zero" "$code" "1"

# =========================================================================
# 5. Piping / broken pipe / non-tty stdout
# =========================================================================
log ""
log "-- piping and stream behavior --"

# broken pipe: downstream closes early, localpilot must not hang or dump a panic
port3=$(free_port)
pid3=$(start_listener "$port3")
sleep 0.2
for i in $(seq 1 40); do
  port_n=$(free_port)
  ( exec nc -l "$port_n" >/dev/null 2>&1 ) &
  LISTENER_PIDS+=("$!")
done
sleep 0.3

out=$("$BIN" list 2>&1 | head -n 3)
hung=$(hangs_longer_than 10 bash -c "'$BIN' list >/tmp/localpilot-headtest.out 2>&1 | head -n 3 >/dev/null")
assert_eq "list piped into 'head -3' does not hang" "$hung" "0"
assert_not_contains "list | head: no Go panic leaked to output" "$out" "panic:"

out=$("$BIN" 2>&1 | cat); code=${PIPESTATUS[0]}
assert_eq "dashboard piped through cat exits 0" "$code" "0"

kill "$pid3" 2>/dev/null

# =========================================================================
# 6. Locale sensitivity
# =========================================================================
log ""
log "-- locale / environment sensitivity --"

out=$(LC_ALL=C LANG=C "$BIN" list --json 2>&1); code=$?
assert_eq "list --json under LC_ALL=C exits 0" "$code" "0"
if have jq && echo "$out" | jq . >/dev/null 2>&1; then
  pass "list --json under LC_ALL=C is still valid JSON"
elif have jq; then
  fail "list --json under LC_ALL=C is NOT valid JSON" "$out"
else
  skip "LC_ALL=C JSON validity" "jq not installed"
fi

# =========================================================================
# 7. Environment variable masking
# =========================================================================
log ""
log "-- secret masking (internal/security) --"

secret_pid=$(AWS_SECRET_ACCESS_KEY=supersecretvalue123 MY_APP_TOKEN=tok_abc123 NOT_SENSITIVE=hello \
  python3 -c '
import os, subprocess
p = subprocess.Popen(["sleep", "5"], env=os.environ.copy())
print(p.pid)
')
sleep 0.2
if [ -n "$secret_pid" ] && kill -0 "$secret_pid" 2>/dev/null; then
  insp=$("$BIN" inspect "$secret_pid" --json 2>&1)
  assert_not_contains "inspect --json: does not leak AWS_SECRET_ACCESS_KEY value" "$insp" "supersecretvalue123"
  kill "$secret_pid" 2>/dev/null
else
  skip "env masking check" "could not capture child PID reliably in this shell"
fi

# =========================================================================
# 8. Concurrency
# =========================================================================
log ""
log "-- concurrent invocations --"

port4=$(free_port)
pid4=$(start_listener "$port4")
sleep 0.2

conc_ok=1
conc_pids=()
for i in 1 2 3 4 5; do
  "$BIN" port "$port4" >/tmp/localpilot-conc-$i.out 2>&1 &
  conc_pids+=("$!")
done
for p in "${conc_pids[@]}"; do wait "$p"; done
for i in 1 2 3 4 5; do
  grep -q "IN USE" /tmp/localpilot-conc-$i.out || conc_ok=0
  rm -f /tmp/localpilot-conc-$i.out
done
if [ "$conc_ok" = 1 ]; then
  pass "5 concurrent 'port' invocations all report consistent results"
else
  fail "5 concurrent 'port' invocations: inconsistent output" "check /tmp/localpilot-conc-*.out"
fi

kill "$pid4" 2>/dev/null

# =========================================================================
# summary
# =========================================================================
log ""
log "=== summary: $PASS passed, $FAIL failed, $SKIP skipped ==="
if [ "$FAIL" -gt 0 ]; then
  log ""
  log "Failed:"
  for n in "${FAILED_NAMES[@]}"; do log "  - $n"; done
  exit 1
fi
exit 0
