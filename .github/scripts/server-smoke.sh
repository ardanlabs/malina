#!/usr/bin/env bash

set -Eeuo pipefail

if [[ -z "${MALINA_LIB:-}" ]]; then
  echo "MALINA_LIB must point to an installed native library bundle" >&2
  exit 1
fi

address="${MALINA_SMOKE_ADDR:-127.0.0.1:$((20000 + RANDOM % 20000))}"
base_url="http://$address"
log_file="$(mktemp -t malina-server-smoke.XXXXXX)"
server_pid=""
passed=false

cleanup() {
  local status=$?
  trap - EXIT INT TERM

  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill -TERM "$server_pid" 2>/dev/null || true
    for _ in {1..40}; do
      kill -0 "$server_pid" 2>/dev/null || break
      sleep 0.25
    done
    if kill -0 "$server_pid" 2>/dev/null; then
      echo "server did not stop cleanly" >&2
      kill -KILL "$server_pid" 2>/dev/null || true
      status=1
    fi
  fi
  if [[ -n "$server_pid" ]]; then
    wait "$server_pid" 2>/dev/null || {
      [[ "$status" -ne 0 ]] || status=1
    }
  fi

  if [[ "$passed" != true || "$status" -ne 0 ]]; then
    echo "--- malina server log ---" >&2
    cat "$log_file" >&2
  fi
  rm -f "$log_file"
  exit "$status"
}
trap cleanup EXIT INT TERM

malina server start --host "$address" --bui=true --model "" >"$log_file" 2>&1 &
server_pid=$!

healthy=false
for _ in {1..120}; do
  if [[ "$(curl -sS -o /dev/null -w '%{http_code}' "$base_url/healthz" 2>/dev/null || true)" == 200 ]]; then
    healthy=true
    break
  fi
  kill -0 "$server_pid" 2>/dev/null || {
    echo "server exited before becoming healthy" >&2
    exit 1
  }
  sleep 0.25
done
[[ "$healthy" == true ]] || { echo "health check timed out" >&2; exit 1; }

check_status() {
  local path=$1
  local expected=$2
  local actual
  actual="$(curl -sS -o /dev/null -w '%{http_code}' "$base_url$path")"
  [[ "$actual" == "$expected" ]] || {
    echo "$path returned $actual, expected $expected" >&2
    return 1
  }
}

check_status /healthz 200
check_status /readyz 503
check_status /admin/ 200
passed=true
