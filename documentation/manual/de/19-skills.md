# Kapitel 19: Skills

Erstelle wiederverwendbare Python-Skills zur Erweiterung von AuraGos Fähigkeiten.

---

## Was sind Skills?

Skills sind benutzerdefinierte Python-Skripte, die als erweiterte Werkzeuge im Agenten verfügbar gemacht werden. Sie ergänzen die eingebauten Tools und sind ideal für:

- **Wiederkehrende Aufgaben** mit komplexer Logik
- **API-Integrationen** mit Authentication
- **Datenverarbeitung** und -transformation
- **Web Scraping** mit spezifischen Anforderungen
- **Verbindungen zu externen Diensten**

---

## Skill-Architektur

Ein Skill besteht aus zwei Dateien:

```
agent_workspace/skills/
├── mein_skill.json       # Manifest (Beschreibung, Parameter)
└── mein_skill.py         # Python-Code (Implementierung)
```

### Das Manifest (JSON)

Das Manifest beschreibt den Skill für den Agenten:

```json
{
  "name": "wetter_abfrage",
  "description": "Holt aktuelle Wetterdaten für eine Stadt",
  "parameters": {
    "type": "object",
    "properties": {
      "stadt": {
        "type": "string",
        "description": "Name der Stadt"
      },
      "einheit": {
        "type": "string",
        "enum": ["celsius", "fahrenheit"],
        "default": "celsius"
      }
    },
    "required": ["stadt"]
  },
  "returns": {
    "type": "object",
    "description": "Temperatur, Luftfeuchtigkeit und Wetterbeschreibung"
  },
  "entry_point": "wetter_abfrage.py",
  "function": "main",
  "dependencies": ["requests"],
  "vault_keys": ["openweather_api_key"]
}
```

**Felder erklärt:**

| Feld | Beschreibung | Pflicht |
|------|--------------|---------|
| `name` | Eindeutiger Name des Skills | Ja |
| `description` | Was macht der Skill (für den Agenten sichtbar) | Ja |
| `parameters` | JSON Schema der Eingabeparameter | Ja |
| `returns` | Beschreibung der Rückgabewerte | Nein |
| `entry_point` | Python-Datei die ausgeführt wird | Ja |
| `function` | Funktionsname im Python-Script (meist `main`) | Ja |
| `dependencies` | Liste der pip-Pakete | Nein |
| `vault_keys` | Liste der Vault-Secret-Namen | Nein |
| `credential_ids` | Liste der Credential-IDs | Nein |

### Der Python-Code

Der Python-Code enthält die eigentliche Logik:

```python
#!/usr/bin/env python3
"""
Skill: Wetter Abfrage
Holt aktuelle Wetterdaten von OpenWeatherMap.
"""

import os
import sys
import json
import requests


def main(stadt: str, einheit: str = "celsius") -> dict:
    """
    Hauptfunktion des Skills.
    
    Args:
        stadt: Name der Stadt
        einheit: celsius oder fahrenheit
    
    Returns:
        Dictionary mit Wetterdaten
    """
    # API Key aus dem Vault
    api_key = os.environ.get('AURAGO_SECRET_OPENWEATHER_API_KEY')
    
    if not api_key:
        return {
            "status": "error",
            "message": "API Key nicht konfiguriert. Bitte im Vault unter 'openweather_api_key' hinterlegen."
        }
    
    # Einheit für API
    units = "metric" if einheit == "celsius" else "imperial"
    
    # API-Request
    url = "https://api.openweathermap.org/data/2.5/weather"
    params = {
        "q": stadt,
        "appid": api_key,
        "units": units
    }
    
    try:
        response = requests.get(url, params=params, timeout=30)
        response.raise_for_status()
        data = response.json()
        
        return {
            "status": "success",
            "data": {
                "stadt": data["name"],
                "land": data["sys"]["country"],
                "temperatur": data["main"]["temp"],
                "einheit": einheit,
                "luftfeuchtigkeit": data["main"]["humidity"],
                "beschreibung": data["weather"][0]["description"],
                "windgeschwindigkeit": data["wind"]["speed"]
            }
        }
        
    except requests.exceptions.RequestException as e:
        return {
            "status": "error",
            "message": f"API Fehler: {str(e)}"
        }


if __name__ == "__main__":
    # Parameter werden als JSON via stdin übergeben
    try:
        params = json.load(sys.stdin)
        result = main(**params)
        print(json.dumps(result, ensure_ascii=False))
    except Exception as e:
        print(json.dumps({
            "status": "error",
            "message": str(e)
        }), file=sys.stderr)
        sys.exit(1)
```

---

## Vault Secrets in Skills nutzen

Skills können auf Secrets aus dem Vault zugreifen, um API-Keys, Tokens oder Passwörter zu verwenden.

### Schritt 1: Secret im Vault speichern

1. **Web-UI öffnen** → "Secrets" → "Neuer Secret"
2. **Name eingeben**: z.B. `openweather_api_key`
3. **Wert eingeben**: Dein API-Key
4. **Speichern**

> ⚠️ **Wichtig:** Nur Secrets, die du selbst erstellt hast, sind in Skills verfügbar. System-Secrets (wie LLM-API-Keys) sind blockiert.

### Schritt 2: Im Manifest deklarieren

```json
{
  "vault_keys": ["openweather_api_key", "weitere_keys"]
}
```

### Schritt 3: Im Python-Code nutzen

```python
import os

# Secrets sind verfügbar als: AURAGO_SECRET_<KEY_NAME>
# Der Name wird automatisch: uppercase, Sonderzeichen werden zu _
api_key = os.environ.get('AURAGO_SECRET_OPENWEATHER_API_KEY')
```

### Sicherheitshinweise

> ⚠️ **Wichtig:** Skills sind ein Sonderfall beim Vault-Zugriff!
> 
> Normalerweise hat der Agent **keinen Zugriff** auf Vault-Secrets. Bei Skills müssen die Geheimnisse jedoch an den Python-Prozess übertragen werden, um vom Skill genutzt werden zu können.
> 
> Das bedeutet:
> - Secrets **verlassen** die geschützte Vault-Umgebung von AuraGo
> - Sie werden als Umgebungsvariablen an den Skill-Prozess übergeben
> - Während der Skill-Ausführung sind sie nur noch durch die **Betriebssystem-Benutzerisolation** geschützt
> - Der Skill-Prozess läuft in einer Sandbox (venv), aber mit Zugriff auf die übergebenen Secrets
> 
> **Empfehlung:**
> - Verwende dedizierte, eingeschränkte API-Keys für Skills (nicht deine Haupt-Keys)
> - Aktiviere `tools.python_secret_injection.enabled` nur wenn nötig
> - Überprüfe den Code von Skills aus unbekannten Quellen vor der Ausführung

- Secrets werden automatisch aus allen Outputs entfernt (Scrubbing)
- Secrets sind nur während der Ausführung verfügbar
- Der Prozess läuft in einer isolierten Umgebung (venv)
- Erfordert `tools.python_secret_injection.enabled: true` in der `config.yaml`

---

## Beispiele

### Beispiel 1: Einfacher Datei-Analyzer

**Manifest** (`file_analyzer.json`):
```json
{
  "name": "file_analyzer",
  "description": "Analysiert eine Textdatei und gibt Statistiken zurück",
  "parameters": {
    "type": "object",
    "properties": {
      "dateipfad": {
        "type": "string",
        "description": "Pfad zur zu analysierenden Datei"
      }
    },
    "required": ["dateipfad"]
  },
  "entry_point": "file_analyzer.py",
  "function": "main"
}
```

**Python** (`file_analyzer.py`):
```python
#!/usr/bin/env python3
import os
import json
import sys


def main(dateipfad: str) -> dict:
    """Analysiere eine Datei und gib Statistiken zurück."""
    
    if not os.path.exists(dateipfad):
        return {
            "status": "error", 
            "message": f"Datei nicht gefunden: {dateipfad}"
        }
    
    with open(dateipfad, 'r', encoding='utf-8', errors='ignore') as f:
        content = f.read()
    
    lines = content.split('\n')
    words = content.split()
    
    return {
        "status": "success",
        "data": {
            "dateipfad": dateipfad,
            "groesse_bytes": os.path.getsize(dateipfad),
            "anzahl_zeilen": len(lines),
            "anzahl_woerter": len(words),
            "anzahl_zeichen": len(content)
        }
    }


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

**Verwendung im Chat:**
```
Du: Nutze den file_analyzer Skill für die Datei dokument.txt
Agent: 🛠️ Skill: file_analyzer
       
       ✅ Analyse abgeschlossen:
       - Größe: 12,450 Bytes
       - Zeilen: 234
       - Wörter: 1,892
       - Zeichen: 12,448
```

### Beispiel 2: GitHub Repository Info

**Manifest** (`github_repo.json`):
```json
{
  "name": "github_repo",
  "description": "Holt Informationen über ein GitHub Repository",
  "parameters": {
    "type": "object",
    "properties": {
      "owner": {
        "type": "string",
        "description": "Repository Besitzer"
      },
      "repo": {
        "type": "string",
        "description": "Repository Name"
      }
    },
    "required": ["owner", "repo"]
  },
  "entry_point": "github_repo.py",
  "function": "main",
  "dependencies": ["requests"],
  "vault_keys": ["github_token"]
}
```

**Python** (`github_repo.py`):
```python
#!/usr/bin/env python3
import os
import json
import sys
import requests


def main(owner: str, repo: str) -> dict:
    """Hole GitHub Repository Informationen."""
    
    token = os.environ.get('AURAGO_SECRET_GITHUB_TOKEN')
    
    url = f"https://api.github.com/repos/{owner}/{repo}"
    headers = {}
    
    if token:
        headers["Authorization"] = f"token {token}"
    
    try:
        response = requests.get(url, headers=headers, timeout=30)
        response.raise_for_status()
        data = response.json()
        
        return {
            "status": "success",
            "data": {
                "name": data["name"],
                "beschreibung": data["description"],
                "stars": data["stargazers_count"],
                "forks": data["forks_count"],
                "sprache": data["language"],
                "erstellt_am": data["created_at"][:10],
                "letztes_update": data["updated_at"][:10]
            }
        }
    except requests.exceptions.RequestException as e:
        return {"status": "error", "message": f"API Fehler: {str(e)}"}


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

### Beispiel 3: Daten-Konverter

**Manifest** (`daten_konverter.json`):
```json
{
  "name": "daten_konverter",
  "description": "Konvertiert Daten zwischen JSON, CSV und YAML",
  "parameters": {
    "type": "object",
    "properties": {
      "daten": {
        "type": "string",
        "description": "Die zu konvertierenden Daten als String"
      },
      "von_format": {
        "type": "string",
        "enum": ["json", "csv", "yaml"],
        "description": "Quellformat"
      },
      "zu_format": {
        "type": "string",
        "enum": ["json", "csv", "yaml"],
        "description": "Zielformat"
      }
    },
    "required": ["daten", "von_format", "zu_format"]
  },
  "entry_point": "daten_konverter.py",
  "function": "main",
  "dependencies": ["pyyaml", "pandas"]
}
```

**Python** (`daten_konverter.py`):
```python
#!/usr/bin/env python3
import json
import sys
from io import StringIO


def main(daten: str, von_format: str, zu_format: str) -> dict:
    """Konvertiere Daten zwischen verschiedenen Formaten."""
    
    try:
        # Parse Eingabe
        if von_format == "json":
            parsed = json.loads(daten)
        elif von_format == "yaml":
            import yaml
            parsed = yaml.safe_load(daten)
        elif von_format == "csv":
            import pandas as pd
            df = pd.read_csv(StringIO(daten))
            parsed = df.to_dict('records')
        
        # Konvertiere Ausgabe
        if zu_format == "json":
            output = json.dumps(parsed, indent=2, ensure_ascii=False)
        elif zu_format == "yaml":
            import yaml
            output = yaml.dump(parsed, allow_unicode=True, default_flow_style=False)
        elif zu_format == "csv":
            import pandas as pd
            if isinstance(parsed, list) and len(parsed) > 0:
                df = pd.DataFrame(parsed)
                output = df.to_csv(index=False)
            else:
                return {"status": "error", "message": "CSV erfordert Liste von Objekten"}
        
        return {
            "status": "success",
            "data": {
                "ausgabe": output,
                "eintraege": len(parsed) if isinstance(parsed, list) else 1
            }
        }
        
    except Exception as e:
        return {"status": "error", "message": f"Konvertierungsfehler: {str(e)}"}


if __name__ == "__main__":
    params = json.load(sys.stdin)
    result = main(**params)
    print(json.dumps(result, ensure_ascii=False))
```

---

## Skills erstellen und verwalten

### Skill erstellen

1. **Dateien erstellen**: Lege `.json` und `.py` Dateien in `agent_workspace/skills/` an
2. **Rechte setzen**: Stelle sicher, dass die Dateien lesbar sind
3. **Agent informieren**: Der Agent erkennt neue Skills automatisch bei `list_skills`

### Skills anzeigen

```
Du: Welche Skills sind verfügbar?
Agent: 🛠️ Verfügbare Skills:
       - file_analyzer: Analysiert Textdateien
       - github_repo: GitHub Repository Informationen
       - daten_konverter: Konvertiert JSON/CSV/YAML
```

### Skill ausführen

```
Du: Führe den file_analyzer Skill aus mit dateipfad="readme.txt"

Oder einfacher:
Du: Analysiere die Datei readme.txt mit dem file_analyzer
```

### Skill löschen

Lösche einfach beide Dateien aus dem `agent_workspace/skills/` Verzeichnis.

---

## Best Practices

### 1. Parameter-Validierung

Validiere immer die Eingaben:

```python
def main(url: str, max_items: int = 10) -> dict:
    # URL validieren
    if not url.startswith(('http://', 'https://')):
        return {"status": "error", "message": "URL muss mit http:// oder https:// beginnen"}
    
    # Bereich validieren
    if not (1 <= max_items <= 100):
        return {"status": "error", "message": "max_items muss zwischen 1 und 100 liegen"}
```

### 2. Timeouts setzen

Nutze immer Timeouts bei HTTP-Requests:

```python
import requests

# Timeout in Sekunden
response = requests.get(url, timeout=30)
```

### 3. Fehlerbehandlung

Fange spezifische Fehler ab:

```python
try:
    # Deine Logik
    return {"status": "success", "data": result}
except FileNotFoundError:
    return {"status": "error", "message": "Datei nicht gefunden"}
except requests.exceptions.RequestException as e:
    return {"status": "error", "message": f"Netzwerkfehler: {e}"}
except Exception as e:
    return {"status": "error", "message": f"Unerwarteter Fehler: {e}"}
```

### 4. Ressourcen schließen

Verwende Context Manager:

```python
# Gut - Datei wird automatisch geschlossen
with open(dateipfad, 'r') as f:
    content = f.read()

# Schlecht - Datei bleibt offen
f = open(dateipfad, 'r')
content = f.read()
```

### 5. Dependencies deklarieren

Liste alle benötigten pip-Pakete im Manifest:

```json
{
  "dependencies": ["requests", "beautifulsoup4", "pandas"]
}
```

Die Pakete werden beim ersten Aufruf automatisch installiert.

---

## Vordefinierte Templates nutzen

AuraGo bietet Templates für häufige Skill-Typen:

### Templates anzeigen

```
Du: Welche Skill-Templates gibt es?
Agent: 📋 Verfügbare Templates:
       - api_client: REST API Client mit Auth
       - file_processor: Datei-Lese- und Schreiboperationen
       - data_transformer: Datenformat-Konvertierung
       - scraper: Web Scraper mit CSS-Selektoren
```

### Skill aus Template erstellen

```json
{
  "action": "create_skill_from_template",
  "template": "api_client",
  "name": "meine_api",
  "description": "Meine API Integration",
  "vault_keys": ["api_key"]
}
```

---

## Troubleshooting

### Skill wird nicht gefunden

| Problem | Lösung |
|---------|--------|
| Dateiname falsch | Prüfe dass `.json` und `.py` identisch benannt sind |
| JSON ungültig | Validiere das JSON mit einem Online-Validator |
| Rechte fehlen | Stelle sicher dass die Dateien lesbar sind |
| Cache veraltet | Rufe `list_skills` auf um den Cache zu aktualisieren |

### Import Fehler

```
ModuleNotFoundError: No module named 'requests'
```

**Lösung:** Füge `requests` zu den `dependencies` im Manifest hinzu.

### Vault Key nicht verfügbar

```
AURAGO_SECRET_MEIN_KEY ist None
```

**Lösung:**
1. Prüfe dass `tools.python_secret_injection.enabled: true` in `config.yaml` ist
2. Stelle sicher dass der Key im Vault existiert
3. Verifiziere dass es ein user-erstellter Secret ist (nicht System-Secret)

### Timeout bei der Ausführung

**Lösung:**
- Erhöhe das Timeout in der Konfiguration
- Führe lange Operationen im Hintergrund aus
- Optimiere den Code (z.B. Streaming für große Daten)

---

## Zusammenfassung

| Aspekt | Beschreibung |
|--------|--------------|
| **Struktur** | 2 Dateien: `.json` (Manifest) + `.py` (Code) |
| **Speicherort** | `agent_workspace/skills/` |
| **Secrets** | Via `vault_keys` im Manifest deklarieren, als `AURAGO_SECRET_*` nutzen |
| **Dependencies** | Im Manifest angeben, werden automatisch installiert |
| **Parameter** | Werden als JSON via stdin übergeben |
| **Rückgabe** | JSON-String via stdout |

---

**Nächste Schritte**

- **[Kapitel 6: Werkzeuge](06-tools.md)** – Die eingebauten Tools kennenlernen
- **[Kapitel 14: Sicherheit](14-sicherheit.md)** – Sicherheitsrichtlinien für Skills
- **Web-UI** → Skills → Neue Skills entdecken
