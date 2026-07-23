#!/bin/sh
set -eu

if [ "${PGBACKREST_ENABLED:-false}" = "true" ] && [ "${1:-}" = "postgres" ]; then
  stanza=${PGBACKREST_STANZA:-automatiza}
  case "$stanza" in
    *[!a-zA-Z0-9_-]*|'')
      echo "invalid PGBACKREST_STANZA" >&2
      exit 64
      ;;
  esac
  set -- "$@" \
    -c archive_mode=on \
    -c archive_timeout=60s \
    -c "archive_command=pgbackrest --stanza=${stanza} archive-push %p"
fi

exec /usr/local/bin/docker-entrypoint.sh "$@"
