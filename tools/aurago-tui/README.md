# aurago-tui

A modern, fancy Terminal UI (TUI) chat client for **AuraGo**.

## Features

- 🎨 **Fancy graphics** – rainbow splash screen, animated glow borders, wave header, mood-based themes
- 🔒 **Native Auth** – logs in with your AuraGo password (+ TOTP/OTP if enabled)
- ⚡ **Live streaming** – SSE-powered real-time message streaming, thinking blocks, tool-call previews
- 🖥️ **Small binaries** – written in Rust with size-optimized release profile

## Build

Requires [Rust](https://rustup.rs/) 1.85+.

```bash
cd tools/aurago-tui
cargo build --release
```

The resulting binary will be at `target/release/aurago-tui` (or `.exe` on Windows).

### Windows Build Note

On Windows you need either:
- **Visual Studio Build Tools** (MSVC toolchain), or
- **MinGW-w64** (GNU toolchain)

installed so that Rust can link native binaries.

(Note: A previous experimental `zigcc.bat` + full Zig toolchain was removed in the 2026-05-18 audit as it was unused and bloated the tree by 443 MB.)

## Usage

```bash
./aurago-tui --url http://localhost:8080
```

The interface defaults to English. Select any supported language with `--lang`
or set `AURAGO_TUI_LANG`; `--lang` takes precedence. The catalogs localize the
100 shared labels and headings, navigation, empty states, and protection dialogs.
Other hardcoded TUI text and server-provided messages may still appear in English.

```bash
./aurago-tui --lang de
AURAGO_TUI_LANG=ja ./aurago-tui
```

Supported codes: `cs`, `da`, `de`, `el`, `en`, `es`, `fr`, `hi`, `it`, `ja`,
`nl`, `no`, `pl`, `pt`, `sv`, and `zh`.

### Keybindings

| Key | Action |
|-----|--------|
| `Enter` | Send message / Login |
| `Shift+Enter` | New line in chat input |
| `Tab` | Switch focus (Password ↔ OTP in login, Chat ↔ Sidebar in chat) |
| `↑ / ↓` | Scroll chat history |
| `Ctrl+L` | Clear chat history |
| `Ctrl+O` | Logout |
| `Ctrl+R` | Scroll to latest message |
| `Ctrl+T` | Toggle theme (debug) |
| `?` / `Esc` | Toggle help overlay |
| `Ctrl+C` / `q` | Quit |

## Architecture

```
src/
├── main.rs          # Entry point, terminal setup, async event loop
├── app.rs           # Central application state
├── config.rs        # Config & session persistence
├── api/
│   ├── mod.rs       # HTTP client wrapper (reqwest + cookies)
│   ├── auth.rs      # Login / logout / history helpers
│   ├── sse.rs       # SSE stream parser
│   └── types.rs     # API DTOs
├── ui/
│   ├── mod.rs       # UI dispatcher
│   ├── login.rs     # Login form with password + OTP
│   ├── chat.rs      # Main chat layout
│   ├── dashboard.rs # System, budget, logs, and activity dashboards
│   ├── plans.rs     # Plan list and detail views
│   ├── missions.rs  # Mission list and detail views
│   ├── skills.rs    # Skill management view
│   ├── containers.rs# Container management view
│   ├── config.rs    # Configuration editor
│   ├── knowledge.rs # Knowledge file browser
│   ├── media.rs     # Media browser
│   ├── splash.rs    # Rainbow ASCII splash
│   ├── theme.rs     # Color schemes & mood theming
│   └── utils.rs     # Shared UI helpers
└── events/
    ├── mod.rs       # Event types
    └── keybindings.rs # Key → Action mapping
```

## License

Same as AuraGo.
