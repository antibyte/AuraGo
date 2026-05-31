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