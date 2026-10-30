# Setup-Wizard – Fehler- und UX-Audit (2026-10-30)

Geprüft: `ui/setup.html`, `ui/js/setup/main.js`, `ui/css/setup.css`, `ui/css/precision-entry.css`,
`internal/server/setup_handlers.go`, `internal/setup/setup_profiles.yaml`, i18n (`ui/lang/*/`).
Live getestet auf einer frischen Scratch-Instanz (Binary-Stand ad01ac264, neuer als alle Setup-Änderungen),
Desktop 1440×900 und Mobile 375×812, Quick-Flow (OpenRouter) und Custom-Flow bis Speichern → Login → erster Chat.

Legende: **B** = Bug (nachgewiesen), **U** = UX-/Optik-Verbesserung. Priorität P0 (sofort) bis P3.

---

## 1. Bugs

| # | Prio | Befund | Nachweis |
|---|------|--------|----------|
| B1 | P0 | **Verbindungstest meldet ✓ bei ungültigem API-Key.** Der Setup-Test prüft nur den Metadaten-Endpunkt; bei OpenRouter ist `/models` öffentlich. Mit `sk-or-v1-invalid…` erscheint grün „Der Metadaten-Endpunkt ist erreichbar“. Der Nutzer erfährt erst bei der ersten Chat-Nachricht „Der LLM-Anbieter hat die Authentifizierung abgelehnt“. | live; `setup_handlers.go:659`, `ai_gateway_handlers.go:167` |
| B2 | P0 | **Veraltete Standardmodelle im Custom-Flow.** `arcee-ai/trinity-large-preview:free` (Hauptmodell + Helper-Vorschlag) und `google/gemini-2.5-flash-lite-preview-09-2025` (Vision, Whisper) gibt es bei OpenRouter nicht mehr (Abgleich mit der Live-Modellliste, 456 Modelle). Wer im Custom-Flow nur den Key einträgt, bekommt ein nicht funktionierendes Setup. Ebenfalls veraltet: OpenAI `gpt-4o`, Anthropic `claude-sonnet-4-20250514`, Helper-Platzhalter `gpt-4o-mini`. | live; `main.js:216,231,238`, `setup.html:371-394` |
| B3 | P1 | **Enter-Taste ist global auf „Weiter“ gelegt.** Der Document-Keydown-Handler ruft bei *jedem* Enter `nextStep()` auf: Enter auf „← Zurück“ geht **vorwärts**, Enter auf „Verbindung testen“, Schritt-Punkten, Theme-Toggle, Links und Checkboxen springt ebenfalls weiter; Escape auf einem Button im OpenRouter-Modellbrowser schließt ihn **und** springt einen Schritt zurück. | live (Enter auf Zurück: step-2 → step-local-llm); `main.js:1453-1465` |
| B4 | P1 | **Setup-Token-Dialog lässt sich nicht per Enter absenden.** `preventDefault()` im globalen Handler verhindert das Form-Submit; stattdessen läuft der Wizard *hinter* dem Modal weiter (gemessen: 0 Submits, Schritt sprang auf „Sicherheit“; ein zweites Enter würde Speichern auslösen). Trifft genau die Remote/Docker-Erstinstallation. | live; `main.js:1457` |
| B5 | P1 | **„Zum Inhalt springen“-Link ist dauerhaft sichtbar** und verdeckt Logo bzw. auf Mobile den Theme-Button. `precision-entry.css` setzt `top` fest, ohne die `:focus`-Logik aus `shared-components.css` zu übernehmen. Betrifft alle `pw-entry-page`-Seiten. | live; `precision-entry.css:17-29` |
| B6 | P1 | **Scrollposition wird beim Schrittwechsel nicht zurückgesetzt.** Scroll-Container ist `#main-content`; nach „Weiter“ startet der neue Schritt mitten im Inhalt, Schrittanzeige und Überschrift sind abgeschnitten (gemessen `scrollTop` 276, Indikator bei −14 px). | live; `updateUI()` in `main.js:1355` |
| B7 | P2 | **HTML-Tags werden als Text angezeigt:** „Nutzt die nativen Tool-Calls… `<strong>Achtung:</strong>` …“. `data-i18n-html` rendert inzwischen sicher als Textknoten, der Übersetzungstext enthält aber noch Markup. | live; `setup.html:310`, `ui/lang/setup/*.json` |
| B8 | P2 | **Passwort wird beim Setup getrimmt, beim Login nicht.** `"  geheim123 "` wird als `geheim123` gespeichert; ein Passwortmanager, der den Originalwert gespeichert hat, scheitert beim Login. | `main.js:1387,1717`, `setup_handlers.go:504` vs. `auth_handlers.go:219` |
| B9 | P2 | **Fehlerursache beim Verbindungstest geht verloren.** Server liefert `code`/Fehlertext (z. B. „only allows HTTPS“, „local or private hosts“, `provider_test_busy`, CSRF/Token), der Client zeigt immer nur „Verbindung fehlgeschlagen.“. Besonders verwirrend für Ollama im LAN (`192.168.x.x` wird abgelehnt, Nutzer sieht keinen Grund). | `main.js:1044-1051`, `setup_handlers.go:642-651` |
| B10 | P2 | **Custom-Flow: Vision/Whisper hängen fest an OpenRouter.** Wählt man OpenAI/Anthropic/Google als Hauptanbieter, bleiben Vision und Whisper auf „OpenRouter“ und bekommen den Key des Hauptanbieters → garantiert 401. | `main.js:1506-1529`, `setup.html:361-388` |
| B11 | P2 | **„Überspringen“ im letzten Schritt speichert** (`nextStep(true)` → `saveConfig`) – Beschriftung passt nicht. „Überspringen ✕“ (Setup abbrechen) bleibt auch auf dem Erfolgsscreen sichtbar. | live |
| B12 | P3 | Nicht übersetzte Texte: Ribbon „Recommended“, Platzhalter „Leave empty to use main API key“, „e.g. Klingon“, Tooltips „Input / Output“/„Context“, Verbindungswarnung „Could not reach setup status endpoint…“. | `main.js:156,322,858-859`, `setup.html:255,350` |
| B13 | P3 | Wechsel Quick → Custom: Passwortfelder sind getrennt und leer (muss neu eingegeben werden); die Vertrauensstufe 3 aus dem Quick-Profil bleibt im Custom-Flow vorausgewählt (Custom-Default wäre 1). | live |
| B14 | P3 | Nach Sprachwechsel auf dem letzten Schritt setzt `applyI18N()` den Button-Text von „Speichern & Starten“ auf „Weiter“ zurück (`data-i18n="setup.nav_next"`). | `main.js:1081-1083` |
| B15 | P3 | Konsolenrauschen auf der Setup-Seite: `/api/ready` → 503 (×4), Service-Worker-Registrierung schlägt fehl (`/sw.js` → 409). | live |

## 2. UX & Optik – was den ersten Eindruck trübt

### Einstieg (Seite 1)
- **U1 (P1) Technischer HTTPS-Kasten ganz oben.** „Verbindung und Browserfunktionen“ mit drei Absätzen über sicheren Kontext ist das Erste, was ein neuer Nutzer liest – auf Mobile füllt er fast den ganzen Bildschirm. Vorschlag: nur bei *unsicherem* Kontext als kompakte Warnzeile zeigen, sonst ganz weglassen oder auf den Erfolgsscreen verschieben.
- **U2 (P1) Kein Willkommensmoment.** Kein Logo-/Hero-Bereich, kein Satz „In 2 Minuten startklar“. Vorschlag: kurze Begrüßung mit Logo, Dauer-Angabe und was man braucht („einen API-Key oder ein lokales Ollama“).
- **U3 (P1) Widersprüchliche Empfehlung.** OpenRouter-Beschreibung sagt „Empfohlen für die meisten Nutzer“, das Ribbon „Recommended“ steht aber auf dem kostenpflichtigen MiniMax-Coding-Plan. Eine klare Empfehlung (und als vorausgewählte Karte) senkt die Entscheidungslast.
- **U4 (P2) Karte „Benutzerdefiniert“ zeigt alle Features durchgestrichen** – liest sich wie „kann nichts“. Badges dort weglassen oder „alles frei konfigurierbar“ zeigen. Generell sind durchgestrichene Badges visuell laut; besser nur vorhandene Fähigkeiten zeigen.
- **U5 (P2) Kein lokaler Weg im Quick-Flow.** Ollama/„ohne Cloud“ gibt es nur im Custom-Flow mit Base-URL-Feld. Eine Karte „Lokal (Ollama)“ mit Auto-Erkennung von `localhost:11434` würde eine große Zielgruppe abholen.
- **U6 (P2) Zwei „Überspringen“-Buttons nebeneinander** (Schritt überspringen / Setup abbrechen) mit fast gleichem Label. Vorschlag: „Später einrichten“ als dezenter Link, Schritt-Überspringen nur auf optionalen Schritten.

### Schritte
- **U7 (P1) „Lokales Testmodell“ ist ein Pflichtschritt in jedem Flow** – für ein experimentelles Nischenfeature. Inhalt wirkt abschreckend: Badge = Titel (doppelt), Beschreibung doppelt, Hardware-Warnung obwohl aus, und „Q4-Evaluation: ca. 94,0 % Schema-Vertrag … das 97-%-Ziel wurde nicht erreicht“ ist Entwicklerjargon mit negativem Unterton. Vorschlag: aus dem Wizard nehmen (Hinweis auf dem Erfolgsscreen/Config) oder als einklappbarer „Erweitert“-Block im Sicherheits-/Fertig-Schritt. Quick-Flow schrumpft dann auf 3 Schritte.
- **U8 (P1) Quick-Flow verrät nicht, was eingerichtet wird.** Modell, Vision/Whisper/Embeddings des Profils sind unsichtbar. Eine kleine Zusammenfassung („Modell: Gemini 3.1 Flash Lite · Bilderkennung ✓ · Sprache ✓ – später änderbar“) schafft Vertrauen.
- **U9 (P1) Echter Key-Check mit positivem Feedback.** Statt „Metadaten prüfen“ (Jargon) ein Button „Key prüfen“, der wirklich authentifiziert (OpenRouter: `GET /api/v1/key` – kostenlos, liefert Limit/Guthaben) und zusätzlich prüft, ob das gewählte Modell in der Modellliste existiert (fängt B2 ab). Ergebnis freundlich: „✓ Key gültig – Guthaben 4,80 $“. Optional automatisch beim Verlassen des Feldes.
- **U10 (P2) Passwortfelder:** kein Anzeigen-Auge (die Login-Seite hat eins), keine Stärkeanzeige, Bestätigungsfeld ohne Live-Abgleich. Fehlertexte erscheinen erst nach „Weiter“.
- **U11 (P2) Custom-Flow überfrachtet Schritt „LLM-Verbindung“:** Titel sagt Verbindung, erste Karte ist das Admin-Passwort; Sprache wird ein zweites Mal abgefragt (schon auf Seite 1 gewählt); Base-URL und „Native Funktionsaufrufe“ sind Expertenoptionen und sollten unter „Erweitert“. Der Link zum Key ist im Custom-Flow nur Text (`<span id="provider-link">`), im Quick-Flow klickbar.
- **U12 (P2) „Native Funktionsaufrufe“ standardmäßig aus**, obwohl alle vorgeschlagenen Modelle das können – sollte je Anbieter vorbelegt sein (Quick-Profile setzen es bereits auf an).
- **U13 (P2) Vertrauensstufe:** Quick-Profile wählen Stufe 3 („Ich bin mutig“: Shell, Python, Dateisystem) vor, ohne Hinweis warum. Für Erstnutzer ist Stufe 2 mit Badge „Empfohlen“ und dem Satz „jederzeit unter Config änderbar“ der sicherere und vertrauensbildendere Default. Die Auswahl ist zudem nur an einem dünnen Rahmen erkennbar – ein Häkchen wie bei den Profilkarten wäre klarer.
- **U14 (P3) „Web-Konfigurationsoberfläche“ abschaltbar im Setup** – wer sie aus Versehen ausschaltet, sperrt sich aus der Konfiguration aus. Aus dem Wizard entfernen.
- **U15 (P3) Kern-Persönlichkeit** nur als nackte Namensliste („Friend“); eine Zeile Beschreibung oder Beispielsatz pro Persönlichkeit macht die Wahl greifbar.

### Abschluss
- **U16 (P1) Sofortiger Login nach dem Setup.** Nach „Zum Chat“ landet man auf der Login-Seite und muss das gerade gesetzte Passwort erneut eingeben. Vorschlag: beim erfolgreichen Setup direkt eine Session ausstellen (gleiches Gerät, CSRF-geschützt) – oder zumindest auf dem Erfolgsscreen ankündigen „Melde dich jetzt mit deinem Admin-Passwort an“.
- **U17 (P2) Erfolgsscreen ohne Live-Check und ohne „erste Schritte“.** „Setup abgeschlossen“ erscheint auch mit totem Key/Modell. Vorschlag: kurzer Live-Ping des Hauptmodells beim Speichern (mit klarer Meldung + „Zurück zum Key“), dazu 3 Beispiel-Prompts („Was kannst du?“, „Fasse diese Webseite zusammen“, …), die direkt in den Chat übernommen werden. Leeres Schrittanzeige-Band und HTTPS-Kasten auf dem Erfolgsscreen ausblenden.
- **U18 (zu prüfen)** Beim ersten Chat erschien eine Karte „Brain in the Machine – Audio konnte nicht geladen werden“ mit umbrechendem „Schließe/n“-Button. In der Scratch-Instanz fehlten die `media_samples`; auf einer echten Installation gegenprüfen.

### Mobile (375 px)
- **U19 (P1)** Header bricht: „Über-spri-nge-n“ vertikal umbrochen, Logo vom Skip-Link verdeckt, Schrittanzeige läuft links und rechts aus dem Bild, Labels wie „Verbindu/ng“, „Persönlic/hkeit“ werden mitten im Wort getrennt (auch auf Desktop: „Persönlichk/eit“). Vorschlag: auf schmalen Screens nur „Schritt 2 von 4 · Verbindung“ + Fortschrittsbalken; Navigation als feste Leiste unten (Zurück / Weiter), Setup-Abbrechen ins Menü.

## 3. Empfohlene Reihenfolge

1. **Sofort (klein, hohe Wirkung):** B2 (Modell-IDs aktualisieren), B3+B4 (Enter/Escape nur außerhalb interaktiver Elemente und nie bei offenem Modal), B5 (Skip-Link nur bei Fokus), B6 (`scrollTop = 0` + Fokus auf Überschrift in `updateUI`), B7, B12, B14.
2. **Kurzfristig:** B1/U9 (echter Key-Check + Modell-Existenzprüfung), B9 (Fehlergrund anzeigen), B10 (Vision/Whisper folgen dem Hauptanbieter), B8 (Trim vereinheitlichen), B11/U6 (Buttons klarer).
3. **Erster-Eindruck-Paket:** U1 + U2 + U3 (Einstieg), U7 (Local-LLM aus dem Pflichtpfad), U8, U13, U16 + U17 (Abschluss mit Auto-Login und ersten Schritten), U19 (Mobile).
4. **Später:** U4, U5 (Ollama-Karte mit Auto-Erkennung), U10–U12, U14, U15.

Ziel-Flow nach den Änderungen: **Willkommen + Sprache → Anbieter wählen → Key + Passwort (mit echtem Check) → Rechte (Empfehlung vorausgewählt) → Fertig, automatisch angemeldet, Beispiel-Prompts.**
