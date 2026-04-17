import sys
import json
import os
import glob
import hashlib
import tarfile
import tempfile
from datetime import datetime

BACKUP_DIR = os.environ.get("AURAGO_BACKUP_DIR", os.path.join(os.getcwd(), "backups"))

def _generate_backup_name(source):
    name = os.path.basename(os.path.normpath(source))
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
    return f"{name}_{timestamp}.tar.gz"

def _create_backup(source, output, exclude_patterns=None):
    if not os.path.exists(source):
        return {"status": "error", "message": f"Source not found: {source}"}

    os.makedirs(BACKUP_DIR, exist_ok=True)
    if not output:
        output = os.path.join(BACKUP_DIR, _generate_backup_name(source))

    exclude_set = set()
    if exclude_patterns:
        for pat in [p.strip() for p in exclude_patterns.split(",") if p.strip()]:
            for match in glob.glob(os.path.join(source, pat), recursive=True):
                exclude_set.add(os.path.normpath(match))

    source = os.path.normpath(source)
    source_name = os.path.basename(source)
    file_count = 0
    total_size = 0

    def tar_filter(info):
        norm = os.path.normpath(info.name)
        for exc in exclude_set:
            if norm.startswith(exc):
                return None
        return info

    with tarfile.open(output, "w:gz") as tar:
        tar.add(source, arcname=source_name, filter=tar_filter)

    archive_size = os.path.getsize(output)
    with open(output, "rb") as f:
        sha256 = hashlib.sha256(f.read()).hexdigest()

    return {
        "status": "success",
        "result": {
            "archive": output,
            "size_bytes": archive_size,
            "size_human": f"{archive_size / 1024 / 1024:.1f} MB" if archive_size > 1024 * 1024 else f"{archive_size / 1024:.1f} KB",
            "sha256": sha256,
            "created_at": datetime.now().isoformat(),
        },
    }

def _list_backups():
    if not os.path.exists(BACKUP_DIR):
        return {"status": "success", "result": {"backups": [], "total": 0}}
    backups = []
    for f in sorted(glob.glob(os.path.join(BACKUP_DIR, "*.tar.gz")), reverse=True):
        stat = os.stat(f)
        backups.append({
            "file": os.path.basename(f),
            "size_bytes": stat.st_size,
            "size_human": f"{stat.st_size / 1024 / 1024:.1f} MB" if stat.st_size > 1024 * 1024 else f"{stat.st_size / 1024:.1f} KB",
            "modified": datetime.fromtimestamp(stat.st_mtime).isoformat(),
        })
    return {"status": "success", "result": {"backups": backups, "total": len(backups)}}

def _restore_backup(source, output):
    if not os.path.exists(source):
        return {"status": "error", "message": f"Archive not found: {source}"}
    if not output:
        output = os.getcwd()
    os.makedirs(output, exist_ok=True)
    abs_output = os.path.abspath(output)
    with tarfile.open(source, "r:gz") as tar:
        # Zip-slip / path traversal protection
        for member in tar.getmembers():
            member_path = os.path.abspath(os.path.join(abs_output, member.name))
            if not member_path.startswith(abs_output + os.sep) and member_path != abs_output:
                return {"status": "error", "message": f"Path traversal detected in archive member: {member.name}"}
        tar.extractall(path=output)
    return {"status": "success", "result": f"Restored to {output}"}

def _cleanup_backups(keep):
    keep = int(keep)
    if not os.path.exists(BACKUP_DIR):
        return {"status": "success", "result": {"removed": 0, "kept": 0}}
    backups = sorted(glob.glob(os.path.join(BACKUP_DIR, "*.tar.gz")), reverse=True)
    kept = backups[:keep]
    removed = backups[keep:]
    for f in removed:
        os.remove(f)
    return {"status": "success", "result": {"removed": len(removed), "kept": len(kept)}}

def {{.FunctionName}}(action="create", source="", output=None, keep=5, exclude=None):
    """{{.Description}}"""
    try:
        if action == "create":
            if not source:
                return {"status": "error", "message": "Source path is required for create action"}
            return _create_backup(source, output, exclude)
        elif action == "list":
            return _list_backups()
        elif action == "restore":
            if not source:
                return {"status": "error", "message": "Archive path is required for restore action"}
            return _restore_backup(source, output)
        elif action == "cleanup":
            return _cleanup_backups(keep)
        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: create, list, restore, cleanup"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
