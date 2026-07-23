#!/usr/bin/env python3
"""Safe, opt-in backup and restore orchestration for Automatiza.

The controller never initializes repositories, changes retention, deletes
objects, or enables PostgreSQL archiving. Those are human-operated runbook
steps. External writes require both BACKUP_ENABLED=true and an enable sentinel.
"""

from __future__ import annotations

import argparse
import contextlib
import datetime as dt
import fcntl
import hashlib
import io
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import uuid
from dataclasses import dataclass
from typing import BinaryIO, Iterable, Mapping, Sequence


SCHEMA_VERSION = 1
COMPONENTS = {
    "postgres-logical",
    "postgres-full",
    "postgres-diff",
    "waha-1",
    "waha-2",
    "vps-config",
}
FRESHNESS_SECONDS = {
    "postgres-logical": 26 * 3600,
    "postgres-full": 8 * 24 * 3600,
    "postgres-diff": 26 * 3600,
    "waha-1": 8 * 3600,
    "waha-2": 8 * 3600,
    "vps-config": 26 * 3600,
}
ERROR_CODES = {
    "BACKUP_ALREADY_RUNNING",
    "BACKUP_CHECKSUM_MISMATCH",
    "BACKUP_CONFIG_INVALID",
    "BACKUP_DESTINATION_FAILED",
    "BACKUP_DISABLED",
    "BACKUP_LOW_CAPACITY",
    "BACKUP_PRODUCER_FAILED",
    "BACKUP_TOOL_MISSING",
    "RESTORE_DATABASE_FAILED",
    "RESTORE_GUARD_REJECTED",
    "RESTORE_TARGET_NOT_EMPTY",
    "VERIFY_ARTIFACT_INVALID",
    "VERIFY_BACKUP_STALE",
    "VERIFY_DESTINATION_CAPACITY",
    "VERIFY_DISK_CRITICAL",
    "VERIFY_INODE_CRITICAL",
    "VERIFY_WAL_CAPACITY",
    "VERIFY_WAL_STALE",
}


class BackupError(RuntimeError):
    def __init__(self, code: str, message: str):
        if code not in ERROR_CODES:
            raise ValueError(f"unknown error code: {code}")
        super().__init__(message)
        self.code = code


def utc_now() -> dt.datetime:
    return dt.datetime.now(dt.timezone.utc)


def utc_text(value: dt.datetime | None = None) -> str:
    return (value or utc_now()).isoformat(timespec="seconds").replace("+00:00", "Z")


def parse_env_file(path: pathlib.Path) -> dict[str, str]:
    values: dict[str, str] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except FileNotFoundError as exc:
        raise BackupError("BACKUP_CONFIG_INVALID", f"configuration file not found: {path}") from exc
    for number, raw in enumerate(lines, 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            raise BackupError("BACKUP_CONFIG_INVALID", f"invalid configuration line {number}")
        key, value = line.split("=", 1)
        key = key.strip()
        if not key or not key.replace("_", "").isalnum():
            raise BackupError("BACKUP_CONFIG_INVALID", f"invalid configuration key on line {number}")
        values[key] = value.strip()
    return values


@dataclass(frozen=True)
class Config:
    values: Mapping[str, str]

    @classmethod
    def load(cls, path: pathlib.Path) -> "Config":
        merged = parse_env_file(path)
        for key, value in os.environ.items():
            if key.startswith(("BACKUP_", "RESTORE_")):
                merged[key] = value
        return cls(merged)

    def get(self, key: str, default: str = "") -> str:
        return self.values.get(key, default)

    def require(self, key: str) -> str:
        value = self.get(key)
        if not value:
            raise BackupError("BACKUP_CONFIG_INVALID", f"required setting is missing: {key}")
        return value

    def bool(self, key: str, default: bool = False) -> bool:
        raw = self.get(key, "true" if default else "false").lower()
        if raw not in {"true", "false"}:
            raise BackupError("BACKUP_CONFIG_INVALID", f"{key} must be true or false")
        return raw == "true"

    def integer(self, key: str, default: int = 0) -> int:
        raw = self.get(key, str(default))
        try:
            return int(raw)
        except ValueError as exc:
            raise BackupError("BACKUP_CONFIG_INVALID", f"{key} must be an integer") from exc

    @property
    def state_dir(self) -> pathlib.Path:
        return pathlib.Path(self.get("BACKUP_STATE_DIR", "/var/lib/automatiza-backup"))

    @property
    def metrics_dir(self) -> pathlib.Path:
        return pathlib.Path(self.get("BACKUP_METRICS_DIR", str(self.state_dir / "metrics")))


def emit(event: str, **fields: object) -> None:
    record = {"timestamp": utc_text(), "event": event, **fields}
    print(json.dumps(record, sort_keys=True, separators=(",", ":")), flush=True)


def atomic_json(path: pathlib.Path, value: Mapping[str, object]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o750)
    fd, temp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temp_path = pathlib.Path(temp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            json.dump(value, handle, sort_keys=True, separators=(",", ":"))
            handle.write("\n")
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temp_path, 0o640)
        os.replace(temp_path, path)
    finally:
        with contextlib.suppress(FileNotFoundError):
            temp_path.unlink()


def load_json(path: pathlib.Path) -> dict[str, object]:
    try:
        value = json.loads(path.read_text(encoding="utf-8"))
    except (FileNotFoundError, json.JSONDecodeError):
        return {}
    return value if isinstance(value, dict) else {}


@contextlib.contextmanager
def component_lock(config: Config, component: str) -> Iterable[None]:
    lock_dir = config.state_dir / "locks"
    lock_dir.mkdir(parents=True, exist_ok=True, mode=0o750)
    with (lock_dir / f"{component}.lock").open("a+", encoding="utf-8") as handle:
        try:
            fcntl.flock(handle.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError as exc:
            raise BackupError("BACKUP_ALREADY_RUNNING", f"{component} backup is already running") from exc
        yield


def require_tool(name: str) -> str:
    resolved = shutil.which(name)
    if not resolved:
        raise BackupError("BACKUP_TOOL_MISSING", f"required tool is unavailable: {name}")
    return resolved


def rclone_command(config: Config, *args: str) -> list[str]:
    rclone = require_tool(config.get("BACKUP_RCLONE_BIN", "rclone"))
    config_path = pathlib.Path(
        config.get("BACKUP_RCLONE_CONFIG", "/etc/automatiza-backup/rclone.conf")
    )
    if not config_path.is_file():
        raise BackupError("BACKUP_CONFIG_INVALID", "rclone configuration file is unavailable")
    return [rclone, "--config", str(config_path), *args]


def require_execute_guard(config: Config) -> None:
    if not config.bool("BACKUP_ENABLED"):
        raise BackupError("BACKUP_DISABLED", "external backup writes are disabled")
    sentinel = pathlib.Path(config.get("BACKUP_ENABLE_SENTINEL", "/etc/automatiza-backup/ENABLED"))
    try:
        content = sentinel.read_text(encoding="utf-8").strip()
    except FileNotFoundError as exc:
        raise BackupError("BACKUP_DISABLED", "backup enable sentinel is absent") from exc
    if content != "AUTOMATIZA_BACKUP_ENABLED_V1":
        raise BackupError("BACKUP_DISABLED", "backup enable sentinel is invalid")


def validate_rclone_target(target: str) -> str:
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]*:[^\r\n]+", target):
        raise BackupError("BACKUP_CONFIG_INVALID", "rclone target must use a named remote")
    return target


def safe_remote(config: Config, key: str) -> str:
    remote = validate_rclone_target(config.require("BACKUP_RCLONE_REMOTE").rstrip("/"))
    if (
        not key
        or key.startswith("/")
        or ".." in pathlib.PurePosixPath(key).parts
        or not re.fullmatch(r"[A-Za-z0-9._/-]+", key)
    ):
        raise BackupError("BACKUP_CONFIG_INVALID", "unsafe backup object key")
    return f"{remote}/{key}"


def artifact_key(component: str, run_id: str, now: dt.datetime | None = None) -> str:
    current = now or utc_now()
    stamp = current.strftime("%Y/%m/%d/%Y%m%dT%H%M%SZ")
    extension = "dump.age" if component == "postgres-logical" else "tar.age"
    return f"{component}/{stamp}-{run_id}.{extension}"


def producer_for(config: Config, component: str) -> list[str]:
    if component == "postgres-logical":
        return [
            require_tool(config.get("BACKUP_DOCKER_BIN", "docker")),
            "exec",
            "-i",
            config.get("BACKUP_PG_CONTAINER", "automatiza-postgres"),
            "pg_dump",
            "-U",
            config.get("BACKUP_DB_USER", "automatiza_backup"),
            "-d",
            config.get("BACKUP_DB_NAME", "automatiza"),
            "-Fc",
        ]
    if component in {"waha-1", "waha-2"}:
        key = "BACKUP_WAHA1_PATH" if component == "waha-1" else "BACKUP_WAHA2_PATH"
        source = pathlib.Path(config.require(key)).resolve()
        allowed_root = pathlib.Path(
            config.get("BACKUP_WAHA_ALLOWED_ROOT", "/var/lib/docker/volumes")
        ).resolve()
        if source == allowed_root or allowed_root not in source.parents:
            raise BackupError("BACKUP_CONFIG_INVALID", f"{component} source is outside the allowed root")
        if not source.is_dir():
            raise BackupError("BACKUP_CONFIG_INVALID", f"{component} source directory is unavailable")
        return [require_tool("tar"), "-C", str(source), "-cf", "-", "."]
    if component == "vps-config":
        allowlist = pathlib.Path(config.require("BACKUP_CONFIG_ALLOWLIST")).resolve()
        if not allowlist.is_file():
            raise BackupError("BACKUP_CONFIG_INVALID", "configuration allowlist is unavailable")
        entries = [
            line.strip()
            for line in allowlist.read_text(encoding="utf-8").splitlines()
            if line.strip() and not line.lstrip().startswith("#")
        ]
        deny_fragments = (".env", "private", "secret", "credential", "age-key")
        if (
            not entries
            or len(entries) > 100
            or any(
                entry.startswith(("/", "-"))
                or ".." in pathlib.PurePosixPath(entry).parts
                or any(fragment in entry.lower() for fragment in deny_fragments)
                for entry in entries
            )
        ):
            raise BackupError("BACKUP_CONFIG_INVALID", "configuration allowlist contains an unsafe entry")
        return [require_tool("tar"), "-C", "/", "-cf", "-", "--", *entries]
    raise BackupError("BACKUP_CONFIG_INVALID", f"unsupported streamed component: {component}")


def _read_stderr(handle: BinaryIO) -> str:
    handle.seek(0)
    text = handle.read(4096).decode("utf-8", "replace").strip()
    return text or "command failed without diagnostic output"


def run_checked(command: Sequence[str], *, input_bytes: bytes | None = None) -> subprocess.CompletedProcess[bytes]:
    result = subprocess.run(command, input=input_bytes, capture_output=True, check=False)
    if result.returncode != 0:
        raise BackupError("BACKUP_PRODUCER_FAILED", "backup command failed")
    return result


def pgbackrest_repository_size(payload: bytes) -> int:
    try:
        info_value = json.loads(payload)
        backups = info_value[0]["backup"]
        latest = backups[-1]
        return int(latest["info"]["repository"]["size"])
    except (json.JSONDecodeError, IndexError, KeyError, TypeError, ValueError) as exc:
        raise BackupError("BACKUP_PRODUCER_FAILED", "pgBackRest info output is invalid") from exc


def upload_bytes(config: Config, remote: str, payload: bytes) -> None:
    result = subprocess.run(
        rclone_command(config, "rcat", remote),
        input=payload,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.PIPE,
    )
    if result.returncode != 0:
        raise BackupError("BACKUP_DESTINATION_FAILED", "backup metadata upload failed")


def remote_bytes(config: Config, remote: str, *, limit: int = 1024 * 1024) -> bytes:
    process = subprocess.Popen(
        rclone_command(config, "cat", remote),
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    assert process.stdout is not None
    value = process.stdout.read(limit + 1)
    if len(value) > limit:
        process.terminate()
    process.stdout.close()
    return_code = process.wait()
    if return_code != 0 or len(value) > limit:
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup metadata is unavailable or oversized")
    return value


def load_remote_manifest(config: Config, artifact: str) -> dict[str, object]:
    marker = remote_bytes(config, safe_remote(config, f"{artifact}.COMPLETED"), limit=128)
    if marker != b"AUTOMATIZA_BACKUP_COMPLETE_V1\n":
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup completion marker is invalid")
    try:
        manifest = json.loads(
            remote_bytes(
                config,
                safe_remote(config, f"{artifact}.manifest.json"),
                limit=64 * 1024,
            )
        )
    except json.JSONDecodeError as exc:
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup manifest is invalid") from exc
    if (
        not isinstance(manifest, dict)
        or manifest.get("schema_version") != SCHEMA_VERSION
        or manifest.get("artifact") != artifact
        or not isinstance(manifest.get("sha256"), str)
        or not re.fullmatch(r"[0-9a-f]{64}", str(manifest.get("sha256")))
        or not isinstance(manifest.get("bytes"), int)
        or int(manifest["bytes"]) < 0
    ):
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup manifest fields are invalid")
    return manifest


def verify_remote_artifact(config: Config, artifact: str) -> int:
    manifest = load_remote_manifest(config, artifact)
    process = subprocess.Popen(
        rclone_command(config, "cat", safe_remote(config, artifact)),
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    assert process.stdout is not None
    digest = hashlib.sha256()
    byte_count = 0
    for chunk in iter(lambda: process.stdout.read(1024 * 1024), b""):
        digest.update(chunk)
        byte_count += len(chunk)
    process.stdout.close()
    if process.wait() != 0:
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup artifact could not be read")
    if digest.hexdigest() != manifest["sha256"] or byte_count != manifest.get("bytes"):
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup artifact checksum does not match")
    return byte_count


def stream_encrypted_backup(config: Config, component: str, run_id: str) -> dict[str, object]:
    producer_cmd = producer_for(config, component)
    age = require_tool(config.get("BACKUP_AGE_BIN", "age"))
    recipient = pathlib.Path(config.require("BACKUP_AGE_RECIPIENT_FILE"))
    if not recipient.is_file():
        raise BackupError("BACKUP_CONFIG_INVALID", "age recipient file is unavailable")
    key = artifact_key(component, run_id)
    remote = safe_remote(config, key)
    digest = hashlib.sha256()
    byte_count = 0
    with tempfile.TemporaryFile() as producer_err, tempfile.TemporaryFile() as age_err, tempfile.TemporaryFile() as remote_err:
        producer = subprocess.Popen(producer_cmd, stdout=subprocess.PIPE, stderr=producer_err)
        assert producer.stdout is not None
        encryptor = subprocess.Popen(
            [age, "-R", str(recipient)],
            stdin=producer.stdout,
            stdout=subprocess.PIPE,
            stderr=age_err,
        )
        producer.stdout.close()
        assert encryptor.stdout is not None
        destination = subprocess.Popen(
            rclone_command(config, "rcat", remote),
            stdin=subprocess.PIPE,
            stdout=subprocess.DEVNULL,
            stderr=remote_err,
        )
        assert destination.stdin is not None
        try:
            while chunk := encryptor.stdout.read(1024 * 1024):
                digest.update(chunk)
                byte_count += len(chunk)
                destination.stdin.write(chunk)
        except BrokenPipeError:
            pass
        finally:
            destination.stdin.close()
            encryptor.stdout.close()
        producer_rc = producer.wait()
        age_rc = encryptor.wait()
        remote_rc = destination.wait()
        if producer_rc != 0:
            raise BackupError("BACKUP_PRODUCER_FAILED", _read_stderr(producer_err))
        if age_rc != 0:
            raise BackupError("BACKUP_PRODUCER_FAILED", _read_stderr(age_err))
        if remote_rc != 0:
            raise BackupError("BACKUP_DESTINATION_FAILED", _read_stderr(remote_err))
    manifest: dict[str, object] = {
        "schema_version": SCHEMA_VERSION,
        "component": component,
        "run_id": run_id,
        "created_at": utc_text(),
        "artifact": key,
        "bytes": byte_count,
        "sha256": digest.hexdigest(),
        "encryption": "age",
    }
    encoded = (json.dumps(manifest, sort_keys=True, separators=(",", ":")) + "\n").encode()
    upload_bytes(config, safe_remote(config, f"{key}.manifest.json"), encoded)
    upload_bytes(config, safe_remote(config, f"{key}.COMPLETED"), b"AUTOMATIZA_BACKUP_COMPLETE_V1\n")
    return manifest


def run_pgbackrest_backup(config: Config, component: str) -> dict[str, object]:
    backup_type = "full" if component == "postgres-full" else "diff"
    docker = require_tool(config.get("BACKUP_DOCKER_BIN", "docker"))
    container = config.get("BACKUP_PG_CONTAINER", "automatiza-postgres")
    stanza = config.get("BACKUP_PGBACKREST_STANZA", "automatiza")
    started = time.monotonic()
    run_checked([docker, "exec", container, "pgbackrest", f"--stanza={stanza}", f"--type={backup_type}", "backup"])
    info = run_checked([docker, "exec", container, "pgbackrest", f"--stanza={stanza}", "--output=json", "info"])
    repository_bytes = pgbackrest_repository_size(info.stdout)
    return {
        "schema_version": SCHEMA_VERSION,
        "component": component,
        "created_at": utc_text(),
        "duration_seconds": round(time.monotonic() - started, 3),
        "pgbackrest_info_sha256": hashlib.sha256(info.stdout).hexdigest(),
        "repository_bytes": repository_bytes,
    }


def verify_pgbackrest(config: Config) -> bool:
    docker = shutil.which(config.get("BACKUP_DOCKER_BIN", "docker"))
    if docker is None:
        return False
    result = subprocess.run(
        [
            docker,
            "exec",
            config.get("BACKUP_PG_CONTAINER", "automatiza-postgres"),
            "pgbackrest",
            f"--stanza={config.get('BACKUP_PGBACKREST_STANZA', 'automatiza')}",
            "verify",
        ],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    return result.returncode == 0


def write_status(config: Config, component: str, *, success: bool, run_id: str, **fields: object) -> None:
    previous = load_json(config.state_dir / "status" / f"{component}.json")
    value: dict[str, object] = {
        "schema_version": SCHEMA_VERSION,
        "component": component,
        "run_id": run_id,
        "finished_at": utc_text(),
        "success": success,
    }
    if success:
        value["last_success_at"] = value["finished_at"]
    elif previous.get("last_success_at"):
        value["last_success_at"] = previous["last_success_at"]
    value.update(fields)
    atomic_json(config.state_dir / "status" / f"{component}.json", value)


def backup(config: Config, component: str, execute: bool) -> None:
    if component not in COMPONENTS:
        raise BackupError("BACKUP_CONFIG_INVALID", f"unsupported component: {component}")
    run_id = uuid.uuid4().hex
    emit(
        "backup_planned",
        component=component,
        run_id=run_id,
        trace_id=run_id,
        execute=execute,
    )
    if not execute:
        return
    require_execute_guard(config)
    if component in {"postgres-full", "postgres-diff"} and not config.bool(
        "BACKUP_PITR_ENABLED"
    ):
        raise BackupError(
            "BACKUP_DISABLED",
            "physical PostgreSQL backups are disabled until PITR validation",
        )
    started = time.monotonic()
    lock_name = "postgres" if component.startswith("postgres-") else component
    with component_lock(config, lock_name):
        try:
            if component in {"postgres-full", "postgres-diff"}:
                manifest = run_pgbackrest_backup(config, component)
            else:
                manifest = stream_encrypted_backup(config, component, run_id)
            duration = round(time.monotonic() - started, 3)
            status_fields = {
                key: value
                for key, value in manifest.items()
                if key not in {"schema_version", "component", "run_id", "duration_seconds"}
            }
            write_status(
                config,
                component,
                success=True,
                run_id=run_id,
                duration_seconds=duration,
                **status_fields,
            )
            emit(
                "backup_succeeded",
                component=component,
                run_id=run_id,
                trace_id=run_id,
                duration_seconds=duration,
            )
        except BackupError as exc:
            outcome = "unknown" if exc.code == "BACKUP_DESTINATION_FAILED" else "failed"
            write_status(
                config,
                component,
                success=False,
                run_id=run_id,
                error_code=exc.code,
                outcome=outcome,
            )
            emit(
                "backup_failed",
                component=component,
                run_id=run_id,
                trace_id=run_id,
                error_code=exc.code,
                outcome=outcome,
            )
            raise


def filesystem_usage(path: pathlib.Path) -> tuple[float, float]:
    stat = os.statvfs(path)
    if stat.f_blocks <= 0 or stat.f_files <= 0:
        raise BackupError("BACKUP_CONFIG_INVALID", f"filesystem has invalid counters: {path}")
    disk = 1.0 - (stat.f_bavail / stat.f_blocks)
    inodes = 1.0 - (stat.f_favail / stat.f_files)
    return disk, inodes


def directory_size(path: pathlib.Path) -> int | None:
    try:
        if not path.is_dir():
            return None
        total = 0
        for root, _, files in os.walk(path, followlinks=False):
            for name in files:
                candidate = pathlib.Path(root) / name
                if not candidate.is_symlink():
                    total += candidate.stat().st_size
        return total
    except OSError:
        return None


def parse_time(value: object) -> float:
    if not isinstance(value, str):
        return 0.0
    try:
        return dt.datetime.fromisoformat(value.replace("Z", "+00:00")).timestamp()
    except ValueError:
        return 0.0


def collect_wal_age(config: Config) -> float | None:
    docker = shutil.which(config.get("BACKUP_DOCKER_BIN", "docker"))
    if docker is None:
        return None
    result = subprocess.run(
        [
            docker,
            "exec",
            config.get("BACKUP_PG_CONTAINER", "automatiza-postgres"),
            "psql",
            "-U",
            config.get("BACKUP_DB_USER", "automatiza_backup"),
            "-d",
            config.get("BACKUP_DB_NAME", "automatiza"),
            "-tA",
            "-c",
            "SELECT COALESCE(EXTRACT(EPOCH FROM now()-last_archived_time),-1)::bigint FROM pg_stat_archiver",
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        return None
    try:
        value = float(result.stdout.strip())
    except ValueError:
        return None
    return None if value < 0 else value


def metric_line(name: str, value: float | int, labels: Mapping[str, str] | None = None) -> str:
    suffix = ""
    if labels:
        safe = []
        for key, raw in sorted(labels.items()):
            escaped = raw.replace("\\", "\\\\").replace('"', '\\"').replace("\n", "\\n")
            safe.append(f'{key}="{escaped}"')
        suffix = "{" + ",".join(safe) + "}"
    return f"{name}{suffix} {value}"


def verify(
    config: Config,
    *,
    now: float | None = None,
    verify_remote: bool = False,
) -> list[dict[str, object]]:
    trace_id = uuid.uuid4().hex
    current = now if now is not None else time.time()
    alerts: list[dict[str, object]] = []
    metrics = [
        "# HELP automatiza_backup_last_success_unixtime Last successful backup timestamp.",
        "# TYPE automatiza_backup_last_success_unixtime gauge",
        metric_line("automatiza_backup_verifier_last_run_unixtime", int(current)),
    ]
    remote_metrics = [
        metric_line(
            "automatiza_backup_remote_verifier_last_run_unixtime",
            int(current),
        )
    ]
    pitr_enabled = config.bool("BACKUP_PITR_ENABLED")
    active_freshness = {
        component: max_age
        for component, max_age in FRESHNESS_SECONDS.items()
        if pitr_enabled or component not in {"postgres-full", "postgres-diff"}
    }
    metrics.append(
        metric_line(
            "automatiza_postgres_archiving_enabled",
            1 if pitr_enabled else 0,
        )
    )
    for component, max_age in sorted(active_freshness.items()):
        status = load_json(config.state_dir / "status" / f"{component}.json")
        last = parse_time(status.get("last_success_at"))
        metrics.append(metric_line("automatiza_backup_last_success_unixtime", last, {"component": component}))
        size = status.get("bytes", status.get("repository_bytes"))
        if isinstance(size, int):
            metrics.append(
                metric_line(
                    "automatiza_backup_last_size_bytes",
                    size,
                    {"component": component},
                )
            )
        if last <= 0 or current - last > max_age:
            alerts.append({"error_code": "VERIFY_BACKUP_STALE", "component": component})
        if verify_remote and component in {
            "postgres-logical",
            "waha-1",
            "waha-2",
            "vps-config",
        }:
            if isinstance(status.get("artifact"), str):
                try:
                    verified_bytes = verify_remote_artifact(config, str(status["artifact"]))
                    remote_metrics.append(
                        metric_line(
                            "automatiza_backup_last_verified_bytes",
                            verified_bytes,
                            {"component": component},
                        )
                    )
                    remote_metrics.append(
                        metric_line(
                            "automatiza_backup_artifact_verified",
                            1,
                            {"component": component},
                        )
                    )
                except BackupError:
                    alerts.append({"error_code": "VERIFY_ARTIFACT_INVALID", "component": component})
                    remote_metrics.append(
                        metric_line(
                            "automatiza_backup_artifact_verified",
                            0,
                            {"component": component},
                        )
                    )
            else:
                alerts.append({"error_code": "VERIFY_ARTIFACT_INVALID", "component": component})
                remote_metrics.append(
                    metric_line(
                        "automatiza_backup_artifact_verified",
                        0,
                        {"component": component},
                    )
                )
    if verify_remote and pitr_enabled:
        physical_verified = verify_pgbackrest(config)
        remote_metrics.append(
            metric_line(
                "automatiza_backup_artifact_verified",
                1 if physical_verified else 0,
                {"component": "postgres-physical"},
            )
        )
        if not physical_verified:
            alerts.append(
                {
                    "error_code": "VERIFY_ARTIFACT_INVALID",
                    "component": "postgres-physical",
                }
            )
    mounts = [pathlib.Path(item) for item in config.get("BACKUP_MONITORED_MOUNTS", "/").split(":") if item]
    for index, mount in enumerate(mounts):
        disk, inodes = filesystem_usage(mount)
        label = f"mount-{index}"
        metrics.append(metric_line("automatiza_disk_used_ratio", round(disk, 6), {"mount": label}))
        metrics.append(metric_line("automatiza_inode_used_ratio", round(inodes, 6), {"mount": label}))
        if disk >= 0.90:
            alerts.append({"error_code": "VERIFY_DISK_CRITICAL", "mount": label, "level": "critical"})
        elif disk >= 0.80:
            alerts.append({"error_code": "VERIFY_DISK_CRITICAL", "mount": label, "level": "high"})
        elif disk >= 0.70:
            alerts.append({"error_code": "VERIFY_DISK_CRITICAL", "mount": label, "level": "warning"})
        if inodes >= 0.90:
            alerts.append({"error_code": "VERIFY_INODE_CRITICAL", "mount": label, "level": "critical"})
        elif inodes >= 0.80:
            alerts.append({"error_code": "VERIFY_INODE_CRITICAL", "mount": label, "level": "high"})
        elif inodes >= 0.70:
            alerts.append({"error_code": "VERIFY_INODE_CRITICAL", "mount": label, "level": "warning"})
    wal_age = collect_wal_age(config) if pitr_enabled else None
    metrics.append(metric_line("automatiza_postgres_last_wal_age_seconds", wal_age if wal_age is not None else -1))
    if pitr_enabled and (wal_age is None or wal_age > 300):
        alerts.append(
            {
                "error_code": "VERIFY_WAL_STALE",
                "level": "critical" if wal_age is None or wal_age > 600 else "warning",
            }
        )
    wal_size: int | None = None
    if pitr_enabled:
        wal_size = directory_size(
            pathlib.Path(
                config.get(
                    "BACKUP_PG_WAL_PATH",
                    "/var/lib/docker/volumes/docker_postgres_data/_data/pg_wal",
                )
            )
        )
        if wal_size is None:
            alerts.append(
                {
                    "error_code": "VERIFY_WAL_CAPACITY",
                    "level": "critical",
                }
            )
        else:
            previous_verification = load_json(
                config.state_dir / "status" / "verification.json"
            )
            previous_wal_size = previous_verification.get(
                "wal_directory_bytes",
                wal_size,
            )
            if not isinstance(previous_wal_size, int):
                previous_wal_size = wal_size
            metrics.append(
                metric_line("automatiza_postgres_wal_directory_bytes", wal_size)
            )
            metrics.append(
                metric_line(
                    "automatiza_postgres_wal_growth_bytes",
                    wal_size - previous_wal_size,
                )
            )
            warn_bytes = config.integer(
                "BACKUP_PG_WAL_WARN_BYTES",
                10 * 1024 * 1024 * 1024,
            )
            critical_bytes = config.integer(
                "BACKUP_PG_WAL_CRITICAL_BYTES",
                20 * 1024 * 1024 * 1024,
            )
            if warn_bytes <= 0 or critical_bytes <= warn_bytes:
                raise BackupError(
                    "BACKUP_CONFIG_INVALID",
                    "WAL capacity thresholds are invalid",
                )
            if wal_size >= critical_bytes:
                alerts.append(
                    {
                        "error_code": "VERIFY_WAL_CAPACITY",
                        "level": "critical",
                    }
                )
            elif wal_size >= warn_bytes:
                alerts.append(
                    {
                        "error_code": "VERIFY_WAL_CAPACITY",
                        "level": "warning",
                    }
                )
    atomic_text(
        config.metrics_dir / "automatiza_backup.prom",
        "\n".join(metrics) + "\n",
    )
    if verify_remote:
        atomic_text(
            config.metrics_dir / "automatiza_backup_remote.prom",
            "\n".join(remote_metrics) + "\n",
        )
    verification_value: dict[str, object] = {
        "schema_version": SCHEMA_VERSION,
        "finished_at": utc_text(),
        "success": not alerts,
        "alerts": alerts,
    }
    if wal_size is not None:
        verification_value["wal_directory_bytes"] = wal_size
    atomic_json(
        config.state_dir
        / "status"
        / ("verification-remote.json" if verify_remote else "verification.json"),
        verification_value,
    )
    for alert in alerts:
        emit("verification_alert", trace_id=trace_id, **alert)
    return alerts


def atomic_text(path: pathlib.Path, value: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True, mode=0o750)
    fd, temp_name = tempfile.mkstemp(prefix=f".{path.name}.", dir=path.parent)
    temp_path = pathlib.Path(temp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(value)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temp_path, 0o640)
        os.replace(temp_path, path)
    finally:
        with contextlib.suppress(FileNotFoundError):
            temp_path.unlink()


def rclone_size(config: Config, remote: str) -> tuple[int, int]:
    remote = validate_rclone_target(remote)
    result = subprocess.run(
        rclone_command(config, "size", remote, "--json"),
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise BackupError("BACKUP_DESTINATION_FAILED", "destination inventory failed")
    try:
        value = json.loads(result.stdout)
        return int(value["bytes"]), int(value["count"])
    except (json.JSONDecodeError, KeyError, TypeError, ValueError) as exc:
        raise BackupError("BACKUP_DESTINATION_FAILED", "destination inventory is invalid") from exc


def postgres_relation_sizes(config: Config) -> dict[str, int]:
    tables = [
        item.strip()
        for item in config.get(
            "BACKUP_MONITORED_TABLES",
            "public.action_history,public.dispatch_logs,public.webhook_logs",
        ).split(",")
        if item.strip()
    ]
    if len(tables) > 20 or any(
        not re.fullmatch(r"[a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*", item)
        for item in tables
    ):
        raise BackupError("BACKUP_CONFIG_INVALID", "monitored table allowlist is invalid")
    if not tables:
        return {}
    sql = " UNION ALL ".join(
        f"SELECT '{table}', pg_total_relation_size(to_regclass('{table}'))"
        for table in tables
    )
    docker = require_tool(config.get("BACKUP_DOCKER_BIN", "docker"))
    result = subprocess.run(
        [
            docker,
            "exec",
            config.get("BACKUP_PG_CONTAINER", "automatiza-postgres"),
            "psql",
            "-U",
            config.get("BACKUP_DB_USER", "automatiza_backup"),
            "-d",
            config.get("BACKUP_DB_NAME", "automatiza"),
            "-tA",
            "-F",
            "\t",
            "-c",
            sql,
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if result.returncode != 0:
        raise BackupError("BACKUP_PRODUCER_FAILED", "PostgreSQL size inventory failed")
    sizes: dict[str, int] = {}
    try:
        for line in result.stdout.splitlines():
            table, raw_size = line.split("\t", 1)
            sizes[table] = int(raw_size)
    except (ValueError, TypeError) as exc:
        raise BackupError("BACKUP_PRODUCER_FAILED", "PostgreSQL size inventory is invalid") from exc
    return sizes


def inventory(config: Config) -> list[dict[str, object]]:
    trace_id = uuid.uuid4().hex
    alerts: list[dict[str, object]] = []
    metrics = [
        "# HELP automatiza_backup_destination_bytes Bytes stored below the configured destination.",
        "# TYPE automatiza_backup_destination_bytes gauge",
        metric_line("automatiza_backup_inventory_last_run_unixtime", int(time.time())),
    ]
    budget = config.integer("BACKUP_DESTINATION_BUDGET_BYTES")
    if budget <= 0:
        raise BackupError("BACKUP_LOW_CAPACITY", "destination capacity budget must be positive")
    destination = validate_rclone_target(config.require("BACKUP_RCLONE_REMOTE").rstrip("/"))
    destination_bytes, destination_objects = rclone_size(config, destination)
    ratio = destination_bytes / budget
    metrics.extend(
        [
            metric_line("automatiza_backup_destination_bytes", destination_bytes),
            metric_line("automatiza_backup_destination_objects", destination_objects),
            metric_line("automatiza_backup_destination_budget_used_ratio", round(ratio, 6)),
        ]
    )
    if ratio >= 0.90:
        alerts.append({"error_code": "VERIFY_DESTINATION_CAPACITY", "level": "critical"})
    elif ratio >= 0.80:
        alerts.append({"error_code": "VERIFY_DESTINATION_CAPACITY", "level": "high"})
    elif ratio >= 0.70:
        alerts.append({"error_code": "VERIFY_DESTINATION_CAPACITY", "level": "warning"})

    prefixes = [
        item.strip()
        for item in config.get("BACKUP_MONITORED_S3_PREFIXES", "").split(",")
        if item.strip()
    ]
    if len(prefixes) > 20:
        raise BackupError("BACKUP_CONFIG_INVALID", "too many monitored object prefixes")
    current: dict[str, object] = {
        "schema_version": SCHEMA_VERSION,
        "finished_at": utc_text(),
        "success": True,
        "alerts": alerts,
        "prefixes": {},
        "tables": postgres_relation_sizes(config),
    }
    previous = load_json(config.state_dir / "status" / "inventory.json")
    previous_prefixes = previous.get("prefixes", {}) if isinstance(previous.get("prefixes"), dict) else {}
    previous_tables = previous.get("tables", {}) if isinstance(previous.get("tables"), dict) else {}
    for index, prefix in enumerate(prefixes):
        size, count = rclone_size(config, prefix)
        label = f"prefix-{index}"
        current_prefixes = current["prefixes"]
        assert isinstance(current_prefixes, dict)
        current_prefixes[label] = {"bytes": size, "objects": count}
        old = previous_prefixes.get(label, {})
        old_size = int(old.get("bytes", size)) if isinstance(old, dict) else size
        metrics.append(metric_line("automatiza_s3_prefix_bytes", size, {"prefix": label}))
        metrics.append(metric_line("automatiza_s3_prefix_growth_bytes_daily", size - old_size, {"prefix": label}))
    current_tables = current["tables"]
    assert isinstance(current_tables, dict)
    for table, size in current_tables.items():
        old_size = int(previous_tables.get(table, size))
        metrics.append(metric_line("automatiza_postgres_table_bytes", size, {"table": table}))
        metrics.append(metric_line("automatiza_postgres_table_growth_bytes_daily", size - old_size, {"table": table}))
    atomic_text(
        config.metrics_dir / "automatiza_capacity.prom",
        "\n".join(metrics) + "\n",
    )
    for alert in alerts:
        emit("inventory_alert", trace_id=trace_id, **alert)
    current["success"] = not alerts
    current["alerts"] = alerts
    atomic_json(config.state_dir / "status" / "inventory.json", current)
    return alerts


def require_restore_guard(config: Config, target: pathlib.Path) -> pathlib.Path:
    if config.get("BACKUP_MODE") != "staging":
        raise BackupError("RESTORE_GUARD_REJECTED", "restore is allowed only in staging mode")
    sentinel = pathlib.Path(config.get("RESTORE_ENABLE_SENTINEL", "/etc/automatiza-backup/RESTORE_ALLOWED"))
    try:
        content = sentinel.read_text(encoding="utf-8").strip()
    except FileNotFoundError as exc:
        raise BackupError("RESTORE_GUARD_REJECTED", "restore sentinel is absent") from exc
    if content != "AUTOMATIZA_STAGING_RESTORE_V1":
        raise BackupError("RESTORE_GUARD_REJECTED", "restore sentinel is invalid")
    resolved = target.resolve()
    allowed_root = pathlib.Path(
        config.get("RESTORE_ALLOWED_ROOT", "/var/lib/automatiza-restore-drill")
    ).resolve()
    forbidden = {pathlib.Path("/"), pathlib.Path("/var/lib/docker"), pathlib.Path("/opt/automatiza")}
    if (
        resolved == allowed_root
        or allowed_root not in resolved.parents
        or resolved in forbidden
        or any(parent in forbidden - {pathlib.Path("/")} for parent in resolved.parents)
    ):
        raise BackupError("RESTORE_GUARD_REJECTED", "restore target is a protected path")
    resolved.mkdir(parents=True, exist_ok=True, mode=0o750)
    if any(resolved.iterdir()):
        raise BackupError("RESTORE_TARGET_NOT_EMPTY", "restore target must be empty")
    return resolved


def safe_extract_stream(stream: BinaryIO, target: pathlib.Path) -> None:
    root = target.resolve()
    try:
        with tarfile.open(fileobj=stream, mode="r|*") as archive:
            for member in archive:
                if member.issym() or member.islnk() or member.isdev():
                    raise BackupError(
                        "RESTORE_GUARD_REJECTED",
                        "archive contains an unsupported special entry",
                    )
                destination = (root / member.name).resolve()
                if root != destination and root not in destination.parents:
                    raise BackupError(
                        "RESTORE_GUARD_REJECTED",
                        "archive entry escapes restore target",
                    )
                if member.isdir():
                    destination.mkdir(parents=True, exist_ok=True)
                    continue
                if not member.isfile():
                    continue
                destination.parent.mkdir(parents=True, exist_ok=True)
                source = archive.extractfile(member)
                if source is None:
                    raise BackupError(
                        "BACKUP_CHECKSUM_MISMATCH",
                        "archive member could not be read",
                    )
                with source, destination.open("wb") as output:
                    shutil.copyfileobj(source, output, length=1024 * 1024)
                os.chmod(destination, member.mode & 0o700)
    except BackupError:
        raise
    except (OSError, tarfile.TarError) as exc:
        raise BackupError(
            "BACKUP_CHECKSUM_MISMATCH",
            "archive could not be safely extracted",
        ) from exc


def restore_archive(config: Config, artifact: str, target: pathlib.Path, execute: bool) -> None:
    resolved = require_restore_guard(config, target)
    trace_id = uuid.uuid4().hex
    emit("restore_planned", component="archive", trace_id=trace_id, execute=execute)
    if not execute:
        return
    verify_remote_artifact(config, artifact)
    age = require_tool(config.get("BACKUP_AGE_BIN", "age"))
    identity = pathlib.Path(config.require("RESTORE_AGE_IDENTITY_FILE"))
    if not identity.is_file():
        raise BackupError("BACKUP_CONFIG_INVALID", "restore identity file is unavailable")
    source = safe_remote(config, artifact)
    remote = subprocess.Popen(
        rclone_command(config, "cat", source),
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    assert remote.stdout is not None
    decrypt = subprocess.Popen([age, "-d", "-i", str(identity)], stdin=remote.stdout, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    remote.stdout.close()
    assert decrypt.stdout is not None
    try:
        safe_extract_stream(decrypt.stdout, resolved)
    except BaseException:
        decrypt.stdout.close()
        decrypt.terminate()
        remote.terminate()
        decrypt.wait()
        remote.wait()
        raise
    decrypt.stdout.close()
    decrypt_rc = decrypt.wait()
    remote_rc = remote.wait()
    if remote_rc != 0 or decrypt_rc != 0:
        raise BackupError("BACKUP_CHECKSUM_MISMATCH", "restore stream failed verification")
    emit("restore_succeeded", component="archive", trace_id=trace_id)


def validate_restore_counts(output: str, min_profiles: int, min_instances: int) -> None:
    if min_profiles <= 0 or min_instances <= 0:
        raise BackupError(
            "BACKUP_CONFIG_INVALID",
            "restore minimum row-count invariants must be positive",
        )
    try:
        profiles, instances = (int(item) for item in output.strip().split("\t", 1))
    except (TypeError, ValueError) as exc:
        raise BackupError(
            "RESTORE_DATABASE_FAILED",
            "restored row-count smoke output is invalid",
        ) from exc
    if profiles < min_profiles or instances < min_instances:
        raise BackupError(
            "RESTORE_DATABASE_FAILED",
            "restored database does not meet minimum row-count invariants",
        )


def restore_dump_to_container(
    config: Config,
    docker: str,
    dump: pathlib.Path,
) -> None:
    container = config.require("RESTORE_PG_CONTAINER")
    database = config.get("RESTORE_DB_NAME", "automatiza_restore")
    user = config.get("RESTORE_DB_USER", "automatiza")
    container_dump = f"/tmp/automatiza-restore-{uuid.uuid4().hex}.dump"
    copied = False
    try:
        empty_check = subprocess.run(
            [
                docker,
                "exec",
                container,
                "psql",
                "-U",
                user,
                "-d",
                database,
                "-tA",
                "-c",
                "SELECT count(*) FROM pg_catalog.pg_tables WHERE schemaname='public'",
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        if empty_check.returncode != 0:
            raise BackupError(
                "RESTORE_DATABASE_FAILED",
                "temporary restore database is unavailable",
            )
        if empty_check.stdout.strip() != "0":
            raise BackupError(
                "RESTORE_TARGET_NOT_EMPTY",
                "temporary restore database must be empty",
            )
        if subprocess.run(
            [docker, "cp", str(dump), f"{container}:{container_dump}"],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        ).returncode != 0:
            raise BackupError(
                "RESTORE_DATABASE_FAILED",
                "backup could not be copied to restore container",
            )
        copied = True
        restore_result = subprocess.run(
            [
                docker,
                "exec",
                container,
                "pg_restore",
                "--exit-on-error",
                "--no-owner",
                "--no-privileges",
                "-U",
                user,
                "-d",
                database,
                container_dump,
            ],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
            check=False,
        )
        if restore_result.returncode != 0:
            raise BackupError(
                "RESTORE_DATABASE_FAILED",
                "pg_restore failed in temporary database",
            )
        smoke = subprocess.run(
            [
                docker,
                "exec",
                container,
                "psql",
                "-U",
                user,
                "-d",
                database,
                "-tA",
                "-F",
                "\t",
                "-c",
                (
                    "SELECT (SELECT count(*) FROM public.profiles), "
                    "(SELECT count(*) FROM public.instances)"
                ),
            ],
            capture_output=True,
            text=True,
            check=False,
        )
        if smoke.returncode != 0:
            raise BackupError(
                "RESTORE_DATABASE_FAILED",
                "restored row-count smoke query failed",
            )
        validate_restore_counts(
            smoke.stdout,
            config.integer("RESTORE_MIN_PROFILES", 1),
            config.integer("RESTORE_MIN_INSTANCES", 1),
        )
    finally:
        if copied:
            cleanup = subprocess.run(
                [docker, "exec", container, "rm", "-f", "--", container_dump],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
                check=False,
            )
            if cleanup.returncode != 0:
                raise BackupError(
                    "RESTORE_DATABASE_FAILED",
                    "temporary container dump cleanup failed",
                )


def restore_postgres_logical(
    config: Config,
    artifact: str,
    target: pathlib.Path,
    execute: bool,
) -> None:
    resolved = require_restore_guard(config, target)
    trace_id = uuid.uuid4().hex
    emit(
        "restore_planned",
        component="postgres-logical",
        trace_id=trace_id,
        execute=execute,
    )
    if not execute:
        return
    if not artifact.startswith("postgres-logical/") or not artifact.endswith(".dump.age"):
        raise BackupError("RESTORE_GUARD_REJECTED", "artifact is not a PostgreSQL logical backup")
    manifest = load_remote_manifest(config, artifact)
    age = require_tool(config.get("BACKUP_AGE_BIN", "age"))
    docker = require_tool(config.get("BACKUP_DOCKER_BIN", "docker"))
    identity = pathlib.Path(config.require("RESTORE_AGE_IDENTITY_FILE"))
    if not identity.is_file():
        raise BackupError("BACKUP_CONFIG_INVALID", "restore identity file is unavailable")
    encrypted = resolved / "postgres.dump.age"
    dump = resolved / "postgres.dump"
    copy_result = subprocess.run(
        rclone_command(
            config,
            "copyto",
            safe_remote(config, artifact),
            str(encrypted),
        ),
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if copy_result.returncode != 0:
        raise BackupError("VERIFY_ARTIFACT_INVALID", "backup artifact could not be downloaded")
    digest = hashlib.sha256()
    with encrypted.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    if digest.hexdigest() != manifest["sha256"] or encrypted.stat().st_size != manifest.get("bytes"):
        raise BackupError("BACKUP_CHECKSUM_MISMATCH", "downloaded backup checksum does not match")
    decrypt_result = subprocess.run(
        [age, "-d", "-i", str(identity), "-o", str(dump), str(encrypted)],
        stdout=subprocess.DEVNULL,
        stderr=subprocess.DEVNULL,
        check=False,
    )
    if decrypt_result.returncode != 0:
        if not config.bool("RESTORE_KEEP_PLAINTEXT"):
            try:
                dump.unlink(missing_ok=True)
                encrypted.unlink(missing_ok=True)
            except OSError as exc:
                raise BackupError(
                    "RESTORE_DATABASE_FAILED",
                    "failed decrypt left restore material that requires cleanup",
                ) from exc
        raise BackupError("BACKUP_CHECKSUM_MISMATCH", "PostgreSQL backup could not be decrypted")
    os.chmod(dump, 0o600)
    try:
        restore_dump_to_container(config, docker, dump)
    finally:
        if not config.bool("RESTORE_KEEP_PLAINTEXT"):
            cleanup_failed = False
            for path in (dump, encrypted):
                try:
                    path.unlink(missing_ok=True)
                except OSError:
                    cleanup_failed = True
            if cleanup_failed:
                raise BackupError(
                    "RESTORE_DATABASE_FAILED",
                    "local restore material cleanup failed",
                )
    atomic_json(
        config.state_dir / "status" / "restore-postgres-logical.json",
        {
            "schema_version": SCHEMA_VERSION,
            "artifact": artifact,
            "finished_at": utc_text(),
            "success": True,
        },
    )
    emit("restore_succeeded", component="postgres-logical", trace_id=trace_id)


def preflight(config: Config) -> None:
    for tool in (
        "tar",
        config.get("BACKUP_DOCKER_BIN", "docker"),
        config.get("BACKUP_RCLONE_BIN", "rclone"),
        config.get("BACKUP_AGE_BIN", "age"),
    ):
        require_tool(tool)
    rclone_command(config, "version")
    safe_remote(config, "_preflight")
    for key in ("BACKUP_AGE_RECIPIENT_FILE", "BACKUP_CONFIG_ALLOWLIST"):
        if not pathlib.Path(config.require(key)).is_file():
            raise BackupError("BACKUP_CONFIG_INVALID", f"required file is unavailable: {key}")
    docker = require_tool(config.get("BACKUP_DOCKER_BIN", "docker"))
    role_check = subprocess.run(
        [
            docker,
            "exec",
            config.get("BACKUP_PG_CONTAINER", "automatiza-postgres"),
            "psql",
            "-U",
            config.get("BACKUP_DB_USER", "automatiza_backup"),
            "-d",
            config.get("BACKUP_DB_NAME", "automatiza"),
            "-tA",
            "-c",
            (
                "SELECT current_setting('transaction_read_only'), "
                "pg_has_role(current_user,'pg_read_all_data','member')::int"
            ),
        ],
        capture_output=True,
        text=True,
        check=False,
    )
    if role_check.returncode != 0 or role_check.stdout.strip() != "on|1":
        raise BackupError(
            "BACKUP_CONFIG_INVALID",
            "dedicated PostgreSQL backup role is unavailable or not read-only",
        )
    budget = config.integer("BACKUP_DESTINATION_BUDGET_BYTES")
    if budget <= 0:
        raise BackupError("BACKUP_LOW_CAPACITY", "destination capacity budget must be positive")
    emit("preflight_succeeded", mode=config.get("BACKUP_MODE", "disabled"))


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", type=pathlib.Path, default=pathlib.Path("/etc/automatiza-backup/config.env"))
    sub = parser.add_subparsers(dest="command", required=True)
    sub.add_parser("preflight")
    backup_parser = sub.add_parser("backup")
    backup_parser.add_argument("component", choices=sorted(COMPONENTS))
    backup_parser.add_argument("--execute", action="store_true")
    verify_parser = sub.add_parser("verify")
    verify_parser.add_argument("--allow-alerts", action="store_true")
    verify_parser.add_argument("--remote", action="store_true")
    inventory_parser = sub.add_parser("inventory")
    inventory_parser.add_argument("--allow-alerts", action="store_true")
    restore_parser = sub.add_parser("restore-archive")
    restore_parser.add_argument("--artifact", required=True)
    restore_parser.add_argument("--target", type=pathlib.Path, required=True)
    restore_parser.add_argument("--execute", action="store_true")
    restore_pg_parser = sub.add_parser("restore-postgres-logical")
    restore_pg_parser.add_argument("--artifact", required=True)
    restore_pg_parser.add_argument("--target", type=pathlib.Path, required=True)
    restore_pg_parser.add_argument("--execute", action="store_true")
    return parser


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        config = Config.load(args.config)
        if args.command == "preflight":
            preflight(config)
        elif args.command == "backup":
            backup(config, args.component, args.execute)
        elif args.command == "verify":
            alerts = verify(config, verify_remote=args.remote)
            if alerts and not args.allow_alerts:
                return 2
        elif args.command == "inventory":
            alerts = inventory(config)
            if alerts and not args.allow_alerts:
                return 2
        elif args.command == "restore-archive":
            restore_archive(config, args.artifact, args.target, args.execute)
        elif args.command == "restore-postgres-logical":
            restore_postgres_logical(
                config,
                args.artifact,
                args.target,
                args.execute,
            )
        return 0
    except BackupError as exc:
        emit("command_failed", error_code=exc.code)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
