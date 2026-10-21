# Kapitel 27: Bookshelf – Bibliothek und E-Book-Reader

Bookshelf ist die Bibliothek und der E-Book-Reader des Virtuellen Desktops. Die App sammelt Bücher aus einem Ordner im
Desktop-Arbeitsbereich, aus Dateien, die du hochlädst, und aus verbundenen WebDAV-, Koofr- oder SMB-Bibliotheken.
Gelesen wird im Browser. Die App ist für Bücher gedacht, die dir gehören und frei von DRM sind. Sie entfernt keinen
DRM-Schutz, verändert deine Originaldateien nie und schickt keine Buchinhalte an ein KI-Modell oder einen
Metadatendienst. Sie ist keine Buchhandlung und synchronisiert nicht mit E-Readern.

Technische Details für Admins (Katalog, Quell-Adapter, Konverter-Sandbox, Build-Prüfungen) stehen in der
[Betriebsreferenz](../../bookshelf.md) (englisch).

## Inhaltsverzeichnis

1. [Was du brauchst](#was-du-brauchst)
2. [Erste Schritte](#erste-schritte)
3. [Bücher hinzufügen](#bücher-hinzufügen)
4. [Lesen](#lesen)
5. [Notizen, Lesezeichen und Fortschritt](#notizen-lesezeichen-und-fortschritt)
6. [Sammlungen und Lesestatus](#sammlungen-und-lesestatus)
7. [Optionale EPUB-Kopien](#optionale-epub-kopien)
8. [Gut zu wissen](#gut-zu-wissen)
9. [Datenschutz und Sicherheit](#datenschutz-und-sicherheit)
10. [Wenn etwas nicht klappt](#wenn-etwas-nicht-klappt)

---

## Was du brauchst

- **Virtueller Desktop an.** Bookshelf läuft im Virtuellen Desktop. Unter **Config → Virtueller Desktop** muss der
  Schalter **Virtuellen Desktop aktivieren** (`virtual_desktop.enabled`) an sein. Solange er aus ist, ist der Desktop
  nicht verfügbar, und der Bookshelf-Schalter wirkt nicht.
- **Bookshelf an.** Unter **Config → Virtueller Desktop** der Schalter **Bookshelf aktivieren**
  (`virtual_desktop.bookshelf_enabled`). Standardmäßig ist er an: `config_template.yaml` setzt
  `bookshelf_enabled: true`, und der Konfigurations-Loader aktiviert ihn auch, wenn der Schlüssel fehlt. Der
  Hilfetext unter dem Schalter sagt noch „standardmäßig ausgeschaltet“. Dieser Text ist falsch.
- **Kein Neustart nötig.** Das Speichern wirkt sofort. Wenn du Bookshelf ausschaltest, brechen laufende Scans und
  Konvertierungen ab, und offene Leselinks werden widerrufen. Dein Katalog bleibt auf der Festplatte.
- **Schreibgeschützter Modus.** Mit **Schreibgeschützter Modus** (`virtual_desktop.readonly`) kannst du Bücher weiter
  öffnen und lesen. Du kannst aber keine Bücher hinzufügen, nicht scannen, keine Details ändern, keinen Lesestatus
  setzen, keine Notizen oder Lesezeichen anlegen und deine Leseposition nicht speichern.
- **Nur für Administratoren.** Bookshelf braucht Administratorzugriff auf den Desktop (Berechtigung `desktop:admin`).
  Der Agent hat kein Bookshelf-Werkzeug und kann deine Bibliothek deshalb weder sehen noch ändern.
- **Optional:** ein E-Book-Konverter, nur für EPUB-Kopien einiger Formate. Siehe
  [Optionale EPUB-Kopien](#optionale-epub-kopien).

## Erste Schritte

1. Öffne **Bookshelf** im Startmenü unter **Büro**. In der deutschen Oberfläche heißt die App **Bücherregal**. Ein
   leeres Regal zeigt *Dein Bücherregal wartet* und die Schaltfläche **Bücher hinzufügen**.
2. Klicke auf **Bücher hinzufügen**. Wähle **Bibliotheksquelle verbinden** für einen Ordner oder eine entfernte
   Bibliothek oder **Bücher hochladen** für Dateien von diesem Gerät.
3. Bei einer Quelle wählst du den **Quelltyp**, tippst unter **Quellenname** einen Namen ein, wählst den Ordner und
   klickst auf **Hinzufügen und scannen**. Bei Uploads wählst du die Dateien aus und klickst auf **Hochladen**.
4. Klicke auf ein Cover, um die Details zu sehen, und dann auf **Buch lesen**.

## Bücher hinzufügen

Bookshelf liest nur deine Quelldateien. Den Katalog mit Titeln, Covern, Sammlungen und Leseposition führt Bookshelf
selbst.

| Quelle | Wo die Bücher liegen | Was du einrichtest |
|---|---|---|
| **Lokaler Ordner** | Ein Ordner im Desktop-Arbeitsbereich (standardmäßig `agent_workspace/virtual_desktop`) | Unter **Speicherort** den Desktop-Arbeitsbereich wählen, dann den Ordner. |
| **Upload** | Eine verwaltete Kopie im Importordner von Bookshelf | Dateien von diesem Gerät auswählen. Die Originale bleiben unberührt. |
| **WebDAV** | Ein Ordner deiner WebDAV-Integration | **Config → WebDAV**: **Aktiviert** und **Im Dateimanager anzeigen** an. |
| **Koofr** | Ein Ordner auf einem Koofr-Mount | **Config → Koofr**: **Aktiviert** und **Im Dateimanager anzeigen** an. Den Mount unter **Speicherort** wählen. |
| **SMB-Freigabe** | Ein Ordner auf einer SMB-Freigabe, die du angibst | **Server**, **Freigabe**, **Benutzername** und **Passwort**. Keine Netzwerksuche, kein Gastzugang, kein SMB1. |

- **Scans starten bewusst.** Beim Hinzufügen eines Ordners startet ein Scan (**Hinzufügen und scannen**). Weitere
  Scans laufen, wenn du unter **Quellen** auf **Jetzt scannen** klickst oder nach dem Zeitplan unter
  **Aktualisierungsplan**: **Nur manuell** (Standard), **Stündlich**, **Alle 6 Stunden** oder **Täglich**.
  Unterordner werden mitgescannt, außer du entfernst den Haken bei **Unterordner einschließen**.
- **Dateien erkennt Bookshelf an der Endung.** Andere Dateien im Ordner werden ohne Meldung übersprungen. Ein Scan
  zeigt, wie viele Dateien er gesehen, hinzugefügt und übersprungen hat.
- **Fehlende Dateien.** Findet ein vollständiger Scan ein Buch nicht mehr, bleibt es im Katalog und ist als
  **Nicht verfügbar** markiert. Ein Scan, der nicht alle Ordner erreicht hat, entfernt nie Bücher.
- **Dateien-App.** Öffnest du in **Dateien** eine EPUB-, MOBI-, AZW-, AZW3-, FB2-, FB2.ZIP-, CBZ- oder CBR-Datei, öffnet
  sie sich in Bookshelf, es sei denn, du hast für diesen Dateityp eine andere Standard-App gewählt. Bei anderen
  Formaten: Rechtsklick auf die Datei, **Öffnen mit...** wählen und **Bookshelf** auswählen. Mit **Immer öffnen mit
  Bücherregal** wird Bookshelf zur Standard-App für den Dateityp. Die Datei wird dort in den Katalog aufgenommen, wo
  sie liegt. Sie wird nicht kopiert.
- **Hochladen.** Jede Datei wird vor dem Senden gegen das Größenlimit geprüft und dann in den Importordner von
  Bookshelf kopiert. Bricht ein Upload ab, bleiben die bereits übertragenen Bücher erhalten. Wiederhole den Vorgang
  mit derselben Auswahl, dann werden nur noch die restlichen Bücher gesendet.
- **Quelle entfernen** unter **Quellen** trennt die Quelle. Ihre Bücher bleiben im Katalog als **Nicht verfügbar**,
  mit Notizen und Leseposition, bis du jedes einzeln mit **Aus Bookshelf entfernen** löschst. Keine Datei wird
  angerührt.

## Lesen

Mit **Buch lesen** öffnest du ein Buch im Reader. Die Werkzeugleiste über der Seite bietet **← Bibliothek**, **‹** und
**›** (vorherige und nächste Seite), **Inhalt**, **Seitenthumbnails**, **Im Buch suchen**, **Notizen und
Lesezeichen**, **Leseeinstellungen** und **Vollbild**. Seitenthumbnails gibt es nur bei Büchern mit fester
Seitenaufteilung, bei PDFs und bei Comics.

- **Seiten umblättern.** Mit den Schaltflächen, den Pfeiltasten oder **Bild auf** und **Bild ab**. Bei Büchern von
  rechts nach links drehen sich die Pfeiltasten um.
- **Inhalt** listet Kapitel oder Seiten auf. Ein Klick auf einen Eintrag springt dorthin.
- **Textbücher** (EPUB mit fließendem Text, TXT, HTML) bieten **Textgröße**, **Zeilenabstand**, **Seitenbreite**
  (Schmal, Komfortabel, Breit) und **Seitenthema** (Papier, Sepia, Dunkel). TXT- und HTML-Dateien bieten außerdem
  **Textkodierung** (Automatisch, UTF-8, Windows-1252, Windows-1251, UTF-16 LE oder UTF-16 BE). Eine Schriftauswahl
  gibt es nicht.
- **Bücher mit fester Seitenaufteilung, PDFs und Comics** bieten **Seitenanpassung** (Seite einpassen oder An Breite
  anpassen), **Zoom** (Automatisch oder 75 bis 250 %), **Doppelseite** und **Leserichtung**.
- **Im Buch suchen** durchsucht den Text. Gescannte PDFs und Comics ohne Text haben keinen durchsuchbaren Text, dort
  findet die Suche nichts. Bookshelf führt keine OCR aus.
- **Passwortgeschützte PDFs** fragen zuerst nach dem Passwort.
- **Links**, die aus dem Buch hinausführen, fragen vorher nach (**Externen Link öffnen?**). Manche Links werden
  abgelehnt.
- Die Leseeinstellungen werden für jedes Buch in diesem Browser gespeichert.

## Notizen, Lesezeichen und Fortschritt

- **Leseposition.** Sie wird beim Lesen gespeichert. Die Werkzeugleiste zeigt **Position gespeichert**,
  **Position wird gespeichert…** oder **Position nicht gespeichert**. Schlägt das Speichern fehl, wähle
  **Erneut versuchen**.
- **Lesezeichen:** Öffne **Notizen und Lesezeichen** und wähle **Lesezeichen hinzufügen**.
- **Markierung:** Markiere zuerst Text und wähle dann **Markierung**. Ohne Auswahl erscheint *Markiere zuerst Text im
  Buch.* In PDFs sind Markierungen und Notizen an den markierten Bereich der Seite gebunden.
- **Notiz:** Wähle **Notiz hinzufügen**, tippe deinen Text und klicke auf **Speichern**.
- Die Einträge stehen unter **Notizen und Lesezeichen**. Ein Klick springt zur Stelle, **×** löscht den Eintrag.
- **Geänderte Dateien.** Ändert sich eine Buchdatei auf der Festplatte, behandelt Bookshelf sie als neue Ausgabe. Die
  gespeicherte Position wird verworfen, und der Status springt auf **Ungelesen**, außer du hast den Status selbst
  gesetzt. Notizen bleiben erhalten, sind aber markiert: *Aus einer früheren Ausgabe; diese Stelle kann abweichen.*
  Ihr Sprunglink ist abgeschaltet.
- **Zwei Fenster.** Hat ein anderes Fenster eine neuere Position gespeichert, fragt Bookshelf, welche bleiben soll:
  **Gespeicherte Position verwenden** oder **Meine aktuelle Position beibehalten**.

## Sammlungen und Lesestatus

- **Lesestatus** ist **Ungelesen**, **Leseansicht** oder **Abgeschlossen**. Du setzt ihn in den Details des Buches. Das
  Regal filtert nach Status und sortiert nach **Kürzlich hinzugefügt**, **Titel**, **Autor** oder **Lesefortschritt**.
- **Sammlungen:** **Neue Sammlung** legt eine an, **Zur Sammlung hinzufügen** nimmt ein Buch auf. Bücher und
  Sammlungen lassen sich mit den Pfeiltasten nach oben oder unten verschieben.
- **Buchdetails bearbeiten** ändert Titel, Autor, Reihe, Beschreibung und Tags. Die Datei selbst bleibt unverändert.
- **Cover ersetzen** setzt ein eigenes PNG- oder JPEG-Cover. Das ist eine Überschreibung im Katalog, die Buchdatei
  bleibt unverändert.
- **Ansichten:** Coverbild, Buchrücken oder Liste. Das Regal zeigt 36 Bücher pro Seite. Suche und Sortierung gelten
  für die ganze Bibliothek.
- **Aus Bookshelf entfernen** nimmt ein Buch aus dem Katalog. Die Quelldatei bleibt, wo sie ist.

## Optionale EPUB-Kopien

Manche Formate lassen sich nicht direkt lesen. Wenn der Server einen E-Book-Konverter hat, kann Bookshelf davon eine
EPUB-Kopie erstellen. Zum Lesen von EPUB, PDF, MOBI und den anderen unterstützten Formaten braucht es keinen
Konverter.

- **Formate:** CBR, RTF, DOCX und ODT. LIT, PDB und LRF sind im Konvertierungscode aufgeführt, der Katalog nimmt diese
  Dateien in dieser Version aber nicht an. Sie lassen sich deshalb noch nicht konvertieren.
- **Für Administratoren:** Calibres `ebook-convert`, `bubblewrap` (`bwrap`) und `prlimit` auf einem Linux-Host oder
  Container installieren, über die übliche Systemverwaltung. Dann **Programm für die Buchkonvertierung**
  (`virtual_desktop.bookshelf_converter_path`) auf `/usr/bin/ebook-convert` oder `/opt/calibre/ebook-convert` setzen.
  Ein leeres Feld deaktiviert die Konvertierung. Bookshelf installiert und lädt nichts herunter.
- **Für dich:** In den Details eines Buches wählst du **EPUB-Kopie erstellen** und danach **EPUB-Kopie lesen**, sobald
  sie fertig ist. Die Kopie bleibt auf dem Server. Dein Original bleibt unverändert, das Layout kann aber vom Original
  abweichen.
- **Grenzen:** Eine Konvertierung bricht nach drei Minuten ab. Die Eingabe darf bis zu
  256 MiB groß sein.
- **Sandbox:** Jede Konvertierung läuft in einem privaten Arbeitsordner, mit einer schreibgeschützten Kopie der
  Eingabe, ohne Netzwerk, mit geleerter Umgebung und Ressourcengrenzen. Verweigert der Host die Sandbox, schlägt die
  Konvertierung fehl. Es gibt keinen Rückfall, der ohne Sandbox läuft.
- **Windows und macOS** können die Sandbox nicht nutzen. Konvertiere das Buch auf einem anderen Computer und lade die
  EPUB-Datei stattdessen hoch.
- DRM wird nie entfernt. ACSM-Lizenzdateien sind keine Bücher.

## Gut zu wissen

| Format | Lesbar in Bookshelf | Hinweise |
|---|---|---|
| EPUB, mit fließendem Text oder fester Seitenaufteilung | Ja | Skripte, externe Bilder und Stile, eingebettete Frames und Formulare werden entfernt |
| MOBI, AZW, AZW3 (DRM-frei, KF8) | Ja | Übliche MOBI- und KF8-Dateien |
| FB2, FB2.ZIP | Ja | |
| CBZ (Comics) | Ja | Seitenbilder: PNG, JPEG, WebP, GIF oder AVIF |
| TXT, HTML, HTM | Ja | Die Textkodierung lässt sich korrigieren |
| PDF | Ja | Passwortabfrage bei Bedarf. Keine OCR. |
| CBR, RTF, DOCX, ODT | Nur als EPUB-Kopie | Braucht den Konverter, siehe [Optionale EPUB-Kopien](#optionale-epub-kopien) |
| DjVu, Kindle KFX, ACSM | Nein | Sie können im Katalog stehen, lassen sich aber nicht öffnen |
| LIT, PDB, LRF | Nein | In dieser Version beim Hochladen abgelehnt und beim Scannen übersprungen |

- **Größe pro Buch:** der kleinere Wert aus 256 MiB und **Max. Dateigröße (MB)** in den Einstellungen des Virtuellen
  Desktops (standardmäßig 50 MB). Größere Dateien werden beim Hochladen abgelehnt und beim Scannen übersprungen.
- **Scans:** bis zu 10.000 Dateien pro Scan, Ordner bis 64 Ebenen tief und höchstens vier Scans gleichzeitig.
- **Archive im Reader:** Eine EPUB-, FB2.ZIP- oder CBZ-Datei darf höchstens 10.000 Einträge, 32 MiB pro Eintrag und
  256 MiB entpackt umfassen.
- **Coverbilder:** PNG oder JPEG, im Dialog höchstens 8 MB.
- **Lese-Cache:** standardmäßig 1 GiB für vorbereitete Kopien. **Auf diesem AuraGo-Server behalten** speichert eine
  Kopie, die auch dann lesbar bleibt, wenn die Quelle offline geht. Die Speicherangabe zeigt belegten Platz, behaltene
  Kopien und das Limit.
- **Plattformen:** Nur die Konvertierung ist auf Linux beschränkt. Bibliothek und Reader funktionieren überall, wo der
  Virtuelle Desktop läuft.
- **Nicht verfügbar:** DRM-Entfernung, OCR, Online-Metadatensuche, Synchronisation mit E-Readern oder Shops, eine
  Schriftauswahl und ein Bookshelf-Werkzeug für den Agenten.

## Datenschutz und Sicherheit

- **Was den Rechner verlässt:** Nichts über deine Bücher oder deine Bibliothek geht an ein KI-Modell oder einen
  Metadatendienst. Die einzigen Verbindungen nach außen sind die Quellen, die du anbindest (WebDAV, Koofr, SMB), und
  die Links, die du bewusst öffnest.
- **Wo es liegt:** alles unter `data/bookshelf/`:
  - `catalog.sqlite`: Titel, Sammlungen, Lesestatus, Leseposition, Notizen, Lesezeichen und Quellen.
  - `covers/` (Coverbilder), `imports/` (Uploads), `renditions/` (EPUB-Kopien) und `cache/` (vorbereitete und
    behaltene Kopien).
  Sichere `catalog.sqlite` und die verwalteten Ordner. `cache/` darfst du löschen, er wird bei Bedarf neu aufgebaut.
  Vor einem Datenbank-Upgrade legt Bookshelf eine Sicherungskopie des Katalogs daneben ab.
- **Lese-Einstellungen** liegen pro Buch im Browser, nicht auf dem Server.
- **SMB-Passwörter** liegen im Vault unter `bookshelf_smb_<source-id>_password`. Beim Entfernen der Quelle wird dieses
  Passwort gelöscht. Python-Werkzeuge können Schlüssel mit dem Präfix `bookshelf_` nicht lesen.
- **Deine Originale** werden von Bookshelf nie geändert, verschoben oder gelöscht, egal ob du hochlädst, entfernst,
  trennst oder ein Buch vergisst.
- **Reader-Sandbox.** Ein Buch öffnet sich in einem eigenen Frame ohne Netzzugriff. Skripte und Formulare im Buch
  werden entfernt. Externe Links brauchen deine Bestätigung.
- **Zugriffsprüfung.** Jede Anfrage nach einem Buch prüft deine Berechtigung erneut. Buchantworten werden mit
  `Cache-Control: no-store` ausgeliefert. Ein Reader-Link läuft nach fünf Minuten ab, wenn er nie benutzt wurde, sonst
  nach 24 Stunden ohne Nutzung.
- **Agent.** Der Agent hat kein Bookshelf-Werkzeug. Er kann Bücher, Notizen und Leseposition weder auflisten, lesen,
  hinzufügen, ändern noch entfernen.

## Wenn etwas nicht klappt

| Meldung | Was hilft |
|---|---|
| „Bookshelf ist ausgeschaltet“ | Schalte **Virtuellen Desktop aktivieren** und **Bookshelf aktivieren** unter **Config → Virtueller Desktop** ein. |
| „Deine Bibliothek konnte nicht geladen werden.“ | Prüfe, ob AuraGo läuft und deine Anmeldung noch gilt, und wähle dann **Erneut versuchen**. |
| „Dieser Speicherort konnte nicht durchsucht werden.“ | Der Ordner oder die Verbindung fehlt oder verweigert den Zugriff. Prüfe die Einstellungen der Quelle. |
| „Der Bibliotheksscan konnte nicht abgeschlossen werden.“ | Prüfe, ob die Quelle erreichbar ist, und scanne erneut. Es laufen höchstens vier Scans gleichzeitig, warte also, bis einer fertig ist. |
| „Der Scan wurde beendet, aber einige Ordner waren nicht erreichbar. Die gefundenen Bücher bleiben erhalten.“ | Behebe das Problem mit dem nicht erreichbaren Ordner oder der Verbindung und scanne erneut. |
| „Der Auftrag wurde abgebrochen. Bereits gefundene Bücher bleiben erhalten.“ | Starte den Scan erneut. |
| „„…“ konnte nicht hochgeladen werden. Bitte versuche es erneut.“ | Versuche es erneut und prüfe den freien Speicherplatz auf dem Server. |
| „„…“ überschreitet die maximale Dateigröße von 50 MB.“ | Nimm eine kleinere Datei oder erhöhe **Max. Dateigröße (MB)** (die harte Grenze liegt bei 256 MiB). |
| „„…“ überschreitet die zulässige Dateigröße. Prüfe das Dateigrößenlimit in den Einstellungen des Virtuellen Desktops.“ | Wie oben. Die Meldung erscheint, solange das Limit noch nicht bekannt ist. |
| „Bereits hochgeladen: 2 von 5. Beim erneuten Versuch werden nur die übrigen Bücher gesendet.“ | Wiederhole den Vorgang mit derselben Auswahl. |
| „Dieses Buch ist derzeit nicht verfügbar.“ | Die Datei oder die Quelle ist offline oder wurde entfernt. Verbinde die Quelle neu oder scanne erneut. |
| „Dieses Buch kann hier nicht geöffnet werden.“ | Das Format lässt sich hier nicht lesen. Siehe die Formattabelle und [Optionale EPUB-Kopien](#optionale-epub-kopien). |
| „Dieses Buch konnte nicht geöffnet werden.“ | Die Datei ist vielleicht beschädigt oder verschlüsselt. |
| „Die Konvertierung ist auf diesem Server nicht verfügbar.“ | Der Server braucht Linux, den Pfad zum Konverter, `bwrap` und `prlimit`. |
| „Die EPUB-Kopie konnte nicht erstellt werden.“ | Die Konvertierung ist fehlgeschlagen oder hat länger als drei Minuten gedauert. Versuche es erneut; das Original ist unberührt. |
| „Mit diesem Passwort ließ sich das PDF nicht entsperren. Versuche es erneut.“ | Gib das Passwort erneut ein. |
| „Leseposition wurde an anderer Stelle geändert“ | Ein anderes Fenster hat eine neuere Position gespeichert. Wähle **Gespeicherte Position verwenden** oder **Meine aktuelle Position beibehalten**. |
| „Position nicht gespeichert“ | Wähle **Erneut versuchen**. Sonst geht die letzte Position beim Verlassen verloren. |
| „Die Quelldatei wurde geändert. Aktualisiere die Quelle oder füge die aktuelle Datei erneut zu Bookshelf hinzu und versuche es dann noch einmal.“ | Die Datei hat sich seit dem Hinzufügen geändert. Scanne oder aktualisiere die Quelle und lies dann erneut. |
| „Füge diesen Speicherort als Quelle hinzu, bevor du das Buch öffnest.“ | Füge den Ordner zuerst unter **Bücher hinzufügen** als Quelle hinzu. |
| „Diese Datei gehört zu keiner verbundenen Bibliothek.“ | Füge den Ordner über **Bücher hinzufügen** hinzu und öffne das Buch dann erneut. |
| „Dieser Link kann nicht aus Bookshelf geöffnet werden.“ | Manche Links werden abgelehnt. Versuche nicht, sie zu öffnen. |
| „Das Coverbild muss kleiner als 8 MB sein.“ | Nimm ein kleineres PNG- oder JPEG-Bild. |
| `unsupported`, `limit_exceeded`, `source_readonly`, `source_access_denied`, `revision_conflict` | Wird als Rohcode angezeigt, wenn es keinen Text dafür gibt. `unsupported`: das Format wird nicht angenommen. `limit_exceeded`: ein Größen- oder Scan-Limit. `source_readonly`: die Quelle oder der Desktop ist schreibgeschützt. `source_access_denied`: bei WebDAV oder Koofr **Im Dateimanager anzeigen** prüfen. `revision_conflict`: das Buch wurde an anderer Stelle geändert, lade neu. |
| `bookshelf_archive_limit`, `bookshelf_invalid_archive`, `bookshelf_unsupported_format`, `bookshelf_reader_timeout` | Wird unverändert angezeigt. Das Buch ist innen zu groß, beschädigt, ein Format, das der Reader nicht verarbeitet, oder es hat zu lange zum Öffnen gedauert. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 8: Integrationen](./08-integrations.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 26: Video Studio](./26-video-studio.md)
**Nächstes Kapitel:** [Kapitel 28: Mailer](./28-mailer.md)
