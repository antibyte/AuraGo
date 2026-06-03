# aurago-tui — Remaining Improvements & Known Gaps

This document lists the non-critical / larger items that were identified during the 2026-05 audit but left out of the immediate bug-fix pass. All hard bugs (scroll, resize, mutex, cookie, dangling streaming, restore safety, dead dependency) have been fixed.

## High-Value Future Work

### 1. Internationalization (i18n)
- All user-facing strings are currently hard-coded English.
- The main AuraGo Web UI supports 15 languages via `ui/lang/`.
- **Suggestion**: Add a simple `i18n/` module + fluent or similar, or at minimum a `translations.rs` const map for the most common UI strings (titles, buttons, status, help).

### 2. Multimodal / Vision & Image Upload
- The TUI chat cannot send images or use the vision capabilities that the backend + Web UI support.
- Would require:
  - File picker (ratatui has community crates or we can use `crossterm` + `dialoguer` in a blocking task).
  - Multipart or base64 upload to a new `/v1/chat/completions` extension or dedicated endpoint.
  - Rendering of image placeholders / thumbnails in the message list.

### 3. Performance & Scalability for Very Long Chats
- Current implementation keeps the entire `chat_messages` + rendered `Vec<Line>` in memory and re-builds on every frame.
- For sessions with thousands of messages the TUI can become slow.
- **Possible solutions**:
  - Virtual / windowed list (only render visible lines).
  - Switch the chat area from one giant `Paragraph` to a `List` or custom widget that only renders the viewport.
  - Prune old messages from the UI model (keep full history only on the server).

### 4. Tests
- There are currently almost no unit or integration tests for the TUI.
- High-value candidates:
  - `apply_sse_event` & streaming logic
  - Cursor movement + unicode edge cases
  - Scroll / auto-scroll state machine
  - Keybinding matrix per `KeyContext`

### 5. Theming & Accessibility
- Mood-based themes are nice, but there is no high-contrast mode or color-blind friendly palette.
- Keyboard-only navigation is already strong, but focus indicators could be more visible.

### 6. Feature Parity Gaps (non-blocking)
- No built-in update wizard inside the TUI (the separate `agocli --update` exists).
- No native file browser / media gallery viewer (the Media screen is mostly list + detail).
- Config editing is powerful but has no validation or "test connection" buttons for integrations.

## Notes on the Parallel Go agocli (bubbletea)

A lighter Go/Bubbletea TUI (`cmd/agocli` in feature worktrees) also exists. It focuses on:
- Chat (simpler)
- First-time `--setup` wizard
- `--update` self-update wizard

It may be merged or kept as a companion tool. The Rust `aurago-tui` is the full-featured daily driver.

## Completed During 2026-05-18 Audit Follow-up

All items from the original audit "Next Steps" list have been addressed (see `reports/aurago-tui-audit-2026-05-18.md` for full details).

- ✅ Removed accidental 443 MB full Zig 0.14 toolchain (`zig/`) + `zigcc.bat` (never wired into any build script, CI, or Cargo config; was bloating clones and disk).
- ✅ Fixed remaining hardcoded `Color::Yellow` / `Color::Black` in UI (now consistently uses `theme.*`).
- ✅ Extracted ~600 LOC of action dispatch + orchestration from monolithic `main.rs` into `src/actions.rs` (major maintainability win).
- ✅ Added first unit tests (cursor unicode handling, `apply_sse_event`, keybinding matrix).
- ✅ Expanded help overlay with previously undocumented global shortcuts.
- ✅ All changes pass `cargo check`, `clippy -D warnings`, and new tests.

**Last updated**: 2026-05-18 (full audit next-steps execution)

## How to Contribute

When adding a feature from this list, please:
1. Update this file (move the item to "Done").
2. Add at least one test for the new logic.
3. Consider i18n from day one for any new strings.

---

**Last updated**: 2026-05-18 (full audit next-steps + post-audit polish: overlays.rs extraction, color mapper unification, minimal i18n scaffolding)

## Completed in 2026-06 "gehe 1-4 an" Optimization Pass (Waves 1-3 + partial 4)
- ✅ P0: `cargo clippy -- -D warnings` now 0 errors (was 12 unused imports + dead_code from post-refactor). All files cleaned (actions, main, chat, overlays, i18n).
- ✅ P2 (item 2): tokio slimmed to exact features needed (no "full"); i18n scaffolding advanced (allows + 2 new fields + 2 strings wired in chat input + hint; 4 were pre-wired in overlays).
- ✅ P2/P3: Added `background_tasks` tracker + spawn_tracked/abort_all/prune to AppState; wired into navigate_to + tick + 7+ dashboard loads + clear chat + samples in actions. Nav aborts prior loaders (prevents pile-up on rapid switch).
- ✅ P1: Chat `draw_messages` now slices to viewport tail + buffer (start_idx via saturating) + full_logical for scrollbar. lines built + clone now O(viewport) not O(all history). Common live-chat-at-bottom case no longer rebuilds 1000s msgs every 100ms tick. Preserves wrap/stream/auto/unicode/scroll/indicator.
- ✅ P3: Import clean + new methods + tracker calls gave measurable hygiene win in main/actions (less noise); no forced large extract this pass (risk low, delta controlled per plan — main still contains run_app/event loop as expected).
- ✅ All waves: rtk cargo check/clippy -D/test passed after batches. 5 existing tests + logic intact. Disposable notes + this entry created.
- ✅ GitNexus impacts + GSD (plan.md + todos + enter/exit) + rtk + disposable rule followed before/during.

**Last updated**: 2026-06-02 (1-4 pass per user request; see .grok/.../plan.md and disposable/aurago-tui-wave*-*.md for details)

## Completed in 2026-06 "Weitere Verbesserungen" Wave A (F1 partial + F4 + F5)
- ✅ F1: Tracker wiring completed for all repeatable nav/action spawns (all load_data_for_screen, load_detail, execute_primary/toggle/confirmed, session ops, post-action refreshes in main event arms, etc.). Only intentional special cases remain bare (startup auth, inner SSE relay/outer in start_chat_session - these use separate sse_handle abort logic). Bare repeatable spawns: 0. spawn_tracked calls: ~30+. 
- ✅ F4: Added basic tracker test (default/prune) in app.rs; total tests now 6.
- ✅ F5: Added fmt --check, clippy -D warnings, test steps to the build job in .github/workflows/aurago-tui-release.yml (run before the release builds).
- All with rtk cargo verify after edits, clippy clean, tests pass.
- disposable/ used for spawn list.

**Last updated**: 2026-06-03 (Wave A of weitere per user "fange an, eine wave nach der anderen")

## Completed in 2026-06 "Weitere Verbesserungen" Wave C F3 (i18n expansion start)
- ✅ F3: Added 10+ new strings to i18n (loading, various *_title for dashboard tabs, list screens like plans/missions, confirm, history, etc.).
- Wired many into dashboard.rs (all tab titles + loading), chat.rs (history_title), overlays.rs (confirm_title), plans.rs, missions.rs, skills.rs, containers.rs (list titles), config.rs (sections), knowledge.rs (files title).
- Reduced some dead_code potential; added allows for pending list titles (media etc not fully static).
- Imports added where needed.
- rtk clippy/check clean.

## Completed in 2026-06 "Weitere Verbesserungen" Wave B (F2)
- ✅ F2: Chat viewport now scroll-position aware. For auto/bottom: tail (as before). For manual scroll: walks cum lines from app.scroll to find start msg - buffer, builds that window, computes rel offset into the built lines for Paragraph.scroll. Scrollbar still uses full_logical + global scroll for thumb. Preserves all previous UX.
- rtk check/clippy 0, tests 6 pass.
- Updated plan and IMPROVEMENTS.

**Last updated**: 2026-06-03 (Wave B)