# Design Document

## Übersicht

Dieses Design beschreibt den visuellen Feinschliff und die Konsistenz-Vereinheitlichung der klassischen Web-UI von AuraGo. Der Ansatz ist bewusst zweistufig:

1. **Konsolidierung des gemeinsamen Design-Systems** – Auflösung der heute widersprüchlichen Design-Tokens und Schärfung der Basiskomponenten in `ui/shared-variables.css`, `ui/css/tokens.css` und `ui/shared-components.css`.
2. **Ausrollen auf die In-Scope-Seiten** – Page-CSS und HTML der eingeschlossenen Seiten werden auf das konsolidierte Token- und Komponentensystem zurückgeführt, ohne den erkennbaren Grundstil zu verändern.

Der Web-Chat (`index.html`, `chat-*.css/js`) und der virtuelle Desktop (`desktop.html`, `desktop-*`-Assets samt der Themes `fruity` und `standard`) sind **strikt ausgeschlossen** und werden weder verändert noch in ihrem Verhalten beeinflusst.

Das Design ist konkret im Bestand verankert. Die nachfolgend dokumentierten Inkonsistenzen wurden im aktuellen Code verifiziert.

### Verankerung im Bestand: festgestellte Inkonsistenzen

| # | Befund (verifiziert) | Quelle | Betrifft |
|---|----------------------|--------|----------|
| A | `--text-base`, `--text-sm`, `--text-lg`, `--text-xs` sind in `shared-variables.css` als **`clamp()`** (fluid) definiert, in `css/tokens.css` zusätzlich als **feste `rem`**. `tokens.css` wird laut `config.html` **nach** `shared-variables.css` geladen und gewinnt im Cascade (gleiche Spezifität, spätere Quelle). Die fluide Typografie ist dadurch faktisch wirkungslos. | `shared-variables.css` (`:root,[data-theme="dark"]`), `css/tokens.css` (`:root`), Ladereihenfolge in `config.html` | Req 1.1, 1.2 |
| B | `--warning-bg` ist **nur** im Dark-Theme-Block (`:root,[data-theme="dark"]`) definiert, **fehlt** im `[data-theme="light"]`-Block. | `shared-variables.css` | Req 1.3, 5.x |
| C | `css/tokens.css` definiert ergänzende Tokens (`--success-dim`, `--warning-dim`, `--danger-dim`, `--info`) nur unter `:root` ohne Light-Theme-Pendant; Statusfarben (`--success/--warning/--danger`) liegen dagegen in `shared-variables.css` mit Light-Override. Die Statusfamilie ist auf zwei Dateien verteilt. | `shared-variables.css`, `css/tokens.css` | Req 1.1, 1.3 |
| D | Page-CSS enthält in grossem Umfang fest kodierte Werte statt Tokens (z. B. allein `dashboard.css` ~115 feste `font-size: …rem` / `border-radius: …px`; ~45 `clamp()`-Vorkommen über die In-Scope-Page-CSS verteilt). | `ui/css/*.css` | Req 1.4, 5.3 |
| E | Eine geteilte Modal-/Toast-Infrastruktur existiert bereits (`showModal`, `showConfirm`, `showAlert`, Toasts) in `js/shared/shared-core.js`. Native `alert()/confirm()` werden in Page-JS heute nicht aufgerufen. Der `i18n_lint_test` erzwingt bereits, dass keine hartkodierten `alert()/confirm()/showToast(...)`-Strings auftreten. | `js/shared/shared-core.js`, `i18n_lint_test.go` | Req 8.1, 8.2 |
| F | Die UI-Regressionssuite (`ui/*_test.go`) ist **statisch** (Asset-Dateien werden gelesen und auf Marker/CSS-Blöcke/i18n-Parität geprüft), nicht rendernd. Helfer: `normalizeAssetText`, `mustReadUIFile`, `cssBlock`, `cssTokenInt`, `cssZIndexFromBlock`. | `ui/chat_regression_test.go`, `ui/config_composio_test.go`, `ui/modal_regression_test.go` | Req 11 |

### Detail zu Befund A (Cascade-Konflikt der Typografie)

`config.html` lädt in dieser Reihenfolge:

```html
<link rel="stylesheet" href="/shared-variables.css">      <!-- --text-base: clamp(...) -->
<link rel="stylesheet" href="/shared-utilities.css">
<link rel="stylesheet" href="/shared-components.css?v=...">
<link rel="stylesheet" href="/shared-animations.css">
<!-- ... -->
<link rel="stylesheet" href="/css/tokens.css">            <!-- --text-base: 1rem (gewinnt) -->
<link rel="stylesheet" href="/css/config.css?v=...">
```

`shared-variables.css` setzt die fluiden Werte auf den Selektoren `:root, [data-theme="dark"]`; `tokens.css` setzt die festen Werte auf `:root`. Bei gleicher Spezifität entscheidet die Quellreihenfolge – die später geladene `tokens.css` überschreibt. Ergebnis: Der `clamp()`-Wert ist toter Code.

## Architektur

### Schicht- und Lade-Architektur (Soll-Zustand)

Das Design-System bleibt in der bestehenden Dateistruktur, erhält aber **eindeutige Zuständigkeiten pro Datei**, damit es genau eine maßgebliche Definition pro Token-Kategorie gibt.

```
ui/shared.css                  (@import-Aggregator; unverändert)
 ├─ shared-variables.css       MASSGEBLICH: Theme-abhängige Tokens
 │                             (Farben, Hintergründe, Border, Statusfarben inkl.
 │                              -bg-Varianten) je dark/light; KEINE Typografie-Skala mehr
 ├─ shared-utilities.css       Atom-Utilities
 ├─ shared-components.css      MASSGEBLICH: Basiskomponenten (.card/.btn/.badge/Form/Header)
 └─ shared-animations.css      Keyframes

ui/css/tokens.css              MASSGEBLICH: Theme-UNABHÄNGIGE Skalen
                               (Spacing, Typografie-Skala, Radien, Schatten,
                                Z-Index, Transitions); Status-DIM-Varianten
                                werden auf shared-variables.css-Tokens zurückgeführt
```

**Leitprinzip „eine Quelle pro Kategorie":**

- **Theme-abhängig** (ändert sich zwischen dark/light) → ausschließlich `shared-variables.css`, jeweils in beiden Theme-Blöcken.
- **Theme-unabhängig** (konstant über Themes) → ausschließlich `css/tokens.css`.
- Die Typografie-Skala (`--text-xs … --text-2xl`) wird **als eine einzige maßgebliche Definition** geführt. Entscheidung: **fluide `clamp()`-Skala** in `css/tokens.css` (theme-unabhängig), und die doppelte Definition in `shared-variables.css` wird entfernt. Damit gewinnt nicht mehr „zufällig die spätere Datei", sondern es existiert nur noch eine Quelle, und die fluide Typografie ist wieder wirksam.

> Hinweis zur Ladereihenfolge: Da Typografie nach der Konsolidierung nur noch in `tokens.css` steht, ist die heutige Reihenfolge unkritisch. Es wird dennoch empfohlen, `tokens.css` früh (vor Page-CSS) zu laden; die HTML-Reihenfolge der In-Scope-Seiten wird dahingehend vereinheitlicht, ohne Excluded-Surfaces anzufassen.

### Verarbeitungsfluss der Konsolidierung

```
Phase 1 – Design-System
  1. Token-Inventar erstellen (alle Custom Properties + Definitionsorte)
  2. Konflikte auflösen:
       - Typografie-Skala: clamp() in tokens.css behalten, Duplikate in
         shared-variables.css entfernen
       - Status-Tokens: --success/--warning/--danger und alle -bg/-dim-Varianten
         in BEIDE Theme-Blöcke von shared-variables.css aufnehmen (inkl. --warning-bg light)
  3. Basiskomponenten in shared-components.css schärfen (Tokens statt Festwerte,
       Naming-Konvention dokumentiert beibehalten)

Phase 2 – Ausrollen je In_Scope_Seite
  4. Page-CSS auf Tokens/Basiskomponenten zurückführen (Festwerte → var(--token))
  5. Abweichungen nur via Descendant-Selektor (.page .component) oder .aura-*-Modifier
  6. Felder mit fester Optionsmenge → <select>
  7. Meldungen/Bestätigungen → showModal/showConfirm/showAlert (kein natives alert())
  8. Neue sichtbare Texte → data-i18n + Einträge in allen 15 Sprachen
  9. Excluded-Surfaces unangetastet lassen; Regressionssuite ausführen
```

## Komponenten und Schnittstellen

### 1. Design-Token-System

**Theme-abhängige Tokens (`shared-variables.css`, je `:root,[data-theme="dark"]` und `[data-theme="light"]`):**

- Hintergründe: `--bg-primary`, `--bg-secondary`, `--bg-tertiary`, `--bg-glass`, `--header-bg`, `--card-bg`, `--sidebar-bg`, `--input-bg`
- Text: `--text-primary`, `--text-secondary`, `--strong-color`
- Accent: `--accent`, `--accent-dim`, `--accent-glow`
- Border: `--border-subtle`, `--border-accent`, `--input-border`
- Schatten/Highlights: `--surface-shadow`, `--surface-shadow-strong`, `--surface-highlight`
- **Statusfarben (NEU: vollständig in beiden Themes):** `--success`, `--warning`, `--danger` **und** die Hintergrundvarianten `--success-bg`, `--warning-bg`, `--danger-bg`

Korrektur zu Befund B/C – `--warning-bg` (und Geschwister) wird im Light-Theme ergänzt:

```css
/* shared-variables.css — [data-theme="light"] (Ergänzung) */
[data-theme="light"] {
    /* … bestehende Light-Tokens … */
    --success-bg: rgba(5, 150, 105, 0.14);
    --warning-bg: rgba(217, 119, 6, 0.16);
    --danger-bg:  rgba(220, 38, 38, 0.14);
}
```

Im Dark-Block werden `--success-bg` und `--danger-bg` analog ergänzt, sodass die gesamte Statusfamilie in beiden Themes vollständig ist. Die bisher in `tokens.css` liegenden `--success-dim/--warning-dim/--danger-dim` werden als Aliasse auf die theme-abhängigen `-bg`-Tokens zurückgeführt oder durch diese ersetzt, um Doppeldefinitionen aufzulösen.

**Theme-unabhängige Skalen (`css/tokens.css`, `:root`):**

```css
:root {
  /* Typografie-Skala – EINZIGE maßgebliche Definition (fluid) */
  --text-xs:   clamp(0.65rem, 0.6rem + 0.15vw, 0.75rem);
  --text-sm:   clamp(0.75rem, 0.7rem + 0.2vw, 0.85rem);
  --text-base: clamp(0.875rem, 0.8rem + 0.25vw, 1rem);
  --text-lg:   clamp(1.1rem, 1rem + 0.3vw, 1.3rem);
  --text-xl:   clamp(1.25rem, 1.1rem + 0.4vw, 1.5rem);
  --text-2xl:  clamp(1.5rem, 1.3rem + 0.6vw, 1.9rem);

  /* Spacing, Radien, Schatten, Z-Index, Transitions – unverändert maßgeblich hier */
}
```

Die vier `--text-*`-Zeilen werden aus `shared-variables.css` **entfernt** (Auflösung Befund A).

### 2. Basiskomponenten (`shared-components.css`)

Die bestehenden Basiskomponenten bleiben mit ihren Einzelklassen-Selektoren maßgeblich und werden auf Tokens verankert:

- **Karte:** `.card`
- **Buttons:** `.btn`, Varianten `.btn-primary/.btn-secondary/.btn-danger/.btn-success/.btn-sm/.btn-header`
- **Badges:** `.badge`, Varianten `.badge-active/.badge-running/.badge-warning/.badge-idle/.badge-ssh`
- **Formularfelder:** `.form-group input/select/textarea`, vereinheitlichter Select-Style
- **Seitenkopf:** `.page-header` (+ `.media-header/.gallery-header`)

**Naming-/Override-Vertrag (unverändert dokumentiert in `shared-variables.css`):**

- Basiskomponenten = Einzelklassen-Selektor, niedrige Spezifität.
- Page-spezifische Anpassung = **Descendant-Selektor** `.page-name .component { … }`.
- Neue page-spezifische Varianten = **`.aura-*`-Präfix** (z. B. `.aura-badge--mission`).
- **Verboten:** Neudefinition von `.card`/`.btn`/`.badge` als reiner Einzelklassen-Selektor im Page-CSS.

### 3. Dropdown-Schnittstelle (Req 7)

Felder mit fester Optionsmenge werden als `<select>` umgesetzt und nutzen den bereits vorhandenen vereinheitlichten Select-Style (`select, .form-select, .field-select`) inkl. Custom-Chevron, Hover-/Focus-Glow und Theme-Tokens. Keine neue Komponente nötig; ggf. werden vorhandene Textfelder mit fixem Wertebereich in `<select>` überführt.

### 4. Modal-/Toast-Schnittstelle (Req 8)

Nutzung der bestehenden Helfer aus `js/shared/shared-core.js`:

```js
// Promise-basiert; ersetzt alert()/confirm()
showAlert(title, message)            // Hinweis (nur OK)
showConfirm(title, message)          // Bestätigung (OK/Abbrechen) -> Promise<boolean>
showModal(title, message, isConfirm, options)
showToast(message, type)             // transiente Meldung
```

Vertrag: In-Scope-Seiten zeigen Mitteilungen/Bestätigungen/Fehler ausschließlich über diese Helfer; native `alert()/confirm()` sind unzulässig. Sichtbare Texte werden über `t(...)`/`data-i18n` lokalisiert (kein hartkodierter String – bereits durch `i18n_lint_test` abgesichert).

### 5. i18n-Schnittstelle (Req 9)

Neue sichtbare Texte erhalten einen `data-i18n`-Schlüssel bzw. werden über `t(key)` aufgelöst. Für jeden neuen Schlüssel wird in **allen 15** Sprachdateien (`cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh`) ein Eintrag mit **sprachspezifischer** Übersetzung angelegt (kein pauschaler EN-Text). Anredeform: persönlich (deutsch „Du"); Sonderzeichen direkt (ö/ä/ü), keine Ersatzschreibung.

## Datenmodelle

Reines UI-/Styling-Feature ohne Persistenz- oder API-Datenmodell. Die relevanten „Modelle" sind die statisch prüfbaren Mengen, über die die Konsolidierung und die Tests quantifizieren:

```text
Design_System_Dateien := { shared-variables.css, css/tokens.css, shared-components.css,
                            shared-utilities.css, shared-animations.css, shared.css }

Token_Kategorie        := { Spacing, Typografie, Radien, Schatten, Transitions, Statusfarben, ... }

Status_Token           := { --success, --warning, --danger,
                            --success-bg, --warning-bg, --danger-bg }

Theme                  := { dark, light }

In_Scope_Seite         := { config, dashboard, containers, gallery, media, knowledge,
                            skills, plans, missions_v2, invasion_control, truenas,
                            cheatsheets, login, setup, 404 }
  -> Page_CSS-Zuordnung (Beispiele): missions_v2 -> css/missions.css,
     invasion_control -> css/invasion.css, übrige analog css/<seite>.css

Excluded_Surface       := { index.html, chat-*.css, chat-*.js,
                            desktop.html, desktop-*.css, desktop_*-Assets }

Sprache (15)           := { cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh }

Basis_Komponente_Klasse:= { .card, .btn, .badge, ... atomare Basisklassen }
```

## Fehlerbehandlung

- **Token-Auflösung:** Verbleibende `var(--token)`-Referenzen erhalten konservative Fallbacks dort, wo bereits üblich (z. B. `var(--accent-dim, rgba(...))`), damit fehlende Tokens nie zu unsichtbaren/kontrastlosen Flächen führen.
- **Theme-Lücken:** Jedes theme-abhängige Token muss in beiden Theme-Blöcken existieren; fehlende Pendants (wie `--warning-bg` light) werden ergänzt, statt auf Vererbung des Dark-Werts zu hoffen.
- **Modale:** `showConfirm` liefert ein `Promise<boolean>`; ablehnende/abbrechende Pfade müssen explizit behandelt werden. Kein stiller Fallback auf `alert()`.
- **i18n-Lücken:** Fehlt ein Schlüssel in einer Sprache, ist das ein Testfehler (siehe Property 6), nicht ein Laufzeit-Fallback auf Englisch.
- **Excluded-Surfaces:** Änderungen, die versehentlich Excluded-Dateien berühren würden, gelten als Fehler und werden durch Regression abgefangen.

## Teststrategie

Die bestehende Suite ist **statische Asset-Analyse** in Go (`ui/*_test.go`). Die hier definierten Korrektheits-Eigenschaften quantifizieren über **endliche, aufzählbare Domänen** (Token-Kategorien, Status-Tokens × Themes, Page-CSS-Dateien, JS-Dateien, 15 Sprachen). Für solche endlichen Domänen ist **erschöpfende tabellengetriebene Iteration** aussagekräftiger als zufälliges Sampling; die Eigenschaften werden daher in Go als tabellengetriebene Tests über die vollständige Domäne umgesetzt (de-facto Property-Tests mit vollständiger Abdeckung statt Stichprobe).

- **Property-Tests (erschöpfend über die Domäne):** Token-Single-Source, Status-/Theme-Parität, Basiskomponenten-Schutz, Kein-natives-Alert, i18n-Vollständigkeit, i18n-Lokalisierung.
- **Beispiel-/Marker-Tests:** Präsenz der Basiskomponenten, konkrete Dropdown-Umwandlungen, Glassmorphism-/Teal-Marker.
- **Integrations-/Smoke-Tests:** Excluded-Surfaces unverändert, Gesamtlauf der Regressionssuite (`go test ./ui/...`), visuelle Theme-Prüfung (manuell, da Rendering/Kontrast nicht statisch beweisbar).

Hilfsfunktionen wieder verwenden: `mustReadUIFile`, `normalizeAssetText`, `cssBlock`, `cssTokenInt`.

**Konfiguration der Property-Tests:** Da die Domänen endlich sind, wird **vollständig** iteriert (alle Status-Tokens × beide Themes; alle In-Scope-Page-CSS; alle 15 Sprachen). Wo eine echte Generator-basierte Variation sinnvoll wäre (sie ist es hier wegen fester Mengen nicht), gilt sonst die Mindestvorgabe von 100 Iterationen. Jeder Property-Test referenziert seine Design-Eigenschaft per Tag:
`Feature: web-ui-ux-improvements, Property {Nr}: {Property-Text}`.

## Correctness Properties

*Eine Property ist ein Merkmal oder Verhalten, das über alle gültigen Ausführungen/Zustände des Systems hinweg gelten soll – eine formale Aussage darüber, was das System tun soll. Properties bilden die Brücke zwischen menschenlesbarer Spezifikation und maschinell prüfbaren Korrektheitsgarantien.*

### Property 1: Token-Kategorie hat genau eine maßgebliche Definition

*Für jede* Design-Token-Kategorie und *für jedes* Design-Token gilt: Das Design-System definiert den Token nicht in mehr als einer Design-System-Datei mit unterschiedlichen Werten; je Kategorie existiert genau eine maßgebliche Quelle (insbesondere ist die Typografie-Skala `--text-*` nur an einer Stelle definiert).

**Validates: Requirements 1.1, 1.2**

### Property 2: Status-Tokens sind in beiden Themes vollständig

*Für jedes* Status-Token (`--success`, `--warning`, `--danger` sowie die zugehörigen Hintergrundvarianten `--success-bg`, `--warning-bg`, `--danger-bg`) und *für jedes* Theme aus `{dark, light}` liefert das Design-System einen definierten Wert.

**Validates: Requirements 1.3, 5.1, 5.2**

### Property 3: Themenabhängige Werte stammen aus Theme-Tokens

*Für jede* themenabhängige Farb-, Hintergrund- oder Rahmen-Deklaration auf den konsolidierten Flächen einer In-Scope-Seite gilt: Der Wert wird über ein theme-abhängiges Token (`var(--…)`) bezogen und nicht als themen-unabhängiger fester Farbwert wiederholt; die Anzahl themen-unabhängiger Festfarben für tokenisierbare Flächen nimmt durch die Änderungen nicht zu.

**Validates: Requirements 5.3, 1.4**

### Property 4: Basiskomponenten werden im Page-CSS nicht neu definiert

*Für jede* In-Scope-Page-CSS-Datei und *für jede* Basiskomponentenklasse (`.card`, `.badge`, `.btn` und weitere atomare Basisklassen) gilt: Die Klasse wird nicht über ihren reinen Einzelklassen-Selektor neu definiert; jede Abweichung erfolgt über einen Descendant-Selektor (`.page-name .component`) oder eine `.aura-*`-Modifikatorklasse.

**Validates: Requirements 2.3, 3.1, 3.2, 4.1**

### Property 5: Keine nativen Browser-Alerts

*Für jede* JavaScript-Datei einer In-Scope-Seite gilt: Mitteilungen, Bestätigungen und Fehlermeldungen werden über die geteilten Helfer (`showModal`/`showConfirm`/`showAlert`/`showToast`) angezeigt, und die nativen Funktionen `alert()`/`confirm()` werden nicht aufgerufen.

**Validates: Requirements 8.1, 8.2**

### Property 6: i18n-Vollständigkeit neuer Texte

*Für jeden* neu eingeführten, für Nutzer sichtbaren i18n-Schlüssel und *für jede* der 15 unterstützten Sprachen (`cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh`) existiert ein Übersetzungseintrag.

**Validates: Requirements 9.1**

### Property 7: i18n-Lokalisierung statt EN-Kopie

*Für jeden* neu eingeführten i18n-Schlüssel gilt: Die nicht-englischen Sprachdateien liefern einen sprachspezifischen Wert und übernehmen nicht durchgängig identisch den englischen Text (Ausnahmen für Eigennamen/Produktbegriffe per Allowlist).

**Validates: Requirements 9.2**
