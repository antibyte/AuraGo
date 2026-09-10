# Default local speech output

New installations select **sanoTTS (CPU)** in Config → Speech Output, with
**Automatic (user language)**. Existing configurations retain their selected
provider, including disabled speech. Users can select another provider at any time.

Python 3.10+ with `venv` and `pip` is required (included in the Docker image;
the native installer offers it). On first speech output, AuraGo automatically
creates `data/sanotts/venv`, installs the bundled runtime and its dependencies,
and downloads only the selected voice into `data/sanotts/voices`. This first use
requires internet access; subsequent synthesis is local. It requires neither an
API key nor a GPU, Docker sidecar, torch, or onnxruntime. Downloads and dependency
installation can make the first call slower. A failed installation can be retried.

Explicit Speech Output language overrides the requested language. In automatic
mode, the request language, UI language, then system language are considered.
Regional tags such as `de-AT` use the same German voice. Unsupported local voice
languages use English. This selects pronunciation, **not translation**: text is
spoken as supplied; the agent should provide English speech text for the fallback.

| Languages | Selected small voice |
| --- | --- |
| English | `heart-nano`, 294k parameters (about 337 KiB weights) |
| German, Czech, Spanish, Italian, Portuguese, Romanian, Russian, Turkish | `<code>-tiny`, about 511k parameters (about 1 MiB fp16 weights) |
| Arabic, French, Indonesian, Vietnamese | `<code>`, about 1.5M parameters |

The [browser demo](https://tts.ampixa.com/sanoTTS/) supports 16 languages. The
pinned Python runtime has 13 packaged languages: Hindi, Nepali, and Chinese
have only browser-format weights upstream and currently use English here.
German and the ten languages added on 2026-09-08 are included in the server packs.

Chat caching retains its existing limits and includes the runtime revision and
effective voice. Telephone synthesis uses private temporary WAV files, removed
after reading; only downloaded voice weights persist. CYD retains its short
English notifications and 8 kHz PCM conversion, using the same CLI runner.

## Runtime provenance and checks

- Upstream: [Ampixa/sanoTTS](https://github.com/Ampixa/sanoTTS/tree/de3f71a8603ee979a74e6f2a7592a0ce795d608b/pypkg).
- Revision: `de3f71a8603ee979a74e6f2a7592a0ce795d608b`, Python runtime `0.6.0`.
- Bundled wheel: `internal/sanotts/runtime/sanotts-0.6.0-py3-none-any.whl`, 4,537,649 bytes.
- SHA-256: `0582dabc20d1b6376bd1e266b889bbca6d839733d3d4bcf2be493748cf139a23`.
- Runtime license: MIT as declared by upstream `pypkg/pyproject.toml` and wheel
  metadata; the complete `runtime/LICENSE.MIT` is installed beside the runtime.
  Copyright Ampixa. The wheel retains upstream notices for bundled
  G2P data. espeak-ng and phonemizer dependencies and voice models use their
  upstream licenses, including GPL-3.0; dependencies are installed separately.
- Rebuild: `python scripts/build-sanotts-runtime.py`. Only this developer build
  downloads the full upstream source archive; user installations receive the wheel.
- Model packages come from [ampixa/sanoTTS](https://huggingface.co/ampixa/sanoTTS)
  using the upstream downloader. Its GitHub release fallback exists only for
  packages actually published there; new language packs currently require HF.
- Unit checks: `go test ./internal/sanotts ./internal/cyd ./internal/config ./internal/tools`.
- Real CPU and automatic first-use check (PowerShell):
  `$env:AURAGO_SANOTTS_SMOKE_DIR="$PWD/disposable/sanotts-smoke-data"; go test ./internal/tools -run TestSanoTTSCPUSmoke -v -count=1`.

The opt-in check creates German WAV output and an English fallback for an
unsupported language, through the actual cached and in-memory provider paths.
