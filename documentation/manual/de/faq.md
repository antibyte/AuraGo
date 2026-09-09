# AuraGo FAQ

Zurück zur [Handbuch-Startseite](../README.md) · [Deutsche Übersicht](README.md)

<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="480" alt="AuraGo-Gopher mit Handbuch"></a>
</p>

## Start

### Wie komme ich am schnellsten hin?
Linux: One-Liner in [Kapitel 2](02-installation.md), danach [Schnellstart](03-schnellstart.md). Docker: Repo klonen und `docker compose up -d` — nicht `config.yaml` von GitHub ziehen, die Datei liegt nicht im Repository.

### Brauche ich Docker?
Nein. Der Kern ist eine Binary. Docker ist die sauberere Isolation und bringt Sidecars mit. Siehe [Docker-Guide](../../docker_installation.md).

### Warum sehe ich nur eine Reparatur-/Anmeldeseite?
Die volle UI ist ein **versioniertes Ressourcenpaket**, nicht komplett im Binary. Ungepinntes `go build` oder fehlendes `aurago-web-assets-*.tar.gz` landen in Recovery. `./aurago --check-assets`, bei Bedarf `--install-assets`, sonst Binary mit Asset-Flags neu bauen. [Web-Assets](../../web-assets.md).

### Welche URL?
Default ist **http://127.0.0.1:8088** bzw. **http://localhost:8088**. Docker und LAN binden oft `0.0.0.0:8088`.

## Sicherheit

### Wohin mit API-Keys?
In den Vault. Nicht in Markdown, Git oder unverschlüsselte Exports. [Kapitel 14](14-sicherheit.md).

### Darf das Ding ins Internet?
Nur mit HTTPS, Login und am besten 2FA. VPN ist die gemütlichere Variante.

### Warum kann der Agent `config.yaml` nicht lesen?
Absicht. Datei-Tools sitzen in `agent_workspace`. `../../config.yaml` und `data/` sind gejailt. Frag nach Systeminformationen oder lege eine Datei im Workspace ab.

### Wo liegen die Logs?
`log/aurago.log` und `log/web_access.log`. Es gibt kein `supervisor.log`.

## Spielzeuge

### Wie viele Tools sind es wirklich?
Der Katalog ist groß und **feature-gated**. Handbücher sagen „100+“, weil das die Größenordnung der dokumentierten nativen Tools ist. Deine Instanz zeigt nur, was Config und Integrationen erlauben. [Kapitel 6](06-tools.md) · [Kapitel 22](22-interne-tools.md).

### Telegram, Discord, MeshCore, SIP?
Alles in [Kapitel 8](08-integrations.md). Telegram extra: [telegram_setup.md](../../telegram_setup.md). MeshCore: [meshcore-de.md](../../meshcore-de.md).

### Gibt es eine Desktop-App?
Ja. [AgoDesk](https://github.com/antibyte/agodesk) für Windows und Linux. Das ist nicht die Web-UI.

### Wo bleiben Eggs und Nests?
[Invasion Control](12-invasion.md) — nicht unter Mission Control. Missionen sind geplante Chats; Invasion ist Remote-Deployment.

### Unterstützt AuraGo MCP?
Ja, Client und Server, hinter `agent.allow_mcp`. [Kapitel 8](08-integrations.md).

## Wenn es knirscht

### UI da, Aktionen tot?
Logs, Danger-Zone-Schalter, Provider-Key. Dann [Kapitel 16](16-troubleshooting.md).

### Was ist die Wahrheitsquelle?
Der Code und `config_template.yaml`. Dieses Handbuch folgt ihnen, nicht umgekehrt.
