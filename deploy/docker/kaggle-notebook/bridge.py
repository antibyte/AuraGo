"""AuraGo notebook protocol v1. No HTTP listener, credentials or shell commands."""
import json
import os
import queue
import sys
import threading
import time

from jupyter_client import KernelManager

MAX_FRAME = 1024 * 1024
MAX_OUTPUT = 4 * MAX_FRAME
write_lock = threading.Lock()
pending = queue.Queue(maxsize=1)
stopping = threading.Event()
manager = KernelManager(kernel_name="python3", autorestart=False)
for directory in (os.environ["HOME"], os.environ["JUPYTER_RUNTIME_DIR"], os.environ["IPYTHONDIR"]):
    os.makedirs(directory, exist_ok=True)
manager.start_kernel(cwd="/workspace")
client = manager.client()
client.start_channels()
client.wait_for_ready(timeout=30)


def emit(kind, request=None, **fields):
    message = {"protocol": 1, "kind": kind, **fields}
    if request:
        for key in ("execution_id", "revision", "cell_id"):
            message[key] = request.get(key, "")
    encoded = json.dumps(message, ensure_ascii=True, separators=(",", ":"))
    if len(encoded) > MAX_FRAME:
        encoded = json.dumps({"protocol": 1, "kind": "truncated", "execution_id": (request or {}).get("execution_id", "")})
    with write_lock:
        sys.stdout.write(encoded + "\n")
        sys.stdout.flush()


def worker():
    while not stopping.is_set():
        try:
            request = pending.get(timeout=0.2)
        except queue.Empty:
            continue
        try:
            if request["command"] == "complete":
                mid = client.complete(request.get("code", ""), request.get("cursor", 0))
                deadline = time.monotonic() + 10
                while time.monotonic() < deadline:
                    reply = client.get_shell_msg(timeout=1)
                    if reply.get("parent_header", {}).get("msg_id") == mid:
                        emit("completion", request, content=reply["content"])
                        break
                continue
            mid = client.execute(request["code"], silent=False, store_history=True, allow_stdin=False, stop_on_error=True)
            used = 0
            truncated = False
            deadline = time.monotonic() + min(max(request.get("timeout_seconds", 1800), 1), 43200)
            while not stopping.is_set():
                if time.monotonic() >= deadline:
                    manager.interrupt_kernel()
                    emit("timeout", request)
                    deadline = float("inf")
                try:
                    message = client.get_iopub_msg(timeout=0.25)
                except queue.Empty:
                    continue
                if message.get("parent_header", {}).get("msg_id") != mid:
                    continue
                kind, content = message["msg_type"], message["content"]
                if kind == "status" and content.get("execution_state") == "idle":
                    emit("done", request)
                    break
                if kind not in ("stream", "display_data", "update_display_data", "execute_result", "error", "execute_input", "clear_output"):
                    continue
                used += len(json.dumps(content))
                if used > MAX_OUTPUT:
                    if not truncated:
                        emit("truncated", request)
                        truncated = True
                    continue
                emit("output", request, output_type=kind, content=content)
        except Exception:
            emit("error", request, message="Kernel communication failed")
        finally:
            pending.task_done()


threading.Thread(target=worker, daemon=True).start()
emit("ready")
try:
    while True:
        line = sys.stdin.buffer.readline(MAX_FRAME + 1)
        if not line:
            break
        if len(line) > MAX_FRAME or not line.endswith(b"\n"):
            emit("error", message="Input frame exceeds limit")
            break
        try:
            request = json.loads(line)
            command = request.get("command")
            if command == "interrupt":
                manager.interrupt_kernel()
                emit("interrupted", request)
            elif command == "shutdown":
                break
            elif command in ("execute", "complete"):
                pending.put_nowait(request)
            else:
                emit("error", request, message="Unsupported bridge command")
        except (ValueError, queue.Full, AttributeError):
            emit("error", message="Invalid request or kernel busy")
finally:
    stopping.set()
    manager.shutdown_kernel(now=True)
    client.stop_channels()
