# Goldfish Adventure: YuE2 Studio song templates

Prompts for generating one original song per room of **Bitte nicht das Universum
füttern** with YuE2 Studio ("Create music", tab **Studio**: Style + Lyrics → full song).
The templates are source material only. Nothing here is wired into the game yet; the
runtime keeps playing the eight Earshot chamber-score loops listed in
`../audio-manifest.json` until generated takes are chosen, mastered and registered.

## Files

| File | Rooms |
| --- | --- |
| `yue2-act1.md` | R01 Schrottplatz, R02 Amt, R03 Café, R04 Startpier |
| `yue2-act2.md` | R05 Schiff, R06 Fracht, R07 Zoll, R08 Versprechensmarkt |
| `yue2-act3.md` | R09 Arboretum, R10 Wetterstation, R11 Depot, R12 Reparaturhalle |
| `yue2-act4.md` | R13 ALLGUT-Empfang, R14 Tribunal, R15 Kern, R16 Observatorium |

Every template has: **Title**, **Style** (vocal song), **Style (instrumental)**,
**Score header** (ABC, optional), **Lyrics** and a short note on the scene.

## Key, tempo and meter of the current loops

The existing score loops are in **3/4** and each has a fixed key (derived from
`../build-audio.py`, `THEMES`). The templates keep each room close to its loop's key and
tempo so a swap does not change the pace of play. The sung songs use 4/4 where the genre
wants it (marked in each score header).

| Loop | Key | BPM | Rooms |
| --- | --- | --- | --- |
| second-chances | G minor | 96 | R01 |
| paper-moons | A major | 112 | R02, R03, R07 |
| borrowed-stars | G minor | 76 | R04, R05, R06 |
| unkept-promises | C minor | 104 | R08 |
| alma | F Dorian | 72 | R09, R12 |
| weather-permission | G minor | 88 | R10 |
| a-useful-no | E Phrygian | 80 | R11, R13, R14, R15 |
| the-next-question | G major | 84 | R16 |

## Using a template in the Studio tab

1. Paste **Title** into the "Untitled" field and **Style** into the style box.
2. Paste **Lyrics** into the lyrics box. The section tags (`[Intro]`, `[Verse 1]`,
   `[Chorus]`, `[Verse 2]`, `[Outro]`) are the Studio's own tag chips.
3. **Score (ABC) box** (below the lyrics): keep the mode on "Melody + chords". Two ways:
   - *Free:* leave the box empty. The model composes the score and shows it afterwards.
   - *Pinned:* paste the template's **Score header** (`X:`, `M:`, `L:`, `Q:`, `K:` lines
     only, no notes) to hold key, meter and tempo of the room's loop. If the result
     sounds stiff, clear the box and use *Free*.
   Recommended flow: press **Compose the score** first (no singing, quick), fix the
   melody in the score editor if you like, then create the song. The score comes back
   with every track, so you can keep a good composition and re-perform it.
4. Leave "Instrumental only" off for the sung version.
   For a **loop-friendly score** tick "Instrumental only" and use **Style (instrumental)**
   instead (the lyrics are then ignored).
5. If the Studio shows a duration limit: 100 s for the sung version, 60 s for a loop.
   The model spends about 25 semantic tokens per second of requested duration, and a
   long Style/Lyrics text shrinks that budget. If the console says the budget ran out
   before the end token, raise the duration or shorten the text.
6. Generate several takes per template (different seeds) and audition them. Keep the
   Style line as one line: the Studio reads it word for word.
7. Optional: save the Style as a Studio style preset (save icon in the Style card) so
   all rooms of an act share one palette.

## Conventions used in every template

- **Style order** follows the Studio's own example: language, genre/mood, lead voice,
  instruments, tempo. Musical instructions live in Style, never in Lyrics.
- **Lyrics** contain only sung words and section tags. The sung language is English:
  YuE2's documented singing languages are English, Chinese and Japanese, German is not
  documented. Instrumental versions are language-free and therefore fit every locale.
- **Palette** matches the existing score: celesta, marimba, plucked strings, felt piano,
  harp, soft bowed strings, brushed drums. No artist, band or franchise names in Style
  and no borrowed melodies or lines. The lyrics are original.
- **Tempo** stays close to the room's current loop (see the table in each act file) so a
  swap does not change the pace of play.
- The songs spoil nothing the game does not already show by the room's own act; R15/R16
  stay neutral about which of the three endings was chosen.

## Using generated audio

1. Save the chosen take as WAV under `../masters/audio/yue2/<room>-<title>-<take>.wav`
   (masters are not shipped with the UI package).
2. Record model, version, date, settings and the generator's licence terms in
   `../audio-notices.txt` before the file is used anywhere. Do not ship audio whose
   licence is unclear.
3. Mastering/looping (target peak -8 dB like the current loops) and the switch of
   `room audio` in `../art-layout.json` are a separate step; the runtime plays one
   `audio` path per room and loops it.
4. Listen to every candidate in the game context. Automated checks do not establish
   that a generated song is musically good.
