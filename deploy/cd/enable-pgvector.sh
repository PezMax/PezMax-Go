#!/usr/bin/env bash
# Enable pgvector on an existing PezMax PostgreSQL container.
#
# The Compose files now use pgvector/pgvector:pg16. That only takes effect for a
# new data volume: init scripts do not run again on an existing volume. This
# script upgrades the running database in place, without touching the volume:
#
#   1. checks whether pgvector is already available / installed;
#   2. pulls the pgvector image and dumps the application database;
#   3. recreates only the postgres container with the new image;
#   4. runs CREATE EXTENSION vector / pg_trgm and verifies HNSW support;
#   5. restores the previous image automatically if any step fails.
#
# The original PostgreSQL 16 data directory is reused as-is (same major
# version), so no dump/restore cycle is needed. Keep the dump as a safety net.
#
# Usage:
#   sudo deploy/cd/enable-pgvector.sh [--dry-run] [--check] [--skip-backup]
#
# Environment:
#   PEZMAX_DB_CONTAINER   PostgreSQL container name (default pezmax-go-postgres)
#   PEZMAX_DB_IMAGE       Target image (default pgvector/pgvector:pg16)
#   PEZMAX_DB_BACKUP_DIR  Dump directory (default /var/backups/pezmax)

set -euo pipefail
umask 077

container=${PEZMAX_DB_CONTAINER:-pezmax-go-postgres}
image=${PEZMAX_DB_IMAGE:-pgvector/pgvector:pg16}
backup_dir=${PEZMAX_DB_BACKUP_DIR:-/var/backups/pezmax}

dry_run=false
check_only=false
skip_backup=false

usage() {
  awk 'NR > 1 && /^#/ { sub(/^# ?/, ""); print; next } NR > 1 { exit }' "$0"
}

while (($#)); do
  case $1 in
    --dry-run) dry_run=true ;;
    --check) check_only=true ;;
    --skip-backup) skip_backup=true ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
  shift
done

if [[ $EUID -ne 0 ]]; then
  echo 'Run as root so Docker can recreate the PostgreSQL container.' >&2
  exit 1
fi

if ! docker inspect "$container" >/dev/null 2>&1; then
  echo "PostgreSQL container not found: $container" >&2
  exit 1
fi
if [[ $(docker inspect -f '{{.State.Running}}' "$container") != true ]]; then
  echo "PostgreSQL container is not running: $container" >&2
  exit 1
fi

db_exec() {
  docker exec "$container" sh -c \
    'exec psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"' sh "$@"
}

vector_version() {
  db_exec -tAc "SELECT extversion FROM pg_extension WHERE extname = 'vector'"
}

vector_available() {
  [[ $(db_exec -tAc "SELECT 1 FROM pg_available_extensions WHERE name = 'vector'") == 1 ]]
}

trgm_installed() {
  [[ $(db_exec -tAc "SELECT 1 FROM pg_extension WHERE extname = 'pg_trgm'") == 1 ]]
}

enable_extensions() {
  db_exec -c 'CREATE EXTENSION IF NOT EXISTS vector' \
          -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm'
  db_exec -c 'CREATE TEMP TABLE _pgvector_smoke (embedding vector(3));
              CREATE INDEX ON _pgvector_smoke USING hnsw (embedding vector_cosine_ops);
              INSERT INTO _pgvector_smoke VALUES ('"'"'[1,2,3]'"'"');
              DROP TABLE _pgvector_smoke;'
  echo "pgvector $(vector_version) enabled in $container."
}

installed_version=$(vector_version)
if [[ -n $installed_version ]]; then
  if vector_available; then
    if trgm_installed; then
      echo "pgvector $installed_version and pg_trgm are already enabled in $container."
      exit 0
    fi
    # pg_trgm ships with the same image and only needs CREATE EXTENSION.
    if [[ $check_only == true ]]; then
      echo "pg_trgm is NOT enabled in $container." >&2
      exit 1
    fi
    if [[ $dry_run == true ]]; then
      echo "Would create the pg_trgm extension in $container."
      exit 0
    fi
    db_exec -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm'
    echo "pg_trgm enabled in $container (pgvector $installed_version)."
    exit 0
  fi
  echo "pgvector $installed_version is recorded in the database, but the running image does not provide it." >&2
  if [[ $check_only == true ]]; then
    exit 1
  fi
elif [[ $check_only == true ]]; then
  echo "pgvector is NOT enabled in $container." >&2
  exit 1
fi

# The extension is present in the image but was never created: no restart
# needed, just install it into the current database.
if vector_available; then
  if [[ $dry_run == true ]]; then
    echo "Would create vector and pg_trgm extensions in $container."
    exit 0
  fi
  enable_extensions
  exit 0
fi

# The current image lacks the extension. Resolve the exact Compose invocation
# from container labels so an existing deployment can be upgraded in place
# without guessing paths.
service=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.service"}}' "$container")
project=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project"}}' "$container")
working_dir=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.working_dir"}}' "$container")
config_files=$(docker inspect -f '{{index .Config.Labels "com.docker.compose.project.config_files"}}' "$container")

if [[ -z $service || -z $config_files ]]; then
  echo "Cannot determine the Compose service for $container." >&2
  echo 'Recreate it manually with the pgvector image, then run:' >&2
  echo '  psql -c "CREATE EXTENSION IF NOT EXISTS vector" -c "CREATE EXTENSION IF NOT EXISTS pg_trgm"' >&2
  exit 1
fi

compose=(docker compose)
[[ -n $project ]] && compose+=(--project-name "$project")
[[ -n $working_dir ]] && compose+=(--project-directory "$working_dir")
[[ -n $working_dir && -f $working_dir/.env ]] && compose+=(--env-file "$working_dir/.env")
IFS=',' read -r -a compose_files <<< "$config_files"
for file in "${compose_files[@]}"; do
  file=${file#"${file%%[![:space:]]*}"}
  file=${file%"${file##*[![:space:]]}"}
  [[ -z $file ]] && continue
  if [[ ! -f $file ]]; then
    echo "Skipping compose file missing from labels: $file" >&2
    continue
  fi
  compose+=(-f "$file")
done

current_image=$(docker inspect -f '{{.Config.Image}}' "$container")
old_image_id=$(docker inspect -f '{{.Image}}' "$container")
echo "Container $container uses $current_image; target image: $image."

if [[ $dry_run == true ]]; then
  echo 'Would:'
  echo "  1. docker pull $image"
  [[ $skip_backup == true ]] || echo "  2. pg_dump the application database to $backup_dir"
  echo "  3. recreate only the $service service with the target image"
  echo '  4. CREATE EXTENSION vector / pg_trgm and verify HNSW'
  exit 0
fi

# Pull first: if the registry is unreachable the running container is untouched.
for attempt in 1 2 3; do
  if docker pull "$image"; then
    break
  fi
  if ((attempt == 3)); then
    echo "Failed to pull $image; PostgreSQL was not modified." >&2
    exit 1
  fi
  echo "Pull failed (attempt $attempt/3); retrying."
  sleep 5
done

if [[ $skip_backup != true ]]; then
  mkdir -p "$backup_dir"
  db_name=$(docker exec "$container" sh -c 'printf %s "$POSTGRES_DB"')
  backup_file=$backup_dir/${db_name:-postgres}-$(date +%Y%m%d-%H%M%S).dump
  echo "Backing up database $db_name to $backup_file."
  docker exec "$container" sh -c \
    'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$backup_file"
  if [[ ! -s $backup_file ]]; then
    echo "Backup is empty: $backup_file. Aborting." >&2
    exit 1
  fi
fi

override=$(mktemp "${TMPDIR:-/tmp}/pezmax-pgvector.XXXXXX.yml")
restore_old_image() {
  local status=$?
  if ((status == 0)); then return 0; fi
  echo "pgvector upgrade failed; restoring the previous image $old_image_id." >&2
  printf 'services:\n  %s:\n    image: %s\n' "$service" "$old_image_id" > "$override"
  "${compose[@]}" -f "$override" up -d --no-deps "$service" >/dev/null 2>&1 || true
  rm -f "$override"
  exit "$status"
}
trap restore_old_image EXIT

printf 'services:\n  %s:\n    image: %s\n' "$service" "$image" > "$override"
"${compose[@]}" -f "$override" up -d --no-deps "$service"

for attempt in {1..30}; do
  health=$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' "$container" 2>/dev/null || true)
  case $health in
    healthy) break ;;
    unhealthy) echo 'PostgreSQL became unhealthy after the image swap.' >&2; exit 1 ;;
    none)
      if docker exec "$container" sh -c 'exec pg_isready -q -U "$POSTGRES_USER" -d "$POSTGRES_DB"'; then break; fi
      ;;
  esac
  if ((attempt == 30)); then
    echo 'PostgreSQL did not become ready after the image swap.' >&2
    exit 1
  fi
  sleep 2
done

enable_extensions
rm -f "$override"
trap - EXIT
echo "Upgrade complete: $container now runs $image."
echo "Restart application containers if they keep stale connections (the Go pool normally reconnects)."
