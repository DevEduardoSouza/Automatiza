#!/usr/bin/env python3
"""Phase 6 reconciliation report (READ-ONLY). WAHA runtime vs DB instances."""
import json, subprocess
KEY = ""
for l in open("/opt/automatiza/docker/.env"):
    if l.startswith("WAHA_API_KEY="):
        KEY = l.strip().split("=", 1)[1]
raw = subprocess.run(["curl","-s","-m","60","-H",f"X-Api-Key: {KEY}",
    "http://127.0.0.1:3001/api/sessions?all=true"], capture_output=True, text=True).stdout
d = json.loads(raw)
waha = {s["name"]:(s.get("status"), bool(s.get("me"))) for s in d}
dbraw = subprocess.run(["docker","exec","-i","automatiza-postgres","psql","-U","automatiza",
    "-d","automatiza","-tA","-F","|","-c","SELECT instance_id,status FROM instances;"],
    capture_output=True, text=True).stdout
db = {}
for l in dbraw.splitlines():
    p = l.split("|")
    if len(p) >= 2 and p[0]: db[p[0]] = p[1]
wset, dset = set(waha), set(db)
print("========= RECONCILIAÇÃO (Fase 6) — READ-ONLY =========")
print("WAHA runtime: %d | DB: %d (connected=%d disconnected=%d pending=%d)" % (
    len(wset), len(dset),
    sum(1 for v in db.values() if v=="connected"),
    sum(1 for v in db.values() if v=="disconnected"),
    sum(1 for v in db.values() if v=="pending")))
print("[1] WAHA sem registro no DB (órfãs runtime): %d" % len(wset-dset))
for n in sorted(wset-dset)[:12]: print("    ", n, waha[n])
print("[2] DB 'connected' sem sessão no WAHA (fantasmas): %d" % len([i for i,s in db.items() if s=="connected" and i not in wset]))
print("[3] DB 'disconnected' mas WAHA WORKING (lag benigno): %d" % len([i for i,s in db.items() if s=="disconnected" and i in waha and waha[i][0]=="WORKING"]))
print("[4] DB sem sessão no WAHA (registros de deletados; CASCATA->groups, NÃO apagar sem dev): %d" % len(dset-wset))
cand = [n for n in wset if not waha[n][1] or waha[n][0] in ("FAILED","SCAN_QR_CODE","STOPPED")]
print("[5] WAHA candidatas a limpeza (não-pareada OU FAILED/SCAN/STOPPED): %d" % len(cand))
for n in sorted(cand)[:15]: print("    ", n, waha[n])
print("========= FIM =========")
