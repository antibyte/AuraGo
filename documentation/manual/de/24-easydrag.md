# Kapitel 24: EasyDrag – Automationen per Drag & Drop

EasyDrag ist der visuelle Editor für Flows. Ein Flow ist eine Mission, die aus Bausteinen besteht: einem oder mehreren **Auslösern** (Zeitplan, Datum & Uhrzeit, Webhook, E-Mail empfangen, MQTT-Nachricht, Home-Assistant-Zustand, Gerät, Telefonanruf über die Fritz!Box, Planer, AuraGo-Start, Budget-Warnung, Mission beendet oder Manueller Start) und danach **Schritten** wie Websuche, KI-Schritt, PDF erstellen, Telegram-Nachricht, E-Mail senden, Home Assistant oder jedes freigegebene Werkzeug. Flows laufen an der Warteschlange des Agenten vorbei und erscheinen in Mission Control mit ihrem Verlauf.

Dieses Kapitel beschreibt den Editor. Wie sich Läufe, Grenzen, Fehler und Geheimnisse auf dem Server verhalten, steht in [Kapitel 11: Flow-Missionen (EasyDrag)](11-missions.md#flow-missionen-easydrag).

## Der erste Flow

1. Öffne **EasyDrag** auf dem Desktop. Wähle **Neuer Flow** und gib ihm einen Namen, oder starte mit einer **Vorlage**.
2. Klicke **Auslöser wählen** und such dir einen Auslöser aus.
3. Wähle den Auslöser aus und drücke **Tab**: Die Schnellsuche fügt den nächsten Schritt direkt dahinter ein und verbindet ihn. Alternativ ziehst du Bausteine aus der Leiste links auf die Fläche oder auf eine Verbindung.
4. Doppelklicke einen Schritt (oder drücke **Enter**), um ihn zu bearbeiten. Links siehst du die Daten früherer Schritte, aus dem letzten Test oder als Beispiel. Ziehe einen Wert in ein Feld, oder klicke ihn an, um ihn an der letzten Cursorposition einzufügen. Bezüge erscheinen als farbige Chips mit Schritt und Feld, etwa „Websuche › results“. Im Text stehen sie als `{{websuche.results}}`.
5. **Testen** (Strg+Enter) speichert den Entwurf und führt ihn aus. Die Schritte leuchten nacheinander auf, an den Verbindungen steht die Anzahl der Einträge. Ein Test hat echte Auswirkungen; siehe [Testen](#testen).
6. **Veröffentlichen** prüft den Flow, zeigt, was er außerhalb von AuraGo tut, und schaltet ihn auf Wunsch gleich aktiv. Nur veröffentlichte Flows laufen selbstständig. Dein Entwurf bleibt bearbeitbar; die veröffentlichte Version läuft.

## Der Editor

- Der Kopf zeigt Name und Stand des Flows (*Entwurf*, *Veröffentlicht*, *Unveröffentlichte Änderungen*, *Inaktiv*) und enthält **Läufe**, **Testen**, **Veröffentlichen**, den Schalter **Aktiv** und das Menü **⋯**: *Jetzt ausführen*, *Flow-Einstellungen*, *Duplizieren*, *Exportieren*, *In Mission Control zeigen* und *Löschen*. Die Fußzeile zeigt den letzten Lauf, Probleme und Hinweise sowie den Speicherstand.
- Ein Flow öffnet sich in lesbarer Größe (mindestens 80 %) und beginnt bei seinem Auslöser. Auf diesem Gerät merkt sich EasyDrag, wo du jeden Flow verlassen hast. **Alles zeigen** (Umschalt+1) zeigt den ganzen Flow; zoomt das weit heraus, zeigen die Karten nur noch ihre Namen, dafür größer.
- In schmalen Fenstern und auf dem Handy ist die Bausteine-Leiste anfangs zu. Geöffnet liegt sie über der Fläche und schließt sich, sobald du einen Schritt hinzufügst oder auf die Fläche tippst. Die Detailansicht eines Schritts nutzt dann Tabs.
- Die *Flow-Einstellungen* enthalten Name, Beschreibung, was passiert, wenn ein Auslöser während eines Laufs feuert (*Warteschlange*, *Parallel ausführen* oder *Überspringen*), die maximale Laufzeit und wohin fehlgeschlagene Läufe gemeldet werden.
- Ist der Desktop schreibgeschützt, öffnet EasyDrag Flows nur zum Lesen.

## Speichern

- EasyDrag speichert eine Sekunde nach deiner letzten Änderung. Die Anzeige unten rechts zeigt den Stand:
  - *Gespeichert*, *Ungespeichert* oder *Wird gespeichert…*: der normale Ablauf.
  - *Offline – neuer Versuch läuft*: AuraGo ist nicht erreichbar oder hat mit einem Serverfehler geantwortet. EasyDrag versucht es selbst wieder.
  - *Nicht gespeichert* mit **Erneut versuchen**: AuraGo hat die Änderung abgelehnt, etwa weil der Flow zu groß ist oder du Flows nicht ändern darfst. Der Tooltip nennt den Grund.
  - *Nicht gespeichert – Fehler beheben*: Der Entwurf hat Fehler, die das Speichern verhindern. Die nächste Änderung wird wieder gesendet.
  - *Konflikt*: Der Flow wurde woanders geändert. EasyDrag fragt, ob du deine Version behältst oder die andere lädst.
- Bis AuraGo deine Änderungen hat, bleibt bis zu 30 Tage eine Kopie in diesem Browser. Öffnest du den Flow wieder, bietet EasyDrag sie an, außer der Flow wurde inzwischen woanders geändert.
- **Strg+S** speichert sofort und öffnet nie „Seite speichern“ des Browsers. Solange ein EasyDrag-Dialog offen ist, speichert Strg+S nicht; das automatische Speichern läuft weiter.

## Testen

- **Testen** speichert zuerst den Entwurf; einen Entwurf, der sich nicht speichern lässt, testet EasyDrag nicht. Der Flow braucht einen eingeschalteten Auslöser.
- Der Testdialog zeigt die Daten, mit denen der Auslöser den Test startet. Hat der Flow mehrere Auslöser, wählst du einen unter *Starten mit*. Ändere das JSON, um andere Fälle auszuprobieren; *Diese Daten für spätere Tests merken* behält deine Änderung.
- Gespeicherte Testdaten zeigen geheime Werte als `[redacted]`. Lässt du sie unverändert, nutzt der Test die echten gespeicherten Daten. Geänderte Daten, die noch `[redacted]` enthalten, laufen, werden aber nicht gemerkt.
- Ein Test hat echte Auswirkungen. Der Dialog nennt jede Art (*Sendet Nachrichten*, *Schreibt Dateien*, *Steuert Geräte*, …) mit ihren Schritten, und **Test starten** bestätigt sie, solange der Flow offen bleibt. *Für diesen Flow nicht mehr fragen* merkt sie auf diesem Gerät. Eine Art, die du noch nicht bestätigt hast, fragt EasyDrag wieder ab.
- **Schritt testen** in der Detailansicht führt diesen Schritt zusammen mit den Schritten davor aus. Sein Dialog fragt deshalb nach den Auswirkungen jedes Schritts, der läuft: des Schritts selbst und der Schritte davor, die der gewählte Auslöser erreicht. Ein ausgeschalteter Schritt läuft nicht, ebenso wenig die Schritte, zu denen nur er führt.
- **Stoppen** im Kopf bricht den Test oder Lauf ab, den du in diesem Fenster gestartet hast. Andere Läufe, die noch nicht beendet sind, stoppst du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)).

## Veröffentlichen

- Der Veröffentlichen-Dialog nennt blockierende Probleme, was der Flow außerhalb von AuraGo tut, Daten von außen, die sensible Schritte erreichen, Hinweise und die Änderungen seit der letzten Veröffentlichung. Mit blockierenden Problemen lässt er sich nur schließen; ein Klick auf ein Problem öffnet seinen Schritt.
- *Jetzt aktivieren, damit die Auslöser greifen* schaltet den Flow mit der Veröffentlichung ein. Danach schaltest du ihn mit dem Schalter **Aktiv** aus und wieder ein. Einen unveröffentlichten Flow kannst du nicht einschalten; EasyDrag bietet an, ihn zuerst zu veröffentlichen.
- Die Bausteine-Leiste markiert Bausteine mit echten Auswirkungen außerhalb von AuraGo. Die Markierung betrachtet nur die Standardeinstellungen eines Bausteins und ist keine Sicherheitsgarantie. Der Veröffentlichen-Dialog nennt die Auswirkungen deiner tatsächlichen Einstellungen.
- **Mit einem Problem veröffentlicht** heißt: Die neue Version läuft, aber Mission Control oder die Timer ließen sich nicht aktualisieren. Der Kopf zeigt dann *Veröffentlichung unvollständig*, und **Veröffentlichen** bleibt verfügbar: Veröffentliche erneut, um den Vorgang abzuschließen.
- Fehlt der Mission-Control-Eintrag des Flows, hilft erneutes Veröffentlichen nicht. Exportiere den Flow, lösche ihn und importiere ihn erneut.

## Läufe und Benachrichtigungen

- **Läufe** zeigt die letzten 50 Läufe, gefiltert nach *Alle*, *Fehler*, *Tests* oder *Live*. Ein Klick öffnet die Ansicht des Laufs: die Flow-Version dieses Laufs mit allen Ein- und Ausgaben, nur zum Lesen. Ein Banner nennt den Lauf; **Zurück zum Entwurf** oder **Esc** führt zurück. In der Ansicht eines Laufs bleibt die Bausteine-Leiste verborgen, und ein fehlgeschlagener Lauf öffnet sich zentriert auf den gescheiterten Schritt.
- Ein Lauf, der noch nicht beendet ist (in der Warteschlange, wartet auf einen freien Platz oder läuft), zeigt in der Liste und im Banner seiner Ansicht **Stoppen**, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat. Bei einem Live-Lauf fragt EasyDrag vorher nach; Schritte, die schon gelaufen sind, werden nicht rückgängig gemacht. Ein schreibgeschützter Desktop bietet kein Stoppen an.
- **Jetzt ausführen** (Menü ⋯ oder das Menü einer Karte auf der Startseite) startet die veröffentlichte Version sofort.
- Ein fehlgeschlagener Live-Lauf benachrichtigt so, wie es die Flow-Einstellungen sagen: auf dem Desktop (Standard), per Push, per Telegram oder gar nicht. Ein Flow benachrichtigt einmal, wenn er zu scheitern beginnt, danach höchstens einmal pro Stunde, solange er weiter scheitert. Ein Klick auf die Desktop-Benachrichtigung öffnet EasyDrag bei diesem Lauf.
- Nach einem Neuladen des Desktops öffnet sich ein offener Flow wieder.

## Wenn ein Schritt scheitert

- In den *Einstellungen* eines Schritts wählst du unter *Wenn der Schritt fehlschlägt*: *Lauf stoppen*, *Weitermachen* (die nächsten Schritte erhalten den Fehler als Daten) oder *Fehlerpfad* (ein eigener Ausgang *Fehler* für Schritte, die auf den Fehler reagieren). *Wiederholungen*, *Sekunden dazwischen* und *Zeitlimit (s)* stehen daneben.
- Der *Datenname* im selben Tab ist der Name, unter dem spätere Schritte die Ausgabe lesen (`{{datenname.feld}}`).

## Geheimnisse

- API-Schlüssel für HTTP-Anfragen legst du als Geheimnis an: **Neues Geheimnis** im Feld *Geheimnis* des Schritts. Ein Wert darf bis zu 4 KiB groß sein, und AuraGo nimmt höchstens 30 Änderungen an Geheimnissen pro Minute an. Gibt es den Namen schon, fragt EasyDrag, bevor es den Wert ersetzt.
- Geheimnisse liegen im Vault als `easydrag_<name>`. Flows können sie nutzen; der Agent kann sie weder auflisten noch lesen oder ändern. Lauf-Daten und die Ansicht eines Laufs zeigen ihre Werte geschwärzt.
- Der Papierkorb neben **Neues Geheimnis** löscht das gewählte Geheimnis nach einer Rückfrage und leert das Feld. Flows, die das Geheimnis nutzen, schlagen fehl, bis es wieder gesetzt ist; nutzen veröffentlichte Flows es noch, nennt eine Benachrichtigung sie.

## Vorlagen, Import und Export

- Die Startseite bietet sechs Vorlagen: *KI-News als PDF per Telegram*, *Webhook per E-Mail zusammenfassen*, *Terminerinnerung*, *Licht aus beim Verlassen*, *Budgetwächter* und *Täglicher RSS-Überblick*. Schritte, deren Integration noch nicht eingerichtet ist, sind markiert; *Einrichten* öffnet die Einstellungen.
- **Exportieren** lädt den Entwurf als `<name>.easydrag.json` herunter. **Importieren** auf der Startseite legt aus so einer Datei einen neuen Flow an (höchstens 2 MiB). Eine Flow-Datei enthält die Namen von Geheimnissen, nie ihre Werte.

## Mission Control, Missionsseite und Dashboard

- In Mission Control tragen Flows das Abzeichen *EasyDrag*, und der Filter *Flows* zeigt nur sie. **In EasyDrag öffnen** ersetzt *Bearbeiten*; **Neuer Flow** öffnet die Startseite von EasyDrag. *Duplizieren* und die Missionsvorbereitung gibt es für Flows nicht.
- Pausieren, Sperren, Ausführen, Löschen und der Verlauf funktionieren wie bei anderen Missionen. *Jetzt ausführen* und *Fortsetzen* bleiben gesperrt, bis der Flow veröffentlicht ist („Noch nicht veröffentlicht“). Zeitpläne lesen sich wie „Mo–Fr um 07:00“, und eine Mission, die noch nie lief, zeigt *Noch nie gelaufen*.
- Wenn du die Mission löschst, löschst du den Flow mit Entwurf, allen veröffentlichten Versionen, den gespeicherten Daten des Auslösers und der Laufhistorie. Flow-Geheimnisse bleiben.
- *Lauf abbrechen* in Mission Control funktioniert, solange ein Lauf des Flows läuft, und nimmt die wartenden Läufe des Flows mit. Einen Lauf, der nur auf einen freien Platz wartet, kannst du dort nicht abbrechen; stoppe ihn in EasyDrag unter **Läufe**.
- Die Missionsseite (`/missions/v2`) zeigt Flows nur zum Lesen: *Bearbeiten* und *Duplizieren* sagen nur, dass der Flow in EasyDrag bearbeitet wird, und *Ausführen* wartet, bis der Flow veröffentlicht und eingeschaltet ist.
- Das Dashboard zeigt die Zeitpläne von Flows in der Cron-Liste nur zum Lesen (*Von EasyDrag verwaltet*).

## Einstellungen

*Config → Agent-Tools → EasyDrag-Flows*:
- *Flows aktivieren*, *Gleichzeitige Läufe (1–32)*, *Gleichzeitige Schritte pro Lauf (1–16)*, *Laufhistorie aufbewahren (Tage, 1–365)* und *Gespeicherte Läufe pro Flow (10–5000)*. Änderungen daran wirken nach einem Neustart.
- *KI-Anbieter für KI-Schritte*: Wähle einen Anbieter aus deiner Provider-Liste. KI-Schritte, die auf „Standardmodell“ stehen, nutzen ihn; ohne Auswahl nutzen sie das Hauptmodell. Die Einstellung gilt sofort.
- Die Agenten-Optionen (*Agent darf Flows nur lesen*, *Agent darf Flows veröffentlichen*) sind gesperrt. Sie greifen, sobald der Agent mit Flows arbeiten kann.
- *EasyDrag öffnen* öffnet die App auf dem Desktop.

## Wenn EasyDrag abgeschaltet ist

EasyDrag braucht *Flows aktivieren* und das Missions-Tool (`tools.missions.enabled`, siehe [Kapitel 11](11-missions.md#voraussetzungen)). Ist eins davon aus, verschwindet das EasyDrag-Symbol, sobald der Desktop seine App-Liste das nächste Mal lädt. Ein schon offenes Fenster zeigt *EasyDrag ist abgeschaltet*: beim Öffnen mit **Einstellungen öffnen** und **Erneut versuchen**, auf der Startseite als Schloss-Karte statt deiner Flows mit denselben beiden Schaltflächen, wobei Neuer Flow, Importieren und die Vorlagen gesperrt sind. Flow-Missionen bleiben in Mission Control, laufen aber nicht.

## Tastenkürzel

Strg+S, Strg+Enter und Strg+K wirken überall im Editor, die übrigen Tasten auf der Arbeitsfläche. Auf dem Mac steht ⌘ für Strg.

| Taste | Wirkung |
|---|---|
| Tab | Schritt hinzufügen (hinter dem ausgewählten Schritt) |
| Enter | Schritt öffnen |
| C | Ausgewählten Schritt verbinden |
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
| Strg+K | Bausteine suchen |
| ? | Diese Kürzel zeigen |
| Esc | Schließen oder Auswahl aufheben; in der Ansicht eines Laufs zurück zum Entwurf |

---

**Siehe auch:** [Kapitel 11: Mission Control](11-missions.md) · [Kapitel 4: Web-Oberfläche](04-webui.md) · [Kapitel 14: Sicherheit](14-sicherheit.md)
