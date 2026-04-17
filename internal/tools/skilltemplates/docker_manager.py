import sys
import json
import os
import requests

DOCKER_SOCKET = os.environ.get("DOCKER_HOST", "/var/run/docker.sock")

def _docker_api(method, path, params=None, timeout=30):
    base = "http://localhost"
    if DOCKER_SOCKET.startswith("unix://") or DOCKER_SOCKET.startswith("/"):
        base = "http+docker://localhost"
    try:
        url = f"http://localhost{path}"
        resp = requests.request(
            method, url,
            params=params,
            timeout=timeout,
            headers={"Content-Type": "application/json"},
        )
    except Exception:
        import http.client
        import urllib.parse
        if DOCKER_SOCKET.startswith("/"):
            sock_path = DOCKER_SOCKET
        else:
            sock_path = DOCKER_SOCKET.replace("unix://", "")
        conn = http.client.HTTPConnection("localhost")
        try:
            conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            conn.sock.connect(sock_path)
        except Exception:
            import socket
            conn.sock = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
            conn.sock.connect(sock_path)
        qs = urllib.parse.urlencode(params) if params else ""
        full_path = f"{path}?{qs}" if qs else path
        conn.request(method, full_path)
        resp_raw = conn.getresponse()
        body = resp_raw.read().decode("utf-8")
        conn.close()
        try:
            data = json.loads(body)
        except ValueError:
            data = body
        return data

    try:
        return resp.json()
    except ValueError:
        return resp.text

def {{.FunctionName}}(action="list", container=None, tail=100, all=False):
    """{{.Description}}"""
    import subprocess

    try:
        if action == "list":
            cmd = ["docker", "ps"]
            if all:
                cmd.append("-a")
            result = subprocess.run(cmd, capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            lines = result.stdout.strip().split("\n")
            containers = []
            if len(lines) > 1:
                header = lines[0]
                for line in lines[1:]:
                    containers.append({"raw": line.strip()})
            return {"status": "success", "result": {"count": len(containers), "containers": containers}}

        elif action == "inspect":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "inspect", container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            data = json.loads(result.stdout)
            return {"status": "success", "result": f"<external_data>{json.dumps(data, ensure_ascii=False)}</external_data>"}

        elif action in ("start", "stop", "restart"):
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", action, container], capture_output=True, text=True, timeout=30)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            return {"status": "success", "result": f"Container {container}: {action} completed"}

        elif action == "logs":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "logs", "--tail", str(tail), container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            return {"status": "success", "result": result.stdout[-5000:]}

        elif action == "stats":
            if not container:
                return {"status": "error", "message": "Container name or ID required"}
            result = subprocess.run(["docker", "stats", "--no-stream", container], capture_output=True, text=True, timeout=15)
            if result.returncode != 0:
                return {"status": "error", "message": result.stderr.strip()}
            data = {"raw": result.stdout.strip()}
            return {"status": "success", "result": data}

        else:
            return {"status": "error", "message": f"Unknown action: {action}. Use: list, inspect, start, stop, restart, logs, stats"}

    except subprocess.TimeoutExpired:
        return {"status": "error", "message": f"Docker command timed out"}
    except FileNotFoundError:
        return {"status": "error", "message": "Docker CLI not found. Is Docker installed?"}
    except Exception as e:
        return {"status": "error", "message": str(e)}
