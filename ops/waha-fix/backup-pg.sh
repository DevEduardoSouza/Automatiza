#!/bin/bash
# Phase 3.2 — nightly Postgres backup to S3 (custom format so pg_restore --list works).
# Streams straight to S3 (no local temp — disk is precious). Retains 14 days.
set -uo pipefail
LOG=/var/log/waha-backup.log
TS=$(date -u +%Y%m%dT%H%M%SZ)
echo "$(date -u +%FT%TZ) pg backup start" >> "$LOG"
if docker exec -i automatiza-postgres pg_dump -U automatiza -d automatiza -Fc \
     | rclone rcat "s3:automatiza-waha/postgres/automatiza-${TS}.dump" 2>>"$LOG"; then
  echo "$(date -u +%FT%TZ) pg backup OK -> postgres/automatiza-${TS}.dump" >> "$LOG"
  rclone delete s3:automatiza-waha/postgres/ --min-age 14d 2>>"$LOG"
else
  echo "$(date -u +%FT%TZ) pg backup FAILED" >> "$LOG"
  python3 /root/waha-fix/lib_alert.py "pg backup FAILED" "pg_dump/upload failed at ${TS} — check /var/log/waha-backup.log" >/dev/null 2>&1
fi
