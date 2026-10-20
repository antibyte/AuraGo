# Tuya cameras

Open **Configuration → go2rtc → Tuya accounts**, or **Network Cameras → Tuya accounts**. Both entry points use the same account editor and camera setup dialog.

## Accounts and setup

- **Tuya Smart:** select the account's region, enter its email and password, and save. **Check saved account** or **Find cameras** lists supported cameras, including shared cameras returned by Tuya. A partial search is explicitly marked incomplete.
- **Tuya Cloud:** create and link a project in the Tuya Developer Platform, authorize **IoT Video Live Stream**, then enter its region, client ID, client secret and UID. Add each camera using its device ID. Cloud projects do not use the Smart account discovery endpoint.
- Direct Smart login in the pinned go2rtc 1.9.14 does not support Smart Life accounts. Use the Cloud project method for those accounts; AuraGo never removes or re-pairs devices automatically.
- Regions are selected explicitly. Smart and Cloud use separate fixed endpoint tables; the application does not try credentials against other regions.

Choose **HD** as the main stream and **SD** as the optional substream. Test each selected quality before adding a camera. If HD fails, **Use SD** changes the selection explicitly and removes the duplicate substream. Some cameras do not offer HD through Tuya WebRTC. A successful probe verifies a JPEG frame and reports its dimensions and codecs; these are the bounded test image's dimensions, not a claim about the camera's maximum resolution.

Camera IDs, snapshots, detection, playback modes and CPU/GPU profiles use the existing go2rtc media paths. Tuya snapshots come from the stream; no separate camera JPEG URL is invented. RTSP TCP/UDP options are hidden for account-managed Tuya cameras. Preloading, detection and talk are initially off. Talk additionally requires an advertised audio backchannel, `go2rtc.talk` or administrator rights, enabled LAN WebRTC and the existing push-to-talk lease.

## Saving and lifecycle

Account metadata and camera bindings are ordinary configuration. Login values and complete generated URLs are Vault-only. Empty or masked credential inputs preserve existing values. Changing login method requires all credentials for the new method. Saving does not log in or start an administrator-stopped sidecar; checks and discovery are explicit actions against a running sidecar.

An account update regenerates main/substream sources of its assigned cameras in one configuration/Vault transaction. Renaming an account keeps its source URLs and media revisions unchanged. Changes to credentials or region retire only the affected camera revisions and their media/talk sessions. Other cameras continue. An account cannot be deleted while referenced, including by disabled cameras.

Validation and publication failures roll back Vault changes. HTTP 202 means the desired configuration was saved but runtime reconciliation is pending; do not repeat the create request. Both the camera app and `PUT /api/config` use the same transaction. A config account list is a complete replacement; an omitted list preserves accounts and `[]` removes only unreferenced accounts.

Selection tokens last five minutes, bind identity/account/revision and are reserved until publication. Limits are 32 setup sessions, 128 camera candidates, 32 searched households and 1 MiB per discovery response. Discovery has a 20-second deadline; connection setup and active probes have a 45-second maximum. One search/probe per account may run concurrently, including native diagnostics. Canceling the dialog releases its requests and clears credential drafts. An abandoned check preserves the last confirmed service status. MQTT cleanup attempts the disconnect notification without a new acknowledgement timeout and allows at most 100 ms to drain its send queue.

## Network and sidecar

Tuya requires provider cloud services, MQTT and possibly provider STUN/TURN servers for the **camera-to-sidecar** connection. Browser playback keeps the existing empty public ICE-server list and LAN WebRTC policy. Warm connections and repeated detection/snapshots can increase traffic and battery drain.

Managed CPU and hardware images retain their reviewed digest pins and build the additional pinned Tuya patch. The private API must advertise `aurago_tuya=post-v1`; unsupported images fail visibly. The extension uses bounded POST requests and never exposes the upstream credential-bearing GET endpoint. Neither `api/tuya` nor the private `api/aurago/tuya` path is available through AuraGo's viewer or optional upstream web UI proxy. Source URLs remain absent from generated configuration, and upstream logs remain disabled.

Administration endpoints: `GET/POST /api/go2rtc/tuya/accounts`, `PATCH/DELETE /api/go2rtc/tuya/accounts/{id}`, and `POST /api/go2rtc/tuya/discover` or `/test`. They require administrator access and same-origin mutation requests. Agents can diagnose configured cameras under their existing rights; they cannot enumerate credentials, manage accounts or access the microphone.

## Verification

Local regression coverage uses private HTTP/Docker fixtures, real JPEGs and the pinned upstream source. Browser checks cover account selection, HD failure with explicit SD selection, opaque camera creation, cancellation, Standard/Fruity, narrow viewports and preserving other configuration drafts. Existing player, source-revision and talk tests remain applicable.

Real Tuya login, camera interoperability, simultaneous HD/SD use, battery behavior and audible talk require a separately enabled device acceptance test. No production instance is restarted by the local tests.

References: [go2rtc 1.9.14 Tuya](https://github.com/AlexxIT/go2rtc/blob/v1.9.14/internal/tuya/README.md), [Tuya regional endpoints](https://developer.tuya.com/en/docs/iot/api-request?id=Ka4a8uuo1j4t4).
