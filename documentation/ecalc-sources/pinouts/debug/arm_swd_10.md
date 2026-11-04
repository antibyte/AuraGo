# arm_swd_10 — Cortex Debug / SWD (10-pin) / Cortex Debug / SWD (10-polig)

Kind: authoritative maker document
Pins: 10; types: power 1, ground 3, signal 1, data 4, nc 1

## View
Written for the header on the board, looking down at its pins (the mating face): `drawn: plug`, 5 rows of 2, odd pins left, pin 1 top left, notch `left`.

Basis, seen in the figures (no text in these documents says "top view"):
- Arm, Cortex-M Debug Connectors, page 2: diagram with "1 2" on top and "9 10" at the bottom, odd pins left, no contact at KEY, the outline interrupted on the left side between pins 3 and 7. It stands next to a photo of the header on a board.
- Keil figure concortex10pindesc.png: same order and an empty position 7 (this figure draws no notch).
- Keil photo concortex10pin.png: the ribbon socket is shown from its mating face with the bump on top and the red wire on the left. Mirrored for the header seen from above and turned so that the bump goes to the left, this puts the red-wire end at the top, in line with pin 1 top left, assuming that the red wire marks pin 1 (Microchip states that for its own cable). The photo does not show which row of that end is pin 1.

## Sources
1. Arm, Cortex-M Debug Connectors, https://documentation-service.arm.com/static/5fce6c49e167456a35b36af1 — page 2, diagram. The document says that it is for general information and not a specification. Accessed 2026-11-04.
2. ARM DSTREAM System and Interface Design Reference Guide, ARM 100956_0527_00_en (DS-5 5.27), section 2.7, table 2-12 (pinout) and table 2-13 (signals), read in the copy at https://www.manualslib.com/manual/2490558/Arm-Ds-5.html?page=40 and page=41. Accessed 2026-11-04.
3. Keil ULINKpro User's Guide, Connectors, https://www.keil.com/support/man/docs/ulinkpro/ulinkpro_cs_connectors.asp with figures concortex10pindesc.png and concortex10pin.png. Accessed 2026-11-04.
4. Microchip SAM E54 Xplained Pro User's Guide, section 5.2.8, table 5-10, https://onlinedocs.microchip.com/oxy/GUID-AA358083-AEED-4BA8-8511-9F986D3390A5-en-US-2/GUID-6AEFC2EC-DBA5-4292-8134-E311C8CE37CD.html — the same connector on a real board. Accessed 2026-11-04.

5. SEGGER Knowledge Base, Debug Connectors, https://kb.segger.com/Connectors — remark on variants without pin 7. Accessed 2026-11-04.

Not reached: the CoreSight Components Technical Reference Manual, appendix C, which Arm names as the detailed specification. Arm's pages load by script and the fetch tool got no text.

## Cross-check
All sources agree on the position of every signal. Differences in naming:
- Pin 1: VCC (Arm's Cortex-M document, Microchip), VTREF (ARM 100956), VTref (Keil). Chose VTref: table 2-13 defines it as an input of the probe.
- Pin 4: SWDCLK (Arm's Cortex-M document, Keil), SWCLK (ARM 100956, Microchip). Chose SWCLK and named SWDCLK in the function.
- Pin 6: Keil also lists EXTa and TRACECTL. Left out, they belong to trace setups.
- Pin 8: NC/TDI (Arm's Cortex-M document, Microchip), TDI (ARM 100956), NC/EXTb/TDI (Keil). Chose NC/TDI.
- Pin 9: GNDDetect (Arm's Cortex-M document, Keil, Microchip), GND (ARM 100956). Chose GNDDetect with `type: ground`, the note names the difference. Only Microchip explains the name ("Ground detection").
- Pin 10: nRESET (Arm's Cortex-M document, Keil, Microchip), nSRST (ARM 100956). Chose nRESET.
- Pin 7: a keying position whose pin must be absent on the target's connector (ARM 100956, table 2-13, read through a summary of that page), no pin (Keil). SEGGER's connector page says that variants with pin 7 removed can also be used, which suggests that headers with a pin 7 exist in practice.

## Choices
- Pin 7 appears as a pin with `type: nc` and not as a `null` position, as the assignment asks and because every source lists it as position 7.
- `key: left` rests on the gap in Arm's drawing alone (a drawing, and the document is not a specification). The Keil photo agrees but cannot settle it. The key field is the least certain part of this item; the pin positions are not in doubt.
- "Active low" is not stated for pin 10 because only the name suggests it here and ARM 100956 says that polarity is configurable.
- 0.05 inch is written as 1,27 mm in German, the usual metric value of that pitch.
