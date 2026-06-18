# Implementation Plan: Web-UI UX-Feinschliff

## Overview

Der Implementierungsansatz ist zweistufig und folgt direkt dem Design:

1. **Phase 1 – Design-System konsolidieren:** Zuerst wird das gemeinsame Fundament widerspruchsfrei gemacht – Auflösung des Typografie-Token-Konflikts (Befund A), Vervollständigung der Status-Tokens in beiden Themes (Befund B/C) und Schärfung der Basiskomponenten auf Token-Basis.
2. **Phase 2 – Ausrollen:** Erst danach werden die In-Scope-Seiten (Page-CSS/HTML/JS) auf das konsolidierte Token- und Komponentensystem zurückgeführt, Dropdowns/Modale vereinheitlicht und neue Texte in alle 15 Sprachen übersetzt.

Web-Chat (`index.html`, `chat-*`) und virtueller Desktop (`desktop.html`, `desktop-*`) bleiben strikt unangetastet. Die bestehende statische Go-UI-Regressionssuite (`ui/*_test.go`) muss durchgehend bestehen; neue Correctness Properties werden als **tabellengetriebene Go-Tests** über die vollständige (endliche) Domäne umgesetzt und nutzen die vorhandenen Helfer (`mustReadUIFile`, `normalizeAssetText`, `cssBlock`, `cssTokenInt`).

## Tasks

- [x] 1. Phase 1 – Typografie-Token-Konflikt auflösen
  - [x] 1.1 Typografie-Skala auf eine maßgebliche Quelle konsolidieren
    - Fluide `clamp()`-Skala (`--text-xs … --text-2xl`) in `ui/css/tokens.css` als einzige maßgebliche Definition belassen/setzen
    - Die doppelten `--text-*`-Definitionen aus `ui/shared-variables.css` (`:root,[data-theme="dark"]`) entfernen (Auflösung Befund A)
    - Token-Inventar der betroffenen Custom Properties + Definitionsorte dokumentieren, um Mehrfachdefinitionen aufzudecken
    - _Requirements: 1.1, 1.2_

  - [ ]* 1.2 Property-Test: Token-Single-Source
    - **Property 1: Token-Kategorie hat genau eine maßgebliche Definition**
    - Tabellengetriebener Go-Test über alle `--text-*`-Tokens: stellt sicher, dass jedes Typografie-Token nur in genau einer Design-System-Datei definiert ist und keine widersprüchliche Doppeldefinition existiert
    - Tag: `Feature: web-ui-ux-improvements, Property 1`
    - **Validates: Requirements 1.1, 1.2**

- [x] 2. Phase 1 – Status-Tokens in beiden Themes vervollständigen
  - [x] 2.1 Statusfamilie in `shared-variables.css` für dark und light komplettieren
    - `--success-bg`, `--warning-bg`, `--danger-bg` in **beiden** Theme-Blöcken (`:root,[data-theme="dark"]` und `[data-theme="light"]`) definieren (Auflösung Befund B)
    - `--success-dim/--warning-dim/--danger-dim` aus `css/tokens.css` als Aliasse auf die theme-abhängigen `-bg`-Tokens zurückführen bzw. ersetzen, um Doppeldefinitionen aufzulösen (Befund C)
    - _Requirements: 1.3, 5.1, 5.2_

  - [ ]* 2.2 Property-Test: Status-/Theme-Parität
    - **Property 2: Status-Tokens sind in beiden Themes vollständig**
    - Tabellengetriebener Go-Test über das Kreuzprodukt `{--success,--warning,--danger,--success-bg,--warning-bg,--danger-bg} × {dark, light}`: jeder Eintrag muss einen definierten Wert liefern
    - Tag: `Feature: web-ui-ux-improvements, Property 2`
    - **Validates: Requirements 1.3, 5.1, 5.2**

- [x] 3. Phase 1 – Basiskomponenten schärfen
  - [x] 3.1 Basiskomponenten in `shared-components.css` auf Tokens verankern
    - `.card`, `.btn` (+ Varianten), `.badge` (+ Varianten), `.form-group input/select/textarea`, `.page-header` auf `var(--token)` statt Festwerte umstellen
    - Naming-/Override-Vertrag (Einzelklassen-Basis, `.page .component`-Descendant, `.aura-*`-Modifier) in `shared-variables.css` dokumentiert beibehalten
    - Glassmorphism, Teal-Accent und Dark/Light-Charakteristik bewahren
    - _Requirements: 2.1, 4.1, 4.2, 6.1, 6.2_

  - [ ]* 3.2 Marker-Test: Präsenz und Token-Verankerung der Basiskomponenten
    - Beispiel-/Marker-Test: prüft, dass `.card`/`.btn`/`.badge`/Form/`.page-header` in `shared-components.css` vorhanden sind und Glassmorphism-/Teal-Marker erhalten bleiben
    - _Requirements: 2.1, 6.1_

- [x] 4. Checkpoint – Konsolidiertes Design-System
  - Ensure all tests pass, ask the user if questions arise.

- [x] 5. Phase 2 – Page-CSS auf Tokens/Basiskomponenten zurückführen
  - [x] 5.1 Tokenisierung: config, dashboard, containers
    - Feste `font-size`/`border-radius`/`clamp()`-Werte in `css/config.css`, `css/dashboard.css`, `css/containers.css` durch `var(--token)` ersetzen
    - Karten/Buttons/Badges/Header aus Basiskomponenten ableiten; Abweichungen nur via Descendant-Selektor oder `.aura-*`
    - _Requirements: 1.4, 2.2, 2.3, 3.1, 3.2, 5.3_

  - [x] 5.2 Tokenisierung: gallery, media, knowledge
    - Festwerte in `css/gallery.css`, `css/media.css`, `css/knowledge.css` auf Tokens zurückführen; Basiskomponenten ableiten
    - _Requirements: 1.4, 2.2, 2.3, 3.1, 3.2, 5.3_

  - [x] 5.3 Tokenisierung: skills, plans, missions
    - Festwerte in `css/skills.css`, `css/plans.css`, `css/missions.css` (für `missions_v2.html`) auf Tokens zurückführen; Basiskomponenten ableiten
    - _Requirements: 1.4, 2.2, 2.3, 3.1, 3.2, 5.3_

  - [x] 5.4 Tokenisierung: invasion_control, truenas, cheatsheets
    - Festwerte in `css/invasion.css`, `css/truenas.css`, `css/cheatsheets.css` auf Tokens zurückführen; Basiskomponenten ableiten
    - _Requirements: 1.4, 2.2, 2.3, 3.1, 3.2, 5.3_

  - [x] 5.5 Tokenisierung: login, setup, 404
    - Festwerte in den Page-CSS zu `login.html`, `setup.html`, `404.html` auf Tokens zurückführen; Basiskomponenten ableiten
    - _Requirements: 1.4, 2.2, 2.3, 3.1, 3.2, 5.3_

  - [ ]* 5.6 Property-Test: Themenabhängige Werte aus Theme-Tokens
    - **Property 3: Themenabhängige Werte stammen aus Theme-Tokens**
    - Tabellengetriebener Go-Test über alle In-Scope-Page-CSS-Dateien: Anzahl themen-unabhängiger Festfarben für tokenisierbare Flächen nimmt nicht zu; themenabhängige Deklarationen nutzen `var(--…)`
    - Tag: `Feature: web-ui-ux-improvements, Property 3`
    - **Validates: Requirements 5.3, 1.4**

  - [ ]* 5.7 Property-Test: Basiskomponenten-Schutz im Page-CSS
    - **Property 4: Basiskomponenten werden im Page-CSS nicht neu definiert**
    - Tabellengetriebener Go-Test über alle In-Scope-Page-CSS × `{.card, .badge, .btn, …}`: kein reiner Einzelklassen-Selektor-Override; Abweichungen nur via Descendant-Selektor oder `.aura-*`
    - Tag: `Feature: web-ui-ux-improvements, Property 4`
    - **Validates: Requirements 2.3, 3.1, 3.2, 4.1**

- [x] 6. Phase 2 – Dropdowns für Auswahlfelder
  - [x] 6.1 Felder mit fester Optionsmenge auf `<select>` umstellen
    - In-Scope-HTML-Felder mit fixem Wertebereich, die heute Textfelder sind, in `<select>` mit vorhandenem vereinheitlichten Select-Style überführen
    - _Requirements: 7.1_

- [x] 7. Phase 2 – Modale Dialoge statt Browser-Alerts
  - [x] 7.1 Mitteilungen/Bestätigungen/Fehler über geteilte Helfer anzeigen
    - In-Scope-Page-JS auf `showAlert`/`showConfirm`/`showModal`/`showToast` aus `js/shared/shared-core.js` ausrichten; native `alert()/confirm()` vermeiden
    - Sichtbare Texte über `t(...)`/`data-i18n` lokalisieren
    - _Requirements: 8.1, 8.2_

  - [ ]* 7.2 Property-Test: Keine nativen Browser-Alerts
    - **Property 5: Keine nativen Browser-Alerts**
    - Tabellengetriebener Go-Test über alle In-Scope-JS-Dateien: kein Aufruf von `alert()`/`confirm()`; Nutzung der geteilten Helfer
    - Tag: `Feature: web-ui-ux-improvements, Property 5`
    - **Validates: Requirements 8.1, 8.2**

- [x] 8. Phase 2 – Übersetzungen für neue Texte
  - [x] 8.1 Neue sichtbare Texte mit `data-i18n` und Einträgen in 15 Sprachen
    - Für jeden neu eingeführten sichtbaren Text einen i18n-Schlüssel anlegen und in allen 15 Sprachdateien (`cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh`) sprachspezifisch übersetzen
    - Deutsch: persönliche Anrede („Du"), Sonderzeichen direkt (ö/ä/ü), keine Ersatzschreibung
    - _Requirements: 9.1, 9.2_

  - [ ]* 8.2 Property-Test: i18n-Vollständigkeit
    - **Property 6: i18n-Vollständigkeit neuer Texte**
    - Tabellengetriebener Go-Test über jeden neuen Schlüssel × 15 Sprachen: ein Übersetzungseintrag existiert je Sprache
    - Tag: `Feature: web-ui-ux-improvements, Property 6`
    - **Validates: Requirements 9.1**

  - [ ]* 8.3 Property-Test: i18n-Lokalisierung statt EN-Kopie
    - **Property 7: i18n-Lokalisierung statt EN-Kopie**
    - Tabellengetriebener Go-Test über jeden neuen Schlüssel: nicht-englische Sprachdateien liefern sprachspezifische Werte (Allowlist für Eigennamen/Produktbegriffe)
    - Tag: `Feature: web-ui-ux-improvements, Property 7`
    - **Validates: Requirements 9.2**

- [ ] 9. Phase 2 – Excluded-Surfaces absichern
  - [ ]* 9.1 Integrationstest: Excluded-Surfaces unverändert
    - Go-Test, der sicherstellt, dass Web-Chat- und Desktop-Assets (`index.html`, `chat-*`, `desktop.html`, `desktop-*`) durch die Änderungen nicht verändert werden
    - _Requirements: 10.1, 10.2_

- [x] 10. Final-Checkpoint – Gesamte Regressionssuite
  - Gesamtlauf `go test ./ui/...` sicherstellen; alle bestehenden und neuen Tests müssen bestehen.
  - Ensure all tests pass, ask the user if questions arise.
  - _Requirements: 11.1_

## Notes

- Tasks mit `*` sind optional (Tests) und können für ein schnelleres MVP übersprungen werden; Kernimplementierung niemals optional.
- Phase 1 (Tasks 1–3) muss vor Phase 2 (Tasks 5–8) abgeschlossen sein, damit das Ausrollen auf einem widerspruchsfreien Fundament basiert.
- Jeder Property-Test referenziert seine Design-Eigenschaft per Tag und iteriert erschöpfend über die endliche Domäne (Status-Tokens × Themes, alle Page-CSS, 15 Sprachen).
- Die Tests sind statische Asset-Analysen und nutzen die vorhandenen Helfer; es wird keine rendernde Test-Infrastruktur eingeführt.
- Excluded-Surfaces (Web-Chat, virtueller Desktop) bleiben in jeder Aufgabe unangetastet.

## Task Dependency Graph

```json
{
  "waves": [
    { "id": 0, "tasks": ["1.1", "3.1"] },
    { "id": 1, "tasks": ["2.1", "1.2", "3.2"] },
    { "id": 2, "tasks": ["5.1", "5.2", "5.3", "5.4", "5.5", "2.2"] },
    { "id": 3, "tasks": ["5.6", "5.7", "6.1", "7.1"] },
    { "id": 4, "tasks": ["7.2", "8.1"] },
    { "id": 5, "tasks": ["8.2", "8.3", "9.1"] }
  ]
}
```
