# AuraGolf P5 – Sound & Musik: Umsetzungsplan

**Ziel:** AuraGolf spielt seine Klänge aus einer eigenen kleinen Web-Audio-Engine: CC0-Effekte mit Varianten, lokal synthetisierte Bedienklänge und Ambiente, vier schleifbare Musiken mit Crossfade und Ducking. Ohne Synth-Studio-Abhängigkeit und ohne externe Ladevorgänge zur Laufzeit.

**Architektur:** Die Assets entstehen reproduzierbar aus `assets/minigolf/audio/sources.json` über `scripts/build-minigolf-audio.mjs`. Das Laufzeit-Manifest `ui/3d/minigolf/audio/manifest.json` ist der einzige Vertrag zur Laufzeit. Drei Module (`minigolf-audio-engine.js`, `minigolf-audio-synth.js`, `minigolf-audio-music.js`) liegen unter der Fassade `minigolf-audio.js`, mit der allein das Spiel spricht.

**Technik:** Vanilla ES-Module, Web Audio API (`AudioBuffer`, `BiquadFilterNode`, `ConvolverNode`, `DynamicsCompressorNode`), ffmpeg ≥ 8 mit libvorbis sowie `tar` für ZIP- und 7z-Archive (nur beim Build), Node-Testskripte (`node:assert`) mit Fake-Web-Audio und Fake-fetch, esbuild-Bundle (`scripts/build-minigolf.mjs`), Go/rod-Browser-Smoke-Test.

**Spec:** [2026-10-19-p5-sound-musik-design.md](2026-10-19-p5-sound-musik-design.md)

**Stand:** Task 1 erledigt (Commit 6a4624bee)

---

## Ausführung

- Umsetzung durch den maker-agent, Verifikation durch den test-agent.
- Tasks 2 und 3 laufen als ein Umsetzungsdurchgang. Task 3 enthält nur noch die Bundle-Größenprüfung.
- Je Task ein kombiniertes Review durch den reader-agent, nur für Critical und Important. Keine Re-Review-Runden.
- Nur die Prüfungen aus diesem Plan ausführen (lean). Der Browser-Smoke-Test läuft ausschließlich in Task 6.

## Arbeitsumgebung und Regeln

- Arbeite im Worktree `.claude/worktrees/admiring-robinson-94cd43` auf dem Branch `claude/admiring-robinson-94cd43`. Der Haupt-Checkout `C:\Users\Andi\Documents\repo\AuraGo` wird erst in Task 6 angefasst (Fast-Forward).
- Die Befehle sind für Git Bash geschrieben. In PowerShell gilt dasselbe mit `Get-FileHash -Algorithm SHA256` statt `sha256sum`.
- Nach jeder Code-Änderung committen (Nutzerwunsch). Nur die im Task genannten Pfade stagen, weil parallele Sessions im selben Checkout arbeiten.
- Vor dem Bearbeiten einer Funktion die Impact-Analyse ausführen: `node reports/memory-correction-index/run.cjs impact <Symbol> --direction upstream --repo .` (in Worktrees ggf. `--repo AuraGo`). Ein Risiko HIGH oder CRITICAL im Bericht nennen. `UNKNOWN` gilt als ungeklärt und wird per Textsuche bestätigt.
- Vor jedem Commit: `node reports/memory-correction-index/run.cjs detect-changes --scope all --repo .`. Ein `partial`- oder `truncated`-Ergebnis ist kein sauberer Check.
- Bundle: `ui/js/vendor/minigolf/game.esm.js` und `manifest.json` entstehen aus `minigolf-game.js` und enthalten die Audio-Module. Ausnahme in P5: Task 2 ändert Module dieser Kette, das Bundle wird aber erst in Task 4 neu gebaut und committet. Task 3 prüft nur die Größe und setzt das Bundle danach zurück. Das Limit liegt bei 5 MiB, die Reserve bei ca. 177 KB.
- Tests lean halten: nur die in diesem Plan genannten Prüfungen.
- Nicht pushen. Das Ergebnis landet nur auf dem lokalen `main`.

## Dateien

| Datei | Verantwortung | Tasks |
|---|---|---|
| `assets/minigolf/audio/sources.json` | Quellentabelle und Rezepte (einzige handgepflegte Eingabe) | 1 |
| `assets/minigolf/audio/README.md` | Provenienz und Rebuild | 1 |
| `scripts/build-minigolf-audio.mjs` | `--check` (lesend) und `--write --sources <dir>` | 1 |
| `ui/3d/minigolf/audio/**` | generiert: Manifest, 48 SFX, 4 Musiken | 1 |
| `package.json` | `build:minigolf` (Task 1), `test:synth-studio` (Task 2) | 1, 2 |
| `ui/js/desktop/apps/minigolf-audio-engine.js` | Engine | 2 |
| `ui/js/desktop/apps/minigolf-audio-synth.js` | Synthese-Helfer | 2 |
| `ui/js/desktop/apps/minigolf-audio-music.js` | Musik, Crossfade, Ducking | 2 |
| `ui/js/desktop/apps/minigolf-audio.js` | Fassade (Neuschreibung) | 2 |
| `scripts/test-minigolf-audio.mjs` | Neu geschrieben: Engine-, Synth-, Musik- und Fassadenprüfungen | 2 |
| `scripts/test-synth-studio-audio.mjs` | Synth-Studio-Prüfungen aus der Minigolf-Prüfung | 2 |
| `assets/minigolf/music/`, `ui/3d/minigolf/music/`, `scripts/build-minigolf-music.mjs` | entfernt | 4 |
| `ui/js/desktop/apps/minigolf-game.js` | drei kleine Stellen plus Import von `AUDIO_CREDITS` | 4 |
| `ui/js/desktop/core/module-loader.js` | Minigolf-Skripte ohne Synth Studio (Zeile 24) | 4 |
| `ui/lang/desktop/*.json` (16) | Schlüssel `minigolf.credits_body` | 4 |
| `ui/testdata/minigolf-fixture.js` | Fixture ohne Synth Studio, Musik-Dekodierung | 4 |
| `ui/desktop_minigolf_test.go` | Asset-Liste, Modul-Loader-Prüfung | 4 |
| `ui/desktop_minigolf_browser_test.go` | Browser-Smoke: Musik, Samples, Underrun | 4 |
| `ui/js/vendor/minigolf/game.esm.js`, `manifest.json` | gebautes Bundle (Größe geprüft in Task 3, Rebuild und Commit in Task 4) | 3, 4 |
| `documentation/minigolf.md` | Nutzerdoku: Sound und Prüfbefehle | 5 |
| `documentation/dependencies.md` | MIDI-Angaben (Zeilen 30–33) | 5 |
| `ui/js/desktop/apps/AGENTS.md` | AuraGolf-Audio-Eintrag | 5 |
| `assets/minigolf/README.md` | Verweis auf das Audio-README | 5 |

---

### Task 1: Asset-Pipeline (erledigt, Commit 6a4624bee)

**Dateien:**
- Angelegt: `assets/minigolf/audio/sources.json`, `assets/minigolf/audio/README.md`, `scripts/build-minigolf-audio.mjs`
- Angelegt (generiert): `ui/3d/minigolf/audio/manifest.json`, `ui/3d/minigolf/audio/sfx/*.ogg` (48), `ui/3d/minigolf/audio/music/*.ogg` (4)
- Geändert: `package.json` (Skript `build:minigolf`)
- Bleibt bis Task 4 bestehen: `assets/minigolf/music/`, `ui/3d/minigolf/music/`, `scripts/build-minigolf-music.mjs`

- [x] **Step 1: Downloads prüfen**

Die elf Downloads aus Spec Abschnitt 3.1 in einen leeren Ordner `disposable/_p5-audio/downloads/` legen (nicht committen). Die Dateinamen stimmen mit der Tabelle überein. Jeder leere Ordner außerhalb des versionierten Verzeichnisbaums funktioniert; die Umsetzung nutzte das Scratch-Verzeichnis der Sitzung.

```bash
ls -l disposable/_p5-audio/downloads/
sha256sum disposable/_p5-audio/downloads/*
```

Expected: Jeder SHA-256-Wert entspricht der Spalte `SHA-256` der Spec, die Größen entsprechen der Spalte `Bytes`. Weicht eine Datei ab: stoppen, Quelle neu laden, nicht umbenennen.

- [x] **Step 2: Werkzeuge prüfen**

```bash
ffmpeg -version
ffmpeg -hide_banner -encoders | grep libvorbis
tar --version
```

Expected: ffmpeg 8.x; der Encoder `libvorbis` ist gelistet. `tar` ist vorhanden (Windows: `%SystemRoot%\System32\tar.exe`).

- [x] **Step 3: `assets/minigolf/audio/sources.json` anlegen**

Die einzige handgepflegte Eingabe. Inhalt:

- `sources`: die elf Quellen aus Spec 3.1 mit `id`, `title`, `author`, `page`, `download`, `file`, `bytes`, `sha256`, `license` (`CC0-1.0`) und `licenseUrl`.
- `sfx`: je Ausgabegruppe und Nummer Quelle, Archivmitglied und Rezeptoptionen (Spec 3.3).
- `music`: je Titel `id`, Quelle, Bereich, Crossfade, Qualität, Titel und Urheber (Spec 3.4).
- Rezeptoptionen: `start`, `end`, `crossfade`, `fadeIn`, `fadeOut {start, duration}`, `trimSilence`, `quality`. Die beiden Rollschleifen setzen `trimSilence: false`.
- Keine Werte, die nicht in der Spec stehen.

Expected: `node -e "JSON.parse(require('fs').readFileSync('assets/minigolf/audio/sources.json','utf8'))"` läuft ohne Fehler.

- [x] **Step 4: Build-Skript schreiben, Fehlschlag prüfen**

`scripts/build-minigolf-audio.mjs` mit zwei Modi (Spec 4.7):

- Standard und `--check`: nur lesend. Prüft Manifest, Dateien, Hashes, `OggS`-Kopf und `\x01vorbis`-Header, Lizenz, Quellen, Budgets, Manifestgröße, fremde `.ogg`-Dateien und den Abgleich des Manifests mit `sources.json` (Quellen, Dateiliste, `sfx`-Zuordnung, `music`-Liste). Bei einem Fehler Exit 1 mit klarer Meldung.
- `--write --sources <dir>`: verarbeitet die Downloads (Hash-Prüfung, sicheres Entpacken, ffmpeg-Pipeline, Manifest). Kodiert in einen temporären Ordner und ersetzt die ausgelieferten Dateien erst, wenn alle Dateien entstanden sind.

Interface: nur CLI-Flags. Keine Ausgabe außerhalb von `ui/3d/minigolf/audio/` und dem Temp-Verzeichnis.

```bash
node scripts/build-minigolf-audio.mjs --check
```

Expected: Exit 1 mit Meldung, dass das Manifest fehlt (noch nicht erzeugt).

- [x] **Step 5: Assets erzeugen und verifizieren**

Zuerst die Ausführung über den Schreibmodus, danach die Prüfung:

```bash
node scripts/build-minigolf-audio.mjs --write --sources disposable/_p5-audio/downloads
node scripts/build-minigolf-audio.mjs --check
```

Expected: `--check` mit Exit 0. `ui/3d/minigolf/audio/` enthält Manifest, 48 SFX und 4 Musiken. Testgrenzen: SFX-Summe ≤ 2 MiB, Musik-Summe ≤ 6 MiB, Manifest ≤ 128 KB. Gebaut: SFX 394.397 B (48 Dateien), Musik 2.442.005 B (4 Tracks), Manifest 23.717 B.

Determinismus: Ein zweiter Schreibvorgang erzeugt identische Bytes. Die Umsetzung hat das mit ffmpeg 8.1.1 zweimal geprüft.

```bash
sha256sum ui/3d/minigolf/audio/manifest.json ui/3d/minigolf/audio/sfx/* ui/3d/minigolf/audio/music/* > disposable/_p5-audio/first.txt
node scripts/build-minigolf-audio.mjs --write --sources disposable/_p5-audio/downloads
sha256sum ui/3d/minigolf/audio/manifest.json ui/3d/minigolf/audio/sfx/* ui/3d/minigolf/audio/music/* | diff disposable/_p5-audio/first.txt -
```

Expected: `diff` ohne Ausgabe.

- [x] **Step 6: `assets/minigolf/audio/README.md` schreiben**

Inhalt:

- Provenienztabelle je Datei: Datei, Werk, Urheber, Seite, Download-URL, Lizenz, Quell-SHA-256.
- Rebuild-Schritte: ffmpeg ≥ 8 mit libvorbis, bsdtar bzw. `tar.exe` für ZIP und 7z, Aufruf von `--write --sources <dir>` und danach `--check`.
- Hinweis, dass die Originale nicht im Repository liegen.

- [x] **Step 7: `package.json` anpassen**

Skript `build:minigolf` ersetzen durch:

```
node scripts/build-minigolf-audio.mjs --check && node scripts/build-minigolf.mjs
```

Bisher stand dort `build-minigolf-music.mjs`.

Expected: `node -e "JSON.parse(require('fs').readFileSync('package.json','utf8'))"` läuft ohne Fehler. Der Bundle-Teil wird in dieser Task nicht ausgeführt.

- [x] **Step 8: Commit**

```bash
git status --short
git diff --check
node reports/memory-correction-index/run.cjs detect-changes --scope all --repo .
git add assets/minigolf/audio/sources.json assets/minigolf/audio/README.md scripts/build-minigolf-audio.mjs ui/3d/minigolf/audio package.json
git commit -m "feat(minigolf): audio asset pipeline with provenance manifest" -m "sources.json drives a reproducible ffmpeg build of the CC0 sound effects and loop-baked music. --check verifies hashes, licenses, budgets and the manifest."
```

Ausgeführt als Commit 6a4624bee.

---

### Task 2: Engine, Synth, Musik, Fassade und Tests (Durchgang mit Task 3)

**Dateien:**
- Anlegen: `ui/js/desktop/apps/minigolf-audio-engine.js`, `ui/js/desktop/apps/minigolf-audio-synth.js`, `ui/js/desktop/apps/minigolf-audio-music.js`, `scripts/test-synth-studio-audio.mjs`
- Neu schreiben: `ui/js/desktop/apps/minigolf-audio.js` (Fassade), `scripts/test-minigolf-audio.mjs` (vollständig; die Synth-Studio-Teile entfallen)
- Ändern: `package.json` (Skript `test:synth-studio`)

In diesem Durchgang gibt es keine Bundle-Änderung und keine Löschung der MIDI-Dateien. Beides folgt in Task 4.

- [ ] **Step 1: Impact prüfen**

```bash
node reports/memory-correction-index/run.cjs impact createAudio --direction upstream --repo .
```

Expected: Aufrufer ist `minigolf-game.js` (`createAudio`, Zeile 45) und die Testskripte. Ein Risiko HIGH oder CRITICAL im Bericht nennen. Die neuen Module haben keinen Impact, weil sie neu sind.

- [ ] **Step 2: Synth-Studio-Prüfungen auslagern**

`scripts/test-synth-studio-audio.mjs` anlegen mit:

- dem Master-Gain-Block aus der bisherigen `scripts/test-minigolf-audio.mjs` (Standardpegel der Synth-Studio-Engine 0,78, stummgeschaltete Engine, Lautstärke 0,4) samt der nötigen Lade-Helfer (`loadClassic` für `gm-data.js`, `synth-studio-presets.js`, `synth-studio-voices.js`, `synth-studio-audio.js`),
- `await import('./test-synth-studio-voices-presets.mjs');`
- `await import('./test-synth-studio-scheduler.mjs');`

`package.json` erhält das Skript `"test:synth-studio": "node scripts/test-synth-studio-audio.mjs"`.

```bash
npm run test:synth-studio
```

Expected: Exit 0. Die Synth-Studio-Teile sind danach nicht mehr Teil der Minigolf-Prüfung.

- [ ] **Step 3: Prüfungen schreiben, Fehlschlag bestätigen**

`scripts/test-minigolf-audio.mjs` wird vollständig neu geschrieben. Die Prüfungen:

Engine und Synth:

- Fake-Web-Audio-Kontext mit `createGain`, `createBufferSource`, `createBiquadFilter`, `createStereoPanner`, `createConvolver`, `createDynamicsCompressor`, `createOscillator`, `createBuffer`, `decodeAudioData`, `resume`, `suspend`, `close`, `state` und `currentTime`. Knoten haben `connect`, `disconnect`, Parameter mit `setTargetAtTime` und Ereignisse `ended`. Ein Fake-Timer liefert `now()`.
- `createImpulse` ist deterministisch (gleiche ID, identische Samples) und klingt ab (Energie der letzten 10 % unter 1 % der ersten 10 %) für `menu`, `pirate`, `temple`, `moon`.
- Vor `ensureContext()` existiert kein `AudioContext`.
- `play` wählt bei mehr als einer Variante nie zweimal in Folge dieselbe.
- Jitter: Rate innerhalb ±6 %, Gain innerhalb ±15 %.
- Cooldown: zweiter Aufruf derselben Gruppe innerhalb von 30 ms liefert `null`.
- Stimmen-Grenze 20: Die 21. Stimme verdrängt die älteste mit gleicher oder niedrigerer Priorität. Gibt es keine, liefert `play` `null`.
- `setReverb`: der erste Aufruf setzt den Buffer direkt.
- `loadSamples`: Fehler werden je Datei einmal gemeldet und nicht geworfen. `sampleState()` zählt sie.

Musik und Fassade:

- Der Test ruft `node scripts/build-minigolf-audio.mjs --check` auf.
- `REQUIRED_GROUPS` enthält alle Gruppen aus Spec 3.3.
- Musik-IDs sind genau `menu`, `pirate`, `temple`, `moon`.
- Die Urheber in `AUDIO_CREDITS` stimmen mit den Manifest-Quellen überein.
- Lebenszyklus mit Fake-Web-Audio und Fake-fetch (liefert das echte Manifest und die Bytes):
  - vor der Geste kein Kontext und kein fetch;
  - falsches Entsperren (`resume` lässt den Kontext auf `suspended`) → `false`, kein fetch;
  - Entsperren → Manifest über `assetURL` angefordert, Menü-Schleife gestartet;
  - `setMusic('coast')` → Crossfade auf `pirate`: neue Quelle gestartet, alte gestoppt;
  - Holz-Aufprall bei Geschwindigkeit 8 → `impact.wood.heavy`, Rate ±6 %, Gain innerhalb der Jitter-Grenzen;
  - keine unmittelbare Variantenwiederholung;
  - Stimmen-Grenze 20;
  - Rollen: Start, Stopp, Rate steigt mit dem Tempo, Untergrundwechsel schaltet die Schleifen um;
  - `setAmbient('night')` startet das Mond-Ambiente; `pause` → `suspend`, Stimmen, Schleifen und Ambiente gestoppt, `effect` → `false`; `resume` → `resume`;
  - `ui('start')` startet einen Oszillator;
  - `win` plant Ducking auf 0,25;
  - `dispose` schließt den Kontext; danach liefern `unlock`, `effect` und `nextTrack` `false`;
  - `dispose` während des Manifest-Ladens → `unlock` liefert `false`, keine Musikquelle.

```bash
node scripts/test-minigolf-audio.mjs
```

Expected: Fehlschlag mit `ERR_MODULE_NOT_FOUND` für das erste noch fehlende Modul.

- [ ] **Step 4: `minigolf-audio-synth.js` implementieren**

Exporte und Vertrag laut Spec 4.4:

- `mulberry32(seed)` (PRNG) und `noiseBuffer(ctx, seconds, color = 'white'|'brown', seed)`.
- `REVERB_PRESETS`: `menu` (0,6 s, wet 0,06), `pirate` (1,0 s, wet 0,12, Dämpfung hoch), `temple` (2,2 s, wet 0,24, dunkler Stein), `moon` (1,5 s, wet 0,18, Flatterechos 12 ms).
- `createImpulse(ctx, id)`: stereofoner Buffer, Seed-Rauschen × exponentieller Abfall (−60 dB nach `seconds`), progressive Einpol-Tiefpass-Dämpfung, einige Early-Reflection-Abgriffe. Deterministisch je `id`.
- `playUi(ctx, destination, kind)` mit `click` (Sinus 1,6 kHz, 25 ms, 8 ms Rauschtick), `confirm` (660 Hz → 990 Hz) und `back` (880 Hz → 587 Hz). Die Knoten trennen sich beim Ereignis `ended`.
- `createAmbience(ctx, destination, world, { random })` → `{ start(), stop(fade = 0.6) }`:
  - `pirate`: 4 s Brown-Noise als Schleife → Tiefpass 650 Hz → Gain mit LFO 0,09 Hz. Möwen alle 6–14 s, 2–3 Schreie (Sägezahn → Bandpass 1,6 kHz, Glide 1500 → 2100 → 1300 Hz über 0,35 s).
  - `temple`: White-Noise-Schleife → Bandpass 5,2 kHz, Q 8 → Gain mit LFO 38 Hz. Vögel alle 4–11 s, 2–5 Sinuspfiffe von 70–160 ms, Glide 2,4 → 3,6 kHz.
  - `moon`: Sinus 55 Hz + Sinus 110,4 Hz + Dreieck 165 Hz → Tiefpass 400 Hz. Piepen alle 5–15 s, 2–3 Sinuspiepen 1320/1760 Hz, je 60 ms, Pause 90 ms.
  - Bett blendet in 1,2 s ein. Ereignisse laufen über Timer, die `stop()` löscht. `stop()` blendet aus, stoppt und trennt danach alle Knoten.

```bash
node --check ui/js/desktop/apps/minigolf-audio-synth.js
```

Expected: keine Ausgabe.

- [ ] **Step 5: `minigolf-audio-engine.js` implementieren**

Signatur: `createEngine({ assetURL, onError, random = Math.random, AudioContextClass })`. Methoden und Graph nach Spec 4.3:

- Kein Kontext vor `ensureContext()`; Kontext mit `{ latencyHint: 'balanced' }`.
- Busse `effects`, `ambience`, `ui` → Gain `fx`; Bus `music` → Musik-Gain; beide → `master` (0,9) → Limiter (`DynamicsCompressorNode`: threshold −3, knee 0, ratio 20, attack 0,003, release 0,12) → Ziel. `effects` und `ambience` zusätzlich → `reverbSend` → `ConvolverNode` → `master`.
- `start()` (liefert `true` nur bei Zustand `running`), `suspend()`, `resume()`, Fehler an `onError`.
- `setMusicVolume(v)` und `setEffectsVolume(v)`: `setTargetAtTime(v, now, 0.03)`. Werte vor dem Kontext bleiben erhalten.
- `async loadSamples(sfxMap)`: `assetURL('/3d/minigolf/audio/' + path)`, `cache: 'force-cache'`, Grenze 512 KB je Datei, `decodeAudioData`, Parallelität 6. Liefert `{ loaded, failed }`, meldet Fehler je Datei einmal.
- `fetchBuffer(path, maxBytes)`: gemeinsamer Helfer; die Musik nutzt 4 MiB.
- `play(group, { gain = 1, rate = 1, pan = 0, delay = 0, priority = 0, bus = 'effects' })`: zufällige Variante ohne Wiederholung in Folge; Jitter Rate × (1 ± 0,06), Gain × (1 ± 0,15); Kette BufferSource → Gain → StereoPanner → Bus; Cooldown 30 ms je Gruppe; höchstens 20 Stimmen, Verdrängung der ältesten Stimme mit `priority` ≤ neu (Ausblendung 15 ms), sonst `null`; Entfernen beim Ereignis `ended`.
- `loop(group, { bus = 'effects' })` → `{ source, setRate(v), setGain(v), setCutoff(hz), setPan(v), stop(fade = 0.12) }`; Kette BufferSource (`loop = true`) → BiquadFilter (Tiefpass) → Gain → StereoPanner → Bus; Setter mit `setTargetAtTime(…, 0.06)`.
- `stopVoices()`: Einzelstimmen mit 20 ms Ausblendung.
- `setReverb(buffer, wet)`: Send auf 0 (Zeitkonstante 0,05), nach 0,25 s `convolver.buffer` tauschen, Send auf `wet` (Zeitkonstante 0,2). Erster Aufruf setzt direkt.
- `activeVoices()`, `sampleState()`, `context()`, `now()`, `dispose()` (stoppt alles, `ctx.close()`).

```bash
node --check ui/js/desktop/apps/minigolf-audio-engine.js
```

Expected: keine Ausgabe.

- [ ] **Step 6: `minigolf-audio-music.js` implementieren**

Signatur: `createMusic(engine, { onTrack, onError })` nach Spec 4.5:

- `setTracks(list)`, `select(id)` (ohne Wirkung bei aktueller oder ausstehender ID; Cache als Map von ID auf Promise, höchstens zwei dekodierte Titel; Generationszähler verwirft veraltete Ladevorgänge).
- Start: BufferSource `loop = true` → Gain (linear 0 → 1 in 1,8 s) → Duck-Gain → Bus `music`. Der alte Titel fährt in 1,8 s auf 0 und wird bei +1,9 s gestoppt. `onTrack({ id, title, author })` beim Start.
- `next()`: manuelle Übersteuerung, reihum durch die vier Titel.
- `duck(seconds, level)`: `setTargetAtTime(level, now, 0.05)`, dann `setTargetAtTime(1, now + seconds, 0.35)`; ein neues Ducking hebt ein früheres auf.
- `state()` → `{ playing, trackId, track, trackIndex, position }`; `stop()`.

```bash
node --check ui/js/desktop/apps/minigolf-audio-music.js
```

Expected: keine Ausgabe.

- [ ] **Step 7: Fassade neu schreiben**

`ui/js/desktop/apps/minigolf-audio.js` nach Spec 5.1 bis 5.9:

- Exporte: `createAudio`, `AUDIO_MANIFEST_PATH = '/3d/minigolf/audio/manifest.json'`, `WORLD_ALIAS = { garden: 'temple', coast: 'pirate', night: 'moon' }`, `REQUIRED_GROUPS`, `AUDIO_CREDITS`.
- `AUDIO_CREDITS`: die vier Musiktitel (Happy Clappy Loop, Pirate Adventure, Hidden Temple, Galactic Temple) sowie „Impact Sounds · Sci-Fi Sounds · Music Jingles“ / Kenney, „Bowling Ball Rolling“ / qubodup, „30 CC0 SFX Loops“ / rubberduck, „6 Short Water Splashes“ / ezwa, „Cannon Fire“ / Thimras.
- Konstruktor ohne Kontext und ohne fetch; Standardlautstärken Musik 0,2, Effekte 0,65.
- Methoden: `unlock()` (eine laufende Promise, Manifestprüfung, Musikstart, Hall, Ambiente, `loadSamples` im Hintergrund), `setMusic(context)`, `setAmbient(world|null)`, `effect(type, opts)` (Typen: `strike`, `impact`, `cup`, `splash`, `cannon`, `boost`, `teleport`, `win` mit den Gain-, Rate- und Ducking-Werten aus Spec 5.2; `pan` überall), `rolling(speed, surface, pan)` (Profile und Formeln aus Spec 5.3), `ui(action)` (Spec 5.4), `pause()`, `resume()`, `nextTrack()`, `setMusicVolume`, `setEffectsVolume`, `dispose()`, `getState()`.
- `win` nutzt die zuletzt gesetzte Welt. Die Fassade merkt sich den Welt-Kontext aus `setMusic`.

Die Aufrufe in `minigolf-game.js` müssen alle in der neuen Fassade existieren. Prüfen:

```bash
grep -n "audio\." ui/js/desktop/apps/minigolf-game.js
```

Erwartet genutzte Methoden: `setMusicVolume`, `setEffectsVolume`, `setAmbient`, `pause`, `resume`, `rolling`, `effect`, `nextTrack`, `unlock`, `getState`, `dispose`. Dazu kommen die neuen Methoden `setMusic` und `ui`, die das Spiel erst in Task 4 aufruft.

- [ ] **Step 8: Prüfungen grün**

```bash
node --check ui/js/desktop/apps/minigolf-audio.js
node scripts/test-minigolf-audio.mjs
npm run test:synth-studio
```

Expected: `node --check` ohne Ausgabe; beide Skripte Exit 0.

- [ ] **Step 9: Commit**

```bash
git status --short
git diff --check
node reports/memory-correction-index/run.cjs detect-changes --scope all --repo .
git add ui/js/desktop/apps/minigolf-audio-engine.js ui/js/desktop/apps/minigolf-audio-synth.js ui/js/desktop/apps/minigolf-audio-music.js ui/js/desktop/apps/minigolf-audio.js scripts/test-minigolf-audio.mjs scripts/test-synth-studio-audio.mjs package.json
git commit -m "feat(minigolf): own audio engine, music crossfades and facade" -m "Engine, synth helpers, music module and facade replace the Synth Studio dependency. The Synth Studio checks move to test-synth-studio-audio.mjs (npm run test:synth-studio). The bundle, the game wiring and the removal of the MIDI assets follow in the next commit."
```

---

### Task 3: Bundle-Größe prüfen

**Dateien:**
- Prüfen: `ui/js/vendor/minigolf/` (Build wird erzeugt und danach zurückgesetzt, kein Commit)

Dieser Task läuft im selben Durchgang wie Task 2 und enthält nur die Größenprüfung.

- [ ] **Step 1: Bundle bauen und Größe prüfen**

```bash
node scripts/build-minigolf.mjs
```

Expected: Build ohne Fehler, Bundle unter 5 MiB. Die Reserve liegt bei ca. 177 KB.

- [ ] **Step 2: Bundle zurücksetzen**

```bash
git restore ui/js/vendor/minigolf
git status --short ui/js/vendor/minigolf
```

Expected: keine Ausgabe. Das Bundle bleibt auf dem Stand aus Task 1. Der Rebuild und sein Commit folgen in Task 4.

---

### Task 4: Spielanbindung, Credits, Locales, Browser-Smoke, MIDI-Entfernung und Bundle

**Dateien:**
- Ändern: `ui/js/desktop/apps/minigolf-game.js` (drei kleine Stellen plus Import von `AUDIO_CREDITS`), `ui/js/desktop/core/module-loader.js` (Zeile 24), `ui/lang/desktop/*.json` (16 Dateien), `ui/testdata/minigolf-fixture.js`, `ui/desktop_minigolf_browser_test.go`, `ui/desktop_minigolf_test.go`, `ui/js/vendor/minigolf/` (Bundle neu bauen)
- Entfernen: `assets/minigolf/music/`, `ui/3d/minigolf/music/`, `scripts/build-minigolf-music.mjs`

Die Entfernung der MIDI-Dateien steht hier und nicht in Task 2, weil `ui/desktop_minigolf_test.go` und die Browser-Fixture bis Task 4 noch auf die alten Musikpfade zeigen.

- [ ] **Step 1: Impact prüfen**

```bash
node reports/memory-correction-index/run.cjs impact syncAmbient --direction upstream --repo .
node reports/memory-correction-index/run.cjs impact click --direction upstream --repo .
```

Expected: Ergebnisse nur für `minigolf-game.js` werten. `click` ist ein häufiger Name. Bei Mehrdeutigkeit die Datei in der Auswertung angeben. HIGH oder CRITICAL im Bericht nennen.

- [ ] **Step 2: Modul-Loader umstellen**

`ui/js/desktop/core/module-loader.js`, Zeile 24: Minigolf-Skripte von `['/js/vendor/synth-studio/gm-data.js', '/js/desktop/apps/synth-studio-voices.js', '/js/desktop/apps/synth-studio-audio.js', '/js/desktop/apps/minigolf.js']` auf `['/js/desktop/apps/minigolf.js']`. Andere Apps bleiben unverändert.

Expected: Die Zeile des Minigolf-Eintrags enthält kein `synth-studio` mehr.

- [ ] **Step 3: `minigolf-game.js`, drei kleine Stellen plus Import**

1. `syncAmbient()` (Zeile 53–55): nach `audio.setAmbient(world);` ergänzen:
   `audio.setMusic(panelKind === 'menu' || !round || round.completed || !course ? 'menu' : course.world);`
2. `click()`: vor dem Ausführen der Aktion `audio.ui(button.dataset.action);` aufrufen.
3. Credits: die fest eingetragene MIDI-Liste `tracks = [` (Zeile 345) durch `AUDIO_CREDITS` ersetzen.
4. Import (Zeile 4), gehört zu Punkt 3: `import { AUDIO_CREDITS, createAudio } from './minigolf-audio.js';`

Die drei Stellen plus der Import ergeben die vier Treffer in der Prüfung unten.

```bash
grep -n "audio\.setMusic\|audio\.ui\|AUDIO_CREDITS" ui/js/desktop/apps/minigolf-game.js
```

Expected: vier Treffer (Import, `setMusic`, `ui`, Credits-Zuweisung).

- [ ] **Step 4: Locales**

Schlüssel `minigolf.credits_body` in allen 16 Dateien `ui/lang/desktop/*.json` ergänzen. Die Locales sind flach: der Schlüssel steht als `"minigolf.credits_body"` auf oberster Ebene, wie `"minigolf.title"`.

- `en`: „Original Blender artwork for AuraGo. Music and recorded sound effects are published under CC0; ambience and interface sounds are synthesized locally.“
- `de`: „Originale Blender-Grafik für AuraGo. Musik und aufgenommene Soundeffekte stehen unter CC0; Ambiente und Bedienklänge werden lokal synthetisiert.“
- Die übrigen 14 Locales (cs, da, el, es, fr, hi, it, ja, nl, no, pl, pt, sv, zh) in der jeweiligen Sprache formulieren, nicht aus dem Englischen kopieren.

```bash
node -e "for (const l of ['cs','da','de','el','en','es','fr','hi','it','ja','nl','no','pl','pt','sv','zh']) { const j = JSON.parse(require('fs').readFileSync('ui/lang/desktop/'+l+'.json','utf8')); if (!j['minigolf.credits_body']) throw new Error(l); }"
grep -l "Original Blender artwork" ui/lang/desktop/*.json
```

Expected: die Node-Prüfung läuft ohne Fehler. `grep` liefert nur `ui/lang/desktop/en.json`.

- [ ] **Step 5: Browser-Fixture und Go-Tests**

`ui/testdata/minigolf-fixture.js`:

- Synth-Studio-Skripte entfernen.
- `renderMusicSamples` ersetzen durch `decodeAudioSamples`.
- Den Handler `/__minigolf-render/` entfernen.
- Kein `armIndependentPlayerTest`.

`ui/desktop_minigolf_browser_test.go` (Smoke, Spec 7):

- `decodeAudioSamples`: jede Musik-OGG mit einem `OfflineAudioContext` dekodieren, dann prüfen: Dauer = Manifest-Sekunden ± 0,05 s, RMS > 0,01, Anteil nahe Vollaussteuerung ≤ 0,001, Schleifennaht (|letzter − erster| Sample je Kanal) < 0,1.
- Nach dem Entsperren warten, bis `audio.samples.failed === 0` und `loaded` der Anzahl der SFX-Dateien aus dem Manifest entspricht.
- `tick`-Prüfungen liefern `position`.
- Die Prüfung unabhängiger Player entfernen.
- Underrun-Prüfungen unverändert, Toleranz null.
- Keine Anfrage verlässt den Test-Origin.

`ui/desktop_minigolf_test.go`:

- Asset-Liste: `3d/minigolf/audio/manifest.json` statt der Musik-Manifeste.
- Modul-Loader-Prüfung: Der Minigolf-Eintrag darf `synth-studio` nicht enthalten.

```bash
go test ./ui -run '^TestDesktopMinigolf(RegistrationAndLocalResources|Locales)$' -count=1
```

Expected: `ok`.

- [ ] **Step 6: MIDI-Altlasten entfernen**

```bash
git rm -r assets/minigolf/music ui/3d/minigolf/music scripts/build-minigolf-music.mjs
git grep -n "build-minigolf-music\|minigolf/music" -- ':!documentation' ':!ui/js/vendor'
```

Expected: Die Löschungen sind gestaged. Die Suche liefert keine Treffer, weil Schritt 5 die Fixture und den Go-Test bereits umgestellt hat.

- [ ] **Step 7: Bundle bauen**

```bash
node scripts/build-minigolf.mjs
node scripts/build-minigolf.mjs --check
```

Expected: Build ohne Fehler unter 5 MiB, `--check` ohne Fehler.

- [ ] **Step 8: Prüfungen ohne Browser**

```bash
node scripts/build-minigolf-audio.mjs --check
node scripts/test-minigolf-audio.mjs
go test ./ui -run Minigolf -count=1
```

Expected: alle drei ohne Fehler. Der Browser-Test wird ohne `AURAGO_RUN_BROWSER_SMOKE` übersprungen.

- [ ] **Step 9: Commit**

```bash
git status --short
git diff --check
node reports/memory-correction-index/run.cjs detect-changes --scope all --repo .
git add ui/js/desktop/apps/minigolf-game.js ui/js/desktop/core/module-loader.js ui/lang/desktop ui/testdata/minigolf-fixture.js ui/desktop_minigolf_browser_test.go ui/desktop_minigolf_test.go ui/js/vendor/minigolf
git commit -m "feat(minigolf): wire own audio into the game, credits and browser smoke" -m "The game switches music per world and sends UI clicks to the audio facade. Credits come from AUDIO_CREDITS, and minigolf.credits_body is added to all 16 Desktop locales. The MIDI music assets and their build script are removed, the minigolf module loader drops Synth Studio, and the bundle is rebuilt."
```

Die Löschungen aus Schritt 6 sind bereits gestaged.

---

### Task 5: Dokumentation

**Dateien:**
- Ändern: `documentation/minigolf.md`, `documentation/dependencies.md`, `ui/js/desktop/apps/AGENTS.md`, `assets/minigolf/README.md`

- [ ] **Step 1: `documentation/minigolf.md`**

- Den Sound-Abschnitt neu schreiben. Er beschreibt: eigene Engine, CC0-Effekte, lokal synthetisierte Bedienklänge und Ambiente, vier CC0-Musiken mit Crossfade. Standardlautstärken 20 % (Musik) und 65 % (Effekte) bleiben. Stummschaltung und Lautstärken bleiben gespeichert. Keine externen Ladevorgänge. Verweis auf `assets/minigolf/audio/README.md`. Die bisherigen Angaben zu MIDI-Originalen, Synth Studio und dem 750-ms-Vorlauf entfallen.
- Prüfbefehle: `node scripts/build-minigolf-music.mjs --check` ersetzen durch `node scripts/build-minigolf-audio.mjs --check`; `npm run test:synth-studio` ergänzen.
- Den Satz zur Variable `AURAGO_BROWSER_MUTE_AUDIO` prüfen und an den Stand nach Task 4 anpassen.

- [ ] **Step 2: `documentation/dependencies.md`**

Zeilen 30–33 (MIDI-Renderer, `build-minigolf-music.mjs --check`) auf den Audio-Stand umstellen: `node scripts/build-minigolf-audio.mjs --check`, statt des MIDI-Renderers die Angabe, dass die Effekte aus CC0-Quellen stammen und die Synthese lokal erfolgt, keine SoundFont, kein Runtime-CDN. Der Synth-Studio-Abschnitt ab Zeile 95 bleibt unverändert.

- [ ] **Step 3: `ui/js/desktop/apps/AGENTS.md`**

Den AuraGolf-Audio-Eintrag (Zeilen 82–92) ersetzen: eigene Engine statt Synth-Studio-Scheduler, Fassade `minigolf-audio.js` mit `setMusic`, `setAmbient`, `effect`, `ui`, `rolling`, Musikmodul, Manifest `ui/3d/minigolf/audio/manifest.json`, Prüfungen `scripts/test-minigolf-audio.mjs` und `npm run test:synth-studio`. Die Prüfzeile (Zeile 93) entsprechend anpassen.

- [ ] **Step 4: `assets/minigolf/README.md`**

Verweis auf `assets/minigolf/audio/README.md` für Töne, Musik, Quellen und Lizenzen ergänzen.

- [ ] **Step 5: Restprüfung**

```bash
git grep -n "build-minigolf-music\|minigolf/music" -- documentation ui/js/desktop/apps/AGENTS.md assets/minigolf
```

Expected: keine Treffer mehr. Treffer für `synth-studio` sind danach nur noch die neuen Prüfbefehle (`test:synth-studio`, `test-synth-studio-audio.mjs`) und die Synth-Studio-eigene Doku. Die Erwähnungen in `minigolf.md` und im AGENTS-Eintrag prüfen.

- [ ] **Step 6: Commit**

```bash
git status --short
git diff --check
node reports/memory-correction-index/run.cjs detect-changes --scope all --repo .
git add documentation/minigolf.md documentation/dependencies.md ui/js/desktop/apps/AGENTS.md assets/minigolf/README.md
git commit -m "docs(minigolf): own audio engine, sources and verification commands" -m "The AuraGolf sound section, the dependency notes and the apps AGENTS entry describe the own engine, the CC0 sources and the new check commands."
```

---

### Task 6: Gesamtprüfung und Landung

**Dateien:** keine Quelländerung. Bundle nur nach einem Rebase neu bauen, falls nötig.

- [ ] **Step 1: Assets**

```bash
node scripts/build-minigolf-audio.mjs --check
```

Expected: Exit 0.

- [ ] **Step 2: Minigolf-Suite**

```bash
npm run test:minigolf
```

Expected: Exit 0. Die Suite umfasst Modell, Physik, Assets, Courses und Audio.

- [ ] **Step 3: Synth-Studio-Prüfungen**

```bash
npm run test:synth-studio
```

Expected: Exit 0.

- [ ] **Step 4: Bundle**

```bash
node scripts/build-minigolf.mjs
git diff --stat -- ui/js/vendor/minigolf
```

Expected: keine Änderung am Bundle, also keine Ausgabe von `git diff --stat`. Weicht das Bundle ab, sind Quellen nach dem letzten Rebuild geändert worden. Dann Diff prüfen und mit eigenem Commit nachziehen.

- [ ] **Step 5: Go-Tests ohne Browser**

```bash
go test ./ui -run Minigolf -count=1
```

Expected: `ok`.

- [ ] **Step 6: Browser-Smoke**

```bash
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopMinigolfBrowser -count=1 -timeout 20m
```

PowerShell: `$env:AURAGO_RUN_BROWSER_SMOKE = '1'; go test ./ui -run TestDesktopMinigolfBrowser -count=1 -timeout 20m`

Expected: `ok`. Die Underrun-Prüfung hat keine Toleranz. Schlägt nur sie fehl, den Lauf einmal wiederholen, bevor eine Regression vermutet wird. Der Test ist unter starker Last schon einmal gescheitert. Scheitert er zweimal, die Ursache durch den debug-agent untersuchen lassen.

- [ ] **Step 7: Desktop-Bundles**

`npm run build:ui -- --check` nur ausführen, wenn Desktop-Bundles betroffen sind. Für diesen Plan wird nicht erwartet, dass sie betroffen sind. Die Änderung an `ui/js/desktop/core/module-loader.js` ist klein und liegt außerhalb der Minigolf-Bundles.

- [ ] **Step 8: Rebase auf `main`**

```bash
git rebase main
```

Bei Konflikten in `minigolf-game.js`, den Locales oder dem Bundle: Konflikt lösen, Bundle neu bauen (`node scripts/build-minigolf.mjs`), dann Schritte 1 bis 3 wiederholen.

Expected: Rebase ohne Konflikt oder mit gelösten Konflikten. `git log --oneline main..HEAD` zeigt vier Commits: Task 1 (6a4624bee), Task 2, Task 4 und Task 5. Task 3 hat keinen eigenen Commit.

- [ ] **Step 9: Fast-Forward auf lokales `main`**

```bash
git -C /c/Users/Andi/Documents/repo/AuraGo branch --show-current
git -C /c/Users/Andi/Documents/repo/AuraGo merge --ff-only claude/admiring-robinson-94cd43
git -C /c/Users/Andi/Documents/repo/AuraGo log --oneline -8
```

Expected: Der Haupt-Checkout steht auf `main`. Das Fast-Forward läuft ohne Meldung „local changes would be overwritten“. Kommt diese Meldung, nicht erzwingen, sondern den Nutzer informieren, weil eine andere Session dieselben Dateien offen hat.

- [ ] **Step 10: Nicht pushen**

Nicht pushen. Die Hörabnahme (Zuordnung der Aufprallklänge, Feintuning des Schlags) bleibt ein separater manueller Schritt des Nutzers (Spec Abschnitt 8).
