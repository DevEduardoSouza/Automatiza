#!/bin/sh
# Human-operated staging drill. It never targets the production compose project.
set -eu

: "${POSTGRES_LOGICAL_ARTIFACT:?set the reviewed postgres-logical artifact key}"
: "${WAHA1_ARTIFACT:?set the reviewed waha-1 artifact key}"
: "${WAHA2_ARTIFACT:?set the reviewed waha-2 artifact key}"
: "${POSTGRES_RESTORE_IMAGE:?pin the approved postgres@sha256 image}"

BACKUPCTL=${BACKUPCTL:-/opt/automatiza/backup/backupctl.py}
BACKUP_CONFIG=${BACKUP_CONFIG:-/etc/automatiza-backup/config.env}
RESTORE_ROOT=${RESTORE_ROOT:-/var/lib/automatiza-restore-drill}
RESTORE_COMPOSE=${RESTORE_COMPOSE:-/opt/automatiza/docker/docker-compose.restore.yml}
RESTORE_PROJECT=automatiza-restore-drill

case "$RESTORE_COMPOSE" in
  *docker-compose.prod.yml)
    echo "refusing production compose file" >&2
    exit 64
    ;;
esac

services=$(docker compose --project-name "$RESTORE_PROJECT" -f "$RESTORE_COMPOSE" config --services)
if [ "$services" != "postgres-restore" ]; then
  echo "restore compose must contain only postgres-restore" >&2
  exit 64
fi
if docker compose --project-name "$RESTORE_PROJECT" -f "$RESTORE_COMPOSE" ps -q | grep -q .; then
  echo "refusing to reuse an existing restore Compose project" >&2
  exit 65
fi
if docker volume inspect "${RESTORE_PROJECT}_restore_postgres_data" >/dev/null 2>&1; then
  echo "refusing to reuse an existing restore volume" >&2
  exit 65
fi

docker compose --project-name "$RESTORE_PROJECT" -f "$RESTORE_COMPOSE" up -d --wait

python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" restore-postgres-logical \
  --artifact "$POSTGRES_LOGICAL_ARTIFACT" \
  --target "$RESTORE_ROOT/postgres" \
  --execute
python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" restore-archive \
  --artifact "$WAHA1_ARTIFACT" \
  --target "$RESTORE_ROOT/waha-1" \
  --execute
python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" restore-archive \
  --artifact "$WAHA2_ARTIFACT" \
  --target "$RESTORE_ROOT/waha-2" \
  --execute

test "$(find "$RESTORE_ROOT/waha-1" -type f -print -quit)"
test "$(find "$RESTORE_ROOT/waha-2" -type f -print -quit)"

echo "Restore artifacts passed structural smoke tests."
echo "Operator must now run application login and disposable WAHA session checks."
echo "Do not clean up until evidence has been captured and approved."
