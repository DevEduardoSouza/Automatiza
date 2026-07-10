#!/usr/bin/env python3
"""Shared e-mail alerting for the waha-fix scripts.
Reads SMTP credentials from /opt/automatiza/docker/.env (never hard-coded here).
All sends are best-effort: a failure is logged to stderr and never raises, so a
broken SMTP path can never take down the watchdog / monitor that calls it."""
import smtplib, ssl, os, sys, time
from email.message import EmailMessage

ENV = "/opt/automatiza/docker/.env"
ALERT_TO = ["bruno.clara@yahoo.com"]
STATE_DIR = "/root/waha-fix"


def _env():
    d = {}
    try:
        with open(ENV) as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#") or "=" not in line:
                    continue
                k, v = line.split("=", 1)
                d[k] = v
    except Exception:
        pass
    return d


def send(subject, body, dedup_key=None, min_interval=0):
    """Send an alert e-mail. When dedup_key+min_interval are given, suppress the
    send if the same key fired within min_interval seconds (rate-limit)."""
    if dedup_key and min_interval:
        stamp = os.path.join(STATE_DIR, f".alert_{dedup_key}.ts")
        try:
            last = float(open(stamp).read().strip())
        except Exception:
            last = 0.0
        now = time.time()
        if now - last < min_interval:
            return False
        try:
            open(stamp, "w").write(str(now))
        except Exception:
            pass
    e = _env()
    host = e.get("SMTP_HOST", "smtp.gmail.com")
    port = int(e.get("SMTP_PORT", "587"))
    user = e.get("SMTP_USER")
    pw = e.get("SMTP_PASSWORD")
    frm = e.get("SMTP_FROM_EMAIL", user)
    if not (user and pw):
        sys.stderr.write("alert: no SMTP creds in .env\n")
        return False
    msg = EmailMessage()
    msg["Subject"] = f"[WAHA-FIX] {subject}"
    msg["From"] = frm
    msg["To"] = ", ".join(ALERT_TO)
    msg.set_content(body + f"\n\n-- \nVPS 157.180.59.35 @ {time.strftime('%FT%TZ', time.gmtime())}")
    try:
        ctx = ssl.create_default_context()
        with smtplib.SMTP(host, port, timeout=30) as s:
            s.starttls(context=ctx)
            s.login(user, pw)
            s.send_message(msg)
        return True
    except Exception as ex:
        sys.stderr.write(f"alert send failed: {ex}\n")
        return False


if __name__ == "__main__":
    subj = sys.argv[1] if len(sys.argv) > 1 else "test alert"
    body = sys.argv[2] if len(sys.argv) > 2 else "This is a test alert from the waha-fix tooling."
    print("sent" if send(subj, body) else "NOT-sent")
