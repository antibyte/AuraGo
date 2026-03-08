# Kapitel 4: Die Web-Oberfläche

Die Web-Oberfläche ist dein Kontrollzentrum für AuraGo. Dieses Kapitel erklärt alle Elemente und Funktionen.

## Übersicht

Die Web-UI ist als Single-Page Application (SPA) aufgebaut – flüssige Navigation ohne Seiten-Neuladung.

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
- `Reset` – Auf Standard zurücksetzen

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
- Kategorien aufklappbar
- Suchfunktion oben
- Roter Punkt = ungespeicherte Änderungen

**Hauptbereich:**
- Formularfelder je nach Kategorie
- Tooltips bei Hover über Feldnamen
- Validierung in Echtzeit

### Änderungen speichern

1. **Werte ändern** in den Formularfeldern
2. **Auf "Save" klicken** oben rechts
3. **Bestätigung** wartet auf Bestätigung
4. **AuraGo neu starten** (manchmal erforderlich)

> ⚠️ **Achtung:** Einige Änderungen erfordern einen Neustart von AuraGo.

## Mission Control

Oberfläche für automatisierte Aufgaben.

### Tabs

| Tab | Inhalt |
|-----|--------|
| Nester | Verbindungen zu Servern (SSH, Docker, etc.) |
| Eier | Vorlagen für Deployments |

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

## Invasion Control

Für das Deployment von Remote-Agenten.

### Konzept

- **Nester** = Zielserver (wo deployed wird)
- **Eier** = Agent-Konfigurationen (was deployed wird)

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
