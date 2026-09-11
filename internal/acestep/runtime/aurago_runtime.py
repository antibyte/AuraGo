"""Private ACE-Step bootstrap. Only pinned models and the managed API are used."""
import asyncio
from contextlib import asynccontextmanager
from dataclasses import asdict
import hashlib
import hmac
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import threading
from urllib.request import Request, urlopen

ROOT = Path("/app")
MODELS = ROOT / "checkpoints"
CACHE = ROOT / ".cache"
RELEASE = json.loads(Path("/opt/aurago/release.json").read_text()) if Path("/opt/aurago/release.json").exists() else {}
STATE = {"ready": False, "state": "starting", "error_code": "", "downloaded_bytes": 0, "total_bytes": 0}
GPU_CONFIG = None
PROFILE = None
PROFILE_CACHE = None
LOCK = threading.Lock()

def memory_gb():
    mem = dict(line.split(":", 1) for line in Path("/proc/meminfo").read_text().splitlines())
    available = int(mem["MemAvailable"].split()[0]) * 1024
    for limit_path, usage_path in [("/sys/fs/cgroup/memory.max", "/sys/fs/cgroup/memory.current"),
                                    ("/sys/fs/cgroup/memory/memory.limit_in_bytes", "/sys/fs/cgroup/memory/memory.usage_in_bytes")]:
        try:
            limit = int(Path(limit_path).read_text())
            available = min(available, max(0, limit - int(Path(usage_path).read_text())))
        except (OSError, ValueError): pass
    return available / 2**30

def render_nodes(backend):
    vendor = {"rocm": "0x1002", "xpu": "0x8086"}.get(backend)
    nodes = []
    for node in sorted(Path("/dev/dri").glob("renderD*")):
        try:
            if (Path("/sys/class/drm") / node.name / "device/vendor").read_text().strip() == vendor:
                nodes.append(str(node))
        except OSError: pass
    return nodes

def device_groups():
    groups = set()
    for node in [*Path("/dev/dri").glob("renderD*"), Path("/dev/kfd")]:
        try:
            if node.stat().st_gid: groups.add(str(node.stat().st_gid))
        except OSError: pass
    return sorted(groups)

def probe():
    import torch
    backend = os.environ["AURAGO_BACKEND"]
    devices = []
    groups = device_groups()
    if backend == "cpu":
        devices.append({"id": "cpu", "name": "CPU", "backend": backend, "index": 0,
                        "free_gb": memory_gb(), "total_gb": int(Path('/proc/meminfo').read_text().split('MemTotal:')[1].split()[0]) / 2**20, "verified": True,
                        "groups": [], "render_nodes": []})
    else:
        runtime = torch.xpu if backend == "xpu" else torch.cuda
        if backend == "rocm" and not torch.version.hip: return {"devices": [], "groups": groups}
        if backend == "cuda" and not torch.version.cuda: return {"devices": [], "groups": groups}
        if runtime.is_available():
            for index in range(runtime.device_count()):
                props = runtime.get_device_properties(index)
                device = f"{'xpu' if backend == 'xpu' else 'cuda'}:{index}"
                try:
                    # A real operation catches missing kernels/driver passthrough.
                    t = torch.ones((16, 16), device=device)
                    assert (t @ t).sum().item() == 4096
                    del t
                    runtime.synchronize(index)
                    free, total = runtime.mem_get_info(index)
                    devices.append({"id": f"{backend}:{index}", "name": props.name,
                                    "backend": backend, "index": index, "total_gb": total / 2**30,
                                    "free_gb": free / 2**30, "verified": True,
                                    "driver": str(torch.version.hip or torch.version.cuda or torch.__version__),
                                    "render_nodes": render_nodes(backend), "groups": groups})
                except (RuntimeError, AssertionError): continue
    return {"devices": devices, "groups": groups}

def choose_profile(device, reserve, conservative=False):
    from acestep.gpu_config import compute_adaptive_config, get_gpu_config
    backend = device["backend"]
    budget = max(0, device["free_gb"] - reserve)
    if backend != "cpu" and budget < 4: raise RuntimeError("acestep_insufficient_vram")
    model = "acestep-v15-xl-turbo" if budget >= 20 and not conservative and backend != "cpu" else "acestep-v15-turbo"
    gpu = compute_adaptive_config(budget, "xl_turbo" if "xl" in model else "turbo")
    if conservative: gpu = get_gpu_config(min(budget, 6))
    lm = gpu.recommended_lm_model if gpu.init_lm_default else ""
    if backend == "cpu": lm = ""
    lm_backend = gpu.recommended_backend if backend == "cuda" else "pt"
    # CPU and non-CUDA runtimes use tested PyTorch paths; CUDA-only kernels
    # must not be selected merely because a VRAM tier recommends them.
    compile_model = bool(gpu.compile_model_default and backend == "cuda")
    quantization = "int8_weight_only" if gpu.quantization_default and compile_model else ""
    if backend in ("rocm", "xpu") and budget < 12:
        lm = ""  # Full precision needs more headroom than CUDA INT8.
    if backend == "xpu":
        lm_backend = "pt"
    profile = {"device": device, "model": model, "lm_model": lm, "lm_backend": lm_backend,
               "max_duration": min(600, gpu.max_duration_with_lm if lm else gpu.max_duration_without_lm),
               "offload": bool(backend != "cpu" and (gpu.offload_to_cpu_default or conservative)),
               "offload_dit": bool(backend != "cpu" and (gpu.offload_dit_to_cpu_default or conservative)),
               "compile": compile_model, "quantization": quantization,
               "flash_attention": False, "ram_gb": memory_gb(), "disk_gb": shutil.disk_usage(MODELS).free / 2**30}
    if profile["ram_gb"] < (12 if backend == "cpu" or profile["offload"] else 4):
        raise RuntimeError("acestep_insufficient_ram")
    # Memory availability changes while serving; hardware identity and applied
    # model settings, rather than fluctuating free bytes, identify a profile.
    identity = {k: v for k, v in profile.items() if k not in ("device", "ram_gb", "disk_gb")}
    identity["device"] = {k: device[k] for k in ("id", "name", "backend", "driver", "total_gb") if k in device}
    identity["release"] = RELEASE
    identity["image"] = os.getenv("AURAGO_IMAGE_PIN", "")
    profile["fingerprint"] = hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest()
    return gpu, profile

def selected_gpu_config():
    return GPU_CONFIG

def file_valid(path, entry):
    if not path.is_file() or path.is_symlink() or path.stat().st_size != entry["size"]: return False
    digest = hashlib.sha256()
    with path.open("rb") as src:
        for block in iter(lambda: src.read(4 << 20), b""): digest.update(block)
    return digest.hexdigest() == entry["sha256"]

def download_file(entry, target):
    target.parent.mkdir(parents=True, exist_ok=True)
    if file_valid(target, entry): return
    part = target.with_name(target.name + ".part")
    if 'runtime_path' in entry:
        source=ROOT/entry['runtime_path']
        if not source.resolve().is_relative_to((ROOT/'acestep/models').resolve()) or not file_valid(source,entry):
            raise RuntimeError('acestep_runtime_code_mismatch')
        shutil.copyfile(source,part)
        part.replace(target)
        with LOCK: STATE['downloaded_bytes']+=entry['size']
        return
    offset = part.stat().st_size if part.exists() else 0
    if offset > entry["size"]: part.unlink(); offset = 0
    if offset == entry["size"]:
        if file_valid(part, entry): part.replace(target); return
        part.unlink(); offset = 0
    headers = {"User-Agent": "AuraGo-ACE-Step"}
    if offset: headers["Range"] = f"bytes={offset}-"
    with urlopen(Request(entry["url"], headers=headers), timeout=90) as response:
        if offset and response.status == 206:
            if not response.headers.get("Content-Range", "").startswith(f"bytes {offset}-"):
                raise RuntimeError("acestep_invalid_download_range")
        else: offset = 0
        with part.open("ab" if offset else "wb") as dest:
            size = offset
            while block := response.read(1 << 20):
                size += len(block)
                if size > entry["size"]: raise RuntimeError("acestep_model_size_mismatch")
                dest.write(block)
                with LOCK: STATE["downloaded_bytes"] += len(block)
            dest.flush(); os.fsync(dest.fileno())
    if not file_valid(part, entry): raise RuntimeError("acestep_model_hash_mismatch")
    part.replace(target)

def ensure_model(model_name, checkpoint_dir):
    if Path(checkpoint_dir).resolve() != MODELS.resolve() or model_name not in RELEASE["models"]:
        raise RuntimeError("acestep_unpinned_model_rejected")
    model_dir = MODELS / model_name
    if model_dir.is_symlink() or not model_dir.resolve().is_relative_to(MODELS.resolve()):
        raise RuntimeError("acestep_invalid_model_path")
    for entry in RELEASE["models"][model_name]["files"]:
        target = model_dir / entry["path"]
        if not target.resolve().is_relative_to(model_dir.resolve()): raise RuntimeError("acestep_invalid_model_path")
        download_file(entry, target)
    return str(model_dir)

def initialize(app):
    try:
        with LOCK: STATE["state"] = "downloading"
        names = ["vae", "Qwen3-Embedding-0.6B", PROFILE["model"]]
        if PROFILE["lm_model"]: names.append(PROFILE["lm_model"])
        files = [(MODELS / name / f["path"], f) for name in names for f in RELEASE["models"][name]["files"]]
        missing = sum(f["size"] for p, f in files if not file_valid(p, f))
        if shutil.disk_usage(MODELS).free < missing + (1 << 30): raise RuntimeError("acestep_insufficient_disk")
        with LOCK: STATE["total_bytes"] = missing
        for name in names: ensure_model(name, str(MODELS))
        with LOCK: STATE["state"] = "loading"
        from acestep.api.startup_model_init import do_model_initialization
        do_model_initialization(app=app, **app.state._model_init_kwargs)
        if not app.state._initialized or bool(PROFILE["lm_model"]) != app.state._llm_initialized:
            if "out of memory" in str(getattr(app.state, '_init_error', '')).lower(): raise RuntimeError("acestep_out_of_memory")
            raise RuntimeError("acestep_models_not_ready")
        expected = 'cpu' if PROFILE['device']['backend'] == 'cpu' else 'xpu' if PROFILE['device']['backend'] == 'xpu' else 'cuda'
        handler = app.state._model_init_kwargs['handler']
        if str(handler.device).split(':')[0] != expected:
            raise RuntimeError("acestep_device_fallback_rejected")
        marker = CACHE / ("qualified-" + PROFILE["fingerprint"] + ".json")
        if not marker.exists() or os.getenv('AURAGO_REQUALIFY') == 'true':
            with LOCK: STATE["state"] = "testing"
            from acestep.inference import GenerationParams, GenerationConfig, generate_music
            result = generate_music(handler, app.state._model_init_kwargs['llm_handler'],
                                    GenerationParams(caption="Gentle piano melody", lyrics="[Instrumental]", duration=10, thinking=bool(PROFILE["lm_model"])),
                                    GenerationConfig(batch_size=1, audio_format="mp3"),
                                    save_dir=app.state.temp_audio_dir)
            if not result.success or not result.audios:
                if "out of memory" in str(result.error).lower(): raise RuntimeError("acestep_out_of_memory")
                raise RuntimeError("acestep_audio_test_failed")
            audio = Path(result.audios[0]["path"])
            check = subprocess.run(["ffprobe", "-v", "error", "-show_entries", "format=duration", "-of", "csv=p=0", str(audio)], capture_output=True, timeout=20)
            if check.returncode or not 8 <= float(check.stdout) <= 15: raise RuntimeError("acestep_audio_test_failed")
            audio.unlink()
            temporary = marker.with_suffix('.part')
            temporary.write_text(json.dumps({"fingerprint": PROFILE["fingerprint"]}))
            temporary.replace(marker)
        if PROFILE_CACHE:
            record={'gpu':asdict(GPU_CONFIG),'profile':PROFILE}
            temporary=PROFILE_CACHE.with_suffix('.part')
            temporary.write_text(json.dumps(record))
            temporary.replace(PROFILE_CACHE)
        with LOCK: STATE.update(ready=True, state="ready")
    except Exception as exc:
        message = str(exc)
        code = "acestep_out_of_memory" if "out of memory" in message.lower() else message if message.startswith("acestep_") and len(message) < 80 else "acestep_initialization_failed"
        with LOCK: STATE.update(ready=False, state="error", error_code=code)
        print(code, file=sys.stderr)

def create_app():
    global GPU_CONFIG, PROFILE, PROFILE_CACHE
    import torch
    from fastapi import Depends, Request as FastRequest
    from acestep.api.http.auth import verify_api_key
    from acestep.api_server import create_app as upstream_app
    backend = os.environ["AURAGO_BACKEND"]
    found = probe()["devices"]
    index = int(os.getenv("AURAGO_DEVICE_INDEX", "0"))
    device = next((d for d in found if d["index"] == index), None)
    if not device: raise RuntimeError("acestep_gpu_not_available")
    if backend != "cpu": (torch.xpu if backend == "xpu" else torch.cuda).set_device(index)
    GPU_CONFIG, PROFILE = choose_profile(device, float(os.getenv("AURAGO_VRAM_RESERVE_GB", "1")), os.getenv("AURAGO_CONSERVATIVE") == "true")
    identity={k:device.get(k) for k in ('id','backend','name','driver','total_gb')}
    identity.update(image=os.getenv('AURAGO_IMAGE_PIN',''),reserve=os.getenv('AURAGO_VRAM_RESERVE_GB','1'),conservative=os.getenv('AURAGO_CONSERVATIVE','false'))
    PROFILE_CACHE=CACHE/('profile-'+hashlib.sha256(json.dumps(identity,sort_keys=True).encode()).hexdigest()+'.json')
    if PROFILE_CACHE.is_file() and os.getenv('AURAGO_REQUALIFY') != 'true':
        from acestep.gpu_config import GPUConfig
        saved=json.loads(PROFILE_CACHE.read_text())
        # Keep the last qualified settings across rollback/restart. The live
        # allocation test still detects newly occupied memory or changed drivers.
        GPU_CONFIG=GPUConfig(**saved['gpu'])
        current=PROFILE
        PROFILE=saved['profile']
        PROFILE.update(device=device,ram_gb=current['ram_gb'],disk_gb=current['disk_gb'])
    STATE["profile"] = PROFILE
    STATE['image_pin'] = os.getenv('AURAGO_IMAGE_PIN','')
    os.environ.update({"ACESTEP_NO_INIT": "true", "ACESTEP_CONFIG_PATH": PROFILE["model"],
                       "ACESTEP_DEVICE": "cpu" if backend == "cpu" else f"{'xpu' if backend == 'xpu' else 'cuda'}:{index}",
                       "ACESTEP_INIT_LLM": str(bool(PROFILE["lm_model"])).lower(), "ACESTEP_LM_MODEL_PATH": PROFILE["lm_model"],
                       "ACESTEP_LM_BACKEND": PROFILE["lm_backend"], "ACESTEP_COMPILE_MODEL": str(PROFILE["compile"]).lower(),
                       "ACESTEP_OFFLOAD_TO_CPU": str(PROFILE["offload"]).lower(), "ACESTEP_OFFLOAD_DIT_TO_CPU": str(PROFILE["offload_dit"]).lower(),
                       "ACESTEP_USE_FLASH_ATTENTION": "false", "AURAGO_QUANTIZATION": PROFILE["quantization"]})
    app = upstream_app()
    original = app.router.lifespan_context
    @asynccontextmanager
    async def lifespan(app):
        async with original(app):
            task = asyncio.create_task(asyncio.to_thread(initialize, app))
            yield
            task.cancel()
    app.router.lifespan_context = lifespan
    @app.get("/aurago/status", dependencies=[Depends(verify_api_key)])
    def status():
        with LOCK: return dict(STATE)
    @app.middleware("http")
    async def private_api(request: FastRequest, call_next):
        from starlette.responses import JSONResponse
        key = os.getenv('ACESTEP_API_KEY', '')
        if not key or not hmac.compare_digest(request.headers.get('authorization', ''), 'Bearer ' + key):
            return JSONResponse({'error': 'unauthorized'}, status_code=401)
        allowed = {("GET", "/aurago/status"), ("GET", "/health"), ("GET", "/v1/audio"),
                   ("GET", "/v1/models"), ("POST", "/release_task"), ("POST", "/query_result")}
        if (request.method, request.url.path) not in allowed:
            from starlette.responses import JSONResponse
            return JSONResponse({"error": "endpoint_disabled"}, status_code=404)
        if request.url.path == "/release_task" and not STATE["ready"]:
            from starlette.responses import JSONResponse
            return JSONResponse({"error": "models_not_ready"}, status_code=503)
        if request.url.path == '/v1/audio':
            path = Path(request.query_params.get('path', ''))
            base = Path(app.state.temp_audio_dir).resolve()
            if path.suffix.lower() != '.mp3' or not path.resolve().is_relative_to(base) or path.is_symlink() or not path.is_file() or path.stat().st_size > 64 << 20:
                return JSONResponse({'error': 'invalid_audio'}, status_code=400)
            check = await asyncio.to_thread(subprocess.run, ['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', str(path)], capture_output=True, timeout=20)
            try: valid = check.returncode == 0 and 1 <= float(check.stdout) <= 601
            except ValueError: valid = False
            if not valid: return JSONResponse({'error': 'invalid_audio'}, status_code=400)
        return await call_next(request)
    return app

if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] == "probe":
        print(json.dumps(probe()))
    else:
        import uvicorn
        uvicorn.run("aurago_runtime:create_app", factory=True, host="0.0.0.0", port=8001, access_log=False)
