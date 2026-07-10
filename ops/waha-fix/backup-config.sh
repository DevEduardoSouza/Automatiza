#!/bin/bash
# Phase 3.2 — nightly encrypted backup of compose/.env/scripts/crons to S3.
# Encrypted with age (public key in age-recipient.txt); only the off-server age
# PRIVATE key can decrypt. Retains 30 days.
set -uo pipefail
LOG=/var/log/waha-backup.log
TS=$(date -u +%Y%m%dT%H%M%SZ)
PUB=$(cat /root/waha-fix/age-recipient.txt 2>/dev/null)
if [ -z "$PUB" ]; then echo "$(date -u +%FT%TZ) config backup: no age recipient" >>"$LOG"; exit 1; fi
TMP=$(mktemp); TMPA=$(mktemp)
tar czf "$TMP" -C / \
  opt/automatiza/docker/docker-compose.yml \
  opt/automatiza/docker/.env \
  root/waha-fix \
  $(cd / && ls etc/cron.d/waha-* etc/cron.d/automatiza-* 2>/dev/null) \
  2>/dev/null
if age -r "$PUB" -o "$TMPA" "$TMP" \
   && rclone rcat "s3:automatiza-waha/config/config-${TS}.tar.gz.age" < "$TMPA" 2>>"$LOG"; then
  echo "$(date -u +%FT%TZ) config backup OK -> config/config-${TS}.tar.gz.age" >> "$LOG"
  rclone delete s3:automatiza-waha/config/ --min-age 30d 2>>"$LOG"
else
  echo "$(date -u +%FT%TZ) config backup FAILED" >> "$LOG"
  python3 /root/waha-fix/lib_alert.py "config backup FAILED" "age/upload failed at ${TS}" >/dev/null 2>&1
fi
rm -f "$TMP" "$TMPA"
