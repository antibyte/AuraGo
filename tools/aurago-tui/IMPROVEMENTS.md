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

**Last updated**: 2026-06-04 (sequential Points 1-4 complete: i18n, tests, high-contrast, long-chat prune)

## Wave C / F3 + F6 completed (wave abgeschlossen)
- F3: Significantly expanded i18n with loading, many *_title (dashboard tabs, list screens, details, login, confirm, history, etc.), confirm action strings, nav labels. Wired across dashboard, chat, overlays, plans, missions, skills, containers, knowledge, config, login, media.
- F6: Cache for line counts (perf), debug task count in status.
- check/clippy 0, tests pass.
- Open from original (help text keys, some toasts, full 15-lang) noted for future, but core UI titles/status now i18n-driven.

**Wave closed per user request "wave abschließen"**. All planned F1-F6 from "weitere Verbesserungen" (after 1-4) addressed in waves A/B/C + "rest f1" + "mache weiter" + "offene punkte" continuations. See disposable/wave-abschliessen-final-2026-06-03.txt for exact delivered bullets, 44 spawn_tracked count, 3 documented bare specials (main:121/851/868), GitNexus detect run, rtk verifies. IMPROVEMENTS + GSD session artifacts serve as living docs (no plan.md md file in session dir).

## Completed 2026-06-04 sequential "arbeite die punkte nacheinander ab" - Point 1: i18n polish (remaining F3)
- Wired help section headers (using pre-existing help_* fields).
- Added ~30 new Strings fields for empty states, list headers (with emoji), media tabs/search, overlays smalls (confirm y/cancel/close, session), status hints, screen titles.
- Wired all list draw_*_header (plans/missions/skills/containers/knowledge) + their "No X found" empties.
- Wired media header tabs (active), search indicators (knowledge+media), loading, dynamic empties.
- Wired dashboard status hints + "No scheduled tasks".
- Wired overlays: no_sessions_yet, session_close, confirm y/other/cancel labels.
- Wired app.rs Screen::title() (all nav screens now pull from i18n, import added).
- GitNexus impacts run first on all edited symbols (draw_help, draw_*_header, draw_dash_status, Screen title, Strings; all LOW risk).
- rtk cargo check/clippy -D / test: clean + 6 pass.
- Updated disposable/next-wave-i18n-completion-2026-06-04.txt + this file.
- No behavior change, more raw English eliminated.
- (Help individual shortcut lines + some padding/inactive tabs + "tasks:" debug left for now; help is special-cased per original scaffolding.)

## Completed 2026-06-04 sequential "arbeite die punkte nacheinander ab" - Point 2: more/better tests
- Expanded from 6 to 13 tests.
- New tests in app.rs: apply_sse delta/done/toolcall (content, cache updates, auto_scroll), load_history (filter internal, set cache+scroll), scroll_to_bottom, append cache.
- Expanded keybindings tests: list j/k/enter/delete, nav esc (more KeyContext coverage).
- Focused on audit/IMPROVEMENTS gaps: SSE streaming logic, scroll/auto state + cache (F6), tracker, key matrix.
- GitNexus impacts on apply_sse_event, append_stream_delta, map_key, ChatMessage etc (all LOW).
- Pre-commit gitnexus__detect_changes (low risk, test additions).
- rtk clippy -D 0, cargo test 13 passed.
- Updated disposable/next-wave-tests-point2-*.txt + IMPROVEMENTS.
- Table-driven style where sensible; no logic changes, pure coverage.

## Completed 2026-06-04 sequential "arbeite die punkte nacheinander ab" - Point 3: Theming/Accessibility (high-contrast)
- Added Theme::high_contrast() with max-contrast palette (Black/White bg/fg, bright pure colors for accent/success/warning/error, White border_focus for strong focus indicators).
- Extended by_name("highcontrast"|"hc") and next_name cycle (default->light->midnight->highcontrast->default).
- Refactored from_mood to from_mood_on_base(mood, base) so personality moods (accent overrides) still apply on top of high-contrast base in chat.
- Updated main draw logic: select high_contrast() base when theme_name=hc, then on_base for chat moods, else normal.
- ToggleTheme (Ctrl+T) now seamlessly includes high-contrast (persisted to config).
- All existing UI (chat, dash tabs, lists, overlays, status, gauges, etc.) automatically uses high-vis colors; no per-widget changes needed.
- GitNexus impacts (Theme/by_name/next_name/dispatch_action etc LOW); detect_changes pre (medium, expected for run_app + theme).
- rtk clippy -D 0, cargo test still 13 passed.
- Updated disposable/next-wave-highcontrast-point3-*.txt + IMPROVEMENTS.
- Focus/borders now much more visible in HC mode; color-blind friendly strong contrasts.

## Completed 2026-06-04 sequential "arbeite die punkte nacheinander ab" - Point 4: Long chat perf/memory (bounded history)
- Added prune_old_messages (MAX=400 msgs) to AppState: drains oldest from front on exceed, uses cached_line_count sum for removed_logical to adjust scroll if !auto_scroll (keeps relative view).
- Wired calls after every grow path: push_user_message, start_assistant_stream, tool push in apply_sse_event, load_history (post-set, so recent kept), + direct ChatError push in main.
- Clears (session new, /clear chat etc) unaffected.
- draw_messages / full_logical / viewport walk / scrollbar now operate only on bounded vec (sum O(remaining) but capped; "new messages" hint, auto MAX, streaming deltas, unicode prefix, cum walk all unchanged).
- load_history from server still gets full (for accuracy), UI immediately prunes to last MAX for mem/perf.
- Impacts pre on append_stream_delta, load_history, draw_messages, ChatMessage (LOW); detect pre (medium on AppState/run_app).
- rtk clippy -D 0, cargo test 13 passed (prune not hit in unit tests).
- Updated disposable + IMPROVEMENTS.
- Addresses audit "keeps entire chat_messages in mem" for 1000s msgs sessions; server history untouched. No big widget refactor (kept optimized Paragraph viewport from F2/F6).

## Completed in 2026-06 "Weitere Verbesserungen" more F3 i18n
- More strings: detail_title, edit_field_title, password/otp/login_title, confirm_* actions.
- Wired details in plans/missions/skills/containers/knowledge, login titles, confirm action texts.
- check/clippy clean.

## Completed in 2026-06 "Weitere Verbesserungen" more F6
- Added simple debug task count to dash status (shows " │ tasks:N" when active background tasks).
- rtk clean.

## Completed in 2026-06 "Weitere Verbesserungen" Wave C F6 (polish cache)
- ✅ F6: Added cached_line_count to ChatMessage, updated in push_user/append/load_history/apply_sse (tool), main error push. Switched full_logical in chat.rs to use cache (was recompute lines() every draw). Good for long chat scrollbar perf.
- rtk clean.

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

**Last updated**: 2026-06-03 (Wave B + full wave close via "wave abschließen"; see top close section + disposable/wave-abschliessen-final-*.txt)