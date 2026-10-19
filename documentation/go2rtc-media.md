# go2rtc camera media

One logical camera keeps its stable ID and existing main-source Vault key.
Optional `sub_source` and `snapshot_source` use separate Vault keys; JSON/YAML
responses contain only configuration indicators. Empty or masked input preserves
each source. `clear_sub_source` and `clear_snapshot_source` explicitly remove it.
Both configuration screens use the same validation and Vault transaction. ONVIF
reserves main/sub/snapshot selection together; a failed save releases the reservation.

Preview and detection `auto` prefer the substream; detail viewing starts with the
main stream. Snapshot `auto` prefers a camera snapshot and permits one stream
fallback, reported separately from the last successful image time. Rotation and
bounded proportional fitting run locally after JPEG retrieval. Snapshot cache,
HTTP media, HLS and WebSocket leases belong to a camera revision. Changing one
camera's sources first registers all replacement aliases, then publishes the new
revision and revokes its old leases without restarting other cameras. Failed or
superseded registrations retain the active revision and clean partial aliases.
Status exposes `media_ready` and `source_pending`; managers additionally receive
the saved `configured_media` settings while playback still uses its active revision.

`original` keeps the original codecs. `h264_opus` and `h264_aac` create fixed
server-generated FFmpeg templates over internal aliases. Video transcoding is
on demand, shared by viewers of the same role/profile, with two different active
profiles maximum. Configured transcoding raises the sidecar to 2 CPUs / 1 GiB;
the CPU limit is capped to the Docker host's CPU count. Preload is optional,
warms only the main source and never counts as a viewer. go2rtc writes its native
preload configuration into a source-free copy in container tmpfs; the host
configuration mount remains read-only.

CPU is the default backend. VAAPI requires one `/dev/dri/renderD*` host node and
its numeric non-root render group. CUDA/NVENC requires one GPU index or UUID.
Only Linux/amd64 Docker hosts support these GPU modes. The hardware image is
`alexxit/go2rtc:1.9.14-hardware@sha256:7fce3ffefe663753510ff4671d8b7496dead49b46c87392898ddef152c25687d`.
The selected node is mapped to `/dev/dri/renderD128`; no privileged container is
used. NVIDIA visibility is disabled unless CUDA and a particular GPU are selected.
Save, start the sidecar and use **Test saved hardware** before selecting GPU
transcoding for playback. Successful verification is stored privately with its
timestamp and tied to the image, hardware settings and Docker host. Failed tests
revoke verification; playback reports the failure and never silently uses CPU.

## WebRTC lifecycle patch

Upstream 1.9.14 does not close asynchronous WebRTC peers when their signaling
WebSocket closes. AuraGo's managed image applies the small reviewed patch in
`internal/tools/go2rtc_sidecar/lifecycle.patch` to upstream commit
`b5948cfb25404cc5cb37b166ecaa2dca20b11d4b`. The patch also snapshots the source map during status reads and locks alias deletion, avoiding concurrent map access during source changes. The source archive and compiler image
are checksum pinned. CPU and hardware builds retain the respective pinned
upstream image as their runtime base and replace only the go2rtc executable.
The image tag includes the base, build recipe and patch hash. Actual upstream
Pion tests run during the build and prove socket revocation and failed-offer cleanup.

On the next explicitly requested/previously configured managed start, an absent
local image is built using the Docker Engine build endpoint. This needs build
permission and network access to the pinned source, compiler and Go modules.
It does not require a Docker CLI on the AuraGo host. A failed build is visible;
there is no fallback to unpatched WebRTC. Custom images remain unchanged, but
their private API must attest `aurago_webrtc_lifecycle: ws-close-v1` before
AuraGo enables WebRTC/talk. Viewing through MSE/HLS/MP4/MJPEG remains available.
HLS requires native HLS playback support in the browser. Public STUN servers
are disabled in both the browser and sidecar.

## Talk and diagnostics

Talk requires enabled LAN WebRTC, the per-camera talk switch, a secure browser
context for microphone capture, and an administrator session or `go2rtc.talk`
token. A `go2rtc.view` token grants no sending rights. One identity-bound talk
lease per camera accepts audio-only `sendonly` offers, expires after 3 seconds
without the one-second heartbeat, and lasts at most 60 seconds. Release, focus
loss, hiding, closing and camera revision changes stop capture and the lease.
The patched sidecar terminates the media peer on socket revocation. Camera
backchannel/codec support is required. No agent operation accesses a microphone.

Administrator `POST /api/go2rtc/diagnostics/{id}` and agent `diagnose_stream`
return the same bounded sanitized diagnosis: container/API state, stage/error
code, duration, image dimensions and codec/backend information. Hardware testing
is a separate explicit administrator action; diagnostics do not start containers.
The Docker status probe respects the request deadline and a three-second cap;
`container_state` distinguishes `running`, `stopped` and `unknown`.

## Validation boundaries

Focused Go tests cover authentication, SDP direction, leases, source revisions,
Vault rollback, image transforms, telemetry, Docker build/payload and hardware
selection. UI smokes exercise the real pinned player fixtures and Standard/Fruity
camera controls. Set `AURAGO_RUN_BROWSER_SMOKE=1` to run browser tests. Set
`AURAGO_GO2RTC_DOCKER_TEST=1` only on a disposable Docker host for the real-image
contract. Live camera backchannels, vendor-specific ONVIF profiles, GPU drivers,
remote network reachability and production upgrades need separate opt-in acceptance.
