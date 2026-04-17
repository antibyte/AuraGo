import os
import sys
import time

sys.path.insert(0, os.path.dirname(__file__))
from aurago_daemon import AuraGoDaemon

def {{.FunctionName}}(daemon, target, threshold, interval, alert_severity):
    """{{.Description}}"""
    threshold = float(threshold)
    interval = int(interval)

    while daemon.is_running():
        try:
            value = None

            if target == "disk":
                st = os.statvfs("/")
                used_pct = (1 - st.f_bavail / st.f_blocks) * 100
                value = round(used_pct, 1)
                label = f"Disk usage: {value}%"

            elif target == "cpu":
                # Simple /proc/stat-based CPU check (Linux)
                try:
                    with open("/proc/stat") as f:
                        a = [int(x) for x in f.readline().split()[1:]]
                    time.sleep(1)
                    with open("/proc/stat") as f:
                        b = [int(x) for x in f.readline().split()[1:]]
                    da = sum(b) - sum(a)
                    idle = (b[3] - a[3])
                    value = round((1 - idle / da) * 100, 1) if da > 0 else 0.0
                except FileNotFoundError:
                    value = 0.0
                label = f"CPU usage: {value}%"

            elif target == "url":
                import urllib.request
                url = os.environ.get("AURAGO_SECRET_MONITOR_URL", "http://localhost")
                try:
                    r = urllib.request.urlopen(url, timeout=10)
                    value = r.getcode()
                    label = f"URL {url} status: {value}"
                except Exception as e:
                    value = 999
                    label = f"URL {url} unreachable: {e}"

            else:
                label = f"Unknown target: {target}"
                daemon.log(label, level="warn")
                time.sleep(interval)
                continue

            daemon.metric(f"{target}_value", value)

            if target == "url":
                exceeded = value != 200
            else:
                exceeded = value >= threshold

            if exceeded:
                daemon.wake_agent(
                    f"Threshold exceeded — {label} (threshold: {threshold})",
                    severity=alert_severity,
                    target=target,
                    value=value,
                    threshold=threshold,
                )
            else:
                daemon.log(f"OK — {label}", level="debug")

        except Exception as e:
            daemon.log(f"Monitor error: {e}", level="error")

        daemon.heartbeat()
        time.sleep(interval)


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
        target=args.get("target", "disk"),
        threshold=args.get("threshold", 90),
        interval=args.get("interval", 60),
        alert_severity=args.get("alert_severity", "warning"),
    )
