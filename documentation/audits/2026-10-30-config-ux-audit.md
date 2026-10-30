# Config-Bereich – UX-, Design- und Feedback-Audit (2026-10-30)

Geprüft: `ui/config.html`, `ui/js/config/*.js` (main, state, actions, catalog, presentation, integration_actions),
alle 101 Renderer in `ui/cfg/*.js`, `ui/css/config*.css`, i18n `ui/lang/config/**`, `ui/lang/help/*`,
Schema-Erzeugung `internal/server/config_handlers_providers.go` + `internal/config/config_types.go`.
Code-Stand HEAD `2686aa483`. Methode: sieben Code-Durchgänge nach fester Checkliste (Beschreibungen, Design,
Feedback, Speichermodell, Validierung, Texte, destruktive Aktionen), eigene Querschnittsmessungen und Nachprüfung
aller P1-Befunde im Code. Die Live-Messung aller Sektionen in einer Scratch-Instanz steht in Abschnitt 7.

Legende: **B** = Bug (im Code nachgewiesen), **U** = UX/Konsistenz. Priorität P0 (sofort) bis P3.
✔ = von mir im Code nachgeprüft; ohne Haken = Befund eines Code-Durchgangs, plausibel, nicht einzeln nachgeprüft.

---

## 0. Kurzfazit

Der Config-Bereich ist funktional sehr breit (124 Sektionen in 15 Gruppen), aber **kein einheitliches Produkt**:
~100 handgeschriebene Renderer mit eigenen Klassen, eigenen Status-Bannern, eigenen Ergebnisanzeigen und eigenen
Speicherwegen. Es gibt gute gemeinsame Bausteine (`AuraConfigForm.renderSpec`, `AuraConfigActions`,
`AuraConfigState`, Precision-Workspace-Layout, Seitensuche bis auf Feldebene), sie werden aber nur teilweise
genutzt. Die größten Probleme für Nutzer:

1. **Feedback ist uneinheitlich und oft unsichtbar.** Viele Aktionen melden nur per Toast (3 s, verdrängt den
   vorherigen), lassen den Server-Grund weg, haben keinen Busy-Zustand oder bleiben ganz stumm. Status-Banner
   bleiben bei ausgeschalteter Integration für immer auf „Prüfe…“ stehen.
2. **Zwei bis drei Speichermodelle nebeneinander**, ohne dass die Oberfläche sagt, was wofür gilt: globales
   Save-Dock, eigene „In Vault speichern“-Buttons (31 Module), sofort wirkende Listen (E-Mail-Konten, MCP-Server,
   Webhooks, Regeln, Personas …). Test-Buttons prüfen mal den gespeicherten, mal den eingegebenen Stand.
3. **Beschreibungen fehlen in großem Umfang.** Live gemessen: 529 von 1272 sichtbaren Steuerelementen (42 %) haben
   keine Beschreibung, in 24 Sektionen hat kein einziges eine. In den generischen Sektionen hat kein einziges der 302 Felder ein
   übersetztes Label (Anzeige „Telegram User Id“, „Readonly“ – auch in der deutschen Oberfläche), 74 haben keinen
   Hilfetext. In den eigenen Renderern fehlen Hilfetexte vor allem bei Zahlenlimits, Enable-/Readonly-Schaltern und
   in Modals.
4. **Sidebar-Struktur ist historisch gewachsen**: Anbieter erst an 4. Stelle, „Agent & KI“ mit 23 Einträgen,
   Sprache/Telefonie über 4 Gruppen verstreut, zwei Einträge heißen „MCP-Server“, Smart Home enthält LDAP.
5. **Echte Fehler** mit Datenverlust- oder Fehlbedienungsrisiko (Abschnitt 1), u. a. Überschreiben aller
   E-Mail-Konten/MCP-Server nach einem fehlgeschlagenen Laden und ein Modul-Ladefehler bei den Wächter-Sektionen.

---

## 1. Bugs

| # | Prio | Befund | Nachweis |
|---|------|--------|----------|
| B1 | P0 | **E-Mail-Konten und MCP-Server können komplett überschrieben werden.** Schlägt das Laden der Liste fehl (HTTP-Fehler/Netz), wird stillschweigend `[]` gecacht; das nächste Hinzufügen/Bearbeiten/Löschen schickt die ganze (leere) Liste per PUT und ersetzt alle bestehenden Einträge. Gleiches Muster bei MCP-Präferenzen. | ✔ `cfg/email.js:5-9,383-389`; ✔ `cfg/mcp.js:98-111,658-665` |
| B2 | P1 | **PromptSec-Wächter und LLM-Wächter kollidieren.** Beide Module deklarieren top-level `let _guardianSection`; sie werden als klassische Skripte in denselben globalen Scope geladen → das zweite geöffnete Modul wirft `SyntaxError: Identifier '_guardianSection' has already been declared`, seine Render-Funktion existiert nicht, und `renderSection` fällt still auf den generischen Schema-Renderer zurück (andere Oberfläche, ohne Test/Overrides) – bis zum Neuladen der Seite. Der Live-Sweep sah es nicht, weil er je Sektion neu lädt. | ✔ reproduziert (Node-VM, beide Skripte in einem Kontext); `cfg/guardian.js:1`, `cfg/llm_guardian.js:1`, `main.js:1660-1675,3503-3517` |
| B3 | P1 | **Status-Banner bleiben auf „Prüfe…“**, wenn die Integration aus ist, keine URL hat oder der Status unbekannt ist: AdGuard, Fritz!Box, TrueNAS, Jellyfin, MQTT, Obsidian, Composio (bei Nicht-OK). Uptime Kuma und Grafana machen es richtig (else-Zweig). | ✔ `cfg/adguard.js:14,42-44`; `fritzbox.js:17,134`; `truenas.js:17,106,130`; `jellyfin.js:17,109,137`; `mqtt.js:19,306`; `obsidian.js:144,157`; `composio.js:186` |
| B4 | P1 | **Echtzeit-Sprache: ungespeicherte Profiländerungen gehen ohne Warnung verloren.** Felder haben kein `data-path`, das Modul ruft nie `setDirty`/`AuraConfigState` – Sektionswechsel verwirft Eingaben, die Sektion sagt nicht, dass sie einen eigenen Speichern-Button hat. | ✔ `cfg/realtime_speech.js` (kein Dirty-Tracking) |
| B5 | P1 | **Secrets-Modal schließt vor dem Request.** Schlägt das Speichern fehl, ist der eingetippte Wert weg, Fehler nur als Toast. | `cfg/secrets.js:196-197,219,227` |
| B6 | P1 | **Fehler werden still verschluckt**: Webhooks (Token an/aus, Token löschen, ausgehenden Webhook löschen tun bei Nicht-OK nichts; Log bleibt auf „Lädt…“; Lade-Fehler zeigen „Keine Webhooks“), Indexierung (Rescan-Fehler ignoriert), Lokale Modelle (Quittieren ohne Fehlerpfad, Smoke-Test-/Benchmark-Ergebnis wird verworfen), MCP-Server (Token-Erzeugung nur `console.error`). | `cfg/webhooks.js:23,548-568,581,835`; `indexing.js:358-369`; ✔ `local_llm.js:383-399,414-421`; `mcp_server.js:200-202` |
| B7 | P1 | **Irreführende Zustände**: Passwort-Karte zeigt „Kein Passwort gesetzt“ + „Passwort setzen“, wenn nur der Status-Abruf scheitert (`auth.js:21-24`); MCP-Server-Toggle zeigt AUS, obwohl die VS-Code-Bridge ihn erzwingt (`mcp_server.js:8,226`); Hugging-Face-Statusfehler erscheint als „öffentlich, nur lesen“ (`huggingface.js:116`); KI-Gateway zeigt bei `setup_required` den HF-Slug-Hilfetext (`ai_gateway.js:178`); gog meldet „Fertig“, sobald die OAuth-URL da ist, vor dem Login (`gog.js:36,71`); Virtueller Desktop meldet „deaktiviert“, wenn der HTTP-Call fehlschlug (`virtual_desktop.js:171`); CYD zeigt bei jedem Flash-Fehler „Token nötig“ (`cyd.js:318`); MCP-Modal schließt und zeigt die Karte auch bei fehlgeschlagenem PUT (`mcp.js:477-481,636-654`). | Code-Durchgänge |
| B8 | P1 | **Composio speichert beim Umschalten eines Toolkits den ganzen Entwurf** per PUT – inkl. ungespeicherter Änderungen an `read_only`, `allow_destructive`, `base_url`. | `cfg/composio.js:487,494-507` |
| B9 | P1 | **Destruktive Aktionen ohne Bestätigung**: Backup-Import (überschreibt die Instanz), Security-Proxy „Zerstören“, Dograh/Lokale Modelle/Space Agent „Neu erstellen“, Selbstsigniertes Zertifikat neu erzeugen, go2rtc Stream entfernen, gog/Google-Workspace trennen, Echtzeit-Sprache Profil löschen, 3D-Drucker entfernen, A2A Skill/Agent entfernen. Dagegen bestätigen S3, Lokale Wikipedia, Fernsteuerung, Regeln, Ollama korrekt (`showConfirm`). | `backup.js:187-230`; `security_proxy.js:39,213`; `dograh.js:72`; `local_llm.js:82`; `space_agent.js:97`; `server.js:168,325`; `go2rtc.js:295`; `gog.js:72,78`; `google_workspace.js:213`; `realtime_speech.js:321`; `three_d_printers.js:167`; `a2a.js:211,253` |
| B10 | P1 | **Leeres Zahlenfeld wird als 0 gespeichert**, Ganzzahlen werden per `parseInt` gekürzt (2,7 → 2). 0 bedeutet bei mehreren Feldern „automatisch/unbegrenzt“ – Leeren ändert also still das Verhalten. Mehrere Module ersetzen umgekehrt 0 still durch Defaults (`parseInt(x)||500`: Output-Kompression, Sandbox, E-Mail, Daemon-Skills, Manus). | ✔ `state.js:149-152`; `output_compression.js:52`; `sandbox.js:103,256-275`; `email.js:263-272`; `daemon_skills.js:6-9` |
| B11 | P1 | **JSON-Textfeld für Objekt-Listen** speichert ungültiges JSON als Rohstring; der Hinweis verspricht, dass auch eine Komma-Liste geht (stimmt nicht). Leere Objekt-Listen (z. B. `agent.action_policy.allow/deny`) fallen auf ein Komma-Textfeld zurück und speichern Strings statt Objekten. | ✔ `state.js:163-164`; `main.js:2583-2590` |
| B12 | P1 | **Readonly-Schalter ohne Beschreibung** bei Docker, Home Assistant, Proxmox, Ansible: Schema-Key `readonly`, Hilfetext-Key `read_only`. | ✔ `help/en.json:93,131,183,304` |
| B13 | P1 | **Ansible-Test im Local-Modus gesperrt** („Pflichtfelder ergänzen“), weil URL+Token immer verlangt werden, obwohl sie nur im Sidecar-Modus nötig sind. | `integration_actions.js:56-61`, `config_types.go:2152` |
| B14 | P2 | **Server-Fehlertexte unescaped in `innerHTML`** (Secrets, Backup, Updates) – Darstellungsfehler, bei manipulierter Antwort HTML-Injektion. | `secrets.js:95`; `backup.js:219`; `updates.js:77-109` |
| B15 | P2 | **Updates**: nach fehlgeschlagener Installation bleibt „Installiere…“ stehen; Countdown-Text „Neuladen in 60 s… (75 s)“. | `updates.js:129,142,146`; `lang/config/updates/en.json:60` |
| B16 | P2 | **Toast-Typ `warn` ist ungestylt und wird nicht assertiv angesagt** (nur `warning` existiert): TTS, Sprachlabor, Bluetooth, Netzwerkfreigaben. | ✔ `shared-core.js:736-751`, `precision-pages.css:868` |
| B17 | P2 | **Doppelte Steuerelemente für denselben Pfad**: `agent.allow_unsafe_host_execution` und `agent.action_policy` erscheinen in „KI-Agent“ (generisch) **und** in der Gefahrenzone. | `main.js:1681-1700`, `danger.js:40,154` |
| B18 | P2 | **Unsichtbare Einstellungen**: `circuit_breaker.llm_per_attempt_timeout_seconds`, `llm_stream_chunk_timeout_seconds`, `final_retry_interval` haben nirgends ein Feld; `sqlite.long_term_path` ist veraltet, wird aber normal editierbar angezeigt. | `main.js:1614`; `config_types.go:1231,1467-1472` |
| B19 | P2 | **Auto-Änderungen ohne Dirty-Markierung**: Video-Download schreibt bei jedem Rendern Defaults inkl. `enabled=true` in den Entwurf; YepAPI wählt den Anbieter automatisch; EvoMap schreibt `node_id`; A2A speichert Platzhalter als echte Werte. | `video_download.js:7-22`; `yepapi.js:42`; `evomap.js:139`; `a2a.js:33,43` |
| B20 | P2 | **Sandbox-Systemstatus** wird einmal pro Seitenaufruf gecacht – nach Installation/Aktivierung bleibt „nicht bereit“ stehen. Vercel hat (anders als Netlify) keinen `config-saved`-Listener → alter Status nach dem Speichern. | `sandbox.js:4-10`; `vercel.js:6-11` |
| B21 | P2 | **Login-Schutz-Schalter** sieht gesperrt aus (`auth-toggle-wrap-disabled` ohne CSS-Regel), ist aber klickbar; Hilfetext sagt, er könne nicht deaktiviert werden. | `auth.js:56-62` |

Verworfen nach Nachprüfung: „Firewall-Schalter auf Nicht-Linux nicht gesperrt“ (das `disabled`-Expando auf dem div wird
vom Guard korrekt gelesen) und „`cfgMaskedSecretFallback` in evomap undefiniert“ (globales `const` aus main.js).

---

## 2. Feedback – sieht der Nutzer sofort, ob etwas funktioniert?

**Gemessen (Code):** ~40 eigene Klassen nur für Test-Ergebnisse (`a2a-test-result`, `fb-test-result`,
`paperless-test-status`, `rc-feedback` …), 37 verschiedene `*-banner`-Klassen, 37 Module melden (auch) per Toast.
`AuraConfigActions` vereinheitlicht nur Bereitschaft und Busy-Zustand von Test-Buttons, **nicht das Ergebnis**.

- **U1 (P1) Toast als einziger Kanal.** Vault-Speichern in ~15 Modulen, Zertifikats-Upload, Persona-/Regel-Speichern,
  SQL-Verbindungstest, Webhook-Test, Security-Proxy-Aktionen, Tailscale Start/Stopp, Composio-Listen, Chromecast-Geräte.
  Toasts verschwinden nach 3 s, ein neuer verdrängt den alten, sie stehen weit weg vom Button.
- **U2 (P1) Server-Grund geht verloren.** Generisches „fehlgeschlagen“ statt der Server-Meldung bei: LLM-Router-Vorschau,
  Decisions, S3 (Grund nur im eingeklappten Detail), WebDAV/Koofr-Vault, Tailscale, Chromecast-Gerätetest,
  3D-Drucker, Hugging Face (nur im Toast), Wächter-Download, Virtueller Desktop (Video/Retro), Skill-Manager,
  Daemon-Skills, MeshCore, CYD, A2A-Servertest, Netzwerkfehler im generischen Integrationstest.
- **U3 (P2) Kein Busy-Zustand** (Doppelklick möglich, keine Rückmeldung): Vault-Speichern fast überall, Zertifikat,
  Passwort/TOTP, Echtzeit-Sprache Test/Stimmen, Dograh MCP/Webhook (falsche Button-IDs `dograh.js:113,119,228`),
  Lokale Modelle, Regeln, Personas, Webhooks POST/PUT, Bluetooth-Gerätaktionen, CYD, go2rtc, Tailscale,
  Newspaper-Readiness, Skill-Manager, Indexierung-Rescan.
- **U4 (P2) Kein Live-Status beim Öffnen**: LDAP, Paperless, GitHub, Updates (Version erst nach Klick), Browser-Automation,
  Space Agent, Video-Download (gar keine Prüfung von yt-dlp/Image), generische Integrationen (Telegram, Discord,
  Home Assistant, Proxmox …). Die Sidebar zeigt nirgends, welche Integration aktiv oder gestört ist.
- **U5 (P2) Erfolg ohne Meldung**: SQL-Verbindung speichern (Modal schließt still), Security-Fixes „Alle beheben“,
  Lokale Modelle „Rolle anwenden“ (Seite lädt neu), Persona aktivieren, Preset anwenden (MCP-Server), Chromecast
  löschen, E-Mail-Konten speichern.
- **Gute Vorbilder zum Standardisieren:** Inline-Statuskasten pro Karte `authSetStatus` (`auth.js:6-13`),
  Aktionsmuster mit Busy, Inline-Banner und Server-Grund (`dograh.js:226-258`, `cloudflare_tunnel.js:360-389`,
  `speech_lab.js:162-180`), sichtbare Sperrgrund-Zeile unter deaktivierten Buttons (`meshcore.js:96-116`,
  `s3.js:160-189`, `ruview.js:146-158`), Zustandsmaschine mit Polling (`guardian.js:180-280`), generischer
  Integrationstest (`integration_actions.js:101-116`, zeigt Server-Meldung).

## 3. Speichermodell – ist klar, was wann gespeichert wird?

- **U6 (P1) Drei Wege, keine Kennzeichnung.** (a) Globales Save-Dock für `data-path`-Felder; (b) eigene
  „In Vault speichern“-Buttons für Secrets in 31 Modulen (presentation.js klebt nachträglich einen Hinweis an,
  `presentation.js:121-131`); (c) sofort wirkende Listen/Aktionen: E-Mail-Konten, MCP-Server, SQL-Verbindungen,
  Webhooks, Regeln, Personas, Indexierungs-Ordner, Netzwerkfreigaben, Fernsteuerungs-Geräte, Zertifikat,
  Passwort/TOTP, Decisions „Einrichten“, Speech-Lab-Stack, SIP-Wizard. Nur gog, KI-Gateway und MCP-Server sagen es.
  Ziel: Secrets laufen wie jedes andere Feld über das Save-Dock (Server schreibt sie in den Vault); sofort wirkende
  Bereiche bekommen einen einheitlichen Kasten „Änderungen hier werden sofort übernommen“.
- **U7 (P1) Test-Buttons ohne einheitliche Regel.** Prüfen gespeicherten Stand still (Bild/Video/Musik, Koofr,
  WebDAV, Netlify, Vercel, here.now, AdGuard, MQTT, LDAP, Tailscale, Uptime Kuma, Grafana, TrueNAS, Jellyfin),
  schicken den Entwurf (Medienkonvertierung, Homepage, Fritz!Box, 3D-Drucker) oder sperren bis zum Speichern
  (Dokument-Ersteller, gog, Ollama, treg, generisch über `requiresSaved`). Empfehlung: **Test mit den eingegebenen
  Werten** als Standard (Server testet den Entwurf, ohne zu speichern) – das ist der natürliche Ablauf
  „eintragen → testen → speichern“. Wo das technisch nicht geht: Button sichtbar sperren mit Grund-Zeile.
- **U8 (P2) Neustart-Bedarf** wird nur nach dem Speichern über das Server-Flag gemeldet (gut gelöst: Warnung + Link,
  `main.js:3135-3139`), aber keine Sektion kennzeichnet vorab, welche Felder einen Neustart brauchen.
- **U9 (P2) Zwei Dirty-Quellen** (`isDirty` global vs. `AuraConfigState`) in Speech Lab; Telefonagent und SIP haben
  eigenen Speichern-Button **und** Dock.

## 4. Beschreibungen und Verständlichkeit

**Generische Sektionen** (Schema-Renderer): 302 Felder, **0 übersetzte Labels** (Anzeige über `formatKey`, englisch,
auch bei `ui_language: de`), **74 ohne Hilfetext**:

| Sektion | Felder | ohne Hilfe | Sektion | Felder | ohne Hilfe |
|---|---|---|---|---|---|
| sqlite (Datenbanken) | 26 | 24 | frigate | 11 | 11 |
| directories | 7 | 7 | agent | 24 | 7 |
| optimizations | 40 | 5 | notifications | 7 | 4 |
| llm | 8 | 3 | proxmox | 8 | 3 |
| ansible | 12 | 3 | network_tools | 7 | 2 |
| rocketchat, docker, logging, personality, home_assistant | je 1–8 | je 1 | übrige 13 | – | 0 |

Weitere Punkte:
- **U10 (P1) Generische Labels** in allen 16 Sprachen nachziehen (`config.<pfad>_label`) – das ist der größte
  sichtbare Konsistenzbruch: Custom-Sektionen haben echte deutsche Labels, generische zeigen „Max Upload Size Mb“.
- **U11 (P2) Fehlende Hilfetexte in Custom-Renderern** (Auswahl): Server (Host, Port, Sprache, HTTPS-Port, Domain,
  Zertifikatspfade, behind_proxy), SQL-Verbindungen (alle Modal-Felder, SSL-Modi unerklärt), Security-Proxy (Domain,
  Ports, Rate-Limit, IP-Filter), LLM-Wächter (fast alle), Echtzeit-Sprache, Telefonagent-Limits, Lokale Modelle
  (alle Selects), Co-Agenten-Limits, Regeln-Editor, Game Maker/CYD Creator/Detective/Newspaper/Flows-Limits,
  Virtueller Desktop (Video Studio, Retro), TTS (ElevenLabs, Supertonic, Piper), Bild/Video/Musik-Generierung,
  Homepage (vorhandene `help.homepage.*` werden nie gerendert), Sandbox (vorhandene `help.sandbox.*` nie gerendert),
  Web-Scraper und Tailscale (Enable-Hilfe existiert, wird nicht gerendert), MeshCore, SIP (~40 Expertenfelder),
  go2rtc, RuView, Fernsteuerung (inkl. sicherheitsrelevantem `ssh_insecure_host_key`), Fritz!Box-Features,
  E-Mail-Konto-Modal, MCP-Modal (inkl. `allow_private_network`, `allow_destructive`), Webhook-Editor, A2A,
  KI-Gateway, gog/Google Workspace/GitHub/YepAPI/treg Enable-Schalter.
- **U12 (P2) Muster „Hilfe nur im Platzhalter“**: TOTP-Felder, Security-Proxy-Routen, Webhook-Feldmapping,
  A2A-Skills, Manifest-Header – Platzhalter verschwinden beim Tippen und sind kein Label.
- **U13 (P2) Begriffe ohne Erklärung bzw. verwechselbar**: „Google Workspace“ vs. „Google Workspace über gog“
  (gleiches Icon, gog wird nirgends erklärt); zwei Einträge „MCP-Server“; „Decisions“, „Manifest“, „OmniRoute“,
  „Dograh“, „treg“, „EvoMap“ ohne Satz, wofür man es braucht; RuView-Beschreibung ist ein RSSI/CSI-Disclaimer.
- **U14 (P2) Hartkodierte Texte** (nicht über `t()`): u. a. „SANs:“, „Key“/„system“ (Secrets), „commit(s)“,
  „Request failed“/„Test failed“ (SQL), Security-Proxy-Meldungen, LLM-Wächter-Tool-Beschreibungen (englische Map,
  `config.llm_guardian.tool_*` existiert in keiner Sprache), TTS-Abschnittstitel, Tailscale „URL:/DNS:/IPs:/Cert:“,
  „Enable HTTPS →“, „Gemini Live“, „Jeff-2B · Q4_K_M“, „RAM:“, „USD“, „Obsidian REST API v“, Einheiten GB/MB/s.
- **U15 (P3) `t('a') || t('b')`-Fallbacks greifen nie**, weil `t()` bei fehlendem Key den Key zurückgibt
  (composio, github, skill_manager, paperless-Guards) → potenziell Rohschlüssel in der Oberfläche.

## 5. Design-Konsistenz

- **U16 (P1) Form-Builder kaum genutzt.** `AuraConfigForm.renderSpec` (`cfg/form-builder.js`) erzeugt einheitliche
  Gruppen, Felder, Toggles, Passwortzeilen – genutzt von 13 von ~100 Modulen (a2a, adguard, agentmail, co_agents,
  evomap, gog, google_workspace, grafana, koofr, meshcore, ruview, treg, webdav). Der Rest baut HTML-Strings.
- **U17 (P2) AdGuard-Klassen als De-facto-Standard**: 43 Module nutzen `adg-*` (`adg-status-banner`, `adg-save-btn`,
  `adg-test-result`). Daneben eigene Familien: `sb-card` (Sandbox **und** SQL), `bck-`, `wh-`, `idx-`, `fw-`, `ma-`,
  `auth-`, `srv-`, `mcp-srv-`, `cmp-`, `gw-`, `cfgdev-`, `fb-feature-card`, `cfg-card` …
- **U18 (P2) Steuerelemente uneinheitlich**: native Checkboxen statt `.toggle` (Echtzeit-Sprache, Gedächtnisanalyse,
  PromptSec-Injection-Guard, Backup, S3-Rechte); WebAuthn „aktiviert“ als Select; `role=switch`-divs ohne
  Tastaturbedienung (LLM-Router, Decisions, Gefahrenzone); „Aktiv/Inaktiv“-Beschriftung nur in einem Teil der Module.
- **U19 (P2) Enable-Schalter nicht oben**: Server (HTTPS nach Vorschau-Block), Output-Kompression, Sandbox,
  Cloudflare Tunnel, Video-Download, Manus, Hugging Face, Composio, Lokale Wikipedia, SIP (im eingeklappten
  Expertenbereich).
- **U20 (P3) Emoji als Abschnitts-Icons** in fast allen Modulen (🔌 📊 💾 🎬 ⚙️ …), während Sidebar und Buttons
  SVG-Icons nutzen (presentation.js ersetzt nur Emoji am Button-Anfang). Icon-only-Buttons mit Emoji und nur
  `title` (TTS 💾 ohne Label, Secrets, Provider, Vault-Löschen).
- **U21 (P3) „Erweitert“-Einklappen** existiert (`enhanceConfigSectionLayout`), ist aber nur für 3 Felder
  klassifiziert (`catalog.js` `sectionTiers`) – Expertenoptionen stehen gleichrangig neben Grundeinstellungen
  (z. B. 51 flache Zeilen in „Tool-Berechtigungen“, 40 in „Optimierungen“, 26 Pfade in „Datenbanken“).
- **Positiv:** Inline-`style=""` in `ui/cfg` ist vollständig bereinigt, native `alert/confirm` nur noch in
  `tailscale.js` (2×), Feldsuche in der Sidebar bis auf Feldebene, Übersicht mit „Zuletzt geöffnet“,
  Unsaved-Changes-Dialog beim Sektionswechsel, Feldfehler nach dem Speichern pro Feld.

## 6. Validierung

- **U22 (P2)** Nur 12 Pfade haben Regeln in `catalog.js` `validationRules`; die vorhandene Regelart `url` wird von
  keinem Eintrag genutzt. Alle übrigen Zahlen haben nur `type=number` (oft ohne min/max/Einheit/Default),
  URLs gar keine Prüfung. Fehler erscheinen erst beim Speichern, nicht inline. Gute Vorbilder: Cloudflare-Port
  (`AuraConfigState.validate`, inline), treg-Kosten (`role=alert`, `aria-invalid`), Provider-`fieldError`,
  SIP-Feldfehler mit Fokus.
- **U23 (P2)** Stilles Verwerfen von Eingaben: Detective-Allowlist (Zeilen ohne „:“), MCP-Env `KEY=VALUE`,
  Webhook-Header, Indexierung leerer Pfad, gog 20-Konten-Limit, E-Mail-ID wird still bereinigt.

---

## 7. Live-Messung (Scratch-Instanz)

Scratch-Instanz aus `git archive` von `a21ba3a3d`, `ui_language: de`, Standard-Template (fast alles aus/leer),
rod-Runner öffnet jede Sektion einzeln (Seite je Sektion neu geladen), 1440×900 plus Mobile 390×844.
Rohdaten außerhalb des Repos: `%TEMP%\agcfg\out\sweep.json`, `summary.md`, 131 Screenshots in `out\shots\`.

**Kennzahlen**

| Messgröße | Wert |
|---|---|
| Sektionen / Gruppen | 124 / 15 (+ Übersicht) |
| sichtbare Steuerelemente | 1272 |
| davon **ohne sichtbare Beschreibung** | **529 (42 %)** in 67 Sektionen |
| Sektionen, in denen **kein** Steuerelement eine Beschreibung hat | 24: providers, realtime_speech, telephone_agent, llm_router, local_llm, decisions, memory_analysis, rules, directories, indexing, web_scraper, sandbox, image_generation, music_generation, video_generation, document_creator, homepage, gog, yepapi, sip, remote_control, security_proxy, llm_guardian, danger_zone |
| Steuerelemente ohne zugänglichen Namen | 44 in 14 Sektionen (u. a. danger_zone 12, indexing 8, heartbeat 6, mcp_server 3, composio 3) |
| Rohe i18n-Keys / „undefined“ / „NaN“ sichtbar | keine |
| Render-Fehler / Timeouts | keine; Median 715 ms, max. 1963 ms (game_maker) |
| Enable-Schalter vorhanden / an erster Stelle | 107 / 98 (nicht oben: optimizations, llm, decisions, embeddings, personality, server, info_tools, tts, danger_zone) |
| Buttons mit Emoji am Anfang | 7 Sektionen (backup_restore 3, google_workspace 2, updates, tts, chromecast, bluetooth, ansible) |
| Inline-`style` im DOM | 5 (embeddings 2, firewall 2, danger_zone 1) |

Meiste Felder ohne Beschreibung: telephone_agent 96/96 · mcp_server 78/81 · sip 47/47 · llm_router 24/24 ·
sqlite 24/26 · meshcore 15/20 · detective 14/15 · fritzbox 14/22 · a2a 12/19 · rules 11/11 · sandbox 11/11 ·
local_llm 10/10 · output_compression 9/22 · virtual_desktop 9/39 · newspaper 8/19 · indexing 8/8 · agent 7/24 ·
directories 7/7. Beispiel generischer Renderer: „Datenbanken“ zeigt 26 Pfadfelder mit englischen Pseudo-Labels
(„Short Term Path“, „Cyd Creator Path“), nur 2 haben eine Hilfe, `long_term_path` ist veraltet (B18).

**Fehler beim bloßen Öffnen** (Standardkonfiguration):
- Sprachlabor: 6× HTTP 502 (`/api/speech-lab/capability`, `…/catalog`).
- Daemon-Skills: 503 auf `/api/daemons`.
- Webhooks: 404 auf `/api/webhooks/presets` – die Route wird nur registriert, wenn der Webhook-Manager läuft
  (`server_routes_config.go:425-436`); das Modul behandelt Fehler als leere Liste und zeigt „Keine Webhooks“ statt
  „Webhooks sind deaktiviert“ (passt zu B6).

**Test-Button-Probe** (40 Buttons mit leerer Konfiguration geklickt):
- 33 sind korrekt gesperrt und zeigen einen Grund – aber immer nur generisch („Fülle zuerst die erforderlichen
  Felder aus und speichere sie.“ bzw. „Speichere zuerst die erforderlichen Zugangsdaten im Vault.“). **U24 (P2):**
  der Grund nennt nie, *welches* Feld fehlt; `catalog.js` kennt die Pfade bereits (`requiredPaths`).
- 6 liefern Inline-Feedback; davon zeigen zwei Rohtexte vom Server: Bluetooth „generate Bluetooth test tone: exec:
  "ffmpeg": executable file not found in %PATH%“, CYD „Test fehlgeschlagen cyd is disabled“ (**U25 (P2)**:
  Server-Codes übersetzen statt Go-Fehler durchzureichen).
- KI-Gateway-Modelltest ist nativ deaktiviert **ohne** Grund-Zeile.
- Kein nativer Dialog, kein Fall „gar nichts passiert“.

**Speichern-Ablauf** (Sektion KI-Agent):
- Ohne Änderung: Leiste sichtbar, „Speichern“ deaktiviert, „0 geänderte Felder“.
- Nach einer Änderung: „1 geänderte Felder“ (**U26 (P3)**: Singular fehlt), Pille „Ungespeicherte Änderungen“.
- Nach dem Speichern: „Konfiguration gespeichert und sofort angewendet.“ – klar und gut. **U27 (P3):** die grüne
  Meldung bleibt stehen, auch wenn danach wieder etwas geändert wird (steht dann neben „Ungespeicherte Änderungen“).

**Sidebar/Mobile:** Feldsuche funktioniert gut („port“ 37 Treffer, „timeout“ 39, „api key“ 13). Mobile 390 px ohne
horizontales Überlaufen, Drawer 330 px breit.

---

## 8. Sidebar / Informationsarchitektur

**Ist:** 124 Einträge in 15 Gruppen (Agent & KI 23 · Agent-Tools 19 · Netzwerk & Remote 15 · Server & System 11 ·
Externe KI 10 · Smart Home 9 · Medien 9 · Produktivität 6 · Web-Publishing 5 · je 4 Messenger/Benachrichtigungen/
Sicherheit · Cloud-Speicher 3 · Container 1 · Gefahrenzone 1).

- **IA1 (P1)** Anbieter (Pflichtschritt jeder Installation) steht an 4. Stelle; „LLM Einstellungen“ an 10., getrennt
  durch Echtzeit-Sprache, Telefonagent, Manifest, OmniRoute, Dograh.
- **IA2 (P1)** „Agent & KI“ mischt LLM-Infrastruktur, Agent-Verhalten und Sprache (23 Einträge).
- **IA3 (P2)** LLM-Quellen/Gateways verteilt: Manifest, OmniRoute, Lokale Modelle unter „Agent & KI“; KI-Gateway und
  Ollama („Lokale LLM-Verwaltung“!) unter „Externe KI“.
- **IA4 (P2)** Sprache/Telefonie über 4 Gruppen: Echtzeit-Sprache + Telefonagent, Sprach-zu-Text + Sprachausgabe +
  Sprachlabor (Medien), SIP-Telefon (Netzwerk & Remote), Telnyx (Messenger).
- **IA5 (P2)** Zwei Einträge „MCP-Server“; AuraGo-als-MCP-Server steht unter „Externe KI“.
- **IA6 (P2)** „Smart Home“ enthält LDAP/AD, AdGuard, Fritz!Box; „Netzwerk & Remote“ enthält Jellyfin, Grafana,
  Uptime Kuma, 3D-Drucker, Kameras, SIP und die Host-Netzwerkfreigaben.
- **IA7 (P2)** Sicherheit zersplittert: Login & 2FA und Firewall unter „Server und System“, Proxy/Wächter/VirusTotal
  unter „Sicherheit“.
- **IA8 (P2)** „Agent-Tools“ mischt Tool-Schalter mit kompletten Apps (Game Maker, CYD Creator, Detective, Newspaper,
  Desktop, Flows, Virtuelle Computer); CYD und CYD Creator in verschiedenen Gruppen.
- **IA9 (P3)** Gruppe „Container“ nur mit Docker; „Benachrichtigungen“ enthält E-Mail/AgentMail/Webhooks und einen
  gleichnamigen Eintrag.
- **IA10 (P2)** Reihenfolge innerhalb der Gruppen = Entstehungsreihenfolge (weder Wichtigkeit noch Alphabet).
- **IA11 (P2)** Sprachmix in DE-Labels: „Decisions“, „Detective“, „Newspaper“, „Logging“, „Embeddings“, „Vision“,
  „Space Agent“; Kurzbeschreibungen mal 2 Stichworte („Log-Einstellungen“), mal ganze Sätze mit Punkt.
- **IA12 (P1) Kein Zustand in der Sidebar** (aktiv / eingerichtet / Fehler / Neustart nötig). Bei 124 Einträgen sieht
  man nicht, was eingeschaltet ist. Vorschlag: kleiner Punkt je Eintrag (grün aktiv, gelb eingerichtet aber aus/
  ungetestet, rot Fehler) + Filter „Nur aktive“; die Übersicht zeigt aktive Integrationen und Probleme.
- **IA13 (P3)** Die Emoji-`icon`-Felder in `SECTIONS` werden in der Sidebar nicht mehr gerendert (SVG-Sprite je Key),
  aber noch an 6 Form-Builder-Sektionen durchgereicht → Pflegefalle.

**Vorschlag Zielstruktur** (14 Gruppen, innerhalb nach Wichtigkeit):

1. **KI-Modelle** – Anbieter · LLM-Einstellungen · LLM-Router · Fallback-LLM · Lokale Modelle · Ollama · Embeddings ·
   Vision · Budget · Gateways (KI-Gateway, Manifest, OmniRoute)
2. **Agent-Verhalten** – KI-Agent · Prompts & Personas · Persönlichkeit · Regeln · Heartbeat · Co-Agenten ·
   Gedächtnisanalyse · Missionsvorbereitung · Entscheidungen (Decisions) · Optimierungen · Output-Kompression
3. **Sprache & Telefonie** – Sprach-zu-Text · Sprachausgabe · Sprachlabor · Echtzeit-Sprache · Telefonagent ·
   SIP-Telefon · Telnyx
4. **Agent-Werkzeuge** – Tool-Berechtigungen · Info-Tools · Netzwerk-Tools · Brave Suche · Web-Scraper ·
   Browser-Automation · Lokale Wikipedia · Sandbox · Skill-Manager · Daemon-Skills
5. **Externe Tool-Quellen** – MCP (externe Server) · Composio · Hugging Face · Manus · treg · EvoMap · YepAPI
6. **Apps & Arbeitsbereiche** – Virtueller Desktop · EasyDrag-Flows · Virtuelle Computer · Game Maker · CYD Creator ·
   Detektiv · Zeitung · Space Agent · Homepage
7. **Medien** – Bild-, Musik-, Videogenerierung · Medienkonvertierung · Video & YouTube · Dokument-Ersteller
8. **Kommunikation** – Telegram · Discord · Rocket.Chat · E-Mail · AgentMail · Push-Benachrichtigungen · Webhooks
9. **Produktivität** – GitHub · Google Workspace (gog als „Zusätzliche Google-Konten“ direkt darunter) ·
   Paperless-ngx · Obsidian
10. **Speicher & Veröffentlichen** – S3 · WebDAV · Koofr · Netlify · Vercel · here.now · Cloudflare Tunnel
11. **Smart Home & Geräte** – Home Assistant · MQTT · Cheap Yellow Display · Chromecast · Bluetooth · MeshCore ·
    3D-Drucker · Kameras (go2rtc, Frigate) · RuView
12. **Infrastruktur & Monitoring** – Docker · Proxmox · TrueNAS · Tailscale · Fritz!Box · AdGuard Home · Uptime Kuma ·
    Grafana · Jellyfin · MeshCentral · Ansible · Fernsteuerung · LDAP/AD · SQL-Verbindungen
13. **Vernetzung & Sicherheit** – Web-Login & 2FA · Security-Proxy · Firewall · PromptSec-Wächter · LLM-Wächter ·
    VirusTotal · AuraGo als MCP-Server · A2A
14. **System** – Server · Verzeichnisse · Datenbanken · Logging · Wartung · Indexierung · Netzwerkfreigaben ·
    Sichern & Wiederherstellen · Updates
— **Gefahrenzone** (wie bisher abgesetzt am Ende)

Die Gruppenreihenfolge folgt dem Einrichtungsweg: erst Modell, dann Verhalten, dann Fähigkeiten, dann Anbindungen,
zuletzt System. Sektions-Keys und URLs (`#providers` …) bleiben unverändert; nur `SECTIONS` und die Gruppenlabels
ändern sich.

---

## 9. Empfohlene Umsetzung

**Welle 0 – Bugs (klein, sofort):** B1–B13, B15–B17, B21. Je Fix ein gezielter Test. B1/B5/B8 zuerst (Datenverlust).

**Welle 1 – Gemeinsame Bausteine** (in `ui/js/config/`, dann Module darauf umstellen):
1. `cfgActionResult(button, promise)`: Busy (disabled + „Läuft…“), Ergebnis-Zeile direkt am Button mit
   Erfolg/Fehler **inkl. Server-Grund**, `role=status`; Toast nur zusätzlich. Ersetzt die ~40 `*-test-result`-Varianten.
2. `cfgStatusBanner(state)` mit festen Zuständen: aus · nicht eingerichtet · prüfe · verbunden · Fehler (Grund) ·
   unbekannt (mit „Erneut prüfen“). Ersetzt die 37 Banner-Klassen und behebt B3 strukturell.
3. Secret-Felder ins Save-Dock (Server schreibt in den Vault) – entfernt den zweiten Speicherweg in 31 Modulen.
   Wo sofort gespeichert wird: einheitlicher Hinweis-Kasten.
4. Einheitliche Testregel „mit eingegebenen Werten testen“ (Server-Endpunkte nehmen den Entwurf), sonst sichtbare
   Sperrgrund-Zeile.
5. `showConfirm` für alle destruktiven Aktionen (Liste B9) und für Schalter wie `allow_destructive`/`allow_delete`.
6. Toast-Typ `warn` → `warning` angleichen; Fehler-Toasts länger stehen lassen.

**Welle 2 – Texte:** `config.<pfad>_label` für alle 302 generischen Felder in 16 Sprachen; fehlende Hilfetexte
(Abschnitt 4, Tabelle + U11); vorhandene, nicht gerenderte Hilfen anschließen (homepage, sandbox, scraper,
tailscale, tts); `readonly`-Keys; Kurzbeschreibungen der Sidebar auf ein Muster bringen (ein Satz, was es tut);
hartkodierte Texte (U14). Ein Guard-Test „jedes sichtbare Steuerelement hat Label + Beschreibung“ (Browser-Test
über alle Sektionen) verhindert Rückfälle.

**Welle 3 – Sidebar:** Zielstruktur (Abschnitt 8), Zustandspunkte + Filter „Nur aktive“, Übersicht mit
aktiven/gestörten Integrationen.

**Welle 4 – Design-Angleichung:** Module schrittweise auf `AuraConfigForm.renderSpec` + gemeinsame Klassen
(`cfg-*` statt `adg-*`) umstellen; Enable-Schalter immer oben; `.toggle` statt nativer Checkboxen; Emoji-Abschnitts-
icons durch SVG ersetzen; `sectionTiers` für Expertenfelder befüllen; Validierungsregeln (URL, Bereiche, Einheiten)
für die generischen Zahlen- und URL-Felder.
