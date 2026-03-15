## Tool: Smart Memory (`smart_memory`)

Intelligentes Memory-Tool mit automatischer Erkennung und Speicherempfehlungen. Dieses Tool analysiert Text, erkennt speicherwerte Informationen und schlägt den optimalen Speicherort vor.

### Wann verwenden?

- **Auto-Extraktion**: Nach wichtigen User-Äußerungen automatisch ausführen
- **Smart Store**: Wenn du dir unsicher bist, WO etwas gespeichert werden soll
- **Smart Query**: Wenn du Informationen aus ALLEN Memory-Ebenen brauchst
- **Konsolidierung**: Am Ende einer Session zur Zusammenfassung

### Operationen

| Operation | Beschreibung | Auto-Confirm |
|-----------|--------------|--------------|
| `auto_extract` | Analysiert Text und schlägt Speicherorte vor | Bei Confidence >95% |
| `store` | Direktes Speichern mit Smart-Routing | Nein |
| `query` | Intelligente Suche über alle Ebenen | Ja |
| `consolidate` | Analysiert Session und schlägt Aktionen vor | Konfigurierbar |
| `suggest` | Holt proaktive Vorschläge | Ja |

### Auto-Extraktion Beispiel

**User sagt:** "Mein Name ist Andre, ich arbeite am AuraGo Projekt und bevorzuge Docker Compose."

**Tool Call:**
```json
{"action": "smart_memory", "operation": "auto_extract", "content": "Mein Name ist Andre, ich arbeite am AuraGo Projekt und bevorzuge Docker Compose.", "context": "conversation"}
```

**Ergebnis:**
```json
{
  "status": "success",
  "findings": [
    {
      "type": "entity",
      "summary": "User heißt Andre",
      "storage_recommendation": "kg",
      "confidence": 0.99,
      "auto_stored": true
    },
    {
      "type": "entity", 
      "summary": "Projekt AuraGo",
      "storage_recommendation": "kg",
      "confidence": 0.95,
      "auto_stored": true
    },
    {
      "type": "relation",
      "summary": "Andre arbeitet an AuraGo",
      "storage_recommendation": "kg",
      "confidence": 0.98,
      "auto_stored": true
    },
    {
      "type": "preference",
      "summary": "Bevorzugt Docker Compose",
      "storage_recommendation": "core",
      "suggested_key": "docker_preference",
      "confidence": 0.92,
      "auto_stored": false,
      "reason": "Warte auf Bestätigung bei Präferenzen"
    }
  ]
}
```

### Smart Store Beispiel

```json
{"action": "smart_memory", "operation": "store", "content": "Docker Compose Setup mit 3 Services: app, db, redis", "storage_hint": "auto"}
```

Das System entscheidet automatisch:
- Wenn Projekt bekannt → KG (Relation: Projekt → nutzt → Docker)
- Wenn technisches Detail → LTM
- Wenn wichtiger Meilenstein → Journal

### Smart Query Beispiel

```json
{"action": "smart_memory", "operation": "query", "content": "Was haben wir gestern über Docker besprochen?"}
```

Durchsucht:
1. LTM (ähnliche Dokumente)
2. Journal (Einträge vom gestrigen Tag)
3. KG (Docker-Projekt-Verbindungen)
4. Notizen (Docker-Todos)

### Konsolidierung Beispiel

```json
{"action": "smart_memory", "operation": "consolidate", "auto_mode": false}
```

Analysiert die aktuelle Session und schlägt vor:
- Präferenzen speichern?
- Journal-Eintrag erstellen?
- Fehler-Learning speichern?
- KG aktualisieren?

### Best Practices

1. **Auto-Extraktion nach wichtigen Äußerungen**
   - User stellt sich vor
   - User nennt Präferenzen
   - User beschreibt Projekt/Setup

2. **Storage Hints nur wenn nötig**
   - `"storage_hint": "auto"` → System entscheidet (empfohlen)
   - `"storage_hint": "core"` → Nur für wichtige Präferenzen
   - `"storage_hint": "journal"` → Nur für Meilensteine

3. **Auto-Confirm nutzen für:**
   - Entitäten (hohe Confidence)
   - Fakten (objektiv)
   - KEINE Präferenzen ohne Bestätigung

### Automatische Trigger

Das Tool wird AUTOMATISCH ausgeführt bei:
- Session-Ende (Konsolidierung)
- User-Einführung (Entity-Extraktion)
- Erfolgreicher Tool-Chain (Meilenstein-Check)
- Fehler mit Lösung (Learning-Extraktion)
