# RTL-SDR radio

RTL-SDR is a built-in Virtual Desktop app for a USB receiver attached to the
AuraGo server. The browser provides the controls and speakers; the server owns
reception, recording, schedules and transcription. Closing the app window leaves
live audio playing through a small desktop control. **Stop** ends that live
session. Closing the browser ends its listening lease after at most 35 seconds;
server recordings continue.

## Setup

The receiver backend requires Linux, a local Docker Engine and a supported
RTL2832/RTL2838 dongle. Run AuraGo as an ordinary, non-root service user with
access to Docker. No physical sound card is needed. Browser clients can run on
Linux, Windows or macOS. Native Windows/macOS receiver setup is not supported in
this version.

1. Enable Virtual Desktop and Docker in AuraGo. Open **RTL-SDR → Receiver setup**.
2. Select the dongle, enable the radio, and choose **Prepare receiver**. The
   versioned optional container image is downloaded on demand. Enabling the
   integration alone does not start a receiver.
3. Choose WFM and a known local FM frequency, then **Listen**. For DAB+, run the
   Band III scan and select a discovered audio service. A suitable antenna and
   local RF coverage are required; USB detection alone cannot prove reception.
4. Enable agent access if the agent should record or schedule radio programmes.
   Transcription uses the existing chat-input ASR settings, including Speech Lab.

The setup reports the physical USB port, device identity and bound driver. The
worker receives only that device node and its existing group permission. The
bundled librtlsdr attempts to detach the DVB driver while receiving and releases
it afterward. AuraGo never blacklists drivers or changes host USB permissions.
If setup reports a permissions error, grant group read/write access to the
selected receiver through the host's device policy, then prepare it again.

For AuraGo itself running in Docker, use a writable persistent mount for its data
directory, expose the local Docker socket, and make the selected USB node and
USB sysfs metadata visible. The manager resolves its own mounted data directory
to the host path; it does not mount the AuraGo container's overlay filesystem.
After USB hotplug, that outer container must also see the new device node. Native
host deployment avoids this additional outer-container device mapping.

The equivalent configuration is:

```yaml
rtl_sdr:
  enabled: false
  read_only: false
  allow_agent: false
  device: ""        # Auto-select only when exactly one dongle is present.
  quota_gb: 10
```

Radio and Desktop read-only settings, Docker permissions and agent access are
rechecked while work runs. Revoking access cancels affected work and preserves
captured audio. Setup/configuration changes require a Desktop administrator.

## Listening and recording

The receiver supports WFM stereo/RDS, NFM, AM, USB, LSB and DAB+. Frequency entry,
individual digits, the tuning knob, keyboard arrows and spectrum clicks control
the same receiver. Available frequency limits and gain steps come from the tuner.
Advanced controls provide bandwidth, gain, AGC, PPM correction and squelch.
During live listening, committing a frequency entry or changing modulation and
advanced controls tunes immediately. The player discards old buffered audio on
each successful retune. AGC controls both RF tuner and digital gain; switch it
off to choose a manual tuner gain. Runtime image `:2` includes the RF AGC fix
against the pinned SDRangel source (upstream's AGC controls digital gain only).
Unsupported frequencies are rejected. DAB+ service names, radiotext and reception
quality come from the ensemble; scanning can take several minutes.

One dongle receives one station at a time. Multiple browser listeners share that
station. Volume and mute are local to each browser and never change recordings.
Same-mode analog tuning keeps the decoder and audio sink alive. Changing to or
from DAB+ fully releases the previous receive chain.

Record immediately, once, daily or weekly. The default duration is ten minutes;
the limit is two hours. Schedules use an explicit IANA timezone such as
`Europe/Berlin`. Daily and weekly schedules retain local wall time across DST:
a nonexistent spring-forward time is skipped, and an autumn repeated hour runs
once. Overlapping reservations, including 30 seconds of warmup, are rejected.

The desktop warns up to one minute before a recording. The server reserves and
tunes the receiver 30 seconds before the start, switches any live listeners, and
restores their previous station afterward if a live lease remains. The typed
scheduler needs no LLM or open browser. Slots are claimed durably before execution;
a restart marks an interrupted capture and never silently replays it. Missed slots
are shown as missed instead of recording an unrelated later programme.

Audio is stored as FLAC under the configured data directory's `rtl-sdr/` folder
and registered in the media library. The app offers playback, download, transcript
segments and explicit deletion. The default quota is 10 GiB and can be changed;
AuraGo never deletes older recordings to make room. Admission reserves up to
200 KB per recorded second; the writer also enforces the remaining quota.

USB removal or reception gaps preserve a partial recording. Reinserting the
dongle at its selected port renews the device mapping; a still-active live session
can recover, while an interrupted recording remains an explicit partial result.

ASR uses one frozen existing route per job and converts audio to mono PCM16 at
16 kHz in chunks of at most 60 seconds. Timestamps identify those audio chunks,
not individual words. Silence/no speech and failed chunks stay visible. Retry
transcription reuses saved audio, preserves successful chunks, and does not claim
the receiver. A retry can run after the USB device has been removed.

## Agent and API

The native `rtl_sdr` tool shares the same service as the UI. Operations are
`status`, `stations`, `scan`, `record`, `schedule`, `recordings`, `schedules`,
`result`, `stop_recording`, `transcribe` and `delete_schedule`. Recordings and
schedules return durable IDs; scan status is available through `status`.
Station/recording lists and transcript results support bounded `offset` pages.
Station metadata and transcripts are isolated as external content.

For example, ask the agent to record a known station daily at noon for ten minutes
and transcribe it. The agent must first obtain the actual frequency or DAB service
ID; a programme title alone does not identify a receivable station.

The authenticated API is under `/api/desktop/rtl-sdr/`: `state`, `devices`,
`receiver`, `config`, `setup`, `tune`, `heartbeat`, `stop`, `stream`, `scan`,
`favorites`, `recordings` and `schedules`. Recording IDs expose `audio` and
`transcribe` subroutes. All native SDR/audio ports stay internal; browser traffic
uses AuraGo's normal authenticated origin. Transcription failures never prevent
downloading an available recording.

## Runtime package and validation

The native sources are pinned in `internal/rtlsdr/runtime/Dockerfile`: SDRangel
7.27.2, welle.io 2.7 and rtl-sdr-blog 1.4.0. Sources and license notices are retained
in the image. The worker runs without networking, Linux capabilities, a writable
root filesystem, a host sound card or a Docker socket. Only its Unix control
socket and the selected USB device are accessible. AuraGo remains CGO-free.

Build the currently expected image locally on the server:

```sh
docker build -f internal/rtlsdr/runtime/Dockerfile \
  -t ghcr.io/antibyte/aurago-rtl-sdr:2 .
docker build -f internal/rtlsdr/runtime/Dockerfile.fixtures \
  -t aurago-rtl-sdr-fixtures .
docker run --rm --network none --cap-drop ALL \
  --security-opt no-new-privileges:true aurago-rtl-sdr-fixtures
```

The fixture image generates original analog and DAB+ IQ files, decodes them with
the actual pinned engines, and verifies audio tones, stereo separation, spectrum,
service discovery and continuous analog retuning. It never transmits RF and is
not installed by the app. `.github/workflows/rtl-sdr.yml` runs these checks on
amd64 and arm64. Its explicit `publish` input builds/signs the multi-platform GHCR
package after those checks; adding the workflow does not itself publish an image.
Increment `RuntimeImage` and the matching Docker/CI tags when changing a released
receiver package.

Go checks cover ownership, recording priority, durable claims, restart, DST,
quota, USB loss, ASR errors/retry, current permissions, API boundaries and all
16 desktop languages. `AURAGO_RUN_BROWSER_SMOKE=1` enables `TestRTLSDRDesktopBrowser`.
Rebuild/check UI bundles and package the version-bound web resources as described
in [web-assets.md](web-assets.md).

The Linux-only `TestRTLSDRHardwareAcceptance` is opt-in through
`AURAGO_RTLSDR_TEST_DIR` and `AURAGO_RTLSDR_TEST_CONFIG`; it uses an isolated data
directory and reads the existing Speech Lab configuration without modifying it.
`AURAGO_RTLSDR_FREQUENCY_HZ` selects a known receivable test frequency. Synthetic
IQ and original test speech verify decoders/ASR separately from real over-the-air
reception. Final site acceptance also needs a known FM station, a DAB+ ensemble
and an intelligible broadcast recording with transcript.

Upstream interfaces: [SDRangel Server](https://github.com/f4exb/sdrangel/blob/97e9e21fe9754b363ae70caac822802d908e8acd/sdrsrv/readme.md),
[BFM stereo/RDS](https://github.com/f4exb/sdrangel/blob/97e9e21fe9754b363ae70caac822802d908e8acd/plugins/channelrx/demodbfm/readme.md),
[welle-cli](https://github.com/AlbrechtL/welle.io/tree/512558d1f8ac4c524d3c63e97510ea36c1bd7a70).
