# Konzept: Komprimierte Gesprächsinhalte in den VectorDB-Index schreiben

## Analyse des Status Quo

### Aktueller Kompressions-Flow

```
Nachrichten akkumulieren in HistoryManager (JSON persistiert)
    │
    ▼  TotalChars() >= charLimit
Älteste Nachrichten werden ausgewählt (GetOldestMessagesForPruning)
    │
    ▼  LLM-Aufruf (~2 Min Timeout)
"Persistent Summary" wird erzeugt (SetSummary → chat_history.json)
    │
    ▼
Originalnachrichten werden gelöscht (DropMessages + DeleteMessagesByID)
    │
    ▼  In jedem Request
Summary wird als [CONTEXT_RECAP]-System-Message injiziert
```

### Was dabei verloren geht

- **Detailwissen**: Code-Snippets, exakte Werte, Dateinamen, Tool-Resultate
- **Semantische Auffindbarkeit**: Die Summary ist nicht über RAG erreichbar — sie wird nur verbatim injiziert
- **Historische Tiefe**: Es existiert immer nur **eine** Summary; ältere werden in neuere eingearbeitet und progressiv verdünnt
- **Chat-Reset-Verlust**: Bei `/clear` (Clear-Button) wird `CurrentSummary` zurückgesetzt — älterer Kontext ist vollständig weg

### Was der VectorDB-Layer heute macht

Der `ChromemVectorDB` speichert Konzept/Inhalt-Paare und erlaubt semantische Suche (`SearchSimilar`). Einträge werden bisher **explizit** gespeichert — nur wenn der Agent `manage_memory` aufruft oder beim Start Tool-Guides und Dokumentation indexiert werden. Gesprächsinhalte landen **nie automatisch** im Index.

---

## Bewertung der Optionen

### Option A: Rohe Nachrichten vor der Kompression speichern

Idee: Die `messagesToSummarize`-Liste direkt vor dem LLM-Aufruf als Einzeldokumente in den VectorDB schreiben.

| Aspekt | Bewertung |
|--------|-----------|
| Inhaltstreue | ✅ Maximale Detailtreue |
| Qualität | ❌ Hoher Rauschanteil (Begrüßungen, Tool-Echo, repetitive Formulierungen) |
| Volumen | ❌ Pro Kompressionszyklus N Embeddings (typisch 5–20 Nachrichten) |
| API-Kosten | ❌ Jedes Embedding kostet einen Embedding-API-Aufruf |
| Auffindbarkeit | ⚠ Gesprächsrauschen senkt die Retrieval-Qualität |
| Implementierung | Mittel |

**Fazit: Nicht empfohlen.** Flooding des Indexes mit Rohtext erzeugt semantisches Rauschen und erhöht Kosten.

---

### Option B: Die generierte Summary in den VectorDB schreiben ✅ EMPFOHLEN

Idee: Unmittelbar nach `SetSummary(newSummary)` wird die Summary als separates Dokument in den VectorDB-Index geschrieben.

```go
// Nach SetSummary im Kompressions-Goroutine:
if s.LongTermMem != nil && !s.LongTermMem.IsDisabled() {
    concept := fmt.Sprintf("Gesprächszusammenfassung %s", time.Now().Format("2006-01-02 15:04"))
    go func() {
        _, err := s.LongTermMem.StoreDocument(concept, newSummary)
        if err != nil {
            s.Logger.Warn("[Compression] Failed to archive summary to VectorDB", "error", err)
        } else {
            s.Logger.Info("[Compression] Summary archived to VectorDB", "concept", concept)
        }
    }()
}
```

| Aspekt | Bewertung |
|--------|-----------|
| Inhaltstreue | ✅ LLM hat bereits Fakten extrahiert und Rauschen entfernt |
| Qualität | ✅ Höchste Semantik-Dichte |
| Volumen | ✅ **1 Dokument** pro Kompressionszyklus |
| API-Kosten | ✅ Minimal (eine Embedding-Anfrage pro Kompression) |
| Auffindbarkeit | ✅ Semantisch suchbar via RAG |
| Chat-Reset-Sicherheit | ✅ Überlebt `/clear`, da VectorDB unabhängig von `chat_history.json` |
| Implementierung | Einfach (3–5 Zeilen im bestehenden Goroutine) |

**Fazit: Klar empfohlen.** Geringe Kosten, hohes Informationsgewicht, robuste Persistenz.

---

### Option C: Hybrid — wichtige User-Nachrichten + Summary

Idee: Zusätzlich zur Summary werden User-Nachrichten, die „substanziell" sind (z. B. > 200 Zeichen, keine Ein-Wort-Antworten), ebenfalls gespeichert. Tool-Antworten und Agent-Outputs werden weiterhin nur zusammengefasst.

**Mehrwert vs. Option B:** Exakter Wortlaut von User-Anforderungen bleibt searchable (hilfreich bei lange zurückliegenden technischen Anforderungen).  
**Nachteil:** Komplexer Code, partielles Rauschen, unklare Schwellwert-Heuristik.

**Fazit: Optionaler späterer Schritt.** Zunächst Option B implementieren und beobachten, ob RAG-Trefferqualität ausreicht.

---

## Empfohlene Implementierung (Option B)

### Änderung in `internal/server/handlers.go`

Im Kompressions-Goroutine, direkt nach dem Block ab `s.HistoryManager.SetSummary(newSummary)`:

```go
// Bestehend:
if len(resp.Choices) > 0 {
    newSummary := resp.Choices[0].Message.Content
    s.HistoryManager.SetSummary(newSummary)
    s.HistoryManager.DropMessages(dropIDs)
    if err := s.ShortTermMem.DeleteMessagesByID(sessionID, dropIDs); err != nil {
        s.Logger.Error("[Compression] Failed to clean up SQLite memory", "error", err)
    }
    s.Logger.Info("[Compression] Background summarization complete and saved", ...)

    // NEU: Summary in VectorDB archivieren
    if s.LongTermMem != nil && !s.LongTermMem.IsDisabled() {
        concept := fmt.Sprintf("Gesprächszusammenfassung %s", time.Now().Format("2006-01-02 15:04"))
        go func(concept, summary string) {
            if _, err := s.LongTermMem.StoreDocument(concept, summary); err != nil {
                s.Logger.Warn("[Compression] VectorDB archive failed", "error", err)
            }
        }(concept, newSummary)
    }
}
```

### Warum ein separater Goroutine?

Der VectorDB-`StoreDocument`-Aufruf benötigt bis zu 30 Sekunden (Embedding-API-Timeout). Der Kompressions-Goroutine läuft bereits im Hintergrund, aber das VectorDB-Schreiben soll den Kompressionspfad nicht weiter verzögern.

---

## Erwarteter Effekt auf RAG

### Heute
```
User-Frage: "Welche Konfigurationsdatei haben wir letzte Woche für X geändert?"
RAG-Ergebnis: Nichts gefunden (Gespräch ist seit langem komprimiert, nur in Summary)
```

### Mit Option B
```
User-Frage: "Welche Konfigurationsdatei haben wir letzte Woche für X geändert?"
RAG-Ergebnis: Treffer in "Gesprächszusammenfassung 2026-03-01 14:23" → Summary enthält die Entscheidung
```

### Grenzfall: Chat-Reset
```
Ohne Option B: Alle früheren Inhalte durch /clear verloren
Mit Option B:  VectorDB-Summaries überleben den Reset; RAG kann weiterhin daraus schöpfen
```

---

## Risiken und Gegenmaßnahmen

| Risiko | Gegenmaßnahme |
|--------|---------------|
| VectorDB wird mit älteren Summaries überflutet | Summaries sind klein (< 1.000 Chars) und qualitativ hochwertig — kein praktisches Problem |
| Embedding-API nicht erreichbar | `IsDisabled()`-Check schützt, Fehler nur geloggt (kein Hard-Fail) |
| Doppelte Einträge bei mehrfacher Kompression | Timestamps im Konzeptnamen machen Dokumente eindeutig |
| Alte Summaries aus anderen Kontexten kontaminieren RAG | Akzeptables Risiko; Summaries sind thematisch breit und für viele Fragen relevant |

---

## Entscheidung

| Frage | Antwort |
|-------|---------|
| Macht es Sinn? | **Ja** — klarer Mehrwert bei minimalem Aufwand |
| Rohe Nachrichten oder Summary? | **Summary** — LLM-destilliert, kein Rauschen |
| Wann implementieren? | Empfohlen als nächster kleiner Schritt |
| Abhängigkeiten? | Keine — LongTermMem ist bereits im Server-Struct vorhanden |

---

## Verwandte Konzepte

- [prompt_builder_optimization_concept.md](prompt_builder_optimization_concept.md) — Tier-System und Budget-Shedding
- [user_profiling_plan.md](user_profiling_plan.md) — Parallele Archivierung von User-Präferenzen
