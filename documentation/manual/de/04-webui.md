# Kapitel 4: Die Web-Oberfläche

Die Web-Oberfläche ist dein Kontrollzentrum für AuraGo. Dieses Kapitel erklärt alle Elemente und Funktionen.

## Übersicht

Die Web-UI ist eine **mehrseitige eingebettete Anwendung** — jeder Bereich (Chat, Config, Dashboard, Missions, …) hat eine eigene HTML-Seite. Navigation zwischen Bereichen lädt die Seite neu. Die meisten Seiten erfordern `web_config.enabled: true`.

```
┌────────────────────────────────────────────────────────────┐
│ ⚡ AURA  GO    [Header-Buttons]              🌙         ≡  │
├────────────────────────────────────────────────────────────┤
│                                                            │
│                      [Hauptbereich]                        │
│                                                            │
│                                                            │
└────────────────────────────────────────────────────────────┘
```

## Der Header

Der Header ist auf allen Seiten identisch und enthält:

### Links: Logo
- **AURA** (in Akzentfarbe) + **GO** (in Standardfarbe)
- Klick öffnet den Chat (Startseite)

### Mitte: Header-Buttons (kontextabhängig)
Je nach Seite werden verschiedene Buttons angezeigt:

**Im Chat:**
- `New Session` – Chat zurücksetzen
- `debug` – Debug-Pill (klickbar zum Umschalten)
- `Agent Active` – Status-Pill

**Im Dashboard:**
- Filter-Buttons für verschiedene Ansichten

**In Config:**
- `Save` – Änderungen speichern
- `Restart` – AuraGo neu starten

### Rechts: Globale Steuerung

| Symbol | Funktion |
|--------|----------|
| 🌙 / ☀️ | Dark/Light Theme umschalten |
| ≡ | Radial-Menü öffnen |

## Das Radial-Menü

Das Radial-Menü ist die Hauptnavigation. Es öffnet sich als kreisförmiges Menü von der oberen rechten Ecke.

```
                    ┌─────────┐
              🥚   │  Chat   │   💬
    Invasion ─────┤   ☰     ├───── Telegram
                  │ Trigger │
              ⚙️   └────┬────┘   📊
    Config ────────────┼────────── Dashboard
                       │
              🚀 ──────┴────── 🚀
            Missions     (weitere)
```

### Menüpunkte

| Icon | Name | Beschreibung |
|------|------|--------------|
| 💬 | Chat | Haupt-Chat-Oberfläche |
| 📊 | Dashboard | System-Metriken und Analytics |
| 🚀 | Missions | Automatisierte Aufgaben |
| ⚙️ | Config | Konfiguration bearbeiten |
| 🥚 | Invasion | Remote-Deployment |
| 🔓 | Logout | Abmelden (wenn Auth aktiviert) |

### Bedienung

1. **Klicke** auf ≡ (oder irgendwo außerhalb zum Schließen)
2. **Wähle** einen Menüpunkt
3. **Die Seite** wechselt sofort

> 💡 Auf Mobilgeräten kannst du auch von rechts nach links wischen.

## Die Chat-Oberfläche

Der Chat ist die am häufigsten genutzte Ansicht.

### Aufbau

```
┌─────────────────────────────────────────────┐
│ Header                                      │
├─────────────────────────────────────────────┤
│                                             │
│ 🤖 Hallo! 👋                                │  ← Agent-Nachricht
│                                             │
│ 🧑 Kannst du mir bei Go helfen?            │  ← Deine Nachricht
│                                             │
│ 🤖 Natürlich! Was möchtest du wissen?      │  ← Agent-Nachricht
│     🛠️ Tool: web_search                     │
│     📄 Suchergebnisse...                    │
│                                             │
├─────────────────────────────────────────────┤
│ 📎 [Eingabefeld                    ] [➤]  │
└─────────────────────────────────────────────┘
```

### Nachrichten-Bubbles

**Agent-Nachrichten:**
- Heller/dunkler Hintergrund (je nach Theme)
- Links ausgerichtet
- Zeigen Tool-Ausführungen an

**Deine Nachrichten:**
- Farbiger Hintergrund (Akzentfarbe)
- Rechts ausgerichtet
- Zeigen Anhänge/Dateien an

### Eingabebereich

| Element | Funktion |
|---------|----------|
| 📎 | Datei-Anhang hochladen |
| Textfeld | Nachricht eingeben |
| ➤ / Enter | Senden |

**Tastenkürzel:**
- `Enter` – Nachricht senden
- `Shift + Enter` – Neue Zeile
- `Strg + C` – Während der Ausgabe: Abbrechen

### Tool-Ausgaben

Wenn der Agent Tools nutzt, werden diese angezeigt:

```
🛠️ Tool: execute_shell
   $ ls -la
   
   📁 Ausgabe:
   total 128
   drwxr-xr-x  5 user user  4096 ...
```

Klicke auf den Pfeil ▼/▶ um Details ein-/auszuklappen.

## Das Dashboard

Das Dashboard zeigt System-Informationen und Statistiken.

### Bereiche

**1. System-Metriken**
- CPU-Auslastung
- RAM-Verbrauch
- Disk-Platz
- Laufzeit

**2. Mood-Verlauf**
- Zeitliche Entwicklung der Agent-Stimmung
- Farbcodiert (Grün = positiv, Rot = negativ)

**3. Prompt-Builder Analytics**
- Token-Verbrauch pro Anfrage
- Kontext-Kompression
- Kosten pro Modell

**4. Memory-Statistiken**
- Größe der Vektordatenbank
- Anzahl gespeicherter Fakten
- Knowledge-Graph-Größe

**5. Budget-Tracking** (falls aktiviert)
- Heutige Kosten
- Tageslimit-Fortschrittsbalken
- Modell-Nutzung

## Die Config-Oberfläche

Hier bearbeitest du die `config.yaml` über ein Web-Formular.

### Struktur

```
┌─────────────────────────────────────────────┐
│ Konfiguration                    [Save]     │
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

**Linke Sidebar:**
- Kategorien aufklappbar (Buttons mit `aria-expanded`)
- **Suche** filtert Bereiche nach Titel und Beschreibung; Tastatur mit Pfeiltasten und Enter
- **Ungespeicherte Änderungen**-Pill im Header bei dirty State

**Hauptbereich:**
- Formularfelder je nach Kategorie
- Tooltips bei Hover über Feldnamen
- Validierung in Echtzeit

### Änderungen speichern

1. **Werte ändern** in den Formularfeldern (Pill **Ungespeicherte Änderungen** bei dirty State).
2. **Fixe Save-Leiste** unten: **Speichern**, Live-Status, Button deaktiviert während des Speicherns.
3. Bestätigungsdialog beim Bereichswechsel mit ungespeicherten Änderungen (Sidebar, Hash, Browser Zurück/Vor).
4. **AuraGo neu starten**, wenn UI oder Doku es verlangen.

> ⚠️ **Achtung:** Verworfene Änderungen nur nach Bestätigung im Modal.

## Virtual Desktop

Der Virtual Desktop öffnet Workspace-basierte Apps direkt im AuraGo-Browser-Desktop. Er ist für Dateiarbeit, Coding, Medienbearbeitung und verwaltete Docker-Apps gedacht, ohne die Web-UI verlassen zu müssen.

### Enthaltene Apps

| App | Zweck |
|-----|-------|
| **Files** | Virtuellen Desktop-Workspace durchsuchen und Dateien in der passenden App öffnen |
| **Code Studio** | Container-basierte IDE mit Dateibaum, Editor, Suche, Terminal und Agent-Kontext |
| **Pixel** | Bildeditor für lokale Dateien, Canvas-Edits, Filter, Crop/Resize und optionale KI-Generierung/-Verbesserung |
| **Zipper** | ZIP-Archive durchsuchen und Dateien in den Workspace extrahieren |
| **Software Store** | Verwaltete Docker-Apps wie Arcane, Termix, code-server, Dozzle, Beszel und Node-RED installieren und bedienen |

### Hinweise zum Software Store

Der Software Store nutzt vollständig von AuraGo verwaltete Docker-Container. Apps können Zugangsdaten über den Vault bereitstellen, Operationsfortschritt anzeigen und Open-Links für konfigurierte Ports liefern. Arcane nutzt einen Docker-Socket-Proxy-Companion für Docker-Verwaltungszugriff. Termix bringt einen `guacd`-Companion-Container für RDP/VNC mit und unterstützt zusätzlich SSH- und Telnet-Verwaltung über die eigene Web-UI.

## Mission Control

Oberfläche für automatisierte Aufgaben (Cron-ähnliche Ausführung).

### Karten-Ansicht

Jede Mission wird als Karte dargestellt:

```
┌─────────────────┐
│ Mission Name    │
│ 🟢 Aktiv        │
│                 │
│ Letzter Lauf:   │
│ Heute, 14:23    │
│                 │
│ [Bearbeiten]    │
└─────────────────┘
```

### Funktionen

- Missionen erstellen und planen (einmalig oder wiederkehrend)
- Prompt-Vorlagen für automatisierte Abläufe
- Historie der vergangenen Läufe einsehen
- Vorbereitete Missionen (Prepared Missions) als Bibliothek

## Invasion Control

Für das Deployment von Remote-Agenten auf Zielservern.

### Konzept

- **Nester** = Zielserver (wo der Remote-Agent deployed wird)
- **Eier** = Agent-Konfigurationen (was genau deployed wird)

### Status-Anzeigen

| Badge | Bedeutung |
|-------|-----------|
| 🟢 Running | Agent läuft |
| 🟡 Hatching | Wird gestartet |
| 🔴 Failed | Fehler aufgetreten |
| ⚪ Idle | Bereit, aber nicht aktiv |

## Responsive Design

Die Web-UI passt sich an verschiedene Bildschirmgrößen an:

### Desktop (> 1024px)
- Alle Features verfügbar
- Sidebar sichtbar
- Mehrspaltige Layouts

### Tablet (768px - 1024px)
- Kompaktere Ansicht
- Einige Sidebars kollabieren
- Touch-optimiert

### Mobile (< 768px)
- Einspaltiges Layout
- Radial-Menü primäre Navigation
- Vereinfachte Eingabe
- Logo-Text ausgeblendet (nur Icon)

## Tipps & Tricks

### Tastenkürzel

| Kürzel | Funktion |
|--------|----------|
| `Strg + K` | Schnell-Suche öffnen |
| `Strg + /` | Tastenkürzel anzeigen |
| `Esc` | Modal schließen, Menü schließen |
| `Strg + Enter` | Im Chat: Senden |

### Die URL-Leiste

- `http://localhost:8088/` – Chat (Standard)
- `/dashboard` – Dashboard
- `/config` – Konfiguration
- `/missions` – Mission Control
- `/invasion` – Invasion Control

## Fehlerbehebung

| Problem | Lösung |
|---------|--------|
| Seite bleibt weiß | Browser-Cache leeren, F5 drücken |
| Buttons reagieren nicht | AuraGo-Prozess prüfen (`ps aux \| grep aurago`) |
| Schrift zu klein/groß | Browser-Zoom (Strg + +/-) |
| Mobile Ansicht komisch | Querformat probieren, Browser-App nutzen |

## Nächste Schritte

- **[Chat-Grundlagen](05-chatgrundlagen.md)** – Effektiv kommunizieren
- **[Werkzeuge](06-tools.md)** – Alle Tools kennenlernen
- **[Konfiguration](07-konfiguration.md)** – Feintuning
