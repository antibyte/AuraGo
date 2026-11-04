# MQTT configuration and operation

Open **Config → Integrations → MQTT**. Enter a complete broker URL, for example
`tcp://localhost:1883` or `mqtts://broker.example:8883`. Topics are a list of MQTT
filters; the UI accepts comma-separated entries and saves an empty control as an
empty list. Main and availability QoS accept the numeric values 0, 1 and 2.

## TLS and credentials

The URL determines whether TLS is used. Secure schemes such as `mqtts`, `ssl`
and `wss` always use TLS, including when the explicit TLS switch is off. Enabling
TLS with a plaintext URL such as `tcp` or `ws` is rejected. Change the URL and
port explicitly to match the broker; AuraGo does not guess either value.

CA files must contain valid PEM certificates. Client authentication requires a
matching certificate and private key. Invalid or unreadable files are rejected
before saving. Paths refer to files accessible to the AuraGo server process.

The password comes from Vault `mqtt_password`, then `MQTT_PASSWORD`, then an
empty value. Leading and trailing spaces are preserved. Deleting the Vault
entry restores the environment fallback if present. The status view reports
the source without exposing its value.

Broker URLs must not contain `user:password@` credentials. Use the username
field and Vault instead; URL credentials would override the selected password
inside the MQTT driver and are rejected before saving or connecting.

## Applying changes and checking the connection

Settings and Vault password changes take effect without a server restart.
Broker, client ID, credentials, TLS, session and availability changes replace
the connection. Topic, QoS, buffer, relay and read-only changes keep it open.
A rejected new password does not restore an older credential.

The status endpoint retains the existing fields and adds `connection_state`,
`config_revision`, `applied_config_revision`, `active_broker`, `active_client_id`,
`effective_tls`, `transport_scheme` and `credential_source`. During a change,
the desired broker may differ from the active broker. `connected` means an
open, confirmed connection; a reconnect attempt alone is not a success.

**Test connection** uses the saved settings and a separate short-lived client.
Save edits before testing. Cancelling the request or reaching the connection
timeout closes its socket. Disabling MQTT cancels relay work and closes the
runtime connection. A graceful stop attempts the configured offline
availability only while its connection is still open.

## Subscriptions and missions

Configuration, Frigate, each enabled MQTT mission and manual tools own their
subscriptions independently. Identical filters share the highest requested QoS;
overlapping wildcard filters remain distinct. Changing or deleting a mission
updates its ownership automatically, including when MQTT is enabled later.

`mqtt_unsubscribe` removes the manual owner only. If configuration, Frigate or a
mission still needs the filter, its result says that the broker filter remains
active. Read-only mode prevents publishing and manual subscription edits;
configured and mission reception remains available.

SUBACK results are checked for every requested filter. Rejected or missing
grants remain visible as failed desired work and retry after a reconnect or a
relevant settings change. A mixed response does not erase successful grants.

With `clean_session: false`, an atomic local ledger tracks known filters and
confirmed manual subscriptions for the broker/client identity. Keep this ledger
with the AuraGo data directory when migrating or restoring an installation.
It contains no message payloads or credentials. Unknown broker subscriptions
from installations without a ledger are not deleted automatically; messages
outside the current desired filters are ignored.

MQTT missions use one dispatch worker with up to 256 waiting jobs. Their topic
and `payload_contains` matching use the original complete text, independently
of observation-buffer truncation. Text above 64 KiB and binary payloads do not
start missions. Refused, quarantined, expired and queue-dropped deliveries do
not consume the mission's trigger interval.

## Relay topics and action permissions

After upgrading, MQTT and Frigate relays pause until both **Relay topics** and
**Relay action permissions** are configured. Broker subscriptions and buffered
observation continue. The MQTT page displays the saved pause reason; granting
permissions never changes your subscriptions or enables a transport exception.

Choose relay filters with a concrete first level, such as `home/door` or
`frigate/events`. Global `#` and `+/state` relay filters are rejected. Manual or
mission subscriptions do not implicitly become generic relay triggers.

Each relay action needs an exact tool, operation and target grant, even when
the general Action Policy is disabled. The UI supplies action dropdowns and
target examples. Add only the actions that this event should be able to perform:

```yaml
mqtt:
  topics: [home/door, frigate/events]
  relay_to_agent: true
  relay_topics: [home/door, frigate/events]
  relay_grants:
    - tool: mqtt_get_messages
      operation: read
      target: tool:mqtt_get_messages
    - tool: mqtt_publish
      operation: publish
      target: mqtt:home/light/set
    - tool: home_assistant
      operation: call_service
      target: ha:light.desk/light.turn_on
    - tool: send_notification
      operation: execute
      target: notification:push
```

Publishing uses `mqtt:<exact-topic>`; subscription edits use
`mqtt_filter:<exact-filter>`. HA reads use `ha:<entity_id>` and service calls use
`ha:<entity_id>/<domain.service>`. Every entity in a multi-entity service call
needs a grant; area/device/label/floor selectors and service-data target overrides
are refused. Notifications name one
configured channel (`ntfy`, `pushover`, `telegram`, `whatsapp`, `discord`, `push`,
`telnyx` or `cyd`). Wildcard action grants, code execution, skills, MCP calls and
delegation are unavailable to relays. An optional `arguments_hash` narrows a
rule using the same canonical SHA-256 semantics as the general Action Policy.

Current and captured permissions must both allow the call. Removing a grant or
relay topic takes effect on an in-flight run without restart. Existing
integration activation, read-only, HA service, Guardian and optional general
Action Policy restrictions still apply. Registered secrets cannot be published.
MQTT missions retain their explicitly configured mission instructions and their
normal execution permissions instead of inheriting these generic relay grants.

## Authentication and transport

All MQTT-triggered agent work requires complete broker credentials and verified
TLS by default. A username alone does not qualify: provide the Vault password
or a valid TLS client certificate/key pair. Client credentials do not establish
who published an event; restrict publishers with broker ACLs.

`allow_unauthenticated_relay` waives authentication only.
`allow_untrusted_transport` waives trusted TLS only. An anonymous plaintext
broker therefore needs **both** explicit exceptions. Missing or null values
remain false after upgrades; old configurations no longer gain an automatic
transport exception. Existing explicit values remain stored, but relays still
need their new topic/action grants. The Security panel reports unsafe exceptions
and missing credentials without exposing their values.

## Load, payloads and status

MQTT and Frigate run in separate internal background sessions without normal
chat history or automatic memory/personality/planner effects. Their model input
keeps at most 16 KiB of payload and a truncation marker. High-threat or incomplete
local scans start no model run: a fixed, deduplicated security diagnostic records
only a digest. Missions scan their whole accepted text before interval admission.

One controller admits at most ten agent jobs per rolling minute, shared by
relays and MQTT mission fan-out. Relay topics debounce at reception for two
seconds. The relay queue holds at most 100 jobs; the mission queue holds 256.
Waiting jobs expire after 60 seconds. Relay runs last at most 120 seconds and
four tool calls, or a lower configured limit. Category `mqtt` participates in
existing daily budget enforcement, including blocking in `partial` mode.

The observation buffer defaults to 500 messages, 256 KiB per payload and
`buffer.max_bytes: 16777216` (16 MiB) total topic/payload bytes. Zero selects the
default; the total limit accepts 1–256 MiB. Oldest messages are removed when a
count, age or byte limit is reached. Binary input (invalid UTF-8 or NUL) stores
only topic/length metadata with `payload_binary: true`; `payload_bytes` retains
the original length and `payload_truncated` identifies shortened text. No binary
content reaches an agent. `mqtt_get_messages` supports MQTT filters including
`+`, `#` and explicit `$SYS/#`; an empty filter returns all buffered messages.

`/api/mqtt/status` includes `relay_enabled`, `relay_pause_reason`, `buffer_bytes`,
`max_buffer_bytes`, queue lengths under `runtime`, and distinct `stats` for
truncation, binary omission, rate limiting, queue overflow, expiry and quarantine.
Overload warnings are rate limited. Disabled MQTT or server shutdown cancels
active relays and joins the workers before dependent services close. CYD
publishing keeps its existing live permission checks and failure logging.

## Generated Python skills

The bundled MQTT publisher template requires successful connection and broker
acknowledgements, checks publication completion and bounds its receive buffer to
50 messages. Timeouts and rejected subscriptions produce an error result. Client
cleanup runs on every return path. These checks apply when generating a new
skill; existing generated skills remain unchanged.
