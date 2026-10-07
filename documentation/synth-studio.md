# Synth Studio

Synth Studio is AuraGo Desktop's compact, browser-rendered music editor. Open it
from the Creative app category. Sound is produced on the computer running the
browser; the AuraGo server stores project files through the existing Desktop
file API. No cloud music service or audio sidecar is required.

## Compose

- Choose an instrument or drag an editable pattern onto the arrangement.
- Use the five banks of ten synthesizer presets: Drums, Bass, Leads, Pads and
  Keys/Plucks. The additional General MIDI bank contains 128 synthesized sounds
  and a percussion kit. These are compact synthesized approximations, not a
  sampled acoustic orchestra.
- Double-click an empty track lane to create a clip. Select a clip to edit notes
  in the piano roll or percussion grid. Drag notes to move them and their right
  edge to change duration. Shift-click selects multiple notes; Delete removes
  the selection. The velocity slider changes the selected notes' strength.
- Drag clips to move them, Alt-drag to copy, and drag their right edge to crop
  or extend. Shift-drag that edge repeats their notes into the extended clip.
  Ctrl/Cmd+D duplicates the selected clip directly after itself.
- Track controls provide volume, mute, solo and recording selection. The sound
  panel provides instrument choice, pan, tone, attack, release, reverb and delay.
  Compact windows expose that panel and the library through toolbar buttons.
- Space plays/pauses, Ctrl/Cmd+S saves, Shift+Ctrl/Cmd+S saves a copy, and
  Ctrl/Cmd+Z / Shift+Ctrl/Cmd+Z undo/redo. The computer-keyboard row A–K plays
  one octave; W, E, T, Y and U supply accidentals.

A project uses 4/4, a constant 40–240 BPM tempo and 480 ticks per quarter note.
There are at most fifteen melodic tracks and one percussion track. The loop
range uses bar positions; the end is exclusive. The demo supplies a starting
arrangement. Browser-tab hiding pauses playback; minimizing only the Desktop
window keeps playback running. Reopening never starts playback automatically.

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
the actual synthesizer sound and effects.

## Save and export

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
50,000 note/controller events and bounded finite tick values. Unknown schema
fields and unsupported project versions are rejected. Audio recording, audio
clips, plug-ins, automation lanes and external MIDI output are outside this
version's scope.

## Maintenance and validation

The lazily loaded `synth-studio-*` modules separate the validated project/MIDI
format, native Web Audio renderer, Desktop persistence and editor. Factory sound
data and the MIDI parser are vendored with licenses under
`ui/js/vendor/synth-studio/`; no runtime CDN is used.

Run `node scripts/test-synth-studio-model.js`,
`node scripts/test-synth-studio-recording.mjs`,
`node scripts/test-synth-studio-storage.mjs`,
`node scripts/test-synth-studio-registration.mjs`, UI bundle
generation/checks, and focused Desktop/server tests after changes. Browser
checks require `AURAGO_RUN_BROWSER_SMOKE=1`; `AURAGO_BROWSER_ARTIFACT_DIR` collects
visual evidence. Real MIDI hardware latency, operating-system permission prompts
and audio output on target devices require separate acceptance testing.
