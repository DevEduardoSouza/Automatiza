import datetime as dt
import importlib.util
import json
import pathlib
import sys
import tempfile
import unittest
from unittest import mock


MODULE_PATH = pathlib.Path(__file__).with_name("monitor.py")
SPEC = importlib.util.spec_from_file_location("waha_monitor", MODULE_PATH)
assert SPEC and SPEC.loader
monitor = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = monitor
SPEC.loader.exec_module(monitor)


class BackupAlertBridgeTest(unittest.TestCase):
    def test_disabled_bridge_does_not_page(self):
        with tempfile.TemporaryDirectory() as directory:
            monitor.BACKUP_ENABLE_SENTINEL = str(pathlib.Path(directory) / "absent")
            sender = mock.Mock()
            with mock.patch.object(monitor, "lib_alert", sender):
                self.assertEqual([], monitor.backup_alerts(now=1_000_000))
            sender.send.assert_not_called()

    def test_missing_waha_2_status_pages_through_existing_backend(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            sentinel = root / "ENABLED"
            sentinel.write_text("AUTOMATIZA_BACKUP_ENABLED_V1\n", encoding="utf-8")
            status_dir = root / "status"
            status_dir.mkdir()
            now = dt.datetime(2026, 7, 23, 12, tzinfo=dt.timezone.utc)
            timestamp = now.isoformat(timespec="seconds").replace("+00:00", "Z")
            for component in ("postgres-logical", "vps-config", "waha-1"):
                (status_dir / f"{component}.json").write_text(
                    json.dumps(
                        {
                            "last_success_at": timestamp,
                            "success": True,
                        }
                    ),
                    encoding="utf-8",
                )
            for filename in (
                "verification.json",
                "verification-remote.json",
                "inventory.json",
            ):
                (status_dir / filename).write_text(
                    json.dumps(
                        {
                            "finished_at": timestamp,
                            "success": True,
                            "alerts": [],
                        }
                    ),
                    encoding="utf-8",
                )
            sender = mock.Mock()
            with (
                mock.patch.object(monitor, "BACKUP_ENABLE_SENTINEL", str(sentinel)),
                mock.patch.object(monitor, "BACKUP_STATUS_DIR", str(status_dir)),
                mock.patch.object(monitor, "lib_alert", sender),
                mock.patch.object(monitor, "log"),
            ):
                issues = monitor.backup_alerts(now=now.timestamp())
            self.assertEqual(["waha-2: stale_or_missing"], issues)
            sender.send.assert_called_once()


if __name__ == "__main__":
    unittest.main()
