#!/bin/bash
# Phase 3.2 — nightly sync of the (post-purge, small) session dirs to S3.
# Changed/deleted files are versioned into a dated backup-dir kept 7 days.
# Skips while the rolling purge runs (files are churning). WAL-mode SQLite copies
# behave like a crash-recovered DB on restore, which is fine for disaster recovery.
set -uo pipefail
LOG=/var/log/waha-backup.log
SRC=/var/lib/docker/volumes/docker_waha_data/_data/gows
if pgrep -f "waha-fix/purge.py" >/dev/null 2>&1; then
  echo "$(date -u +%FT%TZ) sessions backup SKIPPED (purge running)" >> "$LOG"; exit 0
fi
DAY=$(date -u +%Y%m%d)
echo "$(date -u +%FT%TZ) sessions backup start" >> "$LOG"
if rclone sync "$SRC" s3:automatiza-waha/sessions/current/ \
     --backup-dir "s3:automatiza-waha/sessions/versions/$DAY/" \
     --transfers 8 --checkers 16 --fast-list 2>>"$LOG"; then
  echo "$(date -u +%FT%TZ) sessions backup OK" >> "$LOG"
  rclone delete s3:automatiza-waha/sessions/versions/ --min-age 7d --rmdirs 2>>"$LOG"
else
  echo "$(date -u +%FT%TZ) sessions backup FAILED" >> "$LOG"
  python3 /root/waha-fix/lib_alert.py "sessions backup FAILED" "rclone sync of session dirs failed — check /var/log/waha-backup.log" >/dev/null 2>&1
fi
