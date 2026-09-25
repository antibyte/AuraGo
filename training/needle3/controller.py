"""Run inside an explicitly started pod; bound every phase to the creation lease."""
from __future__ import annotations

import argparse
from collections import defaultdict
import os
from pathlib import Path
import signal
import statistics
import subprocess
import sys
import time

from .common import canonical, config, file_digest, read_json, read_jsonl, verify_files, write_json


def run_phase(command, deadline, log_path, lease_path=None, checkpoint_dir=None):
    remaining = deadline - time.time()
    if remaining <= 30:
        raise TimeoutError("phase deadline reached before launch")
    with Path(log_path).open("a", encoding="utf-8") as log:
        process = subprocess.Popen(command, stdout=log, stderr=subprocess.STDOUT, start_new_session=True)
        launched = time.time()
        try:
            while process.poll() is None:
                if time.time() >= deadline - 15:
                    raise TimeoutError("phase time budget exhausted")
                if lease_path and time.time() - read_json(lease_path).get("watchdog_heartbeat_unix", 0) > 60:
                    raise TimeoutError("independent watchdog heartbeat lost")
                if checkpoint_dir:
                    latest = Path(checkpoint_dir) / "latest.json"
                    saved_at = latest.stat().st_mtime if latest.exists() else launched
                    if time.time() - max(launched, saved_at) > 580:
                        raise TimeoutError("no fresh checkpoint within ten-minute limit")
                time.sleep(1)
            if process.returncode:
                raise RuntimeError(f"phase failed with exit code {process.returncode}; inspect {log_path}")
        finally:
            if process.poll() is None:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait()


def gpu_preflight():
    import jax
    if sys.platform != "linux" or sys.version_info[:2] != (3, 12):
        raise ValueError("RunPod requires the locked Linux/Python 3.12 environment")
    fields = subprocess.check_output(["nvidia-smi", "--query-gpu=name,memory.total", "--format=csv,noheader,nounits"], text=True).strip().splitlines()
    if len(fields) != 1 or "RTX 4090" not in fields[0] or int(fields[0].split(",")[-1]) < 24000:
        raise ValueError("expected exactly one 24GB RTX 4090")
    memory_kb = int(next(line.split()[1] for line in Path("/proc/meminfo").read_text().splitlines() if line.startswith("MemTotal:")))
    if (os.cpu_count() or 0) < 6 or memory_kb < 30 * 1024 * 1024 or jax.default_backend() != "gpu":
        raise ValueError("CPU, RAM or CUDA backend does not meet the reservation")
    return {"gpu": fields, "cpus": os.cpu_count(), "memory_kb": memory_kb, "jax": jax.__version__, "backend": jax.default_backend()}


def run(args):
    out, pack = Path(args.out), Path(args.pack)
    out.mkdir(parents=True, exist_ok=True)
    lease = read_json(args.lease)
    if not lease.get("watchdog_armed") or time.time() - lease.get("watchdog_heartbeat_unix", 0) > 60:
        raise ValueError("fresh independent watchdog handshake required before any training")
    manifest = read_json(pack / "manifest.json")
    if not manifest.get("ready_for_gpu"):
        raise ValueError("data preparation is incomplete")
    verify_files(pack, manifest["files"])
    start = lease["created_at_unix"]
    write_json(out / "hardware.json", gpu_preflight())
    baseline = out / "models" / "untrained-w4.cact"
    run_phase([sys.executable, "-m", "training.needle3.export", "--out", str(baseline)], start + 1800, out / "native-export.log", args.lease)
    run_phase([sys.executable, "-m", "training.needle3.smoke", "--weights", str(baseline), "--out", str(out / "native-linux-smoke.json")], start + 1800, out / "native-smoke.log", args.lease)
    base = [sys.executable, "-m", "training.needle3.train", "--pack", str(pack), "--lease", args.lease]
    probe_steps, micro = 8, 4
    probe = out / "throughput"
    while True:
        try:
            run_phase(base + ["--arm", "pilot_multilingual", "--microbatch", str(micro), "--max-steps", str(probe_steps), "--max-seconds", "1500", "--out", str(probe)], start + 1800, out / "throughput.log", args.lease, probe / "checkpoints")
            break
        except RuntimeError:
            text = (out / "throughput.log").read_text(encoding="utf-8", errors="replace").lower()
            if micro == 1 or not any(term in text for term in ("out of memory", "resource_exhausted", "failed to allocate")):
                raise
            micro //= 2
    sample = read_json(probe / "training-summary.json")["seconds_per_step"]
    if len(sample) < 2:
        raise ValueError("insufficient throughput measurements")
    seconds = statistics.median(sample[1:]) * 1.25
    pilot_steps = min(config()["train"]["pilot_steps"], int(max(0, start + 7200 - time.time() - 600) / (seconds * 2)))
    if pilot_steps < 20:
        raise ValueError("setup/throughput leaves insufficient time for comparable pilots")
    write_json(out / "throughput-plan.json", {"microbatch": micro, "pilot_steps": pilot_steps, "conservative_seconds_per_step": seconds})
    for arm in ("pilot_de_en", "pilot_multilingual"):
        run_phase(base + ["--arm", arm, "--microbatch", str(micro), "--max-steps", str(pilot_steps), "--max-seconds", "2700", "--out", str(out / arm)], start + 7200, out / (arm + ".log"), args.lease, out / arm / "checkpoints")
    pilot_summaries = [read_json(out / arm / "training-summary.json") for arm in ("pilot_de_en", "pilot_multilingual")]
    if any(summary["steps"] != pilot_steps for summary in pilot_summaries):
        raise ValueError("pilot arms did not reach identical steps; main run blocked")
    main_steps = int(max(0, start + 32400 - time.time() - 600) / seconds)
    if main_steps < 2:
        raise ValueError("no main-training time remains")
    run_phase(base + ["--microbatch", str(micro), "--max-steps", str(main_steps), "--out", str(out / "main")], start + 32400, out / "main.log", args.lease, out / "main" / "checkpoints")
    # A separate process makes export/evaluation interruptible at the lease boundary.
    run_phase([sys.executable, "-m", "training.needle3.finalize", "--pack", str(pack), "--runs", str(out), "--deadline", str(start + 41400)], start + 41400, out / "finalize.log", args.lease)
    # Every useful result is already on the retained network volume. Download
    # verification happens locally; this controller never deletes that volume.
    result_files = {p.relative_to(out).as_posix(): file_digest(p) for p in out.rglob("*") if p.is_file() and p.name != "results-manifest.json"}
    write_json(out / "results-manifest.json", {"files": result_files, "pod_id": lease["pod_id"], "network_volume_id": lease["network_volume_id"], "ready_for_local_download": True})
    write_json(args.lease + ".complete", {"results_manifest_sha256": file_digest(out / "results-manifest.json")})


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("pack", "lease", "out"):
        parser.add_argument("--" + name, required=True)
    args = parser.parse_args()
    try:
        run(args)
    except BaseException as exc:
        write_json(args.lease + ".failed", {"error_type": type(exc).__name__, "preserve_network_volume": True})
        raise
