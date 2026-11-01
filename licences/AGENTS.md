# Licences

## Purpose

Compact inventory of external projects AuraGo links, ships on the virtual desktop, downloads as optional models/runtimes, or starts as Docker images, plus the license texts those licenses require to travel with the work.

## Ownership

This folder owns `UEBERSICHT.md` and `licences/texts/`. AuraGo's own license stays `LICENSE` at the repository root. Vendored JavaScript copyright lines stay in `ui/js/vendor/**/LICENSES.md`. `THIRD_PARTY_NOTICES.md` and `THIRD_PARTY_LIBRARIES.md` remain the English notices; they point here.

## Local Contracts

- `UEBERSICHT.md` is the German listing. Keep it aligned with the direct `go.mod` require block, root `package.json`, `browser_automation_sidecar/package.json`, `tools/aurago-tui/Cargo.toml`, and `internal/desktopstore/catalog.go`.
- Texts in `texts/` are verbatim upstream or SPDX texts. Do not rewrite them.
- One SPDX text covers a license family. Project copyright lines stay in the overview or in the vendor `LICENSES.md` files. The SPDX MIT text uses year and holder placeholders.
- Floating image tags describe the upstream project, not a digest-pinned legal snapshot.
- Leave Datatype and any image whose license file was not read marked as unverified. Do not invent an SPDX id.
- `command-code` npm metadata `UNLICENSED` stays stated as no granted license.
- Copyleft and the special terms for n8n, Open WebUI, Redis, C64 Pro Mono, and CanvasUI stay visible in the Sonderfälle section.
- Running an unmodified container does not relicense AuraGo's own MIT code.
- Desktop icon notices are generated into `ui/img/{tela,candy,fluent,buuf-nestort}/` for the external resource set. Preserve imported GPL-3.0 artwork and noncommercial CC BY-NC-SA 2.5 Buuf separately from original MIT supplements; see `../assets/desktop-icon-themes/AGENTS.md`.
- WhatsApp: `go.mau.fi/libsignal` (GPL-3.0) is linked into the binary through whatsmeow (MPL-2.0). `LICENSE` stays MIT; `THIRD_PARTY_NOTICES.md`, `THIRD_PARTY_LIBRARIES.md` §9, the Sonderfälle row here and the README license line state that distributed binaries are GPL-3.0 combined works. A GPL-3.0-incompatible module linked into the binary blocks a release. Guard: `internal/audit` `TestWhatsAppLicenseNoticesStayAligned`.

## Work Guidance

## Verification

Compare new direct dependencies and Store catalog entries with `UEBERSICHT.md` when those files change.

Modules linked into the binary: `go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' ./cmd/aurago | sort -u`.

## Child DOX Index
