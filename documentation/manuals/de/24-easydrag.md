# Kapitel 24: EasyDrag – Automationen per Drag & Drop

EasyDrag ist der visuelle Editor für Flows. Ein Flow ist eine Mission, die aus Bausteinen besteht: einem oder mehreren **Auslösern** (Zeitplan, Datum & Uhrzeit, Webhook, E-Mail empfangen, MQTT-Nachricht, Home-Assistant-Zustand, Gerät, Telefonanruf über die Fritz!Box, Planer, AuraGo-Start, Budget-Warnung, Mission beendet oder Manueller Start) und danach **Schritten** wie Websuche, KI-Schritt, PDF erstellen, Telegram-Nachricht, E-Mail senden, Home Assistant oder jedes freigegebene Werkzeug. Flows laufen an der Warteschlange des Agenten vorbei und erscheinen in Mission Control mit ihrem Verlauf.

Dieses Kapitel beschreibt den Editor. Wie sich Läufe, Grenzen, Fehler und Geheimnisse auf dem Server verhalten, steht in [Kapitel 11: Flow-Missionen (EasyDrag)](11-missions.md#flow-missionen-easydrag).

## Inhaltsverzeichnis

1. [Der erste Flow](#der-erste-flow)
2. [Der Editor](#der-editor)
3. [Speichern](#speichern)
4. [Testen](#testen)
5. [Veröffentlichen](#veröffentlichen)
6. [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)
7. [Decisions-Baustein](#decisions-baustein)
8. [Wenn ein Schritt scheitert](#wenn-ein-schritt-scheitert)
9. [Geheimnisse](#geheimnisse)
10. [Vorlagen, Import und Export](#vorlagen-import-und-export)
11. [Mission Control, Missionsseite und Dashboard](#mission-control-missionsseite-und-dashboard)
12. [Einstellungen](#einstellungen)
13. [Wenn EasyDrag abgeschaltet ist](#wenn-easydrag-abgeschaltet-ist)
14. [Tastenkürzel](#tastenkürzel)

---

## Der erste Flow

1. Öffne **EasyDrag** auf dem Desktop. Wähle **Neuer Flow** und gib ihm einen Namen, oder starte mit einer **Vorlage**.
2. Klicke **Auslöser wählen** und such dir einen Auslöser aus.
3. Wähle den Auslöser aus und drücke **Strg+K**: Die Schnellsuche fügt den nächsten Schritt direkt dahinter ein und verbindet ihn. **Tab** verschiebt nur den Fokus. Du kannst auch **Hinzufügen** an der gewählten Karte nutzen oder Bausteine aus der Leiste links auf die Fläche oder auf eine Verbindung ziehen.
4. Doppelklicke einen Schritt (oder drücke **Enter**), um ihn zu bearbeiten. Der Inspektor öffnet sich im Editor, nicht als Dialog. Er hat die Reiter Eingabe, Parameter und Ausgabe. Im Reiter Eingabe siehst Du die Daten früherer Schritte: aus dem letzten Test oder Lauf, oder als Beispiel. Ziehe einen Wert über den Reiter Parameter und dann in ein Feld, oder klicke ihn an, um ihn an der letzten Cursorposition einzufügen. Bezüge erscheinen als farbige Chips mit Schritt und Feld, etwa „Websuche › results“. Im Text stehen sie als `{{websuche.results}}`.
5. **Testen** (Strg+Enter) speichert den Entwurf und führt ihn aus. Die Schritte leuchten nacheinander auf, an den Verbindungen steht die Anzahl der Einträge. Ein Test hat echte Auswirkungen; siehe [Testen](#testen).
6. **Veröffentlichen** prüft den Flow, zeigt, was er außerhalb von AuraGo tut, und schaltet ihn auf Wunsch gleich aktiv. Nur veröffentlichte Flows laufen selbstständig. Dein Entwurf bleibt bearbeitbar; die veröffentlichte Version läuft.

## Der Editor

- Der Kopf zeigt Name und Stand des Flows, mit denselben Wörtern wie Mission Control und die Missionsseite: *Noch nicht veröffentlicht* (nie veröffentlicht), *Veröffentlicht* oder *Unveröffentlichte Änderungen*, daneben *Pausiert*, solange ein veröffentlichter Flow ausgeschaltet ist. Er enthält **Läufe**, **Testen**, **Veröffentlichen**, den Schalter **Aktiv** und das Menü **⋯**: *Jetzt ausführen*, *Flow-Einstellungen*, *Duplizieren*, *Exportieren*, *In Mission Control zeigen* und *Löschen*. Die Fußzeile zeigt den letzten Lauf, Probleme und Hinweise sowie den Speicherstand. „Entwurf“ meint immer die Version, die du bearbeitest, im Unterschied zur veröffentlichten.
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

## Speichern

- EasyDrag speichert eine Sekunde nach deiner letzten Änderung. Die Anzeige unten rechts zeigt den Stand:
  - *Gespeichert*, *Ungespeichert* oder *Wird gespeichert…*: der normale Ablauf.
  - *Offline – neuer Versuch läuft*: AuraGo ist nicht erreichbar oder hat mit einem Serverfehler geantwortet. EasyDrag versucht es selbst wieder.
  - *Nicht gespeichert* mit **Erneut versuchen**: AuraGo hat die Änderung abgelehnt, etwa weil der Flow zu groß ist oder du Flows nicht ändern darfst. Der Tooltip nennt den Grund.
  - *Nicht gespeichert – Fehler beheben*: Der Entwurf hat Fehler, die das Speichern verhindern. Die nächste Änderung wird wieder gesendet.
  - *Konflikt*: Der Flow wurde woanders geändert. EasyDrag fragt, ob du deine Version behältst oder die andere lädst.
- Bis AuraGo deine Änderungen hat, bleibt bis zu 30 Tage eine Kopie in diesem Browser. Öffnest du den Flow wieder, bietet EasyDrag sie an, außer der Flow wurde inzwischen woanders geändert. Wird ein Flow gelöscht, hier oder anderswo, entfernt EasyDrag diese Kopie und alles andere, was sich dieser Browser für den Flow gemerkt hat (wo du ihn verlassen hast, bestätigte Testauswirkungen, den zuletzt gewählten Testauslöser).
- **Strg+S** speichert sofort. Im Editor öffnet es nie „Seite speichern“ des Browsers. In der Ansicht eines Laufs und solange ein echter Dialog offen ist, speichert es nicht. Der Schritt-Inspektor blockiert es nicht, und das automatische Speichern läuft weiter. Eine Notiz oder ein Feld, das du gerade tippst, schreibt EasyDrag vor dem Speichern, Testen, Veröffentlichen oder Verlassen. Ungültiges JSON, eine Zahl außerhalb ihres Bereichs oder ein Datenname, den der Editor ablehnt, bleibt im Feld und blockiert diese Aktion. Text, den du noch mit einer Eingabemethode zusammensetzt, wird nicht abgeschnitten.

## Testen

- **Testen** speichert zuerst den Entwurf; einen Entwurf, der sich nicht speichern lässt, testet EasyDrag nicht. Der Flow braucht einen eingeschalteten Auslöser.
- Der Testdialog zeigt die Daten, mit denen der Auslöser den Test startet. Hat der Flow mehrere Auslöser, wählst du einen unter *Starten mit*. Ändere das JSON, um andere Fälle auszuprobieren; *Diese Daten für spätere Tests merken* behält deine Änderung.
- Gespeicherte Testdaten zeigen geheime Werte als `[redacted]`. Lässt du sie unverändert, nutzt der Test die echten gespeicherten Daten. Geänderte Daten, die noch `[redacted]` enthalten, laufen, werden aber nicht gemerkt.
- Ein Test hat echte Auswirkungen. Der Dialog nennt jede Art (*Sendet Nachrichten*, *Schreibt Dateien*, *Steuert Geräte*, …) mit den Schritten, die sie auslösen können. Jeder Schritt zählt mit dem, was sein Baustein standardmäßig tut, und mit dem, was seine tatsächlichen Einstellungen tun; eine HTTP-Anfrage mit POST zählt deshalb als Senden, obwohl der Baustein allein das nicht tut. Kann AuraGo die Einstellungen nicht prüfen oder kann ein Baustein seine Auswirkungen nicht nennen, sagt der Dialog, dass sich die Auswirkungen nicht vollständig prüfen ließen. **Test starten** bestätigt die genannten Arten, sobald der Test gestartet ist, solange der Flow offen bleibt; *Für diesen Flow nicht mehr fragen* merkt sie auf diesem Gerät. Ein Test, der nicht startet (AuraGo lehnt ihn ab oder ist nicht erreichbar), bestätigt nichts. Eine Art, die du noch nicht bestätigt hast, fragt EasyDrag wieder ab.
- **Schritt testen** in der Detailansicht (auch *Diesen Schritt testen* an der gewählten Karte und in ihrem Kontextmenü) führt diesen Schritt zusammen mit den Schritten davor aus. Sein Dialog fragt deshalb nach den Auswirkungen jedes Schritts, der läuft: des Schritts selbst und der Schritte davor, die der gewählte Auslöser erreicht. Ein ausgeschalteter Schritt läuft nicht, ebenso wenig die Schritte, zu denen nur er führt.
- **Stoppen** im Kopf bricht den Test oder Lauf ab, den du in diesem Fenster gestartet hast, ohne Rückfrage (auch einen Lauf, den du mit *Jetzt ausführen* gestartet hast). Andere Läufe, die noch nicht beendet sind, stoppst du unter **Läufe** (siehe [Läufe und Benachrichtigungen](#läufe-und-benachrichtigungen)). Solange dieser Test oder Live-Lauf noch startet oder läuft, startet EasyDrag keinen weiteren und schickt einen ungewissen Start nicht erneut. Stoppen macht bereits gelaufene Schritte nicht rückgängig. Das automatische Speichern bleibt unverändert. Die Lauf-Liste öffnet sich im Editor, unter dem Kopf und einem Lauf-Banner.

## Veröffentlichen

- Der Veröffentlichen-Dialog nennt blockierende Probleme, was der Flow außerhalb von AuraGo tut, Daten von außen, die sensible Schritte erreichen, Hinweise und die Änderungen seit der letzten Veröffentlichung. Mit blockierenden Problemen lässt er sich nur schließen; ein Klick auf ein Problem öffnet seinen Schritt.
- *Jetzt aktivieren, damit die Auslöser greifen* schaltet den Flow mit der Veröffentlichung ein. Danach schaltest du ihn mit dem Schalter **Aktiv** aus und wieder ein. Einen unveröffentlichten Flow kannst du nicht einschalten; EasyDrag bietet an, ihn zuerst zu veröffentlichen.
- Die Bausteine-Leiste markiert Bausteine mit echten Auswirkungen außerhalb von AuraGo. Die Markierung betrachtet die Standardeinstellungen eines Bausteins (bei einem allgemeinen Werkzeug seine weitreichendste Operation) und ist keine Sicherheitsgarantie. Der Veröffentlichen-Dialog nennt die Auswirkungen deiner tatsächlichen Einstellungen; der Testdialog nennt zusätzlich die der Standardeinstellungen der Bausteine.
- **Mit einem Problem veröffentlicht** heißt: Die neue Version läuft, aber Mission Control oder die Timer ließen sich nicht aktualisieren. Der Kopf zeigt dann *Veröffentlichung unvollständig*, und **Veröffentlichen** bleibt verfügbar: Veröffentliche erneut, um den Vorgang abzuschließen.
- Fehlt der Mission-Control-Eintrag des Flows, hilft erneutes Veröffentlichen nicht. Exportiere den Flow, lösche ihn und importiere ihn erneut.

## Läufe und Benachrichtigungen

- **Läufe** zeigt die letzten 50 Läufe, gefiltert nach *Alle*, *Fehler*, *Tests* oder *Live*. Ein Klick öffnet die Ansicht des Laufs: die Flow-Version dieses Laufs mit allen Ein- und Ausgaben, nur zum Lesen. Ein Banner nennt den Lauf; **Zurück zum Entwurf** oder **Esc** führt zurück. In der Ansicht eines Laufs bleibt die Bausteine-Leiste verborgen, und ein fehlgeschlagener Lauf öffnet sich zentriert auf den gescheiterten Schritt.
- Ein Lauf, der noch nicht beendet ist (*In Warteschlange* – auch wenn er auf einen freien Platz wartet – oder *Läuft*), zeigt in der Liste und im Banner seiner Ansicht **Stoppen**, auch wenn ein Auslöser oder ein anderes Fenster ihn gestartet hat. Außer bei einem Test fragt EasyDrag vorher nach; Schritte, die schon gelaufen sind, werden nicht rückgängig gemacht. Auch ein schreibgeschützter Desktop bietet **Stoppen** an, denn Stoppen ändert keinen Flow; schreibgeschützte Missionen lehnen es ab.
- **Jetzt ausführen** (Menü ⋯, das Menü *Flow* oder das Menü einer Karte auf der Startseite) startet die veröffentlichte Version. Der Flow muss **Aktiv** sein: Solange er pausiert ist, ist *Jetzt ausführen* ausgeschaltet, und sein Tooltip sagt, dass du den Flow zuerst einschalten musst, so wie Mission Control eine pausierte Mission nicht ausführt. **Testen** geht in beiden Fällen.
- Ein fehlgeschlagener Live-Lauf benachrichtigt so, wie es die Flow-Einstellungen sagen: auf dem Desktop (Standard), per Push, per Telegram oder gar nicht. Ein Flow benachrichtigt einmal, wenn er zu scheitern beginnt, danach höchstens einmal pro Stunde, solange er weiter scheitert. Ein Klick auf die Desktop-Benachrichtigung öffnet EasyDrag bei diesem Lauf.
- Nach einem Neuladen des Desktops öffnet sich ein offener Flow wieder.

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

- Die Startseite bietet sechs Vorlagen: *KI-News als PDF per Telegram*, *Webhook per E-Mail zusammenfassen*, *Terminerinnerung*, *Licht aus beim Verlassen*, *Budgetwächter* und *Täglicher RSS-Überblick*. Im Editor tragen Schritte, deren Integration noch nicht eingerichtet ist, ein Abzeichen, und ihre Detailansicht verweist auf die Einstellungen (*Einstellungen öffnen*); in der Bausteine-Leiste bieten solche Bausteine *Einrichten* an.
- **Exportieren** lädt den Entwurf als `<name>.easydrag.json` herunter. **Importieren** auf der Startseite legt aus so einer Datei einen neuen Flow an (höchstens 2 MiB). Eine Flow-Datei enthält die Namen von Geheimnissen, nie ihre Werte; was du in gewöhnliche Felder tippst (etwa die Header-Liste eines allgemeinen Werkzeugs), wird so exportiert, wie du es eingegeben hast.

## Mission Control, Missionsseite und Dashboard

- In Mission Control tragen Flows das Abzeichen *EasyDrag*, und der Filter *Flows* zeigt nur sie. **In EasyDrag öffnen** ersetzt *Bearbeiten*; **Neuer Flow** öffnet die Startseite von EasyDrag. Zeitpläne lesen sich wie „Mo–Fr um 07:00“ (Uhrzeiten im Format deiner Sprache), und eine Mission, die noch nie lief, zeigt *Noch nie gelaufen*.
- Läufe, die nur auf einen freien Platz warten, kannst du in Mission Control nicht abbrechen; stoppe sie hier unter **Läufe**.
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

Strg+S, Strg+Enter und Strg+K wirken im Editor, auch solange der Schritt-Inspektor offen ist. Sie warten, solange ein echter Dialog oder ein Kontextmenü offen ist. Die übrigen Tasten wirken auf der Arbeitsfläche, nicht während du in einem Feld tippst. Auf dem Mac steht ⌘ für Strg. **Tab** verschiebt den Fokus und fügt keinen Schritt hinzu. Die Arbeitsfläche ist ein Halt; die Zoom-Knöpfe folgen. Die Werkzeugknöpfe des ausgewählten Schritts kommen dazu. Dieselben Aktionen gibt es als Knopf oder Taste: **Hinzufügen** setzt den nächsten Schritt an die Karte, **Verbinden** (C) öffnet eine Liste passender Schritte und Anschlüsse, D schaltet den Schritt aus oder ein, Strg+D dupliziert, Entf löscht. Tippt du einen Anschluss an und lässt ihn ohne Ziel los, öffnet dieselbe Liste mit dieser Seite und diesem Anschluss. *Schritt testen* steht an der Karte und im Inspektor (Enter öffnet den Inspektor). Screenreader finden zusätzlich eine Liste der Schritte. Die Liste ist für den Lesemodus da; mit echten Screenreader-Nutzern wurde sie nicht geprüft.

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
| ? | Diese Kürzel zeigen |
| Esc | Schließen oder Auswahl aufheben; in der Ansicht eines Laufs zurück zum Entwurf |

---

**Siehe auch:** [Kapitel 11: Mission Control](./11-missions.md) · [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 14: Sicherheit](./14-security.md)

---

**Vorheriges Kapitel:** [Kapitel 23: Interna](./23-internals.md)
**Nächstes Kapitel:** [Kapitel 25: CYD Creator](./25-cyd-creator.md)
