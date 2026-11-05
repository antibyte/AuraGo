# AgentMail integration

## Purpose

Credentialed AgentMail API access and the optional inbound mail relay.

## Ownership

This package owns API requests, provider types, bounded attachments and relay
lifecycle. Config owns settings, server owns HTTP/admin publication, and agent
owns tool admission. Mailer's provider adapter remains in `../mailer`.

## Local Contracts

- API bases use absolute HTTPS without URL credentials, query or fragment.
  The default client pins public DNS targets through the shared SSRF transport.
  Exact-origin redirect checks also wrap injected clients without mutating them
  or relaxing their stricter policies. No retry of uncertain mutations.
- `APIError` retains provider fields for internal classification; `Error()` exposes
  only the HTTP status. Never return arbitrary provider bodies to browser/agent callers.
- Deleting provider inboxes/messages/drafts requires the separate default-off
  `agentmail.allow_delete` grant at agent/server boundaries; readonly wins.
- Register credentials before requests. Keep relay sender policy, successful-seed
  admission, quarantine, deduplication and shutdown joining intact.

## Work Guidance

Use TLS fixtures with injected trusted clients. Default-client integration fixtures
run through testutil.RunTLSFixtureSubprocess before trusting their local certificate;
keep irreversible root changes in the child and allow loopback only in tests.

## Verification

`go test ./internal/agentmail ./internal/mailer`; server AgentMail/probe/config tests,
agent AgentMail dispatch tests and the existing newspaper delivery classification tests.

## Child DOX Index

None.
