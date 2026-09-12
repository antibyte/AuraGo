"""Authenticated adapter for the pinned acestep.cpp worker. No public native API."""
from collections import deque
from email.parser import BytesParser
from email.policy import default as email_policy
import hashlib
import hmac
import http.client
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
import json
import math
import os
from pathlib import Path
import platform
import re
import secrets
import shutil
import subprocess
import sys
import threading
import time
from urllib.parse import parse_qs, urlencode, urlsplit

import aurago_runtime as common

STATE = common.STATE
LOCK = common.LOCK
PROFILE = None
WORKER = None
JOB = None
LOGS = deque(maxlen=32)
MAX_AUDIO = 64 << 20
BIN = Path('/opt/acestep')
AUDIO = common.CACHE / 'acestep/tmp/api_audio'
FILES = {
    'encoder': 'Qwen3-Embedding-0.6B-Q8_0.gguf',
    'vae': 'vae-BF16.gguf',
}


def probe():
    groups = common.device_groups()
    result = {'devices': [], 'groups': groups}
    # Qwen LM inference on Renoir produces degenerate text/codes with FP16 math.
    # Use the same verified FP32 Vulkan arithmetic for probes and the worker.
    env = dict(os.environ, GGML_VK_DISABLE_F16='1')
    listed = subprocess.run([str(BIN / 'aurago-vulkan-probe')], capture_output=True, timeout=60, env=env)
    if listed.returncode: return result
    for item in json.loads(listed.stdout):
        pci, name = item['pci'], item['backend_name']
        if not re.fullmatch(r'[0-9a-f]{4}:[0-9a-f]{2}:[0-9a-f]{2}\.[0-7]', pci): continue
        if not re.fullmatch(r'Vulkan[0-9]+', name): continue
        nodes = [str(n) for n in Path('/dev/dri').glob('renderD*')
                 if (Path('/sys/class/drm') / n.name / 'device').resolve().name == pci]
        if not nodes: continue  # Also excludes software Vulkan devices such as llvmpipe.
        try:
            test = subprocess.run([str(BIN / 'aurago-vulkan-probe'), name], capture_output=True, timeout=60, env=env)
            if test.returncode: continue
        except subprocess.TimeoutExpired:
            continue
        available = min(item['free'] / 2**30, common.memory_gb()) if item['shared'] else item['free'] / 2**30
        driver = platform.release() + ' | ' + common.RELEASE.get('vulkan', {}).get('ggml_commit', '')
        for module in ('amdgpu', 'i915', 'xe', 'nouveau'):
            version = Path('/sys/module') / module / 'srcversion'
            if version.is_file(): driver += ' | ' + version.read_text().strip()[:128]
        result['devices'].append({'id': 'vulkan:' + name[6:], 'index': int(name[6:]),
                                 'backend': 'vulkan', 'name': item['name'], 'uuid': pci,
                                 'total_gb': item['total'] / 2**30, 'free_gb': available,
                                 'driver': driver, 'render_nodes': nodes, 'groups': groups, 'verified': True})
    return result


def choose_profile(device, reserve, conservative=False):
    budget = device['free_gb'] - reserve
    if budget < 4: raise RuntimeError('acestep_insufficient_vram')
    quant = 'Q4_K_M' if conservative or budget < 6 else 'Q8_0'
    model = 'acestep-v15-xl-turbo-Q8_0.gguf' if budget >= 20 and not conservative else f'acestep-v15-turbo-{quant}.gguf'
    lm = '' if conservative or budget < 8 else 'acestep-5Hz-lm-' + ('1.7B' if budget >= 12 else '0.6B') + '-Q8_0.gguf'
    ram = common.memory_gb()
    if ram < 4: raise RuntimeError('acestep_insufficient_ram')
    profile = {'device': device, 'model': model, 'lm_model': lm, 'lm_backend': 'ggml',
               'max_duration': 120 if conservative or budget < 12 else 240 if budget < 20 else 600,
               'quantization': quant, 'compute_precision': 'fp32', 'offload': False, 'offload_dit': False,
               'compile': False, 'flash_attention': True, 'ram_gb': ram,
               'disk_gb': shutil.disk_usage(common.MODELS).free / 2**30}
    identity = {k: v for k, v in profile.items() if k not in ('device', 'ram_gb', 'disk_gb')}
    identity['device'] = {k: device[k] for k in ('id', 'name', 'uuid', 'driver', 'total_gb')}
    identity.update(release=common.RELEASE['vulkan'], image=os.getenv('AURAGO_IMAGE_PIN', ''))
    profile['fingerprint'] = hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest()
    return profile


def native(method, path, value=None, limit=2 << 20):
    # Fixed loopback connection, no proxy, redirects or caller-provided origins.
    conn = http.client.HTTPConnection('127.0.0.1', 8002, timeout=30)
    try:
        conn.request(method, path, json.dumps(value) if value is not None else None,
                     {'Content-Type': 'application/json'})
        response = conn.getresponse()
        if response.status != 200: raise RuntimeError('acestep_native_request_failed')
        content = response.read(limit + 1)
        if len(content) > limit: raise RuntimeError('acestep_invalid_result')
        return content, response.getheader('Content-Type', '')
    finally:
        conn.close()


def native_job(endpoint, request):
    body, _ = native('POST', endpoint, request)
    job_id = json.loads(body).get('id', '')
    if not isinstance(job_id, str) or not re.fullmatch(r'[0-9a-f]{16}', job_id):
        raise RuntimeError('acestep_invalid_submission')
    path = '/job?' + urlencode({'id': job_id})
    deadline = time.monotonic() + 1800
    while time.monotonic() < deadline:
        if WORKER is not None and WORKER.poll() is not None: raise RuntimeError(worker_error())
        body, _ = native('GET', path)
        status = json.loads(body).get('status')
        if status == 'done': return native('GET', path + '&result=1', limit=MAX_AUDIO + (8 << 20))
        if status != 'running': raise RuntimeError('acestep_generation_failed')
        time.sleep(2)
    raise RuntimeError('acestep_timeout')


def generation_request(body, profile):
    prompt, lyrics = body.get('prompt'), body.get('lyrics', '')
    duration, bpm = body.get('audio_duration', 120), body.get('bpm', 0)
    seed = -1 if body.get('use_random_seed', True) else body.get('seed')
    language = body.get('vocal_language', '')
    if not isinstance(prompt, str) or not prompt.strip() or len(prompt.encode()) > 16000:
        raise ValueError('invalid_music_text')
    if not isinstance(lyrics, str) or len(lyrics.encode()) > 32000: raise ValueError('invalid_music_text')
    if type(duration) not in (int, float) or not math.isfinite(duration) or not 10 <= duration <= profile['max_duration']:
        raise ValueError('music_duration_out_of_range')
    if type(bpm) is not int or bpm != 0 and not 30 <= bpm <= 300: raise ValueError('music_bpm_out_of_range')
    if type(seed) is not int or not -1 <= seed <= 2147483647: raise ValueError('music_seed_out_of_range')
    if not isinstance(language, str) or language and not re.fullmatch(r'[a-z]{2,3}(-[A-Za-z]{2,4})?', language):
        raise ValueError('invalid_vocal_language')
    if not lyrics.strip() and not profile['lm_model']: raise ValueError('lyrics_required')
    return {'caption': prompt, 'lyrics': lyrics, 'duration': duration, 'bpm': bpm,
            'vocal_language': language, 'seed': seed, 'lm_seed': seed,
            'lm_batch_size': 1, 'synth_batch_size': 1, 'inference_steps': 8,
            'use_cot_caption': False, 'task_type': 'text2music', 'output_format': 'mp3',
            'synth_model': profile['model'], 'lm_model': profile['lm_model'], 'vae': FILES['vae']}


def render(request):
    if PROFILE['lm_model']:
        raw, _ = native_job('/lm', request)
        enriched = json.loads(raw)
        if not isinstance(enriched, list) or len(enriched) != 1 or not isinstance(enriched[0], dict):
            raise RuntimeError('acestep_invalid_result')
        # Only generated music content crosses this boundary. Never forward paths/options.
        for key in ('audio_codes', 'lyrics'):
            value = enriched[0].get(key, '')
            if not isinstance(value, str) or len(value) > 100000: raise RuntimeError('acestep_invalid_result')
            if key == 'audio_codes' or not request['lyrics'].strip(): request[key] = value
    raw, content_type = native_job('/synth', request)
    message = BytesParser(policy=email_policy).parsebytes(('Content-Type: ' + content_type + '\r\n\r\n').encode() + raw)
    if message.get_content_type() != 'multipart/mixed': raise RuntimeError('acestep_invalid_audio_result')
    audio = [part.get_payload(decode=True) for part in message.iter_parts() if part.get_content_type() == 'audio/mpeg']
    if len(audio) != 1 or not audio[0] or len(audio[0]) > MAX_AUDIO: raise RuntimeError('acestep_invalid_audio_result')
    path = AUDIO / (secrets.token_hex(16) + '.mp3')
    part = path.with_suffix('.part')
    try:
        part.write_bytes(audio[0])
        check = subprocess.run(['ffprobe', '-v', 'error', '-show_entries', 'format=duration', '-of', 'csv=p=0', str(part)],
                               capture_output=True, timeout=20)
        duration = float(check.stdout) if check.returncode == 0 else 0
        if not math.isfinite(duration) or abs(duration - request['duration']) > 2 or not 1 <= duration <= 601:
            raise RuntimeError('acestep_invalid_audio_duration')
        part.replace(path)
        return path, duration
    finally:
        part.unlink(missing_ok=True)


def worker_error():
    text = '\n'.join(LOGS).lower()
    return 'acestep_out_of_memory' if any(s in text for s in ('out of memory', 'outofdevicememory', 'failed to allocate', 'allocation failed')) else 'acestep_native_worker_failed'


def stop_worker():
    if WORKER is not None and WORKER.poll() is None:
        WORKER.terminate()
        try: WORKER.wait(timeout=5)
        except subprocess.TimeoutExpired: WORKER.kill(); WORKER.wait(timeout=5)


def run_job(job, request):
    try:
        path, duration = render(request)
        result = json.dumps([{'file': '/v1/audio?' + urlencode({'path': str(path)}),
                              'status': 1, 'metas': {'duration': duration}}])
        with LOCK: job.update(status=1, result=result, path=str(path)); STATE['state'] = 'ready'
    except Exception:
        stop_worker()  # Do not leave unobserved native inference running after failure.
        with LOCK: job.update(status=2); STATE.update(ready=False, state='error', error_code=worker_error())


def initialize():
    global PROFILE, WORKER
    try:
        with LOCK: STATE['state'] = 'probing'
        found = probe()['devices']
        pci = os.getenv('AURAGO_DEVICE_PCI', '')
        device = next((d for d in found if d['uuid'] == pci), None) if pci else next((d for d in found if d['index'] == int(os.getenv('AURAGO_DEVICE_INDEX', '0'))), None)
        if not device: raise RuntimeError('acestep_gpu_not_available')
        device['id'] = os.getenv('AURAGO_DEVICE_ID', device['id'])
        PROFILE = choose_profile(device, float(os.getenv('AURAGO_VRAM_RESERVE_GB', '1')), os.getenv('AURAGO_CONSERVATIVE') == 'true')
        identity = {k: device[k] for k in ('id', 'uuid', 'driver', 'total_gb')}
        identity.update(release=common.RELEASE['vulkan'], image=os.getenv('AURAGO_IMAGE_PIN', ''),
                        reserve=os.getenv('AURAGO_VRAM_RESERVE_GB', '1'), conservative=os.getenv('AURAGO_CONSERVATIVE', 'false'))
        profile_cache = common.CACHE / ('vulkan-profile-' + hashlib.sha256(json.dumps(identity, sort_keys=True).encode()).hexdigest() + '.json')
        if profile_cache.is_file() and os.getenv('AURAGO_REQUALIFY') != 'true':
            saved = json.loads(profile_cache.read_text())
            saved.update(device=device, ram_gb=PROFILE['ram_gb'], disk_gb=PROFILE['disk_gb'])
            PROFILE = saved
        with LOCK: STATE.update(profile=PROFILE, image_pin=os.getenv('AURAGO_IMAGE_PIN', ''), state='downloading')
        names = [FILES['encoder'], FILES['vae'], PROFILE['model']]
        if PROFILE['lm_model']: names.append(PROFILE['lm_model'])
        entries = common.RELEASE['vulkan']['models']
        model_dir = common.MODELS / 'vulkan'
        model_dir.mkdir(exist_ok=True)
        if model_dir.is_symlink(): raise RuntimeError('acestep_invalid_model_path')
        missing = sum(entries[name]['size'] for name in names if not common.file_valid(model_dir / name, entries[name]))
        if shutil.disk_usage(model_dir).free < missing + (1 << 30): raise RuntimeError('acestep_insufficient_disk')
        with LOCK: STATE['total_bytes'] = missing
        view = common.CACHE / ('vulkan-models-' + PROFILE['fingerprint'])
        view.mkdir(exist_ok=True)
        for name in names:
            common.download_file(entries[name], model_dir / name)
            link = view / name
            if link.is_symlink(): link.unlink()
            link.symlink_to(model_dir / name)
        AUDIO.mkdir(parents=True, exist_ok=True)
        with LOCK: STATE['state'] = 'loading'
        env = dict(os.environ, GGML_BACKEND='Vulkan' + str(device['index']), GGML_VK_DISABLE_F16='1')
        env.pop('ACESTEP_API_KEY', None)
        WORKER = subprocess.Popen([str(BIN / 'ace-server'), '--models', str(view), '--host', '127.0.0.1',
                                   '--port', '8002', '--keep-loaded', '--max-batch', '1', '--max-seq', '8192',
                                   '--vae-chunk', '256', '--vae-overlap', '64'], env=env,
                                  stdout=subprocess.DEVNULL, stderr=subprocess.PIPE, text=True)
        def logs():
            for line in WORKER.stderr: LOGS.append(line[:2048])
        threading.Thread(target=logs, daemon=True).start()
        deadline = time.monotonic() + 900
        while True:
            if WORKER.poll() is not None: raise RuntimeError(worker_error())
            try:
                native('GET', '/health')
                break
            except (OSError, http.client.HTTPException):
                if time.monotonic() > deadline: raise RuntimeError('acestep_models_not_ready')
                time.sleep(2)
        marker = common.CACHE / ('qualified-' + PROFILE['fingerprint'] + '.json')
        if not marker.is_file() or os.getenv('AURAGO_REQUALIFY') == 'true':
            with LOCK: STATE['state'] = 'testing'
            request = generation_request({'prompt': 'Gentle piano melody', 'lyrics': '[Instrumental]', 'audio_duration': 10,
                                          'use_random_seed': False, 'seed': 0}, PROFILE)
            audio, _ = render(request)
            audio.unlink()
            temporary = marker.with_suffix('.part')
            temporary.write_text(json.dumps({'fingerprint': PROFILE['fingerprint']}))
            temporary.replace(marker)
        temporary = profile_cache.with_suffix('.part')
        temporary.write_text(json.dumps(PROFILE))
        temporary.replace(profile_cache)
        with LOCK: STATE.update(ready=True, state='ready')
    except Exception as exc:
        stop_worker()
        code = str(exc)
        if not re.fullmatch(r'acestep_[a-z_]{1,65}', code): code = 'acestep_initialization_failed'
        with LOCK: STATE.update(ready=False, state='error', error_code=code)
        print(code, file=sys.stderr, flush=True)


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args): pass

    def respond(self, status, value, content_type='application/json'):
        data = value if isinstance(value, bytes) else json.dumps(value).encode()
        self.send_response(status)
        self.send_header('Content-Type', content_type)
        self.send_header('Content-Length', str(len(data)))
        self.send_header('Cache-Control', 'no-store')
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self): self.handle_api()
    def do_POST(self): self.handle_api()

    def handle_api(self):
        global JOB
        self.connection.settimeout(30)
        key = os.getenv('ACESTEP_API_KEY', '')
        if not key or not hmac.compare_digest(self.headers.get('Authorization', ''), 'Bearer ' + key):
            return self.respond(401, {'error': 'unauthorized'})
        url = urlsplit(self.path)
        try:
            if self.command == 'GET' and url.path == '/aurago/status':
                with LOCK:
                    status = dict(STATE)
                if WORKER is not None and WORKER.poll() is not None and status['ready']:
                    status.update(ready=False, state='error', error_code=worker_error())
                return self.respond(200, status)
            if self.command == 'GET' and url.path == '/v1/audio':
                query = parse_qs(url.query, strict_parsing=True)
                with LOCK: path = JOB.get('path') if JOB else None
                if not path or query != {'path': [path]}: return self.respond(400, {'error': 'invalid_audio'})
                audio = Path(path)
                if audio.is_symlink() or audio.parent != AUDIO or not audio.is_file() or audio.stat().st_size > MAX_AUDIO:
                    return self.respond(400, {'error': 'invalid_audio'})
                return self.respond(200, audio.read_bytes(), 'audio/mpeg')
            if self.command != 'POST' or url.path not in ('/release_task', '/query_result'):
                return self.respond(404, {'error': 'endpoint_disabled'})
            size = int(self.headers.get('Content-Length', '0'))
            if not 0 < size <= 256 << 10: return self.respond(413, {'error': 'invalid_body'})
            body = json.loads(self.rfile.read(size))
            if not isinstance(body, dict): raise ValueError('invalid_body')
            if url.path == '/query_result':
                with LOCK:
                    if not JOB or body.get('task_id_list') != [JOB['task_id']]: raise ValueError('invalid_task')
                    result = {k: JOB[k] for k in ('task_id', 'status', 'result')}
                return self.respond(200, {'code': 200, 'data': [result]})
            with LOCK:
                if not STATE['ready']: return self.respond(503, {'error': 'models_not_ready'})
                if JOB and JOB['status'] == 0: return self.respond(409, {'error': 'acestep_busy'})
                request = generation_request(body, PROFILE)
                if JOB and JOB.get('path'): Path(JOB['path']).unlink(missing_ok=True)
                JOB = {'task_id': secrets.token_hex(16), 'status': 0, 'result': ''}
                STATE['state'] = 'busy'
                threading.Thread(target=run_job, args=(JOB, request), daemon=True).start()
                submission = {'code': 200, 'data': {'task_id': JOB['task_id']}}
            return self.respond(200, submission)
        except (ValueError, KeyError, TypeError):
            return self.respond(400, {'error': 'invalid_request'})
        except (OSError, http.client.HTTPException):
            return self.respond(500, {'error': 'acestep_runtime_failed'})


if __name__ == '__main__':
    if len(sys.argv) > 1 and sys.argv[1] == 'probe':
        print(json.dumps(probe()))
    else:
        server = ThreadingHTTPServer(('0.0.0.0', 8001), Handler)
        threading.Thread(target=initialize, daemon=True).start()
        try: server.serve_forever()
        finally: stop_worker(); server.server_close()
