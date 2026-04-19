# AuraGo Handbuch FAQ (DE)

Zurück zur [Handbuch-Startseite](../README.md) und zur [deutschen Übersicht](README.md).

## Allgemein

### Wie starte ich AuraGo am schnellsten?
Nutze die Installationsschritte aus [Kapitel 2](02-installation.md) und danach den [Schnellstart](03-schnellstart.md).

### Brauche ich zwingend Docker?
Nein. Der Kern läuft als Single-Binary. Docker ist für Isolation und Sidecars (z. B. Gotenberg) empfohlen. Siehe [Docker Installation](../../docker_installation.md).

### Wie viele Tools gibt es aktuell?
Die aktuelle Plattform dokumentiert 90+ integrierte Tools plus integrationsspezifische Funktionen. Siehe [Kapitel 6: Werkzeuge](06-tools.md).

## Sicherheit

### Wo speichere ich API-Keys und Passwörter?
Im verschlüsselten AuraGo Vault. Keine Secrets in Markdown, Commits oder unverschlüsselten Exports speichern. Siehe [Kapitel 14: Sicherheit](14-sicherheit.md).

### Darf AuraGo direkt aus dem Internet erreichbar sein?
Ja, aber nur mit HTTPS, Login-Schutz und idealerweise 2FA. Siehe [Kapitel 14: Sicherheit](14-sicherheit.md) und [Installation](02-installation.md).

## Integrationen & Features

### Wo konfiguriere ich Telegram und Discord?
In [Kapitel 8: Integrationen](08-integrations.md) sowie im separaten [Telegram Setup Guide](../../telegram_setup.md).

### Gibt es verteilte Orchestrierung?
Ja. Invasion Control und Remote Control sind in [Kapitel 12](12-invasion.md) und [Kapitel 15](15-coagents.md) beschrieben.

### Unterstützt AuraGo MCP?
Ja, sowohl als Client als auch als MCP-Server. Siehe [Kapitel 8](08-integrations.md) und [Kapitel 7](07-konfiguration.md).

## Fehlerbehebung

### Die UI lädt, aber Aktionen schlagen fehl – was zuerst prüfen?
Logs, Danger-Zone-Flags und Provider-Credentials prüfen. Starte mit [Kapitel 16: Troubleshooting](16-troubleshooting.md).

### Ein Abschnitt wirkt veraltet – was ist maßgeblich?
Codebasis und Konfigurationsschema sind die Quelle der Wahrheit. Von dort aus Dokumentation aktualisieren, siehe [Kapitel 7](07-konfiguration.md).
