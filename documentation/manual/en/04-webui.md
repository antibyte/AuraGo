# Chapter 4: The Web Interface

The web interface is your control center for AuraGo. This chapter explains all elements and functions.

## Overview

The Web UI is built as a Single-Page Application (SPA) – fluid navigation without page reloads.

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
- `Reset` – Reset to defaults

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
- Categories expandable
- Search function at top
- Red dot = unsaved changes

**Main Area:**
- Form fields per category
- Tooltips on hover over field names
- Real-time validation

### Saving Changes

1. **Change values** in the form fields
2. **Click "Save"** at top right
3. **Confirmation** awaits confirmation
4. **Restart AuraGo** (sometimes required)

> ⚠️ **Attention:** Some changes require a restart of AuraGo.

## Mission Control

Interface for automated tasks.

### Tabs

| Tab | Content |
|-----|---------|
| Nests | Connections to servers (SSH, Docker, etc.) |
| Eggs | Templates for deployments |

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
