# aurago-tui

A modern, fancy Terminal UI (TUI) chat client for **AuraGo**.

## Features

- 🎨 **Fancy graphics** – rainbow splash screen, animated glow borders, wave header, mood-based themes
- 🔒 **Native Auth** – logs in with your AuraGo password (+ TOTP/OTP if enabled)
- ⚡ **Live streaming** – SSE-powered real-time message streaming, thinking blocks, tool-call previews
- 🖥️ **Small binaries** – written in Rust with size-optimized release profile

## Build

Requires [Rust](https://rustup.rs/) 1.70+.

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

## Usage

```bash
./aurago-tui --url http://localhost:8080
```

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
│   ├── splash.rs    # Rainbow ASCII splash
│   ├── theme.rs     # Color schemes & mood theming
│   └── animations.rs# Animation helpers
└── events/
    ├── mod.rs       # Event types
    └── keybindings.rs # Key → Action mapping
```

## License

Same as AuraGo.
