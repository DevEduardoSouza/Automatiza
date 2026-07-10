#!/usr/bin/env python3
"""Phase 1.4 guard — hourly, force gows storage OFF + ignore on any session whose
config still has local message storage enabled (i.e. sessions created after the
purge that would otherwise start regrowing the 344 GB of gows_messages).

Only touches sessions that actually need it, paces the PUTs, and skips any session
the rolling purge is currently working on. Safe to run by hand."""
import json, sys, time, subprocess, urllib.request

ENV = "/opt/automatiza/docker/.env"
BASE = "http://127.0.0.1:3001"
LOCK = "/root/waha-fix/purge.lock"
LOG = "/var/log/waha-guard.log"
MAX_PER_RUN = 10  # new sessions are rare; more than this signals a problem, not routine

sys.path.insert(0, "/root/waha-fix")
try:
    import lib_alert
except Exception:
    lib_alert = None


def purge_running():
    try:
        return subprocess.run(["pgrep", "-f", "waha-fix/purge.py"],
                              capture_output=True).returncode == 0
    except Exception:
        return False


def log(msg):
    line = f"{time.strftime('%FT%TZ', time.gmtime())} {msg}"
    try:
        open(LOG, "a").write(line + "\n")
    except Exception:
        pass
    print(line)


def api_key():
    with open(ENV) as f:
        for line in f:
            if line.startswith("WAHA_API_KEY="):
                return line.strip().split("=", 1)[1]
    return ""


KEY = api_key()


def req(method, path, body=None, timeout=45):
    data = json.dumps(body).encode() if body is not None else None
    h = {"X-Api-Key": KEY}
    if data:
        h["Content-Type"] = "application/json"
    r = urllib.request.Request(BASE + path, data=data, method=method, headers=h)
    with urllib.request.urlopen(r, timeout=timeout) as resp:
        return resp.status, resp.read()


def locked():
    out = set(); now = time.time()
    try:
        for l in open(LOCK):
            l = l.strip()
            if not l:
                continue
            p = l.split("\t"); ts = float(p[1]) if len(p) > 1 and p[1] else now
            if now - ts < 1800:
                out.add(p[0])
    except Exception:
        pass
    return out


def storage_off(cfg):
    st = ((cfg or {}).get("gows") or {}).get("storage") or {}
    return st.get("messages") is False


def main():
    if not KEY:
        log("ERROR no api key"); return
    # Never run while the rolling purge is active: not-yet-purged sessions still
    # have storage ON, and mass-PUTting them would be exactly the herd effect the
    # purge avoids. The purge itself sets storage-off on every session it touches.
    if purge_running():
        log("purge running — guard skipped this cycle")
        return
    try:
        _, b = req("GET", "/api/sessions?all=true", timeout=60)
        sessions = json.loads(b)
    except Exception as e:
        log(f"ERROR fetch: {e}"); return
    lk = locked()
    need = [s for s in sessions if not storage_off(s.get("config") or {}) and s.get("name") not in lk]
    if len(need) > MAX_PER_RUN:
        log(f"WARNING {len(need)} sessions need storage-off (>cap {MAX_PER_RUN}); fixing {MAX_PER_RUN}, alerting")
        if lib_alert:
            lib_alert.send("guard: many sessions with storage ON",
                           f"{len(need)} sessions still have gows storage enabled — the purge may "
                           f"not have covered them. Fixing {MAX_PER_RUN}/run to avoid a herd restart.",
                           dedup_key="guard_many", min_interval=3600)
    fixed = 0
    for s in need[:MAX_PER_RUN]:
        name = s.get("name")
        cfg = s.get("config") or {}
        cfg["gows"] = {"storage": {"messages": False, "chats": False, "labels": False, "groups": True}}
        ig = cfg.get("ignore") or {}
        ig["status"] = True; ig["channels"] = True
        cfg["ignore"] = ig
        try:
            req("PUT", f"/api/sessions/{name}", body={"config": cfg}, timeout=45)
            log(f"storage-off applied to new/changed session {name} (was {s.get('status')})")
            fixed += 1
            time.sleep(2)  # pace
        except Exception as e:
            log(f"PUT fail {name}: {e}")
    if fixed:
        log(f"guard: fixed {fixed} session(s)")


if __name__ == "__main__":
    main()
