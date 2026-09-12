"""Offline native adapter checks, runnable with Python's standard library."""
import json
import os
from pathlib import Path
import tempfile
import threading
import unittest
from unittest.mock import patch, Mock
from urllib.request import Request, urlopen
from urllib.error import HTTPError

import vulkan_runtime as runtime
import patch_vulkan


class VulkanContract(unittest.TestCase):
    def profile(self, free=10, conservative=False):
        device = {'id': 'vulkan:0', 'name': 'fixture', 'uuid': '0000:05:00.0', 'driver': 'fixture',
                  'index': 0, 'total_gb': 24, 'free_gb': free, 'verified': True, 'backend': 'vulkan'}
        with tempfile.TemporaryDirectory() as directory, patch.object(runtime.common, 'MODELS', Path(directory)), \
             patch.object(runtime.common, 'memory_gb', return_value=24), patch.object(runtime.common, 'RELEASE', {'vulkan': {}}):
            return runtime.choose_profile(device, 1, conservative)

    def test_profile_boundaries(self):
        with self.assertRaisesRegex(RuntimeError, 'insufficient_vram'): self.profile(4.9)
        self.assertIn('Q4_K_M', self.profile(5)['model'])
        self.assertEqual(self.profile(8)['lm_model'], '')
        self.assertIn('0.6B', self.profile(9)['lm_model'])
        self.assertIn('1.7B', self.profile(13)['lm_model'])
        self.assertIn('xl-turbo', self.profile(21)['model'])
        self.assertNotIn('xl-turbo', self.profile(20.9)['model'])
        self.assertEqual(self.profile(21, True)['lm_model'], '')
        self.assertEqual(self.profile(10)['fingerprint'], self.profile(11)['fingerprint'])
        self.assertEqual(self.profile()['compute_precision'], 'fp32')

    def test_probe_enforces_verified_vulkan_math(self):
        with patch.dict(os.environ, GGML_VK_DISABLE_F16='0'), patch.object(runtime.common, 'device_groups', return_value=[]), \
             patch.object(runtime.subprocess, 'run', return_value=Mock(returncode=1)) as run:
            self.assertEqual(runtime.probe()['devices'], [])
            self.assertEqual(run.call_args.kwargs['env']['GGML_VK_DISABLE_F16'], '1')

    def test_parameters_and_seed_zero(self):
        profile = self.profile()
        request = runtime.generation_request({'prompt': 'Piano', 'lyrics': '[Instrumental]', 'audio_duration': 120,
                                             'bpm': 90, 'vocal_language': 'de', 'use_random_seed': False, 'seed': 0}, profile)
        self.assertEqual(request['seed'], 0)
        self.assertEqual(request['lm_seed'], 0)
        self.assertEqual(request['bpm'], 90)
        self.assertEqual(request['vocal_language'], 'de')
        for field, value in [('audio_duration', 601), ('audio_duration', float('nan')), ('bpm', 301), ('seed', True), ('vocal_language', '../../en')]:
            with self.assertRaises(ValueError):
                runtime.generation_request({'prompt': 'Piano', 'lyrics': 'Hello', 'use_random_seed': False, 'seed': 0, field: value}, profile)
        with self.assertRaisesRegex(ValueError, 'lyrics_required'):
            runtime.generation_request({'prompt': 'Sing'}, self.profile(5))

    def test_native_jobs_reject_foreign_ids_and_failed_status(self):
        with patch.object(runtime, 'native', return_value=(b'{"id":"//evil.test"}', 'application/json')):
            with self.assertRaisesRegex(RuntimeError, 'invalid_submission'): runtime.native_job('/synth', {})
        with patch.object(runtime, 'native', side_effect=[(b'{"id":"0123456789abcdef"}', ''), (b'{"status":"failed"}', '')]):
            with self.assertRaisesRegex(RuntimeError, 'generation_failed'): runtime.native_job('/synth', {})
        with patch.object(runtime, 'native', side_effect=[(b'{"id":"0123456789abcdef"}', ''), (b'{"status":"done"}', ''), (b'audio', 'audio/mpeg')]) as native:
            self.assertEqual(runtime.native_job('/synth', {}), (b'audio', 'audio/mpeg'))
            self.assertEqual(native.call_args.args[1], '/job?id=0123456789abcdef&result=1')

    def test_auth_busy_readiness_and_audio_confinement(self):
        server = runtime.ThreadingHTTPServer(('127.0.0.1', 0), runtime.Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True); thread.start()
        base = f'http://127.0.0.1:{server.server_port}'
        def call(path, value=None, key='test-only'):
            return urlopen(Request(base + path, data=json.dumps(value).encode() if value is not None else None,
                                   headers={'Authorization': 'Bearer ' + key}), timeout=3)
        try:
            with patch.dict(os.environ, ACESTEP_API_KEY='test-only'), patch.object(runtime, 'PROFILE', self.profile()), \
                 patch.object(runtime, 'JOB', None), patch.object(runtime, 'STATE', {'ready': False, 'state': 'loading'}):
                for path, value, key, code in [('/aurago/status', None, 'wrong', 401), ('/release_task', {'prompt': 'Piano'}, 'test-only', 503),
                                                ('/v1/audio?path=/etc/passwd', None, 'test-only', 400), ('/synth', {}, 'test-only', 404)]:
                    with self.assertRaises(HTTPError) as error: call(path, value, key)
                    self.assertEqual(error.exception.code, code)
                runtime.STATE['ready'] = True
                with patch.object(runtime, 'run_job'):
                    with call('/release_task', {'prompt': 'Piano', 'lyrics': '[Instrumental]', 'use_random_seed': False, 'seed': 0}) as response:
                        job_id = json.load(response)['data']['task_id']
                    with self.assertRaises(HTTPError) as error: call('/release_task', {'prompt': 'Piano'})
                    self.assertEqual(error.exception.code, 409)
                    with call('/query_result', {'task_id_list': [job_id]}) as response:
                        self.assertEqual(json.load(response)['data'][0]['status'], 0)
        finally:
            server.shutdown(); server.server_close(); thread.join()

    def test_build_patch_is_strict(self):
        with tempfile.TemporaryDirectory() as directory:
            file = Path(directory) / 'tools/ace-server.cpp'; file.parent.mkdir()
            file.write_text('    g_store = store_create(g_keep_loaded ? EVICT_NEVER : EVICT_STRICT);')
            lm = Path(directory) / 'src/pipeline-lm.cpp'; lm.parent.mkdir()
            lm.write_text('\n'.join([
                '        int tok            = sample_top_k_p(lg.data(), V, temperature, top_p, top_k, seqs[i].rng);',
                '            compact_logits[0] = lc[eos_idx];',
                '                seqs[orig_i].audio_codes.push_back(tok - AUDIO_CODE_BASE);']))
            patch_vulkan.patch(directory)
            self.assertIn('store_require_dit', file.read_text())
            self.assertIn('store_require_lm', file.read_text())
            self.assertIn('key.adapter_scale = 1.0f', file.read_text())
            self.assertIn('ceil(aces[orig_i].duration * 5)', lm.read_text())
            file.write_text('changed upstream')
            with self.assertRaises(AssertionError): patch_vulkan.patch(directory)


if __name__ == '__main__': unittest.main()
