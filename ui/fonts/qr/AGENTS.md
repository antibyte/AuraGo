# QR caption fonts

## Purpose

Local font bytes for headless and browser QR caption rendering; included in the verified web resource set.

## Ownership

This directory owns the fixed Noto fallback files, verbatim OFL notices and `manifest.json` provenance. `internal/qrcode/AGENTS.md` owns font selection and rendering. Server/tool adapters resolve only fixed catalog basenames through `webassets.Namespace("ui")`.

## Local Contracts

- No runtime downloads, system-font dependence, payloads or user font uploads.
- Keep upstream source revisions, byte sizes, SHA-256 and OFL notices paired with each font.
- `sans` and `mono` use embedded Go font data from the existing x/image dependency. `serif` reuses the shipped OFL Liberation Serif font. Noto Arabic, Devanagari, CJK and monochrome emoji supply missing scripts only.
- CJK uses the Simplified Chinese regional glyph style and covers Japanese kana/Han; regional typographic variants are not promised.

## Work Guidance

Retain original font names and exact OFL texts. `Go-fonts-LICENSE.txt` preserves the Go Sans/Mono font-specific BSD notice alongside the compiled fallback usage. Update catalog resources and verification when replacing a font.

## Verification

`go test ./internal/qrcode` verifies font provenance hashes and mixed-script caption rendering. Resource packaging remains owned by `ui/AGENTS.md`.

## Child DOX Index
