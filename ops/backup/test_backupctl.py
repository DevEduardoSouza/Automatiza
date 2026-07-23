import datetime as dt
import importlib.util
import json
import pathlib
import sys
import tarfile
import tempfile
import unittest
from unittest import mock


MODULE_PATH = pathlib.Path(__file__).with_name("backupctl.py")
SPEC = importlib.util.spec_from_file_location("backupctl", MODULE_PATH)
assert SPEC and SPEC.loader
backupctl = importlib.util.module_from_spec(SPEC)
sys.modules[SPEC.name] = backupctl
SPEC.loader.exec_module(backupctl)


class BackupCtlTest(unittest.TestCase):
    def test_parse_env_rejects_non_assignment(self):
        with tempfile.TemporaryDirectory() as directory:
            path = pathlib.Path(directory) / "config"
            path.write_text("BACKUP_ENABLED=false\nnot-an-assignment\n", encoding="utf-8")
            with self.assertRaises(backupctl.BackupError) as raised:
                backupctl.parse_env_file(path)
            self.assertEqual("BACKUP_CONFIG_INVALID", raised.exception.code)

    def test_execute_guard_requires_flag_and_exact_sentinel(self):
        with tempfile.TemporaryDirectory() as directory:
            sentinel = pathlib.Path(directory) / "enabled"
            config = backupctl.Config(
                {"BACKUP_ENABLED": "true", "BACKUP_ENABLE_SENTINEL": str(sentinel)}
            )
            with self.assertRaises(backupctl.BackupError):
                backupctl.require_execute_guard(config)
            sentinel.write_text("wrong\n", encoding="utf-8")
            with self.assertRaises(backupctl.BackupError):
                backupctl.require_execute_guard(config)
            sentinel.write_text("AUTOMATIZA_BACKUP_ENABLED_V1\n", encoding="utf-8")
            backupctl.require_execute_guard(config)

    def test_artifact_keys_are_immutable_and_component_scoped(self):
        moment = dt.datetime(2026, 7, 23, 12, 30, tzinfo=dt.timezone.utc)
        key = backupctl.artifact_key("waha-2", "abc123", moment)
        self.assertEqual("waha-2/2026/07/23/20260723T123000Z-abc123.tar.age", key)
        pg_key = backupctl.artifact_key("postgres-logical", "abc123", moment)
        self.assertEqual(
            "postgres-logical/2026/07/23/20260723T123000Z-abc123.dump.age",
            pg_key,
        )

    def test_atomic_status_preserves_last_success_on_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            config = backupctl.Config({"BACKUP_STATE_DIR": directory})
            backupctl.write_status(config, "waha-1", success=True, run_id="one")
            before = json.loads(
                (pathlib.Path(directory) / "status/waha-1.json").read_text(encoding="utf-8")
            )
            backupctl.write_status(
                config,
                "waha-1",
                success=False,
                run_id="two",
                error_code="BACKUP_PRODUCER_FAILED",
            )
            after = json.loads(
                (pathlib.Path(directory) / "status/waha-1.json").read_text(encoding="utf-8")
            )
            self.assertEqual(before["last_success_at"], after["last_success_at"])
            self.assertFalse(after["success"])

    def test_restore_guard_rejects_production_mode_and_nonempty_target(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            sentinel = root / "allowed"
            sentinel.write_text("AUTOMATIZA_STAGING_RESTORE_V1\n", encoding="utf-8")
            target = root / "restore"
            config = backupctl.Config(
                {
                    "BACKUP_MODE": "production",
                    "RESTORE_ENABLE_SENTINEL": str(sentinel),
                    "RESTORE_ALLOWED_ROOT": str(root),
                }
            )
            with self.assertRaises(backupctl.BackupError):
                backupctl.require_restore_guard(config, target)
            config = backupctl.Config(
                {
                    "BACKUP_MODE": "staging",
                    "RESTORE_ENABLE_SENTINEL": str(sentinel),
                    "RESTORE_ALLOWED_ROOT": str(root),
                }
            )
            resolved = backupctl.require_restore_guard(config, target)
            (resolved / "existing").write_text("x", encoding="utf-8")
            with self.assertRaises(backupctl.BackupError) as raised:
                backupctl.require_restore_guard(config, target)
            self.assertEqual("RESTORE_TARGET_NOT_EMPTY", raised.exception.code)

    def test_verify_has_bounded_labels_and_detects_stale_components(self):
        with tempfile.TemporaryDirectory() as directory:
            config = backupctl.Config(
                {
                    "BACKUP_STATE_DIR": directory,
                    "BACKUP_MONITORED_MOUNTS": directory,
                }
            )
            with mock.patch.object(backupctl, "collect_wal_age", return_value=301):
                alerts = backupctl.verify(config, now=1_000_000)
            stale = {a.get("component") for a in alerts if a["error_code"] == "VERIFY_BACKUP_STALE"}
            self.assertEqual(
                set(backupctl.FRESHNESS_SECONDS) - {"postgres-full", "postgres-diff"},
                stale,
            )
            self.assertFalse(
                any(a["error_code"] == "VERIFY_WAL_STALE" for a in alerts)
            )
            metrics = (
                pathlib.Path(directory) / "metrics/automatiza_backup.prom"
            ).read_text(encoding="utf-8")
            self.assertNotIn(str(pathlib.Path(directory)), metrics)
            self.assertIn('mount="mount-0"', metrics)

    def test_safe_remote_rejects_parent_traversal(self):
        config = backupctl.Config({"BACKUP_RCLONE_REMOTE": "backup:bucket"})
        with self.assertRaises(backupctl.BackupError):
            backupctl.safe_remote(config, "../secret")

    def test_successful_backup_writes_manifest_status_without_field_collision(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            sentinel = root / "enabled"
            sentinel.write_text("AUTOMATIZA_BACKUP_ENABLED_V1\n", encoding="utf-8")
            config = backupctl.Config(
                {
                    "BACKUP_ENABLED": "true",
                    "BACKUP_ENABLE_SENTINEL": str(sentinel),
                    "BACKUP_STATE_DIR": directory,
                }
            )
            manifest = {
                "schema_version": 1,
                "component": "waha-1",
                "run_id": "producer-run",
                "artifact": "waha-1/example.tar.age",
                "bytes": 10,
                "sha256": "0" * 64,
            }
            with mock.patch.object(
                backupctl, "stream_encrypted_backup", return_value=manifest
            ):
                backupctl.backup(config, "waha-1", execute=True)
            status = json.loads(
                (root / "status/waha-1.json").read_text(encoding="utf-8")
            )
            self.assertTrue(status["success"])
            self.assertEqual("waha-1/example.tar.age", status["artifact"])

    def test_component_lock_rejects_concurrent_run(self):
        with tempfile.TemporaryDirectory() as directory:
            config = backupctl.Config({"BACKUP_STATE_DIR": directory})
            with backupctl.component_lock(config, "waha-1"):
                with self.assertRaises(backupctl.BackupError) as raised:
                    with backupctl.component_lock(config, "waha-1"):
                        pass
            self.assertEqual("BACKUP_ALREADY_RUNNING", raised.exception.code)

    def test_physical_backup_is_blocked_until_pitr_gate_is_enabled(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            sentinel = root / "enabled"
            sentinel.write_text("AUTOMATIZA_BACKUP_ENABLED_V1\n", encoding="utf-8")
            config = backupctl.Config(
                {
                    "BACKUP_ENABLED": "true",
                    "BACKUP_ENABLE_SENTINEL": str(sentinel),
                    "BACKUP_STATE_DIR": directory,
                    "BACKUP_PITR_ENABLED": "false",
                }
            )
            with mock.patch.object(backupctl, "run_pgbackrest_backup") as runner:
                with self.assertRaises(backupctl.BackupError) as raised:
                    backupctl.backup(config, "postgres-full", execute=True)
            self.assertEqual("BACKUP_DISABLED", raised.exception.code)
            runner.assert_not_called()

    def test_destination_failure_is_recorded_as_unknown_without_retry(self):
        with tempfile.TemporaryDirectory() as directory:
            root = pathlib.Path(directory)
            sentinel = root / "enabled"
            sentinel.write_text("AUTOMATIZA_BACKUP_ENABLED_V1\n", encoding="utf-8")
            config = backupctl.Config(
                {
                    "BACKUP_ENABLED": "true",
                    "BACKUP_ENABLE_SENTINEL": str(sentinel),
                    "BACKUP_STATE_DIR": directory,
                }
            )
            with mock.patch.object(
                backupctl,
                "stream_encrypted_backup",
                side_effect=backupctl.BackupError(
                    "BACKUP_DESTINATION_FAILED", "uncertain remote result"
                ),
            ):
                with self.assertRaises(backupctl.BackupError):
                    backupctl.backup(config, "waha-1", execute=True)
            status = json.loads(
                (root / "status/waha-1.json").read_text(encoding="utf-8")
            )
            self.assertEqual("unknown", status["outcome"])
            self.assertFalse(status["success"])

    def test_safe_extract_rejects_path_traversal(self):
        with tempfile.SpooledTemporaryFile() as archive_data:
            with tarfile.open(fileobj=archive_data, mode="w") as archive:
                info = tarfile.TarInfo("../escape")
                info.size = 1
                archive.addfile(info, backupctl.io.BytesIO(b"x"))
            archive_data.seek(0)
            with tempfile.TemporaryDirectory() as directory:
                with self.assertRaises(backupctl.BackupError) as raised:
                    backupctl.safe_extract_stream(archive_data, pathlib.Path(directory))
        self.assertEqual("RESTORE_GUARD_REJECTED", raised.exception.code)

    def test_inventory_uses_bounded_prefix_labels_and_tracks_growth(self):
        with tempfile.TemporaryDirectory() as directory:
            config = backupctl.Config(
                {
                    "BACKUP_STATE_DIR": directory,
                    "BACKUP_RCLONE_REMOTE": "backup:automatiza",
                    "BACKUP_DESTINATION_BUDGET_BYTES": "1000",
                    "BACKUP_MONITORED_S3_PREFIXES": "backup:automatiza/media",
                    "BACKUP_MONITORED_TABLES": "public.dispatch_logs",
                }
            )
            with (
                mock.patch.object(
                    backupctl,
                    "rclone_size",
                    side_effect=[(500, 4), (200, 2)],
                ),
                mock.patch.object(
                    backupctl,
                    "postgres_relation_sizes",
                    return_value={"public.dispatch_logs": 300},
                ),
            ):
                self.assertEqual([], backupctl.inventory(config))
            metrics = (
                pathlib.Path(directory) / "metrics/automatiza_capacity.prom"
            ).read_text(encoding="utf-8")
            self.assertIn('prefix="prefix-0"', metrics)
            self.assertNotIn("backup:automatiza/media", metrics)
            status = json.loads(
                (pathlib.Path(directory) / "status/inventory.json").read_text(
                    encoding="utf-8"
                )
            )
            self.assertTrue(status["success"])
            self.assertEqual([], status["alerts"])

    def test_vps_allowlist_rejects_secret_like_paths(self):
        with tempfile.TemporaryDirectory() as directory:
            allowlist = pathlib.Path(directory) / "allowlist"
            allowlist.write_text("opt/automatiza/docker/.env\n", encoding="utf-8")
            config = backupctl.Config({"BACKUP_CONFIG_ALLOWLIST": str(allowlist)})
            with self.assertRaises(backupctl.BackupError) as raised:
                backupctl.producer_for(config, "vps-config")
        self.assertEqual("BACKUP_CONFIG_INVALID", raised.exception.code)

    def test_waha_source_must_be_below_explicit_allowed_root(self):
        with tempfile.TemporaryDirectory() as directory:
            source = pathlib.Path(directory) / "source"
            source.mkdir()
            config = backupctl.Config(
                {
                    "BACKUP_WAHA1_PATH": str(source),
                    "BACKUP_WAHA_ALLOWED_ROOT": "/var/lib/docker/volumes",
                }
            )
            with self.assertRaises(backupctl.BackupError) as raised:
                backupctl.producer_for(config, "waha-1")
        self.assertEqual("BACKUP_CONFIG_INVALID", raised.exception.code)

    def test_remote_manifest_requires_exact_checksum_shape(self):
        config = backupctl.Config({"BACKUP_RCLONE_REMOTE": "backup:automatiza"})
        marker = b"AUTOMATIZA_BACKUP_COMPLETE_V1\n"
        manifest = json.dumps(
            {
                "schema_version": 1,
                "artifact": "waha-1/a.tar.age",
                "sha256": "not-a-checksum",
                "bytes": 1,
            }
        ).encode()
        with mock.patch.object(
            backupctl, "remote_bytes", side_effect=[marker, manifest]
        ):
            with self.assertRaises(backupctl.BackupError) as raised:
                backupctl.load_remote_manifest(config, "waha-1/a.tar.age")
        self.assertEqual("VERIFY_ARTIFACT_INVALID", raised.exception.code)

    def test_pgbackrest_repository_size_uses_latest_backup(self):
        payload = json.dumps(
            [
                {
                    "backup": [
                        {"info": {"repository": {"size": 10}}},
                        {"info": {"repository": {"size": 25}}},
                    ]
                }
            ]
        ).encode()
        self.assertEqual(25, backupctl.pgbackrest_repository_size(payload))

    def test_pitr_checks_are_enabled_only_after_explicit_flag(self):
        with tempfile.TemporaryDirectory() as directory:
            config = backupctl.Config(
                {
                    "BACKUP_STATE_DIR": directory,
                    "BACKUP_MONITORED_MOUNTS": directory,
                    "BACKUP_PITR_ENABLED": "true",
                    "BACKUP_PG_WAL_PATH": str(pathlib.Path(directory) / "missing"),
                }
            )
            with mock.patch.object(backupctl, "collect_wal_age", return_value=None):
                alerts = backupctl.verify(config, now=1_000_000)
            codes = {item["error_code"] for item in alerts}
            self.assertIn("VERIFY_WAL_STALE", codes)
            self.assertIn("VERIFY_WAL_CAPACITY", codes)
            stale = {
                item.get("component")
                for item in alerts
                if item["error_code"] == "VERIFY_BACKUP_STALE"
            }
            self.assertIn("postgres-full", stale)
            self.assertIn("postgres-diff", stale)

    def test_restore_count_smoke_requires_nonempty_core_tables(self):
        backupctl.validate_restore_counts("2\t3\n", 1, 1)
        with self.assertRaises(backupctl.BackupError):
            backupctl.validate_restore_counts("0\t3\n", 1, 1)

    def test_restore_container_copy_is_removed_after_smoke(self):
        with tempfile.TemporaryDirectory() as directory:
            dump = pathlib.Path(directory) / "restore.dump"
            dump.write_bytes(b"test")
            config = backupctl.Config(
                {
                    "RESTORE_PG_CONTAINER": "restore-postgres",
                    "RESTORE_DB_NAME": "restore_db",
                    "RESTORE_DB_USER": "restore_user",
                }
            )
            results = [
                mock.Mock(returncode=0, stdout="0\n"),
                mock.Mock(returncode=0),
                mock.Mock(returncode=0),
                mock.Mock(returncode=0, stdout="2\t3\n"),
                mock.Mock(returncode=0),
            ]
            with mock.patch.object(
                backupctl.subprocess,
                "run",
                side_effect=results,
            ) as runner:
                backupctl.restore_dump_to_container(config, "docker", dump)
            cleanup_command = runner.call_args_list[-1].args[0]
            self.assertEqual(
                ["docker", "exec", "restore-postgres", "rm", "-f", "--"],
                cleanup_command[:6],
            )


if __name__ == "__main__":
    unittest.main()
