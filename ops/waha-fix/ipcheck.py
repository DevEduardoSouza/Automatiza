#!/usr/bin/env python3
"""Anti-ban egress self-check for the WAHA session proxy pool.

WhatsApp unlinks freshly-paired devices at a high rate when hundreds of accounts
share one datacenter IP, so new pairings are routed through a pool of
residential/ISP proxies (see docker/waha-router: WAHA_SESSION_PROXIES). That only
protects us if the proxies actually change the egress IP. A misconfigured,
expired, or transparently-failing proxy silently falls back to the bare VPS IP —
which looks fine to WAHA but re-exposes every session on the one datacenter IP we
were trying to avoid.

This script verifies each configured proxy really moves egress OFF the host IP:
  1. host_ip  = our public IP with NO proxy (baseline).
  2. proxy_ip = public IP fetched THROUGH each proxy.
     * request fails            -> FAIL
     * proxy_ip == host_ip      -> LEAK  (proxy not changing egress / host fallback)
     * proxy_ip != host_ip      -> OK

Exit 0 only if every proxy is OK; exit 1 on any LEAK/FAIL, or if no proxies are
configured at all (sessions egressing on the bare host IP). Safe to run by hand
(`python3 ipcheck.py`) and suitable to gate a cron/alert. Reuses lib_alert when
importable; otherwise it just prints."""
import json, os, sys, urllib.request, urllib.parse

ENV = "/opt/automatiza/docker/.env"
IPIFY = "https://api.ipify.org?format=json"
TIMEOUT = 15

sys.path.insert(0, "/root/waha-fix")
try:
    import lib_alert
except Exception:
    lib_alert = None


def env_value(key):
    """Return env var `key` from os.environ, falling back to the .env file
    (same line-by-line prefix match monitor.py uses for WAHA_API_KEY)."""
    v = os.environ.get(key)
    if v is not None:
        return v
    try:
        with open(ENV) as f:
            for l in f:
                if l.startswith(key + "="):
                    return l.strip().split("=", 1)[1]
    except Exception:
        pass
    return ""


def fetch_ip(opener):
    """GET api.ipify through `opener` and return the reported public IP."""
    r = urllib.request.Request(IPIFY)
    with opener.open(r, timeout=TIMEOUT) as resp:
        return json.loads(resp.read()).get("ip", "")


def proxy_url(server, user, pw):
    """Build an http://[user:pass@]host:port URL for ProxyHandler from a
    `server` that may be "host:port" or "scheme://host:port"."""
    server = server.strip()
    if "://" in server:
        scheme, host = server.split("://", 1)
    else:
        scheme, host = "http", server
    if user:
        cred = urllib.parse.quote(user, safe="")
        if pw:
            cred += ":" + urllib.parse.quote(pw, safe="")
        return f"{scheme}://{cred}@{host}"
    return f"{scheme}://{host}"


def parse_proxies(raw):
    """Parse WAHA_SESSION_PROXIES (comma-separated "server;username;password")
    into a list of (server, proxy_url) tuples. Mirrors docker/waha-router."""
    out = []
    for part in (raw or "").split(","):
        part = part.strip()
        if not part:
            continue
        seg = part.split(";")
        server = seg[0].strip()
        if not server:
            continue
        user = seg[1].strip() if len(seg) >= 2 else ""
        pw = seg[2].strip() if len(seg) >= 3 else ""
        out.append((server, proxy_url(server, user, pw)))
    return out


def alert(subject, body, dedup_key):
    if lib_alert:
        try:
            lib_alert.send(subject, body, dedup_key=dedup_key, min_interval=3600)
        except Exception as e:
            print(f"alert send failed: {e}")


def main():
    # ---- baseline: host public IP with NO proxy ----
    no_proxy = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    try:
        host_ip = fetch_ip(no_proxy)
    except Exception as e:
        print(f"ERROR could not determine host IP (no proxy): {e}")
        alert("ipcheck cannot reach ipify", f"Baseline host-IP lookup failed: {e}",
              "ipcheck_baseline")
        return 1
    print(f"host_ip (baseline, no proxy): {host_ip}")

    proxies = parse_proxies(env_value("WAHA_SESSION_PROXIES"))
    if not proxies:
        msg = ("WAHA_SESSION_PROXIES is empty/unset — new WhatsApp sessions are "
               f"egressing on the BARE HOST IP {host_ip}. Every account shares the "
               "one datacenter IP, which is exactly the ban vector the proxy pool "
               "exists to avoid. Configure the pool and redeploy waha-router.")
        print(f"WARNING {msg}")
        alert("no session proxies configured", msg, "ipcheck_noproxies")
        return 1

    print(f"checking {len(proxies)} proxy(ies)...")
    ok, leaks, fails = [], [], []
    for server, purl in proxies:
        opener = urllib.request.build_opener(
            urllib.request.ProxyHandler({"http": purl, "https": purl}))
        try:
            proxy_ip = fetch_ip(opener)
        except Exception as e:
            print(f"FAIL {server}: {e}")
            fails.append((server, str(e)))
            continue
        if proxy_ip == host_ip:
            print(f"LEAK {server}: egress == host IP {host_ip}")
            leaks.append((server, proxy_ip))
        else:
            print(f"OK {server}: egress {proxy_ip}")
            ok.append((server, proxy_ip))

    print(f"\nsummary: {len(ok)} OK, {len(leaks)} LEAK, {len(fails)} FAIL "
          f"(of {len(proxies)} proxies; host_ip={host_ip})")

    if leaks or fails:
        lines = [f"host_ip (baseline): {host_ip}", ""]
        for s, ip in leaks:
            lines.append(f"LEAK {s}: egress == host IP {ip} (not changing egress / host fallback)")
        for s, err in fails:
            lines.append(f"FAIL {s}: {err}")
        alert(f"{len(leaks)} proxy leak(s), {len(fails)} failure(s)",
              "\n".join(lines), "ipcheck_leakfail")
        return 1
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"FATAL {e}")
        sys.exit(1)
