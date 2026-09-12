#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
pyahu_bin="${PYAHU_BIN:-$repo_root/bin/pyahu}"

if ! command -v k3d >/dev/null 2>&1; then
  echo "k3d is required for the real smoke test" >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required for the real smoke test" >&2
  exit 1
fi
if [[ ! -x "$pyahu_bin" ]]; then
  echo "Pyahu binary not found at $pyahu_bin; run 'mise run build' first" >&2
  exit 1
fi

pick_port() {
  python3 - <<'PY'
import socket

with socket.socket() as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
}

run_id="${GITHUB_RUN_ID:-$$}"
run_id="$(printf '%s' "$run_id" | tr -cd 'a-zA-Z0-9' | tr '[:upper:]' '[:lower:]')"
cluster_name="pyahu-e2e-${run_id:-local}"
namespace="${cluster_name}-dev"
temp_dir="$(mktemp -d)"
stack_file="$temp_dir/pyahu.yaml"
postgres_port="$(pick_port)"
redis_port="$(pick_port)"
while [[ "$redis_port" == "$postgres_port" ]]; do
  redis_port="$(pick_port)"
done

export XDG_CONFIG_HOME="$temp_dir/config"
mkdir -p "$XDG_CONFIG_HOME"

cluster_config=""
if [[ -n "${PYAHU_E2E_K3S_IMAGE:-}" ]]; then
  cluster_config="cluster:
  k3sVersion: $PYAHU_E2E_K3S_IMAGE"
fi

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  set +e
  "$pyahu_bin" --no-input --no-color --quiet --file "$stack_file" down --purge-data --yes >/dev/null 2>&1
  k3d cluster delete "$cluster_name" >/dev/null 2>&1
  rm -rf -- "$temp_dir"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

write_stack_with_redis() {
  cat >"$stack_file" <<YAML
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: $cluster_name
$cluster_config
services:
  postgres:
    enabled: true
    ports:
      primary: $postgres_port
    storage: 256Mi
    databases:
      - name: app
  redis:
    enabled: true
    ports:
      client: $redis_port
    storage: 128Mi
YAML
}

write_stack_without_redis() {
  cat >"$stack_file" <<YAML
apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: $cluster_name
$cluster_config
services:
  postgres:
    enabled: true
    ports:
      primary: $postgres_port
    storage: 256Mi
    databases:
      - name: app
YAML
}

echo "Creating real k3d stack $cluster_name"
write_stack_with_redis
"$pyahu_bin" --no-input --no-color --file "$stack_file" up

status_json="$temp_dir/status.json"
"$pyahu_bin" --no-input --no-color --file "$stack_file" status --output json >"$status_json"
python3 - "$status_json" <<'PY'
import json
import sys

with open(sys.argv[1], encoding="utf-8") as source:
    status = json.load(source)
assert status["running"] is True
services = {service["name"]: service for service in status["services"]}
for name in ("postgres", "redis"):
    assert services[name]["enabled"] is True
    assert services[name]["ready"] is True
PY

python3 - "$cluster_name" "$postgres_port" <<'PY'
import json
import subprocess
import sys

cluster_name, expected_port = sys.argv[1:]
container_ids = subprocess.check_output(
    ["docker", "ps", "--filter", f"name=k3d-{cluster_name}-", "--format", "{{.ID}}"],
    text=True,
).split()
containers = json.loads(subprocess.check_output(["docker", "inspect", *container_ids]))
bindings = []
for container in containers:
    for published in (container["NetworkSettings"]["Ports"] or {}).values():
        for binding in published or []:
            if binding["HostPort"] == expected_port:
                bindings.append(binding)
assert bindings, f"host port {expected_port} is not published by the k3d cluster"
assert all(binding["HostIp"] == "127.0.0.1" for binding in bindings), bindings
PY

echo "Verifying PostgreSQL backup and restore"
mkdir -p "$temp_dir/backups"
"$pyahu_bin" --no-input --no-color --file "$stack_file" backup postgres app --dir "$temp_dir/backups"
backup_file="$(find "$temp_dir/backups" -type f -name '*.dump' -print -quit)"
if [[ -z "$backup_file" || ! -s "$backup_file" ]]; then
  echo "PostgreSQL backup was not created" >&2
  exit 1
fi
"$pyahu_bin" --no-input --no-color --file "$stack_file" restore postgres app --source "$backup_file" --yes

echo "Verifying idempotent apply"
"$pyahu_bin" --no-input --no-color --file "$stack_file" up

echo "Verifying disabled-service pruning and PVC retention"
write_stack_without_redis
"$pyahu_bin" --no-input --no-color --file "$stack_file" up
server_container="k3d-$cluster_name-server-0"
if docker exec "$server_container" kubectl -n "$namespace" get service redis >/dev/null 2>&1; then
  echo "obsolete Redis Service still exists" >&2
  exit 1
fi
if docker exec "$server_container" kubectl -n "$namespace" get statefulset redis >/dev/null 2>&1; then
  echo "obsolete Redis StatefulSet still exists" >&2
  exit 1
fi
docker exec "$server_container" kubectl -n "$namespace" get pvc data-redis-0 >/dev/null

echo "Real k3d smoke test passed"
