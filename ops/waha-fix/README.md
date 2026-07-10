# ops/waha-fix — WAHA fleet operational toolkit

Source-of-truth copy of the operational tooling that keeps the WAHA (WhatsApp)
fleet healthy on the VPS. These scripts were built during the July 2026
stabilization (disk faxina, anti-deletion, session sharding, S3 media + backups).

> **These run via `cron` on the VPS at `/root/waha-fix/`, NOT through the Docker
> deploy pipeline.** The `Deploy` GitHub Action only manages the `docker/` compose
> stack. To update the live copies, `scp` the changed file to
> `root@<vps>:/root/waha-fix/`. This directory is the versioned reference so the
> code no longer lives only on the box.

## Scripts

| File | Cron (`/etc/cron.d/`) | What it does |
|------|-----------------------|--------------|
| `monitor.py` | `waha-monitor` — `*/5 * * * *` | Superset of the old watchdog: restarts **paired** sessions found `FAILED`/`STOPPED` across `waha-1`/`waha-2` (before the backend can delete them), zombie detection, and email alerts on disk/load/health + router-block events. |
| `guard.py` | `waha-guard` — `47 * * * *` | Hourly: forces GOWS on-disk storage **off** for any newly created session (keeps media in S3, disk low). Self-skips while a purge runs. |
| `watchdog.py` | *(retired)* | Predecessor of `monitor.py` (anti-deletion only). Kept for reference; **replaced by `monitor.py`** — do not re-enable both. |
| `purge.py` | *(one-shot)* | Rolling purge of GOWS media/history that had filled the disk (reclaimed ~330 GB). A remediation tool, not a cron job. |
| `reconcile.py` | *(manual)* | Read-only report reconciling automations ↔ WAHA instance ownership. |
| `lib_alert.py` | *(library)* | Shared email-alert helper. Reads `SMTP_*` from the docker `.env`; de-dupes via `.alert_*` timestamp files. |
| `status.sh` | *(manual)* | Quick live status of the fleet. |
| `backup-pg.sh` | `automatiza-backups` | `pg_dump` → `s3:automatiza-waha/postgres/` (14-day retention). |
| `backup-sessions.sh` | `automatiza-backups` | `rclone sync` of session dirs → `s3:.../sessions/current/` + versioned `sessions/versions/<day>/` (7-day retention). |
| `backup-config.sh` | `automatiza-backups` | `.env` + compose tarball, **age-encrypted**, → `s3:.../config/` (30-day retention). |

## S3 layout (`rclone` remote `s3:` → bucket `automatiza-waha`, region `eu-north-1`)

```
automatiza-waha/
├── <waha media>            # WAHA runtime media (WAHA_MEDIA_STORAGE=S3)
├── postgres/               # pg_dump backups (14d)
├── sessions/current/       # latest session dirs (synced)
├── sessions/versions/<day>/# rolled-off session versions (7d)
└── config/                 # age-encrypted .env + compose (30d)
```

## Env vars (ops-managed — live only in the VPS `docker/.env`)

These are **not** GitHub secrets. The `Deploy` workflow preserves them across
deploys (merges the existing values back into `.env`), so they never enter the
repo or CI. Referenced by the `waha`/`waha-2` services in
`docker/docker-compose.prod.yml`.

```env
# WAHA media storage → S3 (keeps the shared disk empty)
WAHA_MEDIA_STORAGE=S3
WAHA_S3_REGION=eu-north-1
WAHA_S3_BUCKET=automatiza-waha
WAHA_S3_ACCESS_KEY_ID=<aws-iam-access-key>
WAHA_S3_SECRET_ACCESS_KEY=<aws-iam-secret>
WAHA_S3_PROXY_FILES=True

# Per-session outbound proxy pool for NEW pairings (waha-router injects it
# round-robin). Format: "host:port;user;pass" comma-separated. Empty/commented
# = disabled (current state). See waha-router service + main.go injectProxy().
# WAHA_SESSION_PROXIES=
```

## waha-router

The `waha-router` service (`docker/waha-router/`) sits in front of `waha-1` +
`waha-2` and: (1) routes each request to the instance owning the session and
merges the session list; (2) **blocks** `DELETE`/`logout` of a paired session
unless the caller sends `X-Confirm-Unpair: true` (structural anti-destruction);
(3) optionally injects a round-robin `config.proxy` into new-session creation
(`WAHA_SESSION_PROXIES`). Backend points `WAHA_API_URL` at it. Emergency bypass:
set `WAHA_API_URL=http://waha:3000` on `server`+`worker` and recreate them.

## Off-server secret

`backup-config.sh` encrypts with **age**. The matching **private** key
(`/root/waha-fix/age-key.txt` on the VPS) is the only thing that can decrypt the
config backups — keep a copy **off the server**. It is deliberately not in this
repo.
