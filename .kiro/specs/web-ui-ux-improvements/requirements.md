# Requirements Document

## Introduction

Dieses Feature umfasst einen visuellen Feinschliff und eine Konsistenz-Vereinheitlichung der klassischen Web-UI von AuraGo. Ziel ist ein einheitliches Erscheinungsbild (Spacing, Typografie, wiederverwendbare Komponenten) im bestehenden Look (Glassmorphism, Teal-Accent, Dark/Light-Themes). Der Ansatz ist zweistufig: Zuerst wird das gemeinsame Design-System (Design-Tokens und Basiskomponenten) geschärft und vereinheitlicht, anschliessend wird dieses System auf alle eingeschlossenen Seiten ausgerollt.

Der Web-Chat und der virtuelle Desktop sind strikt ausgeschlossen und werden nicht angefasst. Die Modernisierung erfolgt behutsam; der Grundstil bleibt erkennbar.

## Glossary

- **Design_System**: Das gemeinsame CSS-Fundament von AuraGo, bestehend aus `ui/shared-variables.css`, `ui/css/tokens.css`, `ui/shared-components.css`, `ui/shared-utilities.css`, `ui/shared-animations.css` und `ui/shared.css`. Es definiert Design-Tokens (Custom Properties) und Basis-/Atomkomponenten.
- **Design_Token**: Eine CSS Custom Property (z. B. `--space-4`, `--text-base`, `--radius-lg`, `--accent`), die einen wiederverwendbaren Design-Wert für Spacing, Typografie, Radien, Schatten, Farben oder Transitions kapselt.
- **Basiskomponente**: Eine atomare, niedrig-spezifische Komponentenklasse des Design_System mit Einzelklassen-Selektor (z. B. `.card`, `.badge`, `.btn`).
- **In_Scope_Seite**: Eine der folgenden HTML-Seiten und der ihr zugeordneten Page-CSS/JS: `config.html`, `dashboard.html`, `containers.html`, `gallery.html`, `media.html`, `knowledge.html`, `skills.html`, `plans.html`, `missions_v2.html`, `invasion_control.html`, `truenas.html`, `cheatsheets.html`, `login.html`, `setup.html`, `404.html`.
- **Excluded_Surface**: Der Web-Chat (`index.html` inklusive `chat-*.css` / `chat-*.js`) und der virtuelle Desktop (`desktop.html` inklusive aller `desktop-*.css` / `desktop_*`-Assets samt der Themes `fruity` und `standard`).
- **Theme_System**: Der Mechanismus zur Themenumschaltung über das `data-theme`-Attribut mit den Werten `dark` (Standard) und `light`.
- **Page_CSS**: Eine seitenspezifische CSS-Datei in `ui/css/` (z. B. `config.css`, `dashboard.css`).
- **i18n_System**: Das Übersetzungssystem mit Sprachdateien unter `ui/lang/` für die 15 unterstützten Sprachen (cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh).
- **UI_Regressionstest_Suite**: Die bestehenden Go-basierten UI-Tests im Verzeichnis `ui/` (Dateien `*_test.go`).
- **Naming_Konvention**: Die in `ui/shared-variables.css` dokumentierten Regeln für CSS-Klassenbenennung und Overrides (Einzelklassen für Basiskomponenten, `.aura-*`-Präfix für neue seitenspezifische Varianten, Descendant-Selektoren für Page-Overrides).

## Requirements

### Requirement 1: Konsolidierung der Design-Tokens

**User Story:** Als Entwickler möchte ich einen einzigen, widerspruchsfreien Satz an Design-Tokens, damit Spacing, Typografie, Radien, Schatten und Farben über alle Seiten hinweg konsistent angewendet werden.

#### Acceptance Criteria

1. THE Design_System SHALL für jede semantische Token-Kategorie (Spacing, Typografie-Schriftgrössen, Radien, Schatten, Transitions, Statusfarben) genau eine massgebliche Definition bereitstellen.
2. IF ein Design_Token in mehr als einer Datei des Design_System mit unterschiedlichen Werten definiert ist, THEN THE Design_System SHALL die Mehrfachdefinition auf eine einzige massgebliche Definition reduzieren.
3. THE Design_System SHALL für jedes Statusfarben-Token (`--success`, `--warning`, `--danger` einschliesslich zugehöriger Hintergrund-Varianten wie `--warning-bg`) sowohl im Theme `dark` als auch im Theme `light` einen definierten Wert bereitstellen.
4. WHERE ein Wert über ein vorhandenes Design_Token verfügbar ist, THE In_Scope_Seite SHALL diesen Token verwenden, anstatt den Wert als feste Zahl im Page_CSS zu wiederholen.

### Requirement 2: Vereinheitlichung der Basiskomponenten

**User Story:** Als Entwickler möchte ich, dass gemeinsame UI-Elemente aus zentralen Basiskomponenten stammen, damit Karten, Buttons, Badges, Formularfelder und Seitenkopfzeilen über alle Seiten hinweg gleich aussehen und sich gleich verhalten.

#### Acceptance Criteria

1. THE Design_System SHALL Basiskomponenten für Karten, Buttons, Badges, Formular-Eingabefelder und Seitenkopfzeilen mit Einzelklassen-Selektoren bereitstellen.
2. THE In_Scope_Seite SHALL ihre Karten, Buttons, Badges, Formular-Eingabefelder und Seitenkopfzeilen aus den Basiskomponenten des Design_System ableiten.
3. WHERE eine In_Scope_Seite eine seitenspezifische Abweichung einer Basiskomponente benötigt, THE In_Scope_Seite SHALL die Abweichung über einen Descendant-Selektor (`.page-name .component`) oder eine `.aura-*`-Modifikatorklasse umsetzen.

### Requirement 3: Schutz der Basiskomponenten gegen Neudefinition

**User Story:** Als Entwickler möchte ich, dass Basiskomponenten nicht in Page-CSS neu definiert werden, damit Spezifitätskonflikte und Duplikate vermieden werden.

#### Acceptance Criteria

1. THE Page_CSS SHALL die Basiskomponentenklassen (`.card`, `.badge`, `.btn` und weitere atomare Basisklassen des Design_System) nicht über deren Einzelklassen-Selektor neu definieren.
2. WHERE ein Page_CSS eine Basiskomponente anpasst, THE Page_CSS SHALL einen Descendant-Selektor oder eine `.aura-*`-Modifikatorklasse gemäss der Naming_Konvention verwenden.

### Requirement 4: Einhaltung der CSS-Naming-Konvention

**User Story:** Als Entwickler möchte ich, dass neue Klassen der dokumentierten Namenskonvention folgen, damit das Stylesheet-System wartbar und konfliktfrei bleibt.

#### Acceptance Criteria

1. WHERE eine neue seitenspezifische Komponentenvariante eingeführt wird, THE Design_System SHALL den `.aura-*`-Präfix für deren Klassennamen verwenden.
2. THE Design_System SHALL die bestehende CSS-Klassenbenennung der In_Scope_Seite beibehalten, sofern keine Änderung zur Konsolidierung erforderlich ist.

### Requirement 5: Funktionsfähigkeit beider Themes

**User Story:** Als Nutzer möchte ich, dass sowohl das dunkle als auch das helle Theme vollständig funktionieren, damit ich die Oberfläche in meinem bevorzugten Erscheinungsbild lesbar nutzen kann.

#### Acceptance Criteria

1. WHILE das Theme_System auf `dark` steht, THE In_Scope_Seite SHALL alle Inhalte mit über das Design_System definierten Farb- und Kontrastwerten darstellen.
2. WHILE das Theme_System auf `light` steht, THE In_Scope_Seite SHALL alle Inhalte mit über das Design_System definierten Farb- und Kontrastwerten darstellen.
3. THE In_Scope_Seite SHALL Farb-, Hintergrund- und Rahmenwerte über die themenabhängigen Design-Tokens beziehen, anstatt themen-unabhängige feste Farbwerte zu verwenden.

### Requirement 6: Erhalt des bestehenden Grundstils

**User Story:** Als Nutzer möchte ich, dass die Oberfläche nach dem Feinschliff vertraut bleibt, damit die Modernisierung behutsam und nicht störend wirkt.

#### Acceptance Criteria

1. THE In_Scope_Seite SHALL den Glassmorphism-Stil, den Teal-Accent und die Dark/Light-Theme-Charakteristik des bestehenden Designs beibehalten.
2. WHERE eine visuelle Modernisierung vorgenommen wird, THE In_Scope_Seite SHALL die bestehende Seitenstruktur und den erkennbaren Grundstil bewahren.

### Requirement 7: Dropdowns für Auswahlfelder

**User Story:** Als Nutzer möchte ich für Felder mit festen Auswahlmöglichkeiten ein Dropdown statt eines Textfelds, damit die Eingabe eindeutig und fehlerfrei ist.

#### Acceptance Criteria

1. WHERE ein Eingabefeld einer In_Scope_Seite eine feste Menge an Auswahloptionen besitzt, THE In_Scope_Seite SHALL dieses Feld als Dropdown darstellen.

### Requirement 8: Modale Dialoge statt Browser-Alerts

**User Story:** Als Nutzer möchte ich Hinweise und Bestätigungen in gestalteten Dialogen sehen, damit die Oberfläche konsistent und professionell wirkt.

#### Acceptance Criteria

1. WHEN eine In_Scope_Seite eine Mitteilung, Bestätigung oder Fehlermeldung anzeigt, THE In_Scope_Seite SHALL einen modalen Dialog verwenden.
2. THE In_Scope_Seite SHALL die native `alert()`-Funktion des Browsers zur Anzeige von Mitteilungen nicht verwenden.

### Requirement 9: Übersetzungen für neue Texte

**User Story:** Als internationaler Nutzer möchte ich, dass neu hinzugefügte Texte in meiner Sprache erscheinen, damit die Oberfläche durchgängig lokalisiert ist.

#### Acceptance Criteria

1. WHEN ein neuer für Nutzer sichtbarer Text in einer In_Scope_Seite eingeführt wird, THE i18n_System SHALL für diesen Text in allen 15 unterstützten Sprachen (cs, da, de, el, en, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh) einen Übersetzungseintrag bereitstellen.
2. THE i18n_System SHALL für neu eingeführte für Nutzer sichtbare Texte sprachspezifische Übersetzungen bereitstellen, anstatt für alle Sprachen ausschliesslich den englischen Text zu verwenden.

### Requirement 10: Beschränkung auf den eingeschlossenen Bereich

**User Story:** Als Wartender möchte ich, dass der Web-Chat und der virtuelle Desktop unberührt bleiben, damit deren spezifische visuelle Verträge nicht beeinträchtigt werden.

#### Acceptance Criteria

1. THE Design_System SHALL die Dateien und Assets der Excluded_Surface nicht verändern.
2. WHEN eine Änderung am Design_System vorgenommen wird, THE Design_System SHALL das Erscheinungsbild und Verhalten der Excluded_Surface unverändert lassen.

### Requirement 11: Erhalt der bestehenden Regressionstests

**User Story:** Als Entwickler möchte ich, dass die bestehenden UI-Tests weiterhin bestehen, damit der Feinschliff keine Regressionen einführt.

#### Acceptance Criteria

1. WHEN die UI_Regressionstest_Suite nach den Änderungen ausgeführt wird, THE UI_Regressionstest_Suite SHALL ohne Fehler bestehen.
