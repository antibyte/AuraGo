"""Build-time only: fetch the reviewed model inventory and verify every byte."""
import hashlib
import json
import os
import urllib.request

with open("/app/models.json", encoding="utf-8") as source:
    manifest = json.load(source)
os.makedirs("/models", exist_ok=True)
for item in manifest["files"]:
    url = "https://huggingface.co/datalab-to/indra-ocr/resolve/" + manifest["revision"] + "/" + item["name"]
    digest = hashlib.sha256()
    size = 0
    with urllib.request.urlopen(url, timeout=120) as response, open("/models/" + item["name"], "wb") as output:
        for chunk in iter(lambda: response.read(1 << 20), b""):
            size += len(chunk)
            if size > item["size"]:
                raise ValueError("model size exceeded")
            digest.update(chunk)
            output.write(chunk)
    if size != item["size"] or digest.hexdigest() != item["sha256"]:
        raise ValueError("model checksum mismatch")
with open("/models/manifest.json", "w", encoding="utf-8") as output:
    json.dump(manifest, output)
