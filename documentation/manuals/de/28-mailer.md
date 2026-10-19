# Kapitel 28: Mailer – E-Mail auf dem Desktop

Mailer ist die E-Mail-App in der Kategorie Kommunikation des Desktops. Es zeigt deine Postfächer (IMAP oder POP3 mit SMTP,
Google, Microsoft und AgentMail) in einem Fenster und synchronisiert sie im Hintergrund, auch wenn das Fenster geschlossen
ist. Es ist für das Lesen und Schreiben deiner eigenen Post da, nicht für einen Kalender. PGP und S/MIME unterstützt es nicht.

Technische Details für Admins (Konfiguration, OAuth-Callback-Adresse, Speicherung, API-Routen) stehen in der
[Betriebsreferenz](../../mailer.md) (englisch).

## Inhaltsverzeichnis
1. [Was du brauchst](#was-du-brauchst)
2. [Die ersten Schritte](#die-ersten-schritte)
3. [Konten verbinden](#konten-verbinden)
4. [Nachrichten lesen und ordnen](#nachrichten-lesen-und-ordnen)
5. [Verfassen und senden](#verfassen-und-senden)
6. [Schreibhilfe](#schreibhilfe)
7. [Synchronisierung im Hintergrund und Agentenzugriff](#synchronisierung-im-hintergrund-und-agentenzugriff)
8. [Gut zu wissen](#gut-zu-wissen)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Was du brauchst

Mailer ist eine eingebaute Desktop-App ohne eigenen Schalter in der Konfiguration. Sie ist da, solange der virtuelle
Desktop eingeschaltet ist, und sie hängt von diesen Einstellungen ab:

- **Config → Virtueller Desktop → „Virtuellen Desktop aktivieren“** (`virtual_desktop.enabled`; die mitgelieferte
  `config_template.yaml` schaltet ihn ein, eine Config ohne den Schlüssel lässt ihn aus). Ist der
  Desktop aus, lehnt Mailer alle Änderungen ab.
- **Mailer → Einstellungen → Konten → „Im Hintergrund synchronisieren“**. Das ist Mailers eigener Ein-/Ausschalter, auf
  neuen Installationen ist er aus. Solange er aus ist, synchronisiert Mailer nicht, lädt keine Nachrichtentexte nach und
  lehnt Änderungen und Versand ab. Konten kannst du trotzdem verbinden. Der Wert steht in Mailers Datenbank, nicht in
  `config.yaml`.
- **Config → Virtueller Desktop → „Schreibgeschützter Modus“** (`virtual_desktop.readonly`). Die Post bleibt lesbar, aber
  Entwürfe, Regeln, Lesemarkierungen, Kontoänderungen und Versand sind gesperrt.

Jede Kontoart hat eigene Voraussetzungen:

- **Klassische IMAP- oder POP3-Konten** brauchen Serverangaben und ein Passwort. Unverschlüsselt geht nicht; nutze TLS
  oder STARTTLS; siehe [E-Mail (IMAP/SMTP) Konfiguration](./08-integrations.md#e-mail-imapsmtp-konfiguration).
- **Google-Konten** brauchen die gog-Integration (**Config → Google Workspace über gog**) oder **Config → Google Workspace**
  mit Mail-Berechtigungen. Gmail-API-Konten brauchen außerdem **Config → Gefahrenzone → „Netzwerk-Anfragen
  (api_request)“** (`agent.allow_network_requests`). Dieser Schalter erlaubt dem Agenten auch beliebige HTTP-Anfragen. Siehe [Google Workspace über gog](./08-integrations.md#google-workspace-über-gog).
- **Microsoft-Konten** brauchen deine eigene Microsoft-App-Registrierung (Client-ID und Client-Secret).
- **AgentMail** braucht **Config → AgentMail** mit eingeschaltetem **Aktiviert** und einer primären Postfach-ID. Mailer
  verbindet dieses Postfach, es legt keines an. Siehe [AgentMail Integration](./08-integrations.md#agentmail-integration).

## Die ersten Schritte

1. Öffne **Mailer** im Startmenü, in der Kategorie **Kommunikation**. Das Fenster öffnet sich maximiert.
2. Öffne **Einstellungen**, wähle den Reiter **Konten** und schalte **Im Hintergrund synchronisieren** ein.
3. Lege ein Konto mit **E-Mail-Konto hinzufügen**, **Google-Konto hinzufügen** oder **Microsoft-Konto hinzufügen** an, oder
   verbinde ein vorhandenes unter **Vorhandenes Konto verbinden**.
4. Starte unter **Verbundene Konten** **Empfang testen** und **Versand testen**. Die Tests melden sich an und prüfen, was
   das Konto kann. Sie verschicken keine Nachricht.
5. Die Liste zeigt alle Posteingänge, dazu **Ungelesen**, **Markiert**, **Geschlummert**, **Entwürfe** und **Postausgang**.

## Konten verbinden

Eine Quelle ist ein Konto, das eine Integration bereits kennt: ein klassisches Konto, ein GoG-Konto, ein
Google-Workspace-Konto oder das AgentMail-Postfach. Verbinden kopiert keine Passwörter oder Tokens nach Mailer.

### Klassische IMAP- oder POP3-Konten

Klicke auf **E-Mail-Konto hinzufügen** und trage Kontoname, E-Mail-Adresse, **Empfangsprotokoll**, **Verschlüsselung beim
Empfang** (**TLS ab Verbindungsaufbau** oder **STARTTLS**), Posteingangsserver und -port, den **SMTP-Postausgangsserver**
samt Port und **Verschlüsselung beim Versand** sowie **Benutzername** und **Passwort** ein.

- Das Feld **Posteingangsport** gilt für beide Protokolle und beginnt bei 993. Setze 995 für POP3 mit TLS, 143 für IMAP mit
  STARTTLS oder 110 für POP3 mit STARTTLS.
- Das Passwort landet im Tresor als `email_<id>_password`. SMTP-Benutzername und SMTP-Passwort sind optional; das
  SMTP-Passwort landet als `email_<id>_smtp_password`.
- Das neue Konto startet ohne Agentenzugriff und ohne Posteingangs-Watcher. Kann Mailer nicht verbinden, bleibt das Konto
  gespeichert: „Das Konto wurde angelegt, aber die Verbindung konnte nicht abgeschlossen werden.“

**Bekanntes Problem in dieser Version:** Das Formular schickt **Passwort** als SMTP-Anmeldung, die der Server für klassische
Konten noch nicht annimmt. Bis das behoben ist, lege das Konto unter **Config → E-Mail** mit denselben Angaben an.

### Google-Konten

**Google-Konto hinzufügen** bietet **Gmail API** (der Standard), IMAP und POP3. Wähle in der Liste eine vorhandene GoG- oder
Google-Workspace-Quelle, oder **Google-OAuth einrichten** mit Client-ID und Client-Secret (das Secret landet im Tresor). Neue Google-Konten dürfen lesen, senden, Lesestatus, Labels und Ordner ändern, aber nicht
endgültig löschen. Zur Weiterleitungsadresse siehe die [Betriebsreferenz](../../mailer.md#oauth-application-registration).

### Microsoft-Konten und AgentMail

**Microsoft-Konto hinzufügen** braucht deine eigene Microsoft-App-Registrierung. Der Empfang läuft über IMAP oder POP3, der
Versand über Microsofts SMTP-Server. Richtlinien des Mandanten können die SMTP-Anmeldung sperren.

AgentMail erscheint als Quelle, sobald ein primäres Postfach eingerichtet ist. Antworten bleiben in der
AgentMail-Konversation, solange der Betreff gleich bleibt.

### Verbinden, Entfernen und Kontooptionen

**Verbinden** verknüpft eine Quelle. **Aus Mailer entfernen** löst ein verbundenes Konto von Mailer. Die Rückfrage lautet:
„Dieses Konto aus Mailer entfernen? Die gemeinsame Integration und gespeicherte Nachrichten bleiben erhalten.“ Integration
und Postfach beim Anbieter bleiben unangetastet.

Jedes verbundene Konto hat unter **Einstellungen → Konten** außerdem: **Konto deaktivieren**, **Konto nur lesen** (Mailer
kann ein Konto nur einschränken, nie erweitern), **Agentenzugriff erlauben** (nur klassische IMAP-Konten mit Passwort, siehe
[Synchronisierung im Hintergrund und Agentenzugriff](#synchronisierung-im-hintergrund-und-agentenzugriff)),
**Neu autorisieren**, wenn der Anbieter neue Berechtigungen braucht, und **Ordnerzuordnung** für Archiv, Entwürfe, Spam,
Gesendet und Papierkorb.

## Nachrichten lesen und ordnen

Die Liste kann alle Konten zeigen. Jedes Konto behält seine Ordner oder Gmail-Labels, eine Konversation bleibt im Konto.

- **Suchen** umfasst nur lokal synchronisierte Nachrichten, und das Ergebnis sagt es.
- **Beim Anbieter suchen** fragt den Anbieter nach weiteren Treffern im gerade angezeigten Konto und Ordner und lädt weitere
  Seiten nach. Wechsel von Konto, Ansicht oder Ordner und das Löschen der Suche schalten es aus.
- **Antworten**, **Allen antworten**, **Weiterleiten**, **Archivieren**, **Papierkorb**, **Verschieben nach …**,
  **Markieren**, **Als gelesen markieren** und Labels wirken auf eine Nachricht. **Löschen** fragt zuerst: „Diese Nachricht
  endgültig löschen?“ Nutze **Papierkorb**, wenn du sie vielleicht zurückholen willst.
- **Schlummern** blendet eine Nachricht bis zur gewählten Zeit in Mailer aus. **EML herunterladen** speichert sie als Datei.
  **Drucken** öffnet eine Vorschau; erlaube dafür Pop-ups.

Das Öffnen markiert eine Nachricht als gelesen, wenn das Konto das erlaubt. Hintergrundprüfungen per IMAP ändern den Lesestatus
nicht. Die Liste zeigt Vorschau und Anhangszahl, bevor der Text geladen wird. Eine nicht lesbare Nachricht bleibt mit einem
Hinweis in der Konversation. Externe Bilder sind zunächst blockiert; **Externe Bilder laden** erlaubt sie für eine
Konversation. Sie kommen dann direkt vom Server des Absenders.

**Regeln** (**Einstellungen → Regeln**) ordnen neue Post ein. Eine Regel passt auf **Von**, **An**, **Betreff**,
**Nachrichtentext** oder **Hat Anhänge** und kann markieren, verschieben, archivieren, in den Papierkorb legen, Labels setzen
oder schlummern. **Betroffene Nachrichten ansehen** zeigt, was sie bei vorhandener Post ändern würde, bevor du **Auf
ausgewählte Nachrichten anwenden** nutzt. Regeln leiten nie weiter und löschen nie endgültig. Eine fehlgeschlagene Aktion
bleibt unter **Regeln für diese Nachricht** sichtbar und lässt sich dort erneut versuchen. **Einstellungen → Vorlagen** und
**Einstellungen → Signaturen** speichern wiederkehrende Texte; eine Signatur gehört zu einem sendenden Konto.

## Verfassen und senden

Klicke auf **Verfassen**. Die Nachricht öffnet sich in einem eigenen Desktop-Fenster mit dem Titel **Neue Nachricht**. **Von**
wählt das sendende Konto und aktualisiert die Signatur. **Cc hinzufügen** und **Bcc hinzufügen** zeigen die weiteren Felder.
Importiertes HTML bleibt, bis du **In bearbeitbaren Text umwandeln** wählst. **Datei anhängen** lädt Dateien hoch, und
**Sticker** fügt ein Bild aus der Sticker-Bibliothek des Desktops als PNG `sticker-<id>.png` ein.

Entwürfe speichern kurz nach deiner letzten Änderung („Entwurf gespeichert“). Hat ein anderes Fenster den Entwurf zuerst
gespeichert, wähle **Meine Änderungen behalten** oder **Andere Version laden**.

- **Senden** legt die Nachricht in den Postausgang. **Versand rückgängig machen** bricht sie innerhalb von zehn Sekunden ab.
  Danach übergibt Mailer sie an den Anbieter, und rückgängig machen holt sie nicht mehr zurück.
- **Später senden** behält die gewählte Zeit und Zeitzone. Überfällige Aufträge laufen nach einem Ausfall, sofern Mailer und
  das Konto eingeschaltet sind. Schaltest du Mailer aus, pausieren sie.
- Der Postausgang zeigt **Geplant**, **Wird gesendet**, **Gesendet**, **Fehlgeschlagen**, **Abgebrochen** und
  **Versandstatus unklar**. **Versand erneut versuchen** reiht eine fehlgeschlagene Nachricht wieder ein.
- **Versandstatus unklar** heißt: Die Verbindung brach ab, nachdem der Anbieter die Nachricht vielleicht schon angenommen
  hat. Mailer sendet sie dann nicht von selbst erneut. Prüfe zuerst den Ordner Gesendet beim Anbieter.

## Schreibhilfe

**Schreibhilfe** im Verfassen-Fenster bietet **Aus Stichpunkten schreiben**, **Antwort entwerfen**, **Text verbessern**,
**Kürzen**, **Ton ändern**, **Übersetzen** und **Konversation zusammenfassen**. Sie nutzt den KI-Anbieter, den du im Feld
wählst, standardmäßig den Hauptanbieter von AuraGo. Einen eigenen Schalter gibt es nicht; das Feld steht bereit, solange der
Desktop eingeschaltet und nicht schreibgeschützt ist.

Vor jeder Anfrage zeigt das Feld den Anbieter sowie den genauen Entwurf und Kontext. Anhänge und der Rest des Postfachs
werden nie mitgeschickt. Prüft LLM Guardian E-Mails, geht der Text zuerst an diesen Prüfer, und das Feld zeigt auch dessen
Anbieter und Modell. Was die Sicherheitsprüfung beanstandet, wird abgelehnt.

Das Ergebnis ist ein Vorschlag. **Vorschlag übernehmen** setzt ihn in den Entwurf, **Verwerfen** wirft ihn weg. Ein Vorschlag für
eine ältere Fassung kann neuere Änderungen nicht überschreiben. Die Schreibhilfe hat keine Werkzeuge und kann weder Empfänger
ändern noch senden, andere Postfächer lesen oder etwas im Gedächtnis speichern. Ihre Nutzung zählt im Budget unter der
Kategorie `mailer`.

## Synchronisierung im Hintergrund und Agentenzugriff

Solange **Im Hintergrund synchronisieren** eingeschaltet ist, synchronisiert AuraGo jedes Konto, auch bei geschlossenem
Mailer-Fenster. Ein ruhiges Postfach wird etwa alle fünf Minuten geprüft. Neue Post, eine unvollständige Seite, eine Löschung
oder ein Fehler holt die nächste Prüfung auf etwa eine Minute vor. Manuelle Synchronisierungen und die erste Prüfung nach dem
Start laufen sofort.

**Agentenzugriff.** Der Agent hat eigene E-Mail-Werkzeuge. Sie arbeiten mit den Konten unter **Config → E-Mail**
(`email_accounts`), die Agentenzugriff erlauben:

- **Agentenzugriff erlauben** setzt `agent_access` für ein Konto. Konten, die du in Mailer anlegst, starten ohne Zugriff.
  Ältere IMAP-Konten ohne diesen Eintrag behalten ihren bisherigen Zugriff.
- POP3-Konten und Konten mit OAuth oder XOAUTH2 geben dem Agenten nie Zugriff, auch wenn der Schalter an ist.
- Ein Konto „nur lesen“ lässt den Agenten Post lesen, aber nicht senden.

**Posteingangs-Watcher.** Unter **Config → E-Mail → „Posteingangs-Watcher“** schaltest du **„Watcher aktiv“** ein und setzt
**„Überwachter Ordner“** (ab Werk INBOX) und **„Intervall (Sekunden)“** (ab Werk 120). Der Watcher prüft IMAP-Konten mit
Agentenzugriff und meldet dem Agenten neue Post, die der Agent dann abrufen kann. Markierte Nachrichten meldet er getrennt.
Im selben Bereich heißt ausgeschaltetes **„Konto aktiv“**: Konto deaktiviert. Ausgeschaltetes **„E-Mail-Versand erlaubt“**
macht das Konto schreibgeschützt.

## Gut zu wissen

| Punkt | Grenze |
|---|---|
| Anhang, je Datei | 25 MiB |
| Anhänge je ausgehender Nachricht | 25 MiB zusammen |
| Ausgehende Nachricht einschließlich Text | 32 MiB |
| Lokal gespeicherte Anhänge insgesamt | 512 MiB |
| Eingabe der Schreibhilfe | 32 KB Entwurf und 32 KB Kontext |
| Klassische E-Mail-Konten in Mailer | 100 |
| GoG-Konten insgesamt | 20 |
| IMAP-Nachricht, die sich laden lässt | 16 MiB |
| AgentMail-Anhänge (gesendet oder geladen) | 25 MiB, unabhängig von `agentmail.max_attachment_mb` |

- **POP3-Post liegt auf deinem Rechner.** Ordner und Lesestatus sind lokal, und die Synchronisierung löscht nie Nachrichten auf
  dem Server.
- **Nicht in dieser Version:** Kalender, PGP und S/MIME sowie selbstständige Postfachaktionen der Schreibhilfe.

## Datenschutz und Sicherheit

**Was den Rechner verlässt.** Mailverkehr geht an die Server, die du eingetragen hast, an die Gmail-API für Google-Konten,
an Microsofts Server für Microsoft-Konten und an die AgentMail-API. Die Schreibhilfe schickt Entwurf und Kontext an deinen gewählten Anbieter und an den E-Mail-Prüfer, wenn
LLM Guardian E-Mails prüft. Externe Bilder laden direkt vom Server des Absenders. AuraGo leitet sie nicht weiter, daher sieht
der Absender deine Netzwerkadresse und den Zeitpunkt deines Öffnens.

**Was wo gespeichert wird.**

- `data/mailer/mailer.db`: Metadaten, Vorschauen, POP3-Kopien, Entwürfe, Regeln, Vorlagen, Signaturen, Postausgangsaufträge
  und Synchronisationsstände.
- `data/mailer/blobs/`: Anhänge.
- Vor einem Schema-Upgrade legt Mailer eine Sicherung neben der Datenbank an (`mailer.db.pre-v<n>-<Zeitstempel>.bak`, nur für
  den Eigentümer lesbar).
- Passwörter, Client-Secrets und OAuth-Tokens bleiben im Tresor: `email_<id>_password`, `email_<id>_smtp_password`,
  `email_<id>_oauth_client_secret` und `oauth_gog_client_<id>`. Die Konfiguration enthält nur Servernamen, Protokolle und
  Verweise.
- Sichere das Mailer-Verzeichnis und den Tresor gemeinsam. POP3-Post und nicht versendete Entwürfe sind kein Cache.

**Wer Mailer nutzen darf.** Die Mailer-API verlangt eine Administrator-Sitzung oder ein Token mit dem Recht `desktop:admin`,
und Änderungen brauchen eine Anfrage von derselben Herkunft. Ein eigenes Auditprotokoll hat Mailer in dieser Version nicht.

**Was der Agent darf und was nicht.** Der Agent erreicht nur Konten mit Agentenzugriff, über seine eigenen E-Mail-Werkzeuge,
die Mailers Datenbank nicht lesen. Die Schreibhilfe kann nichts senden, verschieben oder andere Post lesen. Regeln leiten nie
weiter und löschen nie endgültig. Schlägt die Zertifikatsprüfung eines Servers fehl, korrigiere den Servernamen oder vertraue
dem Zertifikat.

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „E-Mails konnten nicht geladen werden.“ | Klicke auf **Erneut versuchen**. Hilft das nicht, teste das Konto unter **Einstellungen → Konten**. |
| „Mailer ist schreibgeschützt. Änderungen sind deaktiviert.“ | Der schreibgeschützte Modus des Desktops oder ein Konto „nur lesen“ blockiert die Änderung. Dieselbe Meldung erscheint, wenn **Im Hintergrund synchronisieren** aus ist. Prüfe diesen Schalter zuerst. |
| „Die Aktion konnte nicht abgeschlossen werden.“ | Prüfe den Kontostatus unter **Einstellungen → Konten** und versuche es erneut. Bei einem klassischen Konto siehe das bekannte Problem oben. |
| „Hintergrundsynchronisierung pausiert“ | Schalte **Im Hintergrund synchronisieren** unter **Einstellungen → Konten** ein. |
| „Verbindungstest fehlgeschlagen“ | Prüfe Server, Port, Verschlüsselung, Benutzername und Passwort. Die Meldung nennt einen Fehlercode, der die Ursache verrät. |
| „Verbindungstest teilweise erfolgreich“ | Empfang oder Versand funktioniert, aber nicht beides. Teste jede Richtung einzeln. |
| „Das Konto wurde angelegt, aber die Verbindung konnte nicht abgeschlossen werden.“ | Das Konto ist gespeichert. Korrigiere die Einstellungen und verbinde es unter **Vorhandenes Konto verbinden** erneut. |
| „Die Anmeldung ist erforderlich oder abgelaufen. Autorisiere dieses Konto erneut.“ | Klicke unter **Einstellungen → Konten** auf **Neu autorisieren**. |
| „Der Netzwerkzugriff ist in den AuraGo-Einstellungen deaktiviert.“ | Ein Admin schaltet **Netzwerk-Anfragen (api_request)** in der Gefahrenzone ein. Gmail-Konten brauchen das. |
| „Erlaube Pop-ups, um das Freigabefenster zu öffnen.“ | Erlaube Pop-ups für AuraGo und klicke danach auf **Im Freigabefenster fortfahren**. |
| „Versandfehler: {code}“ | Klicke auf **Versand erneut versuchen**. Der Code stammt vom Anbieter oder von der Verbindung. |
| „Upload fehlgeschlagen. Versuche es erneut oder entferne die Datei, bevor Du sendest.“ | Lade die Datei erneut hoch oder entferne den Anhang. Eine Datei darf höchstens 25 MiB groß sein. |
| „Dieser Entwurf wurde an anderer Stelle geändert. Lade ihn vor dem erneuten Speichern neu.“ | Wähle **Meine Änderungen behalten** oder **Andere Version laden**. |
| „Der lokale Mailer-Speicher ist voll. Gib vor der Synchronisierung Speicherplatz frei.“ | Gib Speicherplatz auf der Festplatte frei oder entferne alte Anhänge und Post. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) ·
[Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 27: Bookshelf](./27-bookshelf.md)
**Nächstes Kapitel:** [Kapitel 29: Infra](./29-infra.md)
