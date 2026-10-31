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
- Track controls provide volume, mute, solo and recording selection. The sound
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
Panel height, the selected workbench and library visibility are local view preferences. Dark and light
desktop schemes use graphite and aluminium surfaces respectively. On a window of at least
1400 pixels the arrangement keeps the full width of the workbench and the notes panel
and the sound workbench sit side by side below it, so the timeline stays readable
while the instrument and its effect chain stay in view. A guided tour in the footer
walks through the five workbench areas and can be skipped at any point. Electronic,
Chill and Retro templates start paused; each is a distinct sixteen-bar
composition with its own instruments, groove, harmony and mix.

The library separates selection, preview and insertion and searches all banks
until a category is selected. Search matches names and tags; the tag chips
filter the list, and favourites plus a recent list are kept per browser. The
note editor offers draw/select tools, rectangle
selection, copy/paste, per-selected-note velocity and draggable bar-loop handles.
Keyboard arrows move loop handles by one bar. Control values are displayed and can
be reset. Dragging a sound or volume control previews it immediately and commits
one undo step; Escape or pointer cancellation restores the previous sound.

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
for live arpeggios. Choose a clip, prepare a preview, compare Original/Variation
in a loop, then Apply or Cancel. Apply writes ordinary notes as one undo step.
The visible seed, input notes and controls determine repeatable results. Scale
constraints affect new notes only; fitting existing notes requires an explicit
generator preview and Apply. New projects default to C major with constraints off.

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

## Effect chain and master strip

Every track carries an ordered chain of up to four effect inserts plus two sends.
The available inserts are chorus, phaser, tremolo, flanger, rotary, bitcrush,
soft clip, three-band EQ, stereo width, cabinet, delay and reverb. Each insert
exposes its own character parameters — mix, rate, depth, feedback, tone and
type-specific values — instead of a single fixed sound. The chain runs after the
track pan, so stereo effects work on a real stereo signal, and the reverb and
delay sends leave the fader, so the track volume also controls its own tails.

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
format, the effect rack, native Web Audio renderer, Desktop persistence and
editor. The effect rack lives in `synth-studio-fx.js` and owns the insert
processors, the master strip and the send buses; the schema that validates an
insert lives in `synth-studio-model.js`. Factory sound
data and the MIDI parser are vendored with licenses under
`ui/js/vendor/synth-studio/`; no runtime CDN is used.

Run `node scripts/test-synth-studio-model.js`,
`node scripts/test-synth-studio-audio.mjs`,
`node scripts/test-synth-studio-recording.mjs`,
`node scripts/test-synth-studio-storage.mjs`,
`node scripts/test-synth-studio-registration.mjs`, UI bundle
generation/checks, and focused Desktop/server tests after changes. Browser
checks require `AURAGO_RUN_BROWSER_SMOKE=1`; `AURAGO_BROWSER_ARTIFACT_DIR` collects
visual evidence. Real MIDI hardware latency, operating-system permission prompts
and audio output on target devices require separate acceptance testing.
