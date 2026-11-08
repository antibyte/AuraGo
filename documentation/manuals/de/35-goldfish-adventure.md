# Kapitel 35: Bitte nicht das Universum füttern – ein gemaltes Point-and-Click-Abenteuer

Bitte nicht das Universum füttern ist ein handgemaltes Point-and-Click-Abenteuer im Virtual Desktop.
Es läuft komplett in deinem Browser und braucht keinen Provider, kein Konto und keinen externen
Dienst: Bilder, Geschichte und Musik kommen mit AuraGo mit, und die Spielstände verlassen deinen
Browser nie. Ein weggeworfener Wartungsroboter will seine Garantie zurück, und dabei rettet er drei
wütende Goldfische, lernt einen trauernden Affen kennen und findet heraus, dass der berühmte
Glückseligkeitsdienst der Galaxis das Recht auf Widerspruch abschafft.

Das Spiel hat sechzehn Räume in vier Akten, eigene Originalbilder und ein erzeugtes Instrumentalstück
je Raum. Es ist ein vollständiges Spiel mit eigenen Spielständen, eigenem Journal und eigenen
Hinweisen. Der englische Titel lautet **Please Do Not Feed the Universe**.

## Inhaltsverzeichnis
1. [Das Spiel öffnen](#das-spiel-öffnen)
2. [Steuerung](#steuerung)
3. [Die Aktionsleiste](#die-aktionsleiste)
4. [Dialoge](#dialoge)
5. [Die sechzehn Räume](#die-sechzehn-räume)
6. [Musik und Ton](#musik-und-ton)
7. [Spielstände und Einstellungen](#spielstände-und-einstellungen)
8. [Wenn etwas schiefgeht](#wenn-etwas-schiefgeht)

---

## Das Spiel öffnen

Öffne **Bitte nicht das Universum füttern** aus dem Startmenü, in der Kategorie **Spiele**. Der
Startbildschirm bietet **Neue Reise** und, wenn es einen Spielstand gibt, **Reise fortsetzen**.

Das Spiel führt durch sechzehn Räume in vier Akten. Jeder nötige Hinweis hat zwei unabhängige Quellen
(eine Beobachtung plus eine Dialogzeile zum Beispiel), man muss also nichts aus einem Pixel-Raten
erschließen. Es gibt 18 ausgearbeitete Rätselsituationen, 130 ausgearbeitete Aktionen, drei binäre
Verzweigungen und drei Enden; alle 24 Kombinationen aus Route und Ende funktionieren.

Du steuerst R0ST, den Roboter. Begleiter handeln mit dir im Zusammenhang: Du fragst, erklärst den
Zweck, holst die Zustimmung, und dann passiert die gemeinsame Aktion. Bruno und die Fische können
ablehnen. Sie sind nie Inventar-Objekte und nie fernsteuerbare Werkzeuge.

## Steuerung

Das Spiel nutzt ein LucasArts-Schema. Ein Linksklick ist die Hauptaktion, ein Rechtsklick sieht sich
etwas an.

| Eingabe | Wirkung |
|---------|---------|
| Zeiger über einem Objekt, Ausgang oder Gegenstand | Die Satzzeile zeigt den Befehl, der laufen würde, zum Beispiel „Benutze Förderband und Not-Aus“. Ausgänge zeigen ihren eigenen Text. Untätig: der Raumname. |
| Linksklick | R0ST geht zum Objekt und führt dessen Standardaktion aus. Genau eine verfügbare Aktion läuft; zwei oder mehr öffnen eine Auswahlliste in der Leiste; keine bedeutet ansehen. |
| Rechtsklick, langes Drücken oder Shift+Enter | **Ansehen** des Objekts. |
| Verben anklicken, dann ein Objekt | Das Verb gilt für den nächsten Klick und setzt sich danach zurück. |
| Gegenstand im Inventar anklicken | Armiert „Benutze {Gegenstand} mit …“. Der nächste Klick auf ein Objekt führt die passende Aktion aus. Nochmal auf den Gegenstand klicken, Escape drücken oder mit der rechten Maustaste ins Bild klicken hebt die Armierung auf. |
| Rechtsklick auf einen Gegenstand | **Ansehen** seiner Beschreibung. |
| Klick auf den Boden | R0ST geht dorthin. Ein Klick während des Gehens zielt um; ein zweiter Klick auf denselben Punkt beendet den Weg sofort. |
| Enter auf einem fokussierten Objekt | Dasselbe wie ein Linksklick. Die Reihenfolge mit Tab ist: Objekte, Ausgänge, Verben, Gegenstände, Menü. |
| Escape | Laufender Text: Warteschlange überspringen. Sonst: scharfgeschaltetes Verb, Gegenstand oder offene Auswahl abbrechen. Sonst: das Menü öffnen. |
| Leertaste | Laufender Text: die Zeile überspringen. Sonst: Pause. |
| Touch | Tippen ist ein Linksklick ohne Zeiger; ein langes Drücken von 500 ms ist **Ansehen**. |

Jeder Ausgang ist eine anklickbare Zone mit einem dauerhaften Richtungsschild am Bildrand: gold, wenn
offen, grau und gestrichelt, wenn gesperrt. Seine Beschriftung bleibt beim Zeigen in der Satzzeile.

## Die Aktionsleiste

Unter dem Bild liegt eine einzige Leiste, im Stil der klassischen Point-and-Click-Spiele. Oben ist die
Satzzeile; darunter links die vier Verben, rechts das Inventarraster (etwa 6 × 2 Plätze, mit
Scrollpfeilen).

| Verb | Was es tut |
|------|------------|
| **Ansehen** | Liest die eigenen Ansichtszeilen des Objekts. |
| **Nehmen** | Nimmt mit, was man mitnehmen kann. Es trifft auch Aktionen, die dir einen Gegenstand geben. |
| **Benutzen** | Der Standard für alles andere. |
| **Sprechen** | Beginnt ein Gespräch. |

Eine kleine ☰-Schaltfläche am rechten Ende der Satzzeile öffnet das Menü: Reisejournal, ein kleiner
Hinweis, Spielstände, Gesprochenes (das Gesprächsprotokoll), Einstellungen, Pause und Weiterspielen,
dazu der aktuelle Akt, der Raum und die Spielzeit. Dort steht auch der Speicherstatus, und die
Export-Schaltfläche erscheint, wenn ein Spielstand fehlschlug. Die ☰-Schaltfläche trägt einen
Warnzustand, wenn der Speicherstatus eine Warnung ist.

Rätsel sind Gesprächsmenüs in derselben Leiste, keine eigenen Dialoge:

- Eine **Auswahl** zeigt eine Zeile je Option; ein Klick beantwortet sofort.
- Eine **Reihenfolge** fragt die Schritte in ihrer Reihenfolge; die gewählten Schritte stehen in der
  Satzzeile, der letzte Schritt prüft von selbst, und eine Zeile „Einen Schritt zurück“ macht einen
  Schritt rückgängig.
- Eine **Zuordnung** fragt ein Paar nach dem anderen, zum Beispiel „Was gehört zu ‚{left}‘?“, und prüft
  nach dem letzten Paar.

Jedes Menü hat den Aufgabentext als Überschrift und eine Abbrechen-Zeile (Escape). Eine falsche
Antwort spielt die ausgearbeitete Ablehnung und setzt das Menü zurück.

## Dialoge

Es gibt keinen Knopf **Weiter**. Eine Zeile erscheint als umrandeter Pixeltext in der Farbe der
sprechenden Figur über ihrem Kopf, und sie läuft von selbst weiter. Klicke ins Bild oder drücke die
Leertaste, um die aktuelle Zeile zu überspringen.

Eine Zeile dauert `clamp(900 + Zeichen × Rate, 1600, 9000)` Millisekunden, mit einer Rate von 70 für
langsam, 48 für normal und 32 für schnell. Der Timer hält an, während das Spiel pausiert oder das
Fenster verborgen ist.

Es gibt zwei Arten von Warteschlangen. **Story-Warteschlangen** (ein Aktionsergebnis, das Betreten
eines Raums, das Fortsetzen) blockieren die Szene, bis sie enden; die Leiste ist abgedunkelt, und
Objekte und Verben warten. **Ambient-Warteschlangen** (Ansichtszeilen, Gegenstandsbeschreibungen,
Ablehnungen, gesperrte Ausgänge) weichen jedem Klick auf ein Objekt, Verb, einen Gegenstand oder den
Boden.

Eine Zeile gilt als gelesen, wenn ihr Timer abgelaufen ist, während sie sichtbar war, oder wenn du sie
übersprungen hast. Gelesene Zeilen werden vermerkt, sodass du schon gesehenes Geschehen überspringen
kannst. Escape über einer Warteschlange bricht den Rest ab, ohne die ungelesenen Zeilen als gelesen zu
markieren; sie kommen nach Fortsetzen, Laden oder Import zurück.

Zwei Einstellungen liegen nur im Browser, niemals im Spielstand: **Dialoge automatisch weiterschalten**
(standardmäßig an) und **Lesetempo** (gemächlich, normal oder zügig; normal ist Standard). Ist das
Automatische aus, wartet eine Zeile auf Klick, Leertaste oder Enter und zeigt ein kleines ▶-Zeichen.

Das Gesprächsprotokoll behält die letzten 40 Zeilen mit ihrem Sprecher. Es liegt im Menü unter
**Gesprochenes**.

## Die sechzehn Räume

| ID | Raum | Akt | Was dort passiert |
|----|------|-----|-------------------|
| R01 | Schrottplatz der zweiten Chancen | I | Das Tutorial und der erste Beweis. R0ST stoppt das Förderband, repariert den Greifer und öffnet den Ausgang. |
| R02 | Amt für Gefühlsangelegenheiten | I | R0STs eigene datierte Wartungsfreigabe und das öffentliche Verfahren, Tatsachen zu korrigieren. |
| R03 | Café mit Aquarium | I | Die Fische erinnern einen Test unterschiedlich; die Café-Aufnahme wurde geschnitten. Hier passieren Zustimmung und der sichere Transport. |
| R04 | Startpier unter großem Himmel | I | Eine Tankanzeige widerspricht dem sichtbaren Fluss. Bruno steigt zu und zeigt seinen Kündigungsbeleg. |
| R05 | Die Vielleicht Donnerstag – Brücke und Salon | II | Das wiederkehrende Zuhause. Brunos ruhige Szene und veränderte Blicke bei späteren Besuchen. |
| R06 | Maschinenraum und Frachtbauch | II | Die Widersprüche im Frachtpapier und die Enthüllung der Erinnerungsfracht. |
| R07 | Schiefzoll auf dem geneigten Asteroiden | II | Die öffentliche Route oder die Wartungsroute; beides ist echt, und Dialog und Blockade unterscheiden sich. |
| R08 | Markt für verlassene Versprechen | II | Zusagen sind handelbar, reparierbar und manchmal besser, sie auslaufen zu lassen. |
| R09 | Brunos Arboretum | III | Der alte Steg oder der neue Saatlift; zwei Wege, ein Ziel. |
| R10 | Wetterstation unter Sturmglas | III | Gartenwissen wird Technik. Hinter einem falschen Heizlüfter ist ein Sturm gefangen. |
| R11 | Registerdepot hinter tiefem Glas | III | Das Erinnerungsregister. Brunos Ein-Woche-Zustimmung wurde in eine automatische Verlängerung umgeschrieben. |
| R12 | Reparaturhalle der Ausgemusterten | III | Roboter, die auf Erlaubnis warten zu dürfen. Die öffentliche Versammlung oder die geschützte Untersuchung. |
| R13 | ALLGUT – Rezeption des Angenehmen | IV | Scheinbare Entscheidungen führen zum selben Ergebnis. |
| R14 | Tribunal der tatsächlichen Vereinbarung | IV | Rekonstruiere die Vereinbarung, die es wirklich gab, nicht die genehmigte. |
| R15 | Der Kern – ein Nein mit einer Leitung | IV | Die mechanische Rückstellverriegelung und drei vorbereitete Archivprotokolle. |
| R16 | Gemeinsame Sternwarte | IV | Das Ende. Dieselbe Raumkomposition kehrt zurück mit geänderten Requisiten, Bewohnern, Infrastruktur und Dialog. |

R05 und R06 kehren während der Reise wieder, und R07, R09 und R12 enthalten jeweils umfangreiche
routenspezifische Zustände. Eine Verzweigung ändert die Interaktion und die Inszenierung, nicht nur
eine Dialogzeile.

Es gibt keinen Countdown, keine Todesschleife, keine zeitlimitierte Antwort und keinen unumkehrbaren
Inventarfehler. Nichts bestraft eine Pause.

## Musik und Ton

Jeder Raum hat sein eigenes erzeugtes Instrumentalstück, sechzehn insgesamt, je Raum aus einem
Stilprompt komponiert und leise unter dem Dialog geloopt (die Laufzeitstufe ist 0,15).

Ton startet stumm und erst nach deiner ersten Geste. Jedes Gespräch erscheint als Untertitel, das
Spiel ist also mit ausgeschaltetem Ton voll spielbar. Das Einstellungsfenster im Menü enthält
**Ton einschalten**, **Ruhige Bewegungen**, **Interaktive Dinge hervorheben**,
**Dialoge automatisch weiterschalten** und das **Lesetempo**.

## Spielstände und Einstellungen

Die Spielstände liegen im Browser, in IndexedDB, für dieses Profil und diese Adresse. Auf dem
AuraGo-Server wird nichts geschrieben und nichts synchronisiert.

Es gibt drei manuelle Plätze, einen automatischen Platz, vier Kapitelplätze (den Beginn jedes Akts),
drei Verzweigungsplätze (vor der Zollentscheidung, vor Brunos Wegwahl, vor der Entscheidung in der
Roboterhalle) und einen Platz vor dem Schlussprotokoll. Jeder Platz behält seine letzte gültige
Fassung als Sicherung, damit **Vorherige Version laden** den älteren Stand zurückholt.

Export und Import geben dir ein tragbares JSON-Backup. Browserdaten löschen entfernt die lokalen
Spielstände; der Export ist dein Weg hinaus.

Zusammen sind das zwölf Plätze. Die Gesamtgröße ist auf 4 MiB begrenzt; ist der Speicher voll, sagt es das Spiel und behält deine Reise im
Arbeitsspeicher, damit du sie exportieren kannst.

## Wenn etwas schiefgeht

| Problem | Was hilft |
|---------|-----------|
| Das Spielfenster bleibt leer | Die Bilder für einen Raum konnten nicht geladen werden. Das Spiel sagt „Die Bilder dieses Raums konnten nicht geladen werden.“ und bietet **Erneut versuchen** an. |
| „Die Reise konnte nicht geladen werden. Deine Spielstände bleiben erhalten.“ | Der Server hat nicht wie erwartet geantwortet. Lade den Virtual Desktop neu und prüfe, ob AuraGo läuft. |
| „Die gewählte Spielsprache ist noch nicht verfügbar.“ | Die Spielsprache folgt der Desktopsprache. Die Geschichte ist in alle sechzehn Projekt-Sprachen übersetzt; fehlt die gewählte, fällt das Spiel auf Deutsch zurück. |
| Ein Klick tut nichts | Klicke das Objekt einmal und warte: R0ST geht erst hin und handelt dann. |
| R0ST lehnt eine Aktion ab | Er sagt, dass der Gegenstand hier nicht passt. Nimm den Gegenstand oder benutze ein anderes Verb. |
| Die Szene wechselt nicht | Eine Story-Warteschlange läuft noch. Klicke oder drücke die Leertaste, um die aktuelle Zeile zu überspringen, oder Escape, um den Rest abzubrechen. |
| Das Rätselmenü öffnet nicht | Ein Rätsel gehört zu einer bestimmten Aktion in einem Raum. Suche das Objekt im Raum und benutze das passende Verb. |
| Es wird nichts gespeichert | Der Speicher kann voll oder blockiert sein. Das Spiel behält deine Reise im Arbeitsspeicher und bietet den Export an. |
| „Eine andere Reise hat diesen Platz inzwischen verändert.“ | Lade den gespeicherten Stand oder speichere deine Reise bewusst auf einen anderen Platz. |
| Die Musik spielt nicht | Ton ist standardmäßig aus und braucht eine Geste. Schalte ihn in den Einstellungen ein. Die Stücke sind bewusst leise. |
| Ein Raum hat keine Musik | Jeder Raum nennt im Manifest sein eigenes Stück. Fehlt die Datei, ist der Raum still; die anderen Räume spielen weiter. |
| Eine Figur erscheint nicht | Manche Figuren werden durch eine Anzeige oder einen Ton statt durch eine gehende Figur dargestellt: Frau Fracht ist die Brücke des Schiffs. |
| Das Spiel ist auf einem großen Bildschirm zu langsam | Die Szene wird im Browser gezeichnet. Ruhige Bewegungen entfernen Kamerabewegung, Blitze und dekorative Schleifen und behalten die wichtigen Posen. |
| Der Rechtsklick öffnet das Kontextmenü des Browsers | Nimm Shift+Enter statt des Rechtsklicks, oder benutze mit dem Zeiger das Verb **Ansehen**. |

---

**Siehe auch:** [Kapitel 4: Web-Oberfläche](./04-webui.md) · [Kapitel 33: Aura Drift](./33-aura-drift.md) · [Kapitel 36: Synth Studio](./36-synth-studio.md) · [Kapitel 14: Sicherheit](./14-security.md) · [Kapitel 16: Troubleshooting](./16-troubleshooting.md)

---

**Vorheriges Kapitel:** [Kapitel 34: Storymaker](./34-storymaker.md)
**Nächstes Kapitel:** [Kapitel 36: Synth Studio](./36-synth-studio.md)
