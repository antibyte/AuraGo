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
