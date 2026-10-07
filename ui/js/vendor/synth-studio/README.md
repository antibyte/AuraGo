# Synth Studio MIDI vendor

`tone-midi-2.0.28.bundle.js` is the upstream browser bundle from `@tonejs/midi` 2.0.28 (MIT), including its locked `midi-file` 1.2.4 and `array-flatten` 3.0.0 dependencies. See the adjacent MIT license files.

MIDI import supports Standard MIDI Type 0 and Type 1 with one constant tempo and constant 4/4 timing. Export is Type 1. General MIDI patches keep their number; Synth Studio presets use these approximate export patches: bass 38, leads 81, pads 89, keys 4. Drum tracks use percussion channel 10 and retain their note numbers. These patch choices are compatibility hints because the Synth Studio sounds are generated locally.
