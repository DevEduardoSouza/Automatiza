#!/usr/bin/env python3
"""WAHA anti-deletion watchdog  ---  THE most urgent fix.

The backend runs an hourly (HH:21) job "cleaning up FAILED WAHA sessions" that
DELETEs (unpairs) any session sitting in FAILED. Unpair is IRREVERSIBLE. Live
evidence: 24 "Unpairing the device" + 0 real WhatsApp logouts in a single hour.

This watchdog runs every 5 min and restarts any PAIRED (me != null) session that
is FAILED or STOPPED, so no paired session is ever still FAILED when the HH:21
cleanup fires. It NEVER gives up (progressive backoff, no destructive cap) and it
NEVER deletes/unpairs anything. It coordinates with the rolling purge via a lock
file so it does not fight the purge's intentional stops.

Idempotent, safe to run by hand: `python3 /root/waha-fix/watchdog.py`."""
import json, os, sys, time, subprocess, urllib.request

ENV = "/opt/automatiza/docker/.env"
BASE = "http://127.0.0.1:3001"
STATE = "/root/waha-fix/watchdog_state.json"
ESTABLISHED = "/root/waha-fix/established.json"
PURGE_LOCK = "/root/waha-fix/purge.lock"
LOG = "/var/log/waha-watchdog.log"
ESTABLISHED_MIN_CYCLES = 3   # seen paired in >=3 cycles (~15 min) before it counts as a real number
import re

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
    print(line)


def api_key():
    try:
        with open(ENV) as f:
            for line in f:
                if line.startswith("WAHA_API_KEY="):
                    return line.strip().split("=", 1)[1]
    except Exception:
        pass
    return ""


KEY = api_key()


def req(method, path, timeout=30):
    r = urllib.request.Request(BASE + path, method=method, headers={"X-Api-Key": KEY})
    with urllib.request.urlopen(r, timeout=timeout) as resp:
        return resp.status, resp.read()


def get_sessions():
    _, body = req("GET", "/api/sessions?all=true", timeout=60)
    return json.loads(body)


def restart(name):
    st, _ = req("POST", f"/api/sessions/{name}/restart", timeout=45)
    return st


def load_state():
    try:
        return json.load(open(STATE))
    except Exception:
        return {}


def save_state(s):
    try:
        json.dump(s, open(STATE, "w"))
    except Exception as e:
        log(f"state save fail: {e}")


def load_established():
    try:
        return json.load(open(ESTABLISHED))
    except Exception:
        return {}


def save_established(e):
    try:
        tmp = ESTABLISHED + ".tmp"
        json.dump(e, open(tmp, "w"))
        os.replace(tmp, ESTABLISHED)
    except Exception as ex:
        log(f"established save fail: {ex}")


def purge_locked():
    """Sessions the rolling purge is currently working on. Lock lines are
    'name\\t<epoch>'; entries older than 30 min are treated as stale and ignored
    so a crashed purge can never make the watchdog abandon a session forever."""
    out = set()
    now = time.time()
    try:
        for l in open(PURGE_LOCK):
            l = l.strip()
            if not l:
                continue
            parts = l.split("\t")
            name = parts[0]
            ts = float(parts[1]) if len(parts) > 1 and parts[1] else now
            if now - ts < 1800:
                out.add(name)
    except Exception:
        pass
    return out


def due(attempts, cycles_since):
    """Progressive backoff on a 5-min cron. Never returns 'give up'."""
    if attempts <= 3:
        return True                # every cycle (~5m) for the first attempts
    if attempts <= 6:
        return cycles_since >= 2    # ~10m
    return cycles_since >= 6        # ~30m, forever


def main():
    if not KEY:
        log("ERROR: no WAHA_API_KEY in .env")
        return
    try:
        sessions = get_sessions()
    except Exception as e:
        log(f"ERROR: cannot fetch sessions: {e}")
        if lib_alert:
            lib_alert.send("watchdog cannot reach WAHA API", str(e),
                           dedup_key="wd_api", min_interval=1800)
        return

    locked = purge_locked()
    state = load_state()
    est = load_established()
    now = time.time()
    live = {}
    acted = 0

    for s in sessions:
        name = s.get("name")
        status = s.get("status")
        paired = bool(s.get("me"))
        if not paired:
            continue
        # track establishment: real numbers are seen paired across many cycles;
        # ephemeral failed-pairing stubs are created+deleted within one cycle.
        ent = est.get(name, {"count": 0, "last": 0})
        ent["count"] = min(ent.get("count", 0) + 1, 100000)
        ent["last"] = now
        est[name] = ent
        if status == "WORKING":
            if name in state:
                log(f"RECOVERED {name} -> WORKING after {state[name].get('attempts')} attempt(s)")
            continue
        if status in ("FAILED", "STOPPED"):
            if name in locked:
                log(f"SKIP {name} status={status} (purge lock)")
                if name in state:
                    live[name] = state[name]
                continue
            prev = state.get(name, {"attempts": 0, "last": 0, "first": now})
            cycles_since = (now - prev["last"]) / 300.0 if prev["last"] else 999
            if due(prev.get("attempts", 0), cycles_since):
                try:
                    code = restart(name)
                    prev["attempts"] = prev.get("attempts", 0) + 1
                    prev["last"] = now
                    log(f"RESTART {name} status={status} attempt#{prev['attempts']} http={code}")
                    acted += 1
                    if prev["attempts"] == 5 and lib_alert:
                        lib_alert.send(
                            f"session {name} not recovering (5 failed restarts)",
                            f"Session {name} is still {status} after 5 restart attempts. "
                            f"It likely needs a manual re-pair on the SAME chip (unpair is "
                            f"irreversible). The watchdog keeps retrying with backoff and will "
                            f"NOT let the HH:21 cleanup delete it.",
                            dedup_key=f"wd_{name}", min_interval=21600)
                except Exception as e:
                    prev["attempts"] = prev.get("attempts", 0) + 1
                    prev["last"] = now
                    log(f"RESTART-FAIL {name} status={status} err={e}")
            else:
                log(f"BACKOFF {name} status={status} attempts={prev.get('attempts', 0)}")
            live[name] = prev

    save_state(live)
    # prune established entries not seen for 7 days, then persist
    est = {k: v for k, v in est.items() if now - v.get("last", 0) < 7 * 86400}
    save_established(est)

    # Tripwire: distinguish REAL loss (an ESTABLISHED number deleted) from benign
    # ephemeral churn (failed-pairing stubs created+deleted within one cycle).
    try:
        out = subprocess.run(["docker", "logs", "--since", "6m", "automatiza-waha"],
                             capture_output=True, text=True, timeout=60)
        blob = (out.stdout or "") + (out.stderr or "")
        deleted_names = set(re.findall(r'(session_[a-z0-9]+|user_[a-z0-9]+) - Session deleted', blob))
        total_deleted = blob.count("Session deleted")
        real_losses = [n for n in deleted_names
                       if est.get(n, {}).get("count", 0) >= ESTABLISHED_MIN_CYCLES]
        if total_deleted:
            log(f"deletions last 6m: {total_deleted} total, {len(deleted_names)} distinct, "
                f"{len(real_losses)} ESTABLISHED (ephemeral churn otherwise)")
        if real_losses:
            log(f"TRIPWIRE REAL LOSS: {real_losses}")
            if lib_alert:
                lib_alert.send(
                    f"TRIPWIRE: {len(real_losses)} ESTABLISHED number(s) deleted",
                    "The app deleted established (long-lived paired) sessions — this is the "
                    "irreversible destruction the plan targets:\n  " + "\n  ".join(real_losses) +
                    "\n\nThe watchdog restarts FAILED paired sessions before the HH:21 cleanup; "
                    "the router DELETE-block (Phase 5) stops it structurally. Investigate why "
                    "these reached a deletable state.",
                    dedup_key="wd_realloss", min_interval=3600)
    except Exception as e:
        log(f"tripwire check failed: {e}")

    log(f"cycle done: watching {len(live)}, {acted} restart(s), locked={len(locked)}, established={sum(1 for v in est.values() if v.get('count',0)>=ESTABLISHED_MIN_CYCLES)}")


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        log(f"FATAL {e}")
