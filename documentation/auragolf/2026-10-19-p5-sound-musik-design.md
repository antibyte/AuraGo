# AuraGolf P5 – Sound & Musik (Design)

- **Datum:** 2026-10-19
- **Status:** Freigegeben durch den Nutzer (2026-10-19)
- **Bezug:** [Fahrplan](2026-10-18-overhaul-roadmap-design.md), Abschnitte 3, 4.6 und 5
- **Umsetzungsplan:** [P5-Plan](2026-10-19-p5-sound-musik-plan.md)

P5 ersetzt die Abhängigkeit von Synth Studio durch eine eigene kleine Web-Audio-Engine. Effekte stammen aus CC0-Aufnahmen; Bedienklänge, Ambiente und Hallimpulse entstehen lokal. Zur Laufzeit wird nichts von außen geladen. P5 bringt keine neuen Einstellungen und keine neuen Bedienelemente.

## 1. Ziel und Umfang

- Eigene kleine Web-Audio-Engine für AuraGolf. Der Modul-Loader lädt für Minigolf nur noch `/js/desktop/apps/minigolf.js`. Der Synth-Studio-Code bleibt unverändert.
- Gesampelte Effekte aus CC0-Aufnahmen mit Varianten, zufälliger Tonhöhe (±6 %) und zufälliger Lautstärke (±15 %): Rollschleife (Tempo und Untergrund), Aufprall je Material und Stärke, Lochfall mit Randklappern, Platscher, Kanone, Boost, Teleporter und Schlag sowie ein Siegesjingle je Welt.
- Lokal synthetisiert: UI-Klicks, Ambiente je Welt (Piraten: Wellen und Möwen; Tempel: Insekten und Vögel; Mond: Brummen und Piepen) und je Welt eine Hall-Impulsantwort.
- Limiter auf dem Master-Bus.
- Vier schleifbare CC0-OGG-Musiken (`menu`, `pirate`, `temple`, `moon`) mit 1,8 s Crossfade und Ducking bei Loch, Kanone und Sieg.
- Herkunft: Quellseite, Download-URL, Urheber, Lizenz sowie SHA-256 von Download und ausgelieferter Datei stehen in `assets/minigolf/audio/sources.json`, im Laufzeit-Manifest und in `assets/minigolf/audio/README.md`.
- Budgets: Testgrenzen SFX ≤ 2 MiB, Musik ≤ 6 MiB, Manifest ≤ 128 KB. Gebaut wurden SFX 394.397 B (48 Dateien), Musik 2.442.005 B (4 Tracks) und ein Manifest mit 23.717 B. Damit unterschreiten die gebauten Werte die Richtwerte des Fahrplans (SFX ca. 1–2 MB, Musik ca. 5 MB). Zur Laufzeit wird nichts extern geladen.

### Nicht-Ziele

- Keine Änderungen an Synth Studio.
- Keine neuen Server-Endpunkte.
- Keine neuen Einstellungsschlüssel. Musik- und Effektlautstärke bleiben im bestehenden Schlüssel `aurago.desktop.minigolf.settings.v1`.
- Keine neuen Bedienelemente. Die UI gehört zu P4.
- Keine Verdrahtung der P3-Mechaniken. P5 stellt nur die Effekte `cannon`, `boost` und `teleport` bereit.

## 2. Getroffene Entscheidungen

**Eigene Web-Audio-Engine mit dekodierten Puffern und offline gebackenen Schleifen.** Effekte und Musik liegen als OGG-Dateien vor, werden beim Entsperren dekodiert und als `AudioBuffer` abgespielt. Musikschleifen entstehen beim Build (Abschnitt 3.4), sodass `AudioBufferSourceNode.loop = true` ohne Lücke läuft.

Verworfene Alternativen:

- **Musik per `<audio>` über `MediaElementSource`:** Schleifen entstehen mit Lücken, Crossfades werden ungenau, und das Streaming birgt ein Risiko im Render-Thread.
- **Eine Bibliothek wie Howler:** Sie bringt eine zusätzliche Abhängigkeit mit und deckt weder Hall, Limiter noch Synthese ab.

## 3. Quellen und Dateien

### 3.1 Freigegebene Downloads

Alle Quellen stehen unter CC0-1.0 (https://creativecommons.org/publicdomain/zero/1.0/). Kenney liefert eine `License.txt` mit, ezwa nennt in der `info.txt` Public Domain/CC0, alle OGA-Seiten zeigen CC0. Heruntergeladen am 2026-10-19. Die Originale werden nicht committet.

| ID | Datei | Urheber | Seite | Download-URL | Bytes | SHA-256 |
|---|---|---|---|---|---|---|
| kenney-impact | kenney_impact-sounds.zip | Kenney | https://kenney.nl/assets/impact-sounds | https://kenney.nl/media/pages/assets/impact-sounds/87b4ddecda-1677589768/kenney_impact-sounds.zip | 800850 | 029d734af1582474edf3a694d1b0cebc97c1c152f2f39fa34d4c2bafc5de77f8 |
| kenney-scifi | kenney_sci-fi-sounds.zip | Kenney | https://kenney.nl/assets/sci-fi-sounds | https://kenney.nl/media/pages/assets/sci-fi-sounds/6b296f9ecf-1677589334/kenney_sci-fi-sounds.zip | 5875104 | 119340f351a5098ad814f78719438c0da355a9ce8a4c8a3af6a8d48aa3d49e04 |
| kenney-jingles | kenney_music-jingles.zip | Kenney | https://kenney.nl/assets/music-jingles | https://kenney.nl/media/pages/assets/music-jingles/f37e530b9e-1677590399/kenney_music-jingles.zip | 1239525 | b729ba57959bd58793d2c5cafa348aaf2655d354f3da35ec4729e03ec77197b8 |
| oga-bowling-roll | qubodup-bowling-roll.7z | qubodup | https://opengameart.org/content/bowling-ball-rolling | https://opengameart.org/sites/default/files/qubodup-bowling-roll.7z | 195563 | 3c99cfe42ecf8a87ab13f66f0f4c16d0a2bdbf75f825eb2e959baab581091af8 |
| oga-sfx-loops | sfx_loops.zip | rubberduck | https://opengameart.org/content/30-cc0-sfx-loops | https://opengameart.org/sites/default/files/sfx_loops.zip | 3526706 | 9c013474c7e56192a0d1b2840535a1e1d8b93166948d4a9f7136bb6c3d5421cd |
| oga-water-splash | ezwa-water_splash.7z | ezwa (cut by qubodup) | https://opengameart.org/content/6-short-water-splashes | https://opengameart.org/sites/default/files/ezwa-water_splash.7z | 426627 | 6187fa510cb7b5e8c3ac7c79f294ddc0e44d07472758bf6ecf2628a372ae9a26 |
| oga-cannon-fire | cannon_fire_0.ogg | Thimras | https://opengameart.org/content/cannon-fire | https://opengameart.org/sites/default/files/cannon_fire_0.ogg | 179939 | 2b21ffa41e5c3be4eecdfbd3490544f5aa8b6d1581787265863e13de1a8e599d |
| oga-happy-clappy | HappyClappyLoop.wav | OwlishMedia | https://opengameart.org/content/happy-clappy-loop | https://opengameart.org/sites/default/files/HappyClappyLoop.wav | 6162894 | 1ca66b4e4d41e14a4ccfcc3b758e061cf283b9d5e332c4ea8e00f94f4deb6554 |
| oga-pirate-adventure | pirate_adventure.wav | Eldritch Grim | https://opengameart.org/content/pirate-adventure | https://opengameart.org/sites/default/files/pirate_adventure.wav | 10935228 | 54c3bff9b139054685cd3ecb62c2da8815496df5cc8df7c8888d97891dcb5640 |
| oga-hidden-temple | hidden_temple_0.wav | Eldritch Grim | https://opengameart.org/content/hidden-temple | https://opengameart.org/sites/default/files/hidden_temple_0.wav | 11910076 | 6feaea11112a772b1a4825033fd65f157309ec5216f9a2c074b1efb53954be12 |
| oga-galactic-temple | GalacticTemple.zip | yd | https://opengameart.org/content/galactic-temple | https://opengameart.org/sites/default/files/GalacticTemple.zip | 1083496 | 4713ef3fb215e51fb2bdc2a6ca8e1f399461108fc2efad63874c4df4dba91c8d |

Archivmitglieder für die Rezepte (exakt, relativ zur Archivwurzel):

- Bowling-Roll: `qubodup-bowling-roll/qubodup-bowling_roll-nofadeout.ogg`
- Platscher: `ezwa-water_splash/water_splash-01.flac` (ebenso `02`, `04`, `05`)
- Kenney-Archive: Mitglieder unter `Audio/…`, z. B. `Audio/impactWood_light_000.ogg`
- `rolling.ogg` (aus `sfx_loops.zip`) und `GalacticTemple.ogg` (aus `GalacticTemple.zip`) liegen in der Archivwurzel.
- Direkte Dateien (`cannon_fire_0.ogg`, `pirate_adventure.wav`, `hidden_temple_0.wav`, `HappyClappyLoop.wav`): der Dateiname ist das Mitglied.

### 3.2 Gemessene Fakten

Gemessen mit ffprobe und ffmpeg 8.1.1:

- `HappyClappyLoop.wav`: 17,44 s, f32, 44,1 kHz, Stereo, konstant −20 dB RMS, Anfangs- und Endpegel gleich. Nahtlos, ohne Bearbeitung verwendbar.
- `pirate_adventure.wav`: 61,99 s, Ausblendung beginnt bei ca. 52 s. Schleifenbereich 0–51,5 s, Bake-Crossfade 2,5 s, Ausgabe 49 s.
- `hidden_temple_0.wav`: 67,52 s, Ausblendung bei ca. 64 s. Schleifenbereich 0–63,5 s, Crossfade 2,5 s, Ausgabe 61 s.
- `GalacticTemple.ogg` (im ZIP): 173,14 s, 49 kbps, leises Intro, Abschnitt beginnt bei ca. 28 s. Schleifenbereich 28–128 s, Crossfade 4 s, Ausgabe 96 s. Das hält den dekodierten Speicher bei ca. 37 MB.
- `rolling.ogg` (aus `sfx_loops.zip`): 1,056 s, nahtlose Schleife, konstant −23,5 dB. Wird zu `roll.soft`.
- `qubodup-bowling_roll-nofadeout.ogg`: 4,11 s, 96 kHz. Der Treffer-Transient liegt in den ersten 0,2 s, der stabile Teil bei 0,3–3,7 s. Bereich 0,3–3,7 s, Crossfade 0,3 s. Wird zu `roll.hard`.
- ezwa-Platscher: 01, 02, 04 und 05 sind verwendbar. 03 (Peak −17,5 dBFS) und 06 (Peak −32,5 dBFS) entfallen.
- `cannon_fire_0.ogg`: 3,58 s, Knall mit anschließendem Abklingen. Zuschnitt 0–2,2 s, Ausblendung ab 1,4 s.
- Kenney-Impact-Samples dekodieren mit Peaks bis +2 dBFS. Deshalb wird jede SFX auf −1 dBFS Peak normalisiert.
- Jingles: STEEL07 1,55 s, PIZZI07 1,32 s, NES00 1,76 s.

**Ergebnis des Builds** (Musik, ffmpeg 8.1.1):

| ID | Dauer | Größe | Lautheit | Nahtsprung L | Nahtsprung R |
|---|---|---|---|---|---|
| menu | 17,441 s | 300.688 B | −16,1 LUFS | 0,066 | 0,047 |
| pirate | 49,000 s | 629.819 B | −16,1 LUFS | 0,006 | 0,009 |
| temple | 61,000 s | 746.143 B | −16,0 LUFS | 0,001 | 0,018 |
| moon | 96,000 s | 765.355 B | −16,4 LUFS | 0,001 | 0,004 |

Der Nahtsprung ist die Differenz zwischen letztem und erstem Sample je Kanal; die Grenze liegt bei 0,1 (Abschnitt 7). Beim Titel `moon` kostet die Kodierung mit q1 etwa 0,4 LU. Zwei `--write`-Läufe mit ffmpeg 8.1.1 erzeugten byte-identische Dateien.

### 3.3 SFX-Satz

Mono, 44,1 kHz, libvorbis q4, Peak −1 dBFS, führende Stille entfernt. 48 Dateien. Die Gruppen beziehen ihre Mitglieder aus folgenden Quellen (Kenney-Impact unter `Audio/`):

- `strike`: impactGeneric_light_000..002
- `impact.wood.light`: impactWood_light_000..002; `impact.wood.medium`: impactWood_medium_000..002; `impact.wood.heavy`: impactWood_heavy_000..002
- `impact.stone`: impactGeneric_light_003, impactGeneric_light_004, impactPlank_medium_002 (alle Stufen; die Stufe kommt über Rate und Gain)
- `impact.metal.light`: impactMetal_light_000..002; `impact.metal.medium`: impactMetal_medium_000..001; `impact.metal.heavy`: impactMetal_heavy_000..001
- `impact.soft.light`: impactSoft_medium_000..002; `impact.soft.heavy`: impactSoft_heavy_000..001 (Landungen auf Gras und Sand; auch der Lochboden)
- `cup.rim`: impactPlate_light_000..002; `cup.rattle`: impactTin_medium_000..002
- `splash`: ezwa water_splash-01/02/04/05 (`.flac` im 7z)
- `cannon`: cannon_fire_0.ogg (0–2,2 s, Ausblendung 1,4–2,2 s); `cannon.boom`: kenney-scifi lowFrequency_explosion_001
- `boost`: kenney-scifi thrusterFire_000..001 (0–1,1 s, Einblendung 20 ms, Ausblendung ab 0,6 s)
- `teleport`: kenney-scifi forceField_000..001
- `roll.soft`: oga-sfx-loops rolling.ogg (nahtlos); `roll.hard`: qubodup-Bowling-Roll ohne Ausblendung, Bereich 0,3–3,7 s mit gebackener 0,3-s-Crossfade-Schleife
- `win.pirate`: `Audio/Steel jingles/jingles_STEEL07.ogg`; `win.temple`: `Audio/Pizzicato jingles/jingles_PIZZI07.ogg`; `win.moon`: `Audio/8-Bit jingles/jingles_NES00.ogg` (alle aus den Kenney-Jingles)

Ausgabenamen: `sfx/<Gruppe, Punkte durch Bindestriche ersetzt>-<n>.ogg`, `n` ab 1, z. B. `sfx/impact-wood-light-1.ogg`.

### 3.4 Musiksatz

Stereo, 44,1 kHz, libvorbis, auf −16 LUFS integriert normalisiert, Peak ≤ −1 dBFS. Ausgabenamen: `music/<id>.ogg`.

| ID | Quelle | Bereich | Crossfade | Qualität | Titel | Urheber |
|---|---|---|---|---|---|---|
| menu | oga-happy-clappy | ganz | 0 | q3 | Happy Clappy Loop | OwlishMedia |
| pirate | oga-pirate-adventure | 0–51,5 s | 2,5 s | q2 | Pirate Adventure | Eldritch Grim |
| temple | oga-hidden-temple | 0–63,5 s | 2,5 s | q2 | Hidden Temple | Eldritch Grim |
| moon | oga-galactic-temple (`GalacticTemple.ogg`) | 28–128 s | 4 s | q1 | Galactic Temple | yd |

Schleifen-Bake: Bei einem Crossfade C über den Bereich [a, b] gilt Ausgabe = acrossfade(A = [a + C, b), B = [a, a + C)), Dauer C, Kurve `squ` auf beiden Seiten (Gains als Quadratwurzel, konstante Leistung). Die Kurve steht als Konstante `CROSSFADE_CURVE` im Build-Skript. Die Ausgabe hat die Länge b − a − C, und ihr Ende fließt in ihren Anfang. Deshalb ist `AudioBufferSourceNode.loop = true` nahtlos. Bei C = 0 handelt es sich um reines Zuschneiden.

## 4. Architektur

### 4.1 Dateiübersicht

Neu:

- `scripts/build-minigolf-audio.mjs`: Standard und `--check` prüfen nur lesend. `--write --sources <dir>` verarbeitet die Downloads.
- `assets/minigolf/audio/sources.json`: Quellentabelle (Abschnitt 3.1) und Rezepte (Abschnitte 3.3 und 3.4). Die einzige handgepflegte Eingabe.
- `assets/minigolf/audio/README.md`: Provenienztabelle (Datei, Werk, Urheber, Seite, Download-URL, Lizenz, Quell-SHA-256) und Rebuild-Schritte. Benötigt ffmpeg ≥ 8 mit libvorbis sowie bsdtar bzw. das Windows-`tar.exe` für ZIP und 7z.
- `ui/3d/minigolf/audio/manifest.json`, `ui/3d/minigolf/audio/sfx/*.ogg`, `ui/3d/minigolf/audio/music/*.ogg`: generiert und committet.
- `ui/js/desktop/apps/minigolf-audio-engine.js`, `ui/js/desktop/apps/minigolf-audio-music.js`, `ui/js/desktop/apps/minigolf-audio-synth.js`.
- `scripts/test-synth-studio-audio.mjs`: Synth-Studio-Prüfungen aus der Minigolf-Prüfung (Master-Gain-Block, `await import('./test-synth-studio-voices-presets.mjs')`, `await import('./test-synth-studio-scheduler.mjs')`). Npm-Skript `test:synth-studio`.

Geändert oder neu geschrieben:

- `ui/js/desktop/apps/minigolf-audio.js`: Fassade.
- `scripts/test-minigolf-audio.mjs`: neu geschrieben.
- `ui/js/desktop/apps/minigolf-game.js`: drei kleine Stellen plus der Import von `AUDIO_CREDITS` (Abschnitt 6).
- `ui/js/desktop/core/module-loader.js`, Zeile 24: Minigolf-Skripte werden zu `['/js/desktop/apps/minigolf.js']`.
- `package.json`: `build:minigolf` wird zu `node scripts/build-minigolf-audio.mjs --check && node scripts/build-minigolf.mjs`. Neues Skript `test:synth-studio`.
- `ui/lang/desktop/*.json` (16 Dateien): Schlüssel `minigolf.credits_body`.
- `ui/testdata/minigolf-fixture.js`, `ui/desktop_minigolf_browser_test.go`, `ui/desktop_minigolf_test.go`.
- Bundle: `node scripts/build-minigolf.mjs` erzeugt `ui/js/vendor/minigolf/{game.esm.js,manifest.json}` neu. Limit 5 MiB, Reserve ca. 177 KB. Audio-Code bleibt kompakt.
- Dokumentation: `documentation/minigolf.md` (Sound-Abschnitt und Prüfbefehle), `documentation/dependencies.md` (MIDI-Renderer-Angaben, Zeilen 30–33), `ui/js/desktop/apps/AGENTS.md` (AuraGolf-Audio-Eintrag), `assets/minigolf/README.md` (Verweis auf das Audio-README).

Entfernt:

- `assets/minigolf/music/` (MIDI-Dateien und README)
- `ui/3d/minigolf/music/`
- `scripts/build-minigolf-music.mjs`

### 4.2 Module

- `minigolf-audio-engine.js`: Kontext, Audiograph, Stimmen, Samples und Hall (Abschnitt 4.3).
- `minigolf-audio-synth.js`: reine Hilfsfunktionen. Der Kontext wird übergeben (Abschnitt 4.4).
- `minigolf-audio-music.js`: Musikwiedergabe, Crossfade und Ducking (Abschnitt 4.5).
- `minigolf-audio.js`: Fassade. Das Spiel spricht nur mit ihr (Abschnitt 5).

### 4.3 Engine (`minigolf-audio-engine.js`)

Signatur: `createEngine({ assetURL, onError, random = Math.random, AudioContextClass })`.

- Vor `ensureContext()` existiert kein `AudioContext`. Der Kontext entsteht mit `{ latencyHint: 'balanced' }`.
- Audiograph: Stimmen gehen in den Bus `effects`, Ambiente in den Bus `ambience`, UI-Töne in den Bus `ui`, Musik in den Bus `music`. Die Busse `effects`, `ambience` und `ui` speisen den Gain `fx` (Effektlautstärke). Der Bus `music` geht über den Gain der Musiklautstärke. `fx` und der Musik-Gain münden in den Gain `master` (0,9). Danach folgt der Limiter (`DynamicsCompressorNode` mit threshold −3, knee 0, ratio 20, attack 0,003, release 0,12) und das Ziel.
- Hall: `effects` und `ambience` speisen zusätzlich den Gain `reverbSend` → `ConvolverNode` → `master`.
- `start()` ruft `ctx.resume()` auf und liefert `true` nur, wenn der Zustand danach `running` ist.
- `suspend()` und `resume()` umschließen den Kontext. Fehler gehen an `onError`.
- `setMusicVolume(v)` und `setEffectsVolume(v)` setzen per `setTargetAtTime(v, now, 0.03)`. Werte, die vor dem Kontext gesetzt werden, bleiben erhalten und werden bei der Erzeugung angewendet.
- `async loadSamples(sfxMap)`: holt jede Datei über `assetURL('/3d/minigolf/audio/' + path)` mit `cache: 'force-cache'`, lehnt Dateien über 512 KB ab, dekodiert mit `decodeAudioData` bei einer Parallelität von 6 und liefert `{ loaded, failed }`. Fehler werden je Datei einmal gemeldet, nie geworfen.
- `fetchBuffer(path, maxBytes)`: gemeinsamer Helfer. Die Musik nutzt ihn mit einer Grenze von 4 MiB.
- `play(group, { gain = 1, rate = 1, pan = 0, delay = 0, priority = 0, bus = 'effects' })` liefert eine Stimme oder `null`:
  - Wählt zufällig eine Variante. Dieselbe Variante kommt nie zweimal in Folge, wenn es mehr als eine gibt.
  - Jitter: Rate × (1 ± 0,06), Gain × (1 ± 0,15).
  - Kette: BufferSource → Gain → StereoPanner → Bus.
  - Cooldown von 30 ms je Gruppe. Innerhalb des Cooldowns wird übersprungen.
  - Höchstens 20 Stimmen. Ist die Grenze erreicht, wird die älteste Stimme mit `priority` ≤ der neuen verdrängt (Ausblendung 15 ms). Gibt es keine solche Stimme, wird übersprungen.
  - Stimmen werden beim Ereignis `ended` entfernt.
- `loop(group, { bus = 'effects' })` liefert `{ source, setRate(v), setGain(v), setCutoff(hz), setPan(v), stop(fade = 0.12) }`. Kette: BufferSource mit `loop = true` → BiquadFilter (Tiefpass) → Gain → StereoPanner → Bus. Die Setter nutzen `setTargetAtTime(…, 0.06)`.
- `stopVoices()` stoppt alle Einzelstimmen mit 20 ms Ausblendung.
- `setReverb(buffer, wet)`: Der Send-Gain geht auf 0 (Zeitkonstante 0,05). Nach 0,25 s wird `convolver.buffer` getauscht und der Send auf `wet` zurückgesetzt (Zeitkonstante 0,2). Der erste Aufruf setzt direkt.
- `activeVoices()`, `sampleState()` (liefert `{ loaded, failed }`), `context()`, `now()` und `dispose()` (stoppt alles und ruft `ctx.close()` auf).

### 4.4 Synth-Helfer (`minigolf-audio-synth.js`)

- `mulberry32(seed)`: PRNG. `noiseBuffer(ctx, seconds, color = 'white'|'brown', seed)`: Rauschpuffer.
- `REVERB_PRESETS`:

| ID | Dauer | Wet | Charakter |
|---|---|---|---|
| `menu` | 0,6 s | 0,06 | — |
| `pirate` | 1,0 s | 0,12 | Dämpfung hoch (hell, offen) |
| `temple` | 2,2 s | 0,24 | dunkler Stein |
| `moon` | 1,5 s | 0,18 | metallische Flatterechos mit 12 ms |

- `createImpulse(ctx, id)`: liefert einen stereofonen `AudioBuffer`. Seed-gesteuertes Rauschen × exponentieller Abfall (−60 dB nach `seconds`), progressive Einpol-Tiefpass-Dämpfung und einige Early-Reflection-Abgriffe. Deterministisch je `id`: gleiche `id` ergibt identische Samples.
- `playUi(ctx, destination, kind)`:
  - `click`: Sinus 1,6 kHz, 25 ms, plus 8 ms Rauschtick.
  - `confirm`: zwei Pieptöne, 660 Hz → 990 Hz.
  - `back`: zwei Pieptöne, 880 Hz → 587 Hz.
  - Kurze Gain-Hüllkurven. Die Knoten trennen sich beim Ereignis `ended`.
- `createAmbience(ctx, destination, world, { random })` liefert `{ start(), stop(fade = 0.6) }`:
  - `pirate`: Wellenbett aus 4 s Brown-Noise als Schleife → Tiefpass 650 Hz → Gain, der per LFO (OscillatorNode, 0,09 Hz) moduliert wird. Möwen alle 6–14 s, 2–3 Schreie (Sägezahn → Bandpass 1,6 kHz; Glide 1500 → 2100 → 1300 Hz über 0,35 s).
  - `temple`: Insektenbett aus White-Noise als Schleife → Bandpass 5,2 kHz, Q 8 → Gain, der per LFO mit 38 Hz amplitudenmoduliert wird (Zikade). Vögel alle 4–11 s, 2–5 Sinuspfiffe von 70–160 ms mit Glide 2,4 → 3,6 kHz.
  - `moon`: Brummbett aus Sinus 55 Hz + Sinus 110,4 Hz + Dreieck 165 Hz → Tiefpass 400 Hz. Piepen alle 5–15 s, 2–3 Sinuspiepen mit 1320/1760 Hz, je 60 ms, mit 90 ms Pausen.
  - Das Bett blendet in 1,2 s ein. Ereignisse laufen über Timer, die `stop()` löscht. `stop()` blendet aus und stoppt und trennt danach alle Knoten.

### 4.5 Musik (`minigolf-audio-music.js`)

Signatur: `createMusic(engine, { onTrack, onError })`.

- `setTracks(list)` übernimmt die Liste `music` aus dem Manifest.
- `select(id)`: ohne Wirkung, wenn `id` bereits aktuell oder ausstehend ist. Lädt den Puffer über `engine.fetchBuffer` und dekodiert ihn. Cache als Map von `id` auf Promise, höchstens zwei dekodierte Titel. Einträge, die weder aktuell noch Ziel sind, werden verworfen. Ein Generationszähler verwirft veraltete Ladevorgänge.
- Start: BufferSource mit `loop = true` über einen eigenen Gain (linear von 0 auf 1 in 1,8 s) in einen Duck-Gain und von dort in den Bus `music`. Der alte Titel wird in 1,8 s auf 0 gefahren und bei +1,9 s gestoppt.
- `onTrack({ id, title, author })` wird beim Start eines Titels aufgerufen.
- `next()` schaltet als manuelle Übersteuerung reihum durch die vier Titel, bis ein `select` für einen anderen Kontext kommt.
- `duck(seconds, level)`: Duck-Gain `setTargetAtTime(level, now, 0.05)`, danach `setTargetAtTime(1, now + seconds, 0.35)`. Ein neues Ducking hebt ein früheres auf.
- `state()` liefert `{ playing, trackId, track (Titel), trackIndex, position }`. `position` sind die Sekunden in der Schleife, gemessen ab der Kontextzeit.
- `stop()` stoppt die Quellen.

### 4.6 Laufzeit-Manifest

`ui/3d/minigolf/audio/manifest.json` (≤ 128 KB, `version` 1):

```json
{
  "version": 1,
  "sfx": { "impact.wood.light": ["sfx/impact-wood-light-1.ogg", "..."], "...": [] },
  "music": [ { "id": "menu", "file": "music/menu.ogg", "title": "Happy Clappy Loop", "author": "OwlishMedia", "seconds": 17.44 } ],
  "files": [ { "path": "sfx/impact-wood-light-1.ogg", "bytes": 0, "sha256": "…", "seconds": 0.26, "source": "kenney-impact", "member": "Audio/impactWood_light_000.ogg", "license": "CC0-1.0" } ],
  "sources": [ { "id": "kenney-impact", "title": "Impact Sounds", "author": "Kenney", "page": "…", "download": "…", "file": "kenney_impact-sounds.zip", "bytes": 800850, "sha256": "…", "license": "CC0-1.0", "licenseUrl": "https://creativecommons.org/publicdomain/zero/1.0/" } ]
}
```

- Die Laufzeit nutzt nur `sfx` und `music`. `files` und `sources` dienen der Herkunft.
- Pfade sind relativ zu `/3d/minigolf/audio/` und müssen `^(sfx|music)/[a-z0-9-]+\.ogg$` entsprechen.

### 4.7 Build-Skript (`scripts/build-minigolf-audio.mjs`)

`--check` (Standard, nur lesend, auch von den Tests genutzt):

- Manifest existiert und ist gültiges JSON.
- Jeder Eintrag in `files[]` existiert. Byte-Größe und SHA-256 stimmen. Die Datei beginnt mit `OggS` und enthält den Vorbis-Identifikations-Header `\x01vorbis`.
- Lizenz ist `CC0-1.0`.
- `source` verweist auf einen Eintrag in `sources[]` mit https-URLs für `page` und `download`.
- Jeder Pfad in `sfx` und `music` steht in `files`.
- Das Manifest stimmt mit `sources.json` überein: Quellen, Dateiliste, `sfx`-Zuordnung und `music`-Liste.
- Keine zusätzlichen `.ogg`-Dateien in `ui/3d/minigolf/audio/`.
- Budgets: Summe der SFX-Bytes ≤ 2 MiB, Summe der Musik-Bytes ≤ 6 MiB, Manifest ≤ 128 KB.
- Bei einem Fehler: Exit 1 mit klarer Meldung.

`--write --sources <dir>`:

- Für jeden Eintrag in `sources.json` muss die Datei in `<dir>` liegen und die SHA-256 muss passen. Andernfalls bricht der Lauf ab und gibt die erwartete URL aus.
- ZIP und 7z werden mit `tar -xf` in ein frisches temporäres Verzeichnis entpackt (Windows: `%SystemRoot%\System32\tar.exe`, sonst `bsdtar`). Mitglieder werden nur innerhalb des Entpackverzeichnisses aufgelöst. `..` und absolute Pfade werden abgelehnt.
- Rezeptoptionen in `sources.json`: `start`, `end`, `crossfade`, `fadeIn`, `fadeOut {start, duration}`, `trimSilence`, `quality`.
- Führende Stille wird mit `silenceremove` entfernt (Erkennung bei −50 dB Peak, 2 ms bleiben erhalten). Ausnahme: die beiden Rollschleifen mit `trimSilence: false`.
- Je Ausgabe: Dekodierung mit ffmpeg unter `-fflags +bitexact -flags:a +bitexact`, dann Zuschnitt, Fades und Schleifen-Bake als Filtergraph. SFX mono, Musik stereo, `-ar 44100`.
- Gain: SFX-Peak auf −1 dBFS. Der Peak wird mit `astats` auf der Float-Stufe gemessen, weil `volumedetect` bei 0 dBFS abschneidet. Quellen mit Peaks von +1,5 bis +3,1 dBFS würden sonst zu niedrig gemessen. Musik: integrierte Lautheit mit `ebur128` messen und mit `volume=` so anpassen, dass das Ergebnis −16 LUFS erreicht. Der Gain wird verringert, falls der Peak dabei −1 dBFS überschreiten würde.
- Kodierung: `-c:a libvorbis -q:a <q>` mit `-map_metadata -1`.
- `--write` kodiert zuerst in einen temporären Ordner. Die ausgelieferten Dateien und das Manifest werden erst ersetzt, wenn jede Datei erfolgreich entstanden ist. Das Manifest hat stabile Schlüsselreihenfolge, JSON mit 2 Leerzeichen und einen abschließenden Zeilenumbruch.
- Deterministisch für eine ffmpeg-Version: Ein erneuter Lauf erzeugt identische Bytes.

## 5. Verhalten

### 5.1 Fassaden-API (`minigolf-audio.js`)

Exporte:

- `createAudio`
- `AUDIO_MANIFEST_PATH = '/3d/minigolf/audio/manifest.json'`
- `WORLD_ALIAS = { garden: 'temple', coast: 'pirate', night: 'moon' }`. Entfällt nach P3.
- `REQUIRED_GROUPS`: alle Gruppen aus Abschnitt 3.3.
- `AUDIO_CREDITS`: Titel- und Urheberliste. Enthalten sind die vier Musiktitel (Happy Clappy Loop, Pirate Adventure, Hidden Temple, Galactic Temple) sowie „Impact Sounds · Sci-Fi Sounds · Music Jingles“ (Kenney), „Bowling Ball Rolling“ (qubodup), „30 CC0 SFX Loops“ (rubberduck), „6 Short Water Splashes“ (ezwa) und „Cannon Fire“ (Thimras).

Konstruktor: kein Kontext und kein fetch. Standardlautstärken: Musik 0,2, Effekte 0,65.

- `unlock()`: eine einzige laufende Promise. Ruft `ensureContext` und `start` auf und liefert `false`, wenn der Kontext nicht `running` ist. In diesem Fall wird der Fehler gemeldet, es findet kein fetch statt. Danach wird das Manifest geladen (≤ 128 KB, `version === 1`, gültige Pfade). Wird die Fassade während des Ladens disposed, liefert `unlock` `false`. Anschließend: `music.setTracks`, `music.select` mit der aktuellen Musik-ID, Hall für diese Musik anwenden, Ambiente starten, falls eine Welt gesetzt ist, und `engine.loadSamples(...)` im Hintergrund, ohne darauf zu warten. Liefert `true`.
- `setMusic(context)`: `'menu'` oder eine Welt-ID (mit Alias). Unbekannte Werte werden zu `'menu'`. Die gewünschte ID wird gespeichert. Ist die Fassade entsperrt, folgen `music.select(id)` und `engine.setReverb(createImpulse(ctx, id), REVERB_PRESETS[id].wet)`. Impulse werden je ID gecacht.
- `setAmbient(world|null)`: mit Alias. Dieselbe Welt bewirkt nichts. Sonst wird das alte Ambiente gestoppt und das neue gestartet, sobald entsperrt, nicht pausiert und Effekte > 0 sind.
- `effect(type, opts)`: liefert `false`, wenn disposed, pausiert oder die Effektlautstärke 0 ist. Vor dem Entsperren: `unlock().then(ready => ready && play)`, und der Aufruf liefert `true` wie bisher. Typen siehe Abschnitt 5.2. Alle Typen akzeptieren `pan`.
- `rolling(speed, surface, pan)`: siehe Abschnitt 5.3.
- `ui(action)`: siehe Abschnitt 5.4.
- `pause()`: setzt den Zustand auf pausiert. Stoppt Stimmen, Rollschleifen und Ambiente und ruft `engine.suspend()` auf. Liefert die Position.
- `resume()`: Ist die Fassade nicht entsperrt, wird `paused = false` gesetzt und `false` geliefert, ohne Kontext und ohne fetch. Sonst `engine.resume()`, Ambiente neu starten und zurückgeben, ob die Musik spielt.
- `nextTrack()`, `setMusicVolume`, `setEffectsVolume` (0 stoppt das Ambiente, größer 0 startet es neu) und `dispose()` (idempotent; verwirft späte Ladevorgänge; schließt den Kontext).
- `getState()` liefert `{ unlocked, playing, paused, disposed, trackIndex, track, trackId, position, musicVolume, effectsVolume, activeEffects, rollingSpeed, ambient, music, samples: { loaded, failed }, contextState }`.

### 5.2 Effekte

- `strike` (`speed` 0–12): Gruppe `strike` mit Gain 0,35 + 0,65·min(1, speed/12) und Rate 1 + speed/60. Zusätzlich eine Schicht `cup.rim` mit Gain × 0,15 und Rate 1,6.
- `impact` (`speed`, `surface`, `pan`): Die Materialien `wood`, `stone` und `metal` gelten für Banden. `grass` und `sand` werden zu `soft`. Unbekannte Werte werden zu `wood`. Ist `speed` kleiner als 1,2, entfällt der Effekt. Stufe: unter 2,5 `light`, unter 6 `medium`, sonst `heavy`. Gruppe `impact.<Material>.<Stufe>`, Fallback auf `impact.<Material>.light` bzw. `impact.<Material>`. Bei `soft` wird `medium` auf `light` abgebildet. Gain: 0,2 + speed/10, begrenzt auf den Bereich 0,2 bis 1. Bei `stone` ist die Rate 1,08 (light), 1 (medium) bzw. 0,86 (heavy).
- `cup`: Gruppe `cup.rim` mit Gain 0,55. Danach 2–4 Treffer `cup.rattle` mit kumulativen Abständen von 70–130 ms, Gain 0,45 · 0,72^i und Rate 1,15–1,35. Anschließend `impact.soft.light` mit Gain 0,5, 90 ms nach dem letzten Klappern. Außerdem `music.duck(1.4, 0.35)`.
- `splash` (`speed`): Gain 0,45 + 0,4·min(1, speed/6).
- `cannon`: Gruppe `cannon` mit Gain 0,9 und `cannon.boom` mit Gain 0,6. Außerdem `music.duck(1.2, 0.4)`.
- `boost` (`speed`): Gain 0,55, Rate 1 + speed/40.
- `teleport`: Gain 0,6.
- `win`: Gruppe `win.<aktuelle Welt oder pirate>` mit Gain 0,8 auf dem Bus `effects`. Außerdem `music.duck(3, 0.25)`. Die Fassade merkt sich den zuletzt gesetzten Welt-Musikkontext. So spielt `effect('win')` auch dann den Jingle der Welt, wenn zuvor `setMusic('menu')` gesetzt wurde.

### 5.3 Rollgeräusch

`rolling(speed, surface, pan)`:

- Ist `speed` kleiner als 0,15, werden die Rollschleifen in 0,12 s ausgeblendet. Rückgabe `false`.
- Sonst existieren beide Schleifen `roll.soft` und `roll.hard`. Sie werden lazy erzeugt und gemeinsam gestoppt.
- Profil je Untergrund (Schleife, Rate, Gain, Cutoff in Hz):

| Untergrund | Schleife | Rate | Gain | Cutoff (Hz) |
|---|---|---|---|---|
| `grass` | soft | 1 | 0,55 | 2600 |
| `sand` | soft | 0,8 | 0,7 | 1500 |
| `wood` | hard | 1 | 0,5 | 5000 |
| `stone` | hard | 1,1 | 0,5 | 6500 |
| `metal` | hard | 1,25 | 0,45 | 8000 |

Unbekannte Untergründe werden zu `grass`.

- `n = min(1, speed/14)`. Aktive Schleife: Rate = Rate · (0,55 + 0,9n), Gain = Gain · (0,15 + 0,85·√n), Cutoff = Cutoff · (0,45 + 0,55n). Die andere Schleife bekommt Gain 0.
- Rückgabe `true`. `rollingSpeed` wird im Zustand gehalten.

### 5.4 UI-Töne

`ui(action)` spielt nur, wenn die Fassade entsperrt, nicht pausiert und die Effektlautstärke größer 0 ist.

- `start`, `resume` und `next-hole` spielen `confirm`.
- `close`, `menu` und `pause` spielen `back`.
- Alle anderen Aktionen spielen `click`.
- Drosselung: 40 ms.

### 5.5 Ambiente

- `setAmbient(world)` wählt das Ambiente der Welt (Abschnitt 4.4). Es startet nur, wenn die Fassade entsperrt, nicht pausiert und die Effektlautstärke größer 0 ist.
- Ein Weltwechsel stoppt das alte Ambiente, bevor das neue startet.

### 5.6 Hall

- Jede Welt hat eine Impulsantwort aus `createImpulse` mit den Werten aus Abschnitt 4.4.
- `setMusic` wendet den Hall für die gewählte ID an, mit dem `wet`-Wert des Presets.
- Der Hall wirkt über den Send auf Effekte und Ambiente. Der Wechsel erfolgt sanft nach Abschnitt 4.3 (`setReverb`).

### 5.7 Musik: Crossfade und Ducking

- Titelwechsel: Crossfade von 1,8 s (Abschnitt 4.5).
- Ducking: Loch (`cup`) senkt die Musik für 1,4 s auf 0,35, Kanone (`cannon`) für 1,2 s auf 0,4, Sieg (`win`) für 3 s auf 0,25.
- Zuordnung: `setMusic` wählt zwischen den Titeln `menu`, `pirate`, `temple` und `moon`.

### 5.8 Pause und Fortsetzen

- `pause()` stoppt alle Klänge und das Ambiente, pausiert den Kontext und liefert die Position.
- `resume()` setzt den Kontext fort und startet das Ambiente neu. Vor dem Entsperren liefert es `false`, ohne Kontext und ohne fetch.

### 5.9 Weltalias

`WORLD_ALIAS` gilt in `setMusic` und `setAmbient`: `garden` → `temple`, `coast` → `pirate`, `night` → `moon`. Der Alias entfällt nach P3.

## 6. Spielanbindung

`ui/js/desktop/apps/minigolf-game.js` erhält drei kleine Stellen plus den Import von `AUDIO_CREDITS`. Die Datei wird außerdem von P1, P3 und P4 mitgenutzt.

- `syncAmbient()`: zusätzlich zu `audio.setAmbient(world)` wird `audio.setMusic(panelKind === 'menu' || !round || round.completed || !course ? 'menu' : course.world);` aufgerufen.
- `click()`: `audio.ui(button.dataset.action)` wird vor der Aktion aufgerufen.
- Credits: Die fest eingetragene MIDI-Liste (`tracks`) wird durch `AUDIO_CREDITS` ersetzt. Der Import von `AUDIO_CREDITS` gehört zu dieser Änderung; insgesamt sind es drei kleine Stellen plus der Import.

Modul-Loader: `ui/js/desktop/core/module-loader.js`, Zeile 24, lädt für Minigolf nur noch `/js/desktop/apps/minigolf.js`.

Locales: Der Schlüssel `minigolf.credits_body` steht in allen 16 Dateien `ui/lang/desktop/*.json`. Die englische Fassung lautet:

> Original Blender artwork for AuraGo. Music and recorded sound effects are published under CC0; ambience and interface sounds are synthesized locally.

Die deutsche Fassung lautet:

> Originale Blender-Grafik für AuraGo. Musik und aufgenommene Soundeffekte stehen unter CC0; Ambiente und Bedienklänge werden lokal synthetisiert.

Die übrigen 14 Locales werden in der jeweiligen Sprache formuliert, nicht aus dem Englischen kopiert.

## 7. Tests

### Node-Tests

- `scripts/test-minigolf-audio.mjs` (neu geschrieben) ruft `build-minigolf-audio.mjs --check` auf. Außerdem prüft es:
  - `REQUIRED_GROUPS`: alle Gruppen aus Abschnitt 3.3 sind vorhanden.
  - Musik-IDs sind genau `menu`, `pirate`, `temple`, `moon`.
  - Die Urheber in `AUDIO_CREDITS` passen zu den Manifest-Quellen.
- Lebenszyklus mit Fake-Web-Audio und Fake-fetch, der die echten Manifest- und Byte-Dateien ausliefert:
  - Vor der Geste gibt es weder Kontext noch fetch.
  - Falsches Entsperren (`resume` lässt den Kontext auf `suspended`) liefert `false` und löst kein fetch aus.
  - Entsperren fordert das Manifest über `assetURL` an und startet die Menü-Schleife.
  - `setMusic('coast')` führt zum Crossfade auf `pirate`: Die neue Quelle startet, die alte wird gestoppt.
  - Holz-Aufprall bei Geschwindigkeit 8 spielt `impact.wood.heavy`, Rate innerhalb ±6 %, Gain innerhalb der Jitter-Grenzen.
  - Keine unmittelbare Wiederholung einer Variante.
  - Stimmen-Grenze: 20.
  - Rollen: Start und Stopp, die Rate steigt mit dem Tempo, ein Untergrundwechsel schaltet die Schleifen um.
  - `setAmbient('night')` startet das Mond-Ambiente. `pause` ruft `suspend` auf und stoppt Stimmen, Schleifen und Ambiente, danach liefert `effect` `false`. `resume` ruft `resume` auf.
  - `ui('start')` startet einen Oszillator.
  - `win` plant das Ducking auf 0,25.
  - `dispose` schließt den Kontext. Danach liefern `unlock`, `effect` und `nextTrack` `false`.
  - `dispose` während des Manifest-Ladens: `unlock` liefert `false`, und es entsteht keine Musikquelle.
- Synth: `createImpulse` ist deterministisch und klingt ab (Energie der letzten 10 % unter 1 % der ersten 10 %) für alle vier IDs.
- `scripts/test-synth-studio-audio.mjs` und `npm run test:synth-studio` (verschobene Synth-Studio-Prüfungen).

### Go-Tests

- `ui/desktop_minigolf_test.go`: Die Asset-Liste nutzt `3d/minigolf/audio/manifest.json`. Der Modul-Loader-Eintrag für Minigolf darf `synth-studio` nicht enthalten.

### Browser-Smoke (`AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run TestDesktopMinigolfBrowser`)

- Die Fixture enthält keine Synth-Studio-Skripte und kein `armIndependentPlayerTest`.
- `renderMusicSamples` wird durch `decodeAudioSamples` ersetzt. Jede Musik-OGG wird mit einem `OfflineAudioContext` dekodiert und geprüft:
  - Dauer = Manifest-Sekunden ± 0,05 s,
  - RMS > 0,01,
  - Anteil nahe Vollaussteuerung ≤ 0,001,
  - Sprung an der Schleifennaht (|letzter − erster| Sample je Kanal) < 0,1.
- Nach dem Entsperren gilt `audio.samples.failed === 0` und `loaded` entspricht der Anzahl der SFX-Dateien. Der Test wartet darauf.
- `tick`-Prüfungen liefern `position`.
- Entfernt: der Handler `/__minigolf-render/` und die Prüfung unabhängiger Player.
- Die Underrun-Prüfungen bleiben unverändert und haben keine Toleranz. Der Test ist unter starker Last einmal gescheitert. Vor einer Regressionsvermutung wird daher einmal wiederholt.
- Keine Anfrage verlässt den Test-Origin.

## 8. Abnahmekriterien

- Innerhalb der Testgrenzen aus Abschnitt 1 (SFX ≤ 2 MiB, Musik ≤ 6 MiB, Manifest ≤ 128 KB); gemessen siehe Abschnitt 3.2.
- Kein Underrun im 20-s-Browser-Test.
- Stummschaltung und Lautstärken bleiben gespeichert.
- Keine externen Ressourcen.
- Die Hörabnahme durch den Nutzer bleibt ein separater manueller Schritt. Die Zuordnung der Aufprallklänge und der Schlag könnten Feintuning brauchen. Der Synth-Fallback ist nach der Risikotabelle des Fahrplans erlaubt.

## 9. Risiken

- Es gibt keine golfspezifischen CC0-Putter- oder Lochaufnahmen außerhalb von Freesound, und dort ist ein Login nötig. Deshalb entsteht der Klang aus Kenney-Impacts. Nach dem Hören wird nachjustiert.
- Dekodierter Musikspeicher: höchstens zwei Titel gleichzeitig (ca. 7–40 MB je Titel) während des Crossfades.
- Safari spielt Ogg Vorbis erst ab Safari 18.4. AuraGo-Desktop zielt auf Chromium und Firefox.
- Geteilte Dateien mit parallelen Sitzungen für P2, P3 und P4: `minigolf-game.js` (drei kleine Stellen plus der Import von `AUDIO_CREDITS`), die Locales (ein Schlüssel) und das Bundle (nach einem Rebase neu bauen).
