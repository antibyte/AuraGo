# A2A protocol

## Purpose

Expose authenticated peer-agent routes and connect configured peers without granting model permissions through the transport.

## Ownership

This package owns protocol cards, HTTP authentication, peer clients and the remote-task bridge. internal/agent owns model/tool/session policy; internal/server owns hosting and shutdown.

## Local Contracts

- Inbound API keys use X-API-Key, as advertised by the AuraGo agent card. Query-string API keys do not authenticate. Bearer authentication and public card discovery retain their existing configuration gates.
- Do not infer inbound authentication behavior from another peer's card. Peer discovery and client credentials remain separate from the local server policy.
- Peer card URLs require absolute HTTP(S) URLs without userinfo/query/fragment. Discovery and task HTTP transports stay on that configured origin, including advertised interfaces; LAN peers remain allowed and DNS is pinned. Register peer credentials before discovery. Verify `TestRemoteHTTPClientBindsPeerOrigin`.
- HTTP and all unary/streaming gRPC methods share the same enabled API-key/Bearer check. Missing, empty or invalid gRPC credentials fail before handler execution; public card discovery is HTTP-only.
- Worker admission, cancellation, session scope and draining share the agent registry contract in ../agent/AGENTS.md.
- Peer system queries retain only service `status` and package `detect`, `search`, `list_installed`, `info`. The shared agent role policy denies mutations and `install_package` before approvals, and trims discovery to the same operations.
- Peers cannot create documents or Desktop Notes, or mutate Office files. Office reads/formula evaluation and Notes list/search/read retain their existing grants; dispatch and discovery share the agent role boundary.

## Work Guidance

- Keep authentication tests and the advertised security scheme aligned. Never put real peer credentials in fixtures or diagnostics.

## Verification

- Run go test ./internal/a2a and the agent A2A/role-policy tests.

## Child DOX Index

None.
