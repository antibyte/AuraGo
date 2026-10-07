# Synth Studio vendor manifest

## General MIDI timbre data

`gm-data.js` contains only the names and oscillator-parameter tables adapted
from `webaudio-tinysynth` 1.1.4 by Tatsuya Shinyagaito. The package metadata in
the upstream repository declares Apache-2.0. The exact upstream files used for
the table are:

- `https://raw.githubusercontent.com/g200kg/webaudio-tinysynth/master/package.json`
- `https://raw.githubusercontent.com/g200kg/webaudio-tinysynth/master/webaudio-tinysynth.js`

The source arrays are `program` plus `program1` for the 128 melodic patches,
and `drummap` plus `drummap1` for the 47 percussion keys from note 35 through
81. Each generated entry pairs the upstream display name with its matching
oscillator array; unset drum timbres remain `null`. AuraGo's renderer implements
the oscillator, envelope, frequency-routing, noise, and effects behavior with
native Web Audio. No TinySynth runtime, MIDI scheduler, voice limiter, or
upstream executable code is included.

## MIDI parser bundle

`tone-midi-2.0.28.bundle.js` is the pinned `@tonejs/midi` 2.0.28 browser bundle.
Its locked `midi-file` and `array-flatten` dependencies and licenses are listed
in `README.md` and the adjacent license files.

## SHA-256

These digests pin the checked-in data and parser bundle:

```text
337bcf21bada601ac6cea88c3f8aa8bd798186564247ad91a5822d9e62caffe9  ui/js/vendor/synth-studio/gm-data.js
9165a2b3de9d378eca970b80618c3747c60ed8188b12b0f059ccf44fc6a07cc0  ui/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js
```

To verify from the repository root on PowerShell:

```powershell
Get-FileHash ui/js/vendor/synth-studio/gm-data.js -Algorithm SHA256
Get-FileHash ui/js/vendor/synth-studio/tone-midi-2.0.28.bundle.js -Algorithm SHA256
```
