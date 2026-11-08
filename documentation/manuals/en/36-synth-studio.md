# Chapter 36: Synth Studio – a synthesizer and DAW in the Virtual Desktop

Synth Studio is a synthesizer and a small DAW in the Virtual Desktop. Sound is produced on the computer that
runs the browser; the AuraGo server only stores the project files through the regular Desktop file API. No
cloud music service and no audio sidecar is needed.

You arrange tracks and clips, design your own sounds, perform with chord pads and arpeggios, live-code
patterns in Strudel and mix the result. The full behavioural contract is in the [Synth Studio
guide](../../synth-studio.md).

## Table of Contents
1. [Opening Synth Studio](#opening-synth-studio)
2. [The Five Workspace Modes](#the-five-workspace-modes)
3. [The Library Drawer](#the-library-drawer)
4. [Knobs, Faders and Curves](#knobs-faders-and-curves)
5. [The Master Strip](#the-master-strip)
6. [Strudel Live Coding](#strudel-live-coding)
7. [Musical Tools](#musical-tools)
8. [MIDI Keyboard and Files](#midi-keyboard-and-files)
9. [Saving and Exporting](#saving-and-exporting)
10. [When No Sound Comes Out](#when-no-sound-comes-out)

---

## Opening Synth Studio

Open **Synth Studio** from the start menu, in the category **Creative**. The window opens maximized.

The transport sits above the mode bar: play, pause, stop and record, the position readout, the tempo field,
the metronome, the loop range, undo and redo, the **Master** button and the analyser. Space plays and pauses,
Ctrl/Cmd+S saves, and Ctrl/Cmd+Z undoes.

A project uses 4/4 and a constant 40–240 BPM tempo, with at most fifteen melodic tracks and one percussion
track. The **demo** button loads "Neon Sketch", a synthwave song in A minor at 112 BPM over 32 bars; the
Electronic, Chill and Retro templates are sixteen-bar compositions that start paused. Hiding the browser tab
pauses playback; minimizing only the Desktop window keeps it running.

## The Five Workspace Modes

The transport stays visible above a mode bar with five modes; only the active mode is shown.

| Mode | What it is for |
|------|----------------|
| **Arrange** | The arrangement with tracks and clips. The note editor (piano roll or percussion grid) appears under the arrangement only while a clip is selected. |
| **Sound design** | The sound panel of the selected track, including the programmable synthesizer and the effect chain. |
| **Perform** | Chord pads, scale assistance, arpeggiator, variations, scenes and the momentary Live FX. |
| **Strudel** | The live-coding editor. |
| **Mixer** | The master strip with the effect inserts. |

Perform groups its controls into cards: scenes, the XY pad with Tone/Reverb knobs, the take recorder, Live FX,
the automation editor and the composer, one card per generator. Double-click an empty track lane to create a
clip, and select a clip to edit notes in the piano roll or percussion grid.

## The Library Drawer

The sound library is a drawer that slides in from the side. It is closed by default; open and close it with
the library button in the header, and the drawer state is remembered.

- **Factory banks.** Five banks of twenty-four synthesizer presets — Drums, Bass, Leads, Pads and
  Keys/Plucks — 120 factory sounds in total. They are compact synthesized approximations, not a sampled
  acoustic orchestra.
- **General MIDI.** The additional General MIDI bank holds 128 synthesized sounds and a percussion kit.
- **My synths.** Program your own instrument with the built-in synthesizer and it joins the bank "My synths",
  appears in the instrument lists like a factory bank and stays in this browser. At most twenty-four saved
  synths are kept.
- **Search and tags.** The library separates selection, preview and insertion, and searches all banks until
  you pick a category. Search matches names and tags; favourites plus a recent list are kept per browser.
- **The Synth Studio Kit** covers the 47 percussion notes from 35 to 81 with one coherent voice. The separate
  **GM Drums** instrument follows the General MIDI percussion map instead.

## Knobs, Faders and Curves

Range sliders are drawn as 3D knobs, and the track volume as a horizontal fader. Drag a knob up or down to
change it; hold Shift for fine steps. While a knob has keyboard focus the mouse wheel changes it and the arrow
keys move it in steps. A double click triggers the row's Reset button where the row has one.

The envelope section of the sound designer uses four vertical faders (attack, decay, sustain, release) next to
a small envelope shape that previews what you drag. A vertical fader behaves like the other controls: drag or
click the track, mouse wheel and arrow keys while it has focus, and a double click returns it to the patch
default. The designer edits a browser-local draft, so a fader drag changes the draft and the preview but never
creates a project undo step.

The sound panel offers **Design sound**, which turns the panel into a full synthesizer over the patch model of
the voice engine: oscillators, sub oscillator and noise, three FM modulators, the filter, the amplitude
envelope, the pitch sweep and an LFO. Every change re-registers a runtime preview instrument, so the audition
keys always play the current draft.

## The Master Strip

The **Master** button in the transport opens the master strip: output level, the compressor with threshold,
ratio, knee, attack and release, a three-band EQ, a brick-wall limiter, stereo width and the character of the
shared plate and echo buses.

The output level is a large hero knob with a decibel read-out (20 × log10 of the gain, `-inf` at zero) and a
16 segment LED meter beside it (green, amber, red over 48 dB). The meter reads the same analyser peak, runs
only while the strip is open and never alters the audio.

The EQ group shows the live frequency response of the three bands, and the sound designer shows the response
of the voice filter (type, cutoff, resonance) with a cutoff marker. The curves are pure SVG computed from the
same filter types, frequencies and cutoff mapping the audio engine uses.

Every track carries an ordered chain of up to four effect inserts plus two sends. The available inserts are
chorus, phaser, tremolo, flanger, rotary, bitcrush, soft clip, three-band EQ, stereo width, cabinet, delay,
reverb, tape, auto pan and ring mod. Each insert exposes its own character parameters — mix, rate, depth,
feedback, tone and type-specific values — instead of a single fixed sound. The chain runs after the track pan,
and the reverb and delay sends leave the fader, so the track volume also controls its own tails.

## Strudel Live Coding

Switch to the **Strudel** mode to write and run patterns with the local Strudel 1.3.0 engine. **Run / Update**
or Ctrl+Enter evaluates the current code; updates preserve the running clock. Its Stop button and the main
transport's Stop/Pause stop it immediately. Starting the arrangement stops Strudel, and starting Strudel stops
the arrangement. Closing the window, hiding the browser tab or revoking Desktop write access also stops its
evaluator.

The Melody, Groove and Ambient examples use built-in synthesizers, and tutorial patterns such as
`s("bd sd cp hh")` work with local synthesized kick, snare, clap and hi-hat counterparts rather than the online
REPL's samples. No network or sample download is needed. External sample banks (`samples(...)`), MIDI and microphone
access are blocked in the evaluator.

The scratch draft (maximum 64 KiB) is retained in this browser. Open imports a `.strudel` text file after
confirmation; **Download code** saves a portable copy. This code is separate from the arrangement's
`.aurasynth` file, undo history and WAV/MIDI/stem exports, so export it before clearing browser storage. If
autoplay is blocked, click the sound button shown in the workbench. Errors appear above the editor and stop
the evaluator; fixing the code and running it starts a fresh instance. The evaluator has an opaque sandbox
origin and cannot read the Desktop's DOM, storage, credentials or API responses. Strudel and its integration
are AGPL-3.0-or-later.

## Musical Tools

The Perform mode provides eight diatonic chord pads, major/natural-minor scale assistance, a live/printable
arpeggiator, seeded melodic/rhythmic variations and a Euclidean drum generator. Hold pads or use the
screen/typing/MIDI keyboard for live arpeggios; the latch switch keeps the last chord running after release.

Every generator follows the same flow: choose a clip, prepare a preview, compare Original/Variation in a loop,
then Apply or Cancel. Apply writes ordinary notes as one undo step, and the visible seed, input notes and
controls make the result repeatable. Scale constraints affect new notes only; new projects default to C major
with constraints off.

| Tool | What it does |
|------|--------------|
| **Groove lab** | Quantizes, swings, humanizes and transposes an existing clip in one deterministic, seeded pass. |
| **Chord-progression generator** | Writes a whole diatonic progression into the clip: pop, ballad, jazz, minor, epic or canon. |
| **Bassline generator** | Follows the same progression with root, octave, walking or driving-eighth patterns. |
| **Reverse clip** | Mirrors the selected clip in time. |

Live FX sit after the master strip and are momentary: **Filter drop** sweeps the master low-pass down and back
over 1.5 beats, and **Stutter** gates the mix sixteen times over one bar. They follow the current tempo, are
performance-only, and never enter the project, the WAV mixdown or the stem export.

Create up to eight named scenes from the current loop boundaries. A scene repeats that range; pressing another
pad queues it for the next bar, and pressing more pads replaces that pending choice. Stop is immediate and
clears pending actions. The XY pad controls the selected track's tone horizontally and reverb vertically.
Enable Record automation before a gesture during playback to write tone/reverb points; one gesture is one undo
step.

## MIDI Keyboard and Files

Open the MIDI panel and explicitly connect a keyboard. Web MIDI requires a supporting browser and a secure
context (HTTPS or a browser-recognized local origin). A missing device or denied permission does not block
mouse editing, and only MIDI input is requested; SysEx and MIDI output are not used.

Select the recording track, enable the optional one-bar count-in, and record. Quantization follows the current
grid. The latency offset, from -200 to +200 ms, compensates for the input device and browser; calibrate it
with the actual keyboard. A recording take is limited to 64 bars; reaching the project event, file-size or
timeline limit finishes the notes already captured, so split longer performances into successive clips.

Import accepts bounded Standard MIDI type 0 and type 1 files with a constant supported tempo and 4/4 meter.
Unsupported controller messages, changing tempo or meter, type 2, and SMPTE timing produce an error instead of
silently changing the arrangement. MIDI exports use General MIDI approximations for the custom Synth Studio
presets; use WAV for the actual synthesizer sound and tone/reverb automation.

## Saving and Exporting

Projects are versioned `.aurasynth` JSON files in Desktop storage; the default folder is
`Documents/Synth Studio`. New projects use format v3: project-level harmony, up to eight named bar-range
scenes, track-level automation points, a per-track effect chain and a project master strip. Validation still
accepts v1 and v2 unchanged. The editor upgrades a copy in memory; the first v3 save requires a new filename
in the regular picker, so the original file stays protected.

Editing creates local recovery drafts; named projects autosave. A failed save leaves the document dirty and
retains a recoverable draft. Read-only Desktop mode permits opening and playing files but disables editing,
recording and saving.

The export menu offers WAV, MIDI and stems.

- **WAV** downloads stereo 44.1 kHz PCM16 using the same renderer as playback, including per-track effects. It
  renders one finite arrangement pass and adds an effect tail; the transport loop and metronome are not
  exported. The complete render including the tail is limited to five minutes, 12,000 notes and 256
  overlapping voices.
- **Stems** render one post-fader, pre-master WAV per track with notes, then package them as a store-only ZIP
  with UTF-8 names. Tracks without notes are skipped; the master level, EQ, compression, limiting and width
  are neutralized while the project's plate/echo character and post-fader sends are preserved, so the stems
  can be mixed elsewhere.
- **MIDI** exports the arrangement for other software, with General MIDI approximations for your own synths.

## When No Sound Comes Out

| Symptom | What to check |
|---------|---------------|
| Nothing is audible at all | Open the master strip with the **Master** button and read the output level: the hero knob shows the decibel value and reads `-inf` at zero. Raise it. |
| The LED meter stays dark | The meter runs only while the master strip is open, and it reads the analyser peak. Open the strip and check the analyser in the transport. |
| One track is silent | Check the track head: volume, mute and solo are there. The reverb and delay sends leave the fader, so the track volume also controls its own tails. |
| The sound cuts out when you switch to another app | Hiding the browser tab pauses playback; minimizing only the Desktop window keeps it running. |
| The Strudel mode is silent or stops on its own | If autoplay is blocked, click the sound button shown in the workbench. Closing the window, hiding the browser tab or revoking Desktop write access stops the evaluator. |
| The recorded MIDI is late or early | Set the latency offset in the MIDI panel, from -200 to +200 ms, and calibrate it with your keyboard. |
| A long recording stops early | A take is limited to 64 bars, and it also stops at the project event, file-size or timeline limit. Split longer performances into successive clips. |
| The export is silent or cuts off | WAV renders one finite arrangement pass and adds an effect tail; the transport loop and the metronome are not exported. The complete render is limited to five minutes, 12,000 notes and 256 overlapping voices. |
| Nothing can be saved | In read-only Desktop mode, opening and playing work but editing, recording and saving are switched off. |
| Your own synth is missing in another browser | Saved synths are browser-local. A project from another browser opens anyway and keeps its track, but the synth shows as "Missing synth" until you design it again there. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 22: Internal Tools](./22-internal-tools.md) · [Chapter 26: Video Studio](./26-video-studio.md) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 35: Please Do Not Feed the Universe](./35-goldfish-adventure.md)
