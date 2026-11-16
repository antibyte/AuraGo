# Embedded Tailscale lifecycle

Disabling embedded Tailscale in configuration cancels login, startup and exposure changes, then asynchronously stops all managed listeners and nodes, including Funnel, Homepage, Manifest, Space Agent and Store proxies. The save returns before shutdown finishes so a request arriving through Tailscale can complete. Use the existing Tailscale operation/status view to inspect completion or failure.

Repeated saves while disabled do not start additional shutdown work. Re-enabling permits the existing explicit Start action after shutdown finishes; it does not automatically reconnect. Late results from cancelled operations cannot republish listeners. Configuration keys, state directories and credentials are preserved.
