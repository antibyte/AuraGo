"""Pin public ACE-Step model files; weights are hashed by their HF LFS SHA256.

Run with --write to refresh; default verifies the checked-in manifest structure.
Image digests are added by the image publishing workflow, never invented here.
"""
import argparse
import hashlib
import json
from pathlib import Path
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]
DEST = ROOT / "internal/acestep/release.json"
REVISION = "ca1e85fe9430179831e6bc6be790c332190a3866"

def fetch(url):
    with urlopen(Request(url, headers={"User-Agent": "AuraGo-ACE-Step-manifest"}), timeout=90) as r:
        return r.read()

def generate():
    current = json.loads(DEST.read_text()) if DEST.exists() else {}
    result = {**current, "upstream_commit": REVISION, "images": current.get("images", {}), "models": {}}
    repos = {
        "ACE-Step/Ace-Step1.5": ["vae", "Qwen3-Embedding-0.6B", "acestep-v15-turbo", "acestep-5Hz-lm-1.7B"],
        "ACE-Step/acestep-v15-xl-turbo": ["acestep-v15-xl-turbo"],
        "ACE-Step/acestep-5Hz-lm-0.6B": ["acestep-5Hz-lm-0.6B"],
        "ACE-Step/acestep-5Hz-lm-4B": ["acestep-5Hz-lm-4B"],
    }
    for repo, models in repos.items():
        revision = json.loads(fetch(f"https://huggingface.co/api/models/{repo}"))["sha"]
        files = json.loads(fetch(f"https://huggingface.co/api/models/{repo}/tree/{revision}?recursive=true&expand=false"))
        for model in models:
            entries = []
            for item in files:
                path = item["path"]
                if item["type"] != "file" or path.endswith((".md", ".gitattributes")): continue
                if len(models) > 1:
                    if not path.startswith(model + "/"): continue
                    relative = path[len(model) + 1:]
                else: relative = path
                url = f"https://huggingface.co/{repo}/resolve/{revision}/{path}"
                sha = item.get("lfs", {}).get("oid") or hashlib.sha256(fetch(url)).hexdigest()
                entries.append({"path": relative, "url": url, "size": item["size"], "sha256": sha})
            if not entries: raise RuntimeError(f"No files for {model}")
            result["models"][model] = {"repo": repo, "revision": revision, "files": entries}
    pin_runtime_code(result)
    return result

def pin_runtime_code(result):
    # Upstream deliberately replaces HF model Python with its own revision-bound
    # implementations. Pin those effective files, not the overwritten originals.
    for model, variant in [('acestep-v15-turbo','turbo'),('acestep-v15-xl-turbo','xl_turbo')]:
        source=f'acestep/models/{variant}'
        listing=json.loads(fetch(f'https://api.github.com/repos/ace-step/ACE-Step-1.5/contents/{source}?ref={REVISION}'))
        entries={entry['path']:entry for entry in result['models'][model]['files']}
        for file in listing:
            if not file['name'].endswith('.py') or file['name']=='__init__.py':continue
            path=source+'/'+file['name']
            url=f'https://raw.githubusercontent.com/ace-step/ACE-Step-1.5/{REVISION}/{path}'
            content=fetch(url)
            entries[file['name']]={'path':file['name'],'url':url,'runtime_path':path,'size':len(content),'sha256':hashlib.sha256(content).hexdigest()}
        result['models'][model]['files']=list(entries.values())

def check(value):
    assert value["upstream_commit"] == REVISION
    assert len(value["models"]) == 7
    for model in value["models"].values():
        assert len(model["revision"]) == 40 and model["files"]
        for file in model["files"]:
            assert len(file["sha256"]) == 64 and file["size"] > 0
            if 'runtime_path' in file:
                assert file['url']==f"https://raw.githubusercontent.com/ace-step/ACE-Step-1.5/{REVISION}/"+file['runtime_path']
                assert file['runtime_path'].startswith(('acestep/models/turbo/','acestep/models/xl_turbo/'))
            else:
                assert file["url"].startswith("https://huggingface.co/")
                assert model["revision"] in file["url"]
            assert not file["path"].startswith("/") and ".." not in Path(file["path"]).parts
    for backend, image in value["images"].items():
        assert backend in ("cuda", "rocm", "xpu", "vulkan", "cpu")
        assert image.startswith("ghcr.io/antibyte/aurago-acestep-") and "@sha256:" in image
    if 'vulkan' in value:
        native = value['vulkan']
        assert len(native['upstream_commit']) == 40 and len(native['ggml_commit']) == 40
        assert len(native['revision']) == 40 and len(native['models']) == 7
        for name, entry in native['models'].items():
            assert name.endswith('.gguf') and Path(name).name == name
            assert entry['size'] > 0 and len(entry['sha256']) == 64
            assert entry['url'] == f"https://huggingface.co/{native['repo']}/resolve/{native['revision']}/{name}"


def pin_vulkan(value):
    repo = 'Serveurperso/ACE-Step-1.5-GGUF'
    revision = json.loads(fetch(f'https://huggingface.co/api/models/{repo}'))['sha']
    files = json.loads(fetch(f'https://huggingface.co/api/models/{repo}/tree/{revision}?recursive=true&expand=false'))
    names = {'Qwen3-Embedding-0.6B-Q8_0.gguf', 'vae-BF16.gguf', 'acestep-v15-turbo-Q8_0.gguf',
             'acestep-v15-turbo-Q4_K_M.gguf', 'acestep-v15-xl-turbo-Q8_0.gguf',
             'acestep-5Hz-lm-0.6B-Q8_0.gguf', 'acestep-5Hz-lm-1.7B-Q8_0.gguf'}
    entries = {f['path']: {'url': f"https://huggingface.co/{repo}/resolve/{revision}/{f['path']}",
                           'size': f['size'], 'sha256': f['lfs']['oid']} for f in files if f['path'] in names}
    assert set(entries) == names
    value['vulkan'] = {'upstream_commit': 'daf7644dc7efffe0da14fb92ea698e0a8840b474',
                       'ggml_commit': '7d0241063b6dad00972a69565445dffb91dc3bc9',
                       'repo': repo, 'revision': revision, 'models': entries}

if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--write", action="store_true")
    parser.add_argument("--vulkan", action="store_true", help="Refresh only native GGUF pins; requires --write")
    args = parser.parse_args()
    if args.vulkan and not args.write: parser.error('--vulkan requires --write')
    value = generate() if args.write and not args.vulkan else json.loads(DEST.read_text())
    if args.vulkan: pin_vulkan(value)
    check(value)
    if args.write:
        DEST.parent.mkdir(parents=True, exist_ok=True)
        DEST.write_text(json.dumps(value, indent=2) + "\n", encoding="utf-8")
    print("ACE-Step model manifest verified")
