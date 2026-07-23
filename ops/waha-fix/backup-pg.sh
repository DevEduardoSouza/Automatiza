#!/bin/sh
# Compatibility entrypoint. Retention is enforced by the immutable repository,
# never by this script.
set -eu

BACKUPCTL=${BACKUPCTL:-/opt/automatiza/backup/backupctl.py}
BACKUP_CONFIG=${BACKUP_CONFIG:-/etc/automatiza-backup/config.env}

exec python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" backup postgres-logical --execute
