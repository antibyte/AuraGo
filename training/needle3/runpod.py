"""RunPod request preparation and an independent, pod-specific shutdown guard.

This module never rents a pod or creates/deletes a network volume.
"""
from __future__ import annotations

import argparse
from datetime import datetime, timezone
import json
import math
import os
from pathlib import Path
import re
import time
import urllib.error
import urllib.request

from .common import HOME, canonical, config, file_digest, read_json, verify_files, write_json

IMAGE = "nvidia/cuda@sha256:24c8e3581ea6330038b0d374920721983312627f8adbfcf390bdb4b399d280ed"
API = "https://rest.runpod.io/v1"


def launch_request(pack, volume_id, data_center, quote, run_id):
    cfg = config()["runpod"]
    manifest = read_json(Path(pack) / "manifest.json")
    if not cfg["launch_enabled"]:
        raise ValueError("RunPod launch is disabled until the user's later start instruction")
    if not manifest.get("ready_for_gpu"):
        raise ValueError("dataset is incomplete; no pod may be rented")
    verify_files(pack, manifest["files"])
    if not 0 < quote["gpu_hourly_usd"] <= cfg["max_gpu_hourly_usd"] or not 0 <= time.time() - quote["observed_at_unix"] <= 300:
        raise ValueError("fresh price at or below $1/GPU-hour is required")
    if quote.get("gpu") != cfg["gpu"] or quote.get("cloud") != "SECURE" or quote.get("spot", True):
        raise ValueError("quote must be Secure Cloud on-demand RTX 4090")
    if not all(re.fullmatch(r"[A-Za-z0-9_-]+", value or "") for value in (volume_id, data_center, run_id)):
        raise ValueError("actual network-volume, data-center and run IDs are required")
    return {"name": "aurago-needle3-" + run_id, "image": IMAGE, "cloud": "SECURE", "disk": 20,
            "dataCenterIds": [data_center], "gpu": {"id": cfg["gpu"], "count": 1, "minRamPerGpu": 32, "minVcpuCountPerGpu": 6, "minCudaVersion": "12.8"},
            "mounts": {"network": [{"path": "/workspace", "volumeId": volume_id}]},
            "env": {"AURAGO_NEEDLE_RUN_ID": run_id, "DO_NOT_TRACK": "1", "HF_HUB_DISABLE_TELEMETRY": "1"},
            "args": "sleep infinity", "startSsh": True, "startJupyter": False, "ports": ["22/tcp"]}


def validate_lease(lease, pod, now):
    if lease.get("created_pod_id") != lease.get("pod_id") or pod.get("id") != lease.get("pod_id"):
        raise ValueError("pod does not match the creation receipt")
    if pod.get("name") != "aurago-needle3-" + lease["run_id"]:
        raise ValueError("pod ownership marker differs")
    volume = pod.get("networkVolumeId") or (pod.get("networkVolume") or {}).get("id")
    if not lease.get("network_volume_id") or volume != lease["network_volume_id"]:
        raise ValueError("separate retained network volume is not confirmed")
    created = datetime.fromisoformat(pod["createdAt"].replace("Z", "+00:00")).timestamp()
    if abs(created - lease["created_at_unix"]) > 10 or created > now + 10:
        raise ValueError("lease creation timestamp differs from RunPod")
    if not math.isfinite(created) or lease.get("maximum_seconds") != 43200 or not 0 < lease.get("gpu_hourly_usd", 2) <= 1:
        raise ValueError("lease exceeds the approved time or price limit")
    return created + lease["maximum_seconds"]


def api_request(method, pod_id):
    if not re.fullmatch(r"[A-Za-z0-9_-]+", pod_id):
        raise ValueError("invalid pod ID")
    key = os.environ.get("RUNPOD_API_KEY")
    if not key:
        raise ValueError("RUNPOD_API_KEY is required by the independent guard")
    request = urllib.request.Request(API + "/pods/" + pod_id, method=method, headers={"Authorization": "Bearer " + key})
    with urllib.request.urlopen(request, timeout=20) as response:
        body = response.read()
        return json.loads(body) if body else None


def watchdog(path):
    lease = read_json(path)
    pod = api_request("GET", lease["pod_id"])
    deadline = validate_lease(lease, pod, time.time())
    lease["watchdog_armed"] = True
    lease["watchdog_pid"] = os.getpid()
    lease["watchdog_heartbeat_unix"] = time.time()
    write_json(path, lease)
    # Start termination three minutes before the twelve-hour ceiling to leave
    # room for transient API failures. This runs outside the training process.
    while time.time() < deadline - 180:
        if Path(str(path) + ".complete").exists() or Path(str(path) + ".failed").exists():
            break
        lease["watchdog_heartbeat_unix"] = time.time()
        write_json(path, lease)
        time.sleep(min(10, max(0, deadline - 180 - time.time())))
    while True:
        try:
            current = api_request("GET", lease["pod_id"])
            validate_lease(lease, current, time.time())
            api_request("DELETE", lease["pod_id"])
        except urllib.error.HTTPError as exc:
            if exc.code == 404:
                write_json(str(path) + ".terminated.json", {"pod_id": lease["pod_id"], "confirmed_absent_at_unix": time.time(), "retained_network_volume_id": lease["network_volume_id"]})
                return
            print("RunPod shutdown API error; retrying", flush=True)
        except (urllib.error.URLError, TimeoutError):
            print("RunPod shutdown network error; retrying", flush=True)
        time.sleep(5)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest="action", required=True)
    plan = sub.add_parser("request")
    for name in ("pack", "volume-id", "data-center", "quote", "run-id", "out"):
        plan.add_argument("--" + name, required=True)
    watch = sub.add_parser("watchdog")
    watch.add_argument("--lease", required=True)
    args = parser.parse_args()
    if args.action == "watchdog":
        watchdog(args.lease)
    else:
        write_json(args.out, launch_request(args.pack, args.volume_id, args.data_center, read_json(args.quote), args.run_id))


if __name__ == "__main__":
    main()
