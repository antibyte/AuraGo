## Tool: Memory Reflection (`memory_reflect`)

Reflektiert über vergangene Interaktionen und generiert Erkenntnisse über Patterns, Fehler, Fortschritte und Beziehungen. Nützlich für kontinuierliches Lernen und Verbesserung.

### Wann verwenden?

- Am Ende einer produktiven Woche
- Nach wiederholten Fehlern (Pattern-Analyse)
- Für Fortschrittsberichte
- Um Beziehungen/Projekte zu analysieren
- Vor langfristigen Planungen

### Parameter

| Parameter | Typ | Default | Beschreibung |
|-----------|-----|---------|--------------|
| `scope` | string | required | session/day/week/month/project/all_time |
| `focus` | string | "all" | patterns/errors/progress/relationships/all |
| `output_format` | string | "summary" | summary/detailed/action_items/insights_only |

### Scope

- **session**: Aktuelle Session
- **day**: Heute
- **week**: Letzte 7 Tage
- **month**: Letzte 30 Tage
- **project**: Aktuelles Projekt (aus KG)
- **all_time**: Gesamte Historie

### Focus Areas

#### patterns
Analysiert wiederkehrende Verhaltensmuster:
- Häufige Anfrage-Themen
- Uhrzeit-Präferenzen
- Tool-Nutzungsmuster
- Session-Dauer

#### errors
Analysiert Fehler und Lösungen:
- Häufigste Fehlertypen
- Erfolgreiche Workarounds
- Wiederholte Fehler (noch nicht gelernt)
- Fehler-Trends (↗ ↘)

#### progress
Zeigt Fortschritte und Erfolge:
- Abgeschlossene Projekte/Tasks
- Gelernte Skills
- Eingerichtete Integrationen
- Meilensteine

#### relationships
Analysiert Knowledge Graph:
- Neue Entitäten
- Neue Verbindungen
- Aktive Projekte
- Wichtige Beziehungen

### Output Formats

- **summary**: Übersicht mit Highlights
- **detailed**: Vollständige Analyse mit Beispielen
- **action_items**: Konkrete Vorschläge
- **insights_only**: Nur die "Aha!"-Erkenntnisse

### Beispiele

#### Wochen-Reflektion

```json
{"action": "memory_reflect", "scope": "week", "focus": "all", "output_format": "summary"}
```

**Ergebnis:**
```
📊 Deine Woche mit AuraGo (09.03 - 15.03.2026)

🎯 Highlights:
   ✅ 4 erfolgreiche Docker-Setups
   ✅ 3 Server im Inventory registriert
   ✅ 2 Cron-Jobs eingerichtet
   ✅ 1 Python-Tool erstellt

🔄 Patterns:
   • Hauptfokus: Docker/Infrastructure (65% der Zeit)
   • Aktivste Zeit: 20:00-22:00 Uhr
   • Durchschnittliche Antwortzeit: 45s
   • Lieblings-Tools: docker, filesystem, execute_shell

⚠️ Learnings:
   • 3x Permission-Denied → sudo vergessen
     💡 Vorschlag: Sudo-Standard immer prüfen?
   
   • 2x Container-Name vergeben
     💡 Vorschlag: Naming-Convention etablieren?

📈 Knowledge Graph:
   +5 Entitäten | +8 Relationen
   Neue: Andre, AuraGo, Proxmox, Docker-Compose

🎯 Vorschläge für nächste Woche:
   1. Docker-Volumes Backup einrichten?
   2. Proxmox-Templates erstellen?
   3. SSH-Key-Management automatisieren?
```

#### Fehler-Analyse

```json
{"action": "memory_reflect", "scope": "month", "focus": "errors", "output_format": "detailed"}
```

**Ergebnis:**
```
⚠️ Fehler-Analyse (letzter Monat)

Top 3 Fehlertypen:

1. Permission Denied (12x) ↗ +3 vs. Vormonat
   ├─ Lösung gefunden: 10/12 (83%)
   ├─ Wiederholt: 2x (noch nicht gelernt)
   └─ 💡 Empfehlung: execute_sudo bevorzugen

2. Container-Port belegt (5x) → gleich
   ├─ Lösung gefunden: 5/5 (100%)
   └─ ✅ Gelernt: Prüfung vor Start implementiert

3. SSH-Key Fehler (3x) ↘ -2 vs. Vormonat
   ├─ Lösung gefunden: 3/3 (100%)
   └─ ✅ Verbesserung erkannt!
```

#### Projektspezifisch

```json
{"action": "memory_reflect", "scope": "project", "focus": "progress", "output_format": "action_items"}
```

### Best Practices

1. **Regelmäßige Reflektion**
   - Wöchentlich (Sonntagabend)
   - Nach großen Projekten
   - Bei wiederholten Fehlern

2. **Focus wählen nach Bedarf**
   - Frust mit Fehlern → `focus: "errors"`
   - Motivation boost → `focus: "progress"`
   - Überblick verlieren → `focus: "patterns"`

3. **Action Items umsetzen**
   - Nicht nur lesen!
   - Core Memory aktualisieren
   - Neue Workflows etablieren

4. **Mit User teilen**
   - Wochenberichte sind motivierend
   - Zeigt Fortschritt
   - Baut Vertrauen

### Auto-Reflektion

Das System kann Reflektionen automatisch generieren:

```yaml
# config.yaml
memory:
  reflection:
    auto_weekly: true
    day: sunday
    time: "20:00"
    share_with_user: true  # Im Chat posten
```

**Auto-Post:**
```
🤖 AuraGo Weekly Reflection

Diese Woche haben wir:
✅ 12 Tasks erledigt
✅ 3 Server eingerichtet  
⚠️ 2 Fehler behoben

Top-Learning: "Docker Compose über Docker Run bevorzugen"

[Details ansehen] [Ignorieren]
```
