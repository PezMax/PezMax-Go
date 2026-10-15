#!/usr/bin/env bash
set -euo pipefail

tests_dir=$(cd "$(dirname "$0")" && pwd)
repo_dir=$(cd "$tests_dir/../.." && pwd)
run_id="pz-egress-$(date +%s)-$$"
network="$run_id-network"
control=$(mktemp -d "/tmp/$run_id.XXXXXX")
image=${EDGE_TEST_IMAGE:-pezmax-go-edge:local}
containers=()

cleanup() {
  if ((${#containers[@]})); then docker rm -f "${containers[@]}" >/dev/null 2>&1 || true; fi
  docker network rm "$network" >/dev/null 2>&1 || true
  echo "Test logs: $control"
}
trap cleanup EXIT

wait_for_file() {
  local path=$1
  local deadline=$((SECONDS + 45))
  while [[ ! -f "$path" ]]; do
    if ((SECONDS >= deadline)); then echo "Timed out: $path" >&2; return 1; fi
    sleep 0.1
  done
}

wait_for_http() {
  local container=$1 url=$2
  for _ in {1..50}; do
    if docker exec "$container" wget -q -O /dev/null "$url" >/dev/null 2>&1; then return; fi
    sleep 0.1
  done
  docker logs "$container" >&2 || true
  echo "HTTP not ready: $container" >&2
  return 1
}

docker image inspect "$image" >/dev/null
docker image inspect nginx:1.27-alpine >/dev/null
docker network create "$network" >/dev/null

fixture="$run_id-fixture"
containers+=("$fixture")
docker run -d --name "$fixture" --network "$network" --network-alias fixture \
  --mount "type=bind,source=$tests_dir/fixture.conf,target=/etc/nginx/conf.d/default.conf,readonly" \
  --mount "type=bind,source=$tests_dir,target=/tests,readonly" \
  --tmpfs /fixtures:rw,size=8m --entrypoint sh nginx:1.27-alpine \
  /tests/fixture-start.sh >/dev/null
wait_for_http "$fixture" http://127.0.0.1/health

edge="$run_id-edge"
containers+=("$edge")
docker run -d --name "$edge" --network "$network" --network-alias edge --cap-add NET_ADMIN \
  -e KADMIN_UPSTREAM=fixture:80 -e KADMIN_EDGE_EGRESS_RATE=8mbit \
  -e KADMIN_EDGE_EGRESS_BURST_BYTES=32768 \
  -e KADMIN_EDGE_DL_RATE_FAST=5m -e KADMIN_EDGE_DL_RATE_MID=2560k \
  -e KADMIN_EDGE_DL_RATE_LOW=640k -e KADMIN_EDGE_DL_MAXCONN=16 \
  --mount "type=bind,source=$repo_dir/deploy/nginx.conf.template,target=/etc/nginx/templates/default.conf.template,readonly" \
  --mount "type=bind,source=$repo_dir/deploy/kadmin-proxy-headers.conf,target=/etc/nginx/kadmin-proxy-headers.conf,readonly" \
  --mount "type=bind,source=$tests_dir,target=/tests,readonly" \
  --mount "type=bind,source=$control,target=/test-control" \
  "$image" >/dev/null
wait_for_http "$edge" http://127.0.0.1/stub_status
docker exec "$edge" tc qdisc show dev eth0 | grep -q '^qdisc tbf 1: root'
docker exec "$edge" tc qdisc show dev eth0 | grep -q '^qdisc fq_codel 10: parent 1:1'

clients=()
for number in 1 2 3 4; do
  client="$run_id-client-$number"
  clients+=("$client")
  containers+=("$client")
  docker run -d --name "$client" --network "$network" \
    --mount "type=bind,source=$tests_dir,target=/tests,readonly" \
    --mount "type=bind,source=$control,target=/test-control" \
    --entrypoint sh nginx:1.27-alpine -c 'sleep 3600' >/dev/null
done

run_scenario() {
  local name=$1 count=$2 mixed=$3 number url start end
  for ((number = 1; number <= count; number++)); do
    url="http://edge/datum/download/file?fileId=fixture-$number"
    if [[ "$mixed" == true && $((number % 2)) == 0 ]]; then
      url=http://edge/ordinary/payload.bin
    fi
    docker exec -d "${clients[number-1]}" sh /tests/download-client.sh "$name" "$number" "$url"
  done
  docker exec -d "$edge" sh /tests/monitor-egress.sh "$name"
  for ((number = 1; number <= count; number++)); do
    wait_for_file "$control/$name-client-$number.ready"
  done
  wait_for_file "$control/$name-monitor.ready"
  start=$(date +%s.%N)
  touch "$control/$name.start"
  for ((number = 1; number <= count; number++)); do
    wait_for_file "$control/$name-client-$number.done"
    IFS='|' read -r status bytes first last < "$control/$name-client-$number.done"
    [[ "$status" == 0 && "$bytes" == 2097152 ]]
    grep -Eq 'HTTP/1\.[01] 200' "$control/$name-client-$number.log"
    echo "  client $number: $bytes bytes, $(awk -v a="$first" -v b="$last" 'BEGIN {printf "%.2f", b-a}') s"
  done
  end=$(date +%s.%N)
  touch "$control/$name.stop"
  wait_for_file "$control/$name-monitor.done"
  python3 - "$control/$name-tc.samples" "$name" "$count" "$start" "$end" <<'PY'
import json
import sys

path, name, count, start, end = sys.argv[1:]
count = int(count)
elapsed = float(end) - float(start)
sent = count * 2 * 1024 * 1024
rate = sent / elapsed
budget = 1_000_000
assert rate >= budget * 0.70, f"{name}: pool not saturated ({rate:.0f} B/s)"
assert sent <= budget * elapsed * 1.05 + 32768 + 3000, f"{name}: shared budget exceeded"
samples = []
for line in open(path, encoding="utf-8"):
    stamp, payload = line.split("|", 1)
    root = [q for q in json.loads(payload) if q.get("kind") == "tbf" and q.get("root")]
    assert len(root) == 1 and "bytes" in root[0]
    samples.append((float(stamp), int(root[0]["bytes"])))
assert len(samples) >= 3
windows = {(0, len(samples) - 1)}
for left in range(len(samples) - 1):
    for target in (1.0, 5.0):
        right = next((i for i in range(left + 1, len(samples)) if samples[i][0] - samples[left][0] >= target), None)
        if right is not None:
            windows.add((left, right))
for left, right in windows:
    seconds = samples[right][0] - samples[left][0]
    bytes_sent = samples[right][1] - samples[left][1]
    assert bytes_sent <= budget * seconds * 1.05 + 32768 + 3000, f"{name}: tc window exceeded"
print(f"{name}: {rate:.0f} B/s total, {len(windows)} tc windows passed")
PY
}

run_scenario single 1 false
run_scenario two-clients 2 false
run_scenario four-clients 4 false
run_scenario mixed-routes 4 true
echo 'All isolated edge egress scenarios passed.'
