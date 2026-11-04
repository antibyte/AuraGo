# audio_trrs — 3.5 mm headset jack (TRRS) / 3,5-mm-Headset-Klinke (TRRS)

Kind: cross-checked reproductions
Pins: 4; variants: ctia, omtp; types in each variant: signal 3, ground 1

## View
The catalog schema's `jack` shape: a plug from the side, tip at the left, segments from tip to sleeve (T, R1, R2, S). One fixed view, no mirror question.

## Sources
1. Android Open Source Project, "3.5 mm headset: Accessory specification", https://source.android.com/docs/core/interaction/accessories/headset/plug-headset-spec — mechanical table: "CTIA pinout order (LRGM)" required, except in regions with legal requirements for OMTP; "OMTP pinout order (LRMG)" optional, accessed 2026-11-04
2. Android Open Source Project, "3.5 mm headset jack: Device specification", https://source.android.com/docs/core/interaction/accessories/headset/jack-headset-spec — same two orders for the jack; the OMTP circuit note says to switch the MIC and GND segments, accessed 2026-11-04
3. Texas Instruments, TS3A225E datasheet, SCDS329A, August 2012, revised May 2013, https://www.ti.com/lit/ds/symlink/ts3a225e.pdf — section "Application Information" and table 2, page 21: Standard (CTIA) Tip Left, Ring 1 Right, Ring 2 Ground, Sleeve Microphone; text: the GND and microphone bands are switched between the two, accessed 2026-11-04
4. Wikipedia, "Phone connector (audio)", https://en.wikipedia.org/wiki/Phone_connector_(audio) — OMTP has ground on the sleeve and the microphone on the second ring; CTIA/AHJ reverses these, accessed 2026-11-04
5. Electronic Specifier, product note on the Fairchild FSA8049, https://www.electronicspecifier.com/products/passives/fsa8049-fairchild-semiconductors-audio-jack-detection-switch — names "L/R/M/G" for OMTP and "L/R/G/M" for CTIA, accessed 2026-11-04

## Cross-check
All agree on the swap. **One disagreement inside a source:** TI's table 2 repeats the Standard rows for OMTP ("Ring 2 Audio Right", "Ring 2 Ground", "Sleeve Microphone"). That is a copy error: TI's own text says the bands are switched, and its OMTP band diagram reads G M R L from the sleeve end. I use the diagram and text, in line with Android and Wikipedia. The OMTP spot checks therefore cite Android, Wikipedia and TI's diagram, not TI's rows.

The Android pages give only the letter orders and do not say that they run from tip to sleeve; TI's Standard rows settle that reading.

## Choices
- Variant name `CTIA (AHJ)`: TI calls the same order "Standard", Wikipedia "CTIA/AHJ".
- `signal` for MIC, as the catalog schema's "everything else that is one defined signal".
- No `wire`: no source names colours.
