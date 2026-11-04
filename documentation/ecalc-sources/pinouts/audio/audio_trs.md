# audio_trs — 3.5 mm stereo jack (TRS) / 3,5-mm-Stereoklinke (TRS)

Kind: cross-checked reproductions
Pins: 3; types: signal 2, ground 1

## View
The catalog schema's `jack` shape: a plug from the side, tip at the left, segments from tip to sleeve. It has one fixed view, so no mirror question arises. TI's table 2 draws the same order on a plug photo labelled tip, ring, sleeve.

## Sources
1. Texas Instruments, TS3A225E datasheet, SCDS329A, August 2012, revised May 2013, https://www.ti.com/lit/ds/symlink/ts3a225e.pdf — table 2 "Headset Configuration", page 21: Tip Audio Left, Ring Audio Right, Sleeve Ground, accessed 2026-11-04
2. Rane Corporation, Rane Note 102, "Analog I/O Standards", https://www.ranecommercial.com/legacy/pdf/old/note102.pdf — section on phone jacks: "Tip = Left Positive", "Ring = Right Positive", "Sleeve = Common Ground" for jacks made for headphones, and the balanced wiring tip positive, ring negative, sleeve ground, accessed 2026-11-04
3. Wikipedia, "Phone connector (audio)", https://en.wikipedia.org/wiki/Phone_connector_(audio) — stereo and balanced assignment of tip, ring, sleeve, accessed 2026-11-04

## Cross-check
TI (a chip maker) and Rane (an audio equipment maker) are independent and agree on left, right and ground. Rane writes about 1/4-inch jacks, TI about 3.5 mm jacks; the assignment is the same. No disagreement. No free standard was reached; the convention is reproduced by these sources.

## Choices
- `name` L, R, GND: short forms of what the sources call Audio Left, Audio Right, Ground.
- `signal` for L and R and `ground` for the sleeve, as the catalog schema says.
- No `wire`: no source names colours for a stereo plug.
