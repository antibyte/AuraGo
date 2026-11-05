# Kapitel 24: EasyDrag – Automationen per Drag & Drop

EasyDrag ist der visuelle Editor für Flows. Ein Flow ist eine Mission, die aus Bausteinen besteht: einem oder mehreren **Auslösern** (Zeitplan, Datum & Uhrzeit, Webhook, E-Mail empfangen, MQTT-Nachricht, Home-Assistant-Zustand, Gerät, Person erkannt (eine Netzwerkkamera erkennt eine Person), Telefonanruf über die Fritz!Box, Planer, AuraGo-Start, Budget-Warnung, Mission beendet oder Manueller Start) und danach **Schritten** wie Websuche, KI-Schritt, PDF erstellen, Telegram-Nachricht, WhatsApp-Nachricht, E-Mail senden, Home Assistant oder jedes freigegebene Werkzeug. Flows laufen an der Warteschlange des Agenten vorbei und erscheinen in Mission Control mit ihrem Verlauf.

Dieses Kapitel beschreibt den Editor. Wie sich Läufe, Grenzen, Fehler und Geheimnisse auf dem Server verhalten, steht in [Kapitel 11: Flow-Missionen (EasyDrag)](11-missions.md#flow-missionen-easydrag).

## Inhaltsverzeichnis

1. [Der erste Flow](#der-erste-flow)
2. [Der Editor](#der-editor)
   - [Visuelle Gruppen](#visuelle-gruppen)
3. [Speichern](#speichern)
4. [Testen](#testen)
5. [Veröffentlichen](#veröffentlichen)
6. [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)
   - [Lokale Läufe fortsetzen](#lokale-läufe-fortsetzen)
7. [Decisions-Baustein](#decisions-baustein)
8. [Wenn ein Schritt scheitert](#wenn-ein-schritt-scheitert)
9. [Geheimnisse](#geheimnisse)
10. [Vorlagen, Import und Export](#vorlagen-import-und-export)
11. [Mission Control, Missionsseite und Dashboard](#mission-control-missionsseite-und-dashboard)
12. [Einstellungen](#einstellungen)
13. [Wenn EasyDrag abgeschaltet ist](#wenn-easydrag-abgeschaltet-ist)
14. [Tastenkürzel](#tastenkürzel)
15. [Wiederverwendbare lokale Bausteine](#wiederverwendbare-lokale-bausteine)

---

## Der erste Flow

Die Startseite bietet **Karten** und eine **Kompakte Liste** für dieselben Flows. Markiere
einen Flow mit dem Stern als Favoriten und schalte **Nur Favoriten** ein, um die Liste
einzugrenzen. Unter **Gespeicherte Filter auf diesem Gerät** speicherst Du die aktuelle
Suche, den Statusfilter, die Sortierung und die Favoritenauswahl unter einem Namen.
Die Auswahl dieses Namens stellt alles wieder her; Speichern unter demselben Namen
aktualisiert den Filter. Beim Löschen eines gespeicherten Filters bleiben seine Flows
erhalten. Du kannst bis zu 20 benannte Filter behalten. Diese Ansichten gehören zu
diesem Browser auf diesem Gerät und werden nicht mit Flows exportiert. Verweigert der
Browser das Speichern, sagt EasyDrag Dir Bescheid; die Auswahl gilt dann nur bis zum
Schließen des Fensters. Andere offene EasyDrag-Fenster im selben Browser folgen der
Änderung. Beim Löschen der Browserdaten gehen diese Einstellungen verloren.

**Ordner und Tags** ordnet normale Flows auf AuraGo geräteübergreifend. Öffne am Flow
**⋯ → Ordner und Tags**, wähle einen Ordner und gib bis zu 16 Tags ein. Groß- und
Kleinschreibung werden unterschieden. Unter **Ordner verwalten** erstellst oder benennst
Du flache Ordner um; verschiebe ihre Flows, bevor Du einen belegten Ordner löschst.
Ordner- und Tagfilter lassen sich auch in einem lokalen Filter speichern. Die Zuordnung
ändert weder den Flow noch seine veröffentlichte Version oder Testergebnisse. Hat ein
anderes Fenster zuerst gespeichert, bleibt Deine Eingabe sichtbar; **Gespeicherte Werte
neu laden** fragt vor dem Verwerfen nach. Verborgene Tags bleiben beim Ordnerwechsel
auf dem Server. **Verborgenen Inhalt ersetzen** beginnt eine frische Tagliste. Gib in
diesen Namen und Tags keine Geheimnisse ein.

**Mehrere Flows pausieren:** Wähle in der Bibliothek bis zu 50 veröffentlichte Flows
aus und öffne **Pause prüfen**. Die Auswahl verändert Deine Flows nicht; eine andere
Suche, ein anderer Ordner oder Filter hebt sie auf. Die Vorschau prüft jedes Ziel.
Nur dort als aktiv angezeigte Flows kommen nach einer erneuten Statusprüfung für die
Pause infrage.

Wähle **Aktive Flows pausieren**, um fortzufahren. Künftige Zeitplan- und Timerläufe
stoppen, und die gespeicherte Wiederaufnahme wartet während der Pause. Bereits
angenommene gewöhnliche Läufe können weiterlaufen; ausgeführte Aktionen werden durch
die Pause nicht rückgängig gemacht. Jeder Flow erhält ein eigenes Ergebnis. **Derzeit
pausiert; Speichern unbestätigt** bedeutet: Der Schalter ist aktuell aus, aber der
Schreibvorgang wurde nicht bestätigt. Ob die Pause einen Neustart übersteht, ist damit
nicht geklärt. Fehlgeschlagene oder unbeantwortete Pausen werden nie automatisch erneut
angefordert. Ein unklarer Schreibvorgang oder verlorene Berechtigungen stoppen die
übrigen Anfragen. Prüfe die Ergebnisse, bevor Du einen neuen Vorgang startest. Beim
Schließen des Dialogs werden keine weiteren Pausen angefordert; eine bereits gesendete
Anfrage kann noch abgeschlossen werden.

Für ein erstes Ergebnis ohne Konto wähle **Dein erstes lokales Ergebnis** in der
Rezeptauswahl. Drei Schritte lesen einen Beispielnamen, bilden eine Begrüßung und prüfen,
ob der Name vorhanden ist. Öffne nach dem Test die Ausgabe des Begrüßungsschritts.
Suche und Kategorien helfen bei weiteren Rezepten. Die Karten unterscheiden lokale
Verarbeitung und mögliche externe Auswirkungen. Ein Einrichtungshinweis bedeutet nicht,
dass eine Integration fehlt oder ihre Verbindung geprüft wurde.

**Beschreibe Deine Automatisierung** erstellt aus Deinem Ziel einen KI-Vorschlag. Prüfe Schritte,
Änderungen und mögliche Auswirkungen vor **Übernehmen**. Erst dann entsteht ein neuer
Flow; bei einem bestehenden Flow wird der Vorschlag als eine rückgängig machbare
Änderung übernommen. Veröffentlichen, Aktivieren und Ausführen bleiben eigene Aktionen.
An den eingerichteten KI-Anbieter gehen Dein eingegebenes Ziel und die verfügbaren
Bausteinbeschreibungen, keine gespeicherten Testdaten oder Lauf-Ausgaben. Gib deshalb
auch im Zieltext keine Geheimnisse ein. Ohne verfügbaren Anbieter bleiben Vorlagen
und manuelles Bearbeiten nutzbar.

1. Öffne **EasyDrag** auf dem Desktop. Wähle **Neuer Flow** und gib ihm einen Namen, oder starte mit einer **Vorlage**.
2. Klicke **Auslöser wählen** und such dir einen Auslöser aus.
3. Wähle den Auslöser aus und drücke **Strg+K**: Die Schnellsuche fügt den nächsten Schritt direkt dahinter ein und verbindet ihn. **Tab** verschiebt nur den Fokus. Du kannst auch **Hinzufügen** an der gewählten Karte nutzen oder Bausteine aus der Leiste links auf die Fläche oder auf eine Verbindung ziehen.
4. Doppelklicke einen Schritt (oder drücke **Enter**), um ihn zu bearbeiten. Der Inspektor öffnet sich im Editor, nicht als Dialog. Er hat die Reiter Eingabe, Parameter und Ausgabe. Parameter hat drei Teile: *Werte*, *Einstellungen* und *Notiz*. Im Reiter Eingabe siehst Du die Daten früherer Schritte: aus dem letzten Test oder Lauf, oder als Beispiel. Ziehe einen Wert über den Reiter Parameter und dann in ein Feld, oder klicke ihn an, um ihn an der letzten Cursorposition einzufügen. Bezüge erscheinen als farbige Chips mit Schritt und Feld, etwa „Websuche › results“. Im Text stehen sie als `{{websuche.results}}`.
5. **Testen** (Strg+Enter) öffnet den Testdialog. Wähle **Simulation** mit Beispieldaten oder ausdrücklich einen **Echten Test**, der echte Auswirkungen haben kann. Vor der Ausführung wird der Entwurf gespeichert. Prüfe das Ergebnis und die durchlaufenen Schritte; siehe [Testen](#testen).
6. **Veröffentlichen** prüft den Flow, zeigt, was er außerhalb von AuraGo tut, und schaltet ihn auf Wunsch gleich aktiv. Nur veröffentlichte Flows laufen selbstständig. Dein Entwurf bleibt bearbeitbar; die veröffentlichte Version läuft.

## Wiederverwendbare lokale Bausteine

Öffne auf der Startseite **Wiederverwendbare Bausteine** und wähle **Neuer Baustein**.
Ein Baustein beginnt mit einer Eingabe- und einer Ausgabegrenze. Öffne beide per Doppelklick,
füge benannte Felder hinzu und wähle Text, Zahl, Wahrheitswert, Objekt oder Liste. Pflichtfeld
und zulässiges `null` sind getrennte Einstellungen. Ein nicht übergebener optionaler Wert
unterscheidet sich von ausdrücklich gesetztem `null`; `false` und `0` bleiben echte Werte.
Die Ausgabe bindet jedes deklarierte Feld an einen festen Wert oder Daten eines vorherigen
Schritts. Wähle im Eingabetab das Zielfeld und füge Daten aus dem Baum an der gespeicherten
Cursorposition ein. `{{block_input.data.name}}` liest beispielsweise das Eingabefeld `name`.
Sind gespeicherte Felddeklarationen ungültig, erhält die JSON-Reparaturansicht sie, bis Du eine
korrigierte Liste ausdrücklich übernimmst. Ein Vertrag erlaubt höchstens 64 Felder und 32 KiB Deklarationen.

**Baustein simulieren** speichert und prüft den Entwurf schon vor der ersten Veröffentlichung.
Trage Beispieldaten mit den typisierten Feldern ein; Objekte und Listen verwenden JSON.
Die Simulation verarbeitet ausschließlich lokale Logik und erzeugt keinen Laufverlauf, Timer
und keine Mission. Beispiele bleiben im geöffneten Dialog. **Vorschauhinweise** nennt die
Grenzen: insgesamt 64 KiB, 10.000 Werte und 32 Verschachtelungsebenen. Das Schließen des
Dialogs bricht seine ausstehende Anfrage ab. Prüfe den Feldvertrag und wähle **Veröffentlichen**,
um eine unveränderliche Version anzulegen. Unterstützt werden Felder setzen, Wenn,
Fallunterscheidung, Zusammenführen und Aufrufe veröffentlichter reiner Bausteinversionen.
Die maximale Aufruftiefe beträgt acht; kreisförmige Verweise werden abgelehnt. Jeder Aufruf
bleibt an seine gewählte Version gebunden. Stapelverarbeitung, Warten, Stoppen, KI,
Geheimnisse und externe Werkzeuge sind innerhalb eines Bausteins ausgeschlossen. Die
Simulation erlaubt Dir, abgeschlossene Bausteinschritte im Ergebnis anzusehen.

Füge in einem Flow oder einem anderen Baustein **Baustein aufrufen** hinzu. Wähle einen Baustein und eine
veröffentlichte Version und klicke auf **Gewählte Version verwenden**. Prüfe vor dem Übernehmen
den bisherigen und den neuen Feldvertrag. Fülle benannte Eingaben aus oder ordne Daten aus
vorherigen Schritten zu. Ausgaben liegen direkt am Aufruf, etwa `{{greet.greeting}}`, wenn
sein Datenname `greet` lautet. Eine neue Bausteinversion ändert bestehende Aufrufer nicht.
Aktualisiere jeden Aufrufer ausdrücklich: passende Zuordnungen bleiben erhalten, entfernte
Felder werden genannt, und Rückgängig stellt die alte Referenz wieder her. Unpassende Werte
bleiben bis zur Korrektur sichtbar. Ein Aufruf hat keine automatischen Wiederholungen;
importierte Wiederholungseinstellungen musst Du vor Veröffentlichung oder Ausführung
ausdrücklich auf `0` zurücksetzen.

Unter **Mehr** kannst Du veröffentlichte Versionen ansehen und vor dem Löschen Abhängigkeiten
prüfen. Veröffentlichte Elternversionen und aufbewahrte Läufe können eine Bausteinversion
weiter verwenden; dann bleibt das Löschen gesperrt. **Mehr laden** zeigt ältere Versionen
oder weitere Abhängigkeiten. Die Versionsansicht ist schreibgeschützt; **Aktuellen Entwurf
öffnen** führt zur Bearbeitung zurück. **Als Entwurf wiederherstellen** ersetzt nach einer
Rückfrage den aktuellen Bausteinentwurf durch die gewählte Version. Der laufende Baustein
ändert sich erst beim Veröffentlichen; aufrufende Flows behalten ihre gebundene Version, bis
Du sie ausdrücklich aktualisierst. Der Elternlauf zeigt Baustein-ID und Version des
Aufrufs; aufgezeichnete untergeordnete Schritte bleiben Teil desselben Laufs.

### Aufgezeichnete Bausteinschritte ansehen

Öffne ein Lauf- oder Simulationsergebnis, wähle den Aufruf- oder Stapelschritt und dann
**Ausgabe → Bausteinschritte ansehen**. Wähle einen Aufruf und anschließend einen
aufgezeichneten Schritt. Du kannst seine aufgelösten Parameter oder seine Ausgabe als Baum,
Tabelle oder JSON lesen. Die Pfadleiste und Zurück führen durch verschachtelte Aufrufe.
Version, Eintragsnummer und Versuch unterscheiden wiederholte Aufrufe. Der Eltern-Flow bleibt
unverändert. Die Ansicht zeigt abgeschlossene Aufzeichnungen, keinen Live-Fortschritt und
keine getrennten Bausteinläufe.
Der gewählte Schritt ersetzt die Liste; **Zurück** führt wieder zur Liste. Auf schmalen
Fenstern öffnet sich dieselbe Ansicht als Dialog, damit Werte und Navigation bedienbar bleiben.

Aufruf- und Schrittlisten haben lokale Seiten. Nach einer Kürzung unterscheiden sich
aufbewahrte Einträge von Gesamtzahlen; unbekannte Gesamtzahlen erscheinen als Strich.
Unvollständige, gekürzte und ältere Verläufe werden ausdrücklich gekennzeichnet. Ein fehlender
Eintrag beweist nicht, dass ein Schritt übersprungen wurde. Es werden nur vollständige Werte
bis 16 KiB angezeigt; größere oder bereits gekürzte Werte bleiben vollständig ausgeblendet.
Nach einem Fehler können auch kleinere, später aufgezeichnete Werte fehlen, weil sie aus
gekürztem Fehlertext stammen könnten; die Ausführung bleibt unverändert.
Die Zeiten des letzten Versuchs gelten für den gesamten Schritt einschließlich Wiederholungen.

Beschriftungen stammen aus der genau aufbewahrten Bausteinversion. Ist sie nicht verfügbar,
geschützt oder zu groß, bleiben die Aufzeichnungen lesbar, ohne eine aktuelle Version
einzusetzen. Fehlt das historische Elterndokument, erreichst Du dieselbe Ansicht über die
Laufliste. Simulationen verwenden nur ihre mitgelieferten Fassungen und laden keine echten
Dateien nach.

### Ausgewählte Schritte als Baustein auslagern

Wähle eine zusammenhängende, lineare Folge eingeschalteter **Felder setzen**-Schritte und
anschließend **Auswahl als Baustein auslagern** im Bearbeiten- oder Auswahlmenü. Die erste
Version unterstützt feste Ausgabetypen und ausgeschaltetes **Eingangsdaten behalten**; die Vorschau erklärt,
wenn eine Auswahl oder Referenz nicht sicher ausgelagert werden kann. Ausstehende Änderungen
werden zuerst gespeichert. Benenne den Baustein und prüfe vorgeschlagene Ein- und Ausgaben,
Gruppenänderungen und zusätzlichen Aufwand vor dem Übernehmen.

Beim Übernehmen werden der Baustein erstellt und veröffentlicht sowie die Auswahl im
Flow-Entwurf gemeinsam ersetzt. Der Flow wird dabei weder veröffentlicht noch ausgeführt.
Rückgängig stellt den Flow-Entwurf wieder her, löscht aber nicht den neuen Baustein. Geht die
Antwort verloren, nutze **Status prüfen**. Ein unbekannter Status beweist nicht, dass nichts
passiert ist: Lade den aktuellen Flow und die Bausteinbibliothek neu und prüfe sie vor einem
weiteren Versuch.

### Eine Liste mit einem Baustein verarbeiten

Füge in einem normalen Flow **Für jeden Eintrag** hinzu und wähle ausdrücklich eine
veröffentlichte Bausteinversion. Unter **Bausteineingabe für jeden Eintrag** wählst Du
das typisierte Feld, das jeden ganzen Listeneintrag erhält. Die übrigen **Gemeinsamen
Eingaben** füllst Du einmal aus. **Liste** nimmt eine JSON-Liste oder einen einzelnen
zugeordneten Ausdruck an, etwa `{{trigger.data.items}}`. Eine Zuordnung ersetzt die
feste Liste; sie hängt keinen Ausdruck an deren JSON an. Text wie `{{beispiel.feld}}`
in einem Eintrag bleibt wörtlicher Dateninhalt, auch beim Umbenennen oder Kopieren
eines anderen Schritts.

Wähle **Maximale Einträge** bewusst (1–1.000, anfangs 100). Eine längere Liste wird vor
dem ersten Eintrag abgelehnt und niemals gekürzt. Eine leere Liste liefert erfolgreich
eine leere Ergebnisliste; der JSON-Wert `null` ist keine Liste. Der direkte JSON-Editor erlaubt bis zu 2 MiB,
200.000 Werte und 64 Ebenen; der gesamte Flow muss weiter ins Dokumentlimit passen.
Die Feldinfo hält diese Grenzen bereit. Scheitert die Prüfung des Arbeitsbudgets,
verringere **Maximale Einträge**; ein Klick auf das Problem führt direkt zu diesem Feld.

Die Einträge laufen einzeln in ihrer ursprünglichen Reihenfolge. **Beim ersten Fehler
abbrechen** nennt den fehlgeschlagenen nullbasierten Index und Fehlercode, ohne ein
Teilergebnis zu liefern. **Eintragsfehler sammeln** behält geordnete Erfolgs- und
Fehlereinträge mit erfolgreicher Ausgabe oder stabilem Fehlercode. Abbruch, Zeit- und
Ressourcengrenzen beenden immer die gesamte Verarbeitung. Die Ausgabe bietet `items`,
`count`, `succeeded` und `failed`; die Ergebnisansicht unterscheidet verarbeitete
Gesamtzahlen von aufbewahrten Einträgen und zeigt bis zu 50 Zusammenfassungen. Darunter
bleiben die Ansichten Baum, Tabelle und JSON verfügbar. Aus gekürzten oder ausgelassenen
Ergebnissen kannst Du nicht schließen, ob fehlende Einträge erfolgreich waren oder
scheiterten. **Bausteinschritte ansehen** öffnet aufbewahrte Eintrags- und Aufrufdetails;
der Abbruchmodus liefert weiterhin kein strukturiertes Teilergebnis.

Die Simulation verwendet denselben festgelegten reinen Baustein. Eine spätere
Veröffentlichung aktualisiert diesen Schritt nie automatisch: Prüfe den Versionswechsel
ausdrücklich. Fällt sein Eingabefeld pro Eintrag weg, bleibt die bisherige Auswahl sichtbar
nicht verfügbar, bis Du ein anderes wählst. Die Wiederholungszahl bleibt null. Der gewählte
Baustein darf weitere reine Bausteine aufrufen; externe Werkzeuge und dauerhaft wartende
Verarbeitung bleiben ausgeschlossen.

## Der Editor

Die Schrittliste in der linken Werkzeugleiste zeigt Dir den Flow als sichtbare Liste.
Wähle dort einen Schritt, um ohne Ziehen zu navigieren. **Ausgewählten Pfad fokussieren**
zeigt den verbundenen Pfad. Baustein-Bibliothek und Inspektor lassen sich am Trenner
mit Maus oder Tastatur vergrößern. Diese Ansichten werden nur auf diesem Gerät gemerkt
und ändern den Flow nicht.
Die Breitenmenüs an den Panel-Köpfen bieten dieselben Größen auch per Klick oder Tippen.

Wähle beim Einfügen von Eingabedaten zuerst **Daten einfügen in**. EasyDrag rät kein
Zielfeld mehr. Das gewählte Feld und die Einfügeposition bleiben erhalten; ein breiter
Inspektor kann Quelldaten und Parameter nebeneinander anzeigen.

Um einen Quellwert vor dem Einfügen zu bearbeiten, nutze seinen Knopf **Wert aufbereiten**
oder drücke **Umschalt+F10** auf der fokussierten Quellzeile. Wähle und ordne bis zu acht
Operationen; **In gewähltes Feld einfügen** ersetzt die Auswahl an der gemerkten Cursorposition als eine
rückgängig machbare Änderung. Die Quellenangabe zeigt, woher die Vorschaudaten stammen.
Auch ohne tatsächliches Beispiel kannst Du einen gültigen Ausdruck bauen; fehlende
Daten unterscheiden sich vom ausdrücklichen JSON-Wert `null`. Ein frisches JSON-Beispiel
gilt nur für diese Vorschau und wird weder gespeichert noch in den Flow eingefügt.
Lokale Vorschauen begrenzen Eingaben und JSON-Zwischenergebnisse auf 64 KiB und angezeigte
Ergebnisse auf 8 KiB. Unter **Vorschaugrenzen und Hinweise** stehen die Einzelheiten.
Bei Datum, HTML, Unicode-Groß-/Kleinschreibung und Rundung kann die Browserdarstellung
von der Ausführung abweichen. Die Datumsvorschau braucht einen ausdrücklichen Zeitversatz
in der Eingabe und eine gewählte Zeitzone. Teste den Flow mit seinen tatsächlichen Daten.

- Der Kopf zeigt Name und Stand des Flows, mit denselben Wörtern wie Mission Control und die Missionsseite: *Noch nicht veröffentlicht* (nie veröffentlicht), *Veröffentlicht* oder *Unveröffentlichte Änderungen*, daneben *Pausiert*, solange ein veröffentlichter Flow ausgeschaltet ist. Er enthält **Rückgängig** und **Wiederholen**, **Läufe**, **Testen**, **Veröffentlichen**, den Schalter **Aktiv** und das Menü **⋯**: *Jetzt ausführen*, *Versionsverlauf*, *Flow-Einstellungen*, *Duplizieren*, *Exportieren*, *In Mission Control zeigen* und *Löschen*. In einem Editor, der 560 px oder schmaler ist (auf dem Handy), wandert **Läufe** ins Menü ⋯; **Testen** bleibt direkt erreichbar, und *Pausiert* bleibt neben dem Namen sichtbar; ebenso *Neu veröffentlichen nötig* und die Warnung „Auslöser „Datum & Uhrzeit“ laufen nicht“. Die Fußzeile zeigt den letzten Lauf, Probleme und Hinweise sowie den Speicherstand. „Entwurf“ meint immer die Version, die du bearbeitest, im Unterschied zur veröffentlichten.
- **Rückgängig** und **Wiederholen** im Kopf nehmen die letzte Änderung zurück oder wiederholen sie, auch auf Touchscreens (auf der Fläche tun Strg+Z und Strg+Umschalt+Z dasselbe). Löschst du Schritte, erscheint für einige Sekunden ein Hinweis wie „Alpha gelöscht“ mit **Rückgängig**; eine Rückfrage vor dem Löschen gibt es nicht.
- **Schritt im Flow suchen** (Strg+F, auch im Menü *Ansicht* und im Kontextmenü der Fläche) sucht Schritte nach Name, Datenname, Typ oder Text in den Parametern; Enter springt zum ersten Treffer.
- **Automatisch anordnen** (Alt+Umschalt+F, auch im Menü *Bearbeiten* und im Kontextmenü der Fläche) stellt die Schritte in Spalten vom Auslöser nach rechts auf. Das ist ein einziger Schritt für Rückgängig.
- Ein Rechtsklick (oder langes Drücken) auf eine Karte bietet *Öffnen*, *Diesen Schritt testen*, *Duplizieren*, *Kopieren*, *Ausschalten* oder *Einschalten* und *Löschen*; sind zwei oder mehr Schritte ausgewählt, stellen *Horizontal ausrichten* und *Vertikal ausrichten* sie in eine Zeile oder Spalte. Ein Rechtsklick auf die leere Fläche bietet *Schritt hier hinzufügen*, *Hier einfügen*, *Schritt im Flow suchen*, *Automatisch anordnen*, *Alles auswählen* und *Alles zeigen*.
- Die Übersicht unten links (in Fenstern breiter als 900 px) zeigt den ganzen Flow; ein Klick oder Ziehen darin verschiebt die Ansicht.
- Bis ein Flow seine erste Verbindung hat, erklärt ein Tipp das Verbinden: Ziehe vom Punkt am rechten Rand einer Karte auf die nächste Karte, oder wähle einen Schritt aus und drücke C. Geht eine Verbindung nicht, sagt ein kurzer Hinweis neben dem Zeiger, warum (etwa eine Schleife oder Anschlüsse, die schon verbunden sind).
- Die Schnellsuche fügt einen Schritt ein, wenn du ihn anklickst oder antippst, nicht schon beim Scrollen der Liste. Die *Einstellungen* eines Schritts enthalten auch seine *Position auf der Fläche* (X und Y in Pixeln), so verschiebst du ihn ohne Ziehen.
- Die Startseite zeigt deine Flows mit denselben Ständen und filtert sie nach *Alle*, *Aktiv*, *Pausiert* (veröffentlicht und ausgeschaltet) und *Mit Fehlern*. Schlägt ein späteres Neuladen fehl, bleibt die letzte Liste sichtbar und nur lesbar, bis **Erneut versuchen** klappt. Eine verlorene Anmeldung leert diese Liste. Der erste Ladevorgang und abgeschaltete Flows ersetzen die Liste durch eine Karte mit erneutem Versuch oder Schloss, nicht durch eine leere Bibliothek. Eine leere Bibliothek bietet Vorlagen, **Neuer Flow** und **Importieren**.
- Der Editor folgt Änderungen von anderswo (einem anderen Fenster, Mission Control, der Missionsseite oder dem Agenten): Wird der Flow dort pausiert oder eingeschaltet, folgen Stand, Schalter **Aktiv** und *Jetzt ausführen*; wird er dort gelöscht, sagt EasyDrag das und zeigt die Startseite.
- Ein Flow öffnet sich in lesbarer Größe: Ein kleiner Flow erscheint ganz, ein größerer beginnt bei seinem Auslöser mit 80 %. In Fenstern, die breiter als 560 px sind, merkt sich EasyDrag auf diesem Gerät, wo du jeden Flow verlassen hast. Verschieben und Zoomen der Fläche ändern den Flow nicht: Sie werden weder in ihm gespeichert noch als *Unveröffentlichte Änderungen* gezählt. **Alles zeigen** (Umschalt+1) zeigt den ganzen Flow; zoomt das weit heraus, zeigen die Karten nur noch ihre Namen, dafür größer.
- Der Inspektor sitzt im Editor, nicht in einem Dialog, und EasyDrag öffnet dafür kein fest breites Fenster. Eingabe, Parameter und Ausgabe sind immer Tabs. Unter etwa 760 px wechseln sich Fläche und Inspektor ab. Das EasyDrag-Fenster kann bis 360 px schmal sein; ein Viewport von 320 px passt trotzdem. In schmalen Fenstern und auf dem Handy ist die Bausteine-Leiste anfangs zu. Geöffnet liegt sie über der Fläche und schließt sich, sobald du einen Schritt hinzufügst oder auf die Fläche tippst.
- Die *Flow-Einstellungen* enthalten Name, Beschreibung, was passiert, wenn ein Auslöser während eines Laufs feuert (*Warteschlange*, *Parallel ausführen* oder *Überspringen*), die maximale Laufzeit und wohin fehlgeschlagene Läufe gemeldet werden. Die maximale Laufzeit nimmt Minuten und Sekunden, von 1 Sekunde bis 24 Stunden; bleiben beide Felder leer, darf ein Lauf 30 Minuten dauern, der Standard. **Übernehmen** lässt sie, wie sie ist, solange du sie nicht änderst.
- Ist der Desktop schreibgeschützt, öffnet EasyDrag Flows nur zum Lesen; nur **Stoppen** funktioniert dann noch für Läufe, die nicht beendet sind (außer Missionen sind ebenfalls schreibgeschützt, `tools.missions.readonly`). Wird der Desktop auf schreibgeschützt gestellt, laufen bereits laufende Flows weiter: Du stoppst sie unter **Läufe**. Bei schreibgeschützten Missionen ist es anders: Solange `tools.missions.readonly` an ist, starten veröffentlichte Flows von selbst keine neuen Läufe mehr (Zeitpläne, Ereignisse, Datum/Uhrzeit-Timer) und *Jetzt ausführen* in Mission Control wird abgelehnt, sofort und ohne Neustart.
- Nach einem Update prüft AuraGo beim Start einmal jeden veröffentlichten Flow gegen die aktuellen Regeln. Besteht die veröffentlichte Version sie nicht mehr, trägt der Flow auf seiner Karte auf der Startseite und im Kopf *Neu veröffentlichen nötig*, und Mission Control vermerkt es an der Flow-Mission. Seine Läufe können scheitern, bis du den Flow öffnest, die angezeigten Probleme behebst und ihn erneut veröffentlichst; mit der neuen Veröffentlichung (oder wenn eine spätere Prüfung beim Start den Flow in Ordnung findet) verschwinden die Markierung und der Hinweis in Mission Control.
- Läufe, die schon in einer Warteschlange standen, als Missionen schreibgeschützt wurden oder EasyDrag abgeschaltet wurde, starten auch später nicht: Wenn sie an der Reihe sind, enden sie als *Gestoppt* (automatische Läufe sind pausiert), und Mission Control vermerkt es an der Flow-Mission. Ein Lauf, den du selbst mit *Jetzt ausführen* gestartet hast, läuft trotzdem.
- Kann AuraGo den Flow-Speicher nicht starten (eine gesperrte oder beschädigte `flows.db`), versucht es das im Hintergrund weiter. Bis dahin zeigt die Startseite *Auslöser „Datum & Uhrzeit“ laufen nicht* und der Kopf einen Chip mit denselben Worten; sie nennt Datum & Uhrzeit, weil diese Auslöser von den Timern abhängen.
- In einer Liste aus Schlüsseln und Werten (etwa den Headern einer HTTP-Anfrage) werden zwei Zeilen mit demselben Schlüssel markiert, und aus dieser Liste wird nichts gespeichert, bis jeder Schlüssel eindeutig ist. Zahlen und Ja/Nein-Werte aus einem importierten Flow behalten ihren Typ, bis du sie bearbeitest.

### Visuelle Gruppen

Mit **Visuelle Gruppen** in der linken Werkzeugleiste öffnest Du die Gruppenliste. Wähle
Schritte auf der Fläche aus und klicke auf **Ausgewählte Schritte gruppieren**. Gruppen ordnen
nur die Fläche: Sie fügen keinen Schritt hinzu und ändern weder Verbindungen noch Einstellungen
oder Ausführung. Ein Schritt gehört höchstens einer Gruppe an. Ein Flow oder Baustein kann bis
zu 100 Gruppen enthalten; ein Titel hat 1–80 Zeichen, eine Notiz bis zu 2.048.

Ein Klick auf den Gruppenkopf wählt seine Schritte aus; ein Doppelklick oder Enter öffnet den
Gruppeninspektor. Dort änderst Du Titel, Notiz und Farbe, fügst Mitglieder hinzu oder entfernst
sie, verschiebst die Gruppe über Positionsfelder oder löst sie auf, ohne Schritte zu löschen.
Löschst Du das letzte Mitglied, verschwindet auch die leere Gruppe. Jede Änderung lässt sich
rückgängig machen. Kopierst Du eine vollständige Gruppe, werden ihre Gruppendaten mitkopiert;
bei einer Teilauswahl werden nur die gewählten Schritte ohne Gruppe kopiert.

Einklappen und Anordnungssperre gelten nur für Deine Ansicht auf diesem Gerät. Die Sperre
verhindert das Verschieben und Ausrichten der Gruppenschritte, nicht das Bearbeiten ihrer
Einstellungen oder die Ausführung. Die Schrittliste zeigt auch Mitglieder eingeklappter
Gruppen; wählst Du ein Mitglied oder einen Fehler, wird die Gruppe vor dem Fokussieren
ausgeklappt. In schreibgeschützten Läufen und Versionen ändert Ausklappen nur Deine Ansicht.
Gruppenänderungen erhöhen die Entwurfsrevision; teste die neue Revision vor der Veröffentlichung.

### Ausdrücke für Listen und Objekte

Diese Filter verwenden in der Vorschau und bei der Ausführung dieselbe Ausdruckssprache.
Feldnamen bezeichnen direkte Felder, keine verschachtelten Pfade. Beispielsweise summiert
`{{items | where("status", "eq", "ready") | sum("amount")}}` die Beträge bereiter Einträge.

| Filter | Wirkung |
| --- | --- |
| `pick("name", "value")` | Behält 1–16 benannte Objektfelder. Fehlende Felder werden ausgelassen, `null` bleibt erhalten. |
| `rename("title", "name")` | Benennt ein vorhandenes Feld um. Ein bereits belegtes anderes Zielfeld führt zu einem Fehler. |
| `where("status", "eq", "ready")` | Behält passende Objekte. Operatoren: `eq`, `ne`, `lt`, `le`, `gt`, `ge`. Fehlende Felder passen nie. Gleichheit unterscheidet Datentypen; Größenvergleiche verlangen gleiche Zahlen- oder Texttypen. |
| `sort("score", "desc")` | Sortiert Zahlen numerisch und Text nach Unicode-Codepunkten, jeweils stabil; fehlende Werte und `null` bleiben zuletzt. `sort()` sortiert einfache Werte, `sort("", "desc")` absteigend. Gemischte Typen und Wahrheitswerte führen zu Fehlern. |
| `unique("id")` | Behält den ersten Eintrag je typisiertem Feldwert; fehlend ist etwas anderes als `null`. Für einfache Werte nutzt Du `unique()`. |
| `sum("amount")` | Addiert endliche Zahlen; leere Listen ergeben `0`. Fehlende Werte, `null` und Text führen zu Fehlern. Für eine Zahlenliste nutzt Du `sum()`. |

Die Filter verändern ihre Eingabe nicht. Jede Operation ist auf 10.000 Einträge pro Liste,
256 Felder pro Objekt, 32 Verschachtelungsebenen, 100.000 Werte und 2 MiB Text-/Schlüsselbytes
begrenzt. Zu große Daten führen zu einem klaren Fehler. Die bisherigen Flow-Grenzen gelten
weiter; die lokale visuelle Vorschau darf für flüssige Bedienung kleinere Grenzen setzen.

## Speichern

- EasyDrag speichert eine Sekunde nach deiner letzten Änderung. Die Anzeige unten rechts zeigt den Stand:
  - *Gespeichert*, *Ungespeichert* oder *Wird gespeichert…*: der normale Ablauf.
  - *Offline – neuer Versuch läuft*: AuraGo ist nicht erreichbar oder hat mit einem Serverfehler geantwortet. EasyDrag versucht es selbst wieder; hat AuraGo einen Grund genannt, steht er neben dem Stand.
  - *Nicht gespeichert* mit **Erneut versuchen**: AuraGo hat die Änderung abgelehnt, etwa weil der Flow zu groß ist oder du Flows nicht ändern darfst. Der Grund steht daneben.
  - *Nicht gespeichert – Fehler beheben*: Der Entwurf hat Fehler, die das Speichern verhindern. Die nächste Änderung wird wieder gesendet.
  - *Konflikt*: Der Flow wurde woanders geändert. EasyDrag fragt, ob du deine Version behältst oder die andere lädst.
- Bis AuraGo deine Änderungen hat, bleibt bis zu 30 Tage eine Kopie in diesem Browser. Öffnest du den Flow wieder, bietet EasyDrag sie an, außer der Flow wurde inzwischen woanders geändert. Wird ein Flow gelöscht, hier oder anderswo, entfernt EasyDrag diese Kopie und alles andere, was sich dieser Browser für den Flow gemerkt hat (wo du ihn verlassen hast, bestätigte Testauswirkungen, den zuletzt gewählten Testauslöser).
- **Strg+S** speichert sofort. Im Editor öffnet es nie „Seite speichern“ des Browsers. In der Ansicht eines Laufs und solange ein echter Dialog offen ist, speichert es nicht. Der Schritt-Inspektor blockiert es nicht, und das automatische Speichern läuft weiter. Eine Notiz oder ein Feld, das du gerade tippst, schreibt EasyDrag vor dem Speichern, Testen, Veröffentlichen oder Verlassen. Ungültiges JSON, eine Zahl außerhalb ihres Bereichs oder ein Datenname, den der Editor ablehnt, bleibt im Feld und blockiert diese Aktion. Text, den du noch mit einer Eingabemethode zusammensetzt, wird nicht abgeschnitten.

## Testen

Der Dialog unterscheidet **Echter Test** und **Simulation**. Ein echter Test kann die
aufgeführten Aktionen ausführen. In der Simulation laufen reine Verarbeitungsschritte
lokal; andere unterstützte Schritte verwenden nur Deine Beispielausgaben. Fehlende
oder unpassende Beispiele stoppen die Simulation. Ihre Laufzeit hat keinen Zugriff
auf Werkzeuge, Modelle, Geräte oder Geheimnisse.

Die Beispielausgaben sind JSON mit Schritt-IDs als Schlüsseln und enthalten `output`
sowie optional `ports` oder einen unterstützten `error_code` für einen Fehlerfall.
Ohne Speichern bleiben sie im geöffneten Fenster. Ergebnisse sind ausdrücklich
als simuliert markiert und erscheinen nicht als echte Läufe in der Historie. Ergänze
ausgeblendete oder ausgelassene Werte in bearbeiteten Testdaten vor einem echten Test.
Dateireferenzen in simulierten Ausgaben bleiben Text: Sie öffnen keine Dateien und
laden keine Medienvorschau. Ein erfolgreicher simulierter Schritt bedeutet nicht,
dass eine externe Aktion stattgefunden hat.

Einfache Text-, Zahlen- und Ja/Nein-Felder lassen sich direkt bearbeiten; verschachtelte
Werte bleiben unter **Erweitertes Beispiel-JSON** erreichbar. In der Simulation
kannst Du Eingaben, Beispielausgaben und **Erwartete Ergebnisse** als benanntes privates
Testszenario speichern. Eine Prüfung vergleicht die genaue JSON-Ausgabe eines Schritts,
seine Ausgänge oder beides und zeigt Abweichungen nach der Simulation an.

Gespeicherte Szenarien verwenden ihren ursprünglichen Datenstand. Umbenennen erhält
diese Daten; **Neues Szenario beginnen** startet mit leeren Eingaben. Ändern sich
Ausführung, Parameter oder Verbindungen, muss ein passendes neues Szenario angelegt
werden. Reine Positions-, Beschriftungs- und Notizänderungen machen es nicht ungültig.
Es gibt höchstens 20 Szenarien pro Flow und 256 KiB pro Szenario. Ihre Daten werden auf
dem AuraGo-Server verschlüsselt gespeichert und nicht mit dem Flow exportiert.
Eine vollständige Sicherung braucht auch den passwortgeschützten Vault-Export; ohne
den zugehörigen Schlüssel können diese Daten nicht wiederhergestellt werden.

- **Testen** speichert zuerst den Entwurf; einen Entwurf, der sich nicht speichern lässt, testet EasyDrag nicht. Der Flow braucht einen eingeschalteten Auslöser.
- Der Testdialog zeigt die Daten, mit denen der Auslöser den Test startet. Hat der Flow mehrere Auslöser, wählst du einen unter *Starten mit*. Ändere das JSON, um andere Fälle auszuprobieren; *Diese Daten für spätere Tests merken* behält deine Änderung.
- Gespeicherte Testdaten zeigen geheime Werte als `[redacted]`. Unverändert nutzt der Test die echten gespeicherten Daten. Ersetze exakte Platzhalter für ausgeblendete oder ausgelassene Werte in bearbeiteten Daten vor einem echten Test; unvollständige bearbeitete Daten werden abgelehnt.
- Ein Test hat echte Auswirkungen. Der Dialog nennt jede Art (*Sendet Nachrichten*, *Schreibt Dateien*, *Steuert Geräte*, …) mit den Schritten, die sie auslösen können. Jeder Schritt zählt mit dem, was sein Baustein standardmäßig tut, und mit dem, was seine tatsächlichen Einstellungen tun; eine HTTP-Anfrage mit POST zählt deshalb als Senden, obwohl der Baustein allein das nicht tut. Kann AuraGo die Einstellungen nicht prüfen oder kann ein Baustein seine Auswirkungen nicht nennen, sagt der Dialog, dass sich die Auswirkungen nicht vollständig prüfen ließen. **Test starten** bestätigt die genannten Arten, sobald der Test gestartet ist, solange der Flow offen bleibt; *Für diesen Flow nicht mehr fragen* merkt sie auf diesem Gerät. Ein Test, der nicht startet (AuraGo lehnt ihn ab oder ist nicht erreichbar), bestätigt nichts. Eine Art, die du noch nicht bestätigt hast, fragt EasyDrag wieder ab.
- **Schritt testen** in der Detailansicht (auch *Diesen Schritt testen* an der gewählten Karte und in ihrem Kontextmenü) führt diesen Schritt zusammen mit den Schritten davor aus. Sein Dialog fragt deshalb nach den Auswirkungen jedes Schritts, der läuft: des Schritts selbst und der Schritte davor, die der gewählte Auslöser erreicht. Ein ausgeschalteter Schritt läuft nicht, ebenso wenig die Schritte, zu denen nur er führt.
- **Stoppen** im Kopf bricht den Test oder Lauf ab, den du in diesem Fenster gestartet hast, ohne Rückfrage (auch einen Lauf, den du mit *Jetzt ausführen* gestartet hast). Andere Läufe, die noch nicht beendet sind, stoppst du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)). Solange dieser Test oder Live-Lauf noch startet oder läuft, startet EasyDrag keinen weiteren und schickt einen ungewissen Start nicht erneut. Stoppen macht bereits gelaufene Schritte nicht rückgängig. Das automatische Speichern bleibt unverändert. Die Lauf-Liste öffnet sich im Editor, unter dem Kopf und einem Lauf-Banner.
- Ein Test führt den Entwurf aus, den du siehst: Wurde er inzwischen in einem anderen Fenster geändert, lehnt EasyDrag den Test ab (*Der Flow wurde woanders geändert*). Lade den Flow neu und teste erneut.

## Veröffentlichen

Beim Veröffentlichen einer neuen Flow- oder Bausteinversion kannst Du eine optionale
**Versionsnotiz** ergänzen. Sie wird zusammen mit der Version gespeichert. Im
**Versionsverlauf** gibt es getrennte Aktionen zum Lesen und Bearbeiten; eine leer
gespeicherte Notiz entfernt ihren Text. Eine Notizänderung verändert weder den Entwurf
noch den veröffentlichten Graphen oder Testergebnisse. Leerzeichen bleiben erhalten;
erlaubt sind bis zu 2.048 Zeichen und 4.096 UTF-8-Bytes. Verborgene Notizen musst Du
ausdrücklich mit frischem Text ersetzen. Gib auch dort keine Geheimnisse ein.
Meldet die Veröffentlichung ein Problem mit Mission Control oder Timern, sind Version
und Notiz bereits gespeichert. Erneutes Veröffentlichen desselben Entwurfs behält die
Notiz; ändere sie getrennt über **Notiz bearbeiten**. Konflikte erhalten Deine Eingabe
und wiederholen keine Schreibaktion automatisch. Bei einer Schreibsperre bleibt Dein
Text erhalten, solange Du ihn noch lesen darfst; öffne den Dialog nach Wiederherstellung
des Zugriffs erneut.

Die Prüfung zeigt benannte Änderungen an Schritten und Parametern mit Vorher/Nachher-Werten,
soweit diese sicher angezeigt werden können. Dazu kommt der Testnachweis für genau diesen
Entwurf in diesem Editor; eine weitere Bearbeitung macht ihn ungültig. Geheimnisse bleiben
verborgen und große Vergleiche können als unvollständig markiert sein. Ein erfolgreicher
Test garantiert nicht, dass eine spätere Live-Aktion gelingt.

- Der Veröffentlichen-Dialog nennt blockierende Probleme, was der Flow außerhalb von AuraGo tut, Daten von außen, die sensible Schritte erreichen, Hinweise und die Änderungen seit der letzten Veröffentlichung. Mit blockierenden Problemen lässt er sich nur schließen; ein Klick auf ein Problem öffnet seinen Schritt.
- **Versionsverlauf** (Menü ⋯ oder das Menü *Flow*) listet die veröffentlichten Versionen, die neueste zuerst, mit Datum, Anzahl der Schritte und Verbindungen und *Live* bei der Version, die läuft. **Als Entwurf wiederherstellen** ersetzt deinen Entwurf nach einer Rückfrage durch diese Version; die Live-Version bleibt, bis du veröffentlichst. Einen Entwurf, der sich vorher nicht speichern lässt (offline, ungültig), ersetzt EasyDrag nicht. EasyDrag behält die letzten 50 veröffentlichten Versionen; die Ansicht eines älteren Laufs, dessen Version aufgeräumt wurde, sagt das.
- *Jetzt aktivieren, damit die Auslöser greifen* schaltet den Flow mit der Veröffentlichung ein. Danach schaltest du ihn mit dem Schalter **Aktiv** aus und wieder ein. Einen unveröffentlichten Flow kannst du nicht einschalten; EasyDrag bietet an, ihn zuerst zu veröffentlichen.
- Die Bausteine-Leiste markiert Bausteine mit echten Auswirkungen außerhalb von AuraGo. Die Markierung betrachtet die Standardeinstellungen eines Bausteins (bei einem allgemeinen Werkzeug seine weitreichendste Operation) und ist keine Sicherheitsgarantie. Der Veröffentlichen-Dialog nennt die Auswirkungen deiner tatsächlichen Einstellungen; der Testdialog nennt zusätzlich die der Standardeinstellungen der Bausteine.
- **Mit einem Problem veröffentlicht** heißt: Die neue Version läuft, aber Mission Control oder die Timer ließen sich nicht aktualisieren. Der Kopf zeigt dann *Veröffentlichung unvollständig*, und **Veröffentlichen** bleibt verfügbar: Veröffentliche erneut, um den Vorgang abzuschließen.
- Fehlt der Mission-Control-Eintrag des Flows, verwende **Mission-Control-Verknüpfung reparieren**. Die bestehende Verknüpfung wird deaktiviert wiederhergestellt; Flow, Versionen und Laufhistorie bleiben erhalten. Prüfe den Flow, bevor Du ihn ausdrücklich aktivierst.
- *Daten von außen erreichen einen sensiblen Schritt* (etwa ein Mail-Betreff in einem Shell-Befehl, ein Dateipfad oder ein Empfänger) ist ein Hinweis, kein blockierendes Problem: AuraGo kann nicht erkennen, ob der Schritt die Daten prüft, und viele Flows reichen Text von außen bewusst weiter. Solche Flows lassen sich veröffentlichen und ausführen; stelle sicher, dass sich der Schritt nicht missbrauchen lässt. Daten von außen, die Schema, Host, Port oder Benutzernamen einer Webadresse oder das Ziel eines Geheimnisses bestimmen, sperren die Veröffentlichung und lassen den Schritt auch in Tests und älteren veröffentlichten Versionen scheitern. Schreib die vertrauenswürdige Adresse bis zum ersten `/`, `?` oder `#` nach dem Host direkt in den Schritt; Daten dürfen weiter Pfad, Abfrage, Header und Inhalt füllen. Ein Schritt, der Daten von außen liest, gilt mit allen Ausgabefeldern als ungeprüft, auch mit seinen festen Feldern.
- Ein Header wie *Authorization*, *X-Api-Key* oder *Cookie* mit eingetipptem Wert ist ebenfalls ein Hinweis: Der Wert liegt im Klartext im Flow und seinen Versionen. Lege ihn als Geheimnis an und nutze das Feld *Geheimnis*. Exporte und die Ansicht eines Laufs zeigen solche Werte als `[redacted]`.

## Läufe und Benachrichtigungen

**Läufe vergleichen** stellt zwei abgeschlossene Läufe aus der geladenen, gefilterten
Liste gegenüber (höchstens 200 zur Auswahl). Du siehst geänderte Eingaben, Ausgaben,
Verzweigungen und Zustände. Gekürzte oder ausgeblendete Daten erlauben keinen sicheren
Gleichheitsvergleich und werden als unvollständig gekennzeichnet. **Diagnose herunterladen**
exportiert nur technische Metadaten, ohne Namen, Dateninhalte oder Fehlermeldungstexte.
Beim Vergleichen wird kein Schritt erneut ausgeführt.

- **Läufe** zeigt zunächst die letzten 50 Läufe, gefiltert nach *Alle*, *Fehler*, *Tests* oder *Live*. **Ältere Läufe laden** holt die nächste aufbewahrte Seite; die Suche durchsucht die geladenen Einträge. Aktualisieren beginnt wieder bei der neuesten Seite. Ein Klick öffnet die genaue Flow-Version mit den gespeicherten Ein- und Ausgaben, nur zum Lesen. Schrittverlauf und Fehlerhinweis helfen bei der Diagnose. **Zurück zum Entwurf** oder **Esc** führt zurück. Die Bausteine-Leiste bleibt in der Laufansicht verborgen; ein Fehlerlauf öffnet sich beim gescheiterten Schritt. Ein weiterhin sichtbarer Speicherhinweis trägt **Entwurf: …** und gehört zu Deinem bearbeitbaren Entwurf, nicht zum aufgezeichneten Lauf.
- **Mit diesen Daten erneut testen** öffnet den Testdialog mit den Auslöserdaten des betrachteten Laufs. Ersetze ausgeblendete oder ausgelassene Werte vor einem echten Test. Der Dialog nennt die echten Auswirkungen und fragt vor dem Start; Daten werden nur mit *Diese Daten für spätere Tests merken* gespeichert. Der Knopf fehlt während eines laufenden Tests und bei unvollständig aufbewahrten Auslöserdaten.
- Ein Lauf, der noch nicht beendet ist (*In Warteschlange* – auch wenn er auf einen freien Platz wartet – oder *Läuft*), zeigt in der Liste und im Banner seiner Ansicht **Stoppen**, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat. Außer bei einem Test fragt EasyDrag vorher nach; Schritte, die schon gelaufen sind, werden nicht rückgängig gemacht. Auch ein schreibgeschützter Desktop bietet **Stoppen** an, denn Stoppen ändert keinen Flow; schreibgeschützte Missionen lehnen es ab.
- **Jetzt ausführen** (Menü ⋯, das Menü *Flow* oder das Menü einer Karte auf der Startseite) startet die veröffentlichte Version. Der Flow muss **Aktiv** sein: Solange er pausiert ist, ist *Jetzt ausführen* ausgeschaltet, und sein Tooltip sagt, dass du den Flow zuerst einschalten musst, so wie Mission Control eine pausierte Mission nicht ausführt. **Testen** geht in beiden Fällen.
- Ein Flow, der in seinen Einstellungen auf *Überspringen* steht, überspringt neue Läufe, solange noch einer läuft: **Jetzt ausführen** startet dann nichts und sagt das.
- Ein fehlgeschlagener Live-Lauf benachrichtigt so, wie es die Flow-Einstellungen sagen: auf dem Desktop (Standard), per Push, per Telegram, per WhatsApp oder gar nicht. Ein Flow benachrichtigt einmal, wenn er zu scheitern beginnt, danach höchstens einmal pro Stunde, solange er weiter scheitert. Ein Klick auf die Desktop-Benachrichtigung öffnet EasyDrag bei diesem Lauf.
- Nach einem Neuladen des Desktops öffnet sich ein offener Flow wieder.
- Beendet ein *Stopp*-Schritt einen Lauf, während andere Schritte noch laufen, haben diese bis zu 30 Sekunden Zeit; danach werden sie gestoppt und zeigen *Wirkung möglicherweise erfolgt*: Was sie senden oder ändern, kann schon passiert sein. Scheitert einer davon nach einem Stopp mit *Erfolg*, gilt der Lauf als fehlgeschlagen; nach einem Stopp mit *Fehler* bleibt dessen Meldung.
- Ein *Warten*-Schritt gibt seinen Platz frei, solange er wartet, damit wartende Flows andere nicht aufhalten; danach reiht sich der Lauf wieder für einen freien Platz ein. Höchstens 64 Läufe warten gleichzeitig so; darüber hinaus behält ein Warten seinen Platz.
- Ein Flow mit Nebenläufigkeit *parallel* belegt höchstens die Hälfte der gleichzeitigen Läufe. Insgesamt warten höchstens 200 Läufe (und 64 MiB ihrer Auslöser-Daten) auf ihren Start; darüber hinaus werden neue Auslöser verworfen, und Mission Control vermerkt das am Flow.
- Live- und Testläufe werden getrennt aufbewahrt: *Gespeicherte Läufe pro Flow* zählt die Live-Läufe, die neuesten 20 Testläufe bleiben unabhängig davon erhalten. Sehr große Läufe behalten nur die Ein- und Ausgaben ihrer ersten Schritte (4 MiB pro Lauf); spätere Schritte zeigen *Gekürzt*.

### Lokale Läufe fortsetzen

**Lokale Läufe nach Neustart fortsetzen** ist unter **Einstellungen → EasyDrag-Flows**
standardmäßig ausgeschaltet. Einschalten erfordert einen Neustart. Ausschalten gilt ohne
Neustart und erhält gespeicherte Läufe. Der beobachtete Verarbeitungsstatus und die Zähler
beschreiben den Server; ein ungespeicherter Schalter ändert sie nicht. Unbekannte Zähler
werden als unbekannt angezeigt.

Dieser erste Modus unterstützt einen manuellen Auslöser, Felder setzen, Wenn, Verzweigung,
Zusammenführen, Stopp und **Fortsetzbare Wartezeit**. Geprüft wird der gesamte Graph, auch
deaktivierte oder unerreichbare Schritte. Aufrufe, Stapel, Tools, Modelle und externe Effekte
sind ausgeschlossen. Die fortsetzbare Wartezeit akzeptiert 1–86.400 Sekunden. Sie ist vom
gewöhnlichen **Warten** getrennt, dessen Verhalten unverändert bleibt. Veröffentliche den
geeigneten Flow und starte ihn ausdrücklich als Live-Lauf; Entwurfstests und Simulationen
starten keine Fortsetzung.
Die Wartezeit muss in die ursprüngliche Gesamtlaufzeit passen. Datumsabhängige Ausdrücke
werden abgewiesen, wenn die ursprüngliche Zeitzone nicht sicher wiederherstellbar ist.

Die vorhandene Laufansicht unterscheidet **Wartend** und **Fortsetzung blockiert** und zeigt,
soweit verfügbar, die beobachtete Weckzeit und das ursprüngliche späteste Laufende. Ein
blockierter Lauf gilt dadurch weder als fehlgeschlagen noch als abgeschlossen. Die Erklärung
nennt ausgeschaltete Verarbeitung, fehlende Freigabe, nicht verfügbare Schlüssel oder Speicher
beziehungsweise inkompatiblen Zustand. Verwende bei der Reparatur den passenden Tresor und die
passende Sicherung; die Oberfläche ersetzt keinen fehlenden Schlüssel und verwirft keinen
Zwischenstand automatisch.

Ein ausdrücklich verfolgter aktiver Lauf behält bei neuer Verbindung und Fensterwiederherstellung
seine Lauf-ID. Sein genauer gespeicherter Graph öffnet sich schreibgeschützt; Dein aktueller
Entwurf bleibt getrennt. Abgeschlossene, bestätigte Schritte bleiben erhalten. Rein lokale
Berechnungen, die vor einem Absturz noch nicht bestätigt gespeichert waren, können wiederholt
werden. Die ursprüngliche Laufzeitgrenze gilt weiter; gespeicherte Ausgabevorschauen können
weiterhin gekürzt sein.
Eine Veröffentlichung als v2 ersetzt keinen bereits zugelassenen v1-Lauf und verlängert
dessen ursprüngliche Frist nicht.

**Stoppen** fordert den Abbruch des beobachteten gespeicherten Zustands an. **Wird gestoppt…**
beweist noch kein Laufende; die Ansicht wartet auf die Serverbestätigung. Hat sich der Zustand
geändert, lade ihn neu und prüfe ihn vor einem weiteren Stopp. Unsichere Anfragen werden nie
automatisch erneut gesendet. Du brauchst die aktuelle Stopp-Berechtigung. Für diese fortsetzbaren
Läufe bleibt Stoppen auch bei schreibgeschützten Missionen, schreibgeschütztem Desktop oder
gesperrter App erlaubt; die Verarbeitung wird dadurch nicht eingeschaltet. Für gewöhnliche,
nicht fortsetzbare Läufe gilt die bisherige Missions-Schreibschutzgrenze weiter.
Ein inkompatibler alter Zwischenstand lässt sich getrennt abbrechen,
selbst wenn sein historischer Lauf bereits beendet ist; dessen ursprüngliches Ergebnis bleibt.
Mission Control lehnt einen missionsweiten Stopp mit gespeicherten Läufen ab, bevor ein Lauf
gestoppt wird. Öffne den gewünschten Lauf in EasyDrag und stoppe ihn dort einzeln.

Dieser Modus startet keine abhängigen Missionen, Planer-Folgeaktionen oder Abschlussbenachrichtigungen.
Der Missionsverlauf kann den Lauf zeigen, sobald seine Quelle verfügbar ist; der kumulative
Laufzähler der Mission bleibt in diesem ersten Modus unverändert. Einzelne Bausteinaufrufe
erhalten keinen eigenen Verlauf.
Ein aktiver Verlaufseintrag ohne prüfbare Quelle zeigt **Unbekannt**; sein zuletzt erfasster
Zustand bestätigt keinen aktuellen Fortschritt.

## Decisions-Baustein

*Entscheidung* nutzt den optionalen, von AuraGo verwalteten Decisions-Dienst. Er kann nativ oder in Docker laufen. Der Baustein nimmt einen Zustandswert und bis zu acht registrierte Bildreferenzen an; Vorlagen können beides liefern. Der Server löst Bildreferenzen innerhalb der AuraGo-Mediengrenze auf, bevor das Modell sie erhält. Fehlt der Dienst oder ist er nicht verfügbar, zeigt der Baustein an, dass eine Einrichtung nötig ist.

- *Eine weiterleiten* stellt eine einzelne *Auswahl*- oder Ja/Nein-Frage (*Noul*). Jede Auswahl hat eine stabile ID und einen eigenen Ausgang `choice_<id>` sowie *Unsicher*. Beim Umsortieren bleiben Verbindungen erhalten; eine geänderte ID erzeugt einen anderen Ausgang. Antworten unter der Konfidenzschwelle (Standard 0,70) gehen zu *Unsicher*.
- *Alle auswerten* nimmt bis zu 16 Fragen in ihrer Reihenfolge entgegen und liefert ein `answers`-Objekt mit Frage-IDs als Schlüsseln. Fragetypen: *Auswahl*, *Ja/Nein* und *Bewertung*. Liegt mindestens eine Antwort unter der Schwelle, geht der Flow zu *Unsicher*, sonst über `out` weiter.
- Jede Frage hat Vorlagen-fähige Anweisungen. Auswahl-IDs sind stabile Bezeichner, ihre Namen dienen der Anzeige. Bewertungsoptionen erhalten unterschiedliche Zahlenwerte (standardmäßig ihre nullbasierte Reihenfolge); das Ergebnis enthält den Erwartungswert. Ja/Nein verwendet immer die festen IDs `no` und `yes`.
- Modellausgaben sind nicht vertrauenswürdige Daten. Technische Fehler stoppen den Lauf oder nehmen den *Fehlerpfad*; *Weitermachen* steht nicht zur Auswahl, da ein fehlgeschlagener Aufruf kein gültiges Entscheidungsergebnis hat.
- GPU/iGPU-Inferenz ist standardmäßig erforderlich. CPU-Fallback muss ausdrücklich unter **Config → Decisions** freigegeben werden und kann dort wieder entzogen werden. Status und fehlende Einrichtung zeigt der Baustein an.

## Wenn ein Schritt scheitert

- In den *Einstellungen* eines Schritts wählst du unter *Wenn der Schritt fehlschlägt*: *Lauf stoppen*, *Weitermachen* (die nächsten Schritte erhalten den Fehler als Daten; bei einem *Wenn*-Schritt über *Nein*, bei einem *Verzweigung*-Schritt über *Sonst*, nie über *Ja* oder einen Fall) oder *Fehlerpfad* (ein eigener Ausgang *Fehler* für Schritte, die auf den Fehler reagieren). *Wiederholungen*, *Sekunden dazwischen* und *Zeitlimit (s)* stehen daneben.
- Der *Datenname* im selben Tab ist der Name, unter dem spätere Schritte die Ausgabe lesen (`{{datenname.feld}}`).

## Geheimnisse

- API-Schlüssel für HTTP-Anfragen legst du als Geheimnis an: **Neues Geheimnis** im Feld *Geheimnis* des Schritts. Ein Wert darf bis zu 4 KiB groß sein, und AuraGo nimmt höchstens 30 Änderungen an Geheimnissen pro Minute an. Gibt es den Namen schon, fragt EasyDrag, bevor es den Wert ersetzt.
- Geheimnisse liegen im Vault als `easydrag_<name>`. Flows können sie nutzen; der Agent kann sie weder auflisten noch lesen oder ändern. Lauf-Daten und die Ansicht eines Laufs zeigen ihre Werte geschwärzt, mit einer Grenze für sehr kurze Werte (siehe [Kapitel 11](11-missions.md#flow-geheimnisse)).
- Der Papierkorb neben **Neues Geheimnis** löscht das gewählte Geheimnis nach einer Rückfrage und leert das Feld. Flows, die das Geheimnis nutzen, schlagen fehl, bis es wieder gesetzt ist; nutzen veröffentlichte Flows es noch, nennt eine Benachrichtigung sie.
- Solange ein Schritt ein Geheimnis nutzt, dürfen Daten von außen (eine Mail, ein Webhook, ein anderer Web-Schritt) nicht bestimmen, wohin es geht. Schreib die Adresse selbst bis zum ersten `/`, `?` oder `#` nach dem Host aus, zum Beispiel `https://crm.example/api/contacts?email={{trigger.data.from}}`: Daten dürfen dann Pfad und Abfrage füllen, aber nicht Schema, Host, Port oder Benutzernamen. `https://api.example.com{{…}}` oder `https://{{…}}/` lehnt das Veröffentlichen ab, und eine Version, die vor dieser Regel veröffentlicht wurde, lässt diesen Schritt scheitern, statt das Geheimnis zu senden. Header dürfen Daten enthalten. *Header-Name* und *Wert-Präfix* musst du ausschreiben, solange ein Geheimnis gesetzt ist.

## Vorlagen, Import und Export

- Die Startseite bietet zwölf Vorlagen, darunter *Dein erstes lokales Ergebnis*, E-Mail- und Gerätebenachrichtigungen, einen Hinweis bei Personenerkennung, eine MQTT-Temperaturwarnung und eine manuelle Notiz als PDF. Im Editor tragen Schritte, deren Integration noch nicht eingerichtet ist, ein Abzeichen, und ihre Detailansicht verweist auf die Einstellungen (*Einstellungen öffnen*); in der Bausteine-Leiste bieten solche Bausteine *Einrichten* an.
- Die neuen Benachrichtigungsvorlagen wählen standardmäßig Web Push; richte es ein oder wähle vor der Veröffentlichung einen anderen Kanal. Wähle für die Temperaturwarnung ein MQTT-Topic und prüfe den Schwellenwert. Fehlende oder nichtnumerische Temperaturen stoppen den Lauf mit einem Fehler; eine erfolgreiche Simulation mit Beispieldaten beweist nicht, dass Sensor oder Benachrichtigungskanal funktionieren. Das Kamerabeispiel sendet Text ohne Bildanhang. Simulierte Benachrichtigungs- und PDF-Ergebnisse senden keine Nachricht und erzeugen keine Datei.
- Die E-Mail-, Geräte- und Kamerabeispiele starten mit breiten Filtern: Sie erfassen passende Nachrichten aus dem überwachten Posteingang, das Trennen von Geräten oder Personenerkennungen scharfgeschalteter Kameras aus den eingerichteten Quellen. Setze vor der Veröffentlichung die gewünschten Filter für Betreff/Absender, Gerät, Kamera oder Zone, wenn der Flow enger reagieren soll.
- **Exportieren** lädt den Entwurf als `<name>.easydrag.json` herunter. **Importieren** auf der Startseite legt aus so einer Datei einen neuen Flow an (höchstens 2 MiB). Vault-Verweise enthalten Geheimnisnamen. Zugangs-Header (*Authorization*, *X-Api-Key*, *Cookie*, ...) werden als `[redacted]` exportiert; registrierte Geheimnisse in Gruppentiteln und -notizen werden maskiert. Überschreitet der maskierte Text die Längenbegrenzung, wird er mit Auslassungspunkten gekürzt. Andere gewöhnliche Felder werden wie eingegeben exportiert und können sensible Werte enthalten. Ein Export mit visuellen Gruppen braucht eine aktuelle AuraGo-Version, die sie unterstützt; ältere Versionen lehnen ihn ab, statt die Gruppen stillschweigend zu entfernen.

## Mission Control, Missionsseite und Dashboard

- In Mission Control tragen Flows das Abzeichen *EasyDrag*, und der Filter *Flows* zeigt nur sie. **In EasyDrag öffnen** ersetzt *Bearbeiten*; **Neuer Flow** öffnet die Startseite von EasyDrag. Zeitpläne lesen sich wie „Mo–Fr um 07:00“ (Uhrzeiten im Format deiner Sprache), und eine Mission, die noch nie lief, zeigt *Noch nie gelaufen*.
- Läufe, die nur auf einen freien Platz warten, kannst du in Mission Control nicht abbrechen; stoppe sie hier unter **Läufe**.
- *Ausführen* in Mission Control antwortet *übersprungen*, wenn der Flow nur einen Lauf zur Zeit erlaubt und gerade einer läuft.
- Ausführen, Pausieren und Löschen von Flow-Missionen, die Missionsseite und das Dashboard beschreibt [Kapitel 11: Flow-Missionen](11-missions.md#flow-missionen-easydrag).

## Einstellungen

*Config → Agent-Tools → EasyDrag-Flows*:
- *Flows aktivieren*, *Gleichzeitige Läufe (1–32)*, *Gleichzeitige Schritte pro Lauf (1–16)*, *Laufhistorie aufbewahren (Tage, 1–365)* und *Gespeicherte Läufe pro Flow (10–5000)*. Änderungen daran wirken nach einem Neustart; schaltest du *Flows aktivieren* aus, ist EasyDrag sofort abgeschaltet (siehe [Wenn EasyDrag abgeschaltet ist](#wenn-easydrag-abgeschaltet-ist)), veröffentlichte Flows starten von da an aber von selbst keine neuen Läufe mehr (Zeitpläne, Ereignisse, Datum/Uhrzeit-Timer); ein Lauf, der schon läuft, geht weiter, bis du ihn unter **Läufe** stoppst.
- *KI-Anbieter für KI-Schritte*: Wähle einen Anbieter aus deiner Provider-Liste. KI-Schritte, die auf „Standardmodell“ stehen, nutzen ihn; ohne Auswahl nutzen sie das Hauptmodell. Die Einstellung gilt sofort.
- Die Agenten-Optionen (*Agent darf Flows nur lesen*, *Agent darf Flows veröffentlichen*) sind gesperrt. Sie greifen, sobald der Agent mit Flows arbeiten kann.
- *EasyDrag öffnen* öffnet die App auf dem Desktop.

## Wenn EasyDrag abgeschaltet ist

EasyDrag braucht *Flows aktivieren* und das Missions-Tool (`tools.missions.enabled`, siehe [Kapitel 11](11-missions.md#voraussetzungen)). Ist eins davon aus, verschwindet das EasyDrag-Symbol, sobald der Desktop seine App-Liste das nächste Mal lädt. Lädt EasyDrag in diesem Zustand (ein neues Fenster, **Erneut versuchen** oder das Öffnen eines Flows), zeigt es *EasyDrag ist abgeschaltet* mit **Einstellungen öffnen** und **Erneut versuchen**. Die Startseite zeigt dieselbe Karte mit beiden Schaltflächen statt deiner Flows, sobald sie ihre Liste das nächste Mal lädt; Neuer Flow, Importieren und die Vorlagen sind dann gesperrt. Ein schon geöffneter Flow bleibt im Editor; sein nächstes Speichern zeigt *Offline – neuer Versuch läuft*, bis EasyDrag wieder eingeschaltet ist. Flow-Missionen bleiben in Mission Control, ihre Zeitpläne, Ereignisse und Datum/Uhrzeit-Timer starten aber keine Läufe, solange EasyDrag abgeschaltet ist, und *Jetzt ausführen* in Mission Control wird abgelehnt. Einen schon laufenden Lauf kannst du weiter stoppen.

## Tastenkürzel

Strg+S, Strg+Enter und Strg+K wirken im Editor, auch solange der Schritt-Inspektor offen ist. Sie warten, solange ein echter Dialog oder ein Kontextmenü offen ist. Die übrigen Tasten wirken auf der Arbeitsfläche, nicht während du in einem Feld tippst. Auf dem Mac steht ⌘ für Strg. **Tab** verschiebt den Fokus und fügt keinen Schritt hinzu. Die Arbeitsfläche ist ein einziger Tab-Stopp; die Zoom-Knöpfe folgen. Die Werkzeugknöpfe des ausgewählten Schritts kommen dazu. Dieselben Aktionen gibt es als Knopf oder Taste: **Hinzufügen** setzt den nächsten Schritt an die Karte, **Verbinden** (C) öffnet eine Liste passender Schritte und Anschlüsse, D schaltet den Schritt aus oder ein, Strg+D dupliziert, Entf löscht. Tippt du einen Anschluss an und lässt ihn ohne Ziel los, öffnet dieselbe Liste mit dieser Seite und diesem Anschluss. *Schritt testen* steht an der Karte und im Inspektor (Enter öffnet den Inspektor). Screenreader finden zusätzlich eine Liste der Schritte neben der Fläche. Die Liste ist für den Lesemodus da; mit echten Screenreader-Nutzern wurde sie nicht geprüft.

| Taste | Wirkung |
|---|---|
| Tab | Fokus verschieben. Fügt keinen Schritt hinzu |
| Enter | Schritt-Inspektor öffnen |
| C | Ausgewählten Schritt verbinden (Schritt und Anschluss wählen) |
| D | Schritte aus- oder einschalten |
| Entf | Auswahl löschen |
| Pfeiltasten | Zwischen Schritten wechseln |
| Umschalt+Pfeiltasten | Auswahl verschieben |
| Leertaste halten | Fläche verschieben |
| Umschalt+1 | Ganzen Flow zeigen |
| + / − | Hinein- oder herauszoomen |
| Strg+0 | Auf 100 % zoomen |
| Strg+Z / Strg+Umschalt+Z | Rückgängig / Wiederholen |
| Strg+C / X / V / D | Kopieren / Ausschneiden / Einfügen / Duplizieren |
| Strg+A | Alles auswählen |
| Strg+S | Jetzt speichern |
| Strg+Enter | Testen |
| Strg+K | Vom einen ausgewählten Schritt aus dessen Ausgang anfügen; sonst Bausteine suchen |
| Strg+F | Schritt im Flow suchen |
| Alt+Umschalt+F | Automatisch anordnen |
| ? | Diese Kürzel zeigen |
| Esc | Schließen oder Auswahl aufheben; in der Ansicht eines Laufs zurück zum Entwurf |

---

**Siehe auch:** [Kapitel 11: Mission Control](./11-missions.md) · [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 14: Sicherheit](./14-security.md)

---

**Vorheriges Kapitel:** [Kapitel 23: Interna](./23-internals.md)
**Nächstes Kapitel:** [Kapitel 25: CYD Creator](./25-cyd-creator.md)
