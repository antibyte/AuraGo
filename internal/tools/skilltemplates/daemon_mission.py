import os
import sys
import time
import json
import glob

sys.path.insert(0, os.path.dirname(__file__))
from aurago_daemon import AuraGoDaemon

def {{.FunctionName}}(daemon, watch_dir, status_file, backup_pattern, check_interval, cooldown):
    """{{.Description}}

    Monitor a backup directory or status file and emit events that can trigger
    a mission. Configure the daemon with a trigger_mission_id in the Web UI
    to automatically run a follow-up mission when backup events occur.
    """
    check_interval = int(check_interval)
    cooldown = int(cooldown)
    last_alert = 0

    def get_backup_status():
        """Read backup status from status file or detect latest backup."""
        result = {"status": "unknown", "message": "", "timestamp": None}

        if status_file and os.path.isfile(status_file):
            try:
                with open(status_file) as f:
                    data = json.load(f)
                result["status"] = data.get("status", "unknown")
                result["message"] = data.get("message", "")
                result["timestamp"] = data.get("timestamp")
                return result
            except (json.JSONDecodeError, IOError) as e:
                return {"status": "error", "message": str(e), "timestamp": None}

        if watch_dir and os.path.isdir(watch_dir):
            pattern = backup_pattern or "*.backup"
            files = glob.glob(os.path.join(watch_dir, pattern))
            if files:
                latest = max(files, key=os.path.getmtime)
                mtime = os.path.getmtime(latest)
                result["status"] = "completed"
                result["message"] = f"Latest backup: {os.path.basename(latest)}"
                result["timestamp"] = time.strftime("%Y-%m-%dT%H:%M:%S", time.localtime(mtime))
                result["file"] = latest
                result["size_bytes"] = os.path.getsize(latest)
                return result

        return result

    daemon.log(f"Monitoring backup: watch_dir={watch_dir}, status_file={status_file}", level="info")

    while daemon.is_running():
        try:
            status = get_backup_status()

            if status["status"] == "error":
                daemon.wake_agent(
                    f"Backup status unreadable: {status['message']}",
                    severity="warning",
                    backup_status=status["status"],
                    watch_dir=watch_dir,
                    status_file=status_file,
                )

            elif status["status"] == "failed":
                daemon.wake_agent(
                    f"Backup failed: {status['message']}",
                    severity="critical",
                    backup_status=status["status"],
                    message=status["message"],
                    timestamp=status["timestamp"],
                    watch_dir=watch_dir,
                    status_file=status_file,
                )

            elif status["status"] == "completed":
                now = time.time()
                if now - last_alert >= cooldown:
                    last_alert = now
                    daemon.wake_agent(
                        f"Backup completed: {status['message']}",
                        severity="info",
                        backup_status=status["status"],
                        message=status["message"],
                        timestamp=status["timestamp"],
                        file=status.get("file", ""),
                        size_bytes=status.get("size_bytes", 0),
                        watch_dir=watch_dir,
                    )
                else:
                    daemon.log(f"Backup OK (cooldown active): {status['message']}", level="debug")

            else:
                daemon.log(f"Backup status: {status['status']} — {status['message']}", level="debug")

        except Exception as e:
            daemon.log(f"Backup monitor error: {e}", level="error")

        daemon.heartbeat()
        time.sleep(check_interval)


if __name__ == "__main__":
    daemon = AuraGoDaemon()
    args = {}
    if len(sys.argv) > 1:
        try:
            args = json.loads(sys.argv[1])
        except Exception:
            pass
    {{.FunctionName}}(
        daemon,
        watch_dir=args.get("watch_dir", "/var/backups"),
        status_file=args.get("status_file", ""),
        backup_pattern=args.get("backup_pattern", "*.backup"),
        check_interval=args.get("check_interval", 60),
        cooldown=args.get("cooldown", 300),
    )
