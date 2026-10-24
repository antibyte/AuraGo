# CYD notification devices

## Purpose

Own device notification delivery, the glass overlay and bounded speech playback.

## Ownership

`hub.go` owns notification routing; `tts.go` owns the speaker and PCM conversion.
`internal/server/cyd_handlers.go` connects MeshCore events to the hub. Shared TTS
providers and fallback remain owned by `internal/tools` and `internal/phonon`.

## Local Contracts

- `Notify` retains ordinary title/body speech; `NotifySpeech` supplies an
  independent spoken line while preserving the visible notification payload.
- MeshCore notifications speak only `MeshAnnounce`, never the sender, channel
  or message preview. Protected previews remain locked on the display.
- The announcement follows the configured system language, then UI language,
  then the effective voice language. Regional tags use their base language;
  unsupported languages use the English phrase. Keep all 16 UI languages.
- Reuse the configured synthesis hook, cancellation, serialization, bounded
  audio and existing PCM decoding. Do not provision another TTS runtime here.

## Work Guidance

Keep notification display text separate from the spoken announcement.

## Verification

Run `go test ./internal/cyd` and server `TestMeshIncomingSpeechUsesPreview`.
Real device playback remains a separate hardware check.

## Child DOX Index

None.
