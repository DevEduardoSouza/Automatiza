#!/usr/bin/env python3
"""Phase 1 rolling purge — stop growth + reclaim disk, one session at a time,
BIG-FIRST, fully resumable, coordinated with the anti-deletion watchdog.

Per session:  lock -> stop -> wait STOPPED -> PUT storage-off+ignore config
              -> sqlite DELETE gows_messages/label_associations/labels + VACUUM
              -> start -> wait WORKING -> unlock -> record.

Safety:
  * NEVER touches whatsmeow_* (credentials) or gows_groups (kept).
  * Space guard: skip a session's VACUUM unless free > db_size + 5 GB.
  * Writes the current session to purge.lock (with timestamp) so the watchdog
    skips it instead of fighting the intentional stop.
  * Resumable: purge_state.json records done/failed; re-runs skip 'done'.
  * On any DB error the session is restarted and marked failed (never left down).

Usage:
  python3 purge.py                 # all sessions, big-first
  python3 purge.py --only NAME     # single session (probe)
  python3 purge.py --max N         # first N sessions (big-first)
  python3 purge.py --dry-run       # plan only, no changes
"""
import json, os, sys, time, glob, sqlite3, urllib.request, argparse

ENV = "/opt/automatiza/docker/.env"
BASE = "http://127.0.0.1:3001"
GOWS = "/var/lib/docker/volumes/docker_waha_data/_data/gows"
STATE = "/root/waha-fix/purge_state.json"
LOCK = "/root/waha-fix/purge.lock"
LOG = "/var/log/waha-purge.log"
PURGE_TABLES = ("gows_messages", "gows_label_associations", "gows_labels")
MIN_FREE_MARGIN_GB = 5

sys.path.insert(0, "/root/waha-fix")
try:
    import lib_alert
except Exception:
    lib_alert = None


def log(msg):
    line = f"{time.strftime('%FT%TZ', time.gmtime())} {msg}"
    try:
        with open(LOG, "a") as f:
            f.write(line + "\n")
    except Exception:
        pass
    print(line, flush=True)


def api_key():
    with open(ENV) as f:
        for line in f:
            if line.startswith("WAHA_API_KEY="):
                return line.strip().split("=", 1)[1]
    return ""


KEY = api_key()


def req(method, path, body=None, timeout=45):
    data = json.dumps(body).encode() if body is not None else None
    headers = {"X-Api-Key": KEY}
    if data:
        headers["Content-Type"] = "application/json"
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=headers)
    with urllib.request.urlopen(r, timeout=timeout) as resp:
        return resp.status, resp.read()


def get_session(name):
    _, b = req("GET", f"/api/sessions/{name}", timeout=30)
    return json.loads(b)


def wait_status(name, want, timeout=90):
    end = time.time() + timeout
    while time.time() < end:
        try:
            s = get_session(name)
            st = s.get("status")
            if isinstance(want, (list, tuple)):
                if st in want:
                    return st
            elif st == want:
                return st
        except Exception:
            pass
        time.sleep(2)
    try:
        return get_session(name).get("status")
    except Exception:
        return "UNKNOWN"


def free_gb():
    s = os.statvfs("/")
    return s.f_bavail * s.f_frsize / (1024**3)


def dir_size_mb(name):
    total = 0
    for f in glob.glob(os.path.join(GOWS, name, "*")):
        try:
            total += os.path.getsize(f)
        except Exception:
            pass
    return total / (1024**2)


def set_lock(name):
    try:
        with open(LOCK, "w") as f:
            f.write(f"{name}\t{int(time.time())}\n")
    except Exception as e:
        log(f"lock write fail: {e}")


def clear_lock():
    try:
        open(LOCK, "w").close()
    except Exception:
        pass


def load_state():
    try:
        return json.load(open(STATE))
    except Exception:
        return {}


def save_state(s):
    tmp = STATE + ".tmp"
    json.dump(s, open(tmp, "w"))
    os.replace(tmp, STATE)


def apply_config(name):
    """GET current config, merge storage-off + ignore, PUT back (preserves webhooks)."""
    s = get_session(name)
    cfg = s.get("config") or {}
    cfg["gows"] = {"storage": {"messages": False, "chats": False, "labels": False, "groups": True}}
    ig = cfg.get("ignore") or {}
    ig["status"] = True
    ig["channels"] = True
    cfg["ignore"] = ig
    req("PUT", f"/api/sessions/{name}", body={"config": cfg}, timeout=45)


def vacuum_db(name):
    """DELETE purge tables + VACUUM the session's gows.db. Returns (before_mb, after_mb)."""
    db = os.path.join(GOWS, name, "gows.db")
    if not os.path.exists(db):
        raise FileNotFoundError(db)
    before = dir_size_mb(name)
    con = sqlite3.connect(db, timeout=180)
    try:
        cur = con.cursor()
        cur.execute("PRAGMA busy_timeout=180000")
        cur.execute("SELECT name FROM sqlite_master WHERE type='table'")
        existing = {r[0] for r in cur.fetchall()}
        try:
            cur.execute("PRAGMA wal_checkpoint(TRUNCATE)")
        except Exception:
            pass
        for t in PURGE_TABLES:
            if t in existing:
                cur.execute(f"DELETE FROM {t}")
        con.commit()
        try:
            cur.execute("PRAGMA wal_checkpoint(TRUNCATE)")
            con.commit()
        except Exception:
            pass
        con.isolation_level = None
        cur.execute("VACUUM")
        con.isolation_level = ""
    finally:
        con.close()
    after = dir_size_mb(name)
    return before, after


def purge_one(name, dry=False):
    db_mb = dir_size_mb(name)
    fg = free_gb()
    need = db_mb / 1024 + MIN_FREE_MARGIN_GB
    log(f">>> {name} size={db_mb:.0f}MB free={fg:.1f}GB need>{need:.1f}GB")
    if fg < need:
        log(f"!!! SPACE GUARD: free {fg:.1f}GB < need {need:.1f}GB — ABORT run")
        if lib_alert:
            lib_alert.send("purge space guard tripped",
                           f"free {fg:.1f}GB < need {need:.1f}GB before {name}; purge aborted.",
                           dedup_key="purge_guard", min_interval=3600)
        return "abort", 0
    if dry:
        return "dry", 0
    set_lock(name)
    try:
        # stop
        req("POST", f"/api/sessions/{name}/stop", timeout=45)
        st = wait_status(name, "STOPPED", timeout=60)
        if st != "STOPPED":
            log(f"    {name} did not STOP (status={st}); restarting, mark failed")
            try:
                req("POST", f"/api/sessions/{name}/start", timeout=45)
            except Exception:
                pass
            return "failed", 0
        time.sleep(2)  # let WAHA fully release the DB file
        # config off (session is stopped -> no restart race)
        try:
            apply_config(name)
        except Exception as e:
            log(f"    {name} config PUT warn: {e}")
        # delete + vacuum
        try:
            before, after = vacuum_db(name)
            freed = before - after
            log(f"    {name} vacuum: {before:.0f}MB -> {after:.0f}MB (freed {freed:.0f}MB)")
        except Exception as e:
            log(f"    {name} VACUUM ERROR: {e} — restarting session, mark failed")
            try:
                req("POST", f"/api/sessions/{name}/start", timeout=45)
            except Exception:
                pass
            return "failed", 0
        # start
        req("POST", f"/api/sessions/{name}/start", timeout=45)
        wst = wait_status(name, ("WORKING",), timeout=90)
        if wst != "WORKING":
            log(f"    {name} not WORKING yet (status={wst}); watchdog will follow up")
        return "done", freed
    finally:
        clear_lock()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--only")
    ap.add_argument("--max", type=int)
    ap.add_argument("--dry-run", action="store_true")
    a = ap.parse_args()
    if not KEY:
        log("ERROR no api key"); return

    # registered session names (never touch a dir with no live session -> would 404)
    try:
        _, b = req("GET", "/api/sessions?all=true", timeout=60)
        api_names = {s["name"] for s in json.loads(b)}
    except Exception as e:
        log(f"ERROR fetching session list: {e}"); return

    # build big-first ordered list of session dirs that have a gows.db AND are registered
    dirs = []
    for d in glob.glob(os.path.join(GOWS, "*")):
        n = os.path.basename(d)
        if os.path.isdir(d) and os.path.exists(os.path.join(d, "gows.db")):
            if n in api_names:
                dirs.append(n)
            else:
                log(f"SKIP orphan dir (not in API): {n}")
    dirs.sort(key=lambda n: dir_size_mb(n), reverse=True)

    state = load_state()
    if a.only:
        targets = [a.only]
    else:
        targets = [n for n in dirs if state.get(n, {}).get("status") != "done"]
        if a.max:
            targets = targets[: a.max]

    log(f"==== PURGE start: {len(targets)} target(s), free={free_gb():.1f}GB, dry={a.dry_run} ====")
    total_freed = 0
    done = failed = 0
    for i, name in enumerate(targets, 1):
        log(f"[{i}/{len(targets)}]")
        try:
            status, freed = purge_one(name, dry=a.dry_run)
        except Exception as e:
            log(f"    {name} UNEXPECTED {e}")
            status, freed = "failed", 0
            clear_lock()
        if status == "abort":
            break
        if status not in ("dry",):
            state[name] = {"status": status, "freed_mb": round(freed), "ts": int(time.time())}
            save_state(state)
        if status == "done":
            done += 1; total_freed += freed
        elif status == "failed":
            failed += 1
        time.sleep(1)  # gentle pacing between sessions
    log(f"==== PURGE run done: {done} purged, {failed} failed, freed {total_freed/1024:.1f}GB, free now {free_gb():.1f}GB ====")
    if not a.dry_run and not a.only and lib_alert and (done or failed):
        lib_alert.send("purge run finished",
                       f"{done} sessions purged, {failed} failed, freed {total_freed/1024:.1f}GB. "
                       f"Free now {free_gb():.1f}GB.", dedup_key="purge_done", min_interval=1800)


if __name__ == "__main__":
    main()
