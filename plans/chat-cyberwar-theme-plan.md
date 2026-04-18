# Chat Theme Plan: Cyberwar

## Zielbild

Der Chat bekommt ein neues Premium-Theme `cyberwar` mit maximaler Stilwirkung:

- visueller Crossover aus Cyberpunk und Star Trek UI
- glossy, hochpolierte Oberflächen statt flacher Panels
- starke Kontraste zwischen sehr dunklem Fundament und extrem bunten Neon-Akzenten
- dichte, aber kontrollierte Motion
- optionaler WebGL-Screen-Layer für atmosphärische High-End-Effekte

Das Ergebnis soll nicht retro, schmutzig oder kaputt wirken, sondern wie ein luxuriöses taktisches Interface aus einer futuristischen Kommandozentrale unter Cyberangriff.

## Ausgangslage im Code

Die Chat-UI bringt bereits die nötigen Einstiegspunkte mit:

- Theme-State in [ui/shared.js](ui/shared.js)
- Theme-Picker in [ui/index.html](ui/index.html) und [ui/js/chat/main.js](ui/js/chat/main.js)
- Chat-Hauptstyling in [ui/css/chat.css](ui/css/chat.css)
- generische Tokens in [ui/css/tokens.css](ui/css/tokens.css)
- theme-sensitive Renderer in [ui/js/chat/mermaid-renderer.js](ui/js/chat/mermaid-renderer.js) und [ui/js/chat/modules/mermaid-loader.js](ui/js/chat/modules/mermaid-loader.js)

Wichtig:

- Es existiert bereits ein `retro-crt`-Theme.
- Es gibt außerdem eine ungetrackte Datei `ui/js/crt-shader.js`, die wie ein experimenteller WebGL-Overlay-Prototyp aussieht.

Der Cyberwar-Plan sollte deshalb nicht am Theme-System vorbei arbeiten, sondern es gezielt erweitern.

## Scope

- Nur die Chat-Seite ist in Scope.
- Bestehende Chat-Funktionen müssen visuell und funktional intakt bleiben.
- Das neue Theme wird zusätzlich zu `dark`, `light` und `retro-crt` eingeführt.
- Theme-Wechsel muss live ohne Reload funktionieren.

## Design-Richtung

### Leitidee

Nicht "Hacker-Terminal", sondern "militärisch-futuristische taktische Konsole".

Das Theme soll diese Spannungen kombinieren:

- tiefes Schwarz und dunkle Navy-Flächen
- extrem leuchtende Cyan-, Magenta-, Electric-Blue-, Acid-Lime- und Amber-Akzente
- hochglänzende Glas- und Lack-Oberflächen
- segmentierte UI-Konturen und Statusleisten
- holografische Layer, Scans, Sweeps, Pulse und Datenstrom-Anmutung

### Stilreferenz

Mischung aus:

- Cyberpunk-Neonästhetik
- Star-Trek-LCARS-ähnlicher Systempräzision
- Sci-Fi-Tactical-Display
- polished HUD statt gritty dystopia

### Design-Prinzipien

1. Effekte überall, aber kontrolliert.
2. Lesbarkeit bleibt wichtiger als Effektstärke.
3. Die Oberfläche soll hochwertig und "expensive" wirken, nicht verspielt-chaotisch.
4. Jede Animation braucht eine semantische Rolle:
   - Fokus
   - Systemaktivität
   - Energiefluss
   - Statusänderung
   - atmosphärische Tiefe

## Theme-Modell

Neuer Theme-Key:

- `cyberwar`

Erweiterungen:

- `CHAT_THEMES` in [ui/shared.js](ui/shared.js) um `cyberwar` ergänzen
- Theme-Picker in [ui/index.html](ui/index.html) und [ui/js/chat/main.js](ui/js/chat/main.js) erweitern
- Highlight-Handling um ein eigenes Cyberwar-Profil ergänzen

## Visuelle Architektur

### 1. Fundament

Die Basis ist kein reines Schwarz, sondern ein geschichteter dunkler Raum:

- Schwarzblau
- tiefe violette Schatten
- vereinzelte neonfarbene atmosphärische Gradienten
- subtile Raster-, Grid- oder Targeting-Strukturen

Ziel:

- Der Chat soll wie ein aktives Kommandodeck wirken, nicht wie eine normale App mit dunklem Theme.

### 2. Glossy-System

Gloss ist Kernbestandteil des Themes.

Glossy-Behandlung für:

- Header
- Pills
- Buttons
- Composer
- Dropdowns
- Bubbles
- Modals
- Session Drawer

Optische Bausteine:

- Glas-Layer
- harte Specular Highlights
- Edge-Shine
- innenliegende Lichtkanten
- farbige Reflexe entlang von Border-Linien
- Soft-Bloom um aktive Bereiche

### 3. UI-Geometrie

Formensprache:

- nicht nur runde Cards
- Mischung aus:
  - soften Radien
  - segmentierten Ecken
  - asymmetrischen Einschnitten
  - Panel-Stufen
  - taktischen Kantenlinien

Empfehlung:

- User-Bubbles dürfen aggressiver und energiegeladener sein
- Bot-Bubbles präziser, systemischer und "console-grade"
- Controls im Header eher wie Instrument-Module

### 4. Farbdramaturgie

Vorgeschlagene Rollen:

- Primär: Electric Cyan
- Sekundär: Plasma Magenta
- Tertiär: Hyper Blue
- Alarm/Combat: Acid Lime oder Laser Red je nach Status
- Utility/Command: Amber oder LCARS-ähnliches Warm Signal

Farbprinzip:

- dunkle Flächen bleiben wirklich dunkel
- bunte Farben leuchten stark und selektiv
- nie alles gleichzeitig maximal sättigen
- wichtige Hotspots bekommen Bloom, der Rest bleibt kontrolliert

## Komponenten-Plan

### Header

Der Header wird zur taktischen Schaltzentrale:

- mehrschichtiger Gloss-Hintergrund
- animierte Accent-Linien
- aktive Status-Pills mit Scan- oder Sweep-Effekt
- Theme-Picker optisch wie ein kleines "system skin selector" Modul

### Chat-Fläche

- stärker inszenierter Hintergrund
- Tiefenlayer mit Lichtfeldern, Partikelhauch oder Subgrid
- optionale horizontale oder diagonale Energielinien im Hintergrund
- Message-Area klar von Hintergrund getrennt, aber nicht flach

### Message-Bubbles

Bot:

- taktische Glass-Panels
- neonkonturierte Ränder
- feine Innenreflexe
- subtiler Bloom am Rand

User:

- kräftigere, energiereichere Flächen
- höherer Kontrast
- mehr Glow und "command issued" Charakter

Technische Outputs:

- stärker Richtung Holo-Console oder Debug-Terminal
- codierte Strukturen, subtile Grid-Hintergründe, digital scan feel

### Composer

Der Composer ist im Cyberwar-Theme ein Haupt-Showpiece.

Ziel:

- glossy command input dock
- stärkeres Tiefengefühl
- leuchtende Fokuszustände
- animierte Border-Energie im Fokus
- Tool-Buttons wie modulare Schaltflächen mit individueller Energiesignatur

### Session Drawer

- wirkt wie Seitenpanel eines taktischen Betriebssystems
- Sessions als mission logs / comm channels
- Hover und active states mit Sweep-Linien oder Scanner-Highlight

### Modals und Overlays

- holografischer Glass-Look
- weichere Pop-in-Animation
- klar abgesetzte Neon-Borders
- reduzierte Hintergrundunschärfe plus stärkerer Farb-Glow

## Motion-System

### Grundsatz

Viele Effekte, aber nicht alles permanent und gleich stark.

Motion-Kategorien:

1. Ambient motion
2. focus motion
3. state motion
4. feedback motion

### Ambient motion

Langsame, dauerhafte Hintergrundbewegung:

- Gradient drift
- scanning sweeps
- volumetric glow breathing
- minimaler grid/parallax shift

### Focus motion

Beim Hover/Fokus:

- edge glints
- bloom increase
- border energy travel
- glossy highlight sweep

### State motion

Für Statusanzeigen:

- pulsing pills
- warning shimmer
- mission-active scanner bars
- soft color cycling bei aktiven Systemelementen

### Message motion

Für neue Nachrichten:

- kein Standard-Fade
- stattdessen kontrolliertes "materialize" mit Lichtkante
- kurze Energieverdichtung im Bubble-Rand
- optional subtile Trail-/Bloom-Aufladung

## WebGL-Strategie

### Ziel

WebGL soll Atmosphäre erzeugen, nicht den gesamten Chat rendern.

Empfohlene Rolle von WebGL:

- separater Overlay- oder Background-Layer
- nur für additive Effekte
- niemals für Kernlayout oder Interaktion

### Geeignete WebGL-Effekte

1. volumetrische Farbnebel im Hintergrund
2. langsame Datenstrom-/Grid-Animation
3. refraktive Screen-Energie
4. moving highlight fields
5. scanner sweep / tactical radar wash
6. Partikel oder signal noise mit sehr geringer Dichte

### Nicht empfohlen

- permanente starke Verzerrung des gesamten DOM
- heavy post-processing auf allen UI-Layern
- Vollbildshader, die Textschärfe ruinieren
- Effektketten, die Mobile oder ältere Geräte zerstören

### Technische Empfehlung

Falls WebGL genutzt wird:

- neuer Chat-spezifischer Layer, z. B. `ui/js/chat/cyberwar-shader.js`
- Canvas host möglichst auf `#chat-box`
- `pointer-events: none`
- Theme-gesteuert via `data-theme="cyberwar"`
- Fallback auf reine CSS-Version, wenn WebGL nicht verfügbar ist

Die vorhandene Datei `ui/js/crt-shader.js` kann als technischer Referenzpunkt dienen, sollte aber nicht 1:1 übernommen werden, weil Cyberwar keine CRT-Simulation ist.

## Renderer- und Modulverträglichkeit

Besondere Prüfpunkte:

- Mermaid muss im Cyberwar-Theme eigene Farben oder dunkles High-Contrast-Profil bekommen
- Highlight.js sollte nicht nur `github-dark` bleiben, sondern Cyberwar-spezifische Overrides erhalten
- Drag-and-drop, STT, Warnings, Mood-Panel und Session Drawer brauchen Cyberwar-kompatible Oberflächen
- Image Lightbox und Modals dürfen nicht stilistisch aus dem Theme herausfallen

## Accessibility und Safety Rails

Auch im Effekt-Theme Pflicht:

- `prefers-reduced-motion` respektieren
- Lesbarkeit vor Glow priorisieren
- Fokuszustände klar sichtbar halten
- Textselection brauchbar halten
- keine hit-testing Probleme durch Overlay-Layer
- keine Animation, die Eingaben oder Streaming instabil macht

## Performance-Strategie

Cyberwar soll luxuriös wirken, aber progressiv degraden können.

### Qualitätsstufen

#### Stufe A: Full

- CSS ambient layers
- stärkere glow- und gloss-effekte
- WebGL background/overlay aktiv
- zusätzliche border animations

#### Stufe B: Balanced

- keine WebGL-Layer
- nur CSS-Gradient-Drift, glow, sweep
- reduzierte Shadow- und Blur-Ketten

#### Stufe C: Safe

- statische Hintergrundlayer
- minimale Hover-Animationen
- kein Dauer-Flicker, kein starker Drift

Trigger für Reduktion:

- Mobile
- `prefers-reduced-motion`
- schwächere Geräte
- fehlendes WebGL

## Implementierungsphasen

### Phase 1: Theme-Infrastruktur erweitern

- `cyberwar` als neuen Theme-Key einführen
- Picker erweitern
- Icon und Label ergänzen
- Theme-Wechselpfade stabil halten

Betroffene Dateien:

- [ui/shared.js](ui/shared.js)
- [ui/index.html](ui/index.html)
- [ui/js/chat/main.js](ui/js/chat/main.js)
- Chat-Sprachdateien unter `ui/lang/chat/`

### Phase 2: Design-Tokens definieren

- neue Cyberwar-spezifische Tokenblöcke anlegen
- Farben, Glows, Glass-Layer, Border-Profile, Surface-Hierarchien definieren

Betroffene Dateien:

- [ui/css/tokens.css](ui/css/tokens.css)
- [ui/css/chat.css](ui/css/chat.css)

### Phase 3: Core Chat Restyle

- Header
- Pills
- Chat viewport
- Bubbles
- Composer
- Footer controls
- Session Drawer
- Modals

### Phase 4: Motion und Gloss-Polish

- Sweep-Effekte
- Edge glints
- active-state pulses
- Nachrichteneinblendungen im neuen Stil
- ambient gradients

### Phase 5: Optionales WebGL-Layer

- dedizierten Cyberwar-Shader als Enhancement bauen
- Fallback sauber abfangen
- keine Abhängigkeit der Kernoptik vom Shader

### Phase 6: Theme-sensitive Renderer

- Mermaid
- Code highlighting
- sonstige theme-aware module

### Phase 7: Regression und Feinschliff

- Desktop
- Tablet
- Mobile
- Dark/Light/Retro/Cyberwar Wechsel
- Streaming
- Voice/STT
- Modals
- Drawer
- Attachments

## Empfohlene Dateistruktur für die Umsetzung

Minimal-invasiv:

- bestehende Chat-Dateien erweitern

Sauberer bei größerem Umfang:

- `ui/css/chat-cyberwar.css` als separates Theme-Layer
- optional `ui/js/chat/cyberwar-shader.js`

Empfehlung:

- Theme-Grundlagen in `chat.css`
- sehr große Cyberwar-Blöcke in eigene Datei auslagern, um `chat.css` nicht weiter aufzublähen

## Akzeptanzkriterien

- Chat unterstützt `cyberwar` live ohne Reload
- Theme wirkt deutlich anders als `dark` und `retro-crt`
- glossy, polished, high-end sci-fi look ist klar erkennbar
- starke dunkel-vs-neon-Kontraste funktionieren ohne Lesbarkeitsverlust
- bestehende Chat-Funktionen bleiben intakt
- Mermaid und Code-Blöcke fügen sich ins Theme ein
- Mobile und reduced-motion degradieren sauber
- WebGL ist optionales Enhancement, kein Single Point of Failure

## Empfehlung zur Produktentscheidung

Der Cyberwar-Look ist so dominant, dass er als bewusstes Premium-Theme behandelt werden sollte, nicht als neues Default.

Deshalb empfohlen:

- `dark` bleibt Standard
- `light` bleibt funktional
- `retro-crt` bleibt Spezialtheme
- `cyberwar` wird als Showcase-/Premium-Theme eingeführt

## Kurzfazit

Die vorhandene Theme-Infrastruktur reicht aus, um Cyberwar sauber zu integrieren. Der entscheidende Erfolgsfaktor ist nicht "noch mehr Glow", sondern ein konsistentes System aus:

- dunkler Raumtiefe
- glossy Sci-Fi-Chrome
- präziser Farbdramaturgie
- statusbezogener Motion
- optionalem WebGL-Atmosphärenlayer

Wenn sauber umgesetzt, kann Cyberwar das visuell stärkste Theme der gesamten App werden.
 