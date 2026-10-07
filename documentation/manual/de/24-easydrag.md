# Kapitel 24: EasyDrag – Automationen per Drag & Drop

EasyDrag ist der visuelle Editor für Flows. Ein Flow ist eine Mission, die aus Bausteinen besteht: einem oder mehreren **Auslösern** (Zeitplan, Datum & Uhrzeit, Webhook, E-Mail empfangen, MQTT-Nachricht, Home-Assistant-Zustand, Gerät, Telefonanruf über die Fritz!Box, Planer, AuraGo-Start, Budget-Warnung, Mission beendet oder Manueller Start) und danach **Schritten** wie Websuche, KI-Schritt, PDF erstellen, Telegram-Nachricht, E-Mail senden, Home Assistant oder jedes freigegebene Werkzeug. Flows laufen an der Warteschlange des Agenten vorbei und erscheinen in Mission Control mit ihrem Verlauf.

Dieses Kapitel beschreibt den Editor. Wie sich Läufe, Grenzen, Fehler und Geheimnisse auf dem Server verhalten, steht in [Kapitel 11: Flow-Missionen (EasyDrag)](11-missions.md#flow-missionen-easydrag).

## Der erste Flow

1. Öffne **EasyDrag** auf dem Desktop. Wähle **Neuer Flow** und gib ihm einen Namen, oder starte mit einer **Vorlage**.
2. Klicke **Auslöser wählen** und such dir einen Auslöser aus.
3. Wähle den Auslöser aus und drücke **Tab**: Die Schnellsuche fügt den nächsten Schritt direkt dahinter ein und verbindet ihn. Alternativ ziehst du Bausteine aus der Leiste links auf die Fläche oder auf eine Verbindung.
4. Doppelklicke einen Schritt (oder drücke **Enter**), um ihn zu bearbeiten. Links siehst du die Daten früherer Schritte: aus dem letzten Test oder Lauf, oder als Beispiel. Ziehe einen Wert in ein Feld, oder klicke ihn an, um ihn an der letzten Cursorposition einzufügen. Bezüge erscheinen als farbige Chips mit Schritt und Feld, etwa „Websuche › results“. Im Text stehen sie als `{{websuche.results}}`.
5. **Testen** (Strg+Enter) speichert den Entwurf und führt ihn aus. Die Schritte leuchten nacheinander auf, an den Verbindungen steht die Anzahl der Einträge. Ein Test hat echte Auswirkungen; siehe [Testen](#testen).
6. **Veröffentlichen** prüft den Flow, zeigt, was er außerhalb von AuraGo tut, und schaltet ihn auf Wunsch gleich aktiv. Nur veröffentlichte Flows laufen selbstständig. Dein Entwurf bleibt bearbeitbar; die veröffentlichte Version läuft.

## Der Editor

- Der Kopf zeigt Name und Stand des Flows, mit denselben Wörtern wie Mission Control und die Missionsseite: *Noch nicht veröffentlicht* (nie veröffentlicht), *Veröffentlicht* oder *Unveröffentlichte Änderungen*, daneben *Pausiert*, solange ein veröffentlichter Flow ausgeschaltet ist. Er enthält **Läufe**, **Testen**, **Veröffentlichen**, den Schalter **Aktiv** und das Menü **⋯**: *Jetzt ausführen*, *Flow-Einstellungen*, *Duplizieren*, *Exportieren*, *In Mission Control zeigen* und *Löschen*. Die Fußzeile zeigt den letzten Lauf, Probleme und Hinweise sowie den Speicherstand. „Entwurf“ meint immer die Version, die du bearbeitest, im Unterschied zur veröffentlichten.
- Die Startseite zeigt deine Flows mit denselben Ständen und filtert sie nach *Alle*, *Aktiv*, *Pausiert* (veröffentlicht und ausgeschaltet) und *Mit Fehlern*.
- Der Editor folgt Änderungen von anderswo (einem anderen Fenster, Mission Control, der Missionsseite oder dem Agenten): Wird der Flow dort pausiert oder eingeschaltet, folgen Stand, Schalter **Aktiv** und *Jetzt ausführen*; wird er dort gelöscht, sagt EasyDrag das und zeigt die Startseite.
- Ein Flow öffnet sich in lesbarer Größe: Ein kleiner Flow erscheint ganz, ein größerer beginnt bei seinem Auslöser mit 80 %. In Fenstern, die breiter als 560 px sind, merkt sich EasyDrag auf diesem Gerät, wo du jeden Flow verlassen hast. Verschieben und Zoomen der Fläche ändern den Flow nicht: Sie werden weder in ihm gespeichert noch als *Unveröffentlichte Änderungen* gezählt. **Alles zeigen** (Umschalt+1) zeigt den ganzen Flow; zoomt das weit heraus, zeigen die Karten nur noch ihre Namen, dafür größer.
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
- Bis AuraGo deine Änderungen hat, bleibt bis zu 30 Tage eine Kopie in diesem Browser. Öffnest du den Flow wieder, bietet EasyDrag sie an, außer der Flow wurde inzwischen woanders geändert. Wird ein Flow gelöscht, hier oder anderswo, entfernt EasyDrag diese Kopie und alles andere, was sich dieser Browser für den Flow gemerkt hat (wo du ihn verlassen hast, bestätigte Testauswirkungen, den zuletzt gewählten Testauslöser).
- **Strg+S** speichert sofort. Im Editor öffnet es nie „Seite speichern“ des Browsers; in der Ansicht eines Laufs und solange ein Dialog offen ist, speichert es nicht, und das automatische Speichern läuft weiter.

## Testen

- **Testen** speichert zuerst den Entwurf; einen Entwurf, der sich nicht speichern lässt, testet EasyDrag nicht. Der Flow braucht einen eingeschalteten Auslöser.
- Der Testdialog zeigt die Daten, mit denen der Auslöser den Test startet. Hat der Flow mehrere Auslöser, wählst du einen unter *Starten mit*. Ändere das JSON, um andere Fälle auszuprobieren; *Diese Daten für spätere Tests merken* behält deine Änderung.
- Gespeicherte Testdaten zeigen geheime Werte als `[redacted]`. Lässt du sie unverändert, nutzt der Test die echten gespeicherten Daten. Geänderte Daten, die noch `[redacted]` enthalten, laufen, werden aber nicht gemerkt.
- Ein Test hat echte Auswirkungen. Der Dialog nennt jede Art (*Sendet Nachrichten*, *Schreibt Dateien*, *Steuert Geräte*, …) mit den Schritten, die sie auslösen können. Jeder Schritt zählt mit dem, was sein Baustein standardmäßig tut, und mit dem, was seine tatsächlichen Einstellungen tun; eine HTTP-Anfrage mit POST zählt deshalb als Senden, obwohl der Baustein allein das nicht tut. Kann AuraGo die Einstellungen nicht prüfen oder kann ein Baustein seine Auswirkungen nicht nennen, sagt der Dialog, dass sich die Auswirkungen nicht vollständig prüfen ließen. **Test starten** bestätigt die genannten Arten, solange der Flow offen bleibt; *Für diesen Flow nicht mehr fragen* merkt sie auf diesem Gerät. Eine Art, die du noch nicht bestätigt hast, fragt EasyDrag wieder ab.
- **Schritt testen** in der Detailansicht (auch *Diesen Schritt testen* an der gewählten Karte und in ihrem Kontextmenü) führt diesen Schritt zusammen mit den Schritten davor aus. Sein Dialog fragt deshalb nach den Auswirkungen jedes Schritts, der läuft: des Schritts selbst und der Schritte davor, die der gewählte Auslöser erreicht. Ein ausgeschalteter Schritt läuft nicht, ebenso wenig die Schritte, zu denen nur er führt.
- **Stoppen** im Kopf bricht den Test oder Lauf ab, den du in diesem Fenster gestartet hast, ohne Rückfrage (auch einen Lauf, den du mit *Jetzt ausführen* gestartet hast). Andere Läufe, die noch nicht beendet sind, stoppst du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)).

## Veröffentlichen

- Der Veröffentlichen-Dialog nennt blockierende Probleme, was der Flow außerhalb von AuraGo tut, Daten von außen, die sensible Schritte erreichen, Hinweise und die Änderungen seit der letzten Veröffentlichung. Mit blockierenden Problemen lässt er sich nur schließen; ein Klick auf ein Problem öffnet seinen Schritt.
- *Jetzt aktivieren, damit die Auslöser greifen* schaltet den Flow mit der Veröffentlichung ein. Danach schaltest du ihn mit dem Schalter **Aktiv** aus und wieder ein. Einen unveröffentlichten Flow kannst du nicht einschalten; EasyDrag bietet an, ihn zuerst zu veröffentlichen.
- Die Bausteine-Leiste markiert Bausteine mit echten Auswirkungen außerhalb von AuraGo. Die Markierung betrachtet die Standardeinstellungen eines Bausteins (bei einem allgemeinen Werkzeug seine weitreichendste Operation) und ist keine Sicherheitsgarantie. Der Veröffentlichen-Dialog nennt die Auswirkungen deiner tatsächlichen Einstellungen; der Testdialog nennt zusätzlich die der Standardeinstellungen der Bausteine.
- **Mit einem Problem veröffentlicht** heißt: Die neue Version läuft, aber Mission Control oder die Timer ließen sich nicht aktualisieren. Der Kopf zeigt dann *Veröffentlichung unvollständig*, und **Veröffentlichen** bleibt verfügbar: Veröffentliche erneut, um den Vorgang abzuschließen.
- Fehlt der Mission-Control-Eintrag des Flows, hilft erneutes Veröffentlichen nicht. Exportiere den Flow, lösche ihn und importiere ihn erneut.

## Läufe und Benachrichtigungen

- **Läufe** zeigt die letzten 50 Läufe, gefiltert nach *Alle*, *Fehler*, *Tests* oder *Live*. Ein Klick öffnet die Ansicht des Laufs: die Flow-Version dieses Laufs mit allen Ein- und Ausgaben, nur zum Lesen. Ein Banner nennt den Lauf; **Zurück zum Entwurf** oder **Esc** führt zurück. In der Ansicht eines Laufs bleibt die Bausteine-Leiste verborgen, und ein fehlgeschlagener Lauf öffnet sich zentriert auf den gescheiterten Schritt.
- Ein Lauf, der noch nicht beendet ist (*In Warteschlange* – auch wenn er auf einen freien Platz wartet – oder *Läuft*), zeigt in der Liste und im Banner seiner Ansicht **Stoppen**, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat. Außer bei einem Test fragt EasyDrag vorher nach; Schritte, die schon gelaufen sind, werden nicht rückgängig gemacht. Ein schreibgeschützter Desktop bietet kein Stoppen an.
- **Jetzt ausführen** (Menü ⋯, das Menü *Flow* oder das Menü einer Karte auf der Startseite) startet die veröffentlichte Version. Der Flow muss **Aktiv** sein: Solange er pausiert ist, ist *Jetzt ausführen* ausgeschaltet, und sein Tooltip sagt, dass du den Flow zuerst einschalten musst, so wie Mission Control eine pausierte Mission nicht ausführt. **Testen** geht in beiden Fällen.
- Ein fehlgeschlagener Live-Lauf benachrichtigt so, wie es die Flow-Einstellungen sagen: auf dem Desktop (Standard), per Push, per Telegram oder gar nicht. Ein Flow benachrichtigt einmal, wenn er zu scheitern beginnt, danach höchstens einmal pro Stunde, solange er weiter scheitert. Ein Klick auf die Desktop-Benachrichtigung öffnet EasyDrag bei diesem Lauf.
- Nach einem Neuladen des Desktops öffnet sich ein offener Flow wieder.

## Wenn ein Schritt scheitert

- In den *Einstellungen* eines Schritts wählst du unter *Wenn der Schritt fehlschlägt*: *Lauf stoppen*, *Weitermachen* (die nächsten Schritte erhalten den Fehler als Daten) oder *Fehlerpfad* (ein eigener Ausgang *Fehler* für Schritte, die auf den Fehler reagieren). *Wiederholungen*, *Sekunden dazwischen* und *Zeitlimit (s)* stehen daneben.
- Der *Datenname* im selben Tab ist der Name, unter dem spätere Schritte die Ausgabe lesen (`{{datenname.feld}}`).

## Geheimnisse

- API-Schlüssel für HTTP-Anfragen legst du als Geheimnis an: **Neues Geheimnis** im Feld *Geheimnis* des Schritts. Ein Wert darf bis zu 4 KiB groß sein, und AuraGo nimmt höchstens 30 Änderungen an Geheimnissen pro Minute an. Gibt es den Namen schon, fragt EasyDrag, bevor es den Wert ersetzt.
- Geheimnisse liegen im Vault als `easydrag_<name>`. Flows können sie nutzen; der Agent kann sie weder auflisten noch lesen oder ändern. Lauf-Daten und die Ansicht eines Laufs zeigen ihre Werte geschwärzt, mit einer Grenze für sehr kurze Werte (siehe [Kapitel 11](11-missions.md#flow-geheimnisse)).
- Der Papierkorb neben **Neues Geheimnis** löscht das gewählte Geheimnis nach einer Rückfrage und leert das Feld. Flows, die das Geheimnis nutzen, schlagen fehl, bis es wieder gesetzt ist; nutzen veröffentlichte Flows es noch, nennt eine Benachrichtigung sie.

## Vorlagen, Import und Export

- Die Startseite bietet sechs Vorlagen: *KI-News als PDF per Telegram*, *Webhook per E-Mail zusammenfassen*, *Terminerinnerung*, *Licht aus beim Verlassen*, *Budgetwächter* und *Täglicher RSS-Überblick*. Im Editor tragen Schritte, deren Integration noch nicht eingerichtet ist, ein Abzeichen, und ihre Detailansicht verweist auf die Einstellungen (*Einstellungen öffnen*); in der Bausteine-Leiste bieten solche Bausteine *Einrichten* an.
- **Exportieren** lädt den Entwurf als `<name>.easydrag.json` herunter. **Importieren** auf der Startseite legt aus so einer Datei einen neuen Flow an (höchstens 2 MiB). Eine Flow-Datei enthält die Namen von Geheimnissen, nie ihre Werte; was du in gewöhnliche Felder tippst (etwa die Header-Liste eines allgemeinen Werkzeugs), wird so exportiert, wie du es eingegeben hast.

## Mission Control, Missionsseite und Dashboard

- In Mission Control tragen Flows das Abzeichen *EasyDrag*, und der Filter *Flows* zeigt nur sie. **In EasyDrag öffnen** ersetzt *Bearbeiten*; **Neuer Flow** öffnet die Startseite von EasyDrag. Zeitpläne lesen sich wie „Mo–Fr um 07:00“ (Uhrzeiten im Format deiner Sprache), und eine Mission, die noch nie lief, zeigt *Noch nie gelaufen*.
- Läufe, die nur auf einen freien Platz warten, kannst du in Mission Control nicht abbrechen; stoppe sie hier unter **Läufe**.
- Ausführen, Pausieren und Löschen von Flow-Missionen, die Missionsseite und das Dashboard beschreibt [Kapitel 11: Flow-Missionen](11-missions.md#flow-missionen-easydrag).

## Einstellungen

*Config → Agent-Tools → EasyDrag-Flows*:
- *Flows aktivieren*, *Gleichzeitige Läufe (1–32)*, *Gleichzeitige Schritte pro Lauf (1–16)*, *Laufhistorie aufbewahren (Tage, 1–365)* und *Gespeicherte Läufe pro Flow (10–5000)*. Änderungen daran wirken nach einem Neustart; schaltest du *Flows aktivieren* aus, ist EasyDrag sofort abgeschaltet (siehe [Wenn EasyDrag abgeschaltet ist](#wenn-easydrag-abgeschaltet-ist)), veröffentlichte Flows laufen aber bis zum Neustart weiter.
- *KI-Anbieter für KI-Schritte*: Wähle einen Anbieter aus deiner Provider-Liste. KI-Schritte, die auf „Standardmodell“ stehen, nutzen ihn; ohne Auswahl nutzen sie das Hauptmodell. Die Einstellung gilt sofort.
- Die Agenten-Optionen (*Agent darf Flows nur lesen*, *Agent darf Flows veröffentlichen*) sind gesperrt. Sie greifen, sobald der Agent mit Flows arbeiten kann.
- *EasyDrag öffnen* öffnet die App auf dem Desktop.

## Wenn EasyDrag abgeschaltet ist

EasyDrag braucht *Flows aktivieren* und das Missions-Tool (`tools.missions.enabled`, siehe [Kapitel 11](11-missions.md#voraussetzungen)). Ist eins davon aus, verschwindet das EasyDrag-Symbol, sobald der Desktop seine App-Liste das nächste Mal lädt. Lädt EasyDrag in diesem Zustand (ein neues Fenster, **Erneut versuchen** oder das Öffnen eines Flows), zeigt es *EasyDrag ist abgeschaltet* mit **Einstellungen öffnen** und **Erneut versuchen**. Die Startseite zeigt dieselbe Karte mit beiden Schaltflächen statt deiner Flows, sobald sie ihre Liste das nächste Mal lädt; Neuer Flow, Importieren und die Vorlagen sind dann gesperrt. Ein schon geöffneter Flow bleibt im Editor; sein nächstes Speichern zeigt *Offline – neuer Versuch läuft*, bis EasyDrag wieder eingeschaltet ist. Flow-Missionen bleiben in Mission Control, laufen aber nicht.

## Tastenkürzel

Strg+S, Strg+Enter und Strg+K wirken überall im Editor, aber nicht, solange ein Dialog oder ein Kontextmenü offen ist; die übrigen Tasten wirken auf der Arbeitsfläche. Auf dem Mac steht ⌘ für Strg. Von außen ist die Arbeitsfläche ein Halt für **Tab**; die Werkzeugknöpfe eines Schritts kommen dazu, solange er ausgewählt ist, und Screenreader finden zusätzlich eine Liste der Schritte.

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
