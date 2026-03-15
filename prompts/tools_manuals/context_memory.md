## Tool: Context Memory (`context_memory`)

Kontext-bewusste Memory-Abfrage über alle Speicherebenen hinweg. Kombiniert Long-Term Memory, Knowledge Graph, Journal und Notizen in einer intelligenten Suche.

### Wann verwenden?

- Wenn `query_memory` nicht genug liefert
- Wenn du Zusammenhänge brauchst (nicht nur einzelne Fakten)
- Wenn du Zeit-Relevanz berücksichtigen willst
- Wenn du Beziehungen zwischen Entitäten finden willst

### Parameter

| Parameter | Typ | Default | Beschreibung |
|-----------|-----|---------|--------------|
| `query` | string | required | Natürlichsprachige Suchanfrage |
| `context_depth` | string | "normal" | shallow/normal/deep |
| `sources` | array | ["ltm", "kg"] | Welche Ebenen durchsuchen |
| `time_range` | string | "all" | all/today/last_week/last_month |
| `include_related` | boolean | true | KG-Nachbarn einbeziehen |

### Context Depth

- **shallow**: Direkte Treffer nur
  - Schnell, präzise
  - Nutze für: Schnelle Fakten-Prüfung

- **normal**: Treffer + direkte KG-Nachbarn
  - Balanciert
  - Nutze für: Standard-Recherchen

- **deep**: Volle Graph-Expansion + temporale Suche
  - Gründlich
  - Nutze für: Komplexe Zusammenhänge

### Sources

| Source | Enthält | Nutzen |
|--------|---------|--------|
| `ltm` | VectorDB-Dokumente | Fakten, Setups, Learnings |
| `kg` | Knowledge Graph | Beziehungen, Entitäten |
| `journal` | Journal-Einträge | Meilensteine, Reflektionen |
| `notes` | Notizen/Todos | Aktuelle Tasks |
| `core` | Core Memory | Präferenzen, Identität |

### Beispiele

#### Suche nach Projektzusammenhang

```json
{"action": "context_memory", "query": "AuraGo Projekt", "context_depth": "deep", "sources": ["ltm", "kg", "journal"]}
```

**Ergebnis:**
```
📚 LTM (3 Treffer):
   • "AuraGo Docker Setup" [Score: 0.94]
   • "AuraGo Projektstruktur" [Score: 0.87]
   • "AuraGo GitHub Integration" [Score: 0.82]

🔗 Knowledge Graph:
   Andre ──arbeitet_an──► AuraGo ──nutzt──► Docker
                              │
                              ├──nutzt──► SQLite
                              │
                              └──nutzt──► Go 1.26

📔 Journal:
   • [15.03] Meilenstein: "AuraGo initial setup completed"
   • [14.03] Task: "AuraGo Docker Compose erstellt"
```

#### Zeitlich eingeschränkte Suche

```json
{"action": "context_memory", "query": "Docker Fehler", "time_range": "last_week", "sources": ["journal", "ltm"]}
```

#### Schnelle Fakten-Prüfung

```json
{"action": "context_memory", "query": "Server IP", "context_depth": "shallow", "sources": ["core", "kg"]}
```

### Kombinierte Ergebnisse

Das Tool liefert ein **kombiniertes Ranking**:

```json
{
  "status": "success",
  "combined_results": [
    {
      "rank": 1,
      "source": "kg",
      "type": "entity_network",
      "relevance": 0.96,
      "content": "Andre → arbeitet_an → AuraGo → läuft_auf → Proxmox",
      "reasoning": "Direkte Verbindung zum User"
    },
    {
      "rank": 2,
      "source": "ltm",
      "type": "document",
      "relevance": 0.91,
      "content": "AuraGo Proxmox Deployment Guide...",
      "doc_id": "mem_12345"
    },
    {
      "rank": 3,
      "source": "journal",
      "type": "milestone",
      "relevance": 0.88,
      "content": "AuraGo erfolgreich auf Proxmox deployed",
      "date": "2026-03-10"
    }
  ]
}
```

### Best Practices

1. **Tiefe nach Komplexität wählen**
   - Einfache Fakten → shallow
   - Standard-Suche → normal
   - Troubleshooting → deep

2. **Sources einschränken für Fokus**
   - Technische Fragen → ["ltm", "notes"]
   - Organisatorisches → ["journal", "notes"]
   - Alles → ["ltm", "kg", "journal", "notes", "core"]

3. **Time Range für Frische**
   - "Was haben wir gestern..." → "last_week"
   - Aktuelle Todos → "today"
   - Historisches → "all"

4. **Related Entities für Kontext**
   - `include_related: true` → Findet verbundene Infos
   - z.B. Suche nach "Docker" findet auch "AuraGo" wenn verknüpft
