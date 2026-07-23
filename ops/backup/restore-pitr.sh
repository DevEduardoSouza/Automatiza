#!/bin/sh
# Human-operated PITR drill. Creates a new, isolated Compose project and volume.
set -eu

: "${PITR_DRILL_ID:?set a unique lowercase drill identifier}"
: "${PITR_TARGET:?set the approved UTC recovery target, for example 2026-07-23 03:00:00+00}"
: "${PGBACKREST_RESTORE_IMAGE:?pin the approved pgBackRest image digest}"

case "$PITR_DRILL_ID" in
  *[!a-z0-9-]*|'')
    echo "PITR_DRILL_ID must contain only lowercase letters, digits and hyphens" >&2
    exit 64
    ;;
esac

RESTORE_SENTINEL=${RESTORE_ENABLE_SENTINEL:-/etc/automatiza-backup/RESTORE_ALLOWED}
if [ ! -f "$RESTORE_SENTINEL" ] || [ "$(tr -d '\n' < "$RESTORE_SENTINEL")" != "AUTOMATIZA_STAGING_RESTORE_V1" ]; then
  echo "staging restore sentinel is absent or invalid" >&2
  exit 64
fi

RESTORE_COMPOSE=${RESTORE_PITR_COMPOSE:-/opt/automatiza/docker/docker-compose.restore-pitr.yml}
PGBACKREST_STANZA=${PGBACKREST_STANZA:-automatiza}
project="automatiza-pitr-${PITR_DRILL_ID}"
volume="${project}_restore_postgres_data"

if docker volume inspect "$volume" >/dev/null 2>&1; then
  echo "refusing to reuse non-empty restore volume: $volume" >&2
  exit 65
fi

docker compose --project-name "$project" -f "$RESTORE_COMPOSE" run --rm \
  pgbackrest-restore \
  "--stanza=$PGBACKREST_STANZA" \
  --type=time \
  "--target=$PITR_TARGET" \
  --target-action=promote \
  restore

docker compose --project-name "$project" -f "$RESTORE_COMPOSE" up -d --wait postgres-restored
docker compose --project-name "$project" -f "$RESTORE_COMPOSE" exec -T postgres-restored \
  psql -U "${RESTORE_DB_USER:-automatiza}" -d "${RESTORE_DB_NAME:-automatiza}" -tA \
  -c "SELECT (to_regclass('public.profiles') IS NOT NULL AND to_regclass('public.sessions') IS NOT NULL)::int"

echo "PITR structural smoke completed for project $project."
echo "Do not remove its volume before evidence and recovery timestamps are reviewed."
