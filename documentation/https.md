# HTTPS, installation and browser access

Fresh AuraGo installations use HTTPS with an automatically generated self-signed
certificate. Native setup selects port 443 when it can bind `server.host`, then
8443; if both fail, initialization fails. The chosen port is saved. Port probing
cannot reserve a port until startup: a later conflict produces a startup error,
never a silent switch to HTTP or another port. Existing configs, including files
with no HTTPS section, retain their previous effective behavior on upgrade.

The installer asks separately about access from other devices and HTTPS. An
unattended install uses HTTPS on loopback only. A local certificate needs no
public domain or HTTP redirect listener. Verify the certificate before trusting
it in your browser/device. Self-signed HTTPS does not establish public trust;
install a trusted certificate or use a trusted HTTPS proxy when appropriate.

Docker uses internal port 8443 for newly generated configs. Compose publishes
host port 8443; `AURAGO_HTTPS_PORT=443` selects host port 443 when it is free. Existing and
user-supplied configs are preserved. Match your published ports to any custom
internal listener. Healthchecks use `aurago --healthcheck`, which checks the
configured HTTPS or HTTP listener (including HTTP Egg deployments). Internal
agent requests retain the separate loopback HTTP listener on `server.port`.

## Initial configuration CLI

With a config file already copied from the release template:

```bash
./aurago --config config.yaml --init-only --https=true --host=127.0.0.1 \
  --cert-mode=selfsigned --https-port=0 --http-port=0
```

Only explicitly supplied flags are written, together and atomically. Omitting
`--https` preserves the setting; `--https=false` deliberately disables it.
`--https-port=0` performs the initial 443/8443 selection; supply a nonzero port
to choose it explicitly. Host selection is independent of HTTPS. Write failures
return a nonzero status. Runtime config loading never performs automatic port
selection. Keep your bind capability when replacing a Linux binary, or use 8443.
The systemd installers retain `CAP_NET_BIND_SERVICE` for low ports.

Let’s Encrypt is an explicit installer choice with a public domain and email.
Public port 80 must reach the configured HTTP-01 challenge listener; choose
`cert_mode: auto` and `http_port: 80`. With `http_port: 0`, there is no HTTP
listener; TLS-ALPN-01 then requires public port 443 to reach AuraGo directly.
Public DNS, firewalls, forwarding and certificate issuance need live validation.
Older public-domain auto-TLS configs that omit `http_port` retain their previous
port-80 challenge listener. An explicitly configured `http_port: 0` stays disabled.
Custom certificates continue to use `cert_mode: custom` with your certificate
and key files. All TLS listeners and preflight probes honor `server.host`.

## Setup, HTTP opt-out and restart

The Setup Wizard displays HTTPS status and checks `window.isSecureContext`.
Browser support and device permissions are separate requirements. Many Desktop
features, especially voice chat and microphone access, can fail over remote
HTTP. Localhost and a trusted HTTPS proxy can still provide a secure context.
See [MDN: Secure contexts](https://developer.mozilla.org/en-US/docs/Web/Security/Defenses/Secure_Contexts).

Change HTTPS in **Config → Server** after setup. Switching it off opens the
shared confirmation dialog. Cancelling preserves the switch and all drafts;
confirming only edits the draft. Use **Save**, then restart. The saved response
shows the future address. Direct access changes to its new protocol and port;
proxy origins and unknown public port mappings remain unchanged. If you use
Docker mappings, update them yourself and open the published address. New
certificates may require a browser trust step on the next visit. A new origin
may also require signing in again.

Before restarting into HTTP, AuraGo sends `Strict-Transport-Security: max-age=0`
on the successful HTTPS save response and subsequent built-in TLS responses.
That clears AuraGo's own HSTS policy in the current browser. Other browsers may
need to visit HTTPS before the restart too. Browsers ignore HSTS sent over HTTP
or a connection with certificate errors. A parent's `includeSubDomains`, a
browser preload entry or a proxy's HSTS header may still force HTTPS; AuraGo
cannot remove those policies. Keep a working trusted HTTPS route until those
policies are resolved. See [MDN: HSTS](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Strict-Transport-Security).
