# Local Phonon speech

New installations select **Phonon (CPU)** and **Automatic (user language)** in
Config → Speech Output. Phonon 40m multilingual speaks German, English, French,
Spanish and Portuguese. Other languages, unavailable resources and incompatible
native runtimes use the existing sanoTTS provider. Explicit existing provider
choices, including disabled speech, survive upgrades. Speech Lab retains its
explicit chat/telephone routing. TTS pronounces the supplied text; it does not
translate it.

The installer's release packages and Docker image contain the model, tokenizer,
CPU executable and ten voices. Users need no Hugging Face account, API token,
Python, PyTorch, GPU or separate speech server for Phonon. sanoTTS retains its
existing Python and first-use download requirements when it is used as fallback.

| Language | Female | Male |
| --- | --- | --- |
| German | Nele | Anton |
| English | Zoey | Easton |
| French | Noémie | Augustin |
| Spanish | Noa | Marcos |
| Portuguese | Manuela-Lu | Davi |

An empty `tts.phonon.voice_id` selects a bundled voice by language and persona.
Servant, Professional, Thinker, Evil, Psycho, Friend and Terminator use male
voices; the other built-in, neutral, custom and unknown personas use female
voices. A manual voice ID overrides this selection across persona changes.
An explicit TTS language overrides the request language, UI language and agent
system language, in that order. Regional tags use their base language.

The configuration page lists all 102 pinned catalog voices and their installation
status. Downloading a voice does not select or save it. Choose an installed voice
and save the configuration separately. Failed downloads preserve the selection;
the downloaded file is verified before atomic publication. Additional voices
persist under `data/phonon/voices/` and work offline after downloading.
Unknown IDs and user-supplied URLs or filesystem paths are never download inputs.

## Runtime and release resources

- Runtime: [Gradium Phonon `ptts` 0.4.0](https://github.com/gradium-ai/xn-ptts/releases/tag/v0.4.0).
- Model: [gradium/phonon-40m-multilingual](https://huggingface.co/gradium/phonon-40m-multilingual),
  revision `1537ce46c64f42903e0d608b132ccb073dac46d2`.
  The q8 GGUF is 74,553,568 bytes, SHA-256
  `96fa067813244e0c6c5997b98ec2596b65ec8574b14916b48e97db056ff97e27`.
- Voices: [gradium/phonon-voices](https://huggingface.co/gradium/phonon-voices),
  revision `0ee17205f254b5abc43b266c8a0f215565c352f7`.
  Each embedding is 256,136 bytes. All IDs, hashes and runtime archive/binary
  checksums are pinned in `internal/phonon/manifest.json`.
- Every runtime, model, tokenizer and voice file is pinned by exact size and
  SHA-256; the finished archives additionally record SHA-256 for every included file.

Maintainers obtain the gated model once with approved access. Set `HF_TOKEN` in
the packaging process, then run `python scripts/prepare-phonon.py --fetch`.
Alternatively, use `--fetch --source-dir /authorized/checkpoint` to copy the model
from an already authorized local download. Tokens are never packaged. The helper
strips Authorization on redirects and never prints credentials.

`python scripts/prepare-phonon.py --check` verifies prepared resources without
network access. `--pack` produces `deploy/phonon-model.dat` and five
`deploy/phonon-runtime_<os>_<arch>.dat` archives. Release builders require these
verified resources before publication and include them in the release checksums.
Only manifest-listed files and required notices enter the packages.
Windows release builders require Python 3.10+ for packaging only. They resolve
the installed interpreter through `py -3`, then `python` or `python3`, before
cleaning generated artifacts; the selected interpreter need not be on `PATH`.
Release cleanup retains the prepared `deploy/phonon` directory. When building in
a clean worktree, copy that ignored directory from the prepared checkout first;
the builder verifies it again before packaging.
The model/voice archive is about 67 MB compressed; adding one native runtime
brings the installation download to about 75–77 MB, depending on the platform.
The Docker publishing workflow reuses the model package from the corresponding
AuraGo release (`--from-release TAG`). Source installations, source updates and
manual Docker builds use `--from-release latest`: this selects `model_package`
from the source manifest, with a fixed release URL, size and SHA-256, independently
of the latest AuraGo binary release. The package is currently shared with Speech
Lab v0.2.19. Both paths also verify every model and voice against the source
manifest. No Hugging Face approval or token is needed on the user's installation
or in CI. Existing verified resources can be reused offline.

Packages install beside the executable under `phonon/<bundle-id>/`. The bundle ID
includes runtime/model/catalog revisions, so rollback executables keep their own
resources. `--target windows_amd64 --install-dir /path/to/bin` stages a development
installation. Docker stages only its target under `/opt/aurago/phonon/`; the
operator-only `AURAGO_PHONON_DIR` specifies this resource base. Writable data
volumes do not hide bundled models or voices.

Published runtime requirements: Linux glibc 2.35+, macOS 15+, Windows 10+;
x86-64 binaries require x86-64-v3 (AVX2/FMA/F16C). Linux and macOS also have
ARM64 builds. No native upstream Windows ARM64 binary is available. Unsupported
platforms and failed native execution use sanoTTS. Synthesis is serialized,
cancellable, bounded to 120 seconds and returns temporary mono PCM16 WAV audio
at 24 kHz, compatible with the existing telephone decoder/resampler.

## Licenses and verification

The model weights use **CC BY-NC 4.0: noncommercial use only**. Attribution:
Gradium, Phonon 40m multilingual, unchanged original q8 checkpoint; see the
model URL above and `licences/texts/CC-BY-NC-4.0.txt`. Commercial use requires
appropriate permission from Gradium. Runtime code is MIT/Apache-2.0 and the voice
catalog declares Apache-2.0. These component licenses do not change AuraGo's own
MIT license. The packages carry the license texts and this attribution.

Run focused tests for `internal/phonon`, config, tools, agent, server and the TTS
configuration browser. `AURAGO_PHONON_SMOKE_DIR` opts into the actual ten-voice
CPU smoke test; point it at `deploy/phonon` after preparing the model. Run the
smoke check with networking disabled to verify bundled offline operation.
