# Licences

## Purpose

Compact inventory of external projects AuraGo links, ships on the virtual desktop, downloads as optional models/runtimes, or starts as Docker images, plus the license texts those licenses require to travel with the work.

## Ownership

This folder owns `UEBERSICHT.md`, `THIRD_PARTY_NOTICES.md`, `THIRD_PARTY_LIBRARIES.md`, `texts/` and `linked-go-modules.tsv`, the reviewed licence row of every module the release binaries link (see Local Contracts). AuraGo's own license stays `LICENSE` at the repository root. Vendored JavaScript copyright lines stay in `ui/js/vendor/**/LICENSES.md`.

## Local Contracts

- The English inventories live here in source checkouts. Release packagers and the main Dockerfile copy `licences/THIRD_PARTY_NOTICES.md` to the existing top-level `THIRD_PARTY_NOTICES.md` artifact/install path; preserve that basename for installers and legal checks. Markdown links in source notices are relative to this folder.

- `UEBERSICHT.md` is the German listing. Keep it aligned with the direct `go.mod` require block, root `package.json`, `browser_automation_sidecar/package.json`, `tools/aurago-tui/Cargo.toml`, and `internal/desktopstore/catalog.go`.
- Texts in `texts/` are verbatim upstream or SPDX texts. Do not rewrite them.
- One SPDX text covers a license family. Project copyright lines stay in the overview or in the vendor `LICENSES.md` files. The SPDX MIT text uses year and holder placeholders.
- Floating image tags describe the upstream project, not a digest-pinned legal snapshot.
- Leave Datatype and any image whose license file was not read marked as unverified. Do not invent an SPDX id.
- `command-code` npm metadata `UNLICENSED` stays stated as no granted license.
- Copyleft and the special terms for n8n, Open WebUI, Redis, C64 Pro Mono, and CanvasUI stay visible in the Sonderfälle section.
- Running an unmodified container does not relicense AuraGo's own MIT code.
- Desktop icon notices are generated into `ui/img/{tela,candy,fluent,buuf-nestort}/` for the external resource set. Preserve imported GPL-3.0 artwork and noncommercial CC BY-NC-SA 2.5 Buuf separately from original MIT supplements; see `../assets/desktop-icon-themes/AGENTS.md`.
- GPL-3.0 combined work: the release binaries link two GPL-3.0 sources, and `LICENSE` stays MIT. WhatsApp: `go.mau.fi/libsignal` (GPL-3.0) is linked through whatsmeow (MPL-2.0). Native code graph: `github.com/odvcencio/gotreesitter` `grammars/runtime` compiles the GPL-3.0 `caddy` and `disassembly` external scanners and the MPL-2.0 `nim` files (`nim_scanner.go`, `support_nim.go`) unless built with the module's `gotreesitter_no_copyleft` tag, which no release builder sets. The module's row is `MIT AND GPL-3.0-only AND MPL-2.0`, with its `LICENSE`, `licenses/texts/GPL-3.0.txt` and `licenses/texts/MPL-2.0.txt`. `THIRD_PARTY_NOTICES.md`, `THIRD_PARTY_LIBRARIES.md` §9, the Sonderfälle row here and the README license line state that distributed binaries are GPL-3.0 combined works. A GPL-3.0-incompatible module linked into the binary blocks a release. Guards: `internal/audit` `TestWhatsAppLicenseNoticesStayAligned` and the linked-module gate below.
- Linked-module licence gate: `linked-go-modules.tsv` has one row per module that `./cmd/aurago`, `./cmd/remote` and `./cmd/config-merger` link for any of the six release targets (linux, darwin and windows, on amd64 and arm64). Columns: module, SPDX expression (`A AND B` when every part applies, `A OR B` when one alternative is enough), licence files relative to the module root (comma-separated) and an optional note. `repo:licences/texts/X.txt` names a verbatim text of this repository for a part the module does not ship. Rows are sorted and unique; a row for a module that no target links is stale. The file's header is the format reference.
- `TestLinkedGoModulesHaveGPL3CompatibleLicenses` (`internal/audit/linked_module_licenses_test.go`) runs `go list -deps` for the six targets with `CGO_ENABLED=0` and fails on:
  - a linked module without a row (the message suggests a licence file and a guessed SPDX id; read the file before adding the row), or a stale row;
  - an SPDX part that is not on the allowlist or not declared by its row (`GPL-3.0` means `GPL-3.0-only`);
  - a listed licence file that is missing (exact letter case), or whose text lacks the phrases of its SPDX part;
  - the negative fingerprints: a text with GPL-2.0, AGPL-3.0, SSPL-1.0 or EPL-2.0 without a GPL secondary-licence grant is refused even beside an allowed text, and an Eclipse Distribution License text must name the Eclipse Foundation;
  - a GPL-3.0 module that `THIRD_PARTY_NOTICES.md` does not name in backticks;
  - the `SPDX-License-Identifier` headers of the Go files the targets compile (the first 32 KiB of each): every identifier must be allowed and declared, so a header can narrow a row but never widen it. An MPL-2.0 file marked "Incompatible With Secondary Licenses" blocks the release.
- The allowlist is `gpl3CompatibleLicenses` in that test: GPL-3.0-only, MPL-2.0, Apache-2.0, BSD-3-Clause, BSD-2-Clause, ISC, MIT, Zlib, Unlicense and CC0-1.0, each with the phrases its text must carry. Widening it needs a user decision, not a row edit. `TestLinkedModuleLicenseFingerprintsClassifyTheShippedTexts` checks that each shipped text classifies as expected, so a change to the phrases must keep it green.
- Paho: `github.com/eclipse/paho.mqtt.golang` is dual EPL-2.0 OR EDL-1.0. EPL-2.0 alone cannot be combined with GPL-3.0, so its row declares `BSD-3-Clause` (the EDL-1.0 terms) with the module's own `edl-v10` file, and the GPL-3.0 binaries use the module under EDL-1.0. Releases ship AuraGo's EDL-1.0 text in `licences/texts/EDL-1.0.txt`, which `THIRD_PARTY_NOTICES.md` names; the packaging test checks its title and copyright line.
- Release packaging: `scripts/make_deploy.sh`, `scripts/make_release.ps1` and `scripts/make_release.bat` run the gate before their UI build, cleanup or packaging steps. Each stages `LICENSE`, `THIRD_PARTY_NOTICES.md`, `licences/texts/` and `licences/linked-go-modules.tsv` into `resources.dat`, and each checks that the archive lists `./LICENSE`, `./THIRD_PARTY_NOTICES.md`, `./licences/texts/GPL-3.0-only.txt` and `./licences/linked-go-modules.tsv`. The Dockerfile copies the same files into `/app`; `security-gates.yml` tests them in the built image, and `.dockerignore` must not exclude `licences`.
- Install and update: `install.sh` and `update.sh` extract the signature-verified `resources.dat` and copy `licences/`, `LICENSE` and `THIRD_PARTY_NOTICES.md` into the install directory. An archive that contains a symbolic link aborts the install or update, so a link never steers a copy.
- Release notes: the Windows builders write a "License and source" section that links the tag (`https://github.com/antibyte/AuraGo/tree/<tag>`), its tar.gz (`archive/refs/tags/<tag>.tar.gz`) and the notices at the tag, and states that the binaries are GPL-3.0 combined works.
- Tag retention: the Windows builders keep the latest three stable releases (drafts and prereleases excluded) and delete older ones with `gh release delete` and no `--cleanup-tag`, so their tags remain as corresponding source. `AURAGO_KEEP_RELEASES=1` skips retention; `release.yml` sets it. The packaging test rejects `--cleanup-tag` in either Windows builder.
- Guard: `internal/audit/whatsapp_release_packaging_test.go` (`TestReleasePackagingCarriesTheGPLNotices`) checks the gate order, the staging and archive lines, the installers, the Dockerfile, `.dockerignore`, the image test, the EDL-1.0 text, the notice headings and the release-note strings above.

## Work Guidance

## Verification

Compare new direct dependencies and Store catalog entries with `UEBERSICHT.md` when those files change.

Modules linked into the binary: `go list -deps -f '{{with .Module}}{{.Path}} {{.Version}}{{end}}' ./cmd/aurago | sort -u`.

Linked-module gate and release packaging: `CGO_ENABLED=0 go test ./internal/audit -run '(TestLinkedGoModulesHaveGPL3CompatibleLicenses|TestLinkedModuleLicenseFingerprintsClassifyTheShippedTexts|TestSPDXHeaderExpressionsAreCheckedAgainstTheManifestRow|TestReleasePackagingCarriesTheGPLNotices|TestWhatsAppLicenseNoticesStayAligned)' -count=1`. The gate reads the module cache, so run `go mod download` first if a module is missing. It is skipped under `-short`; run it without that flag.

## Child DOX Index
