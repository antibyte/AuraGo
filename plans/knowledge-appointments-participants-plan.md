# Plan: Knowledge Appointments Participants

## Kurzbewertung des geprüften Stands

Die Idee aus dem bisherigen Bericht ist richtig, aber der Stand im Code ist bereits weiter als der alte Entwurf:

- Das Datenmodell wurde in [internal/planner/planner.go](internal/planner/planner.go) um `contact_ids` und `participants` erweitert.
- Die Migration für `appointment_contacts` existiert bereits in [internal/planner/planner_schema.go](internal/planner/planner_schema.go).
- API, Modal, Kartenansicht und Übersetzungen wurden schon teilweise angepasst.

Offen sind jetzt vor allem die Punkte, die für ein sicheres Merge wichtig sind:

- fehlende Validierung und Transaktionssicherheit beim Speichern von Teilnehmern
- mögliche verwaiste Datensätze beim Löschen von Terminen
- fehlende Tests für Migration, CRUD und Handler
- UI/UX-Lücken und kleine Inkonsistenzen im Picker

Dieser Plan ersetzt deshalb den alten Architektur-Entwurf durch einen konkreten Restarbeitsplan.

## Ziel

Termine in der Wissenszentrale sollen Beteiligte aus dem Adressbuch zuverlässig speichern, korrekt wieder ausliefern und im UI sauber bedienbar anzeigen, ohne Inkonsistenzen zwischen Planner, Contacts-DB und Frontend zu erzeugen.

## Nicht-Ziel für diesen Schritt

- keine neue Terminfilterung nach Beteiligten
- keine automatische Knowledge-Graph-Kanten für Teilnehmer
- keine Erstellung neuer Kontakte direkt aus dem Termin-Modal

## Befunde aus der Prüfung

### Backend

- [internal/server/handlers_planner.go](internal/server/handlers_planner.go) speichert Teilnehmer aktuell nachgelagert und schluckt Fehler nur als Warnung.
- [internal/planner/planner_appointment_contacts.go](internal/planner/planner_appointment_contacts.go) dedupliziert IDs, validiert aber nicht, ob Kontakte wirklich existieren.
- [internal/planner/planner.go](internal/planner/planner.go) löscht Termine, bereinigt aber die Join-Tabelle nicht explizit.
- Die Enrichment-Logik arbeitet funktional, macht aber mehrfach Einzelabfragen pro Termin.

### Frontend

- [ui/knowledge.html](ui/knowledge.html) und [ui/js/knowledge/appointments.js](ui/js/knowledge/appointments.js) haben den Picker bereits, aber noch ohne vollständige UX-Politur.
- Die Suche berücksichtigt aktuell Name, Mail und Telefon, jedoch nicht Beziehung.
- Der Empty-State im Picker nutzt noch einen Kontakt-Text statt eines teilnehmerspezifischen Texts.
- Styling und Zustände funktionieren grob, sollten aber noch gegen bestehende Design-Tokens und Mobile-Verhalten geprüft werden.

### Qualitätssicherung

- In [internal/planner/planner_test.go](internal/planner/planner_test.go) fehlen Tests für `appointment_contacts`.
- Für die Planner-HTTP-Handler gibt es derzeit keine gezielten Tests für Teilnehmer-Payloads.

## Umsetzungsplan

### Phase 1: Datenintegrität absichern

Ziel: Keine stillen Teilfehler mehr zwischen Termin und Teilnehmer-Zuordnung.

Arbeitspakete:

1. Schreibpfad für Termine und Teilnehmer atomar machen.
2. `contact_ids` vor dem Persistieren gegen die Contacts-DB validieren.
3. Doppelte und leere IDs weiterhin serverseitig bereinigen.
4. Klare 4xx-Fehler für ungültige oder nicht mehr vorhandene Kontakte zurückgeben.
5. Löschpfad so anpassen, dass `appointment_contacts` beim Entfernen eines Termins sauber bereinigt wird.

Betroffene Dateien:

- [internal/planner/planner.go](internal/planner/planner.go)
- [internal/planner/planner_appointment_contacts.go](internal/planner/planner_appointment_contacts.go)
- [internal/server/handlers_planner.go](internal/server/handlers_planner.go)

Abnahme:

- Ein Termin wird nur dann erstellt oder aktualisiert, wenn auch seine Teilnehmer-Zuordnungen erfolgreich gespeichert wurden.
- Ein gelöschter Termin hinterlässt keine verwaisten Zeilen in `appointment_contacts`.
- Ungültige `contact_ids` führen zu einem verständlichen API-Fehler statt zu stiller Teilfunktion.

### Phase 2: Lesepfade und Datenanreicherung härten

Ziel: Konsistente API-Antworten bei vertretbarer Komplexität.

Arbeitspakete:

1. Enrichment so umbauen, dass Kontaktdaten möglichst gebündelt statt pro Termin mehrfach geladen werden.
2. API-Antworten für `GET /api/appointments` und `GET /api/appointments/{id}` konsistent halten.
3. Verhalten definieren, wenn Contacts-DB nicht verfügbar ist:
   - mindestens `contact_ids` aus Planner weiterhin liefern
   - `participants` sauber leer oder mit klaren Placeholdern zurückgeben
4. Reihenfolge der ausgewählten Teilnehmer stabil halten.

Betroffene Dateien:

- [internal/planner/planner_appointment_contacts.go](internal/planner/planner_appointment_contacts.go)
- [internal/server/handlers_planner.go](internal/server/handlers_planner.go)

Abnahme:

- Listen- und Detailansicht liefern dieselbe Teilnehmerlogik.
- Fehlende Kontakte führen nicht zu kaputten Responses.
- Mehrere Termine mit Teilnehmern erzeugen keine unnötige N+1-Last.

### Phase 3: UI und UX finalisieren

Ziel: Der Picker soll sich wie ein fertiges Produkt verhalten, nicht wie ein technischer Proof of Concept.

Arbeitspakete:

1. Teilnehmer-Suche um `relationship` ergänzen.
2. Eigenen Empty-State und Fehlertext für den Picker verwenden.
3. Darstellung der Teilnehmer in Termin-Karten visuell sauber einpassen.
4. Inline-Styles im Chip-Empty-State vermeiden und in CSS überführen.
5. Prüfen, dass Bearbeiten, Entfernen und erneutes Öffnen des Modals keinen stale State hinterlassen.
6. Mobile-Verhalten für Dropdown, Chips und Kartenansicht prüfen.

Betroffene Dateien:

- [ui/knowledge.html](ui/knowledge.html)
- [ui/js/knowledge/appointments.js](ui/js/knowledge/appointments.js)
- [ui/css/knowledge.css](ui/css/knowledge.css)
- [ui/lang/knowledge/de.json](ui/lang/knowledge/de.json)
- [ui/lang/knowledge/en.json](ui/lang/knowledge/en.json)
- weitere Dateien unter `ui/lang/knowledge/`

Abnahme:

- Teilnehmer können hinzufügen, entfernen und beim Editieren korrekt wiedersehen.
- Keine Duplikate im Picker.
- Termin-Karten zeigen Teilnehmer kompakt und stabil an.
- Das Verhalten funktioniert auch auf kleinen Screens.

### Phase 4: Tests und Merge-Sicherheit

Ziel: Die Änderung wird gegen Regressionen abgesichert.

Arbeitspakete:

1. Planner-Tests für Migration nach V5 ergänzen.
2. Tests für `SetAppointmentContacts`, `GetAppointmentContactIDs` und Delete-Cleanup ergänzen.
3. Handler-Tests für `POST`, `PUT`, `GET list`, `GET detail` mit Teilnehmerdaten ergänzen.
4. Negativtests für ungültige `contact_ids` ergänzen.
5. Wenn praktikabel, einen kleinen Frontend-Smoke-Test oder dokumentierte manuelle UAT-Schritte festhalten.

Betroffene Dateien:

- [internal/planner/planner_test.go](internal/planner/planner_test.go)
- neue Tests in [internal/server](internal/server)

Abnahme:

- Migrationstest bestätigt das neue Schema.
- CRUD-Tests decken Teilnehmer-Zuordnungen mit ab.
- Fehlerfälle sind automatisiert abgesichert.

## Empfohlene Reihenfolge

1. Phase 1 zuerst abschließen, weil sonst stille Datenfehler möglich bleiben.
2. Danach Phase 2, damit API-Verhalten und Lesepfade stabil werden.
3. Anschließend Phase 3 für UI-Polish und konsistente Texte.
4. Phase 4 parallel zum Abschluss der jeweiligen Backend- und UI-Arbeiten einziehen, spätestens aber vor Merge vollständig abschließen.

## Risiken und Entscheidungen

- Da Planner- und Contacts-Daten getrennt liegen, sind klassische Foreign Keys vermutlich nicht zuverlässig nutzbar. Integrität muss daher im Anwendungscode erzwungen werden.
- Wenn Kontakte nachträglich gelöscht werden, muss bewusst entschieden werden, ob die API Placeholder zurückgibt oder Zuordnungen aktiv bereinigt.
- Wenn die Änderung ohne zusätzliche Handler-Tests gemerged wird, ist das Risiko für stille Regressionsfehler hoch.

## Definition of Done

- Termine speichern Teilnehmer zuverlässig über `contact_ids`.
- API liefert stabile `participants`-Daten für Liste und Detail.
- Löschen hinterlässt keine verwaisten Zuordnungen.
- Picker funktioniert beim Anlegen und Bearbeiten ohne State-Fehler.
- Übersetzungen sind in allen `knowledge`-Sprachdateien vorhanden.
- Relevante Planner- und Handler-Tests sind grün.
