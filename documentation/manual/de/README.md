# AuraGo Benutzerhandbuch

Willkommen zum AuraGo Benutzerhandbuch – deiner umfassenden Anleitung für den persönlichen KI-Agenten.

## Was ist AuraGo?

AuraGo ist ein vollständig autonomer KI-Agent, der als einzelne portable Binary mit eingebetteter Web-Oberfläche ausgeliefert wird. Verbinde ihn mit einem beliebigen OpenAI-kompatiblen LLM-Provider und er wird zu einem persönlichen Assistenten, der Code ausführen, Dateien verwalten, Smart-Home-Geräte steuern, E-Mails senden, sich alles merken und sogar seinen eigenen Quellcode verbessern kann.

## Für wen ist dieses Handbuch?

| Wenn du... | Starte mit... |
|------------|---------------|
| Neu bei AuraGo bist | [Kapitel 1: Einführung](01-einfuehrung.md) → [Kapitel 2: Installation](02-installation.md) |
| Schnell loslegen willst | [Kapitel 3: Schnellstart](03-schnellstart.md) |
| Die Oberfläche verstehen willst | [Kapitel 4: Web-Oberfläche](04-webui.md) |
| Mehr über Features erfahren willst | [Kapitel 6: Werkzeuge](06-tools.md) |
| Fortgeschrittene Themen suchst | [Kapitel 11-15](11-missions.md) |
| Ein Problem hast | [Kapitel 16: Troubleshooting](16-troubleshooting.md) |

## Handbuch-Struktur

### Teil 1: Grundlagen
1. [Einführung](01-einfuehrung.md) – Was ist AuraGo?
2. [Installation](02-installation.md) – System einrichten
3. [Schnellstart](03-schnellstart.md) – Die ersten 5 Minuten
4. [Web-Oberfläche](04-webui.md) – Navigation & UI
5. [Chat-Grundlagen](05-chatgrundlagen.md) – Kommunikation

### Teil 2: Features im Detail
6. [Werkzeuge](06-tools.md) – 30+ Tools nutzen
7. [Konfiguration](07-konfiguration.md) – Feintuning
8. [Integrationen](08-integrations.md) – Telegram, Discord, etc.
9. [Gedächtnis & Wissen](09-memory.md) – Speicher verstehen
10. [Persönlichkeit](10-personality.md) – Character anpassen

### Teil 3: Fortgeschritten
11. [Mission Control](11-missions.md) – Automatisierung
12. [Invasion Control](12-invasion.md) – Remote Deployment
13. [Dashboard](13-dashboard.md) – Analytics & Metriken

### Teil 4: Für Profis
14. [Sicherheit](14-sicherheit.md) – Vault, Auth, Best Practices
15. [Co-Agenten](15-coagents.md) – Parallele Agenten
16. [Troubleshooting](16-troubleshooting.md) – Problemlösung
17. [Glossar](17-glossar.md) – Begriffe erklärt
18. [Anhang](18-anhang.md) – Referenzmaterial

## Schnell-Navigation

### Die wichtigsten Befehle im Chat
```
/help          - Alle Befehle anzeigen
/reset         - Chat-Verlauf löschen
/stop          - Aktuelle Aktion abbrechen
/debug on/off  - Debug-Modus umschalten
/budget        - Kostenübersicht anzeigen
```

### Schnell-Links
- [Vollständige Konfigurationsreferenz](../configuration.md)
- [Telegram Setup](../telegram_setup.md)
- [Docker Guide](../docker.md)

## Sicherheitshinweis

> ⚠️ **Wichtig:** AuraGo kann beliebige Shell-Befehle ausführen und Systemdateien ändern. Exponiere die Web-UI niemals ungeschützt im Internet. Verwende immer VPN, Reverse Proxy mit Authentifizierung oder Firewall-Regeln.

---

*Dieses Handbuch wird kontinuierlich aktualisiert. Die englische Version findest du [hier](../en/README.md).*
