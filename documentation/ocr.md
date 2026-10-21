# Optional Indra OCR

Enable **Settings → OCR**, save, then choose **Setup**. Docker must be enabled
and writable. Setup pulls the immutable release image; the operator never needs
Python, model downloads or a Docker build. Start/stop/test are explicit actions;
status is passive. Enabled, installed OCR starts with AuraGo. A manual stop lasts
until explicit start or server restart.
Startup also reaps orphan parser containers/networks belonging to this installation,
including when OCR is disabled, when the existing Docker control permits cleanup.

The OCR Desktop app accepts PDF, PNG, JPEG, TIFF, WebP and BMP through multiple
uploads, drag-and-drop or the existing Desktop file picker. Start a batch, inspect
each file/page beside text, Markdown or JSON, copy, export or save through normal
Desktop file permissions. Accepted jobs continue after closing the app. Reopen to
see your still-available jobs. Cancel preserves already finished pages/files.
Inputs and results expire 24 hours after completion. Restart discards them; this
is not a permanent history. Unsubmitted uploads expire after an hour.

## Configuration and boundaries

```yaml
ocr:
  enabled: false
  agent_access: true
  automatic_chat: true
  automatic_indexing: true
```

The three subordinate options only take effect after global activation.
Agent access exposes `ocr` operations extract/status/result/cancel. Automatic
chat enrichment shares a 15-second wait budget per turn, leaves pending jobs
available to that same agent session, and injects a bounded external-data excerpt
only into the current request. Indexing keeps existing directory/type/permission
rules; OCR supplements image analysis and multimodal input. Its version/model/
mode enter the index fingerprint. Incomplete OCR cannot replace a valid index.
Disabled/unavailable OCR keeps existing extraction fallbacks.

Auto mode uses native text and recognizes scanned content in mixed PDFs.
Force OCR ignores text layers. Recognition can be wrong, particularly handwriting
and complex tables; page warnings and incomplete results remain visible.
Automatic orientation can also fail on narrow cropped text strips; rotate such
inputs upright before recognition when their result is unreadable.
No searchable PDF output, GPU, native installation or permanent history is provided.

- 20 files per batch, 32 MiB and 100 pages per file; stricter ingress limits apply.
- One worker, 300 seconds/file, at most four CPU threads, 2 GiB container RAM.
- 1 GiB temporary budget: 960 MiB host inputs/results plus 64 MiB parser tmpfs.
  At most 16 MiB result data per file; excessive output fails explicitly.
- Token-protected private HTTP, non-root/read-only image, dropped capabilities,
  no-new-privileges, no document directory mounts, no parser Docker access.
- Native AuraGo uses a loopback port on a dedicated non-masquerading bridge.
  Container AuraGo joins a dedicated internal network on its same Docker Engine.
  The existing Engine client supports the configured local socket/npipe/proxy.
  An arbitrary remote Engine from a native host is outside this transport contract.

## API

Administrator routes: GET `/api/ocr/status`, POST `setup`, `start`, `stop`,
`test` below `/api/ocr/`. Setup/start respond 202 and publish lifecycle status.

Desktop routes below `/api/desktop/ocr/`:
GET `status`; POST `uploads` (one multipart `file`);
GET/POST `jobs` (create with `uploads`, `paths`, `mode`);
GET `jobs/{id}`; POST `jobs/{id}/cancel`;
GET `jobs/{id}/result?file_id=...`, `source?file_id=...`,
`export?file_id=...&format=text|md|json`, or `export?format=zip`.
Scopes, readonly gates and job ownership are enforced independently.
The private adapter is not a public API and accepts only authorized snapshots.

## Building and publishing

Indra 1.1.2 source is pinned to
`0e96b93c6810f17cf8352acbb4487cc9e45b66d5`; default (non-math) model v1.1.0
is pinned to `bd02167fdf2005a19962cc15188f8496a2c87d3b`.
`internal/ocr/runtime/requirements.txt` locks all Python artifacts by SHA-256,
`models.json` records model sizes/hashes, and startup verifies every model.
All models are in the image. Inference and verification work offline.

Maintainer build (no ARM emulation required for cross-wheel assembly):

```sh
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg SOURCE_DATE_EPOCH=0 --provenance=false \
  --output type=oci,dest=ocr-image.oci.tar,rewrite-timestamp=true \
  --metadata-file ocr-image.metadata.json \
  -t ghcr.io/antibyte/aurago-ocr:1.1.2 -f deploy/docker/Dockerfile.ocr .
python scripts/ocr/pin_image.py ocr-image.oci.tar ocr-image.metadata.json
gofmt -w internal/ocr/release.go
docker load --input ocr-image.oci.tar
```

Review the generated real index/platform config digests before committing.
The release workflow builds and tests first. Publishing is an explicit separate
workflow action; server rollout/restart is another delivery boundary. Until that
image is published, new installations cannot pull it. Exact OCI imports work
locally through the pinned platform config IDs.

Generate synthetic fixtures with `scripts/ocr/smoke_fixtures.py` inside the image.
Mount the bundled `ui/fonts/RobotoCondensed-Variable.ttf` and pass its path as
the second argument so German glyphs render correctly; `smoke.sh` does this.
Set `AURAGO_OCR_SMOKE_IMAGE` to the immutable local image ID and
`AURAGO_OCR_SMOKE_DIR` to that fixture directory, then run
`go test ./internal/ocr -run TestOCRDockerSmoke -v`.
The Docker-to-Docker smoke uses a Linux test binary, its own controller container,
the same Engine socket, and `AURAGO_OCR_SMOKE_IN_DOCKER=1`. Never expose that
socket to the parser container. Native arm64 CI smoke gates an arm64 publication.

## Licenses

Code: Apache-2.0. Models and outputs: separate modified OpenRAIL-M terms. Preserve
the complete [model license](../licences/texts/Indra-OCR-models.txt), attribution
and required notices. The image retains all license files; JSON/ZIP exports carry
the full model terms, and text/Markdown exports include the pinned license link.
See [third-party notices](../THIRD_PARTY_NOTICES.md#optional-indra-ocr-service).
