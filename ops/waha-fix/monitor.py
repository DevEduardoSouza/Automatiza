#!/usr/bin/env python3
"""Phase 5.5 monitor — superset of the watchdog. Runs every 5 min via cron.

Does everything the watchdog does (restart paired FAILED/STOPPED before the
HH:21 cleanup; established-session tracking; real-loss tripwire) PLUS:
  * zombie detection: WORKING sessions whose GOWS engine is actually
    disconnected -> restart (rate-limited, circuit-breaker);
  * infra alerts: disk >85%, load >15, WAHA container unhealthy;
  * autoscale signal: any instance at >=330 sessions -> alert to add waha-N.

Multi-instance aware via INSTANCES (default single localhost:3001). Safe to run
by hand. Coordinates with the rolling purge via purge.lock like the watchdog.
When deployed, it REPLACES the waha-watchdog cron (do not run both)."""
import json, os, sys, time, subprocess, urllib.request, re

ENV = "/opt/automatiza/docker/.env"
# name -> base url. Direct instance URLs (not via router) so the monitor can
# restart FAILED/STOPPED sessions on whichever instance owns them.
INSTANCES = {"waha-1": "http://127.0.0.1:3001", "waha-2": "http://127.0.0.1:3002"}
# containers whose health the monitor watches
WATCH_CONTAINERS = ["automatiza-waha", "automatiza-waha-2", "automatiza-waha-router"]
STATE = "/root/waha-fix/watchdog_state.json"
ESTABLISHED = "/root/waha-fix/established.json"
ZOMBIE_STATE = "/root/waha-fix/zombie_state.json"
ORPHAN_STATE = "/root/waha-fix/orphan_state.json"
PURGE_LOCK = "/root/waha-fix/purge.lock"
LOG = "/var/log/waha-monitor.log"
ESTABLISHED_MIN_CYCLES = 3
ZOMBIE_SCAN_PER_CYCLE = 60      # detail-GETs per cycle (rotating) to bound cost
ZOMBIE_RESTARTS_PER_CYCLE = 5   # circuit-breaker
CAP = 367
AUTOSCALE_AT = 330
DISK_ALERT_PCT = 85
LOAD_ALERT = 15.0

sys.path.insert(0, "/root/waha-fix")
try:
    import lib_alert
except Exception:
    lib_alert = None


def log(msg):
    line = f"{time.strftime('%FT%TZ', time.gmtime())} {msg}"
    try:
        open(LOG, "a").write(line + "\n")
    except Exception:
        pass
    print(line)


def api_key():
    try:
        with open(ENV) as f:
            for l in f:
                if l.startswith("WAHA_API_KEY="):
                    return l.strip().split("=", 1)[1]
    except Exception:
        pass
    return ""


KEY = api_key()


def req(method, base, path, timeout=30):
    r = urllib.request.Request(base + path, method=method, headers={"X-Api-Key": KEY})
    with urllib.request.urlopen(r, timeout=timeout) as resp:
        return resp.status, resp.read()


def jload(p, d):
    try:
        return json.load(open(p))
    except Exception:
        return d


def jsave(p, o):
    try:
        tmp = p + ".tmp"; json.dump(o, open(tmp, "w")); os.replace(tmp, p)
    except Exception as e:
        log(f"save {p} fail: {e}")


def purge_locked():
    out = set(); now = time.time()
    try:
        for l in open(PURGE_LOCK):
            l = l.strip()
            if not l:
                continue
            p = l.split("\t"); ts = float(p[1]) if len(p) > 1 and p[1] else now
            if now - ts < 1800:
                out.add(p[0])
    except Exception:
        pass
    return out


def due(attempts, cycles_since):
    if attempts <= 3:
        return True
    if attempts <= 6:
        return cycles_since >= 2
    return cycles_since >= 6


def infra_alerts():
    # disk
    try:
        s = os.statvfs("/")
        pct = 100 - int(s.f_bavail * 100 / s.f_blocks)
        if pct >= DISK_ALERT_PCT and lib_alert:
            lib_alert.send(f"disk {pct}% used", f"Root filesystem at {pct}% — investigate growth.",
                           dedup_key="mon_disk", min_interval=10800)
    except Exception:
        pass
    # load
    try:
        load1 = os.getloadavg()[0]
        if load1 > LOAD_ALERT and lib_alert:
            lib_alert.send(f"load {load1:.1f}", f"1-min load average {load1:.1f} (>{LOAD_ALERT}).",
                           dedup_key="mon_load", min_interval=3600)
    except Exception:
        pass
    # container health (waha-1, waha-2, router)
    for cname in WATCH_CONTAINERS:
        try:
            out = subprocess.run(["docker", "inspect", "-f", "{{.State.Health.Status}}", cname],
                                 capture_output=True, text=True, timeout=20)
            h = (out.stdout or "").strip()
            if h and h not in ("healthy", "") and lib_alert:
                lib_alert.send(f"{cname} is {h}",
                               f"Container {cname} health={h}. If it's the router and it's down, "
                               f"the backend can't reach WAHA — bypass: set WAHA_API_URL back to "
                               f"http://waha:3000 on server+worker and `docker compose up -d server worker`.",
                               dedup_key=f"mon_health_{cname}", min_interval=1800)
        except Exception:
            pass


def router_block_alert():
    """Alert when the router SAVED a paired number from the app's DELETE/logout."""
    try:
        out = subprocess.run(["docker", "logs", "--since", "6m", "automatiza-waha-router"],
                             capture_output=True, text=True, timeout=30)
        blob = (out.stdout or "") + (out.stderr or "")
        blocks = [l for l in blob.splitlines() if "DELETE-BLOCK" in l]
        if blocks:
            log(f"ROUTER blocked {len(blocks)} unpair attempt(s) in last 6m")
            if lib_alert:
                sample = "\n  ".join(blocks[-5:])
                lib_alert.send(
                    f"router blocked {len(blocks)} unpair attempt(s)",
                    "The waha-router refused the app's attempt to unpair paired number(s) — "
                    "these numbers were SAVED (would have needed re-pairing otherwise):\n  " + sample,
                    dedup_key="mon_routerblock", min_interval=10800)
    except Exception as e:
        log(f"router block check fail: {e}")


def orphan_alert(paired):
    """Alert when a number that is ALIVE and paired in WAHA has no row in the app DB.

    This is the silent failure mode behind "o número desconectou sozinho": the chip
    never left WhatsApp — the app lost its instances row (a delete path removed the
    record while the WAHA session kept running). The app then stops listing the
    number and every automation silently drops it, while WAHA still reports WORKING.
    Nothing in the stack notices: the backend reconciler only walks DB rows, so a
    session with no row is invisible to it by construction.

    Only NEW orphans alert. The pre-existing backlog is seeded as the baseline on
    first run, so this stays quiet until something orphans a number for real.
    """
    try:
        out = subprocess.run(
            ["docker", "exec", "automatiza-postgres", "psql", "-U", "automatiza",
             "-d", "automatiza", "-t", "-A", "-c", "SELECT instance_id FROM instances;"],
            capture_output=True, text=True, timeout=30)
        if out.returncode != 0:
            log(f"orphan check: psql failed rc={out.returncode}")
            return
        db = {l.strip() for l in out.stdout.splitlines() if l.strip()}
        if not db:
            log("orphan check: empty DB read, skipping (fail-safe)")
            return

        orphans = {n: p for n, p in paired.items() if n not in db}
        st = jload(ORPHAN_STATE, {})
        known = set(st.get("known", []))
        first_run = "known" not in st

        new = sorted(set(orphans) - known)
        jsave(ORPHAN_STATE, {"known": sorted(orphans), "ts": time.time()})
        log(f"orphans: {len(orphans)} paired session(s) with no DB row (new this cycle: {len(new)})")

        if new and not first_run and lib_alert:
            lines = "\n  ".join(f"{orphans[n]}  ({n})" for n in new[:20])
            lib_alert.send(
                f"{len(new)} live number(s) lost their app record",
                "These numbers are PAIRED and alive in WAHA but have no row in the "
                "instances table — the app shows them as disconnected and every "
                "automation silently stopped using them. They do NOT need re-pairing; "
                "the DB row must be restored (see ops/waha-fix/README.md):\n  " + lines,
                dedup_key="mon_orphan", min_interval=3600)
    except Exception as e:
        log(f"orphan check fail: {e}")


def main():
    if not KEY:
        log("ERROR no api key"); return
    locked = purge_locked()
    state = jload(STATE, {})
    est = jload(ESTABLISHED, {})
    zomb = jload(ZOMBIE_STATE, {"offset": 0})
    now = time.time()
    live = {}
    acted = 0
    all_names = []           # (name, base) for zombie scan
    per_instance = {}
    paired_now = {}          # session name -> phone, for the orphan check

    for iname, base in INSTANCES.items():
        try:
            _, b = req("GET", base, "/api/sessions?all=true", timeout=60)
            sessions = json.loads(b)
        except Exception as e:
            log(f"ERROR fetch {iname}: {e}")
            if lib_alert:
                lib_alert.send(f"monitor cannot reach {iname}", str(e), dedup_key=f"mon_api_{iname}", min_interval=1800)
            continue
        per_instance[iname] = len(sessions)
        for s in sessions:
            name = s.get("name"); status = s.get("status"); paired = bool(s.get("me"))
            if not paired:
                continue
            paired_now[name] = (s.get("me") or {}).get("id", "").split("@")[0]
            ent = est.get(name, {"count": 0, "last": 0})
            ent["count"] = min(ent.get("count", 0) + 1, 100000); ent["last"] = now
            est[name] = ent
            if status == "WORKING":
                all_names.append((name, base))
                continue
            if status in ("FAILED", "STOPPED"):
                if name in locked:
                    if name in state:
                        live[name] = state[name]
                    continue
                prev = state.get(name, {"attempts": 0, "last": 0})
                cyc = (now - prev["last"]) / 300.0 if prev["last"] else 999
                if due(prev.get("attempts", 0), cyc):
                    try:
                        st, _ = req("POST", base, f"/api/sessions/{name}/restart", timeout=45)
                        prev["attempts"] = prev.get("attempts", 0) + 1; prev["last"] = now
                        log(f"RESTART {name}@{iname} status={status} attempt#{prev['attempts']} http={st}")
                        acted += 1
                        if prev["attempts"] == 5 and lib_alert:
                            lib_alert.send(f"session {name} not recovering",
                                           f"{name}@{iname} still {status} after 5 restarts. Likely needs manual re-pair (same chip).",
                                           dedup_key=f"mon_{name}", min_interval=21600)
                    except Exception as e:
                        prev["attempts"] = prev.get("attempts", 0) + 1; prev["last"] = now
                        log(f"RESTART-FAIL {name}@{iname} {e}")
                live[name] = prev

    # ---- zombie detection: rotating detail-GET over WORKING sessions ----
    zrestarts = 0
    if all_names:
        off = zomb.get("offset", 0) % len(all_names)
        window = all_names[off:off + ZOMBIE_SCAN_PER_CYCLE]
        zomb["offset"] = (off + ZOMBIE_SCAN_PER_CYCLE) % max(1, len(all_names))
        for name, base in window:
            if name in locked or zrestarts >= ZOMBIE_RESTARTS_PER_CYCLE:
                continue
            try:
                _, b = req("GET", base, f"/api/sessions/{name}", timeout=20)
                d = json.loads(b)
                eng = d.get("engine") or {}
                gows = eng.get("gows") or {}
                grpc = eng.get("grpc") or {}
                connected = gows.get("connected")
                stream = grpc.get("stream")
                # zombie: says WORKING but engine is not actually connected
                if d.get("status") == "WORKING" and (connected is False or stream not in (None, "READY")):
                    st, _ = req("POST", base, f"/api/sessions/{name}/restart", timeout=45)
                    log(f"ZOMBIE-RESTART {name}@{base} gows.connected={connected} grpc.stream={stream} http={st}")
                    zrestarts += 1; acted += 1
            except Exception:
                pass

    jsave(STATE, live)
    est = {k: v for k, v in est.items() if now - v.get("last", 0) < 7 * 86400}
    jsave(ESTABLISHED, est)
    jsave(ZOMBIE_STATE, zomb)

    # ---- autoscale signal ----
    for iname, n in per_instance.items():
        if n >= AUTOSCALE_AT and lib_alert:
            lib_alert.send(f"{iname} at {n} sessions (>= {AUTOSCALE_AT})",
                           f"{iname} holds {n} sessions (cap {CAP}). Provision another WAHA instance and migrate.",
                           dedup_key=f"mon_scale_{iname}", min_interval=43200)

    # ---- real-loss tripwire (established only) ----
    try:
        out = subprocess.run(["docker", "logs", "--since", "6m", "automatiza-waha"],
                             capture_output=True, text=True, timeout=60)
        blob = (out.stdout or "") + (out.stderr or "")
        deleted = set(re.findall(r'(session_[a-z0-9]+|user_[a-z0-9]+) - Session deleted', blob))
        real = [n for n in deleted if est.get(n, {}).get("count", 0) >= ESTABLISHED_MIN_CYCLES]
        if real:
            log(f"TRIPWIRE REAL LOSS: {real}")
            if lib_alert:
                lib_alert.send(f"TRIPWIRE: {len(real)} established number(s) deleted",
                               "Established sessions deleted:\n  " + "\n  ".join(real), dedup_key="mon_realloss", min_interval=3600)
    except Exception as e:
        log(f"tripwire fail: {e}")

    infra_alerts()
    router_block_alert()
    orphan_alert(paired_now)
    log(f"cycle: instances={per_instance} restarts={acted} zombies={zrestarts} "
        f"established={sum(1 for v in est.values() if v.get('count',0)>=ESTABLISHED_MIN_CYCLES)} locked={len(locked)}")


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        log(f"FATAL {e}")
