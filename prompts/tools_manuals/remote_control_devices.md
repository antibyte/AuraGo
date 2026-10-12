# remote_control_devices

Device status includes effective rights, capabilities and configuration state.
`pending` means saved but not yet acknowledged; `confirmed` is a classic-client
ACK; `error` requires reconnect/diagnosis; `server_enforced` describes AgoDesk
server policy without claiming a client ACK. Revocation closes all sessions and
cancels work. A revoked device needs a fresh one-time token to enroll again.
Missing telemetry is unknown (`null`), not a zero measurement.

List, inspect, and revoke connected remote-control devices.

Use `device_id` when available; `device_name` is accepted as a lookup alias.
