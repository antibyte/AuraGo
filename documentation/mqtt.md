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

MQTT missions use a single dispatch worker with up to 256 waiting jobs. When
full, new jobs are dropped and counted. A dropped job does not consume the
mission's trigger interval.

## Relay processing and permissions

MQTT and Frigate messages run in their own internal background sessions. They
do not inherit normal chat history or create automatic conversation summaries,
memory extraction, personality changes or planner reminders. Their existing
tools remain available under the current configuration and the run's own
restrictions. Background operational failures remain recorded for diagnosis.

The controller runs one relay worker with at most 100 waiting messages. Disabling
MQTT or stopping the server cancels its current relay and joins the workers.
`/api/mqtt/status` exposes subscription failures and dropped relay/mission work
through `runtime.subscriptions` and `stats`; overload drops newly arriving work.

CYD publishing uses the live server permissions even before the first agent
turn. Disabled MQTT, read-only mode and Egg Mode continue to restrict publishing;
publication failures are recorded in the server log.

## Generated Python skills

The bundled MQTT publisher template requires successful connection and broker
acknowledgements, checks publication completion and bounds its receive buffer to
50 messages. Timeouts and rejected subscriptions produce an error result. Client
cleanup runs on every return path. These checks apply when generating a new
skill; existing generated skills remain unchanged.
