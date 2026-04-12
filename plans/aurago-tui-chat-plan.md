# Plan: AuraGo TUI Chat Client

> **Ziel:** Ein kleines, standalone CLI-Tool für AuraGo, das einen modernen, grafisch aufwendigen Chat im Terminal ermöglicht. Da Go sehr große Binaries erzeugt, wird das Tool in **Rust** entwickelt, das mit entsprechenden Build-Flags extrem kleine, native Binaries produzieren kann.

---

## 1. Technologie-Stack

| Komponente | Technologie | Begründung |
|------------|-------------|------------|
| **Sprache** | Rust | Kleine Binaries möglich (`strip`, `lto`, `panic=abort`, UPX), exzellentes TUI-Ökosystem |
| **TUI-Framework** | `ratatui` | De-facto-Standard für moderne Terminal-UIs in Rust, flexibel, performant, großes Widget-Ökosystem |
| **Terminal-I/O** | `crossterm` | Plattformübergreifend (Windows, Linux, macOS), unterstützt Maus, Tastatur, Resizing |
| **Async Runtime** | `tokio` | Für parallele HTTP-Requests und SSE-Stream-Verarbeitung |
| **HTTP Client** | `reqwest` + `eventsource-stream` | Einfacher REST-Client + native SSE-Unterstützung |
| **Serialisierung** | `serde` + `serde_json` | JSON-Verarbeitung für API und SSE |
| **CLI Args** | `clap` | Moderne, typsichere Kommandozeilenargumente |
| **Fancy UI Extras** | `tui-big-text`, `tui-logger`, `tui-textarea`, `tui-image` (optional) | Große ASCII-Texte, Inline-Bilder (iTerm/Sixel), bessere Logs |
| **Animationen** | Eigene Frame-Loop in `ratatui` | Partikel-System, Wellen-Effekte, Typing-Indikatoren, Farbverlaufs-Animationen |
| **TOTP/OTP** | `totp-rs` (lightweight) | OTP-Code-Validierung vor dem Login (nur lokale Generierung/Anzeige, Server prüft) |
| **Konfiguration** | `directories` + `toml` | Standard-Config-Dir für das OS |

### Binary-Size Optimierung (Ziel: < 3 MB)
```toml
[profile.release]
opt-level = "z"      # Size optimization
lto = true           # Link-time optimization
codegen-units = 1    # Slower compile, smaller binary
strip = true         # Remove debug symbols
panic = "abort"      # No unwinding overhead
```
Optional: `upx --best` nach dem Build für weitere ~40% Reduktion.

---

## 2. Projektstruktur

```
tools/aurago-tui/
├── Cargo.toml
├── README.md
├── src/
│   ├── main.rs              # CLI-Entrypoint, Config-Loading
│   ├── app.rs               # Zentraler App-State (Model)
│   ├── ui/
│   │   ├── mod.rs           # UI-Routing (welcher Screen ist aktiv)
│   │   ├── login.rs         # Passwort + OTP Login-Formular
│   │   ├── chat.rs          # Haupt-Chat-Layout
│   │   ├── splash.rs        # Startup-Animation / ASCII-Art Intro
│   │   ├── sidebar.rs       # Konversations-Liste
│   │   ├── input_bar.rs     # Eingabezeile mit Fancy-Border
│   │   ├── status_bar.rs    # Verbindung, Token-Count, Mood
│   │   ├── animations.rs    # Partikel, Wellen, Spinner-Systeme
│   │   └── theme.rs         # Farbschema, Stile
│   ├── api/
│   │   ├── mod.rs           # HTTP-Client-Wrapper
│   │   ├── auth.rs          # Login, Logout, Auth-Status, Cookie-Jar
│   │   ├── sse.rs           # SSE-Event-Stream Parser
│   │   └── types.rs         # API-DTOs (Message, Delta, Event etc.)
│   ├── events/
│   │   ├── mod.rs           # Event-Loop (Tastatur + API-Events)
│   │   └── keybindings.rs   # Tastenkürzel-Map
│   └── config.rs            # URL, Theme, Session-Cookie-Persistenz
```

---

## 3. Kommunikation mit AuraGo API

### 3.1 Authentifizierung
Das TUI-Tool authentifiziert sich **regulär über das AuraGo-Login-System**:
1. **Auth-Status prüfen:** Beim Start wird `/api/auth/status` abgefragt.
2. **Passwort-Login:** Wenn Auth aktiv ist, zeigt das TUI einen Login-Screen mit Passwort-Eingabe.
3. **TOTP/OTP-Token:** Wenn der Server signalisiert, dass TOTP erforderlich ist, erscheint ein weiteres Eingabefeld für den 6-stelligen OTP-Code.
4. **Session-Cookie:** Nach erfolgreichem Login wird das Session-Cookie gespeichert (im RAM + optional in der Config für Wiederverwendung) und für alle folgenden Requests verwendet.
5. **Logout:** `Ctrl + O` führt einen Logout über `/api/auth/logout` durch und löscht das lokale Cookie.

### 3.2 Endpunkte

| Methode | Endpoint | Zweck |
|---------|----------|-------|
| `GET` | `/api/auth/status` | Prüft ob Auth aktiv ist und ob TOTP eingerichtet |
| `POST` | `/api/auth/login` | Passwort (+ optional OTP) senden, Session-Cookie erhalten |
| `POST` | `/api/auth/logout` | Session beenden |
| `GET` | `/api/health` | Verbindungs-Check |
| `GET` | `/history` | Chat-Verlauf laden |
| `DELETE` | `/clear` | Verlauf löschen |
| `POST` | `/v1/chat/completions` | Nachricht senden, Agent wecken |
| `GET` | `/events` | **SSE-Stream** für Live-Updates |

### 3.3 SSE-Event-Handling
Das TUI subscribed auf `/events` und parst folgende Event-Typen:

| SSE Event | Visualisierung im TUI |
|-----------|----------------------|
| `llm_stream_delta` | Streamender Text erscheint Char-by-Char im Chat |
| `llm_stream_done` | Eingabefeld wird wieder freigegeben, Cursor stoppt |
| `thinking_block` | Ein- / Ausklappbare "Thinking..."-Box mit animiertem Rahmen |
| `tool_call_preview` | Fancy Toast / Panel mit Tool-Name und JSON-Preview |
| `token_update` | Live-Anzeige der verbrauchten Tokens in der Status-Bar |
| `agent_status` | Mood-Icon und Status-Text (z. B. "arbeitet...", "idle") |
| `personality_update` | Hintergrundfarbe oder Rahmen ändert sich leicht je nach Mood |
| `system_warning` | Modal-Overlay mit Warning-Icon |
| `toast` | Kurzlebige Banner-Animation oben im Terminal |

---

## 4. UI-Konzept & Grafik-Spielereien

### 4.1 Login-View (bei aktiviertem Auth)
```
┌─────────────────────────────────────────────────────────────────────┐
│        ╔═══════════════════════════════════════════════════╗        │
│        ║           🔐  AuraGo Terminal Chat               ║        │
│        ╠═══════════════════════════════════════════════════╣        │
│        ║  Server: http://localhost:8080                    ║        │
│        ║                                                   ║        │
│        ║  Passwort: [****************************      ]   ║        │
│        ║  OTP Code: [123456    ] (nur wenn TOTP aktiv)   ║        │
│        ║                                                   ║        │
│        ║          [ 🔓  Anmelden ]                       ║        │
│        ╚═══════════════════════════════════════════════════╝        │
│                                                                     │
│                     [animierter Spinner bei Login]                  │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.2 Layout (Standard-View nach Login)
```
┌─────────────────────────────────────────────────────────────────────┐
│  [Splash-Header / großer ASCII-AuraGo-Text]         🌙 Mood: Curious│
├──────────┬──────────────────────────────────────────────────────────┤
│          │  ┌────────────────────────────────────────────────────┐  │
│  📜      │  │  🧑 Du: Wie ist das Wetter?                        │  │
│  History │  │                                                    │  │
│          │  │  🤖 Aura: Das Wetter ist heute sonnig und 20°C.    │  │
│  Chat 1  │  │       [streamender Text mit Glow-Effekt]           │  │
│  Chat 2  │  │                                                    │  │
│  ...     │  │  💭 Thinking...  [animierte Box]                   │  │
│          │  │                                                    │  │
│          │  │  🔧 Tool: web_search {"q":"Wetter Berlin"}         │  │
│          │  └────────────────────────────────────────────────────┘  │
│          │                                                          │
├──────────┴──────────────────────────────────────────────────────────┤
│  > Eingabe hier...                                    [Senden]      │
├─────────────────────────────────────────────────────────────────────┤
│  ⚡ Connected │ Tokens: 1.240 │ Model: gemini-2.0-flash │ ⏱ 12:34  │
└─────────────────────────────────────────────────────────────────────┘
```

### 4.2 Fancy-Grafik-Features

| Feature | Technische Umsetzung |
|---------|---------------------|
| **ASCII-Splash-Intro** | `tui-big-text` oder eigener ASCII-Renderer mit Farbverlauf. Beim Start 1-2 Sekunden animiertes Einblenden. |
| **Typing-Particle-Effekt** | Während der Agent antwortet, schweben kleine "✦"-Partikel neben der Nachricht aufwärts. Eigener `Widget`, der einen `Vec<Particle>` rendert. |
| **Animierte Border-Glow** | Der Fokus-Rahmen des Eingabefeldes pulsiert in HSB-Farben (Regenbogen-Shift über die Zeit). |
| **Wave-Header** | Der obere Header zeigt eine Sinus-Welle als Hintergrund-ASCII (`~ ~ ~`) die langsam nach rechts wandert. |
| **Mood-basierte Farbthemen** | Personality-Update ändert das Terminal-Theme (z. B. `Fröhlich` = warmes Gelb/Orange, `Nachdenklich` = kühles Blau/Lila). |
| **Thinking-Block Animation** | Eine kollabierbare Box mit einem laufenden `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`-Spinner und einem sich langsam füllenden Fortschrittsbalken. |
| **Markdown-lite Rendering** | Inline-Code wird in einem kontrastreichen Block gerendert, URLs werden unterstrichen. |
| **Bild-Vorschau (Optional)** | Wenn das Terminal `sixel` oder `iTerm` inline images unterstützt, können generierte Bilder direkt im Chat angezeigt werden (`tui-image`). |
| **Sound-Visualisierung (Fake)** | Bei Voice-Input könnte eine sich wellenförmig bewegende Linie angezeigt werden (nur visuell). |

---

## 5. Interaktion & Tastenkürzel

| Taste | Aktion |
|-------|--------|
| `Enter` | Nachricht absenden |
| `Shift + Enter` | Neue Zeile in der Eingabe |
| `↑ / ↓` | Durch Chat-Verlauf scrollen |
| `Ctrl + C` oder `q` | Beenden (mit Bestätigung falls Nachricht im Flug) |
| `Ctrl + L` | Chat-Verlauf löschen (`/clear`) |
| `Ctrl + O` | Logout (Session beenden) |
| `Ctrl + R` | An Chat-Verlauf scrollen / zurück zum aktuellen |
| `Ctrl + T` | Theme manuell wechseln (Debug/Spaß) |
| `Tab` | Zwischen Chat-Bereich und Sidebar wechseln |
| `Esc` | Modal/Overlay schließen |

---

## 6. Build & Deployment

### 6.1 Lokale Entwicklung
```bash
cd tools/aurago-tui
cargo run -- --url http://localhost:8080
# Tool fragt bei Bedarf interaktiv nach Passwort & OTP
```

### 6.2 Release-Build (kleine Binary)
```bash
cargo build --release
strip target/release/aurago-tui
# Optional:
upx --best target/release/aurago-tui
```

### 6.3 CI/CD Integration
Da es sich um ein separates Tool handelt, kann es entweder:
- Im selben Repo unter `tools/aurago-tui/` versioniert werden
- Eine eigene GitHub-Release-Pipeline bekommen, die Cross-Compiles für Linux, macOS und Windows erstellt

---

## 7. Phasen / Umsetzungs-Reihenfolge

1. **Phase 1: Skelett, Auth & API**
   - Projekt-Setup (`cargo init`), Dependencies
   - HTTP-Client mit Cookie-Jar (`reqwest` features)
   - Auth-Flow implementieren: `/api/auth/status`, `/api/auth/login`, `/api/auth/logout`
   - Session-Cookie persistieren (`~/.config/aurago-tui/session.toml`)
   - SSE-Client für `/events` mit rudimentärem Parsing

2. **Phase 2: Grundlegende TUI (Login + Chat)**
   - `ratatui` + `crossterm` Event-Loop
   - Login-Screen mit Passwort-Maske und OTP-Eingabe
   - Chat-Layout: Sidebar, Chat-Bereich, Input-Bar, Status-Bar
   - Chat-Verlauf anzeigen und scrollen
   - Nachrichten senden

3. **Phase 3: Streaming & State**
   - `llm_stream_delta` in Echtzeit rendern
   - `llm_stream_done` behandeln
   - `thinking_block` anzeigen
   - `token_update` in Status-Bar

4. **Phase 4: Fancy-Grafik & Animationen**
   - Splash-Screen
   - Partikel-System für aktive Antworten
   - Animierter Glow-Rahmen
   - Mood-Theming
   - Wave-Header

5. **Phase 5: Polishing**
   - Config-File (`~/.config/aurago-tui/config.toml`)
   - Auto-Login mit gespeichertem Cookie
   - Fehler-Handling (Reconnect, Auth-Fehler, Session abgelaufen → zurück zu Login)
   - Keybindings-Help-Overlay
   - README mit Installationsanleitung

---

## 8. Risiken & Hinweise

- **Terminal-Feature-Support:** Nicht alle Terminals unterstützen 24-Bit-Farben, Maus oder Sixel-Bilder. Das Tool muss mit `crossterm::terminal::supports_truecolor` o. ä. abfragen und auf 256-Farben oder gar 16-Farben zurückfallen.
- **Windows:** `crossterm` deckt die meisten Windows-Terminals ab, aber `Conhost` (alte cmd.exe) kann bei komplexen Layouts ruckeln. Empfohlen: Windows Terminal.
- **Binary-Size:** Auch Rust-Binaries können groß werden, wenn zu viele Dependencies reinkommen. Exotische Crates sparsam einsetzen und Features (`--no-default-features`) nutzen.

---

## 9. Zusammenfassung

Das `aurago-tui` wird ein **visuell beeindruckendes, kleines Binary** in Rust, das den vollen AuraGo-Chat inklusive Streaming, Tool-Call-Vorschau, Mood-Updates und Token-Tracking ins Terminal bringt. Es nutzt die bestehende REST- und SSE-API von AuraGo, integriert sich nativ in das vorhandene Session-Auth-System mit Passwort und OTP, und kommt ohne Änderungen am Backend aus.
