# Chat Theme Plan: Ocean

## Zielbild

Der Chat bekommt ein neues Premium-Theme `ocean` mit ruhiger, dunkler und blau dominierter Designsprache:

- tiefes Ozeanblau statt Schwarz als Grundstimmung
- sehr sauberer, aufgeräumter Look wie aus einem modernen UI/UX-Design-Book
- dezente, hochwertige Bewegung statt Showeffekten
- weiche Tiefenstaffelung, klare Flächen und kontrollierte Kontraste
- subtiler WebGL-Layer mit Wasser- bzw. Ripple-Bewegung zwischen Chat-Hintergrundbild und Chat-Inhalt

Das Theme soll elegant, ruhig und hochwertig wirken, nicht verspielt, nicht neonlastig und nicht "techy-chaotisch".

## Ausgangslage im Code

Die bestehende Chat-Architektur unterstützt bereits benannte Themes und Theme-spezifische Erweiterungen:

- Theme-State in [ui/shared.js](ui/shared.js)
- Theme-Picker in [ui/index.html](ui/index.html) und [ui/js/chat/main.js](ui/js/chat/main.js)
- Basis-Chat-Styling in [ui/css/chat.css](ui/css/chat.css)
- spezialisierte Theme-Layer in [ui/css/chat-cyberwar.css](ui/css/chat-cyberwar.css), [ui/css/chat-lollipop.css](ui/css/chat-lollipop.css) und [ui/css/chat-dark-sun.css](ui/css/chat-dark-sun.css)
- Theme-sensitive Mermaid-Renderer in [ui/js/chat/mermaid-renderer.js](ui/js/chat/mermaid-renderer.js) und [ui/js/chat/modules/mermaid-loader.js](ui/js/chat/modules/mermaid-loader.js)
- bestehende Shader-Architektur in [ui/js/chat/cyberwar-shader.js](ui/js/chat/cyberwar-shader.js) und [ui/js/chat/dark-sun-shader.js](ui/js/chat/dark-sun-shader.js)

Ocean sollte dieselbe Struktur nutzen und kein neues Theme-System einführen.

## Scope

- Nur die Chat-Seite ist in Scope.
- Bestehende Chat-Funktionen müssen visuell und funktional intakt bleiben.
- Das Theme wird zusätzlich zu `dark`, `light`, `retro-crt`, `cyberwar`, `lollipop` und `dark-sun` eingeführt.
- Theme-Wechsel muss live ohne Reload funktionieren.
- WebGL bleibt optionales Enhancement und darf nie Lesbarkeit oder Interaktion gefährden.

## Design-Richtung

### Leitidee

Nicht "Unterwasser-Fantasy", sondern:

- ruhige maritime Produktästhetik
- dunkle, hochwertige UI-Flächen
- klar gesetzte Hierarchie
- subtile Tiefe und Materialität
- leise Bewegung wie Licht auf Wasser

### Stilreferenz

Mischung aus:

- Editorial UI aus hochwertigen Produktdesign-Büchern
- maritime Blaupalette
- cleanen Dashboard-Systemen
- minimalen Motion-Systemen mit atmosphärischer Tiefe

### Design-Prinzipien

1. Ruhe vor Spektakel.
2. Klare Lesbarkeit und großzügige Flächen.
3. Blautöne tragen die Atmosphäre, nicht dekorative Ornamente.
4. Effekte sind fühlbar, aber nie dominant.
5. Die Oberfläche soll "crafted" und bedacht wirken.

## Theme-Modell

Neuer Theme-Key:

- `ocean`

Erweiterungen:

- `CHAT_THEMES` in [ui/shared.js](ui/shared.js)
- Picker in [ui/index.html](ui/index.html)
- Theme-Icon in [ui/js/chat/main.js](ui/js/chat/main.js)
- neues i18n-Key `chat.theme_ocean` in allen Chat-Sprachdateien
- dunkle Highlight.js-Basis beibehalten, Feintuning per CSS

Vorgeschlagenes Picker-Icon:

- `◌`

## Visuelle Architektur

### 1. Fundament

Basisfarben:

- Tiefsee-Navy
- gedämpftes Petrolblau
- kühles Slate-Blue
- hellere Aqua-Reflexe für Fokus und Akzente

Hintergrundwirkung:

- dunkle, ruhige Ozeantiefe
- leichte Lichtfelder statt harter Glow-Spots
- saubere Layer mit viel Luft

### 2. Oberflächen

Flächen sollen klar, weich und hochwertig wirken:

- matte bis seidenmatte Panels
- keine aggressive Gloss-Inszenierung
- sanfte Kantenlichter
- dezente innere Tiefe
- kontrollierte Schatten statt dramatischer Relief-Looks

### 3. Tiefenmodell

Tiefe entsteht durch:

- leicht gestaffelte Oberflächen
- weiche Layer-Schatten
- kühle Edge-Lights
- sanfte Transparenz im Header und in Overlays

Das Theme soll modern wirken, nicht skeuomorph.

### 4. Farbdramaturgie

Rollen:

- Primär: Deep Ocean Blue
- Sekundär: Petrol / Teal-Blue
- Tertiär: Soft Aqua
- Highlight: kühles Cyan-Blau in geringer Dosis

Prinzip:

- Flächen bleiben dunkel und ruhig
- Akzente nur an Interaktionspunkten und Statusflächen
- keine harten Neon-Kontraste

## Komponenten-Plan

### Header

- ruhige dunkle Leiste mit elegantem Tiefenverlauf
- Buttons wie hochwertige Produkt-Controls
- Pills klar, klein und sauber abgesetzt
- Theme-Picker wirkt wie präzises, neutrales UI-Element

### Chat-Fläche

- dunkles blaues Fundament mit sehr subtilen Lichtinseln
- Hintergrundbild bleibt sichtbar, aber zurückhaltend
- zwischen Watermark und Inhalt liegt eine ruhige Ripple-/Wasserbewegung

### Message-Bubbles

Bot:

- dunkle, kühle Panels
- leicht aufgehellte Outline
- präzise, saubere Lesefläche

User:

- etwas kräftigere Blau- bzw. Petrolflächen
- klarer Kontrast, aber nicht laut
- stärkerer Fokuspunkt als Bot-Bubbles

Technische Ausgaben:

- dunkle, strukturierte Panels
- kühle blaue Rahmenakzente
- saubere Code-Lesbarkeit ohne Gimmicks

### Composer

- wie ein ruhiges Control-Dock
- dunkle, hochwertige Einfassung
- klare Fokuszustände mit sanftem Aqua-Licht
- Send-Button als stärkster Akzent, aber ohne Glow-Explosion

### Session Drawer / Modals

- dunkle, aufgeräumte Karten
- dezente Transparenz möglich
- ruhige Schatten und dünne Linien
- stärker "product UI" als "sci-fi panel"

## Motion-System

### Grundsatz

Das Theme lebt von kontrollierter, langsamer Bewegung.

Motion-Kategorien:

1. ambient water motion
2. focus motion
3. state motion
4. message reveal motion

### Ambient motion

- langsame Ripple-Bewegung
- leichte Lichtverschiebungen
- kaum wahrnehmbare Tiefendrift im Hintergrund

### Focus motion

- dezente Lift-Effekte
- leicht stärkere Kantenlichter
- sanfte Aqua-Aufhellung im Fokus

### State motion

- aktive Controls mit ruhigem Puls
- Hover-Zustände mit kleiner Helligkeitsverschiebung
- keine sweependen Sci-Fi-Effekte

### Message motion

- neue Nachrichten weich und ruhig einblenden
- keine harten Scale- oder Glow-Effekte

## WebGL-Strategie

### Ziel

Der WebGL-Layer soll wie ruhiges Wasser zwischen Hintergrundbild und Chat-Inhalt liegen.

Er darf:

- das Hintergrundbild subtil beleben
- leichte Ripple-Verformungen erzeugen
- einen Hauch von Wasseroberfläche vermitteln

Er darf nicht:

- Text oder Bubbles überlagern
- die UI spiegelnd oder glitschig machen
- Leistung unnötig belasten

### Geeignete Effekte

1. langsame kreisförmige Ripples
2. sanfte horizontale Wasserverzerrung
3. dezente Lichtbrechung
4. minimale Tiefenbewegung

### Technische Anforderungen

- neues Skript `ui/js/chat/ocean-shader.js`
- Layer an `#chat-box` gebunden, nicht full-screen
- klar zwischen `#chat-box::after` und `#chat-content`
- `pointer-events: none`
- defensiver `z-index`, damit keine Bubbles verdeckt werden
- Fallback auf rein CSS-basierte Hintergrundruhe bei fehlendem WebGL
- starke Reduktion oder Abschaltung bei `prefers-reduced-motion`

## Renderer- und Modulverträglichkeit

- Mermaid bekommt einen eigenen `ocean`-Branch
- dunkles Highlight.js-Basisstylesheet bleibt erhalten
- Theme-Picker, Mood-Panel, Drawer, Warnings, Lightbox und Modals müssen visuell konsistent mitziehen

## Accessibility und Performance

- ausreichend Kontrast zwischen Bubble-Flächen und Text
- Fokuszustände müssen trotz dezenter Wirkung klar erkennbar bleiben
- Shader darf CPU/GPU nicht unnötig belasten
- mobile Geräte erhalten eine deutlich sparsamere Variante
- `prefers-reduced-motion` reduziert Ambient-Bewegung und Ripple-Effekt spürbar

## Umsetzungsschritte

1. Theme-Key `ocean` in Theme-State, Picker und Chat-JS ergänzen.
2. Übersetzungs-Key `chat.theme_ocean` in allen Chat-Sprachdateien ergänzen.
3. Neues Stylesheet `ui/css/chat-ocean.css` anlegen und in [ui/index.html](ui/index.html) laden.
4. Farb- und Oberflächenmodell für Header, Drawer, Composer, Bubbles und Modals umsetzen.
5. Mermaid-Theme-Konfiguration für `ocean` ergänzen.
6. Neues Shader-Skript `ui/js/chat/ocean-shader.js` als dezenten Ripple-Layer einführen.
7. Cache-Busting für neue Assets ergänzen.
8. Live-Wechsel, Lesbarkeit, Z-Index und Mobile-Verhalten prüfen.

## Testplan

- Theme-Wechsel live zwischen allen bestehenden Themes und `ocean`
- Picker-Icon, aktive Auswahl und `localStorage['aurago-theme']`
- Sichtbarkeit und Lesbarkeit von Bot-, User-, Technical- und Streaming-Bubbles
- Header, Pills, Composer, Session Drawer, Theme-Picker, Mood-Panel und Modals
- Ripple-Layer sichtbar, aber dezent, und klar hinter Chat-Inhalt
- Mermaid-Diagramme und Codeblöcke mit gutem Kontrast
- Mobile-Layout und reduzierte Shader-Variante
- `prefers-reduced-motion`
- keine Overlay-, Z-Index- oder Pointer-Event-Probleme

## Annahmen

- `ocean` ist ein Showcase-Theme, nicht das neue Default.
- Das Theme bleibt chat-lokal und verändert nicht das globale Site-Theming.
- Das Hintergrundbild des Chats bleibt Teil der Inszenierung.
- Die Wasser-/Ripple-Bewegung soll eher "beruhigend" als "effektvoll" wirken.
- Wenn WebGL auf einem Gerät nicht stabil läuft, muss die CSS-Variante weiterhin hochwertig aussehen.
