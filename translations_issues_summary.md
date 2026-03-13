# Übersetzungsprüfung - Zusammenfassung

**Datum:** 2026-03-13  
**Referenz:** Deutsch (de) + Englisch (en)  
**Geprüfte Sprachen:** 14 (cs, da, el, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh)

---

## Ergebnisübersicht

| Sprache | Dateien | Null/Leer | Falsches Script | Englisch | Unübersetzt | Status |
|---------|---------|-----------|-----------------|----------|-------------|--------|
| cs | 31 | 0 | 0 | 23 | 34 | ⚠️ Mittel |
| da | 31 | 0 | 0 | 46 | 20 | ⚠️ Mittel |
| de | 31 | 0 | 0 | 0 | 0 | ✅ OK |
| el | 31 | 0 | **801** | 1 | 0 | 🔴 Kritisch |
| en | 31 | 0 | 0 | 0 | 0 | ✅ OK |
| es | 31 | 0 | 0 | 38 | 3 | ⚠️ Mittel |
| fr | 31 | 0 | 0 | 21 | 3 | ⚠️ Mittel |
| hi | 31 | 0 | **136** | 0 | 0 | 🔴 Kritisch |
| it | 31 | 0 | 0 | 72 | 3 | ⚠️ Mittel |
| ja | 31 | 0 | **150** | 0 | 0 | 🔴 Kritisch |
| nl | 31 | 0 | 0 | 44 | 32 | ⚠️ Mittel |
| no | 31 | 0 | 0 | 52 | 22 | ⚠️ Mittel |
| pl | 31 | 0 | 0 | 12 | 3 | ⚠️ Mittel |
| pt | 31 | 0 | 0 | 25 | 42 | ⚠️ Mittel |
| sv | 31 | 0 | 0 | 29 | 20 | ⚠️ Mittel |
| zh | 31 | 0 | **257** | 0 | 0 | 🔴 Kritisch |

**Gesamtprobleme:** 1.889

---

## Kritische Probleme (Falsches Schriftsystem)

### Griechisch (el) - 801 Probleme
Fast alle el-Dateien enthalten Texte mit falschem Schriftsystem (lateinische/tschechische Wörter statt Griechisch).

**Betroffene Dateien:**
- `chat/el.json`
- `common/el.json`
- `config/sections/el.json`
- `dashboard/el.json`
- `help/el.json`
- `invasion/el.json`
- `missions/el.json`
- `setup/el.json`

**Beispiele:**
- `"chat.personality_loading": "Nacitani..."` (tschechisch statt Griechisch)
- `"chat.sse_thinking": "Premysli..."` (tschechisch statt Griechisch)

### Hindi (hi) - 136 Probleme
Fast alle hi-Dateien enthalten tschechische Wörter statt Hindi (Devanagari).

**Beispiele:**
- `"chat.agent_active": "Agent je aktivni"` (tschechisch)
- `"chat.attachment_clear_title": "Vymazat prilohu"` (tschechisch)
- Einige korrekte Hindi-Texte sind gemischt mit falschen

### Japanisch (ja) - 150 Probleme
Gemischte Inhalte - einige Texte sind auf Englisch/Latein statt Japanisch.

**Beispiele:**
- `"chat.credits_pill_title": "OpenRouter クレジット"` (gemischt)
- `"chat.budget_tooltip_template": "予算: {{cost}} / {{limit}}"` (teilweise lateinisch)

### Chinesisch (zh) - 257 Probleme
Gemischte Inhalte - einige Texte sind auf Englisch/Latein statt Chinesisch.

**Beispiele:**
- `"chat.token_counter_format": "{{count}} 令牌"` (gemischt)
- `"chat.sse_thinking": "AuraGo 正在思考..."` (Englischer Name + Chinesisch)

---

## Mittlere Probleme (Englische Texte)

Die folgenden Sprachen enthalten einige englische Texte statt Übersetzungen:

| Sprache | Anzahl | Typische Funde |
|---------|--------|----------------|
| it | 72 | `chat.confirm_clear_msg`, `config/sections` |
| no | 52 | `config/adguard` Hilfetexte |
| da | 46 | `login.subtitle` |
| nl | 44 | `config/adguard` Hilfetexte |
| es | 38 | Verschiedene Chat-Texte |
| sv | 29 | `login.subtitle` |
| pt | 25 | `config/https.behind_proxy_notice` |
| fr | 21 | Verschiedene |
| cs | 23 | Verschiedene |
| pl | 12 | Verschiedene |

---

## Geringe Probleme (Unübersetzt = identisch mit EN)

Texte, die identisch mit der englischen Referenz sind:

| Sprache | Anzahl |
|---------|--------|
| pt | 42 |
| cs | 34 |
| nl | 32 |
| no | 22 |
| sv | 20 |
| da | 20 |
| es, fr, it, pl | 3 je |

---

## Empfohlene Prioritäten

1. **Prio 1 (Kritisch):** Griechisch und Hindi komplett neu übersetzen
2. **Prio 2 (Hoch):** Japanisch und Chinesisch bereinigen (gemischte Inhalte)
3. **Prio 3 (Mittel):** Englische Texte in lateinischen Sprachen korrigieren
4. **Prio 4 (Niedrig):** Unübersetzte Texte vervollständigen

---

## Referenz

- **Deutsch (de):** ✅ Vollständig korrekt
- **Englisch (en):** ✅ Vollständig korrekt

Alle anderen Sprachen benötigen Überarbeitung.
