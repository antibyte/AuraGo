# Kapitel 1: Einführung

<p align="center">
  <a href="../images/manual-hero.webp"><img src="../images/manual-hero.webp" width="560" alt="AuraGo-Gopher mit einem gezeichneten Handbuch an der Home-Lab-Werkbank"></a>
</p>

AuraGo ist kein Chatfenster mit Plugins. Es ist ein selbst gehosteter Agent in Go, der auf deinem Rechner wohnt, Tools ausführt und sich merkt, was ihr schon besprochen habt.

Die ausführbare Datei ist portabel. Die **volle Web-UI** liegt daneben als passendes lokales Ressourcenpaket. Im Binary bleibt eine kleine Reparatur-/Anmeldeseite. Ein nacktes `go build` ohne Asset-Flags ist Absicht nur Recovery — nicht die ganze Anwendung. Details: [Web-Assets](../../web-assets.md).

Verbinde ein OpenAI-kompatibles Modell (hosted oder lokal). Schalte die Integrationen ein, die du wirklich willst. Der Rest bleibt aus.

## Was er tun kann

- **Denken und nachfassen** — mehrere Tool-Runden, Fehler lesen, nochmal versuchen
- **Code ausführen** — Python und Shell, wenn du die Danger Zone öffnest
- **Dateien im Workspace** — lesen, schreiben, suchen. Nicht in `config.yaml` oder `data/`
- **Home Lab anfassen** — Docker, Proxmox, SSH, Home Assistant, Kameras, Drucker
- **Reden** — Web-Chat, Telegram, Discord, E-Mail, SIP, Speech Lab, MeshCore
- **Sich erinnern** — Verlauf, Kernfakten, RAG, Knowledge Graph, Notizen
- **Autopilot** — Missionen, Co-Agenten, Eggs auf Nests
- **Dinge bauen** — Dokumente, Bilder, Musik, Websites, Offline-Spiele im Game Maker

Er verbessert nicht „seinen eigenen Quellcode“, weil er nett ist. Self-Update und Workspace-Schreiben sind getrennte, abschaltbare Fähigkeiten.

## Für wen

Heimlabor, NAS-Keller, jemand der einen Agenten neben Docker und Home Assistant stellen will. Entwickler, die Automatisierung und Reviews an eine Maschine mit Rechten delegieren. Weniger: ein gehostetes SaaS-Produkt oder ein Forschungs-Framework.

Die UI spricht 16 Sprachen. Persönlichkeiten ändern die Haltung, nicht die Berechtigungen.

<p align="center">
  <a href="../../../assets/readme/persona-party.webp"><img src="../../../assets/readme/persona-party.webp" width="640" alt="Zehn AuraGo-Persönlichkeiten als gezeichnete Gruppe"></a>
</p>

[Persönlichkeiten](10-personality.md) · [AgoDesk](https://github.com/antibyte/agodesk) wenn du den Browser überspringen willst.

## Die grobe Verdrahtung

[![Systemverdrahtung von Kanälen über die Agent-Schleife zu Tools und Vault](../../../assets/readme/system-wiring.svg)](../../../assets/readme/system-wiring.svg)

```
Kanäle / Missionen
        │
        ▼
   Agent-Schleife  ──  Modell (Provider oder lokal)
        │
        ├── Gedächtnis (STM, Core, RAG, Graph)
        ├── Co-Agenten
        └── Tools (nur was Config erlaubt)
                │
                └── Vault für Secrets, nie als Prompt-Futter
```

Persönlichkeit sitzt auf dem Ton. Guardian und Danger Zone sitzen auf den Werkzeugen.

## Sicherheit, bevor du startest

> AuraGo führt Code auf **deinem** System aus. Eine VM, Docker oder ein eigener Rechner sind die vernünftige Default-Wahl. Ein falsch verstandener Prompt plus offene Shell ist kein theoretisches Risiko.

> Die Web-UI nicht nackt ins Internet hängen. VPN (Tailscale, WireGuard), Reverse-Proxy mit Auth, oder die eingebaute Anmeldung plus 2FA.

Datei-Tools bleiben in `agent_workspace`. `../../config.yaml` und `data/` scheitern absichtlich. Mehr: [Kapitel 14](14-sicherheit.md).

## Nächste Schritte

1. **[Installation](02-installation.md)** — Binary, Docker oder Source, plus Ressourcenpaket
2. **[Schnellstart](03-schnellstart.md)** — Setup, erster Chat, harmlose Befehle
3. **[Web-Oberfläche](04-webui.md)** — Chat, Desktop, Config

> Tipp: Fang mit der Web-UI an. Frag nach Systeminformationen, nicht nach der Config-Datei.
