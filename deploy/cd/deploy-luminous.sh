#!/usr/bin/env bash
set -euo pipefail

source_repo=${PEZMAX_CD_SOURCE_REPO:-/home/Luminous/PezMax-Go}
deploy_root=${PEZMAX_CD_ROOT:-/home/Luminous/pezmax-cd}
git_proxy=${PEZMAX_GIT_PROXY:-http://127.0.0.1:7890}
repo_name=PezMax/PezMax-Go
branch=Luminous
git_bin=/usr/bin/git
dry_run=false

if [[ ${1:-} == --dry-run ]]; then
  dry_run=true
elif [[ $# -ne 0 ]]; then
  echo 'Usage: deploy-luminous.sh [--dry-run]' >&2
  exit 2
fi

if [[ $EUID -ne 0 ]]; then
  echo 'Run as root so Docker can update the application containers.' >&2
  exit 1
fi
if [[ ! -f $source_repo/.env ]]; then
  echo "Missing server environment file: $source_repo/.env" >&2
  exit 1
fi

mkdir -p "$deploy_root/releases" "$deploy_root/state"
exec 9>"$deploy_root/deploy.lock"
if ! flock -n 9; then
  echo 'Another PezMax deployment is still running.'
  exit 0
fi

git_source() {
  sudo -u Luminous env HTTPS_PROXY="$git_proxy" HTTP_PROXY="$git_proxy" \
    "$git_bin" -C "$source_repo" "$@"
}

target=$(git_source ls-remote origin "refs/heads/$branch" | awk 'NR == 1 {print $1}')
if [[ ! $target =~ ^[0-9a-f]{40}$ ]]; then
  echo 'Could not resolve the Luminous commit.' >&2
  exit 1
fi

state_file=$deploy_root/state/current.sha
if [[ -f $state_file && $(<"$state_file") == "$target" ]]; then
  echo "Already deployed: $target"
  exit 0
fi

ci_url="https://api.github.com/repos/$repo_name/actions/runs?branch=$branch&head_sha=$target&event=push&per_page=10"
ci_result=$(curl -fsS --retry 2 --connect-timeout 5 --max-time 20 \
  -H 'Accept: application/vnd.github+json' -H 'User-Agent: pezmax-cd' \
  "$ci_url" | python3 -c '
import json, sys
target = sys.argv[1]
data = json.load(sys.stdin)
runs = data.get("workflow_runs", [])
matches = [run for run in runs if run.get("path") == ".github/workflows/ci.yml"]
run = matches[0] if matches else {}
if run and (run.get("head_sha") != target or run.get("head_branch") != "Luminous" or run.get("event") != "push"):
    raise SystemExit("GitHub returned a run for a different commit")
print(run.get("conclusion") or run.get("status") or "not-started")
' "$target")
if [[ $ci_result != success ]]; then
  echo "Waiting for CI on $target ($ci_result)."
  exit 0
fi

echo "CI passed for $target."
if [[ $dry_run == true ]]; then
  exit 0
fi

git_source fetch --no-tags origin "refs/heads/$branch"
fetched=$(git_source rev-parse FETCH_HEAD)
if [[ $fetched != "$target" ]]; then
  echo 'Luminous moved during deployment; retry on the next poll.'
  exit 0
fi

release=$deploy_root/releases/$target
if [[ ! -d $release ]]; then
  temporary=$deploy_root/releases/.tmp-$target-$$
  mkdir -p "$temporary"
  trap 'rm -rf -- "$temporary"' EXIT
  git_source archive "$target" | tar -x -C "$temporary"
  ln -s "$source_repo/.env" "$temporary/.env"
  mv "$temporary" "$release"
  trap - EXIT
fi

cd "$release"
export PEZMAX_RELEASE_SHA=$target
docker compose --profile app --profile edge config --quiet

# Build every application image before changing running containers.
docker compose --profile app --profile edge build kadmin admin-web edge

latest=$(git_source ls-remote origin "refs/heads/$branch" | awk 'NR == 1 {print $1}')
if [[ $latest != "$target" ]]; then
  echo 'Luminous moved during image build; leaving the running deployment unchanged.'
  exit 0
fi

rollback_tag=rollback-$(date +%s)
had_web=false
for service in kadmin edge admin-web; do
  container=pezmax-go-$service
  if old_image=$(docker inspect -f '{{.Image}}' "$container" 2>/dev/null); then
    docker image tag "$old_image" "pezmax-go-$service:$rollback_tag"
    [[ $service != admin-web ]] || had_web=true
  elif [[ $service != admin-web ]]; then
    echo "Missing running container: $container" >&2
    exit 1
  fi
done

rollback() {
  echo "Deployment failed; restoring application images tagged $rollback_tag." >&2
  trap - ERR
  set +e
  export PEZMAX_RELEASE_SHA=$rollback_tag
  docker compose --profile app --profile edge up -d --no-deps --no-build kadmin edge
  if [[ $had_web == true ]]; then
    docker compose --profile app --profile edge up -d --no-deps --no-build admin-web
  else
    docker compose --profile app stop admin-web
    docker compose --profile app rm -f admin-web
  fi
  exit 1
}
trap rollback ERR

wait_for_backend() {
  local attempt ip code
  for attempt in {1..30}; do
    ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' pezmax-go-kadmin 2>/dev/null || true)
    if [[ -n $ip ]]; then
      code=$(curl -sS -o /dev/null -w '%{http_code}' --max-time 3 \
        "http://$ip:9033/api/auth/captcha" 2>/dev/null || true)
      if [[ $code == 200 ]]; then return 0; fi
    fi
    sleep 2
  done
  echo 'Backend readiness check failed.' >&2
  return 1
}

wait_for_healthy() {
  local container=$1 attempt status
  for attempt in {1..30}; do
    status=$(docker inspect -f '{{.State.Health.Status}}' "$container" 2>/dev/null || true)
    if [[ $status == healthy ]]; then return 0; fi
    if [[ $status == unhealthy ]]; then break; fi
    sleep 2
  done
  echo "Container did not become healthy: $container" >&2
  return 1
}

docker compose --profile app up -d --no-deps --no-build kadmin
wait_for_backend
docker compose --profile app --profile edge up -d --no-deps --no-build edge admin-web
wait_for_healthy pezmax-go-edge
wait_for_healthy pezmax-go-admin-web
docker exec pezmax-go-admin-web wget -q -O /dev/null http://127.0.0.1/api/auth/captcha

printf '%s\n' "$target" > "$state_file"
trap - ERR
echo "Deployed $target successfully. Database, Redis and MinIO containers were untouched."
