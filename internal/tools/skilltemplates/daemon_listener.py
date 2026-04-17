import os
import sys
import json
import socket
import threading
import time

sys.path.insert(0, os.path.dirname(__file__))
from aurago_daemon import AuraGoDaemon

def {{.FunctionName}}(daemon, socket_path, protocol, max_clients):
    """{{.Description}}"""
    max_clients = int(max_clients)

    # Clean up stale socket file
    if os.path.exists(socket_path):
        try:
            os.remove(socket_path)
        except OSError:
            pass

    srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
    srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    srv.settimeout(2.0)
    srv.bind(socket_path)
    srv.listen(max_clients)
    os.chmod(socket_path, 0o660)

    daemon.log(f"Listening on {socket_path} (protocol={protocol})", level="info")

    active_threads = []

    def handle_client(conn, addr):
        try:
            buf = b""
            conn.settimeout(5.0)
            while daemon.is_running():
                try:
                    data = conn.recv(4096)
                except socket.timeout:
                    continue
                if not data:
                    break
                buf += data
                while b"\n" in buf:
                    line, buf = buf.split(b"\n", 1)
                    text = line.decode("utf-8", errors="replace").strip()
                    if not text:
                        continue
                    if protocol == "json":
                        try:
                            payload = json.loads(text)
                        except json.JSONDecodeError:
                            payload = {"raw": text}
                    else:
                        payload = {"raw": text}

                    message = payload.get("message", text) if isinstance(payload, dict) else text
                    severity = payload.get("severity", "info") if isinstance(payload, dict) else "info"
                    daemon.wake_agent(
                        message=str(message),
                        severity=severity,
                        source="socket",
                        socket_path=socket_path,
                        payload=payload,
                    )
        except Exception as e:
            daemon.log(f"Client handler error: {e}", level="error")
        finally:
            conn.close()

    try:
        while daemon.is_running():
            # Clean up finished threads
            active_threads = [t for t in active_threads if t.is_alive()]
            try:
                conn, addr = srv.accept()
                if len(active_threads) >= max_clients:
                    conn.close()
                    daemon.log("Max clients reached, rejecting connection", level="warn")
                    continue
                t = threading.Thread(target=handle_client, args=(conn, addr), daemon=True)
                t.start()
                active_threads.append(t)
            except socket.timeout:
                pass
            daemon.heartbeat()
    finally:
        srv.close()
        try:
            os.remove(socket_path)
        except OSError:
            pass


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
        socket_path=args.get("socket_path", "/tmp/aurago_daemon.sock"),
        protocol=args.get("protocol", "json"),
        max_clients=args.get("max_clients", 5),
    )
