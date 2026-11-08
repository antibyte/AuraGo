# Mirage als optionale Vision-Möglichkeit für schwache Systeme

Datum: 2026-11-08 · Status: Entwurf, vom Nutzer im Gespräch freigegeben

## Ziel

[Cactus-Compute/mirage](https://huggingface.co/Cactus-Compute/mirage) (51,3M Parameter, `mirage.cact` ca. 22 MB, Apache-2.0) wird eine optionale, lokale Vision-Quelle für Systeme, die kein bildfähiges Hauptmodell und keinen Cloud-Vision-Provider haben. Eine GPU wird automatisch genutzt, wo die Engine es kann.

## Fakten zum Modell (Stand 2026-11-06, Blog + HF-Seite)

- Läuft nur über die Cactus-Engine (Python-Paket `cactus-compute` oder native C-API). Kein GGUF, ONNX oder Ollama-Format.
- Eingabe: ein Bild (512-px-Canvas). Kein freier Prompt.
- Ausgabe: strukturiertes JSON mit Szene, Objekten (Name, Box 0–1000, Attribute, Maske), OCR-Text, Relationen/Events. `extract()` beantwortet feste Enum-Entscheidungen (nicht im Umfang).
- Sprachen: en, de, fr, es, it, nl, pl (`language="de"`).
- Tempo laut Blog: 74 ms (NPU) / 107 ms (CPU) auf einem Galaxy S26.
- **GPU-Unterstützung (CUDA/Vulkan/Metal) ist nicht dokumentiert.** Das Verhalten muss beim Bau des Sidecars geprüft werden (siehe Offene Punkte).

## Entscheidungen

1. **Runtime: Managed Sidecar-Container** (`aurago_mirage`), nach dem Muster von Piper (`internal/tools/piper_tts.go`) und den Ollama-Embeddings (`internal/tools/ollama_embeddings.go`). Keine Host-Abhängigkeit, kein cgo.
2. **Verhalten: Beschreibung als Text.** Mirage liefert die Beschreibung, das Hauptmodell beantwortet freie Fragen anhand davon. Keine neue Tool-Oberfläche, kein `extract`.

## Architektur

### Sidecar

- Image wird von AuraGo gebaut (`BuildImageContextWait`, Backoff wie bei go2rtc `ensureManagedSidecarImage`), enthält Python, `cactus-compute` und einen kleinen HTTP-Server.
- Gewichte (`mirage.cact`, `config.json`) werden beim ersten Start von Hugging Face in ein Datenvolume (`data/mirage`) geladen und danach wiederverwendet. Kein Download zur Laufzeit außer beim ersten Start.
- Endpunkte:
  - `POST /v1/chat/completions` im OpenAI-Format. Extrahiert das Bild aus `image_url` (Data-URL), ruft `Mirage().describe(...)` auf und antwortet mit der formatierten Beschreibung als `message.content`. Der Text-Teil der Nachricht wird ignoriert.
  - `GET /health` mit `{ "ready": bool, "backend": "cpu|nvidia|amd|intel|vulkan" }`.
- Container-Härtung wie bei den anderen Sidecars (`hardenManagedSidecarHostConfig`: `CapDrop ALL`, `no-new-privileges`). Bindung nur an das interne Netz bzw. Loopback.
- Eintrag in `ManagedSidecarContainerNames` (`internal/tools/managed_sidecar_names.go`).

### GPU-Automatik

- `DetectGPU(cfg.Vision.Mirage.GPU)` (`auto` | `none` | `nvidia` | `amd` | `intel` | `vulkan`) liefert `GPUInfo`. Geräte und Umgebung werden wie beim Ollama-Embeddings-Sidecar an den Container durchgereicht.
- Beim Start meldet der Sidecar über `/health`, welches Backend die Engine tatsächlich nutzt. Die UI zeigt das an. Kann die Engine die GPU nicht nutzen, läuft sie auf CPU. Das ist kein Fehler.
- Einschränkung: `DetectGPU` kennt nur Linux. Windows und macOS laufen auf CPU.

### Integration in die Vision-Kette

- Neuer Provider-Typ `mirage` für `cfg.Vision.ProviderType`. `analyzeImageReferenceWithPromptContext` (`internal/tools/vision.go`) löst Base-URL und Modell für diesen Typ aus der Mirage-Konfiguration auf, benötigt keinen API-Schlüssel und führt den normalen OpenAI-kompatiblen Aufruf aus.
- **Auto-Fallback:** Ist kein Vision-Provider konfiguriert oder das Hauptmodell nicht bildfähig und `vision.mirage.enabled` ist gesetzt, nutzt die Kette Mirage. Betrifft `analyze_image`, `internal/telegram/vision.go` und den Chat-Fallback `fallbackVisionAnalysis` (`internal/server/multimodal_ingest.go`). Ein explizit konfigurierter Provider hat immer Vorrang.
- Ausgabeformat (Text): Szene, dann Objekte (Name, grobe Position), dann Textzeilen, dann Relationen. Begrenzt durch `max_objects` (Standard 20), `max_text_lines` (Standard 30) und `max_tokens`. Die Sprache folgt der AuraGo-Sprache, wenn von Mirage unterstützt, sonst Englisch.
- Die Ausgabe läuft durch die bestehenden Schutzmechanismen (`security.GateScan`, `security.IsolateExternalData`), weil OCR-Text aus fremden Bildern Prompt-Injection enthalten kann. Der Fallback-Hinweis `visionFallbackNotice` benennt Mirage als begrenzte Beschreibung.

### Konfiguration

`vision.mirage`: `enabled` (Standard false), `container_name`, `container_port`, `image`, `data_path`, `gpu` (Standard `auto`), `max_objects`, `max_text_lines`.

UI: Abschnitt in den Vision-Einstellungen mit Schalter, GPU-Auswahl und Statusanzeige (läuft, Backend, Gewichte geladen). Texte in allen 16 Sprachen, Handbücher (en/de) und `documentation/configuration.md` werden ergänzt. Das Config-Schema folgt den bestehenden Vorgaben (kein externer Font oder Asset, Konfig-Switches mit Rerender).

## Fehlerbehandlung

- Docker nicht erreichbar, Build- oder Download-Fehler: klare Fehlermeldung an die Vision-Kette, keine stillen Abbrüche. Build-Fehler lösen einen Backoff aus, keine Endlosschleife.
- Sidecar nicht bereit: kurzes Warten auf `/health`, danach Fehler. Die Kette läuft ohne Bildbeschreibung weiter, mit dem Hinweis „Bild konnte nicht analysiert werden".
- Beschädigte Bilder und Bilder über dem bestehenden Pixel-Limit werden wie bisher in `PrepareVisionDataURL` abgelehnt, bevor sie den Sidecar erreichen.

## Tests

- Go-Unit-Tests: Provider-Auflösung für `mirage`, Fallback-Reihenfolge (expliziter Provider > Mirage > Fehler), Textformatierung samt Limits, GPU-Auswahl mit gefaktem `DetectGPU`, Config-Defaults.
- Test des Sidecar-Servers (Python) gegen ein Fake-Cactus-Modul: Data-URL-Dekodierung, Antwortformat, `/health`.
- Einmaliger Live-Test mit dem echten Modell auf der lokalen Docker-Desktop-Instanz (Rezept: `reference_local_docker_sidecar_probe`). CPU-Pfad zwingend, GPU-Pfad nur wenn ein Linux-Host mit GPU verfügbar ist (aurago-test hat keine bekannte GPU).
- Browser-Test der Einstellungs-UI nach den vorhandenen Mustern, i18n-Lint für die 16 Sprachen.

## Außerhalb des Umfangs

- `extract()` und Enum-Entscheidungen, Masken-/Box-Ausgabe an die UI, native cgo-Anbindung, GPU-Erkennung unter Windows/macOS.

## Offene Punkte (beim Bauen zu klären)

1. Ob und wie die Cactus-Engine in einem Linux-Container GPU-Geräte nutzt. Wenn nicht, bleibt `gpu` ein reiner Durchreich-Parameter, und die UI zeigt immer „CPU". Der Rest des Designs bleibt gültig.
2. Genaue Python-Version und Pakete des Images (Cactus nennt Python 3.9+), Größe des fertigen Images.
3. Ob `cactus-compute` beim ersten Aufruf selbst herunterlädt (dann den Cache auf das Datenvolume lenken) oder ob AuraGo die Dateien vorab holen muss.
