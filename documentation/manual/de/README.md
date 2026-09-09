<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="560" alt="AuraGos türkiser Gopher liest ein gezeichnetes Home-Lab-Handbuch an einer Werkbank"></a>
</p>

<h1 align="center">AuraGo Benutzerhandbuch</h1>

<p align="center"><strong>Ein Gopher. Eine lächerliche Werkzeugkiste. Jetzt mit Inhaltsverzeichnis.</strong></p>

Dein selbst gehosteter Agent kann per SSH aufs NAS, über Mesh-Funk antworten, ein Browser-Spiel bauen und sich merken, woran ihr gestern gehangen habt. Dieses Handbuch erklärt, wie du ihn aufsetzt, mit ihm sprichst und die Spielzeuge einschaltest — ohne die Danger Zone aus Versehen weit zu öffnen.

<p align="center">
  <a href="#die-werkzeugkiste">Kapitel</a> · <a href="03-schnellstart.md">Schnellstart</a> · <a href="faq.md">FAQ</a> · <a href="../en/README.md">English</a>
</p>

[![Gezeichnete Feature-Karte: der Gopher verbindet Home-Lab, Gedächtnis, Studios, Funk, Autopilot und virtuelle Arbeitsflächen](../../../assets/readme/gopher-feature-map.webp)](../../../assets/readme/gopher-feature-map.webp)

Die Illustrationen sind Anleitungszeichnungen, keine UI-Screenshots. Klicke ein Bild für die Vollgröße.

## Die Werkzeugkiste

- **Dein Home Lab, auf Sprechbasis.** Docker, Proxmox, TrueNAS, Home Assistant, MQTT, Fritz!Box, AdGuard, Tailscale, SSH und Wake-on-LAN.
- **Ein Desktop mit Nebenquests.** Chat, Dateien, Terminal, Galerie, Kalender, Radio, Winamp-artiger Player. Dazu Code Studio, Homepage Studio und Game Maker.
- **Er erinnert sich.** Verlauf, Kernfakten, lokale Embeddings, Dokument-RAG, Knowledge Graph, Notizen, Journal. Persönlichkeit ändert den Ton, nicht die Rechte.
- **Autopilot.** Missionen, Cron, Webhooks, Co-Agenten. **Invasion Control** schlüpft Worker-„Eggs“ auf Remote-„Nests“. Ja, so heißen die wirklich.
- **Sprache und Funk.** Telegram, Discord, Rocket.Chat, E-Mail, Realtime Speech, SIP, Speech Lab. **MeshCore** für vertrauenswürdige Direktnachrichten und eingeschränkte Kanalantworten.
- **Seltsames Zeug.** 3D-Drucker, go2rtc-Kameras, Bluetooth-Audio, ESP32 Cheap Yellow Display, here.now, lokale Modelle (Qwen, Ling, experimentelles Spark). Skills und MCP, wenn dir das noch nicht reicht.

[Alle Integrationen](08-integrations.md) · [Tool-Katalog](22-interne-tools.md) · [Persönlichkeiten](10-personality.md) · [AgoDesk](https://github.com/antibyte/agodesk)

### Ja, es gibt einen echten Desktop

[![Echter AuraGo Virtual Desktop: App-Launcher, Galerie, Radio und ein Winamp-artiger Player](../../screenshots/desktop.png)](../../screenshots/desktop.png)

*Echter Screenshot. Der Desktop ist experimentell.*

<details>
<summary>Themes (Chat)</summary>

| Cyberwar | Retro CRT | Dark Sun | Lollipop |
|:--------:|:---------:|:--------:|:--------:|
| [![Cyberwar](../../screenshots/theme1.png)](../../screenshots/theme1.png) | [![Retro CRT](../../screenshots/theme2.png)](../../screenshots/theme2.png) | [![Dark Sun](../../screenshots/theme3.png)](../../screenshots/theme3.png) | [![Lollipop](../../screenshots/theme4.png)](../../screenshots/theme4.png) |

Es gibt 13 Chat-Themes, darunter auch Standard und Hell. Virtual Desktop kennt **Fruity** und **Standard**.

</details>

### AgoDesk — derselbe Agent, direkt auf dem Rechner

<p align="center">
  <a href="../../../assets/readme/agodesk.png"><img src="../../../assets/readme/agodesk.png" width="640" alt="AgoDesk-Desktop-Chat verbunden mit AuraGo"></a>
</p>

**[AgoDesk für Windows und Linux](https://github.com/antibyte/agodesk)** bringt Chat, Sprache und Datei-Uploads ohne Browser. Computer- und Browser-Use nur in dem Rahmen, den du freigibst.

## So hängt es zusammen

[![Systemverdrahtung: Kanäle und Trigger erreichen die Agent-Schleife; Modelle, Gedächtnis und Co-Agenten hängen daran; gegatetes Tool-Dispatch erreicht Infrastruktur und Medien](../../../assets/readme/system-wiring.svg)](../../../assets/readme/system-wiring.svg)

Nachrichten und Missionen landen in der **Agent-Schleife**. Sie baut Kontext, ruft dein Modell, führt erlaubte Tools aus und liest die Ergebnisse. **Co-Agenten** liefern Teilaufgaben zurück. Zugangsdaten holen Service-Clients aus dem **Vault**. Die volle Web-UI ist ein **versioniertes lokales Ressourcenpaket**; im Binary bleibt eine kleine Reparatur-/Anmeldeseite.

Persönlichkeit ändert den Ton, nicht die Rechte. Shell, Python, Schreiben, Netz und Remote-Ausführung haben eigene Danger-Zone-Schalter. Guardian prüft zusätzlich. Schalte ein, was du brauchst.

[Gedächtnis](09-memory.md) · [Sicherheit](14-sicherheit.md) · [Web-Assets](../../web-assets.md) · [Architektur](../../architecture.md)

## Schnellstart

> **Immer noch Work in Progress.** Ein Maintainer, ungleichmäßige Tests, manchmal raue Kanten. Linux zuerst; Windows und macOS sind weniger erprobt. Features hängen von Rechten, Providern, Hardware und oft Docker ab.

```bash
curl -fsSL https://raw.githubusercontent.com/antibyte/AuraGo/main/install.sh | bash
```

Der Installer startet AuraGo und druckt URL plus Erstpasswort. Meist **http://localhost:8088**. Passwort ändern, `/setup` zu Ende bringen, ein Modell wählen, dann etwas Harmloses wie **„Zeig mir Systeminformationen.“**

Daten liegen bei deiner Installation. **Gehostete Modelle sehen trotzdem ihre Request-Inhalte.** Self-hosted heißt nicht automatisch offline. Nach außen: Login, HTTPS, optional 2FA. Mehr in [Kapitel 14](14-sicherheit.md).

[Kapitel 2: Installation](02-installation.md) · [Kapitel 3: Schnellstart](03-schnellstart.md) · [Docker-Guide](../../docker_installation.md)

## Handbuch-Karte

### Teil 1 — Ankommen
1. [Einführung](01-einfuehrung.md) — Was das Ding ist, und was nicht
2. [Installation](02-installation.md) — Binary, Docker, Build, Ressourcenpaket
3. [Schnellstart](03-schnellstart.md) — Die ersten fünf Minuten
4. [Web-Oberfläche](04-webui.md) — Chat, Desktop, Config, Themes
5. [Chat-Grundlagen](05-chatgrundlagen.md) — Wie du mit dem Agenten redest

### Teil 2 — Die Spielzeuge
6. [Werkzeuge](06-tools.md) — Der große Werkzeugkasten
7. [Konfiguration](07-konfiguration.md) — Provider-System und Feintuning
8. [Integrationen](08-integrations.md) — Vom NAS bis zum Funkgerät
9. [Gedächtnis](09-memory.md) — Verlauf, Kernfakten, RAG, Graph
10. [Persönlichkeit](10-personality.md) — Dieselbe Toolbox, andere Haltung

### Teil 3 — Autopilot
11. [Mission Control](11-missions.md) — Geplante Arbeit
12. [Invasion Control](12-invasion.md) — Eggs und Nests
13. [Dashboard](13-dashboard.md) — Zahlen, Issues, Affect, 3D-Graph

### Teil 4 — Vorsicht und Tiefe
14. [Sicherheit](14-sicherheit.md) — Vault, Jail, Guardian, 2FA
15. [Co-Agenten](15-coagents.md) — Helfer für Teilaufgaben
16. [Troubleshooting](16-troubleshooting.md) — Wenn nur die Recovery-Seite da ist
17. [Glossar](17-glossar.md) — Begriffe
18. [Anhang](18-anhang.md) — Kurzreferenz
19. [Skills](19-skills.md) — Python-Skills und Agent Skills

### Teil 5 — Nachschlagen
20. [Chat-Commands](20-chat-commands.md)
21. [API-Referenz](21-api-reference.md)
22. [Interne Tools](22-interne-tools.md)
23. [Interna](23-interna.md)

[FAQ](faq.md) · [Handbuch-Start](../README.md)

## Chat-Kurzbefehle

```
/help          Alle Befehle
/reset         Chat-Verlauf löschen
/stop          Laufende Aktion abbrechen
/restart       AuraGo neu starten
/debug on/off  Debug-Modus
/budget        Kosten
/personality   Persönlichkeit wechseln
/voice         Sprachausgabe
/warnings      Systemwarnungen
/sudopwd       Sudo-Passwort in den Vault
/addssh        SSH-Host merken
/credits       OpenRouter-Guthaben
```

Die native Tool-Liste ist groß und feature-gated — nicht jede Installation sieht dieselben 100+ Namen. Maßgeblich sind [Kapitel 22](22-interne-tools.md) und das, was Config gerade erlaubt.

## Wichtige Hinweise

**Web-UI zuerst.** Mission Control und Invasion Control leben in der UI und der REST-API, nicht als extra CLI.

**Workspace-Jail.** Datei-Tools bleiben in `agent_workspace`. `config.yaml` und `data/` sind von dort aus nicht lesbar. Frag nach Systeminfos, nicht nach der Config-Datei.

**Exponieren nur mit Absicht.** AuraGo kann Shell und Dateien anfassen. Nach außen: VPN, Reverse-Proxy oder eingebaute Auth plus 2FA.

Die UI spricht **16 Sprachen**. Pick your company in **Config → Personality**.

[![Flaggen der 16 UI-Sprachen](../../../assets/readme/language-flags.webp)](../../../assets/readme/language-flags.webp)

*Stand: 9. September 2026. Die englische Fassung ist [hier](../en/README.md).*
