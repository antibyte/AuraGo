# Isolated Desktop app previews

VM web previews and Store web apps can use a separate HTTPS site. The gateway
keeps AuraGo sessions and management credentials out of guest applications while
preserving guest JavaScript, relative assets, login cookies, APIs, uploads and
WebSocket upgrades. CommandCode uses the same gateway for its preview port.

## Trusted homelab use

Isolation is optional for trusted applications on a private administrator-owned
network. It does not require a new domain, local alias or TLS deployment unless
the operator chooses to enable the gateway. Legacy previews retain their shared
browser trust boundary; credential filtering does not make them isolated.
Keep public exposure and externally supplied active content outside that trust
assumption: use isolated previews before publishing or running untrusted content.
Both legacy VM previews and the isolated gateway remove reserved AuraGo response
cookies while preserving guest login cookies. CommandCode removes AuraGo cookies
and internal credential headers on HTTP and WebSocket requests to dev servers,
without stripping the guest application's own Authorization or CSRF headers.
Legacy CommandCode responses can still set cookies for the shared host; this
remains part of the trusted homelab boundary, not an isolated guest environment.
CommandCode source changes take effect in installed apps only after image
publication and an explicit Store update; neither happens during local testing.

## Prepare without switching existing apps

```yaml
server:
  https:
    domain: desktop.example.org
  preview_domain: apps.example.net
  preview_enabled: false
```

The preview domain must belong to a different registrable site from AuraGo. For
example, `desktop.example.org` and `apps.example.net` are suitable; two subdomains
of `example.org` are not. An empty domain disables preparation. Merely setting
the domain does not change existing app launches.

Set AuraGo's primary DNS hostname in `server.https.domain` (or `server.host`
when binding directly to a DNS name). The gateway validates this identity before
dispatching preview hosts. Missing or overlapping domains leave the existing
AuraGo router intact and reject isolated launches with a configuration error.

Route `*.apps.example.net` through the existing HTTPS ingress to AuraGo, preserving
Host and WebSocket upgrades. Configure wildcard TLS at that ingress, or supply an
appropriate custom certificate to AuraGo. The built-in single-domain ACME flow
does not provision wildcard certificates. When TLS terminates at a reverse proxy,
configure its exact source address in `server.https.trusted_proxy_cidrs` and enable
`behind_proxy`. Untrusted forwarding headers never establish HTTPS. The external
HTTPS port is retained in generated URLs, including nonstandard development ports.

Only managed resource IDs and persisted Store ports select upstreams. A native
AuraGo process reaches local published ports; a remote Docker daemon requires a
reachable LAN binding. In Docker, AuraGo and the app must already share a network;
the gateway resolves that network through the Docker API. Preparation does not
change running containers, expose extra ports or attach them to new networks.
Check reachability from the actual AuraGo deployment before enabling the switch.

For staging, append `isolated=1` to an authenticated Store `open-url` API request
(preserving `port_id`, when present), or to a VM `/web/{port}/` link. The Store
response keeps its existing `status`, `url`, and `app` fields. Open the returned
URL in the same context that will host the app: an iframe for embedded use, a new
tab for external use. Launch URLs expire after one minute and can be used once.
Request a new launch URL for each retry or new window; never publish or log it.

## Acceptance and activation

Keep `preview_enabled: false` until these checks pass on the intended browser,
ingress and runtime, for every installed app and enabled preview port:

- HTTPS certificate validation, correct wildcard routing, and upstream reachability.
- JavaScript and relative assets; existing app login/logout and OAuth redirects.
- Guest session and CSRF cookies; upload, download and API mutations.
- WebSocket traffic, reconnects, reloads, and opening a second window.
- CommandCode live preview and Uptime Kuma's dashboard destination.
- AuraGo logout/token revocation closes active gateway connections, while another
  valid AuraGo session remains usable. Readonly blocks mutating methods and
  write-capable WebSocket upgrades without closing ordinary readers.
- Guest code cannot read AuraGo cookies or authenticated AuraGo API responses.

Gateway and guest cookies are host-bound, Secure and partitioned. This permits
embedded apps in current browsers with third-party cookie restrictions. The
cookie partition belongs to the embedding site, so opening an app in a new tab
requires a fresh launch and may use a separate guest login. Browsers that cannot
retain these cookies must fail acceptance; do not activate and silently revert
to the shared origin. [Partitioned cookie behavior](https://developer.mozilla.org/en-US/docs/Web/Privacy/Guides/Third-party_cookies/Partitioned_cookies).

Enable **Server → Isolated app previews → Enable after acceptance** only after
the checks succeed. Explicit isolated launches then fail closed on missing or
invalid configuration. Legacy VM paths become launch handoffs; mutating app
requests must use the isolated host. Resource cookies never grant access to the
main AuraGo router or to another resource. Existing exact-path file tickets do
not gain preview rights.

The gateway validates the original AuraGo session or token on each request and
during streams. It does not replace an app's own authentication. The pinned
boringd web route forwards guest Authorization without requiring a management
bearer; AuraGo must never add the boringd token to that route.

Local fixture tests establish the gateway's protocol behavior. They do not
establish DNS/TLS readiness or acceptance of real installed apps. While the
switch remains off, the legacy VM same-origin and Store host-cookie risks remain
open. No background migration or automatic activation is performed.
