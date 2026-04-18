# Chat Theme Plan: Retro CRT Shader Upgrade

## Zielbild

Das bestehende `retro-crt`-Theme soll von einem guten CRT-Look zu einer außergewöhnlich coolen, glaubwürdigen High-End-Retro-Simulation weiterentwickelt werden.

Nicht gemeint ist:

- billiger Glitch-Lärm
- aggressive Unlesbarkeit
- zufällige Effekte ohne stilistische Funktion

Gemeint ist:

- ein Bildschirm, der sich wie echte alte Röhrenhardware anfühlt
- mehrere Effektlagen mit klarer Aufgabe
- visuelle Wow-Momente, ohne die Nutzbarkeit zu opfern
- ein Finish, das eher an ikonische Premium-CRT-Ästhetik als an Meme-Filter erinnert

## Ausgangslage im Code

Der Retro-CRT-Stack existiert bereits:

- Theme-State in [ui/shared.js](ui/shared.js)
- Picker-Integration in [ui/index.html](ui/index.html) und [ui/js/chat/main.js](ui/js/chat/main.js)
- CRT-CSS-Basis in [ui/css/chat.css](ui/css/chat.css)
- aktueller WebGL-Overlay in [ui/js/crt-shader.js](ui/js/crt-shader.js)

Der aktuelle Shader liefert bereits:

- Barrel-Distortion-Hinweis
- Scanline-Feeling
- Vignette
- leichte Phosphor-Tönung
- minimales Flicker-Verhalten

Der Upgrade-Plan sollte den vorhandenen Shader nicht wegwerfen, sondern den CRT-Stack in mehrere bewusst inszenierte Shader- und CSS-Rollen aufteilen.

## Scope

- Nur das Chat-Theme `retro-crt` ist in Scope.
- Bestehende Interaktionen müssen erhalten bleiben.
- Texte, Buttons, Streaming und Overlays müssen nutzbar bleiben.
- Alle CRT-Effekte bleiben progressive Enhancements.
- `prefers-reduced-motion` und schwächere Geräte müssen eine klare reduzierte Variante erhalten.

## Design-Richtung

### Leitidee

Nicht nur "grüne alte Anzeige", sondern eine heroische, cineastische CRT-Simulation:

- phosphorgrüne Tiefe
- echte Röhrencharakteristik
- analoge Signalinstabilität
- Glas, Wölbung, Bloom und Persistence
- geerdete Imperfektion mit Stil

### Stilreferenz

Mischung aus:

- hochwertigem Terminal-Monitor
- Vintage-Broadcast-Equipment
- militärischer/industrieller Kontrolltechnik
- Sci-Fi-Displays mit echter Analog-Physik

### Design-Prinzipien

1. Jeder Effekt braucht eine technische oder ästhetische Begründung.
2. Die besten Effekte passieren in Schichten, nicht in einem Monster-Shader.
3. Lesbarkeit bleibt unantastbar.
4. Das CRT-Gefühl soll physisch wirken, nicht bloß farblich.
5. Der Look darf dramatisch sein, aber nicht hektisch.

## Upgrade-Ziel: Neuer CRT-Stack

Der neue Retro-CRT-Look soll in sechs koordinierten Ebenen gedacht werden.

### 1. Screen Glass Layer

Rolle:

- vermittelt, dass wir nicht direkt auf Pixel schauen, sondern durch Glas

Effekte:

- stärkere, aber immer noch subtile Curvature
- weiche Glasreflexe
- leichte Off-Axis-Helligkeitsverläufe
- differenzierte Randabdunklung

Technisch:

- bestehender CRT-Shader erweitert oder separater Screen-Surface-Shader
- kann teilweise auch in CSS leben, um GPU-Kosten klein zu halten

### 2. Raster Layer

Rolle:

- macht die Bildstruktur glaubwürdig

Effekte:

- fein abgestimmte horizontale Scanlines
- schwache vertikale Phosphor-/Slot-Mask-Anmutung
- Zeilenhelligkeit reagiert leicht auf Bildenergie

Technisch:

- Shader-basierte Modulation statt bloßer fixer CSS-Linien
- trotzdem fallbackfähig in CSS

### 3. Phosphor Response Layer

Rolle:

- macht helle Inhalte "elektrisch"

Effekte:

- Tight glow um helle Elemente
- breiter Bloom mit sehr wenig Alpha
- lokale Überstrahlung bei Buttons, Überschriften, Cursor, Typing und aktiven Zuständen

Technisch:

- bevorzugt lokal an Signal-/Luminanzzonen orientiert
- kein globaler Weichzeichner über den ganzen Screen

### 4. Analog Signal Layer

Rolle:

- gibt dem Bild Leben

Effekte:

- langsame Helligkeitspumpe
- minimale horizontale Line-Wobble
- sehr feines RF-/Signalrauschen
- subtile Sync-Unruhe

Technisch:

- amplitude sehr klein
- niemals Layout oder Klickflächen bewegen
- eher Screen-Illusion als Content-Transform

### 5. Persistence / Ghosting Layer

Rolle:

- erzeugt echten Röhrencharakter

Effekte:

- kurze Nachleuchteffekte an sehr hellen Bereichen
- leichter Nachhall bei neuen Streaming-Tokens
- minimales Trail-Verhalten bei starken Kontrastkanten

Technisch:

- optionales High-End-Feature
- erste Kandidatin zum Abschalten auf schwächeren Geräten
- darf nie flächig "schmieren"

### 6. Dramatic Hero FX Layer

Rolle:

- liefert das "unfassbar coole" Premium-Finish

Effekte:

- gelegentliche warme/kalte degauss-artige Randpulse beim Theme-Wechsel
- seltene Power-supply micro surges
- sehr subtile horizontal-traveling brightness sweep
- optionales phosphor recharge feeling bei Aktivitätspeaks

Technisch:

- sparsam und ereignisgetrieben
- keine dauerhafte Dauerdisco

## Konkrete Shader-Ideen

### Shader A: CRT Surface Shader

Zweck:

- übernimmt Curvature, Vignette, Glasreflex und Basis-Scanline-Logik

Erweiterungen gegenüber heute:

- realistischere Wölbung
- stärkerer Randfalloff
- differenzierterer Corner-Roll-off
- sanfte Fresnel-artige Screen-Highlights

### Shader B: Phosphor Bloom Shader

Zweck:

- ergänzt weiche Luminanzenergie

Inhalt:

- lokale Bloom-Kronen
- zweistufiges Glow-Verhalten
- stärkere Reaktion auf helle Controls und Textcluster

Wichtig:

- nie auf voller Deckkraft
- kann auch nur auf ausgewählten Screen-Bereichen laufen

### Shader C: Signal Disturbance Shader

Zweck:

- gibt dem Bild analoge Instabilität

Inhalt:

- sehr subtile line wobble
- weak sync drift
- analog noise modulation
- micro shimmer

Wichtig:

- nicht konstant gleich stark
- leichte Variabilität über die Zeit

### Shader D: Persistence Shader

Zweck:

- simuliert phosphor persistence / afterglow

Inhalt:

- ghosted luminance echoes
- kurze "memory" des hellen Bilds
- fühlt sich besonders gut bei Cursor, Stream und aktiven Buttons an

Wichtig:

- nur als optionales High-End-Layer
- sehr restriktiv in Dauer und Alpha

## Architekturvorschlag

### Variante 1: Mehrere spezialisierte Overlays

Neue Dateien:

- `ui/js/crt-screen-shader.js`
- `ui/js/crt-signal-shader.js`
- `ui/js/crt-persistence-shader.js`

Vorteile:

- klar getrennte Verantwortlichkeiten
- besser tuningbar
- Effekte einzeln deaktivierbar

Nachteil:

- mehr Layer- und Z-Index-Disziplin notwendig

### Variante 2: Ein Master-Shader plus Event-Hooks

Neue Richtung:

- `ui/js/crt-shader.js` stark ausbauen
- thematische Effektmodi intern staffeln

Vorteile:

- weniger DOM-/Canvas-Layer
- einfachere Einbindung

Nachteil:

- komplexerer Shader
- schwieriger zu debuggen und fein zu justieren

### Empfehlung

Hybrid:

- ein stärkerer Hauptshader für Screen Surface + Raster + Basissignal
- ein optionaler zweiter Shader oder Canvas-Layer nur für Persistence / Hero FX

So bleibt das Setup kontrollierbar und gleichzeitig beeindruckend.

## Geplante Effektpakete

### Paket 1: Screen Authenticity

- bessere Curvature
- besserer Corner Falloff
- glaubwürdigeres Glass
- dynamischere Scanlines
- differenziertere Vignette

### Paket 2: Signal Life

- line wobble
- sync drift
- analog shimmer
- weak noise modulation
- phosphor energy breathing

### Paket 3: Premium Drama

- persistence
- occasional sweep
- degauss/power bloom on theme activation
- activity-reactive luminance surges

## Event-getriebene Effekte

Bestimmte CRT-Effekte sollten nicht permanent laufen, sondern nur bei Anlässen:

### Theme activation

- kurze power-on glow stabilization
- minimales degauss-inspired edge pulse

### New message arrives

- kleiner phosphor charge-up
- lokaler brightness pulse

### Streaming in progress

- ganz leichte progressive afterglow
- Cursor-Zone wirkt lebendiger

### Hover / focus

- controls bekommen nur einen Hauch zusätzlicher phosphor energy
- nicht wie moderne neon UI, sondern wie stärker angesteuerte Röhrenzonen

## CSS-Erweiterungen

Auch ohne neue Shader sollte die CRT-Basis parallel verbessert werden:

- bessere component-local bloom tokens
- differenziertere Texthelligkeitsstufen
- refined code-block phosphor treatment
- stabilere button contrast hierarchy
- evtl. separates treatment für:
  - tool output
  - composer
  - modals
  - session drawer

CSS bleibt wichtig, damit der CRT-Look auch ohne High-End-Shaders noch hervorragend wirkt.

## Safety und Nutzbarkeit

### Muss erhalten bleiben

- Textlesbarkeit
- Textauswahl
- Streaming-Performance
- Klickflächen
- Mermaid-Nutzung
- Code-Lesbarkeit
- Modal-/Drawer-Interaktion
- Mobile-Nutzbarkeit

### Muss vermieden werden

- schwarzer Rand, der Inhalte verdeckt
- zu starke barrel distortion auf Text
- flächiges Flackern
- full-screen overlay bugs
- massive GPU-Last auf schwächeren Geräten
- ghosting, das Chat-Inhalte unlesbar macht

## Performance- und Fallback-Strategie

- `prefers-reduced-motion`: fast alle zeitbasierten Störungen aus
- Mobile:
  - kein Persistence-Layer
  - vereinfachte Scanline-/Surface-Version
  - reduzierte Noise- und Distortion-Amplitude
- fehlendes WebGL:
  - starke CSS-Fallback-Version
  - Screen-Rand, Scanline, Bloom-Hints bleiben

## Technische Umsetzungsschritte

1. Bestehenden CRT-Shader analysieren und in Surface-/Signal-Verantwortung aufteilen.
2. CRT-Overlay-Stack definieren: welche Layer bleiben in CSS, welche wandern in WebGL.
3. Hauptshader für Curvature, Raster und Basissignal deutlich erweitern.
4. Optionalen zweiten Persistence-/Hero-FX-Layer entwerfen.
5. Event-Hooks für Theme-Wechsel, neue Nachrichten und Streaming definieren.
6. CRT-CSS in [ui/css/chat.css](ui/css/chat.css) an die neuen Shader-Level anpassen.
7. Z-Index-, Pointer-Event- und Mobile-Fallbacks absichern.
8. Sichtbarkeit, Kontrast und Performance im echten Browser gegenprüfen.

## Testplan

- Theme-Wechsel auf `retro-crt`
- Sichtbarkeit aller Chat-Bubbles
- Buttons, Composer, Drawer, Modals und Tool-Outputs
- Streaming unter laufenden Shadern
- Mermaid und Codeblöcke
- Mobile/kleinere Auflösungen
- `prefers-reduced-motion`
- Shader on/off, WebGL unsupported
- keine verdeckenden Ränder oder Interaktionsprobleme

## Erfolgsmaßstab

Der Upgrade ist gelungen, wenn das Theme:

- sofort als außergewöhnlich hochwertiger CRT-Look wahrgenommen wird
- deutlich physischer und glaubwürdiger wirkt als heute
- trotzdem produktiv benutzbar bleibt
- auf guten Geräten sichtbar spektakulär ist
- auf schwächeren Geräten sauber und elegant degradiert
