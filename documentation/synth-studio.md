# Synth Studio

Synth Studio is AuraGo Desktop's browser-rendered creative instrument. Open it
from the Creative app category. Sound is produced on the computer running the
browser; the AuraGo server stores project files through the existing Desktop
file API. No cloud music service or audio sidecar is required.

Planned expansions — new sounds, a rebuilt demo, a parameterised effect chain and
a UI refresh — were tracked in `documentation/synth-studio-plan.md` and are now
implemented. That plan remains the record of the intended scope.

## Compose

- Choose an instrument or drag an editable pattern onto the arrangement.
- Use the five banks of twenty-four synthesizer presets: Drums, Bass, Leads, Pads and
  Keys/Plucks — 120 factory sounds in total. The additional General MIDI bank contains
  128 synthesized sounds and a percussion kit. These are compact synthesized
  approximations, not a sampled acoustic orchestra.
- Program your own instrument with the built-in synthesizer (see
  "The programmable synthesizer"). Saved synths form the bank "My synths", appear in
  the instrument lists like a factory bank and stay in this browser.
- Every factory sound carries character tags. The library filters by tag and keeps
  favourites and a recent list in the browser. The "Synth Studio Kit" covers the
  47 percussion notes from 35 to 81 with one coherent voice. The separate
  "GM Drums" instrument follows the General MIDI percussion map instead.
- Double-click an empty track lane to create a clip. Select a clip to edit notes
  in the piano roll or percussion grid. Drag notes to move them and their right
  edge to change duration. Shift-click selects multiple notes; Delete removes
  the selection. The velocity slider changes the selected notes' strength.
- Drag clips to move them, Alt-drag to copy, and drag their right edge to crop
  or extend. Shift-drag that edge repeats their notes into the extended clip.
  Ctrl/Cmd+D duplicates the selected clip directly after itself.
- Track controls provide volume, mute, solo and recording selection. Every track
  head carries a track menu that duplicates the track or deletes it together with
  its clips; deletion asks for confirmation and moves the selection to a track
  that still exists. The sound
  panel provides instrument choice, pan, tone, resonance, drive, attack and release,
  the two effect sends (reverb and delay) and the effect chain.
- Space plays/pauses, Ctrl/Cmd+S saves, Shift+Ctrl/Cmd+S saves a copy, and
  Ctrl/Cmd+Z / Shift+Ctrl/Cmd+Z undo/redo. The computer-keyboard row A–K plays
  one octave; W, E, T, Y and U supply accidentals.

A project uses 4/4, a constant 40–240 BPM tempo and 480 ticks per quarter note.
There are at most fifteen melodic tracks and one percussion track. The loop
range uses bar positions; the end is exclusive. The demo supplies a starting
arrangement. Browser-tab hiding pauses playback; minimizing only the Desktop
window keeps playback running. Reopening never starts playback automatically.

## Instrument workspace

The transport stays visible while Notes, Sound and Performance share a resizable workbench.
Panel height, the selected workbench, library visibility and the panel sizes are
local view preferences. Dark and light
desktop schemes use graphite and aluminium surfaces respectively. On a window of at least
1400 pixels the arrangement keeps the full width of the workbench and the notes panel
and the sound workbench sit side by side below it, so the timeline stays readable
while the instrument and its effect chain stay in view. The lower band always keeps
the height the user gave it: the sound panel scrolls inside its own column instead
of being squeezed into an unreadable strip. A guided tour in the footer
walks through the five workbench areas and can be skipped at any point. Electronic,
Chill and Retro templates start paused; each is a distinct sixteen-bar
composition with its own instruments, groove, harmony and mix.

Three draggable separators size the studio: the library rail on the left, the sound
column on the right and the workbench band under the arrangement. Every separator is
a real `separator` role: the arrow keys move it in steps, Shift multiplies the step
and a double click restores the default size. The sizes are stored per browser and
applied again on the next start.

The library separates selection, preview and insertion and searches all banks
until a category is selected. Search matches names and tags; the tag chips
filter the list, and favourites plus a recent list are kept per browser. The
note editor offers draw/select tools, rectangle
selection, copy/paste, per-selected-note velocity and draggable bar-loop handles.
Keyboard arrows move loop handles by one bar. Control values are displayed and can
be reset. Dragging a sound or volume control previews it immediately and commits
one undo step; Escape or pointer cancellation restores the previous sound.

## The programmable synthesizer

The sound panel of a melodic track offers "Design sound". It turns the panel
into a full synthesizer over the patch model of the voice engine: oscillators
(waveform, level, unison, detune, a second oscillator, pulse width), sub
oscillator and noise, three FM modulators with ratio, depth and decay, the
filter (type, cutoff, resonance, envelope, decay), the amplitude envelope,
the pitch sweep and an LFO with target, waveform, rate, depth and start delay.
Every change immediately re-registers a runtime preview instrument, so the
audition keys always play the current draft. Track parameters (tone, resonance,
drive, attack, release, sends) and the effect chain keep shaping the sound on
top of the patch, exactly as with a factory preset.

A design started from a track saves as a new synth and assigns it to that
track as one undoable project step; starting from a GM program or a factory
preset uses its patch as the starting draft, and a design opened from the
"edit" pencil on a "My synths" entry edits that synth without touching any
track. Updating a stored synth that a track already plays lets that track
hear the draft on its next notes without saving. The reset action restores
the pristine starting draft; save, save-as, delete and rename go through the
regular Desktop dialogs and notice system, and stay disabled in read-only
mode.

Saved synths are browser-local patches — at most twenty-four, each clamped to
the engine's parameter vocabulary before it reaches audio or storage —
registered into the preset catalog on every app start. Projects only store
the `user-*` instrument id, so the project format is unchanged, and a
project from another browser opens anyway: a missing synth validates by its
id, keeps its track, plays a plain fallback wave and stays selectable in the
instrument list as "Missing synth" until it is designed again there. MIDI
export approximates user synths with a General MIDI program, WAV export
renders their actual sound, and the drum kits (one patch per note) are
deliberately not editable through this surface.

## Colour and visual identity

The app owns a spectrum of eight hues (`--ss-color-0` … `--ss-color-7`). Each hue
is mixed with the current text colour, so one token set stays legible on the dark
and on the light desktop scheme. The spectrum is applied consistently rather than
decoratively: track colours and the matching clip headers, the arrangement track
names, the piano-roll spine, library category and tag chips, the eight scene and
chord pads, the workbench tab underline, the effect-insert markers and the master
strip group headings each take one hue from it.

Piano-roll notes carry their own pitch hue, so a chord reads as a colour block and
a percussion lane as a row of distinct colours. Notes darken on the light scheme to
keep their contrast against the lighter grid. The arrangement itself keeps a neutral
grid, so the clip colours stay the only saturated element on the timeline.

The app accent (`--ss-accent`) is the mint signal colour of the playhead, focus
rings and analyser; the Desktop accent stays reserved for the surrounding shell.
The output meter and the analyser read the `--ss-meter` token and therefore follow
the active scheme. On windows of at most 540 pixels the email-export button
collapses to an envelope glyph so no toolbar action is pushed out of reach.

Mix changes preserve the audio clock. Structural edits are queued at the next
bar, including requests inside the audio scheduler's look-ahead window. Reserved
notes from an obsolete request are cancelled before that boundary. A queued
notice remains until the change is audible. Stop discards pending changes.
Live editing does not recreate AudioContext.

## MIDI keyboard and files

Open the MIDI panel and explicitly connect a keyboard. Web MIDI requires a
supporting browser and a secure context (HTTPS or a browser-recognized local
origin). A missing device or denied permission does not block mouse editing.
Only MIDI input is requested; SysEx and MIDI output are not used.

Select the recording track, enable the optional one-bar count-in, and record.
Quantization follows the current grid. The latency offset, from -200 to +200 ms,
compensates for the input device and browser; calibrate it with the actual
keyboard. Sustain, pitch bend and modulation are recorded with notes. Stopping,
disconnecting the device, losing the browser focus while recording, and closing
the app release active input notes.

A recording take is limited to 64 bars. Reaching the project event, file-size
or timeline limit finishes the notes already captured; split longer performances
into successive clips.

Import accepts bounded Standard MIDI type 0 and type 1 files with a constant
supported tempo and 4/4 meter. Unsupported controller messages, changing tempo
or meter, type 2, and SMPTE timing produce an error instead of silently changing
the arrangement. MIDI exports retain notes and supported controllers, but use
General MIDI approximations for the custom Synth Studio presets. Use WAV for
the actual synthesizer sound, effects and tone/reverb automation. The MIDI export
notice explains that sound automation remains in WAV.

## Musical tools

The Performance workbench provides eight diatonic chord pads, major/natural-minor
scale assistance, a live/printable arpeggiator, seeded melodic/rhythmic variations
and a Euclidean drum generator. Hold pads or use the screen/typing/MIDI keyboard
for live arpeggios; the latch switch keeps the last chord running after release
and starts a new one on the next key. The arpeggiator order also offers chord
stabs and strummed chords, with the same note output printed or played live.
Choose a clip, prepare a preview, compare
Original/Variation in a loop, then Apply or Cancel. Apply writes ordinary notes
as one undo step. The visible seed, input notes and controls determine repeatable
results. Scale constraints affect new notes only; fitting existing notes requires
an explicit generator preview and Apply. New projects default to C major with
constraints off.

The Groove lab quantizes, swings, humanizes and transposes an existing clip with
one deterministic, seeded pass. The chord-progression generator writes a whole
diatonic progression (pop, ballad, jazz, minor, epic or canon) into the clip, and
the bassline generator follows the same progression with root, octave, walking or
driving-eighth patterns. Reverse clip mirrors the selected clip in time. All four
use the same preview, comparison and one-step Apply flow as the other tools.

## Scenes, automation and performance takes

Create up to eight named scenes from the current loop boundaries. A scene repeats
that arrangement range; pressing another pad queues it for the next bar.
Pressing more pads replaces that pending choice. Text and pad states distinguish
active, queued and stopped scenes. Stop is immediate and clears pending actions.
At a scene boundary, held and sustained notes enter their release phase, sustain
and controller state are reset/chased for the new range, and existing effect tails
continue through the track buses. Notes already sounding before a target range
are not retriggered.

The XY pad controls the selected track's tone horizontally and reverb vertically.
The two labelled sliders provide the same functions with the keyboard. Enable
Record automation before a gesture during playback to write tone/reverb points;
one gesture is one undo step. Capture is limited to 20 samples per second plus
gesture endpoints and reduces redundant points. Edit automation exposes individual
points, bar positions and values, with Apply/Delete and undo. The automatable
parameters are tone, resonance, drive, reverb and delay. Clear automation
removes the selected track's curve. Without recording, a manual gesture temporarily
overrides an existing curve, which resumes on release.

Record performance captures scene choices and XY gestures in a temporary take.
The remaining bar count reflects the smaller of 64 bars and the existing WAV,
event, clip and project-size budgets. At the limit, recording finishes cleanly and
keeps the material already captured. Stop also creates a browser-local recovery
draft. A failed draft write is shown and prevents silently closing through the
normal window guard. Recovery waits for the source project's initial load.

Adopt as new project creates an unsaved, linear project with editable clips and
automation; the source file is preserved. Scene repeats become separate clips,
sustain becomes note duration, and pitch bend/modulation are retained. Save the
new project through the usual picker. A new recording or cancellation can discard
the temporary take explicitly. These takes capture arrangement playback and sound
gestures, not microphone/audio input.

Live playback and WAV use the same event preparation and native
[AudioParam scheduling](https://developer.mozilla.org/en-US/docs/Web/API/AudioParam),
with smoothed parameter changes. The compact output meter and clipping indicator
are visual feedback only; musical timing does not depend on their refresh rate.

## Save and export

New projects use format v3: project-level harmony (root, major/minor, snap), up to
eight named bar-range scenes, track-level automation points using arrangement ticks,
a per-track effect chain and a project master strip. Validation still accepts v1
and v2 unchanged. The editor upgrades a copy in memory; the first v3 save requires
a new filename in the regular picker.
Autosave stays suspended until that copy succeeds, protecting the original file.
Draft recovery and conditional writes continue to apply.

The export menu offers WAV, MIDI and stems. Stems render one post-fader,
pre-master WAV per track with notes, then package them as a store-only ZIP with
UTF-8 names. Tracks without notes are skipped; the master level, EQ, compression,
limiting and width are neutralized while the project's plate/echo character and
post-fader sends are preserved, so the stems can be mixed elsewhere.

## Effect chain and master strip

Every track carries an ordered chain of up to four effect inserts plus two sends.
The available inserts are chorus, phaser, tremolo, flanger, rotary, bitcrush,
soft clip, three-band EQ, stereo width, cabinet, delay, reverb, tape, auto pan
and ring mod. Tape adds wow/flutter and soft saturation, auto pan moves the
signal across the stereo field, and ring mod blends in a carrier-modulated
metallic layer. Each insert
exposes its own character parameters — mix, rate, depth, feedback, tone and
type-specific values — instead of a single fixed sound. The chain runs after the
track pan, so stereo effects work on a real stereo signal, and the reverb and
delay sends leave the fader, so the track volume also controls its own tails.

The Performance workbench also carries two momentary Live FX after the master
strip: Filter drop sweeps the master low-pass down and back over 1.5 beats, and
Stutter gates the mix sixteen times over one bar. They follow the current tempo,
are performance-only, and never enter the project, WAV mixdown or stem export.

The Master button in the transport opens the master strip: output level, the
compressor with threshold, ratio, knee, attack and release, a three-band EQ, a
brick-wall limiter, stereo width and the character of the shared plate and echo
buses. The echo time follows
the project tempo when a division is selected, so a one-eighth delay stays in
time when the tempo changes. The analyser in the transport shows the master
output and is visual feedback only; musical timing does not depend on its
refresh rate.

The sound engine gained a per-voice LFO that can target pitch, amplitude, the
voice filter or the pulse width, continuous pulse-width modulation, selectable
filter types, a bipolar filter envelope that can close the filter, pink and brown
noise and a third FM modulator.

Projects are versioned `.aurasynth` JSON files in Desktop storage; the default
folder is `Documents/Synth Studio`. The regular
Desktop file picker, conditional writes and conflict dialog are reused. Editing
creates local recovery drafts; named projects autosave. A failed save leaves
the document dirty and retains a recoverable draft. Closing offers recovery
when a remote save cannot complete. Drafts remain browser-local and do not
replace an explicit project save. Read-only Desktop mode permits opening and
playing files but disables editing, recording and saving.

WAV export downloads stereo 44.1 kHz PCM16 using the same renderer as playback,
including per-track effects. It renders one finite arrangement pass and adds
an effect tail; the transport loop and metronome are not exported. To bound
browser memory and processing, the complete render including the tail is
limited to five minutes, 12,000 notes and 256 overlapping voices. Shorten or
simplify the arrangement if this limit is reached.

Project validation limits input to 5 MiB, sixteen tracks, 256 clips per track,
50,000 notes/controllers/automation points and bounded finite tick values. Unknown schema
fields and unsupported project versions are rejected. Audio recording, audio
clips, plug-ins and external MIDI output are outside this
version's scope.

## Maintenance and validation

The lazily loaded `synth-studio-*` modules separate the validated project/MIDI
format, the effect rack, native Web Audio renderer, stem packaging, Desktop
persistence and editor. The effect rack lives in `synth-studio-fx.js` and owns
the insert processors, the master strip, the Live FX nodes and the send buses;
the schema that validates an insert lives in `synth-studio-model.js`. Stem
isolation, CRC-32 and the store-only ZIP writer live in
`synth-studio-export.js`. The programmable synthesizer lives in
`synth-studio-designer.js`: it normalizes drafts against the voice-engine
bounds, stores the browser-local user patch bank, registers it into
`synth-studio-presets.js` (visible instruments plus a runtime-only preview
entry) and owns the editor surface. Factory sound
data and the MIDI parser are vendored with licenses under
`ui/js/vendor/synth-studio/`; no runtime CDN is used.

Run `node scripts/test-synth-studio-model.js`,
`node scripts/test-synth-studio-audio.mjs`,
`node scripts/test-synth-studio-export.mjs`,
`node scripts/test-synth-studio-designer.mjs`,
`node scripts/test-synth-studio-recording.mjs`,
`node scripts/test-synth-studio-storage.mjs`,
`node scripts/test-synth-studio-registration.mjs`, UI bundle
generation/checks, and focused Desktop/server tests after changes. Browser
checks require `AURAGO_RUN_BROWSER_SMOKE=1`; `AURAGO_BROWSER_ARTIFACT_DIR` collects
visual evidence. Real MIDI hardware latency, operating-system permission prompts
and audio output on target devices require separate acceptance testing.
