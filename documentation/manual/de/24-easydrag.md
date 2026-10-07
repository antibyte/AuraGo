# 24 · EasyDrag – Automationen per Drag & Drop

EasyDrag ist der visuelle Editor für Flows. Ein Flow ist eine Mission, die aus Bausteinen besteht: einem oder mehreren **Auslösern** (Zeitplan, Datum & Uhrzeit, Webhook, E-Mail empfangen, MQTT-Nachricht, Home-Assistant-Zustand, Gerät, Telefonanruf über die Fritz!Box, Planer, AuraGo-Start, Budget-Warnung, Mission beendet oder Manueller Start) und danach **Schritten** wie Websuche, KI-Schritt, PDF erstellen, Telegram-Nachricht, E-Mail senden, Home Assistant oder jedes freigegebene Werkzeug. Flows laufen an der Warteschlange des Agenten vorbei und erscheinen in Mission Control mit ihrem Verlauf.

## Der erste Flow

1. Öffne **EasyDrag** auf dem Desktop. Wähle **Neuer Flow** und gib ihm einen Namen, oder starte mit einer **Vorlage**.
2. Klicke **Auslöser wählen** und such dir einen Auslöser aus.
3. Wähle den Auslöser aus und drücke **Tab**: Die Schnellsuche fügt den nächsten Schritt direkt dahinter ein und verbindet ihn. Alternativ ziehst du Bausteine aus der Leiste links auf die Fläche oder auf eine Verbindung.
4. Doppelklicke einen Schritt (oder drücke **Enter**), um ihn zu bearbeiten. Links siehst du die Daten früherer Schritte, aus dem letzten Test oder als Beispiel. Ziehe einen Wert in ein Feld, oder klicke ihn an, um ihn an der letzten Cursorposition einzufügen. Bezüge erscheinen als farbige Chips mit Schritt und Feld, etwa „Websuche › results“. Im Text stehen sie als `{{websuche.results}}`.
5. **Testen** (Strg+Enter) speichert den Entwurf und führt ihn aus. Die Schritte leuchten nacheinander auf, an den Verbindungen steht die Anzahl der Einträge. Ein Test hat echte Auswirkungen: Der Testdialog nennt jede Art (etwa „Sendet Nachrichten“ oder „Schreibt Dateien“) mit ihren Schritten, und **Test starten** bestätigt sie. Gespeicherte Testdaten zeigt er mit `[redacted]` statt geheimer Werte; lässt du sie unverändert, nutzt der Test die echten gespeicherten Daten.
6. **Veröffentlichen** prüft den Flow, zeigt, was er außerhalb von AuraGo tut, und schaltet ihn auf Wunsch gleich aktiv. Nur veröffentlichte Flows laufen selbstständig. Dein Entwurf bleibt bearbeitbar; die veröffentlichte Version läuft.

## Gut zu wissen

- **Automatisch gespeichert.** EasyDrag speichert eine Sekunde nach deiner letzten Änderung. Die Anzeige unten rechts zeigt den Stand: *Gespeichert*, *Wird gespeichert…*, *Offline – neuer Versuch läuft* (EasyDrag versucht es selbst wieder) oder *Nicht gespeichert* mit **Erneut versuchen**. Wurde der Flow woanders geändert, fragt EasyDrag, welche Version gelten soll. Bis AuraGo deine Änderungen hat, bleibt eine Kopie im Browser; beim nächsten Öffnen bietet EasyDrag sie an. **Strg+S** speichert sofort und öffnet nie „Seite speichern“ des Browsers. Solange ein EasyDrag-Dialog offen ist, speichert Strg+S nicht; das automatische Speichern läuft weiter.
- **Läufe.** Die Schaltfläche **Läufe** zeigt Tests und Live-Läufe, gefiltert nach *Alle*, *Fehler*, *Tests* oder *Live*. Ein Klick öffnet die Ansicht des Laufs: die Flow-Version dieses Laufs mit allen Ein- und Ausgaben, nur zum Lesen. **Zurück zum Entwurf** oder **Esc** führt zurück.
- **Fehlerpfad.** In den *Einstellungen* eines Schritts wählst du unter „Wenn der Schritt fehlschlägt“: *Lauf stoppen*, *Weitermachen* (die nächsten Schritte erhalten den Fehler als Daten) oder *Fehlerpfad* (ein eigener Ausgang *Fehler* für Schritte, die auf den Fehler reagieren). Wiederholungen, Sekunden dazwischen und ein Zeitlimit stehen daneben.
- **Geheimnisse.** API-Schlüssel für HTTP-Anfragen legst du im Feld *Geheimnis* des Schritts über **Neues Geheimnis** an. Sie liegen im Vault. Flows können sie nutzen, der Agent kann sie nicht lesen.
- **Mission Control.** Flows tragen dort das Abzeichen *EasyDrag*. **In EasyDrag öffnen** ersetzt *Bearbeiten* und springt in den Editor. Pausieren, Sperren, Ausführen, Löschen und der Verlauf funktionieren wie bei anderen Missionen; *Jetzt ausführen* und *Fortsetzen* bleiben aber gesperrt, bis der Flow veröffentlicht ist („Noch nicht veröffentlicht“).
- **Einstellungen.** *Config → Agent-Tools → EasyDrag-Flows* begrenzt gleichzeitige Läufe, gleichzeitige Schritte pro Lauf und die Laufhistorie. Dort wählst du aus deiner Provider-Liste auch den KI-Anbieter für KI-Schritte, die auf „Standardmodell“ stehen.

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
