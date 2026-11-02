# WhatsApp Stage 4: Finish Implementation Plan (Tasks 1–7)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The WhatsApp channel is finished and releasable:
- operators have a setup guide and manual sections;
- the security page warns about risky WhatsApp settings, and the Danger Zone text names the WhatsApp device keys;
- the morning briefing reports WhatsApp;
- all WhatsApp strings are reviewed in 16 locales;
- the audit manifests are re-verified;
- every release is blocked by a GPL-3.0 compatibility check over the linked modules, and carries the GPL-3.0 text and the notices in `resources.dat` and the Docker image;
- release notes link the tagged source;
- the Telegram parity sweep is closed.

**Master plan:** `plans/2026-10-31-whatsapp-integration-plan.md`. This plan covers the stage-4 table (lines 120–129), "Stage 4 — Finish" (299–313), "Telegram parity sweep" (315–337) and "Risks" (339–345).
**Executed stage plans:** `plans/2026-10-31-whatsapp-2-text.md`, `-3-media-tools.md`, `-3b-targets.md`. Their deviation lists describe what landed.
**Design:** `plans/2026-10-31-whatsapp-integration-design.md` (Licensing 218–225, Reliability 227–234). It is final.
**Base:** worktree `C:\Users\Andi\Documents\repo\AuraGo\.worktrees\whatsapp-stage2`, branch `claude/whatsapp-stage2`, HEAD around `756dd4b23`. Stages 1, 2, 3 and 3b are done there.

**Outward-facing rule:** this stage edits release scripts, workflows and docs only. Nobody pushes, tags, publishes, dispatches `release.yml` or runs `make_release.ps1`/`make_release.bat` without `-Local`.

---

## Stage ground rules (on top of the master plan's standing rules)

The 3b ground rules apply unchanged: worktree commits, GitNexus from the worktree root, privacy, Go source, lean tests. On top of them:

- **DOX before editing.** Read these guides:
  - root `AGENTS.md` (release lines 150–163, routing row 248);
  - `internal/whatsapp/AGENTS.md`;
  - `internal/server/AGENTS.md` ("WhatsApp Runtime Contract");
  - `internal/agent/AGENTS.md`;
  - `internal/planner/AGENTS.md` (58–59, briefing);
  - `licences/AGENTS.md`;
  - `internal/releaseverify/AGENTS.md` (before touching `install.sh`/`update.sh`);
  - `ui/AGENTS.md`;
  - `internal/flows/AGENTS.md` (Task 7 only).
- **Commits.**
  - Run `git add <own paths>`; stage child `AGENTS.md` files with `git add -f`.
  - Before a root-`AGENTS.md` commit, `git ls-files -v AGENTS.md` must print `H`.
  - Before each commit, run `git diff --cached --check` and scan the diff for real numbers and secrets.
  - Use `-m "<msg>" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"`.
- **GitNexus.**
  - Impact: `node ../../.gitnexus/run.cjs impact "<symbol>" --direction upstream --repo ../..`.
  - Before each commit: `node ../../.gitnexus/run.cjs detect-changes --scope all --repo .`. A `partial` or `truncated` result means run it again.
  - The index is main, so stage symbols come back UNKNOWN. Confirm them with `graft callers` or a text search, and say so in the report.
- **Privacy.**
  - Fixtures use `+4917012345678` and `+4917099998888`.
  - Docs reuse the UI placeholder `+491701234567`.
  - No hint text, briefing data or log line carries a phone number or a masked JID.
- **Files.**
  - LF line endings, no BOM.
  - Go source never contains a backslash-u escape: write curly quotes and umlauts literally.
  - Scratch files go in `disposable/_whatsapp/`.
- **UI strings.**
  - Stage 4 adds 5 keys and changes 1, in 16 locales. All are listed under "New UI strings".
  - docs-agent writes the JSON in the same batch, before the maker commits.
  - German uses "du" (as the files do) and real umlauts.
  - `ui/lang/config/security/*.json` is sorted by key.
- **Lean tests.** No `t.Parallel` in tests that set `integrationstatus` WhatsApp state.

---

## Deviations from the master plan

1. **Licence check: an in-repo manifest and an audit test, not `go-licenses report`** (Task 6; open question 1 has the options).
   - `go-licenses` is not installed, not vendored and not a tool dependency.
   - `THIRD_PARTY_LIBRARIES.md:383-385` only mentions it as an optional, unpinned `@latest`.
   - The new check:
     - lists every module linked into `./cmd/aurago` for all six release targets (`go list -deps`, `CGO_ENABLED=0`);
     - requires a reviewed row in `licences/linked-go-modules.tsv`;
     - checks that row's licence file in the module cache against fixed marker phrases;
     - fails on any licence outside a GPL-3.0-compatible allowlist.
   - It runs in `go test ./...` (Security Gates, which `release.yml` waits for) and first thing in `make_deploy.sh`, `make_release.ps1` and `make_release.bat`.
2. **Paho MQTT is used under EDL-1.0.**
   - `github.com/eclipse/paho.mqtt.golang` v1.5.1 is dual EPL-2.0 / EDL-1.0. The module cache has `LICENSE` (EPL-2.0 with a dual-licence header), `epl-v20` and `edl-v10`.
   - EPL-2.0 alone is GPL-3.0-incompatible. The GPL-3.0 binaries rely on EDL-1.0 (BSD-3-Clause).
   - This is recorded in the manifest note and in `THIRD_PARTY_LIBRARIES.md` §1 and §9. `licences/UEBERSICHT.md:95` already says "EPL-2.0 OR BSD-3-Clause".
3. **More release files than the master table lists.** The stage-4 table at master lines 128–129 lists only `make_deploy.sh`, `make_release.ps1` with its scripts, `Dockerfile` and `release.yml`. The plan also edits:
   - `make_release.bat`: a second Windows builder, pinned by `TestMakeReleaseRequiresCleanExplicitBuildIdentity`;
   - `install.sh` and `update.sh`: they copy only `prompts`, `agent_workspace`, `ui`, `assets` and the config template out of `resources.dat` (`install.sh:2429-2441`, `update.sh:3261-3273`), so notices in the archive would never reach the install;
   - `.github/workflows/security-gates.yml`: one Docker-image step that proves the notices are in the image.

   No change is needed in:
   - `release.yml`: it runs `make_release.ps1`, and its `verify-source` job already waits for Security Gates;
   - the scripts that `make_release.ps1` calls;
   - `internal/setup` (`--setup` extracts all of `resources.dat`).
4. **"Release archives" means `resources.dat`.**
   - The binaries are uploaded as raw files. `resources.dat` is the archive every install path extracts beside the binary.
   - Raw-binary downloads get the licence through the release notes' "License and source" section.
   - Each builder checks the `resources.dat` listing and refuses to publish without the notices.
5. **Local keyed retention keeps tags.**
   - `make_release.ps1:500` and `make_release.bat:472` delete old releases with `--cleanup-tag`. The release notes link the tag as the GPL-3.0 corresponding source, and GHCR keeps old image tags.
   - Retention therefore deletes the release only. `release.yml` already keeps everything (`AURAGO_KEEP_RELEASES=1`).
6. **Docs also cover `07-configuration.md` and `14-security.md` (en/de).**
   - The 3b docs pass found no `whatsapp:` reference anywhere, so 07 gets the section row, structure line and compact-table row.
   - 14 names `data/whatsapp/device.db` next to `data/vault.bin` under host execution (stage-1 deviation 16).
7. **Security hints read the configuration only.**
   - `CheckSecurity` stays a pure function of the configuration. The hints do not read link state, and they are suppressed in egg mode, where the runtime refuses to start.
   - Both use `SevWarning`, as the design says. "Dedicated without owner" is a misconfiguration, not an exposure, and the text says AuraGo answers nobody.
   - Title and description are translated (`security.hint.<id>.*`), because neither carries server data.
8. **Maintenance check states.** Like Telegram, every enabled and configured state other than `healthy` fails, including `not_linked`, `linking` and `connecting`. The result data has no masked JID.
9. **Locale review adds one help key and one fix.**
   - `self_chat_prefix` had no help text, and its echo rule (`own_prefix` drops) is invisible to operators. The fix adds `help.whatsapp.self_chat_prefix`, its `config_help.json` entry and the `help:` argument in `ui/cfg/whatsapp.js`.
   - Czech `easydrag.node.notify_whatsapp.description` uses "ti", while every other Czech WhatsApp string uses the formal form.
10. **Task 5 has no planned edit.** It re-runs the manifests and fixes only what is red.
11. **Parity-sweep fixes live in Task 7.** They are four test or CI gaps (Task 7a), not new features.

## Dispatch grouping

| Batch | Tasks | Agents | Depends on | Files |
| --- | --- | --- | --- | --- |
| B1 | 1 docs | docs-agent; maker-agent for the audit test | — | new `documentation/whatsapp_setup.md`; `documentation/manuals/{en,de}/{07-configuration,08-integrations,14-security}.md`; `README.md`; new `internal/audit/whatsapp_docs_test.go` |
| B2 | 2 security, 3 briefing | maker-agent; docs-agent for `ui/lang/config/{security,danger}/<16>` | — | `internal/server/security_check.go`; new `internal/server/security_check_whatsapp_test.go`; `ui/lang/config/security/<16>.json`; `ui/lang/config/danger/<16>.json`; new `ui/config_whatsapp_security_hints_test.go`; `internal/agent/maintenance_briefing.go`; new `internal/agent/maintenance_whatsapp_check_test.go` |
| B3 | 4 locales | docs-agent; maker-agent for JS, help JSON and test | B2 (reviews its strings) | `ui/lang/config/whatsapp/<16>.json`; `ui/lang/easydrag/cs.json` (plus files of review fixes, if any); `ui/cfg/whatsapp.js`; `ui/config_help.json`; new `ui/whatsapp_locale_review_test.go` |
| B4 | 6 licence and packaging | maker-agent; docs-agent for the inventories | B1 (both edit `internal/audit`) | new `internal/audit/linked_module_licenses_test.go`; new `licences/linked-go-modules.tsv`; new `internal/audit/whatsapp_release_packaging_test.go`; `make_deploy.sh`; `make_release.ps1`; `make_release.bat`; `install.sh`; `update.sh`; `Dockerfile`; `.github/workflows/security-gates.yml` (Docker step only); `THIRD_PARTY_LIBRARIES.md`; `THIRD_PARTY_NOTICES.md`; `documentation/dependencies.md` |
| B5 | 5 audit | test-agent | B1–B4 | none (only if red) |
| B6 | 7 sweep, DOX, acceptance | maker-agent; docs-agent; the user | B5 | `internal/flows/catalog_actions_notify_hooks_test.go`; `internal/server/flows_catalog_env_test.go`; `internal/server/flows_tool_invoker_documents.go` (comment); `internal/server/flows_tool_invoker_documents_hardening_test.go`; `.github/workflows/security-gates.yml` (race line); `internal/audit/security_gates_test.go`; `AGENTS.md`; `internal/{whatsapp,server,planner}/AGENTS.md`; `licences/AGENTS.md`; master plan "Deliberately not mirrored" |

**Parallelism:**
- B1 and B2 are file-disjoint and compile different packages, so they can run at the same time in the shared worktree.
- B3 and B4 are disjoint too (`ui/` versus `internal/audit` plus scripts), and can run in parallel once B2 and B1 are committed.
- `security-gates.yml` is edited by B4 and then B6, in that order.
- Safe default: B1, B2, B3, B4, B5, B6 in sequence.

**Security review:**
- One combined security-agent pass (Critical and Important only, no re-review) after B4 and before B5.
- It covers:
  - B2: hint and Danger Zone accuracy;
  - B1: the "Security and privacy" section of `whatsapp_setup.md`;
  - B4: gate bypasses, install and update copying, tag retention, Docker COPY sources.
- The checklist is at the end.

## Open questions for the user

1. **May `go-licenses` become a tool dependency?** Options:
   - **A (default, recommended):** the in-repo manifest and audit test of deviation 1.
     - No new module, offline and deterministic.
     - It blocks local and CI releases.
     - Cost: about 200 reviewed manifest rows once, plus one row per new module.
   - **B:** `go get -tool github.com/google/go-licenses/v2@<pinned version>`.
     - Adds a `tool` directive and its classifier dependencies to `go.mod`/`go.sum`, but not to the binary.
     - It would need an allowlist and a Paho exception anyway.
     - It would run as `go tool go-licenses check ./cmd/aurago --allowed_licenses=…` next to A, not instead of it.
   - **C:** an advisory Security Gates job, `go install github.com/google/go-licenses/v2@<pinned>` (the govulncheck pattern, root `AGENTS.md:156`).
     - No `go.mod` change, but it needs network and cannot gate local releases.

   Without an answer, Task 6 implements A.

---

## Task 1: Operator docs (B1)

**Files:**
- new `documentation/whatsapp_setup.md`;
- `documentation/manuals/en/08-integrations.md` (jump list `:17`, new section after "Telegram Bot Setup" `:63-90`);
- `documentation/manuals/de/08-integrations.md` (`:17`, after `:63-88`);
- `documentation/manuals/{en,de}/07-configuration.md`: en `:59` table, `:125` structure, `:1033` compact row; de `:114`, `:526`;
- `documentation/manuals/{en,de}/14-security.md`: en `:578`, de `:621`;
- `README.md` `:50,165`;
- new `internal/audit/whatsapp_docs_test.go`.

**`documentation/whatsapp_setup.md`** is English, like `telegram_setup.md`, with no images and no external links. Required sections and facts:
- `# WhatsApp Setup Guide`, with one paragraph:
  - AuraGo is a linked companion device named "AuraGo";
  - it uses the unofficial WhatsApp Web multi-device protocol;
  - it talks only to owner numbers.
- `## Before you start`:
  - WhatsApp's terms forbid automated messaging, and numbers can be banned;
  - **a dedicated number is recommended**;
  - needed: a phone with WhatsApp, admin access, a data directory;
  - not available in egg mode.
- `## Choose a mode`:
  - `dedicated` (recommended): AuraGo has its own number, and you write to it from your `owner_numbers` (up to 10, international format);
  - `self_chat`: your personal number, where only the "Message yourself" chat reaches AuraGo, AuraGo's messages start with `self_chat_prefix`, and the security page warns.
- `## Step 1: Configure AuraGo`:
  - Config → Messenger → WhatsApp;
  - the `whatsapp:` YAML block, exactly as in `config_template.yaml:1437-1447`, with `owner_numbers: ["+491701234567"]`;
  - saving restarts the channel without relinking.
- `## Step 2: Link the account`:
  - QR: phone Settings → Linked devices → Link a device;
  - pairing code: "Link with phone number instead";
  - a session lasts 3 minutes;
  - passkey linking is not supported, so use the QR code;
  - a second link replaces the first after confirmation.
- `## Step 3: Send a test message`: at most one a minute; it reaches every owner number.
- `## Phone rules`:
  - **open WhatsApp on the phone at least every 14 days**, or WhatsApp removes linked devices;
  - **up to 4 linked devices per account**;
  - AuraGo appears as "AuraGo" in the list.
- `## No bulk sending`:
  - AuraGo sends only to the owners (self-chat: your own account);
  - no tool, flow or API messages other numbers;
  - **never bulk-send** or script mass messages through the linked device;
  - temporary bans (WhatsApp reason codes 101–106) show as `temp_banned`;
  - the sender paces itself to 20 messages per minute (burst 5).
- `## Read-only mode`: `whatsapp.readonly: true` blocks `send_whatsapp`, notifications, flows, camera alerts and newspaper delivery; replies to the owner still work.
- `## What reaches WhatsApp`:
  - text, voice notes (transcribed), images (vision), documents (saved to `attachments/`), placeholders for video and stickers, locations, contact cards (third-party), quoted replies;
  - forwarded messages are third-party: no commands, no answers to questions;
  - answers carry images, voice notes in `/voice` mode, plain text without link previews;
  - `send_whatsapp`, `send_notification` channel `whatsapp`, camera alerts with photo, EasyDrag `notify.whatsapp` and `notify_on_error`, newspaper delivery;
  - groups, broadcasts, newsletters and status are ignored.
- `## Status and troubleshooting`:
  - a table of the states `disabled`, `not_configured`, `not_linked`, `linking`, `connecting`, `healthy`, `degraded`, `logged_out`, `conflict`, `temp_banned`, `outdated`, each with what to do;
  - `outdated` needs an AuraGo update, because whatsmeow is pinned;
  - counters `dropped_non_owner`, `dropped_unresolved_lid`, `dropped_filtered`, `delivery_failures`;
  - `GET /api/whatsapp/status` (admin);
  - the morning briefing line `whatsapp: …`;
  - the self-test `integration:whatsapp`, which never sends.
- `## Unlink, relink and backups`:
  - Unlink removes AuraGo on the phone when connected; otherwise remove it under Linked devices;
  - `<data_dir>/whatsapp/device.db` is never backed up, so link again after a restore or a move.
- `## Security and privacy`:
  - the device store is a credential (directory 0700, file 0600);
  - agent file tools, outgoing attachments and Docker host access refuse it;
  - shell and Python under `agent.allow_unsafe_host_execution` can read it, like the vault;
  - logs and status show only the last four digits;
  - message content of dropped messages is never logged;
  - security-page hints `whatsapp_self_chat_personal_account` and `whatsapp_dedicated_no_owner`.
- `## License`:
  - the source is MIT;
  - release binaries link GPL-3.0 `go.mau.fi/libsignal` and are GPL-3.0 combined works;
  - `THIRD_PARTY_NOTICES.md`;
  - each release's notes link the tagged source.

**Manual sections:**
- en `### WhatsApp Setup`, de `### WhatsApp einrichten`, each with:
  - a short intro;
  - `#### Web UI Setup` / `#### Einrichtung in der Web-UI` (4 steps);
  - a `> [!WARNING]` with the 14-day rule, 4 devices, dedicated number and no bulk sending (de: "Massenversand");
  - `#### YAML Reference` / `#### YAML-Referenz`;
  - a link `(../../whatsapp_setup.md)`.
- Jump lists add `[WhatsApp](#whatsapp-setup)` / `[WhatsApp](#whatsapp-einrichten)` after Telegram.

**07-configuration:**
- en:
  - `| \`whatsapp\` | WhatsApp channel (linked device, owner numbers) |` after the telegram row;
  - `whatsapp:         # WhatsApp channel (linked device)` after `telegram:`;
  - compact row `| \`whatsapp\` | WhatsApp channel | \`whatsapp:\`<br>\`  enabled: false\`<br>\`  mode: dedicated\`<br>\`  owner_numbers: []\`<br>\`  readonly: false\` |`.
- de: `whatsapp:         # WhatsApp-Kanal (verknüpftes Gerät)` and `| \`whatsapp\` | WhatsApp-Kanal | …same YAML… |`.

**14-security:** one sentence after the host-execution paragraph.
- en: "Host commands that run this way can read AuraGo's data folder, including `data/vault.bin` and the linked WhatsApp device keys in `data/whatsapp/device.db`; agent file tools refuse both files, this exception does not."
- de: "Befehle, die so laufen, können AuraGos Datenordner lesen, auch `data/vault.bin` und die Schlüssel des verknüpften WhatsApp-Geräts in `data/whatsapp/device.db`; die Datei-Tools des Agenten verweigern beide Dateien, diese Ausnahme nicht."

**README:**
- `:50` becomes `- **Give it a voice.** Telegram, WhatsApp, Discord, Rocket.Chat, email, realtime speech and SIP telephony. Speech Lab adds local speech recognition and synthesis.`
- `:165` becomes `| talk to it from my phone | [Telegram setup](documentation/telegram_setup.md) · [WhatsApp setup](documentation/whatsapp_setup.md) · [more channels](documentation/manuals/en/08-integrations.md) |`.
- These pinned strings stay byte-identical:
  - `**Source code: [MIT licensed](LICENSE).**`
  - `GPL-3.0 combined works`
  - `- **Wikipedia in your pocket.**`
  - `[Local Wikipedia](documentation/local-wikipedia.md)`
  - the dashboard screenshot strings.

**Tests** (`internal/audit/whatsapp_docs_test.go`, `requireAll` plus `readRepoFile`, `t.Parallel()`):
- `TestWhatsAppSetupGuideCoversOperatorRules` requires in `documentation/whatsapp_setup.md`:
  - the 14 headings above;
  - `at least every 14 days`, `up to 4 linked devices`, `dedicated number`, `never bulk-send`, `20 messages per minute`;
  - `whatsapp:`, `mode: dedicated`, `self_chat`, `owner_numbers`, `device.db`, `allow_unsafe_host_execution`, `/api/whatsapp/status`;
  - the 11 state codes in backticks, `` `dropped_non_owner` ``, `` `dropped_unresolved_lid` ``;
  - `whatsapp_self_chat_personal_account`, `GPL-3.0`, `THIRD_PARTY_NOTICES.md`.
  - It also fails if the guide contains `4917012345678` (fixture leakage).
- `TestWhatsAppManualSectionsAndReadme`:
  - en 08: `[WhatsApp](#whatsapp-setup)`, `### WhatsApp Setup`, `#### Web UI Setup`, `whatsapp:`, `(../../whatsapp_setup.md)`, `14 days`, `4 linked devices`;
  - de 08: `[WhatsApp](#whatsapp-einrichten)`, `### WhatsApp einrichten`, `#### Einrichtung in der Web-UI`, `14 Tage`, `4 verknüpfte Geräte`, `Massenversand`;
  - both sections sit before "## Testing Integrations" / "## Integrationen testen" (`strings.Index` order);
  - 07 en/de contain `` | `whatsapp` | ``;
  - 14 en/de contain `data/whatsapp/device.db`;
  - README contains both new strings.

Command: `CGO_ENABLED=0 go test ./internal/audit/ -run '(TestWhatsAppSetupGuide|TestWhatsAppManualSections|TestWhatsAppLicenseNotices|TestLocalWikipedia|TestDashboardDocs)' -count=1`

**GitNexus:** no symbol is edited. Run `detect-changes`.
**Commit:** `docs(whatsapp): add the operator setup guide and the manual sections`

---

## Task 2: Security check and the Danger Zone wording (B2)

**Files:**
- `internal/server/security_check.go`: after hint 18 (`:509-519`), plus a helper near `fritzBoxAnyGroupWritable`;
- new `internal/server/security_check_whatsapp_test.go`;
- `ui/lang/config/security/<16>.json` (4 keys);
- `ui/lang/config/danger/<16>.json` (value of `config.danger.unsafe_host_execution.desc`, `:25`);
- new `ui/config_whatsapp_security_hints_test.go`.

```go
	// 18b. whatsapp_self_chat_personal_account — the channel runs on the owner's own account.
	if cfg.WhatsApp.Enabled && !cfg.EggMode.Enabled && cfg.WhatsApp.Mode == config.WhatsAppModeSelfChat {
		hints = append(hints, SecurityHint{
			ID: "whatsapp_self_chat_personal_account", Severity: SevWarning,
			Title: "WhatsApp: self-chat mode uses your personal account",
			Description: "whatsapp.mode is self_chat, so AuraGo is a linked device of your own WhatsApp account. " +
				"It reads only the Message yourself chat, but the linked-device keys in <data_dir>/whatsapp/device.db give access to every chat of the account, " +
				"and a ban for automated messaging hits your personal number. Prefer whatsapp.mode: dedicated with a separate number.",
			AutoFixable: false,
		})
	}

	// 18c. whatsapp_dedicated_no_owner — dedicated mode that would answer nobody.
	if cfg.WhatsApp.Enabled && !cfg.EggMode.Enabled && cfg.WhatsApp.Mode != config.WhatsAppModeSelfChat &&
		!whatsAppHasValidOwnerNumber(cfg.WhatsApp.OwnerNumbers) {
		hints = append(hints, SecurityHint{
			ID: "whatsapp_dedicated_no_owner", Severity: SevWarning,
			Title: "WhatsApp: dedicated mode has no owner number",
			Description: "whatsapp.mode is dedicated, but whatsapp.owner_numbers holds no valid international number. " +
				"AuraGo keeps the WhatsApp channel off and answers nobody until an owner number is saved.",
			AutoFixable: false,
		})
	}

// whatsAppHasValidOwnerNumber reports whether one owner number survives config.NormalizeE164,
// the rule the WhatsApp runtime applies before it starts in dedicated mode (ConfigState).
func whatsAppHasValidOwnerNumber(numbers []string) bool
```

- `Mode != self_chat` treats an unnormalized `""` as dedicated, as `NormalizeWhatsAppConfig` does.
- The Danger Zone string changes in all 16 locales (table under "New UI strings"). `ui/cfg/danger.js` is unchanged.

**Tests:**
- `TestCheckSecurityWarnsAboutWhatsAppSelfChatOnAPersonalAccount`:
  - enabled, `self_chat` → the hint, `SevWarning`, not auto-fixable, description contains `device.db` and `dedicated`;
  - disabled, dedicated with owner `+4917012345678`, or `EggMode.Enabled` → no hint.
- `TestCheckSecurityWarnsWhenDedicatedWhatsAppHasNoOwner`:
  - enabled with mode `dedicated` or `""` and owners `nil`, `[]` or `["0170 123"]` → the hint;
  - `["+4917012345678"]`, `self_chat` without owners, disabled, or egg mode → none;
  - no hint text contains `4917012345678`.
- `TestWhatsAppSecurityHintsAreLocalized` (ui):
  - all 4 security keys are non-empty in 16 locales, and non-en values differ from en;
  - all 16 `config.danger.unsafe_host_execution.desc` values contain `WhatsApp`;
  - `../internal/server/security_check.go` contains `ID: "whatsapp_self_chat_personal_account"` and `ID: "whatsapp_dedicated_no_owner"`.

Commands:
- `CGO_ENABLED=0 go test ./internal/server/ -run '(TestCheckSecurity)' -count=1`
- `go test -count=1 ./ui/... -run '(TestWhatsAppSecurityHints|TestInvasionPlaintextHint|Translations)'`

**GitNexus:** `CheckSecurity` (callers `handleSecurityHints` and `ApplyHardening`; MEDIUM expected, two appended branches, no existing hint changes). `whatsAppHasValidOwnerNumber` is new.
**Commit:** `feat(security): warn about WhatsApp self-chat on a personal account and dedicated mode without an owner`

---

## Task 3: Maintenance briefing check (B2)

**Files:**
- `internal/agent/maintenance_briefing.go`: `runMaintenanceIntegrationChecks` `:98-106`, new function after `runTelegramIntegrationCheck` `:164-186`;
- new `internal/agent/maintenance_whatsapp_check_test.go`.

```go
// in runMaintenanceIntegrationChecks, after runTelegramIntegrationCheck(now):
		runWhatsAppIntegrationCheck(now),

// runWhatsAppIntegrationCheck reads the passive WhatsApp status. It never connects, links or
// sends, so the nightly briefing cannot message the owner. The data holds no number or JID.
func runWhatsAppIntegrationCheck(now time.Time) memory.IntegrationCheckResult {
	status := integrationstatus.WhatsAppStatus()
	result := memory.IntegrationCheckResult{
		ID: "whatsapp", CheckedAt: now.UTC().Format(time.RFC3339),
		Data: map[string]interface{}{
			"state": status.State, "linked": status.Linked,
			"delivery_failures": status.DeliveryFailures, "dropped_unresolved_lid": status.DroppedUnresolvedLID,
		},
	}
	switch status.State {
	case integrationstatus.WhatsAppStateDisabled, integrationstatus.WhatsAppStateNotConfigured:
		result.Status, result.Code, result.Detail = "skipped", status.State, "WhatsApp is disabled or not fully configured"
		return result
	case integrationstatus.WhatsAppStateHealthy:
		result.Status, result.Detail = "passed", "linked device is connected"
		return result
	}
	result.Status, result.Code, result.Detail = "failed", status.LastError, "WhatsApp is not connected"
	if result.Code == "" {
		result.Code = status.State
	}
	return result
}
```

Egg mode maps to `disabled` (stage-1 deviation 4), so it is skipped. `formatMorningBriefing` already prints `- whatsapp: failed (logged_out)`.

**Tests** (no `t.Parallel`; Cleanup `integrationstatus.SetWhatsAppConfigState(integrationstatus.WhatsAppStateDisabled, "")`):
- `TestWhatsAppIntegrationCheckReadsPassiveStatus`, table:

  | Status setup | Expected |
  | --- | --- |
  | `SetWhatsAppConfigState(disabled, "")` | `skipped/disabled` |
  | `SetWhatsAppConfigState(not_configured, "dedicated")` | `skipped/not_configured` |
  | `SetWhatsAppConfigState(connecting, "dedicated")` + `SetWhatsAppState(healthy)` | `passed` with empty code |
  | `MarkWhatsAppError(degraded, "keepalive_timeout")` | `failed/keepalive_timeout` |
  | `SetWhatsAppState(not_linked)` | `failed/not_linked` |
  | `MarkWhatsAppError(logged_out, "logged_out")` | `failed/logged_out` |

  The Data keys are exactly `state`, `linked`, `delivery_failures` and `dropped_unresolved_lid`.
- `TestMaintenanceIntegrationChecksIncludeWhatsAppAfterTelegram`: the IDs of `runMaintenanceIntegrationChecks(&config.Config{}, nil, time.Now())` are `[sandbox virtual_computers telegram whatsapp model_limits]`. All checks are skipped or offline: the sandbox and VMs are disabled, and model limits come from `ResolveModelLimitsCached` with no network.

Command: `CGO_ENABLED=0 go test ./internal/agent/ -run '(TestWhatsAppIntegrationCheck|TestMaintenanceIntegrationChecks|TestMorningBriefing)' -count=1`

**GitNexus:** `runMaintenanceIntegrationChecks` (caller `completeMaintenanceRun`; LOW to MEDIUM, one appended entry). `runWhatsAppIntegrationCheck` is new.
**Commit:** `feat(maintenance): check WhatsApp in the morning briefing`

---

## Task 4: Locale review (B3)

**Scope.** Every WhatsApp key from stages 1–4 in 16 locales:
- `ui/lang/config/whatsapp` (all 51 keys);
- `config/sections` (`config.section.whatsapp.label`, `.desc`);
- `dashboard` (`dashboard.integration_whatsapp`);
- `easydrag` (4 keys);
- `config/newspaper` (4 keys);
- `desktop` (`newspaper.send_whatsapp`, `send_whatsapp_test`, `whatsapp_daily`, `whatsapp_ready`, `whatsapp_unavailable`);
- the B2 keys in `config/security` and `config/danger`.

**docs-agent checks, per locale:**
- a real translation, not English (except brand-only values);
- the register matches the rest of the file (German "du", Czech the formal form used throughout `config/whatsapp/cs.json`);
- the same term for "self chat", "owner number" and "linked device" across files (`config/whatsapp` is the reference);
- real umlauts and diacritics;
- placeholders `{number}`, `{time}`, `{message}` kept.

Fix every finding in place.

**Known findings:**
- `ui/lang/easydrag/cs.json` `easydrag.node.notify_whatsapp.description`: "Pošle ti zprávu…" becomes "Pošle vám zprávu přes WhatsApp, volitelně se souborem."
- `self_chat_prefix` has no help. Add `help.whatsapp.self_chat_prefix` (16, after `help.whatsapp.send_read_receipts`).
  - `ui/cfg/whatsapp.js:49` becomes `field('self_chat_prefix', t('config.whatsapp.self_chat_prefix'), { help: t('help.whatsapp.self_chat_prefix') })`.
  - `ui/config_help.json` `whatsapp` gains `"self_chat_prefix": {"title": "Self-chat prefix", "description": "Self-chat mode only. AuraGo starts its messages with this text (at most 16 characters). Notes to yourself that start with it are ignored, so AuraGo never answers its own echo."}`.

**Test:** `TestWhatsAppLocaleStringsAreCompleteAndTranslated` (`ui/whatsapp_locale_review_test.go`, `readJSONFileMap`):
- every key above is non-empty in 16 locales;
- non-en values differ from en unless the en value is `WhatsApp` or starts with `{`;
- no German value matches `\b(Sie|Ihnen|Ihre?[nmrs]?)\b`.

Commands:
- `go test -count=1 ./ui/... -run '(Precision|Config|I18N|Translations|TestWhatsAppLocale|TestWhatsAppSecurityHints|TestNewspaperTranslations)'`
- `node --check ui/cfg/whatsapp.js`
- No bundle check: `ui/cfg/whatsapp.js` is not in `scripts/build-ui-bundles.js`.

**GitNexus:** `renderWhatsAppSection` (JS). It is reached by dynamic dispatch (`ui/js/config/main.js:3853`), so UNKNOWN is expected; confirm with a text search.
**Commit:** `fix(i18n): review the WhatsApp strings and explain the self-chat prefix`

---

## Task 5: Audit re-verification (B5, test-agent)

```bash
CGO_ENABLED=0 go test ./internal/audit/ -run '(TestRouteContractManifestCoversRegisteredServerRoutes|TestMessagingIngressManifestDeclaresEachIngressPolicy|TestToolPermissionMatrixCoversHighRiskBuiltins|TestNetworkClientInventoryCoversProductionHTTPClients|TestSQLiteOpensAreBackedUpOrExcluded|TestDBMigrationManifestTracksRuntimeSQLiteDomains|TestWhatsApp|TestLinkedGoModules|TestReleasePackaging)' -count=1
CGO_ENABLED=0 go test ./internal/security/ -run '(TestDirectiveShape)' -count=1
```

Expected state:
- 4 `/api/whatsapp/*` admin routes (`audit_manifests.go:199-202`);
- `/api/desktop/newspaper/whatsapp/test` under `/api/desktop/` (3b deviation 14);
- the ingress row `whatsapp` (`:493`);
- the `send_whatsapp` matrix row (`:64`);
- the network entry `internal/whatsapp/` (`:246`);
- `device.db` in `sqliteBackupExclusions`.

Green means no commit; report the PASS counts. Red means fix the manifest row only and commit `test(audit): realign the WhatsApp manifests`. GitNexus impact `MessagingIngressManifest` / `RouteContractManifest` only if edited.

---

## Task 6: License check and release packaging (B4)

### 6a: The linked-module licence gate

**Files:** new `licences/linked-go-modules.tsv`, new `internal/audit/linked_module_licenses_test.go`, `THIRD_PARTY_LIBRARIES.md`, `THIRD_PARTY_NOTICES.md`, `documentation/dependencies.md`.

**Manifest format** (LF, sorted by module path, one row per module, version-independent):

```
# Modules linked into ./cmd/aurago (CGO_ENABLED=0, six release targets) and the licence AuraGo's
# GPL-3.0 binaries rely on. Columns: module, SPDX (A AND B allowed), licence file(s) relative to
# the module root (comma-separated), note. Checked by TestLinkedGoModulesHaveGPL3CompatibleLicenses.
github.com/eclipse/paho.mqtt.golang	BSD-3-Clause	edl-v10	dual EPL-2.0 OR EDL-1.0; GPL-3.0 binaries use EDL-1.0 (BSD-3-Clause)
go.mau.fi/libsignal	GPL-3.0-only	LICENSE
go.mau.fi/whatsmeow	MPL-2.0	LICENSE
```

**Test** (`package audit`, no new imports beyond the standard library):

```go
// gpl3CompatibleLicenses are the licences a module linked into cmd/aurago may carry, each with the
// phrases its licence file must contain (lower case, whitespace collapsed). Anything else
// (GPL-2.0-only, EPL without a secondary-licence grant, CDDL, SSPL, BUSL, Commons Clause, AGPL, an
// unknown text) blocks a release until a reviewed licence is recorded.
var gpl3CompatibleLicenses = map[string][]string{
	"GPL-3.0-only": {"gnu general public license", "version 3, 29 june 2007"},
	"MPL-2.0":      {"mozilla public license", "2.0"},
	"Apache-2.0":   {"apache license", "version 2.0"},
	"BSD-3-Clause": {"redistribution and use in source and binary forms", "endorse or promote"},
	"BSD-2-Clause": {"redistribution and use in source and binary forms"},
	"ISC":          {"permission to use, copy, modify, and", "distribute this software for any purpose"},
	"MIT":          {"permission is hereby granted, free of charge"},
	"Zlib":         {"this software is provided 'as-is'", "altered source versions must be plainly marked"},
	"Unlicense":    {"this is free and unencumbered software released into the public domain"},
	"CC0-1.0":      {"cc0 1.0 universal"},
}
var licenseGuessOrder = []string{"GPL-3.0-only", "MPL-2.0", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC", "MIT", "Zlib", "Unlicense", "CC0-1.0"}
var releaseTargets = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64", "windows/arm64"}

type linkedModuleLicense struct{ Path, SPDX, Note string; Files []string }

func normalizedLicenseText(raw []byte) string                               // lower case, curly quotes to ASCII (literal runes), collapse whitespace
func licenseGuess(text string) string                                       // first id of licenseGuessOrder whose phrases all match, else "UNKNOWN"
func readLinkedModuleManifest(t *testing.T) []linkedModuleLicense           // skips blank and # lines; 3 or 4 tab-separated columns
func linkedModules(t *testing.T, target string) map[string]string          // module path -> dir
func TestLinkedModuleLicenseFingerprintsClassifyTheShippedTexts(t *testing.T)
func TestLinkedGoModulesHaveGPL3CompatibleLicenses(t *testing.T)
```

`linkedModules` runs `go list -deps -f "{{with .Module}}{{if not .Main}}{{.Path}}\t{{.Dir}}{{end}}{{end}}" ./cmd/aurago` in `repoPath()`, with env `GOOS`/`GOARCH` from the target, `CGO_ENABLED=0` and `GOFLAGS=-mod=readonly`. An empty Dir fails with "run go mod download".

**Assertions:**
- `TestLinkedModuleLicenseFingerprintsClassifyTheShippedTexts` (offline, milliseconds). `licenseGuess` over `licences/texts/` gives:
  - `MIT.txt` → MIT, `Apache-2.0.txt` → Apache-2.0, `BSD-3-Clause.txt` → BSD-3-Clause, `BSD-2-Clause.txt` → BSD-2-Clause;
  - `ISC.txt` → ISC, `MPL-2.0.txt` → MPL-2.0, `GPL-3.0-only.txt` → GPL-3.0-only;
  - `Unlicense.txt` → Unlicense, `Zlib.txt` → Zlib;
  - `GPL-2.0-only.txt`, `AGPL-3.0-only.txt`, `EPL-2.0.txt` and `SSPL-1.0.txt` → UNKNOWN.
- `TestLinkedGoModulesHaveGPL3CompatibleLicenses`:
  - skips under `testing.Short()` or without `go` on PATH;
  - takes the union over the six targets, run concurrently;
  - every linked module has a row; a missing one fails with a paste-ready line `add: <path>\t<guess>\t<first of LICENSE, LICENSE.md, LICENSE.txt, LICENCE, COPYING>`;
  - every `AND` part is a key of `gpl3CompatibleLicenses`, else `"<path>: <id> is not GPL-3.0 compatible; release blocked"`;
  - every listed file exists, and for each SPDX part at least one listed file contains all its phrases;
  - MPL-2.0 rows: no non-test `.go` file of the module contains `Incompatible With Secondary Licenses`;
  - every GPL row's module path appears in backticks in `THIRD_PARTY_NOTICES.md`;
  - no stale row (a module no target links), no duplicate, sorted.

**Populating the manifest** (maker-agent, once):
1. Run the test.
2. Review every printed `add:` line by reading the licence file in the module cache. Combined files may need `A AND B`, for example `gopkg.in/yaml.v3`: `MIT AND Apache-2.0`.
3. Paste the reviewed lines.
4. Any `UNKNOWN`, or a licence outside the allowlist that has no compatible alternative, means **stop and report**. Never widen the allowlist alone.

**Inventory docs** (docs-agent):
- `THIRD_PARTY_LIBRARIES.md`:
  - `:79` Paho row licence becomes `EPL-2.0 or EDL-1.0 (BSD-3-Clause); GPL-3.0 binaries use EDL-1.0`;
  - `:364` §9 row the same;
  - `:383-385` the go-licenses lines become `# Licences of every module linked into cmd/aurago (blocks a release on anything else)` and `go test ./internal/audit -run TestLinkedGoModulesHaveGPL3CompatibleLicenses -count=1`.
- `THIRD_PARTY_NOTICES.md`, after `:15`: "Release archives (`resources.dat`) and the Docker image (`/app`) carry `LICENSE`, this file and `licences/texts/`; each release's notes link the tagged source. `licences/linked-go-modules.tsv` records every module linked into `cmd/aurago` with the licence AuraGo relies on; `TestLinkedGoModulesHaveGPL3CompatibleLicenses` blocks a release on any other licence."
- `documentation/dependencies.md`, after `:32`: "A new or upgraded Go module linked into `cmd/aurago` needs a reviewed row in `licences/linked-go-modules.tsv`; the release builders run `TestLinkedGoModulesHaveGPL3CompatibleLicenses` first and stop on a GPL-3.0-incompatible licence."
- `TestWhatsAppLicenseNoticesStayAligned` must stay green unchanged.

**How it blocks a release:**
- **CI:** `security-gates.yml` runs `go test ./... -p 1`, which includes the test. `release.yml` `verify-source` waits for that run on the exact commit.
- **Builders:** `make_release.ps1` (also inside `release.yml`), `make_release.bat` and `make_deploy.sh` run the test before cleanup and build (6b).

### 6b: Packaging, release notes, retention

**`make_deploy.sh`:**
- After `derive_cleanup_artifact_lists` (`:176`):

  ```bash
  # A GPL-3.0-incompatible module linked into the binary blocks the release (licences/AGENTS.md).
  go test ./internal/audit -run 'TestLinkedGoModulesHaveGPL3CompatibleLicenses' -count=1 || {
    echo "A module linked into cmd/aurago has no reviewed GPL-3.0-compatible licence; nothing was built." >&2
    exit 1
  }
  ```

- After the config sed (`:237`):

  ```bash
  # GPL-3.0 combined work: the licence texts and notices travel with every binary.
  mkdir -p "$TMPDIR_RES/licences"
  cp LICENSE THIRD_PARTY_NOTICES.md "$TMPDIR_RES/"
  cp -r licences/texts "$TMPDIR_RES/licences/"
  cp licences/linked-go-modules.tsv "$TMPDIR_RES/licences/"
  ```

- After `tar -tzf … >/dev/null` (`:267`). The here-string avoids a SIGPIPE under `pipefail`:

  ```bash
  RESOURCE_ENTRIES="$(tar -tzf "$DEPLOY_DIR/$RESOURCES")"
  for notice in ./LICENSE ./THIRD_PARTY_NOTICES.md ./licences/texts/GPL-3.0-only.txt ./licences/linked-go-modules.tsv; do
    grep -Fx -- "$notice" <<< "$RESOURCE_ENTRIES" >/dev/null || { echo "resources.dat lacks $notice; nothing was published." >&2; exit 1; }
  done
  ```

**`make_release.ps1`:**
- After `$env:GOARCH = $hostGoArch` (`:104`), before deploy cleanup:

  ```powershell
  # A GPL-3.0-incompatible module linked into the binary blocks the release (licences/AGENTS.md).
  go test ./internal/audit -run 'TestLinkedGoModulesHaveGPL3CompatibleLicenses' -count=1
  if ($LASTEXITCODE -ne 0) { throw 'A module linked into cmd/aurago has no reviewed GPL-3.0-compatible licence; nothing was built.' }
  ```

- After the config `Set-Content` (`:192`). The destination `licences` exists, so `texts` is copied into it and does not nest:

  ```powershell
  # GPL-3.0 combined work: the licence texts and notices travel with every binary.
  Copy-Item -LiteralPath 'LICENSE', 'THIRD_PARTY_NOTICES.md' -Destination $tmpStage -Force
  $null = New-Item -ItemType Directory -Force -Path (Join-Path $tmpStage 'licences')
  Copy-Item -LiteralPath 'licences\texts' -Destination (Join-Path $tmpStage 'licences') -Recurse -Force
  Copy-Item -LiteralPath 'licences\linked-go-modules.tsv' -Destination (Join-Path $tmpStage 'licences') -Force
  ```

- After `tar -tzf $resourcesOut | Out-Null` and its check (`:215-218`):

  ```powershell
  $resourceEntries = @(tar -tzf $resourcesOut)
  foreach ($notice in @('./LICENSE', './THIRD_PARTY_NOTICES.md', './licences/texts/GPL-3.0-only.txt', './licences/linked-go-modules.tsv')) {
      if ($resourceEntries -notcontains $notice) { throw "resources.dat lacks $notice; nothing was published." }
  }
  ```

- Release notes: append to the `$notes` here-string after "### Included binaries" (`:464-467`). `{VERSION}` is replaced by the existing `-replace`.

  ```
  ### License and source

  AuraGo's source code is MIT licensed. The binaries of this release link the GPL-3.0 WhatsApp stack (`go.mau.fi/libsignal` through `go.mau.fi/whatsmeow`), so they are distributed as GPL-3.0 combined works.

  - Corresponding source: tag [{VERSION}](https://github.com/antibyte/AuraGo/tree/{VERSION}) ([tar.gz](https://github.com/antibyte/AuraGo/archive/refs/tags/{VERSION}.tar.gz)), with the module versions pinned in `go.mod` and `go.sum`.
  - License texts and notices: `LICENSE`, `THIRD_PARTY_NOTICES.md` and `licences/` in `resources.dat` and in the Docker image under `/app`, and [online at the tag](https://github.com/antibyte/AuraGo/blob/{VERSION}/THIRD_PARTY_NOTICES.md).
  ```

- Retention `:500` becomes `gh release delete $rel.tagName --yes 2>$null`, preceded by `# Keep the tag: every GPL-3.0 release links it as its corresponding source.`

**`make_release.bat`:**
- After the dirty check (`:114`):

  ```bat
  REM -- A GPL-3.0-incompatible module linked into the binary blocks the release.
  go test ./internal/audit -run TestLinkedGoModulesHaveGPL3CompatibleLicenses -count=1
  if errorlevel 1 (
      echo [ERROR] A module linked into cmd/aurago has no reviewed GPL-3.0-compatible licence. Nothing was built.
      exit /b 1
  )
  ```

- After the config sanitizer (`:189`):

  ```bat
  REM GPL-3.0 combined work: the licence texts and notices travel with every binary.
  copy /y "LICENSE" "%TMPSTAGE%\LICENSE" >nul || exit /b 1
  copy /y "THIRD_PARTY_NOTICES.md" "%TMPSTAGE%\THIRD_PARTY_NOTICES.md" >nul || exit /b 1
  xcopy /e /i /q "licences\texts" "%TMPSTAGE%\licences\texts" >nul || exit /b 1
  copy /y "licences\linked-go-modules.tsv" "%TMPSTAGE%\licences\linked-go-modules.tsv" >nul || exit /b 1
  ```

- After `tar -tzf "deploy\resources.dat" >nul` and its check (`:205-209`):

  ```bat
  for %%N in ("./LICENSE" "./THIRD_PARTY_NOTICES.md" "./licences/texts/GPL-3.0-only.txt" "./licences/linked-go-modules.tsv") do (
      tar -tzf "deploy\resources.dat" | findstr /l /x /c:%%~N >nul
      if errorlevel 1 (
          echo [ERROR] deploy\resources.dat lacks %%~N. Nothing was published.
          exit /b 1
      )
  )
  ```

- Notes block (`:379-402`): append inside the parentheses. Every `(`/`)` is escaped as `^(`/`^)`.

  ```bat
      echo.
      echo ### License and source
      echo.
      echo AuraGo's source code is MIT licensed. The binaries of this release link the GPL-3.0 WhatsApp stack ^(`go.mau.fi/libsignal` through `go.mau.fi/whatsmeow`^), so they are distributed as GPL-3.0 combined works.
      echo.
      echo - Corresponding source: tag [!VERSION!]^(https://github.com/antibyte/AuraGo/tree/!VERSION!^), tar.gz https://github.com/antibyte/AuraGo/archive/refs/tags/!VERSION!.tar.gz, with the module versions pinned in `go.mod` and `go.sum`.
      echo - License texts and notices: `LICENSE`, `THIRD_PARTY_NOTICES.md` and `licences/` in `resources.dat` and in the Docker image under `/app`.
  ```

- Retention `:472`: drop `--cleanup-tag`.

**`install.sh`** after `:2431` and **`update.sh`** after `:3268`. `update.sh` uses `cp -r` and `$DIR`.

```bash
    # Licence texts and notices of the GPL-3.0 combined binaries (always overwrite).
    if [ -d "$TMPEXT/licences" ]; then cp -a "$TMPEXT/licences" "$INSTALL_DIR/"; fi
    for notice in LICENSE THIRD_PARTY_NOTICES.md; do
        if [ -f "$TMPEXT/$notice" ]; then cp "$TMPEXT/$notice" "$INSTALL_DIR/$notice"; fi
    done
```

The generated `# BEGIN GENERATED RELEASE VERIFIER` blocks stay untouched.

**`Dockerfile`** after `COPY deploy/docker /app/deploy/docker` (`:84`). `.dockerignore` already re-includes `**/THIRD_PARTY_NOTICES.md` and `**/LICENSE*`, and `*.tsv`/`*.txt` are not excluded.

```dockerfile
# GPL-3.0 combined work: the licence texts and notices travel beside the binary;
# the release notes link the tagged source.
COPY LICENSE THIRD_PARTY_NOTICES.md /app/
COPY licences/texts /app/licences/texts
COPY licences/linked-go-modules.tsv /app/licences/linked-go-modules.tsv
```

**`security-gates.yml`**, Docker job, after "Build image without publishing":

```yaml
      - name: Verify the GPL-3.0 notices beside the AuraGo binary
        if: matrix.id == 'aurago'
        run: >-
          docker run --rm --entrypoint sh "aurago-ci-aurago:test" -c
          'test -s /app/LICENSE && test -s /app/THIRD_PARTY_NOTICES.md &&
          test -s /app/licences/texts/GPL-3.0-only.txt && test -s /app/licences/linked-go-modules.tsv'
```

**Static test** `TestReleasePackagingCarriesTheGPLNotices` (`internal/audit/whatsapp_release_packaging_test.go`, `readRepoFile`, CRLF normalized):
- each of `make_deploy.sh`, `make_release.ps1` and `make_release.bat` contains:
  - its licence-gate line, placed before its first `tar -czf` (`make_release.ps1`: before `Get-ChildItem -LiteralPath 'deploy'`);
  - its staging lines and its `./licences/texts/GPL-3.0-only.txt` listing check;
- `make_release.ps1` and `make_release.bat`:
  - notes contain `https://github.com/antibyte/AuraGo/tree/`, `archive/refs/tags/` and `GPL-3.0 combined works`;
  - no line contains `--cleanup-tag`;
- `install.sh` and `update.sh` contain `"$TMPEXT/licences"` and `THIRD_PARTY_NOTICES.md`;
- `Dockerfile` contains the three COPY lines;
- `.dockerignore` contains `!**/THIRD_PARTY_NOTICES.md` and `!**/LICENSE*` and no `licences` line;
- `security-gates.yml` contains `test -s /app/licences/texts/GPL-3.0-only.txt`.

**Tests that must stay green unchanged:**
- `TestReleasePowerShellPassesPinnedNativeBuildArguments` (its extracted segments are untouched);
- `TestWindowsReleaseUploadsDraftBeforePublishingAndRetention`;
- `TestMakeDeploy*`, `TestMakeReleaseRequiresCleanExplicitBuildIdentity`;
- `TestReleaseCI*`, `TestReleaseBootstrapScriptsMatch`;
- `TestRootDockerignoreKeepsSecretsAndWorktreesOutOfTheBuildContext`, `TestGoSecurityChecksRunIndependently`.

**Local verification and what cannot be verified here:**
- **Script syntax:**
  - `bash -n make_deploy.sh install.sh update.sh`;
  - `pwsh -NoProfile -Command '$e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile("make_release.ps1",[ref]$null,[ref]$e); if ($e) { $e; exit 1 }'`.
- **Windows bsdtar listing form** (cheapest proof of the `./licences/...` entries): in `disposable/_whatsapp/tarprobe`, create `stage/licences/texts/x.txt`, run `tar -czf t.tgz -C stage .`, then `tar -tzf t.tgz`. The output must show `./licences/texts/x.txt`; else adjust the three listing checks.
- **The `resources.dat` contents:** a full local build (`AURAGO_RUN_RELEASE_BUILD_TESTS=1 go test ./internal/audit -run '^TestMakeDeployCustomTargetArtifactsAreChecksummedAndCleaned$' -count=1`) needs a clean committed tree and the ignored `deploy/phonon` inputs (root `AGENTS.md:163`). Run it only if those exist. Otherwise the builders' own listing check proves it on the first release.
- **The Docker image:** a local `docker build` needs the prepared Phonon resources. The new CI step proves the notices on every push.
- **Release notes rendering and the GitHub tag links:** verifiable only on the first real release. That is a user acceptance item; the static test pins the text.

**GitNexus:** no indexed Go symbol is edited (new test files, scripts, YAML). Run `detect-changes`.

**Commits:**
- `build(release): block releases on a GPL-3.0-incompatible linked module` (6a);
- `build(release): ship the GPL-3.0 notices with resources.dat and the image and link the tagged source` (6b).

---

## Task 7: Parity sweep, DOX closeout and acceptance (B6)

### 7a: Sweep classification

The read-only pre-run on 2026-11-02 used ripgrep over the worktree with the master plan's exclusions, and found 384 files naming Telegram. It skips items stages 2–3b covered. The re-run below must reproduce it with `git grep`:

```bash
mkdir -p disposable/_whatsapp && cd "$(git rev-parse --show-toplevel)"
git grep -il telegram -- ':!vendor' ':!graft' ':!.gitnexus' ':!*.md' ':!plans' ':!reports' ':!documentation' ':!disposable' | sort > disposable/_whatsapp/tg.txt
git grep -il whatsapp -- ':!vendor' ':!graft' ':!.gitnexus' | sort > disposable/_whatsapp/wa.txt
comm -23 disposable/_whatsapp/tg.txt disposable/_whatsapp/wa.txt > disposable/_whatsapp/tg-only.txt
comm -12 disposable/_whatsapp/tg.txt disposable/_whatsapp/wa.txt | while read -r f; do git grep -n -i telegram -- "$f" | grep -v -i whatsapp; done > disposable/_whatsapp/both-tg-lines.txt
```

**Mirror (fixed in 7b):**
1. `internal/flows/catalog_actions_notify_hooks_test.go`: the notify-node hardening loops omit `TypeWhatsApp`:
   - `TestNotifyFailsClosed` (`:157-159`);
   - `TestNotifyValidate` rows and nil-node loop (`:240-244`, `:283`);
   - `notifyCheckArgs` (`:329`);
   - `TestNotifyHooksSurviveOddParams` (`:359-361`);
   - `TestNotifyNeedsAListedChannel` (`:482`).
2. `internal/server/flows_catalog_env_test.go:30`: `TestFlowToolAvailability` has no `send_whatsapp` row for the stage-3 mapping `flows_catalog_env.go:227-228`.
3. `internal/server/flows_tool_invoker_documents_hardening_test.go:225-237` and the comment `flows_tool_invoker_documents.go:26`: a documents-folder attachment must pass unchanged to `send_whatsapp`.
4. `.github/workflows/security-gates.yml:127` and `internal/audit/security_gates_test.go:45`: the race job lacks `./internal/whatsapp` (concurrent workers, send queue, `eventMu`).

**Covered by this stage:** `internal/server/security_check.go` (Task 2), `internal/agent/maintenance_briefing.go` (Task 3).

**Deliberately not mirrored.** Append to the master plan list, dated 2026-11-02:
- Telegram's own implementation: `internal/telegram/**`, `internal/tools/telegram_document*.go`, `internal/server/telegram_{runtime,handlers}*.go`, `internal/integrationstatus/telegram*.go`, `go.mod`/`go.sum` (the bot module).
- `ui/lang/config/generic/<16>.json` (`config.telegram.*_label`): WhatsApp field labels live in `ui/lang/config/whatsapp/`.
- The connection-test button (existing rule) also covers:
  - `internal/server/integration_test_handlers.go:91-98` and `server_routes_config.go:126-128`;
  - `ui/config_test_buttons_test.go:24`, `ui/config_spacing_browser_test.go:21`, `ui/config_precision_workspace_test.go:624`.
- `internal/server/self_test_handlers.go:52-54` (+`_test`): Telegram has no `Enabled` field. WhatsApp is found by reflection (stage-2 deviation 6).
- `ui/js/desktop/apps/easydrag-runs.js:21`, `easydrag-publish.js:7` (`sends_message: 'brand-telegram'`): a generic effect icon, not a channel.
- Built-in templates stay unchanged (3b), which covers:
  - `internal/flows/templates_builtin*.go`, `service_test.go`, `catalog_tools_quarantine_test.go`, `catalog_i18n_hardening_test.go`, `helpers_test.go`;
  - `ui/testdata/easydrag-fixture.js`, `scripts/test-easydrag*.mjs`.
- `internal/agent/dispatch_email.go:22` (`logTextRunes`) and `dispatch_messaging.go:38,46,274`: `send_whatsapp` logs no model text.
- Telegram's `title` and file-only precheck have no WhatsApp counterpart. The WhatsApp equivalents are `send_whatsapp_test.go` and `TestFlowCopyRefusalDoesNotRecommendAnAllowRule`. Covered files: `internal/agent/flow_tool_extensions*_test.go`, `send_telegram_path_hardening_test.go`, `flow_tool_output_provenance_test.go`.
- `internal/persondetect/annotate.go:22,25`: Telegram's photo limits are the strictest, and WhatsApp gets the same frame. `internal/newspaper/render.go:136` and `internal/persondetect/alert_rules.go:8` are comments.
- Bot-token secrets (WhatsApp has none):
  - `internal/logger/redaction_test.go`;
  - token rows in `catalog_actions_notify_hooks_test.go:98-106`;
  - `internal/server/config_write_protection_test.go`, `sip_config_lifecycle_test.go`;
  - `.gitignore:43`, `.gitnexusignore:165` (`telegram_token.txt`).
- `internal/server/config_handlers_test.go:1599`: an illustrative partial section list, not a registry.
- `internal/tools/skill_templates.go`: part of the Python skill template rule.
- Test fixtures that use Telegram as a sample:
  - `internal/memory/*_test.go`, `internal/persondetect/{api_types,service_api}_test.go`;
  - `internal/server/{go2rtc_detection_streams,go2rtc_detection_config,dashboard_memory_curation,config_draft_test_config,newspaper_handlers,newspaper_delivery_link_preview,flows_handlers_catalog,flows_catalog_env_hardening}_test.go`;
  - `internal/config/go2rtc_detection_test.go`, `internal/heartbeat/runner_test.go`, `internal/security/provenance_batch_test.go`, `internal/rocketchat/bot_session_test.go`;
  - remaining `internal/agent/*_test.go` (task router, emotion, operational notice, journal, loopback);
  - `ui/desktop_*_browser_test.go`, `scripts/test-ui-regressions.mjs`.
  - The 7b maker confirms each is a fixture.
- `agent-tools/*.txt`: untracked tool output. `git ls-files agent-tools` must be empty; `git grep` does not list it.

**Files naming both.** Spot-checked; every Telegram-only line is Telegram's own section or lifecycle with a WhatsApp counterpart already present:
- `internal/server/server.go:649-654`, `config_handlers_main.go`, `server_routes.go`;
- `ui/js/config/main.js:155-156,687-688`;
- `config_template.yaml`.

The full `both-tg-lines.txt` pass classifies any new hit the same way.

### 7b: Sweep fixes (maker-agent)

- **`catalog_actions_notify_hooks_test.go`:**
  - add `TypeWhatsApp: {"message": "m"}` to the params maps at `:158` and `:360`;
  - `TypeWhatsApp` in the nil-node loop (`:283`) and in `TestNotifyNeedsAListedChannel` (`[]string{TypeTelegram, TypeWhatsApp, TypePush}`);
  - rows `{"whatsapp fine", TypeWhatsApp, {"message": "m", "file": "a.pdf"}, nil}`;
  - `{"whatsapp too long", TypeWhatsApp, {"message": long(whatsAppTextLimit + 1)}, []string{"message"}}`;
  - `{"whatsapp file", TypeWhatsApp, {"file": map[string]any{"name": "x"}}, []string{"file"}}`;
  - `{"whatsapp templates", TypeWhatsApp, {"message": "{{trigger.data.m}}", "file": "{{trigger.data.f}}"}, nil}`;
  - in `notifyCheckArgs`: `case "send_whatsapp": if _, ok := c.Args["title"]; ok || utf8.RuneCountInString(str("message")) > whatsAppTextLimit { return "a whatsapp text that does not fit" }`.
- **`flows_catalog_env_test.go`:** `check("send_whatsapp", flows.NeedsSetupState, "whatsapp")` after the Telegram row.
- **`TestC13AttachmentsFromTheDocumentsFolder`:**
  - the allow map gains `"send_whatsapp": true`;
  - a third call `c13Request("send_whatsapp", {"message": "x", "file_path": pdf})` must leave `(*calls)[2].tc.Params["file_path"] == pdf`.
  - The comment at `flows_tool_invoker_documents.go:26` reads "send_telegram, send_whatsapp and send_email open their files with tools.OpenOutgoingAttachment".
- **Race job:**
  - `security-gates.yml:127` becomes `go test -race ./internal/llm ./internal/server ./internal/memory ./internal/tools ./internal/telegram ./internal/whatsapp ./internal/agent -p 1 -timeout=30m`;
  - `security_gates_test.go:45` package list gains `"whatsapp"`.
  - `-race` needs CGO and a C compiler, so this box cannot run it. The static test pins the wiring, and the first CI run is the proof.

Commands:
- `CGO_ENABLED=0 go test ./internal/flows/ -run '(TestNotify|TestWhatsAppNode)' -count=1`
- `CGO_ENABLED=0 go test ./internal/server/ -run '(TestFlowToolAvailability|TestC13AttachmentsFromTheDocumentsFolder)' -count=1`
- `CGO_ENABLED=0 go test ./internal/audit/ -run '(TestGoSecurityChecksRunIndependently)' -count=1`

**GitNexus:** none (tests, a comment, YAML). Run `detect-changes`.
**Commit:** `test(whatsapp): close the Telegram parity sweep`

### 7c: DOX (docs-agent)

- **`internal/whatsapp/AGENTS.md`:**
  - Purpose: stage 4 adds the operator guide, security hints, the briefing check and the release licence gate.
  - Stage scope: stage 4 is done; operator guide `documentation/whatsapp_setup.md`.
  - Verification: the new tests by name, and the stage-4 real-device checklist (7d), which this file does not claim.
- **`internal/server/AGENTS.md`**, WhatsApp Runtime Contract:
  - `whatsapp_self_chat_personal_account` and `whatsapp_dedicated_no_owner` are config-only, suppressed in egg mode and carry no number;
  - their texts are translated in `ui/lang/config/security`.
- **`internal/planner/AGENTS.md:58`:** the briefing checks include `whatsapp` (passive status). "Background checks never send Telegram or WhatsApp messages."
- **`licences/AGENTS.md`:**
  - Ownership adds `linked-go-modules.tsv`.
  - Local Contracts:
    - a row per linked module with the licence relied on;
    - Paho uses EDL-1.0;
    - the allowlist is the test's map, and widening it needs a user decision;
    - the builders and Security Gates run the gate.
  - Verification: the test command.
- **Root `AGENTS.md`** (check `git ls-files -v AGENTS.md` prints `H`):
  - `:248` append "operator guide, security hints, briefing check, release licence gate";
  - `:158` "…retains the latest three stable AuraGo releases and keeps the tags of deleted ones (their GPL-3.0 corresponding source) unless retention is disabled";
  - `:162` "…without authorization to delete older releases".
- **Master plan:** the "Deliberately not mirrored" additions from 7a and one line under "Risks": "Stage 4 added the release licence gate (`TestLinkedGoModulesHaveGPL3CompatibleLicenses`)."
- **Unchanged, with reasons:**
  - `internal/agent/AGENTS.md`: no contract lists briefing checks;
  - `ui/AGENTS.md`: no new UI contract;
  - `internal/flows/AGENTS.md`: tests only;
  - `internal/security`, `qrcode` and `fileutil` guides: not touched.

**Commit:** `docs(whatsapp): close stage 4 and record the parity sweep`

### 7d: Real-device acceptance (the user, checklist only)

- [ ] First run the still-open device checklists:
  - stage 1 Task 15 (`plans/2026-10-31-whatsapp-1-foundation.md`);
  - stage 2 Task 11;
  - stage 3 Task 12;
  - 3b "Task 12 additions".
- [ ] Design checklist on the second number:
  - link by QR and by pairing code;
  - a text round trip;
  - a voice note in, and one out (it plays);
  - an image and a document in;
  - a file out (`send_whatsapp`);
  - a notification;
  - unlink;
  - restart without relinking;
  - self-chat mode.
- [ ] Follow `documentation/whatsapp_setup.md` from scratch. Every step matches the UI, and the phone lists "AuraGo" under Linked devices.
- [ ] Security page:
  - self-chat on: "WhatsApp: self-chat mode uses your personal account" (German UI: translated);
  - dedicated without owner: "no owner number", which disappears after saving an owner.
- [ ] Danger Zone → Unsafe host execution names the WhatsApp device keys (en and de).
- [ ] Config → WhatsApp shows the self-chat prefix help.
- [ ] Morning briefing:
  - while healthy: `whatsapp: passed`;
  - after unlinking on the phone: `whatsapp: failed (logged_out)`;
  - disabled: `whatsapp: skipped (disabled)`;
  - the check sends no WhatsApp message.
- [ ] First real release, when the user publishes one:
  - the release page shows "License and source" with working tag and tar.gz links;
  - `resources.dat` contains `LICENSE`, `THIRD_PARTY_NOTICES.md` and `licences/texts/GPL-3.0-only.txt`;
  - a fresh `install.sh` install has them in the install directory;
  - the GHCR image has `/app/LICENSE`.

---

## New UI strings (all locales)

### `ui/lang/config/security/<locale>.json`: `security.hint.whatsapp_self_chat_personal_account.title` / `.description`

| Locale | Title | Description |
| --- | --- | --- |
| cs | WhatsApp: režim chatu se sebou používá váš osobní účet | AuraGo je propojené zařízení vašeho vlastního účtu WhatsApp. Čte jen váš chat sám se sebou, ale klíče zařízení otevírají všechny vaše chaty a zablokování kvůli automatizovanému používání zasáhne vaše osobní číslo. Bezpečnější je vlastní číslo pro AuraGo. |
| da | WhatsApp: selvchat-tilstand bruger din personlige konto | AuraGo er en tilknyttet enhed på din egen WhatsApp-konto. Den læser kun din chat med dig selv, men enhedsnøglerne giver adgang til alle dine chats, og en blokering for automatiseret brug rammer dit personlige nummer. Et eget AuraGo-nummer er sikrere. |
| de | WhatsApp: Selbst-Chat-Modus nutzt dein persönliches Konto | AuraGo ist ein verknüpftes Gerät deines eigenen WhatsApp-Kontos. Es liest nur deinen Chat mit dir selbst, aber die Geräteschlüssel öffnen alle deine Chats, und eine Sperre wegen automatisierter Nutzung trifft deine persönliche Nummer. Eine eigene AuraGo-Nummer ist sicherer. |
| el | WhatsApp: η λειτουργία συνομιλίας με τον εαυτό σου χρησιμοποιεί τον προσωπικό σου λογαριασμό | Το AuraGo είναι συνδεδεμένη συσκευή του δικού σου λογαριασμού WhatsApp. Διαβάζει μόνο τη συνομιλία με τον εαυτό σου, αλλά τα κλειδιά της συσκευής δίνουν πρόσβαση σε όλες τις συνομιλίες σου, και ένας αποκλεισμός λόγω αυτοματοποιημένης χρήσης πλήττει τον προσωπικό σου αριθμό. Ένας ξεχωριστός αριθμός είναι ασφαλέστερος. |
| en | WhatsApp: self-chat mode uses your personal account | AuraGo is a linked device of your own WhatsApp account. It reads only your Message yourself chat, but the device keys give access to all your chats, and a ban for automated use hits your personal number. A dedicated number is safer. |
| es | WhatsApp: el modo de chat contigo mismo usa tu cuenta personal | AuraGo es un dispositivo vinculado de tu propia cuenta de WhatsApp. Solo lee tu chat contigo mismo, pero las claves del dispositivo dan acceso a todos tus chats, y un bloqueo por uso automatizado afecta a tu número personal. Un número propio para AuraGo es más seguro. |
| fr | WhatsApp : le mode discussion avec vous-même utilise votre compte personnel | AuraGo est un appareil connecté de votre propre compte WhatsApp. Il lit uniquement votre discussion avec vous-même, mais les clés de l’appareil donnent accès à toutes vos discussions, et un blocage pour usage automatisé touche votre numéro personnel. Un numéro dédié est plus sûr. |
| hi | WhatsApp: सेल्फ़-चैट मोड आपके निजी खाते का उपयोग करता है | AuraGo आपके अपने WhatsApp खाते का लिंक किया गया डिवाइस है। यह केवल आपकी खुद से की गई चैट पढ़ता है, लेकिन डिवाइस की कुंजियाँ आपकी सभी चैट तक पहुँच देती हैं, और स्वचालित उपयोग के कारण लगी रोक आपके निजी नंबर पर लगती है। अलग नंबर अधिक सुरक्षित है। |
| it | WhatsApp: la modalità chat con te stesso usa il tuo account personale | AuraGo è un dispositivo collegato del tuo account WhatsApp. Legge solo la tua chat con te stesso, ma le chiavi del dispositivo danno accesso a tutte le tue chat, e un blocco per uso automatizzato colpisce il tuo numero personale. Un numero dedicato è più sicuro. |
| ja | WhatsApp: セルフチャットモードは個人アカウントを使います | AuraGo はあなた自身の WhatsApp アカウントのリンク済みデバイスです。読むのは自分とのチャットだけですが、デバイスの鍵ですべてのチャットにアクセスでき、自動利用による利用停止は個人の番号に及びます。専用の番号のほうが安全です。 |
| nl | WhatsApp: zelfchatmodus gebruikt je persoonlijke account | AuraGo is een gekoppeld apparaat van je eigen WhatsApp-account. Het leest alleen je chat met jezelf, maar de apparaatsleutels geven toegang tot al je chats, en een blokkade wegens geautomatiseerd gebruik treft je persoonlijke nummer. Een eigen AuraGo-nummer is veiliger. |
| no | WhatsApp: selvchat-modus bruker den personlige kontoen din | AuraGo er en tilknyttet enhet på din egen WhatsApp-konto. Den leser bare chatten med deg selv, men enhetsnøklene gir tilgang til alle chattene dine, og en utestengelse for automatisert bruk rammer det personlige nummeret ditt. Et eget AuraGo-nummer er tryggere. |
| pl | WhatsApp: tryb czatu z samym sobą używa Twojego konta osobistego | AuraGo jest połączonym urządzeniem Twojego własnego konta WhatsApp. Czyta tylko Twój czat z samym sobą, ale klucze urządzenia dają dostęp do wszystkich Twoich czatów, a blokada za automatyczne użycie dotyka Twojego prywatnego numeru. Osobny numer jest bezpieczniejszy. |
| pt | WhatsApp: o modo de conversa consigo mesmo usa a sua conta pessoal | O AuraGo é um dispositivo conectado da sua própria conta do WhatsApp. Ele lê apenas a sua conversa consigo mesmo, mas as chaves do dispositivo dão acesso a todas as suas conversas, e um bloqueio por uso automatizado atinge o seu número pessoal. Um número próprio para o AuraGo é mais seguro. |
| sv | WhatsApp: självchattläget använder ditt personliga konto | AuraGo är en länkad enhet på ditt eget WhatsApp-konto. Den läser bara din chatt med dig själv, men enhetsnycklarna ger åtkomst till alla dina chattar, och en spärr för automatiserad användning drabbar ditt personliga nummer. Ett eget AuraGo-nummer är säkrare. |
| zh | WhatsApp：自聊模式使用你的个人账号 | AuraGo 是你自己 WhatsApp 账号的关联设备。它只读取你与自己的聊天，但设备密钥可以访问你的所有聊天；因自动化使用而被封禁时，受影响的是你的个人号码。使用 AuraGo 专用号码更安全。 |

### `ui/lang/config/security/<locale>.json`: `security.hint.whatsapp_dedicated_no_owner.title` / `.description`

| Locale | Title | Description |
| --- | --- | --- |
| cs | WhatsApp: chybí číslo vlastníka | Je zvolené vlastní číslo pro AuraGo, ale není zadané žádné platné číslo vlastníka. Kanál WhatsApp zůstane vypnutý a nikomu neodpoví, dokud nezadáte své číslo v mezinárodním formátu. |
| da | WhatsApp: intet ejernummer | Eget AuraGo-nummer er valgt, men der er intet gyldigt ejernummer. WhatsApp-kanalen forbliver slukket og svarer ingen, indtil du tilføjer dit nummer i internationalt format. |
| de | WhatsApp: keine Besitzernummer | Die eigene AuraGo-Nummer ist gewählt, aber es ist keine gültige Besitzernummer eingetragen. Der WhatsApp-Kanal bleibt aus und antwortet niemandem, bis du deine Nummer im internationalen Format einträgst. |
| el | WhatsApp: δεν υπάρχει αριθμός κατόχου | Έχει επιλεγεί ξεχωριστός αριθμός AuraGo, αλλά δεν έχει οριστεί έγκυρος αριθμός κατόχου. Το κανάλι WhatsApp μένει ανενεργό και δεν απαντά σε κανέναν, μέχρι να προσθέσεις τον αριθμό σου σε διεθνή μορφή. |
| en | WhatsApp: no owner number | Dedicated mode is on, but no valid owner number is set. The WhatsApp channel stays off and answers nobody until you add your number in international format. |
| es | WhatsApp: no hay número de propietario | Está elegido el número propio de AuraGo, pero no hay ningún número de propietario válido. El canal de WhatsApp sigue apagado y no responde a nadie hasta que añadas tu número en formato internacional. |
| fr | WhatsApp : aucun numéro de propriétaire | Le numéro dédié à AuraGo est choisi, mais aucun numéro de propriétaire valide n’est défini. Le canal WhatsApp reste désactivé et ne répond à personne tant que vous n’ajoutez pas votre numéro au format international. |
| hi | WhatsApp: कोई मालिक नंबर नहीं | AuraGo का अलग नंबर चुना गया है, लेकिन कोई मान्य मालिक नंबर सेट नहीं है। जब तक आप अपना नंबर अंतरराष्ट्रीय फ़ॉर्मैट में नहीं जोड़ते, WhatsApp चैनल बंद रहता है और किसी को जवाब नहीं देता। |
| it | WhatsApp: nessun numero del proprietario | È scelto il numero dedicato ad AuraGo, ma non è impostato alcun numero del proprietario valido. Il canale WhatsApp resta spento e non risponde a nessuno finché non aggiungi il tuo numero in formato internazionale. |
| ja | WhatsApp: オーナー番号がありません | AuraGo 専用の番号が選ばれていますが、有効なオーナー番号が設定されていません。国際形式で自分の番号を追加するまで、WhatsApp チャンネルはオフのままで、誰にも返信しません。 |
| nl | WhatsApp: geen eigenaarsnummer | Een eigen AuraGo-nummer is gekozen, maar er is geen geldig eigenaarsnummer ingesteld. Het WhatsApp-kanaal blijft uit en antwoordt niemand tot je je nummer in internationaal formaat toevoegt. |
| no | WhatsApp: ikke noe eiernummer | Eget AuraGo-nummer er valgt, men ingen gyldige eiernumre er angitt. WhatsApp-kanalen forblir av og svarer ingen før du legger til nummeret ditt i internasjonalt format. |
| pl | WhatsApp: brak numeru właściciela | Wybrano osobny numer AuraGo, ale nie ustawiono żadnego prawidłowego numeru właściciela. Kanał WhatsApp pozostaje wyłączony i nikomu nie odpowiada, dopóki nie dodasz swojego numeru w formacie międzynarodowym. |
| pt | WhatsApp: nenhum número de proprietário | O número próprio do AuraGo está escolhido, mas não há nenhum número de proprietário válido. O canal do WhatsApp continua desligado e não responde a ninguém até que adicione o seu número no formato internacional. |
| sv | WhatsApp: inget ägarnummer | Eget AuraGo-nummer är valt, men inget giltigt ägarnummer är angivet. WhatsApp-kanalen förblir avstängd och svarar ingen förrän du lägger till ditt nummer i internationellt format. |
| zh | WhatsApp：没有所有者号码 | 已选择 AuraGo 专用号码，但没有设置有效的所有者号码。在你以国际格式添加自己的号码之前，WhatsApp 渠道保持关闭，不会回复任何人。 |

docs-agent aligns "owner number" with each locale's `config.whatsapp.owner_numbers`.

### `ui/lang/config/danger/<locale>.json`: `config.danger.unsafe_host_execution.desc` (changed)

The new sentence goes before "Every use is logged…". The rest stays byte-identical to `:25`. docs-agent uses the locale's existing word for the vault if `ui/lang/config/misc` differs.

| Locale | Inserted sentence |
| --- | --- |
| cs | Příkazy na hostiteli pak mohou číst datovou složku AuraGo včetně souboru trezoru a klíčů propojeného zařízení WhatsApp. |
| da | Kommandoer på værten kan så læse AuraGos datamappe, også vault-filen og nøglerne til den tilknyttede WhatsApp-enhed. |
| de | Befehle auf dem Host können dann AuraGos Datenordner lesen, auch die Vault-Datei und die Schlüssel des verknüpften WhatsApp-Geräts. |
| el | Οι εντολές στον κεντρικό υπολογιστή μπορούν τότε να διαβάσουν τον φάκελο δεδομένων του AuraGo, μαζί με το αρχείο του θησαυροφυλακίου και τα κλειδιά της συνδεδεμένης συσκευής WhatsApp. |
| en | Host commands can then read AuraGo's data folder, including the vault file and the keys of the linked WhatsApp device. |
| es | Los comandos del equipo pueden leer entonces la carpeta de datos de AuraGo, incluido el archivo de la bóveda y las claves del dispositivo de WhatsApp vinculado. |
| fr | Les commandes de l’hôte peuvent alors lire le dossier de données d’AuraGo, y compris le fichier du coffre et les clés de l’appareil WhatsApp connecté. |
| hi | तब होस्ट पर चलने वाले कमांड AuraGo का डेटा फ़ोल्डर पढ़ सकते हैं, जिसमें वॉल्ट फ़ाइल और लिंक किए गए WhatsApp डिवाइस की कुंजियाँ भी शामिल हैं। |
| it | I comandi sull’host possono quindi leggere la cartella dati di AuraGo, compresi il file del vault e le chiavi del dispositivo WhatsApp collegato. |
| ja | その場合、ホスト上のコマンドは AuraGo のデータフォルダーを読み取れます。保管庫ファイルやリンク済み WhatsApp デバイスの鍵も含まれます。 |
| nl | Opdrachten op de host kunnen dan de gegevensmap van AuraGo lezen, ook het kluisbestand en de sleutels van het gekoppelde WhatsApp-apparaat. |
| no | Kommandoer på verten kan da lese datamappen til AuraGo, også hvelvfilen og nøklene til den tilknyttede WhatsApp-enheten. |
| pl | Polecenia na hoście mogą wtedy czytać folder danych AuraGo, w tym plik sejfu i klucze połączonego urządzenia WhatsApp. |
| pt | Os comandos no anfitrião podem então ler a pasta de dados do AuraGo, incluindo o ficheiro do cofre e as chaves do dispositivo WhatsApp ligado. |
| sv | Kommandon på värden kan då läsa AuraGos datamapp, även valvfilen och nycklarna till den länkade WhatsApp-enheten. |
| zh | 此时主机上的命令可以读取 AuraGo 的数据文件夹，包括保险库文件和已关联 WhatsApp 设备的密钥。 |

### `ui/lang/config/whatsapp/<locale>.json`: `help.whatsapp.self_chat_prefix` (new)

| Locale | Text |
| --- | --- |
| cs | Jen v režimu chatu se sebou. AuraGo začíná své zprávy tímto textem (nejvýše 16 znaků). Poznámky sobě, které jím začínají, se ignorují. |
| da | Kun i selvchat-tilstand. AuraGo starter sine beskeder med denne tekst (højst 16 tegn). Noter til dig selv, der starter med den, ignoreres. |
| de | Nur im Selbst-Chat-Modus. AuraGo beginnt seine Nachrichten mit diesem Text (höchstens 16 Zeichen). Notizen an dich selbst, die damit beginnen, werden ignoriert. |
| el | Μόνο στη συνομιλία με τον εαυτό σου. Το AuraGo ξεκινά τα μηνύματά του με αυτό το κείμενο (έως 16 χαρακτήρες). Οι σημειώσεις προς τον εαυτό σου που ξεκινούν με αυτό αγνοούνται. |
| en | Self-chat mode only. AuraGo starts its messages with this text (at most 16 characters). Notes to yourself that start with it are ignored. |
| es | Solo en el modo de chat contigo mismo. AuraGo empieza sus mensajes con este texto (16 caracteres como máximo). Las notas para ti que empiecen con él se ignoran. |
| fr | Uniquement en mode discussion avec vous-même. AuraGo commence ses messages par ce texte (16 caractères au maximum). Les notes à vous-même qui commencent par ce texte sont ignorées. |
| hi | केवल सेल्फ़-चैट मोड में। AuraGo अपने संदेश इसी टेक्स्ट से शुरू करता है (अधिकतम 16 अक्षर)। खुद को भेजे गए जो नोट इससे शुरू होते हैं, उन्हें अनदेखा किया जाता है। |
| it | Solo nella modalità chat con te stesso. AuraGo inizia i suoi messaggi con questo testo (al massimo 16 caratteri). Le note a te stesso che iniziano così vengono ignorate. |
| ja | セルフチャットモードのみ。AuraGo は自分のメッセージをこの文字列で始めます（最大 16 文字）。この文字列で始まる自分宛てのメモは無視されます。 |
| nl | Alleen in de zelfchatmodus. AuraGo begint zijn berichten met deze tekst (maximaal 16 tekens). Notities aan jezelf die ermee beginnen, worden genegeerd. |
| no | Bare i selvchat-modus. AuraGo starter meldingene sine med denne teksten (maks. 16 tegn). Notater til deg selv som starter med den, ignoreres. |
| pl | Tylko w trybie czatu z samym sobą. AuraGo zaczyna swoje wiadomości od tego tekstu (maks. 16 znaków). Notatki do siebie, które od niego się zaczynają, są pomijane. |
| pt | Apenas no modo de conversa consigo mesmo. O AuraGo começa as mensagens dele com este texto (no máximo 16 caracteres). As notas para si mesmo que começam com ele são ignoradas. |
| sv | Bara i självchattläget. AuraGo börjar sina meddelanden med den här texten (högst 16 tecken). Anteckningar till dig själv som börjar med den ignoreras. |
| zh | 仅限自聊模式。AuraGo 会以此文本开头发送消息（最多 16 个字符）。以此开头的自我备注会被忽略。 |

### Fix

- `ui/lang/easydrag/cs.json` `easydrag.node.notify_whatsapp.description` becomes `Pošle vám zprávu přes WhatsApp, volitelně se souborem.`

---

## Security review checklist (security-agent, one pass after B4)

- **B2 hints:**
  - config-only, egg mode suppresses them;
  - no number or masked JID in any text;
  - `whatsAppHasValidOwnerNumber` matches the runtime's `hasValidOwnerNumber` rule (`NormalizeE164`);
  - nothing auto-fixes.
- **Danger Zone:** the claim "host commands can read the data folder, vault file and device keys" is accurate for `allow_unsafe_host_execution` and does not overstate (the vault is encrypted at rest).
- **`whatsapp_setup.md` "Security and privacy":**
  - it matches the stage-1 deviation 16 boundary;
  - it does not promise that shell or Python are denied.
- **B4 gate:**
  - `-short` is the only skip, and Security Gates and the builders never pass `-short`;
  - an UNKNOWN or non-allowlisted licence fails;
  - MPL "Incompatible With Secondary Licenses" fails;
  - the gate runs before any cleanup or build in all three builders.
- **B4 packaging:**
  - `install.sh` and `update.sh` copy only fixed names from the already verified, extracted archive, with no globbing of archive paths into the install directory;
  - the Docker COPY sources are tracked files outside the excluded directories;
  - dropping `--cleanup-tag` deletes no less data than before, only keeps tags;
  - the release notes carry no secret.

---

## Verification sweep (test-agent, after B6)

```bash
CGO_ENABLED=0 go build ./...
CGO_ENABLED=0 go vet ./internal/server/ ./internal/agent/ ./internal/audit/ ./internal/flows/
CGO_ENABLED=0 go test ./internal/server/ -run '(TestCheckSecurity|WhatsApp|TestFlowToolAvailability|TestC13AttachmentsFromTheDocumentsFolder|TestNewspaper)' -count=1
CGO_ENABLED=0 go test ./internal/agent/ -run '(TestWhatsAppIntegrationCheck|TestMaintenanceIntegrationChecks|TestMorningBriefing|WhatsApp)' -count=1
CGO_ENABLED=0 go test ./internal/flows/ -count=1
CGO_ENABLED=0 go test ./internal/audit/ -count=1
CGO_ENABLED=0 go test ./internal/whatsapp/ ./internal/integrationstatus/ -count=1
go test -count=1 ./ui/... -run '(Precision|Config|I18N|Translations|WhatsApp|TestNewspaperTranslations)'
node --check ui/cfg/whatsapp.js
bash -n make_deploy.sh install.sh update.sh
pwsh -NoProfile -Command '$e=$null; [void][System.Management.Automation.Language.Parser]::ParseFile("make_release.ps1",[ref]$null,[ref]$e); if ($e) { $e; exit 1 }'
```

- Report the PASS counts and the duration of `TestLinkedGoModulesHaveGPL3CompatibleLicenses`.
- Then run `node ../../.gitnexus/run.cjs detect-changes --scope compare --base-ref main --repo .`. `partial` or `truncated` means run it again.
- No browser smoke is required. The static UI tests cover the help line and the hints; the user's checklist covers the rendered pages.

## Risks

- **Licence manifest upkeep.** Every dependency bump that adds a module now needs a reviewed row. The test prints a paste-ready line, so the cost is a review, not a hunt.
- **Fingerprint limits.** The marker phrases prove the licence family, not every clause. A module that changes its licence between versions still fails only if its text changes. Upgrades stay a human review (`documentation/dependencies.md`).
- **Module availability in CI.** `go list -deps` for Windows and Darwin on Linux CI downloads modules the host build does not need. Network is available in Security Gates; a proxy outage fails the gate, which is the safe direction.
- **bsdtar listing form on Windows.** The PowerShell and batch listing checks assume `./`-prefixed forward-slash entries. The tar probe in 6b settles it before commit.
- **OCI label.** `docker/metadata-action` labels the image with the repository licence (MIT). The image now carries the authoritative notices. Overriding the label to `GPL-3.0-only` is a follow-up for the user, not part of this stage.
- **The race job adds WhatsApp.** A real data race in `internal/whatsapp` would turn Security Gates red on the next push. That is the intent, but it may need a fix round outside this plan.
