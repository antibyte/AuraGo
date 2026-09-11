"""Offline contract checks; run with the pinned upstream installed."""
import hashlib
from dataclasses import asdict
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import aurago_runtime as runtime

class RuntimeContract(unittest.TestCase):
    def test_profiles(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(runtime, 'MODELS', Path(directory)), patch.object(runtime, 'memory_gb', return_value=32):
            for backend in ['cuda', 'rocm', 'xpu', 'cpu']:
                device = {'id': backend + ':0', 'index':0, 'name':'fixture', 'backend':backend, 'total_gb':24, 'free_gb':21, 'driver':'fixture', 'verified':True}
                gpu, profile = runtime.choose_profile(device, 1)
                json.dumps(asdict(gpu))
                self.assertEqual(profile['model'], 'acestep-v15-turbo' if backend == 'cpu' else 'acestep-v15-xl-turbo')
                self.assertLessEqual(profile['max_duration'],600)
                if backend != 'cuda':
                    self.assertEqual(profile['lm_backend'],'pt')
                    self.assertFalse(profile['compile'])
                    self.assertFalse(profile['quantization'])
                device['free_gb']=20.9
                _, profile=runtime.choose_profile(device,1)
                self.assertEqual(profile['model'],'acestep-v15-turbo')
                _, reduced=runtime.choose_profile(device,1,True)
                self.assertEqual(reduced['model'],'acestep-v15-turbo')
            device['backend']='cuda';device['free_gb']=4.9
            with self.assertRaisesRegex(RuntimeError,'insufficient_vram'):runtime.choose_profile(device,1)

    def test_verified_resumable_download(self):
        class Response:
            status=206
            headers={'Content-Range':'bytes 3-5/6'}
            def __enter__(self):return self
            def __exit__(self,*args):pass
            def read(self,size):data=getattr(self,'data',b'def');self.data=b'';return data
        content=b'abcdef'
        entry={'url':'https://example.test/model','size':6,'sha256':hashlib.sha256(content).hexdigest()}
        with tempfile.TemporaryDirectory() as directory:
            target=Path(directory)/'model';target.with_suffix('.part').write_bytes(b'abc')
            with patch.object(runtime,'urlopen',return_value=Response()) as request:
                runtime.download_file(entry,target)
                self.assertEqual(request.call_args.args[0].headers['Range'],'bytes=3-')
            self.assertEqual(target.read_bytes(),content)
            with patch.object(runtime,'urlopen',side_effect=AssertionError('offline cache requested network')):
                runtime.download_file(entry,target)
            target.unlink();target.with_suffix('.part').write_bytes(b'xxx')
            with patch.object(runtime,'urlopen',return_value=Response()):
                with self.assertRaisesRegex(RuntimeError,'hash_mismatch'):runtime.download_file(entry,target)
            self.assertFalse(target.exists())

    def test_model_paths_and_pins(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(runtime,'MODELS',Path(directory)), patch.object(runtime,'RELEASE',{'models':{'test':{'files':[]}}}):
            with self.assertRaisesRegex(RuntimeError,'unpinned'):runtime.ensure_model('other',directory)
            with self.assertRaisesRegex(RuntimeError,'unpinned'):runtime.ensure_model('test','/elsewhere')
            runtime.RELEASE['models']['test']['files']=[{'path':'../../escape'}]
            with self.assertRaisesRegex(RuntimeError,'invalid_model_path'):runtime.ensure_model('test',directory)

    def test_runtime_model_code_is_verified_offline(self):
        with tempfile.TemporaryDirectory() as directory, patch.object(runtime,'ROOT',Path(directory)):
            source=Path(directory)/'acestep/models/turbo/model.py';source.parent.mkdir(parents=True);source.write_bytes(b'pinned code')
            entry={'runtime_path':'acestep/models/turbo/model.py','size':11,'sha256':hashlib.sha256(b'pinned code').hexdigest()}
            target=Path(directory)/'checkpoint/model.py'
            with patch.object(runtime,'urlopen',side_effect=AssertionError('network used')):
                runtime.download_file(entry,target)
            self.assertEqual(target.read_bytes(),b'pinned code')
            target.unlink();source.write_bytes(b'changed')
            with self.assertRaisesRegex(RuntimeError,'runtime_code_mismatch'):runtime.download_file(entry,target)

    def test_private_api_and_readiness(self):
        from fastapi.testclient import TestClient
        with patch.dict('os.environ',{'ACESTEP_API_KEY':'test-only','AURAGO_BACKEND':'cpu'}), patch.object(runtime,'initialize'), patch.object(runtime,'memory_gb',return_value=32):
            app=runtime.create_app()
            with TestClient(app) as client:
                self.assertEqual(client.get('/aurago/status').status_code,401)
                headers={'Authorization':'Bearer test-only'}
                self.assertEqual(client.get('/aurago/status',headers=headers).status_code,200)
                self.assertEqual(client.post('/release_task',headers=headers,json={}).status_code,503)
                self.assertEqual(client.post('/v1/init',headers=headers,json={}).status_code,404)
                self.assertEqual(client.get('/v1/audio?path=/etc/passwd',headers=headers).status_code,400)

if __name__=='__main__':unittest.main()
