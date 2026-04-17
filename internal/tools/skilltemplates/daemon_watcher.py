import os
import sys
import time
import hashlib

sys.path.insert(0, os.path.dirname(__file__))
from aurago_daemon import AuraGoDaemon

def {{.FunctionName}}(daemon, watch_path, patterns, events, cooldown, recursive):
    """{{.Description}}"""
    import fnmatch

    cooldown = int(cooldown)
    recursive = str(recursive).lower() in ("true", "1", "yes")
    pattern_list = [p.strip() for p in patterns.split(",") if p.strip()] if patterns else []
    event_set = set(e.strip().lower() for e in events.split(",") if e.strip()) if events else {"created", "modified", "deleted"}

    def matches_pattern(name):
        if not pattern_list:
            return True
        return any(fnmatch.fnmatch(name, p) for p in pattern_list)

    def scan_dir(path):
        result = {}
        try:
            if recursive:
                for root, dirs, files in os.walk(path):
                    for f in files:
                        fp = os.path.join(root, f)
                        try:
                            st = os.stat(fp)
                            result[fp] = (st.st_mtime, st.st_size)
                        except OSError:
                            pass
            else:
                for f in os.listdir(path):
                    fp = os.path.join(path, f)
                    if os.path.isfile(fp):
                        try:
                            st = os.stat(fp)
                            result[fp] = (st.st_mtime, st.st_size)
                        except OSError:
                            pass
        except OSError as e:
            daemon.log(f"Scan error: {e}", level="error")
        return result

    last_alert = {}
    prev_snapshot = scan_dir(watch_path)
    daemon.log(f"Watching {watch_path} — {len(prev_snapshot)} files, patterns={pattern_list}", level="info")

    while daemon.is_running():
        time.sleep(5)
        curr_snapshot = scan_dir(watch_path)
        now = time.time()

        # Detect created files
        if "created" in event_set:
            for fp in curr_snapshot:
                if fp not in prev_snapshot and matches_pattern(os.path.basename(fp)):
                    if now - last_alert.get(fp, 0) >= cooldown:
                        last_alert[fp] = now
                        daemon.wake_agent(
                            f"File created: {fp}",
                            severity="info",
                            event="created",
                            path=fp,
                        )

        # Detect modified files
        if "modified" in event_set:
            for fp in curr_snapshot:
                if fp in prev_snapshot and curr_snapshot[fp] != prev_snapshot[fp] and matches_pattern(os.path.basename(fp)):
                    if now - last_alert.get(fp, 0) >= cooldown:
                        last_alert[fp] = now
                        daemon.wake_agent(
                            f"File modified: {fp}",
                            severity="info",
                            event="modified",
                            path=fp,
                        )

        # Detect deleted files
        if "deleted" in event_set:
            for fp in prev_snapshot:
                if fp not in curr_snapshot and matches_pattern(os.path.basename(fp)):
                    if now - last_alert.get(fp, 0) >= cooldown:
                        last_alert[fp] = now
                        daemon.wake_agent(
                            f"File deleted: {fp}",
                            severity="warning",
                            event="deleted",
                            path=fp,
                        )

        prev_snapshot = curr_snapshot
        daemon.heartbeat()


if __name__ == "__main__":
    daemon = AuraGoDaemon()
    args = {}
    if len(sys.argv) > 1:
        import json
        try:
            args = json.loads(sys.argv[1])
        except Exception:
            pass
    {{.FunctionName}}(
        daemon,
        watch_path=args.get("watch_path", "."),
        patterns=args.get("patterns", ""),
        events=args.get("events", "created,modified,deleted"),
        cooldown=args.get("cooldown", 10),
        recursive=args.get("recursive", True),
    )
