# Media transfers

## Purpose

Own shared attachment/image transfers, image preparation and cancellation cleanup.

## Ownership

This package owns transfer helpers; messenger packages own sender admission and
delivery. `../security/AGENTS.md` owns public HTTP address and redirect policy.

## Local Contracts

- Answer-image HTTP(S) URLs use `security.NewPublicHTTPClient` through
  `SaveURLToDirContext`. Reject internal/reserved targets and recheck/pin every
  redirect. Remove URL fragments before transport; never enable LAN access to
  make answer-image downloads succeed.
- Provider attachments and ordinary file transfers retain their separate
  transport. Local `/files/` answer images retain rooted workspace staging.
- Preserve content/size validation, image sanitization, caller cancellation and
  removal of partial files. Never accept a response merely because it is an image.

## Work Guidance

- Test-only local clients may replace the public image client in positive
  fixtures; policy tests must exercise the production address checks.

## Verification

- `go test ./internal/media`; `TestAnswerImage*` covers private targets and
  redirects, and existing transfer tests cover limits/cancellation/extensions.

## Child DOX Index

None.
