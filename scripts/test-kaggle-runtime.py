#!/usr/bin/env python3
"""Smoke the shipped bridge in its production container isolation profile."""
import argparse
import json
import queue
import subprocess
import threading
import time
import uuid


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--image", required=True)
    parser.add_argument("--platform", default="linux/amd64")
    args = parser.parse_args()
    name = "aurago-kaggle-smoke-" + uuid.uuid4().hex
    command = ["docker", "run", "--rm", "-i", "--name", name, "--platform", args.platform,
               "--network", "none", "--read-only", "--user", "1000:100", "--cpus", "2",
               "--memory", "4g", "--pids-limit", "256", "--cap-drop", "ALL",
               "--security-opt", "no-new-privileges:true", "--workdir", "/workspace",
               "--tmpfs", "/tmp:rw,nosuid,nodev,size=268435456,mode=1777",
               "--tmpfs", "/workspace:rw,nosuid,nodev,size=268435456,uid=1000,gid=100,mode=0700",
               "-e", "HOME=/tmp/home", "-e", "JUPYTER_RUNTIME_DIR=/tmp/runtime", "-e", "IPYTHONDIR=/tmp/ipython",
               "--entrypoint", "python", args.image, "/opt/aurago/bridge.py"]
    process = subprocess.Popen(command, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                               stderr=subprocess.DEVNULL, text=True, encoding="utf-8", bufsize=1)
    events = queue.Queue(maxsize=256)

    def read():
        try:
            for line in process.stdout:
                if len(line) > 1024 * 1024 + 1:
                    raise AssertionError("bridge frame exceeds limit")
                events.put(json.loads(line), timeout=5)
        except Exception as error:
            events.put(error)
        finally:
            events.put(None)

    threading.Thread(target=read, daemon=True).start()

    def wait(kind, execution=None, timeout=120):
        received = []
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            event = events.get(timeout=max(0.1, deadline - time.monotonic()))
            assert isinstance(event, dict), "bridge stopped before completion"
            assert event["protocol"] == 1
            if execution is not None and event.get("execution_id") != execution:
                continue
            received.append(event)
            if event["kind"] == kind:
                return received
        raise AssertionError("bridge timed out")

    def send(command, code="", execution=""):
        process.stdin.write(json.dumps({"command": command, "code": code, "execution_id": execution,
                                       "revision": "smoke", "cell_id": execution, "timeout_seconds": 60}) + "\n")
        process.stdin.flush()

    try:
        wait("ready", timeout=240)
        send("execute", "answer = 40", "one")
        wait("done", "one")
        send("execute", "print(answer + 2)", "two")
        assert any("42" in str(e.get("content", {})) for e in wait("done", "two")), "variable state lost"
        send("execute", "import os; assert not any('KAGGLE' in k or 'AURAGO_MASTER' in k for k in os.environ); print('environment-ok')", "env")
        assert any("environment-ok" in str(e.get("content", {})) for e in wait("done", "env")), "kernel environment failed"
        send("execute", "import time; time.sleep(60)", "interrupt")
        time.sleep(1)
        send("interrupt", execution="interrupt")
        assert any("KeyboardInterrupt" in str(e.get("content", {})) for e in wait("done", "interrupt")), "interrupt failed"
        send("execute", "print('x' * (5 * 1024 * 1024))", "limit")
        assert any(e["kind"] == "truncated" for e in wait("done", "limit")), "output bound missing"
        send("shutdown")
        process.stdin.close()
        assert process.wait(timeout=30) == 0, "bridge shutdown failed"
        print(f"Kaggle notebook bridge smoke passed: {args.platform}")
    finally:
        subprocess.run(["docker", "rm", "-f", name], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=30)
        if process.poll() is None:
            process.kill()
            process.wait(timeout=10)


if __name__ == "__main__":
    main()
