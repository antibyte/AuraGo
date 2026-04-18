# Chat Theme Plan: Dark Sun

## Zielbild

Der Chat bekommt ein neues Premium-Theme `dark-sun` mit maximaler Dramatik:

- extrem dunkles Fundament mit roter, orangefarbener und goldener Hitzeenergie
- matte Oberflächen statt Glas-Look
- ausgeprägter 3D-Eindruck durch Tiefe, Kantenlicht und Relief
- starke, absichtlich überzeichnete Animationen
- faszinierender WebGL-Layer als atmosphärisches Solar-/Plasma-Enhancement

Das Theme soll wie die Hochzeit aus brutalem Minimalismus und opernhafter Übertreibung wirken: wenige Formen, wenig UI-Rauschen, aber monumentale Präsenz, Hitze, Gravitation und Licht.

## Ausgangslage im Code

Die bestehende Chat-Architektur unterstützt bereits benannte Themes:

- Theme-State in [ui/shared.js](ui/shared.js)
- Theme-Picker in [ui/index.html](ui/index.html) und [ui/js/chat/main.js](ui/js/chat/main.js)
- Theme-sensitive Mermaid-Renderer in [ui/js/chat/mermaid-renderer.js](ui/js/chat/mermaid-renderer.js) und [ui/js/chat/modules/mermaid-loader.js](ui/js/chat/modules/mermaid-loader.js)
- Spezialtheme-Layer in [ui/css/chat-cyberwar.css](ui/css/chat-cyberwar.css) und [ui/css/chat-lollipop.css](ui/css/chat-lollipop.css)
- WebGL-Enhancement-Vorbild in [ui/js/chat/cyberwar-shader.js](ui/js/chat/cyberwar-shader.js)

Dark Sun sollte dieselbe Struktur nutzen, statt ein separates Theme-System einzuführen.

## Scope

- Nur die Chat-Seite ist in Scope.
- Bestehende Funktionen müssen visuell und funktional intakt bleiben.
- Das Theme wird zusätzlich zu `dark`, `light`, `retro-crt`, `cyberwar` und `lollipop` eingeführt.
- Theme-Wechsel muss live ohne Reload funktionieren.
- WebGL ist optionales Enhancement, nie Voraussetzung für Kernoptik oder Lesbarkeit.

## Design-Richtung

### Leitidee

Nicht "rote Neon-Cyberpunk-App", sondern:

- dunkler Sternenkern
- heiße Sonnenkorona
- verbrannte Mineraloberflächen
- monumentale, matte Kontrollpaneele
- dramatische Lichtkanten und Energieflüsse

### Stilreferenz

Mischung aus:

- solarer Finsternis
- vulkanischer Sci-Fi-Architektur
- minimalistischen Monolith-Panels
- opernhafter Lichtdramaturgie

### Design-Prinzipien

1. Wenige Formen, große Wirkung.
2. Matte Flächen, nicht glossy.
3. Räumlichkeit durch Tiefe, nicht durch Dekor.
4. Effekte sollen staunen auslösen, aber die Oberfläche nicht unbenutzbar machen.
5. Text- und Bubble-Lesbarkeit bleibt wichtiger als Showeffekte.

## Theme-Modell

Neuer Theme-Key:

- `dark-sun`

Erweiterungen:

- `CHAT_THEMES` in [ui/shared.js](ui/shared.js)
- Picker in [ui/index.html](ui/index.html)
- Theme-Icon in [ui/js/chat/main.js](ui/js/chat/main.js)
- neues i18n-Key `chat.theme_dark_sun` in allen Chat-Sprachdateien
- dark highlight.js base bleibt aktiv

Vorgeschlagenes Picker-Icon:

- `☼`

## Visuelle Architektur

### 1. Fundament

Basisfarben:

- fast schwarzes Braun / Charcoal
- tiefer Ascheschwarzton
- Ember Red
- Burnt Orange
- Solar Gold

Hintergrundwirkung:

- dunkler Raum mit schwacher Korona-Atmosphäre
- Hitzeinseln statt bunter Flächen
- schwere visuelle Gravitation

### 2. Oberflächen

Flächen sollen matt, schwer und fast mineralisch wirken:

- dunkle Platten
- minimal raue Lichtbrechung
- starke innere Schatten
- feine warme Kantenlichter
- 3D-Stufen statt Glanzspiegelung

### 3. 3D-Look

Die Tiefe ist zentrales Stilmittel:

- inset shadows
- raised rims
- edge bevels
- layer stacking
- warme Seitenlichter
- Fokuszustände mit glühender Tiefenaufladung

### 4. Farbdramaturgie

Rollen:

- Primär: Ember Red
- Sekundär: Burnt Orange
- Tertiär: Solar Gold
- Warnung/Peak Energy: helles Lava-Orange

Prinzip:

- Flächen bleiben fast schwarz
- Licht kommt nur selektiv und mit Wucht
- Highlights wirken wie Hitze, nicht wie Neon

## Komponenten-Plan

### Header

- massives dunkles Instrumentpanel
- sehr dezente Grundform, aber dramatisches warmes Lichtband
- Controls als matte Hardware-Schalter
- Theme-Picker wirkt wie Solar-Modeschalter

### Chat-Fläche

- dunkler Solarraum mit langsamer Korona-Atmosphäre
- kaum Muster, aber viel Tiefenwirkung
- leichte Wärmeschlieren und Staub-/Glutpartikel-Anmutung

### Message-Bubbles

Bot:

- schwere dunkle Panels
- warmes Randlicht
- starker Relief-Look

User:

- heißere rot/orange Flächen
- intensivere Energie
- trotzdem nicht glossy

Technische Ausgaben:

- obsidianartige Terminal-Paneele
- warme Syntax-/Randakzente
- kontrollierte Hitzespuren

### Composer

- eingelassenes Hauptkontrollpult
- sehr dunkle matte Grundfläche
- Fokuszustand wie aufgeladene Sonnenenergie
- Send-Button als stärkster Wärme-Hotspot

### Session Drawer / Modals

- monolithische dunkle Karten
- massive Tiefe
- warme Kantenlinien
- Licht nur an Kanten, Headern und Fokusflächen

## Motion-System

### Grundsatz

Minimalistische Komposition, maximal dramatische Bewegung.

Motion-Kategorien:

1. ambient solar motion
2. focus heat motion
3. state ignition motion
4. message materialization

### Ambient motion

- langsame Korona-Atmung
- tiefe Glow-Wellen
- minimale Ember-Drifts
- Heat-haze-artige Hintergrundverformung

### Focus motion

- Kanten glühen auf
- Panels heben sich räumlich stärker heraus
- warme Lichtpulse im Fokus

### State motion

- aktive Pills pulsieren mit kontrollierter Sonnenenergie
- Warnungen brennen wärmer statt bloß heller

### Message motion

- neue Nachrichten materialisieren mit kurzer Wärmeverdichtung
- keine hektischen Einblendungen
- stattdessen schweres, dramatisches Auftauchen

## WebGL-Strategie

### Ziel

WebGL soll Staunen erzeugen, nicht Layout übernehmen.

Rolle:

- eigener Background-/Overlay-Layer
- nur additive Atmosphäreneffekte
- DOM bleibt voll lesbar und interaktiv

### Geeignete Effekte

1. Sonnenkorona-/Plasma-Strömungen
2. langsame dunkle Solarwirbel
3. Heat-haze-Refraktion
4. Glutpartikel mit geringer Dichte
5. dunkle Flecken / magnetische Turbulenz
6. Flare-artige Wärmewellen

### Technische Anforderungen

- neues Skript `ui/js/chat/dark-sun-shader.js`
- Theme-gesteuert über `data-theme="dark-sun"`
- `pointer-events: none`
- defensiver `z-index`, damit keine Bubbles verdeckt werden
- Degradation bei `prefers-reduced-motion`, Mobile und fehlendem WebGL

## Renderer- und Modulverträglichkeit

- Mermaid bekommt eigenen `dark-sun`-Branch
- dark highlight.js base bleibt aktiv, Farbanpassung per CSS
- Drawer, Warnings, Lightbox, Modals und Composer müssen denselben Tiefenstil nutzen

## Accessibility und Performance

- `prefers-reduced-motion` respektieren
- Shader nur als Enhancement
- Text, Fokuszustände und Bubble-Kontrast dürfen nie unter Solar-Effekten leiden
- Mobile-Version reduziert Ambient-Effekte deutlich

## Implementierungsphasen

### Phase 1

- Plan-Datei anlegen
- `dark-sun` als Theme-Key ergänzen
- Picker + Icon + Übersetzungen erweitern

### Phase 2

- neues CSS-Layer `chat-dark-sun.css`
- Tokens, Panels, Bubbles, Header, Composer, Drawer, Modals

### Phase 3

- Mermaid + code highlighting anpassen

### Phase 4

- WebGL-Shader als optionales Enhancement
- defensive Stacking- und Reduced-Motion-Absicherung

### Phase 5

- Smoke-Checks, Diff-Check, JSON-/JS-Validierung

## Akzeptanzkriterien

- `dark-sun` funktioniert live ohne Reload
- Theme unterscheidet sich klar von `dark` und `cyberwar`
- Oberflächen wirken matt, tief und räumlich
- Rot-/Orange-Akzente dominieren ohne Lesbarkeitsverlust
- WebGL erzeugt ein deutliches Wow-Gefühl, ohne Chat-Inhalte zu verdecken
- Mermaid, Code und Modals fügen sich sichtbar ins Theme ein
- Reduced-Motion und No-WebGL degradieren sauber

## Kurzfazit

Dark Sun soll der dramatischste, schwerste und visuell mächtigste Chat-Look werden:

- dunkel
- heiß
- monumental
- minimalistisch in der Form
- maximal überzeichnet in Energie, Bewegung und Atmosphäre
