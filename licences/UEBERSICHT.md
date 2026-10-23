# Externe Projekte und Lizenzbedingungen

## Phonon-Sprachausgabe

Gradium Phonon wird als getrenntes lokales Sprachpaket mitgeliefert. Die Laufzeit
`ptts` 0.4.0 steht unter MIT/Apache-2.0, der Katalog `gradium/phonon-voices` unter
Apache-2.0. Die unveränderten Gewichte von `gradium/phonon-40m-multilingual`
stehen unter **CC BY-NC 4.0 und sind nur für nichtkommerzielle Nutzung freigegeben**.
Eine kommerzielle Nutzung benötigt eine entsprechende Erlaubnis von Gradium.
Quellen, feste Revisionen und Attribution stehen in `documentation/phonon.md`;
das Paket enthält diese Hinweise sowie `texts/CC-BY-NC-4.0.txt`,
`texts/Apache-2.0.txt` und den originalen Laufzeittext `texts/Phonon-MIT.txt`.
AuraGos eigene MIT-Lizenz bleibt bestehen.

Der native Codegraph verwendet zusätzlich `github.com/odvcencio/gotreesitter`
`v0.56.0` (MIT, Copyright (c) 2026 Oscar Villavicencio). Eingebunden sind nur die
Go-, JavaScript-, TypeScript-/TSX- und Python-Grammatiken (MIT; Max Brunsfeld,
2014 für Go/JavaScript, 2017 für TypeScript/TSX, 2016 für Python). Die genauen
Revisionen und Urheberzeilen stehen in `THIRD_PARTY_NOTICES.md`, der Lizenztext
in `licences/texts/MIT.txt`.

Stand: 14. Oktober 2026. AuraGo selbst steht unter der MIT-Lizenz in `LICENSE`.
Diese Datei listet fremde Projekte, die der Quellbaum direkt einbindet, auf dem
virtuellen Desktop mitliefert oder als Docker-Image startet.

Die zugehörigen Lizenztexte, die bei einer Weitergabe mitmüssen, liegen in
`licences/texts/`. Urheberzeilen der JavaScript-Bundles stehen vollständig neben
dem Code in `ui/js/vendor/**/LICENSES.md`. Die englische Bestandsliste
`THIRD_PARTY_NOTICES.md` und `THIRD_PARTY_LIBRARIES.md` bleibt für einzelne
Laufzeiten und Go-Module die ausführlichere Herkunftsangabe.

Floating Tags wie `:latest` beschreiben das Upstream-Projekt, nicht einen
eingefrorenen Digest. Vor einer Veröffentlichung den konkreten Image-Digest
gegen diese Liste prüfen.

## Was die üblichen Lizenzen verlangen

| Lizenz | Bedingung für die Weitergabe |
| --- | --- |
| MIT, BSD-2-Clause, BSD-3-Clause, ISC | Nutzen und Ändern ist erlaubt. Urheberzeile und Lizenztext bleiben am Werk. BSD-3 verbietet zusätzlich Werbung mit dem Namen der Urheber. |
| Apache-2.0 | Wie eine freizügige Lizenz, plus Beibehaltung von NOTICE-Dateien und Kennzeichnung eigener Änderungen. |
| MPL-2.0 | Geänderte Dateien unter dieser Lizenz bleiben unter der MPL. Der übrige eigene Code kann anders lizenziert sein. |
| EPL-2.0 | Entsprechend für EPL-Dateien. Paho erlaubt daneben die Eclipse Distribution License, das ist BSD-3-Clause. |
| GPL-2.0, GPL-3.0 | Das Programm und abgeleitete Werke nur unter derselben GPL, mit Lizenztext und Quellcode dieser Version. |
| AGPL-3.0 | Wie die GPL, und wer das Programm über ein Netz anbietet, stellt den Quellcode dieser Programmversion bereit. |
| OFL-1.1 | Schriften dürfen eingebettet werden. Der Schriftname bleibt reserviert, geänderte Schnitte brauchen einen anderen Namen. |
| CC BY-SA 4.0 | Namensnennung und Weitergabe bearbeiteter Fassungen unter derselben Lizenz. |
| Unlicense | Public Domain, soweit das örtliche Recht das trägt. |
| PostgreSQL | BSD-ähnliche Lizenz der PostgreSQL-Entwickler, Text in `texts/PostgreSQL.txt`. |

Das ist eine Lesehilfe für die mitgelieferten Texte, keine Rechtsberatung.

## Sonderfälle

Diese Einträge binden enger als MIT.

| Projekt | Lizenz | Was gilt |
| --- | --- | --- |
| Stockfish 19.0.1 | GPL-3.0 | Schach-Engine im Desktop, `ui/js/vendor/stockfish/`. Text: `texts/GPL-3.0-only.txt`. |
| Papirus, WhiteSur | GPL-3.0 | Icon-Themes des virtuellen Desktops. Texte liegen auch in `ui/img/papirus/` und `ui/img/whitesur/`. |
| Layerling 1.71.0 | AGPL-3.0-only | Optionale CAD-App. Lizenz und Quellarchiv (`source.zip`) liegen in `ui/js/vendor/layerling/`. |
| NASSCAD v4.7.0-desktop | CC BY-NC 4.0 mit zusätzlichen Upstream-Erlaubnissen | NassLab-Code und fremde Komponenten haben getrennte Bedingungen. `LICENSE`, `THIRD-PARTY.md` und die IFC-Lizenz werden mit den CAD-Ressourcen ausgeliefert. |
| ByteStash, IT-Tools, AdGuard Home | GPL-3.0 | Store-Images, unverändert aus der Registry. |
| OliveTin, RomM, MinIO, Garage | AGPL-3.0 | Store- oder Sidecar-Images. Änderungen an diesen Programmen bleiben AGPL, und Nutzer des laufenden Dienstes bekommen dessen Quellcode. |
| SDRangel 7.27.2 | GPL-3.0 | Im RTL-SDR-Image zusammengesetzt. Hinweise bleiben im Image. |
| welle.io 2.7, rtl-sdr-blog 1.4.0 | GPL-2.0 | Ebenfalls im RTL-SDR-Image. Text: `texts/GPL-2.0-only.txt`. |
| OpenSCAD | GPL-2.0 plus CGAL-Ausnahme | Store- und Desktop-Laufzeit, Digest in `internal/desktopstore/catalog.go`. Die Ausnahme steht in `texts/OpenSCAD-CGAL-exception.txt`. |
| QuakeJS / ioquake3 | GPL-2.0 für die Engine | Spieldaten sind Werke von id Software und keine Open-Source-Lizenz. |
| noVNC, Diago, go-sql-driver/mysql, chromem-go | MPL-2.0 | Dateiweise Copyleft. Text: `texts/MPL-2.0.txt`. |
| Eclipse Paho MQTT | EPL-2.0 OR BSD-3-Clause | MQTT-Client. Text: `texts/EPL-2.0.txt`. |
| elkjs | EPL-2.0 | Transitiv über Mermaid im Browser-Bundle. |
| espeak-ng | GPL-3.0 | Wird für sanoTTS getrennt installiert, nicht im Wheel. |
| Px437 IBM VGA | CC BY-SA 4.0 | Terminal-Schrift, unverändert. Text: `texts/CC-BY-SA-4.0.txt`. |
| C64 Pro Mono | Style-Lizenz, kein Open Source | Nur in Software, die Endnutzer kostenlos bekommen. Datei nicht umbenennen, nicht ändern, nicht als eigenen Download anbieten. Text: `texts/C64-Pro-Mono.txt`. |
| CanvasUI | MIT plus Commons Clause | Darf in einer Anwendung stecken. Der Verkauf der Komponenten allein, im Bundle oder als Port ist ausgeschlossen. Text: `texts/CanvasUI-Commons-Clause.txt`. |
| n8n | Sustainable Use License | Eigene interne oder private Nutzung. Weitergabe nur kostenlos und nicht kommerziell. Enterprise-Dateien (`.ee.`) sind ausgenommen. Text: `texts/n8n-Sustainable-Use.md`. |
| Open WebUI | eigene Lizenz | BSD-Gerüst plus Markenbindung: „Open WebUI“-Branding darf nur weg, wenn höchstens 50 Personen in 30 Tagen Zugriff haben, oder mit schriftlicher Erlaubnis. Text: `texts/Open-WebUI.txt`. |
| Redis 7 | RSALv2 OR SSPL-1.0 | Keine OSI-Open-Source-Lizenz. Dograh zieht `redis:7`. Texte: `texts/Redis-7.4.txt` und `texts/SSPL-1.0.txt`. |
| command-code 1.74.1 | UNLICENSED | npm-Metadaten räumen keine Lizenz ein. Das CommandCode-Image installiert dieses Paket. |
| Datatype | nicht belegt | Schrift in `ui/fonts/`, ohne Lizenzdatei im Baum. |
| Manifest | nicht belegt | Image `manifestdotbuild/manifest:5`. Öffentliches Lizenzfile war nicht abrufbar. |
| Wikipedia-Text | CC BY-SA 4.0 | Erst nach Admin-Installation eines ZIM, nicht im Quellbaum. Medien haben eigene Lizenzen. |
| Emularity | GPL-3.0 | Retro Games lädt den Loader zur Laufzeit von archive.org. AuraGo liefert ihn nicht mit. Spielstände und ROMs bleiben bei den Rechteinhabern. |

## Virtueller Desktop

Eingebaute Apps, die ein fremdes Programm oder eine fremde Bibliothek ausführen.
AuraGo-eigene Oberflächen ohne Fremdcode stehen hier nicht.

| Fläche | Fremdprojekt | Lizenz |
| --- | --- | --- |
| Schach | chess.js 1.4.0, cm-chessboard 8.15.3, Stockfish 19.0.1 | BSD-2-Clause, MIT, GPL-3.0 |
| Terminal, Code Studio, Retro-Net | xterm.js 6, Fit- und WebGL-Addon | MIT |
| Webamp | webamp 2.3.1 | MIT |
| PDF-Anzeige | PDF.js 6.4.299, zuzüglich OpenJPEG, qcms, JBIG2 und der Standardschriften laut `ui/js/vendor/pdf/` | Apache-2.0 und die dort liegenden Schriftlizenzen |
| VNC in Quick Connect und virtuellen Rechnern | noVNC 1.7.0 | MPL-2.0 |
| Autor | `@docx-editor.dev` 2.29.0 | Apache-2.0; Schriften zusätzlich OFL-1.1 und GUST. Volltexte in `ui/js/vendor/writer/LICENSES.md` |
| Tabellen | Univer OSS 1.0.4 | Apache-2.0 |
| Notizen | Milkdown Crepe/Kit 7.22.3, ProseMirror | MIT |
| Code Studio, OpenSCAD, Cheater | CodeMirror 6 | MIT |
| Cheater, Viewer, Notizen, Autor, Code Studio | marked 18, markdown-it 15, DOMPurify 3.4.17, highlight.js 11.12 | MIT, MIT, Apache-2.0 OR MPL-2.0, BSD-3-Clause |
| System World, Tiefsee, Galaxy, Wissensgraph | three.js 0.186.1; RuView zusätzlich three 0.160.0 | MIT |
| Wissensgraph | 3d-force-graph 1.80.1, force-graph 1.52.0 | MIT |
| Game Maker | Phaser 4.2.1 | MIT |
| Game Maker 3D | Google Draco | Apache-2.0 |
| Diagramme | Chart.js 4.5.1, Mermaid 12 | MIT |
| Personas | Rive Canvas 2.43.1 | MIT. Die `.riv`-Dateien sind eigene AuraGo-Assets. |
| Tresor | argon2id 1.0.1 | MIT |
| Live Speech | ONNX Runtime Web 1.30.0, Silero VAD 6.2.1 | MIT |
| TeeVee | hls.js 1.7.3, Bootstrap Icons | Apache-2.0, MIT (`ui/img/teevee/ICONS-LICENSE.txt`) |
| CYD-Flasher | esp-web-tools 10.4.0, esptool-js | Apache-2.0 |
| Synth Studio | `@tonejs/midi` 2.0.28, Tabellen aus webaudio-tinysynth 1.1.4 | MIT, Apache-2.0 |
| RuView | ruview, Copyright 2024 rUv | MIT |
| Login und City-Rain | CanvasUI | MIT plus Commons Clause |
| Galaxy-Bedienelemente | Lucide | ISC |
| Desktop-Themes | Papirus, WhiteSur | GPL-3.0 |
| OpenSCAD | openscad, gepinnter Digest | GPL-2.0 plus CGAL-Linking-Ausnahme |
| Ziro Designer | `ghcr.io/antibyte/aurago-ziro-designer:ziro-7b45970-1` | GPL-3.0-or-later; KiCad-Bibliotheken CC-BY-SA-4.0 mit KiCad-Ausnahme; Quellen und Hinweise im Image |
| God's Eye View | Upstream von Bilawal Sidhu, Image `gev-7596522-1` | MIT, `ui/img/desktop/store/gods-eye-view.LICENSE` |
| Retro Games | Emularity von archive.org | GPL-3.0, nicht mitgeliefert |
| Quake | QuakeJS-Rootless-Image | GPL-2.0-Engine, proprietäre Spieldaten |
| Writer-Hinweis | Humanizer 3.0, Copyright 2025 Siqi Chen | MIT, Text in `THIRD_PARTY_NOTICES.md` |

Weitere mitgebaute npm-Pakete der Oberfläche: `esbuild` 0.28.2 (MIT),
`headroom-ai` 0.39.1 (Apache-2.0), `stats.js` 0.17.0 (MIT), `rollup` (MIT, nur Build).
Transitive Pakete dieser Bundles, darunter `elkjs` (EPL-2.0), `pako` (MIT AND Zlib)
und `robust-predicates` (Unlicense), stehen mit Urheberzeile in
`ui/js/vendor/LICENSES.md`, `notes/LICENSES.md`, `writer/LICENSES.md` und
`sheets/LICENSES.md`.

## Docker: Store, Sidecars, Basis-Images

### Store-Katalog

Quelle: `internal/desktopstore/catalog.go`.

| App | Image | Lizenz |
| --- | --- | --- |
| God's Eye View | `ghcr.io/antibyte/aurago-gods-eye-view:gev-7596522-1` | MIT |
| Homarr | `ghcr.io/homarr-labs/homarr:latest` | Apache-2.0 |
| n8n | `ghcr.io/n8n-io/n8n:latest` | Sustainable Use License |
| Node-RED | `ghcr.io/node-red/node-red:latest` | Apache-2.0 |
| Open WebUI | `ghcr.io/open-webui/open-webui:main` | Open WebUI License |
| ByteStash | `ghcr.io/jordan-dalby/bytestash:latest` | GPL-3.0 |
| IT-Tools | `ghcr.io/corentinth/it-tools:latest` | GPL-3.0 |
| FileBrowser Quantum | `ghcr.io/gtsteffaniak/filebrowser:stable` | Apache-2.0 |
| OliveTin | `ghcr.io/olivetin/olivetin:latest` | AGPL-3.0 |
| AdGuard Home | `adguard/adguardhome` | GPL-3.0 |
| Excalidraw | `excalidraw/excalidraw:latest` | MIT |
| Uptime Kuma | `ghcr.io/louislam/uptime-kuma:2` | MIT |
| Stirling PDF | `ghcr.io/stirling-tools/stirling-pdf:latest` | MIT für den Community-Code. `app/proprietary/` und `app/saas/` haben eigene Lizenzen. |
| QuakeJS | `docker.io/awakenedpower/quakejs-rootless:latest` | GPL-2.0-Engine, Spieldaten gesondert |
| RomM | `ghcr.io/rommapp/romm:latest` | AGPL-3.0 |
| RomM MariaDB | `ghcr.io/linuxserver/mariadb:latest` | GPL-2.0 |
| Beszel und Agent | `ghcr.io/henrygd/beszel/beszel:latest`, `beszel-agent` | MIT |
| Dozzle | `ghcr.io/amir20/dozzle:latest` | MIT |
| Arcane | `ghcr.io/getarcaneapp/manager:latest` | BSD-3-Clause |
| Socket-Proxy von Beszel, Dozzle, Arcane | `tecnativa/docker-socket-proxy:latest` | Image führt HAProxy (GPL-2.0). Die Proxy-Skripte sind Apache-2.0. |
| code-server | `ghcr.io/linuxserver/code-server:latest` | Die App ist MIT. Das linuxserver-Image packt sie mit Distributionspaketen. |
| Termix | `ghcr.io/lukegus/termix:latest` | Apache-2.0 |
| Termix guacd | `guacamole/guacd:1.6.0` | Apache-2.0 |
| CommandCode | `ghcr.io/antibyte/aurago-commandcode:latest` | Eigenes Image. Enthält `command-code@1.74.1`, npm-Lizenz `UNLICENSED`. |
| OpenSCAD | `openscad/openscad@sha256:147e48525bec392bcf628d7a6d5ea4ccac71b16251952328f86e1061cbf47c37` | GPL-2.0 plus CGAL-Linking-Ausnahme |

### Von AuraGo gesteuerte Produkte

| Produkt | Referenz | Lizenz |
| --- | --- | --- |
| Gotenberg 8 | `gotenberg/gotenberg:8` | MIT |
| Docker Socket Proxy | `tecnativa/docker-socket-proxy:v0.5.0` mit Digest in `docker-compose.yml` | HAProxy GPL-2.0 im Image |
| go2rtc 1.9.14 | `alexxit/go2rtc`, Digest in der Config | MIT |
| yt-dlp | `ghcr.io/jauderho/yt-dlp`, Digest in der Config | Unlicense |
| Wyoming Piper | `rhasspy/wyoming-piper:latest` | MIT, Copyright 2023 Michael Hansen |
| Supertonic 1.3.1 | PyPI-Paket im Image `aurago-supertonic` | MIT im Upstream-Repository `supertone-inc/supertonic`. Das PyPI-Feld `license` ist leer. |
| Code Studio | `ghcr.io/antibyte/aurago-code-studio:latest` | Eigenes Image auf Node 22 |
| PostgreSQL 16 | `postgres:16-alpine` für SQL-Verbindungen | PostgreSQL License |
| pgvector | `pgvector/pgvector:pg17` für Dograh | PostgreSQL License |
| MySQL 8.0 | `mysql:8.0` | GPL-2.0 |
| MariaDB 11 | `mariadb:11` | GPL-2.0 |
| Redis 7 | `redis:7` für Dograh | RSALv2 OR SSPL-1.0 |
| MinIO | `minio/minio:latest` für Dograh | AGPL-3.0 |
| coturn 4.8.0 | `coturn/coturn:4.8.0` | BSD-3-Clause, Copyright 2011–2013 Citrix Systems |
| Garage 2.3.0 | `dxflrs/garage:v2.3.0` mit Digest | AGPL-3.0 |
| OmniRoute 3.8.39 | `diegosouzapw/omniroute:3.8.39` | MIT |
| Dograh API und UI | `ghcr.io/dograh-hq/dograh-api`, `dograh-ui` | BSD-2-Clause |
| Manifest 5 | `manifestdotbuild/manifest:5` | Lizenz hier nicht belegt |
| Ansible 14.4.0 | Sidecar `Dockerfile.ansible` | GPL-3.0-or-later |
| ACE-Step | Laufzeit-Image | Quellcode Apache-2.0. Modellgewichte folgen ihrer Model Card und liegen nicht im Quellbaum. |
| sanoTTS 0.6.0 | Wheel in `internal/sanotts/runtime/` | MIT, Copyright Ampixa. espeak-ng ist GPL-3.0 und wird getrennt installiert. |
| RTL-SDR-Image | SDRangel, welle.io, rtl-sdr-blog | GPL-3.0, GPL-2.0, GPL-2.0 |
| Caddy | Security-Proxy | Apache-2.0 |
| cloudflared 2026.10.0 | Optionaler Tunnel | Apache-2.0 |
| llama.cpp | Embedding- und lokale Modell-Sidecars | MIT |
| ONNX Runtime 1.26.0 | Native Embeddings | MIT |
| Granite Embedding 97M R2 | Modell, bei Bedarf geladen | Apache-2.0 |
| Playwright 1.64.1 | Browser-Sidecar, Image `mcr.microsoft.com/playwright:v1.64.1-noble` | Apache-2.0. Das enthaltene Chromium steht unter der BSD-Lizenz des Chromium-Projekts. |
| CloakBrowser 0.5.11 | Browser-Sidecar | MIT |
| gog-Buildhilfe | `internal/gog/build/LICENSE.gog` | MIT, Copyright 2026 Peter Steinberger |

Basis-Images (`golang`, `python`, `node`, `debian`, `ubuntu`, `alpine`) sind
Distributionen. Ihre Pakete behalten jeweils die eigene Lizenz. Node ist MIT,
CPython steht unter der PSF-Lizenz, Alpine bringt musl (MIT) und BusyBox (GPL-2.0) mit.

## Schriften und Icons

| Familie | Lizenz |
| --- | --- |
| Geist, Geist Mono | OFL-1.1 |
| Inter, Darker Grotesque, Exo 2, Oxanium, Special Elite, Schoolbell, Shadows Into Light Two, Press Start 2P | OFL-1.1 |
| Roboto Condensed, Barlow Condensed, Share Tech Mono | OFL-1.1, Texte in `ui/fonts/` |
| Px437 IBM VGA 8x16 | CC BY-SA 4.0 |
| C64 Pro Mono | Style-Lizenz |
| Datatype | nicht belegt |
| Papirus, WhiteSur | GPL-3.0 |
| Lucide | ISC |
| Bootstrap Icons in TeeVee | MIT |

OFL-Text: `texts/OFL-1.1.txt`.

## Go-Direktabhängigkeiten

Aus dem ersten `require`-Block in `go.mod`. Transitive Module, vor allem über
Tailscale, stehen in `go.sum` und sind hier nicht Zeile für Zeile wiederholt.
`modernc.org/sqlite` ist BSD-3-Clause und enthält SQLite als Public Domain.

### Apache-2.0

`github.com/a2aproject/a2a-go/v2` v2.6.0,
`github.com/danielthedm/promptsec` v0.1.0,
`github.com/ebitengine/purego` v0.11.1,
`github.com/gocolly/colly/v2` v2.3.0,
`github.com/hajimehoshi/go-mp3` v0.3.4,
`github.com/minio/minio-go/v7` v7.3.0,
`github.com/pdfcpu/pdfcpu` v0.15.0,
`github.com/sashabaranov/go-openai` v1.42.1,
`github.com/vishen/go-chromecast` v0.3.4,
`github.com/xeipuuv/gojsonschema` v1.2.0,
`google.golang.org/grpc` v1.83.2.

### MIT

`codeberg.org/readeck/go-readability/v2` v2.1.2,
`github.com/BurntSushi/toml` v1.6.0,
`github.com/JohannesKaufmann/html-to-markdown/v2` v2.5.2,
`github.com/SherClockHolmes/webpush-go` v1.4.0,
`github.com/chromedp/cdproto`,
`github.com/chromedp/chromedp` v0.16.0,
`github.com/creack/pty` v1.1.24,
`github.com/dlclark/regexp2` v1.12.0,
`github.com/evanw/esbuild` v0.28.2,
`github.com/go-ldap/ldap/v3` v3.4.14,
`github.com/go-rod/rod` v0.116.2,
`github.com/go-telegram-bot-api/telegram-bot-api/v5` v5.5.1,
`github.com/itlightning/dateparse` v0.2.1,
`github.com/johnfercher/maroto/v2` v2.4.2,
`github.com/lib/pq` v1.12.3,
`github.com/mdlayher/vsock` v1.3.0,
`github.com/phpdave11/gofpdf` v1.4.3,
`github.com/pion/ice/v4` v4.3.0,
`github.com/pion/interceptor` v0.1.49,
`github.com/pion/logging` v0.2.4,
`github.com/pion/webrtc/v4` v4.2.17,
`github.com/pkoukk/tiktoken-go` v0.1.8,
`github.com/prometheus-community/pro-bing` v0.9.1,
`github.com/robfig/cron/v3` v3.0.1,
`github.com/tailscale/go-winio`,
`github.com/tidwall/gjson` v1.19.0,
`github.com/tidwall/sjson` v1.2.5,
`github.com/tiendc/go-deepcopy` v1.7.2,
`github.com/ysmood/gson` v0.7.3.

### MIT AND Apache-2.0

`gopkg.in/yaml.v3` v3.0.1. Die aus libyaml portierten Dateien bleiben MIT,
Copyright Kirill Simonov. Der übrige Code ist Apache-2.0.

### BSD-2-Clause

`github.com/beevik/etree` v1.8.1,
`github.com/emiago/sipgo` v1.6.0,
`github.com/godbus/dbus/v5` v5.2.2,
`github.com/gorilla/websocket`,
`github.com/huin/goupnp` v1.3.0,
`github.com/pkg/sftp` v1.13.11.

### BSD-3-Clause

`github.com/PuerkitoBio/goquery` v1.13.0,
`github.com/blevesearch/snowballstem` v0.9.0,
`github.com/bwmarrin/discordgo` v0.29.0,
`github.com/gofrs/flock` v0.13.1,
`github.com/google/uuid` v1.6.0,
`github.com/klauspost/compress` v1.20.1 (Copyright 2012 The Go Authors und 2019 Klaus Post; `zstd/internal/xxhash` ist MIT),
`github.com/ledongthuc/pdf`,
`github.com/miekg/dns` v1.1.73,
`github.com/shirou/gopsutil/v4` v4.26.8,
`github.com/ulikunitz/xz` v0.5.17,
`github.com/xuri/excelize/v2`,
`go.bug.st/serial` v1.8.0,
`golang.org/x/crypto` v0.57.0,
`golang.org/x/image` v0.46.0,
`golang.org/x/net` v0.60.0,
`golang.org/x/sync` v0.23.0,
`golang.org/x/sys` v0.48.0,
`golang.org/x/text` v0.42.0,
`modernc.org/sqlite` v1.60.1,
`tailscale.com` v1.102.5.

Go-Fonten aus `golang.org/x/image/font/gofont` (Bigelow & Holmes) haben ihren
BSD-Text in `THIRD_PARTY_NOTICES.md`.

### MPL-2.0

`github.com/emiago/diago` v0.40.0,
`github.com/go-sql-driver/mysql` v1.10.1,
`github.com/philippgille/chromem-go` v0.7.0.

### EPL-2.0 OR BSD-3-Clause

`github.com/eclipse/paho.mqtt.golang` v1.5.1.

## Rust-TUI

Direktabhängigkeiten von `tools/aurago-tui`, Lizenzfeld von crates.io:

| Crate | Lizenz |
| --- | --- |
| tokio, ratatui, crossterm, urlencoding | MIT |
| reqwest, eventsource-stream, futures, serde, serde_json, toml, directories, clap, unicode-width, chrono, anyhow, base64 | MIT OR Apache-2.0 |

## Texte in `licences/texts`

Ein Text pro Lizenz. Die Urheberzeile des jeweiligen Projekts steht in der
Tabelle oben oder in den genannten Vendor-Dateien. Der MIT-Text der SPDX-Fassung
enthält Platzhalter für Jahr und Namen. Die konkreten Zeilen ersetzen diese Platzhalter.

| Datei | Gilt für |
| --- | --- |
| `MIT.txt` | MIT-Projekte |
| `Apache-2.0.txt` | Apache-2.0-Projekte |
| `BSD-2-Clause.txt` | BSD-2-Clause |
| `BSD-3-Clause.txt` | BSD-3-Clause und die EDL-Alternative von Paho |
| `ISC.txt` | ISC, darunter Lucide |
| `MPL-2.0.txt` | MPL-2.0 |
| `EPL-2.0.txt` | EPL-2.0, Paho und elkjs |
| `GPL-2.0-only.txt` | GPL-2.0 |
| `OpenSCAD-CGAL-exception.txt` | OpenSCAD-Ausnahme zum Linken gegen CGAL |
| `GPL-3.0-only.txt` | GPL-3.0 |
| `AGPL-3.0-only.txt` | AGPL-3.0 |
| `OFL-1.1.txt` | Schriftarten unter der OFL |
| `CC-BY-SA-4.0.txt` | Px437 und Wikipedia-Text |
| `Unlicense.txt` | yt-dlp und robust-predicates |
| `PostgreSQL.txt` | PostgreSQL und pgvector |
| `Zlib.txt` | Zlib-Anteil von pako |
| `SSPL-1.0.txt` | eine der beiden Redis-Lizenzen |
| `Redis-7.4.txt` | RSALv2 und SSPL, wie sie Redis 7.4 beilegt |
| `n8n-Sustainable-Use.md` | n8n |
| `Open-WebUI.txt` | Open WebUI |
| `CanvasUI-Commons-Clause.txt` | CanvasUI |
| `C64-Pro-Mono.txt` | C64 Pro Mono |
