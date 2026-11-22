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
- `SaveExplicitMediaContext` accepts document, audio and video HTTP(S) URLs
  after the agent checks config and effective network grants. It allows LAN
  and loopback destinations, checks the dialed address on each connection,
  and strips referrers on redirects. The client accepts URLs without credentials
  and rejects HTML responses before saving a file.
- Provider attachments use their own client. Local `/files/` answer images use
  rooted workspace staging.
- Preserve content/size validation, image sanitization, caller cancellation and
- `SanitizeImageFile` refuses images over 40 Mpx (`maxImagePixels`, checked with `image.DecodeConfig` before decoding). Verify `TestSanitizeImageFilePixelCap`.
  removal of partial files. Never accept a response merely because it is an image.

## Work Guidance

- Test-only local clients may replace the public image client in positive
  fixtures; policy tests must exercise the production address checks.

## Verification

- `go test ./internal/media`; `TestAnswerImage*` covers private targets and
  redirects, and existing transfer tests cover limits/cancellation/extensions.

## Child DOX Index

None.
