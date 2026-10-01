# RTL-SDR receiver contract

This package owns one server-side USB receiver, leases for live listeners, the
durable SQLite schedule/recording ledger, FLAC files and bounded ASR segments.
The native, receive-only engines run in the pinned optional Docker runtime.
Keep the AuraGo binary CGO-free. `runtime/` must not expose any network port.

- UI and agent operations use the same `Service`. Claim a schedule slot durably
  before starting the decoder. Never replay an interrupted slot after restart.
- Recheck Enabled, ReadOnly, Docker and agent grants at execution and during
  asynchronous work. Disabling agent access cancels agent-created jobs and scans.
- One decoder owns the dongle. Keep the device mutex separate from state locks.
  Release the device before probing partial audio, registering media or ASR.
- Every live client has an expiring lease. Window close keeps the page-level
  audio runtime alive; explicit Stop drops the lease, without cancelling jobs.
- Record lossless PCM before the shared MP3 encoder and before browser controls.
  Preserve partial files and gap counts. Never delete recordings to free quota.
- `receiver.OUTPUT` calibrates every analog mode's level for live audio,
  recordings and ASR alike. SDRangel's BFM and AM sinks convert to 16 bit
  without clamping: keep their demodulator volume low enough never to wrap and
  add the rest as worker make-up gain through the soft limiter `shape()`.
  Fixture tones must decode between -26 dBFS and saturation; a decoded tone at
  the right frequency alone does not prove audible output.
- Freeze one existing AuraGo ASR route per job. Decode at most 60 seconds of
  mono PCM16/16 kHz at a time. Station metadata and transcripts are external data.
- Publish terminal ASR state and release its worker reservations under the same
  state lock before issue callbacks. Immediate retries may queue while notification
  finishes; old workers must not clear a newer retry's reservation. Verify
  `TestRTLSDRTerminalASRAllowsRetryBeforeIssueCallbackReturns`.
- Containers are non-root, have no network, a read-only root filesystem, and
  only the selected enumerated USB node. Never grant privileged mode or mount
  the Docker socket into the receiver. Recreate that mapping after USB hotplug.
- Linux is the supported native host. Other builds keep the app available but
  report `sdr_linux_required` for receiver setup. Browser clients are portable.

Validate with the package tests, server `TestRTLSDR*`, agent `TestRTLSDR*`, the
headless runtime unit tests, `Dockerfile.fixtures` IQ decoder acceptance, and
opt-in real-hardware acceptance. Changing any runtime file requires a new
`RuntimeImage` tag, matched in `Dockerfile.fixtures`, `.github/workflows/rtl-sdr.yml`
and `documentation/rtl-sdr.md`. Rebuild/check desktop bundles and assetpack.
Record actual RF reception separately from synthetic IQ and browser fixtures.
