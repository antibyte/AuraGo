# Bluetooth devices and audio output

AuraGo supports Bluetooth on Linux through BlueZ's system D-Bus API. At startup
it performs a passive capability probe: no discovery scan, pairing, connection,
or audio-default change is made. The native `bluetooth` tool is only registered
when BlueZ is reachable and at least one adapter is powered on.

Windows and macOS builds contain unsupported stubs so the portable binary keeps
compiling, but Bluetooth operations are not offered on those platforms.

## Requirements

Install and enable BlueZ plus one supported user-session audio stack:

- BlueZ service and tools: commonly the `bluez` package and `bluetooth.service`
- PipeWire: `pw-dump`, `pw-play`, a running per-user PipeWire session, and
  WirePlumber
- or PulseAudio: `pactl`, a running per-user PulseAudio session, and an FFmpeg
  build with the `pulse` output muxer
- FFmpeg for decoding local music, generated music, TTS, and the UI test tone

Typical Debian/Ubuntu packages are:

```bash
sudo apt install bluez ffmpeg pipewire pipewire-audio wireplumber
```

For a PulseAudio installation, use the distribution's `pulseaudio-utils`
package instead of the PipeWire packages. Package names differ by
distribution.

AuraGo must run in the same user session that owns the PipeWire or PulseAudio
socket. A system service without access to that session can detect BlueZ but
will report Bluetooth audio as unavailable.

## Configuration

```yaml
bluetooth:
  enabled: true
  readonly: true
  allow_playback: false
  scan_timeout_seconds: 10
  default_device: ""
  audio_backend: auto
```

- `readonly` blocks pairing, connect, and disconnect operations.
- `allow_playback` separately enables play, speak, playback status, and stop.
  Turning it off stops playback started by AuraGo.
- `audio_backend` accepts `auto`, `pipewire`, or `pulse`.
- `default_device` is an optional Bluetooth address. If it is empty, playback
  uses exactly one connected audio device; ambiguity is an error.

The Config UI can reprobe capabilities, discover devices, pair/connect them,
and play a short local test tone. These actions use saved configuration only.
An optional pairing PIN is held only for the current API request and is never
stored, logged, or exposed to the LLM-facing tool.

## Playback behavior

`play` accepts exactly one workspace/data-local `local_path` or an audio/music
item from the Media Registry. Prefix data-relative paths with `data/`. URLs and
streaming-service identifiers are not accepted. The selected device must
already be paired; AuraGo may connect it but never pairs it implicitly.

PipeWire playback uses a target-specific `pw-play` stream. PulseAudio playback
uses FFmpeg's Pulse output with the matched Bluetooth sink. AuraGo does not
change the system default device. A new `play` or `speak` replaces the previous
AuraGo-owned Bluetooth stream, and `stop` affects only that stream.

## Docker

Bluetooth is deliberately unavailable in the standard AuraGo container.
Passing the host system D-Bus socket and a user's audio-session socket into a
container changes the host security boundary and is therefore not enabled or
documented as a default deployment path. Use a native Linux installation for
Bluetooth.

## Troubleshooting

- `BLUETOOTH_UNAVAILABLE`: verify `bluetoothctl show`, the BlueZ service, and
  that an adapter is powered on.
- Audio unavailable: run `pw-dump` and `pw-play --help`, or `pactl info`, as the
  same user that runs AuraGo.
- PulseAudio unavailable: verify `ffmpeg -hide_banner -muxers` lists `pulse`.
- No sink after connect: wait for WirePlumber/PulseAudio to create the A2DP or
  LE Audio sink, then use **Detect again** in the Bluetooth settings.
- `PAIRING_INTERACTION_REQUIRED`: complete display/confirmation pairing outside
  AuraGo or retry from the admin UI with a known numeric PIN.
