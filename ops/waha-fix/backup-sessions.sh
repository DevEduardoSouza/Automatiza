#!/bin/sh
# Compatibility entrypoint. Both shards are independent backup units.
set -eu

BACKUPCTL=${BACKUPCTL:-/opt/automatiza/backup/backupctl.py}
BACKUP_CONFIG=${BACKUP_CONFIG:-/etc/automatiza-backup/config.env}

python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" backup waha-1 --execute
exec python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" backup waha-2 --execute
