#!/bin/sh
# Compatibility entrypoint. The allowlist deliberately excludes restore keys.
set -eu

BACKUPCTL=${BACKUPCTL:-/opt/automatiza/backup/backupctl.py}
BACKUP_CONFIG=${BACKUP_CONFIG:-/etc/automatiza-backup/config.env}

exec python3 "$BACKUPCTL" --config "$BACKUP_CONFIG" backup vps-config --execute
