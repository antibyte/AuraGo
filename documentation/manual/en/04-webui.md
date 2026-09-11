# Chapter 4: The Web Interface

<p align="center">
  <a href="../images/manual-desktop.webp"><img src="../images/manual-desktop.webp" width="560" alt="AuraGo gopher inside a hand-inked retro desktop with a CRT and tiny windows"></a>
</p>

Chat, Config, Desktop, Missions — separate pages, not a SPA framework. The files come from the **local resource set**, not from `go:embed` of the whole `ui/` tree. Missing set: recovery page.

Most pages need `web_config.enabled: true`. Navigation reloads the page.

[![Real Virtual Desktop](../../screenshots/desktop.png)](../../screenshots/desktop.png)

```
┌────────────────────────────────────────────────────────────┐
│ ⚡ AURA  GO    [Header Buttons]              🌙         ≡  │
├────────────────────────────────────────────────────────────┤
│                                                            │
│                      [Main Area]                           │
│                                                            │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

## The Header

The header is identical on all pages and contains:

### Left: Logo
- **AURA** (in accent color) + **GO** (in default color)
- Click opens the chat (homepage)

### Center: Header Buttons (context-dependent)
Different buttons appear depending on the page:

**In Chat:**
- `New Session` – Reset chat
- `debug` – Debug pill (clickable to toggle)
- `Agent Active` – Status pill

**In Dashboard:**
- Filter buttons for different views

**In Config:**
- `Save` – Save changes
- `Restart` – Restart AuraGo

### Right: Global Controls

| Symbol | Function |
|--------|----------|
| 🌙 / ☀️ | Toggle dark/light theme |
| ≡ | Open radial menu |

## The Radial Menu

The radial menu is the main navigation. It opens as a circular menu from the top-right corner.

```
                    ┌─────────┐
              🥚   │  Chat   │   💬
    Invasion ─────┤   ☰     ├───── Telegram
                  │ Trigger │
              ⚙️   └────┬────┘   📊
    Config ────────────┼────────── Dashboard
                       │
              🚀 ──────┴────── 🚀
            Missions     (more)
```

### Menu Items

| Icon | Name | Description |
|------|------|-------------|
| 💬 | Chat | Main chat interface |
| 📊 | Dashboard | System metrics and analytics |
| 🚀 | Missions | Automated tasks |
| ⚙️ | Config | Edit settings |
| 🥚 | Invasion | Remote deployment |
| 🔓 | Logout | Sign out (if auth enabled) |

### Operation

1. **Click** on ≡ (or anywhere outside to close)
2. **Select** a menu item
3. **The page** switches immediately

> 💡 On mobile devices, you can also swipe from right to left.

## The Chat Interface

The chat is the most frequently used view.

### Layout

```
┌─────────────────────────────────────────────┐
│ Header                                      │
├─────────────────────────────────────────────┤
│                                             │
│ 🤖 Hello! 👋                                │  ← Agent message
│                                             │
│ 🧑 Can you help me with Go?                │  ← Your message
│                                             │
│ 🤖 Of course! What would you like to know? │  ← Agent message
│     🛠️ Tool: web_search                     │
│     📄 Search results...                    │
│                                             │
├─────────────────────────────────────────────┤
│ 📎 [Input field                    ] [➤]  │
└─────────────────────────────────────────────┘
```

### Message Bubbles

**Agent Messages:**
- Light/dark background (depending on theme)
- Left-aligned
- Show tool executions

**Your Messages:**
- Colored background (accent color)
- Right-aligned
- Show attachments/files

### Input Area

| Element | Function |
|---------|----------|
| 📎 | Upload file attachment |
| Text field | Type message |
| ➤ / Enter | Send |

**Keyboard Shortcuts:**
- `Enter` – Send message
- `Shift + Enter` – New line
- `Ctrl + C` – During output: Cancel

### Tool Outputs

When the agent uses tools, they are displayed:

```
🛠️ Tool: execute_shell
   $ ls -la
   
   📁 Output:
   total 128
   drwxr-xr-x  5 user user  4096 ...
```

Click the arrow ▼/▶ to expand/collapse details.

## The Dashboard

The dashboard shows system information and statistics.

### Sections

**1. System Metrics**
- CPU usage
- RAM consumption
- Disk space
- Uptime

**2. Mood History**
- Temporal development of agent mood
- Color-coded (Green = positive, Red = negative)

**3. Prompt Builder Analytics**
- Token consumption per request
- Context compression
- Cost per model

**4. Memory Statistics**
- Size of vector database
- Number of stored facts
- Knowledge graph size

**5. Budget Tracking** (if enabled)
- Today's costs
- Daily limit progress bar
- Model usage

## The Config Interface

Here you edit the `config.yaml` via a web form.

### Structure

```
┌─────────────────────────────────────────────┐
│ Configuration                    [Save]     │
├──────────┬──────────────────────────────────┤
│          │                                  │
│ ▶ Server │  Host: [127.0.0.1   ]           │
│ ▶ LLM    │  Port: [8088        ]           │
│ ▶ Agent  │                                  │
│ ▶ Tools  │  [✓] Enable Web UI             │
│ ...      │                                  │
│          │                                  │
└──────────┴──────────────────────────────────┘
```

### Navigation

**Left Sidebar:**
- Categories expandable (semantic buttons with `aria-expanded`)
- **Search** filters sections by title and description; keyboard navigation with arrows and Enter
- **Unsaved changes** pill in the header when any field is dirty

**Main Area:**
- Form fields per category
- Tooltips on hover over field names
- Real-time validation

### Saving Changes

1. **Change values** in the form fields (the unsaved-changes pill appears when something is dirty).
2. Use the **sticky save bar** at the bottom: **Save**, live status text, and disabled state while a save is in progress.
3. Confirm when prompted if you switch sections with unsaved edits (sidebar click, hash change, or browser Back/Forward).
4. **Restart AuraGo** when the UI or docs indicate it (server port, some integrations).

> ⚠️ **Attention:** Leaving a section without saving discards in-memory edits only after you confirm discard in the modal.

## Virtual Desktop

The Virtual Desktop opens workspace-backed apps in AuraGo's browser desktop. It is designed for file work, coding, media editing, and managed Docker apps without leaving the Web UI. Three virtual **Spaces** (taskbar pager, `Ctrl+Alt+←/→`, Spaces overview via `F3` or `Ctrl+Alt+↑`) group windows per workspace; desktop icons, widgets, and gadgets stay global.

### Included Apps

| App | Purpose |
|-----|---------|
| **Files** | Browse the virtual desktop workspace and open files in the right app |
| **Code Studio** | Container-backed IDE with file tree, editor, search, terminal, and agent context |
| **Terminal** | Workspace terminal with selectable CRT styles (retro tube look) |
| **Writer / Sheets / Notes / Viewer** | Word processing, spreadsheets, Markdown notes (list, live preview, tags), and a file viewer |
| **Pixel** | Image editor for local files, canvas edits, filters, crop/resize, and optional AI generation/enhancement |
| **Zipper** | Browse ZIP archives and extract files into the workspace |
| **Camera / Gallery / Music Player** | Camera capture, gallery, and music playback |
| **Radio / TeeVee / Noisemaker** | Vintage stereo radio receiver, CRT television for live streams (IPTV), and a sound/music generator |
| **Game Maker Studio** | Isolated offline 2D/3D game creation with a sprite library and asset browser |
| **OpenSCAD / 3D Viewer** | Parametric 3D modeling with preview and an STL viewer |
| **Homepage Studio** | Build, validate, and deploy managed website projects |
| **Software Store** | Install and operate managed Docker apps such as Arcane, Termix, code-server, Dozzle, Beszel, and Node-RED |
| **Quick Connect** | Manage SSH/VNC connections |
| **Network Cameras** | View go2rtc camera streams |
| **Virtual Computers** | Operate Boring Computers machines and workspaces |
| **MeshCore** | Desktop messenger for the MeshCore radio (see Chapter 8) |
| **Phone** | SIP softphone (browser phone) |
| **Live Speech / Agent Chat** | Realtime voice window and desktop chat with the agent |
| **Mission Control / Looper** | Automated tasks and iterative agent workflows |
| **Calendar / Todo / People / Pet Picker** | Scheduling, tasks, contacts (KG-enriched), and the desktop pet |
| **Cheater** | Cheat-sheet manager with Markdown and attachments |
| **Calculator / Settings / System Info / Log Viewer** | Calculator, desktop settings, system diagnostics, and a live log tail |
| **Galaxa Deluxe / Chess / NASSCAD / Sysworld** | Arcade shooter, chess (with agent opponent), docking game, and a 3D system visualization |

### Widgets

The widget drawer pins widgets such as system monitor, clock, weather, and chat. The optional **builtin-meshcore** widget is hidden by default and shows the latest MeshCore conversations read-only.

### Software Store Notes

The Software Store uses AuraGo-managed Docker containers. Apps can expose credentials through the Vault, show operation progress, and provide open links for their configured ports. Arcane uses a Docker socket proxy companion for Docker management access. Termix includes a `guacd` companion container for RDP/VNC support and also supports SSH and Telnet management from its own Web UI.

### Sounds

UI sounds for the Virtual Desktop are **opt-in** and **off by default**. Open **Settings → Sound** to enable them, pick one of five synthesized themes (Crystal, Wood, Analog, Workshop, Water), adjust master volume, and toggle categories for windows, notifications, navigation, and files/dialogs. Use **Preview** on a theme card to listen without changing your saved selection. Sounds require a normal user gesture in the browser tab before the first playback; they stay silent during session restore and while the tab is hidden.

### Themes

- **Chat:** 13 themes, including Cyberwar, Retro CRT, Dark Sun, Lollipop, Ocean, Papyrus, 8bit, Black Matrix, Sandstorm, ThreeDee, and the LCARS-inspired **Galaxy**.
- **Virtual Desktop:** **Fruity** (Apple-inspired with dock and topbar) and **Standard** (Windows/Ubuntu-style productivity surface with taskbar).

## Mission Control

Scheduled work and prepared prompts — **not** eggs and nests. Those live under [Invasion Control](12-invasion.md).

Typical pieces: queue, execution, history, and prepared missions.

### Card View

Each mission is shown as a card:

```
┌─────────────────┐
│ Mission Name    │
│ 🟢 Active       │
│                 │
│ Last Run:       │
│ Today, 14:23    │
│                 │
│ [Edit]          │
└─────────────────┘
```

## Looper

Looper is a separate desktop app for a short, goal-driven agent loop. You describe the finished result, the work to do in every round, and how the loop should judge the artifact. An optional finish step can open the file on the desktop.

Each round works on the artifact, then an independent review returns a score. The loop stops when the target score is reached, the round limit is hit, the score stops improving, or you pause or stop it. Pause waits for the current round. The last 20 runs stay in History.

Five built-in examples live under `Documents/Looper/` in the desktop workspace: short story, Python tool, research briefing, project README, and flashcards. Save your own loops next to them.

## Invasion Control

For deploying remote agents.

### Concept

- **Nests** = Target servers (where to deploy)
- **Eggs** = Agent configurations (what to deploy)

### Status Indicators

| Badge | Meaning |
|-------|---------|
| 🟢 Running | Agent is running |
| 🟡 Hatching | Starting up |
| 🔴 Failed | Error occurred |
| ⚪ Idle | Ready but not active |

## Responsive Design

The Web UI adapts to different screen sizes:

### Desktop (> 1024px)
- All features available
- Sidebar visible
- Multi-column layouts

### Tablet (768px - 1024px)
- More compact view
- Some sidebars collapse
- Touch-optimized

### Mobile (< 768px)
- Single-column layout
- Radial menu primary navigation
- Simplified input
- Logo text hidden (icon only)

## Tips & Tricks

### Keyboard Shortcuts

| Shortcut | Function |
|----------|----------|
| `Ctrl + K` | Open quick search |
| `Ctrl + /` | Show keyboard shortcuts |
| `Esc` | Close modal, close menu |
| `Ctrl + Enter` | In chat: Send |

### The Address Bar

- `http://localhost:8088/` – Chat (default)
- `/dashboard` – Dashboard
- `/config` – Configuration
- `/missions` – Mission Control
- `/invasion` – Invasion Control

## Troubleshooting

| Problem | Solution |
|---------|----------|
| Page stays white | Clear browser cache, press F5 |
| Buttons not responding | Check AuraGo process (`ps aux \| grep aurago`) |
| Font too small/large | Browser zoom (Ctrl + +/-) |
| Mobile view broken | Try landscape mode, use browser app |

## Next Steps

- **[Chat Basics](05-chat-basics.md)** – Communicate effectively
- **[Tools](06-tools.md)** – Learn all tools
- **[Configuration](07-configuration.md)** – Fine-tuning
