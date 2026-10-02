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

The installers can do all of this for you; see [Installer setup](#installer-setup).

## Installer setup

`install.sh`, `update.sh` and `install_service_linux.sh` ask once: "Use
Bluetooth in AuraGo (manage devices, headphones and speakers)?". The question
also appears without an adapter, so a dongle plugged in later works right
away. The answer is stored in `data/bluetooth-setup` (`enabled` or `declined`)
and is not asked again. Containers are never asked; Bluetooth is not supported
there.

Answer without the question:

- `AURAGO_BLUETOOTH=yes` or `AURAGO_BLUETOOTH=no` for all three scripts, for
  example `curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | AURAGO_BLUETOOTH=yes bash`
- `./update.sh --bluetooth` or `./update.sh --no-bluetooth` to change it later

Without a terminal and without a stored answer (for example the in-app
updater), nothing changes. `update.sh --yes` never answers the question.

On "yes" the scripts prepare the server. Each step is skipped when it is
already done, and a failed step never stops the installation; a summary lists
the commands to run by hand.

1. Packages: BlueZ, PipeWire with WirePlumber and its Bluetooth plugin, and
   FFmpeg when missing. An installed PulseAudio stays; then only its Bluetooth
   module and `pactl` are added.
2. `bluetooth.service` is enabled and started; a software block is lifted.
3. Linger for the service user (`loginctl enable-linger`), so its audio
   session runs without a login.
4. The user's audio units: `pipewire.socket`, `pipewire-pulse.socket` and
   `wireplumber.service`, or `pulseaudio.socket`.
5. WirePlumber may run its BlueZ monitor without a logind seat, which headless
   servers lack: `~/.config/wireplumber/wireplumber.conf.d/80-aurago-bluez-headless.conf`
   (WirePlumber 0.5) or `~/.config/wireplumber/bluetooth.lua.d/80-aurago-bluez-headless.lua`
   (0.4).
6. `/etc/systemd/system/aurago.service.d/aurago-bluetooth.conf` sets
   `XDG_RUNTIME_DIR=/run/user/<uid>` for the service, starts it after the
   user's session and adds the `bluetooth` group when it exists. If systemd
   rejects the file, it is removed again.
7. `config.yaml`: `bluetooth.enabled: true` and `bluetooth.allow_playback: true`.
8. A read-only check reports whether headphones and speakers can connect.

`config.yaml` changes only when the answer changes; afterwards the Config page
decides. With a stored "yes", every `update.sh` run repairs steps 1–6.

On "no" the scripts set `bluetooth.enabled: false` and remove the
`aurago-bluetooth.conf` drop-in. Packages, linger and WirePlumber files stay,
because other software may use them. To remove them by hand:

```bash
sudo loginctl disable-linger <service-user>
rm ~/.config/wireplumber/wireplumber.conf.d/80-aurago-bluez-headless.conf
```

A service that runs as `root` gets no audio session: device management works,
headphones and speakers do not.

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

- `readonly` stops the agent from pairing, connecting and disconnecting. Admins in
  the desktop app and on the Config page are not restricted by it.
- `allow_playback` lets the agent play, speak, read playback status and stop.
  The admin test tone works without it. Turning it off stops playback started by AuraGo.
- `audio_backend` accepts `auto`, `pipewire`, or `pulse`.
- `default_device` is an optional Bluetooth address. If it is empty, playback
  uses exactly one connected audio device; ambiguity is an error.

The Config UI can reprobe capabilities, discover devices, pair/connect them,
and play a short local test tone. These actions use saved configuration only.
An optional pairing PIN is held only for the current API request and is never
stored, logged, or exposed to the LLM-facing tool.

## Desktop app

The virtual desktop shows a **Bluetooth** app only while the server has a
Bluetooth adapter that is not blocked by a hardware switch. Plugging in or
removing a USB dongle shows or hides the app without a reload. The app can turn
the adapter on and off, scan, pair (including number comparison and passkey
entry), connect, disconnect, trust, remove, play a test tone and make the server
visible for 1–10 minutes so a phone can pair with it. Pairing questions appear
as a dialog with a 20-second timeout; unanswered questions are rejected.

If interactive pairing fails with "Bluetooth isn't available", check that the
AuraGo service user may register BlueZ agents (on most distributions membership
in the `bluetooth` group or the default BlueZ D-Bus policy is sufficient).

## Headsets in Live Speech

A paired headset with a microphone (Handsfree `0000111e` or Headset `00001108`
UUID) can be the audio device of Live Speech. `GET /api/realtime-speech/audio-devices`
lists such devices; the Live Speech bridge then switches the headset to HFP,
records with `pw-record` and plays with `pw-play`, and restores the previous
profile afterwards. HFP is mono speech quality; music played by the agent at
the same time also sounds like a phone call until the session ends. This needs
the PipeWire stack; PulseAudio is not supported for headsets.

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
  AuraGo or retry from the admin UI with a known numeric PIN, or pair from the
  desktop Bluetooth app, which can answer number comparison and passkey prompts.
- `BLUETOOTH_PROFILE_UNAVAILABLE` (BlueZ `br-connection-profile-unavailable`):
  the device paired, but the server runs no service for any of its profiles.
  Headphones and speakers only connect while PipeWire with WirePlumber or
  PulseAudio with Bluetooth support runs and registers its A2DP/HFP endpoints
  with BlueZ. Retrying does not help until that audio stack is running.
  `./update.sh --bluetooth` sets up that audio stack.
