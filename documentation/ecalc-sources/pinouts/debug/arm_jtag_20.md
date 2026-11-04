# arm_jtag_20 — ARM JTAG (20-pin) / ARM JTAG (20-polig)

Kind: authoritative maker document
Pins: 20; types: power 1, ground 9, signal 4, data 5, nc 1

## View
Written for the male box header on the board, looking down at its pins (the mating face): `drawn: plug`, 10 rows of 2, odd pins left, pin 1 top left, notch `left`.

Basis, seen in the figures themselves (rendered or downloaded and looked at; no text in these documents says "top view"):
- Arm, Cortex-M Debug Connectors, page 3: diagram with "1 2" on top and "19 20" at the bottom, odd pins left, the outline interrupted on the left side between TMS and TDO. It stands next to a photo of the boxed header on a board labelled JTAG.
- SEGGER figure JTAG_Port.png: same layout, notch on the left at pins 9 and 11. SEGGER calls the part a "keyed box header (2.54mm male)".
- Keil figure conjtag.png: same layout and left gap, beside a photo of the header with its notch on the left.

Derived check of the mirror direction: Microchip's Atmel-ICE guide (page 15) says that pin 1 of its 100-mil socket sits to the right of the keying "when the connector is seen from the cable". Looking at a socket from the cable is the same direction as looking down on the header it plugs into. Rotating this drawing until the notch is on top puts pin 1 to the right of the notch, so the drawing is the view from above and not the mirror image. This sentence is about the AVR cable, so it supports the IDC convention but does not name the ARM connector.

## Sources
1. ARM DSTREAM System and Interface Design Reference Guide, ARM 100956_0527_00_en (DS-5 5.27), section 2.4, table 2-6 (pinout) and table 2-7 (signals), read in the copy at https://www.manualslib.com/manual/2490558/Arm-Ds-5.html?page=33 (also page=34, page=35). Arm's own site served no text to the fetch tool (support.arm.com loads the document by script). Accessed 2026-11-04.
2. Arm, Cortex-M Debug Connectors, https://documentation-service.arm.com/static/5fce6c49e167456a35b36af1 — page 3, 20-pin IDC diagram. The document says that it is for general information and not a specification. Accessed 2026-11-04.
3. SEGGER Knowledge Base, 20-pin J-Link Connector, https://kb.segger.com/20-pin_J-Link_Connector with figure https://kb.segger.com/images/3/3c/JTAG_Port.png — JTAG table and notes. Accessed 2026-11-04.
4. Keil ULINKpro User's Guide, Connectors, https://www.keil.com/support/man/docs/ulinkpro/ulinkpro_cs_connectors.asp with figure conjtag.png in the same folder. Accessed 2026-11-04.
5. Microchip Atmel-ICE User Guide, Atmel-42330C, 10/2016, https://ww1.microchip.com/downloads/en/DeviceDoc/Atmel-ICE_UserGuide.pdf — page 15, keying sentence (view check only). Accessed 2026-11-04.

## Cross-check
Signal pins 1, 3, 5, 7, 9, 11, 13 and 15 agree everywhere apart from spelling. Disagreements:
- Pin 2: ARM 100956 and SEGGER NC; Keil and Arm's Cortex-M document "VCC (optional)". Chose NC (the DSTREAM reference table), the optional VCC is in the pin note.
- Pins 14, 16, 18, 20: ARM 100956, Arm's Cortex-M document and Keil GND; SEGGER reserved ("do not assume them to be connected to GND inside J-Link"). Chose GND as the ARM standard, SEGGER's difference is in the item notes and pin notes.
- Pin 17: ARM 100956 DBGRQ; Arm's Cortex-M document NC/DBGRQ; Keil N/C; SEGGER DBGRQ, not connected in the J-Link. Chose DBGRQ.
- Pin 19: ARM 100956 DBGACK; Arm's Cortex-M document NC/DBACK; Keil N/C; SEGGER 5V-Supply output, max. 300 mA. Chose DBGACK and put a caution in the pin note, because a 5 V output on this pin can damage a target that uses it as a signal.
- Pin 15: nSRST (ARM 100956), nRESET (SEGGER, Arm's Cortex-M document), RESET (Keil). Chose nSRST and named the others in the note.
- Pin 1: VTREF (ARM 100956), VTref (SEGGER), VCC (Arm's Cortex-M document, Keil). Chose SEGGER's spelling VTref; the role is the same.

Unverified: nothing about pin numbers or names. Not read: IEEE 1149.1 itself (it does not define this header).

## Choices
- `key: left` comes from the drawings, not from text. The notch position at pins 9 and 11 is drawn by SEGGER only; the catalog schema's `key` carries the side only.
- `type: power` for VTref: it connects to the target's rail although the probe only senses it. Stated in the pin note.
- TCK, TMS and RTCK are `data` by the catalog schema's rule on bus lines and their clocks; nTRST, nSRST, DBGRQ and DBGACK are `signal`.
- "Normally active low" for the two resets: Arm says that polarity and strength are configurable on DSTREAM but that both lines are pulled high and driven low to reset.
- Pin 2 is `nc` although some targets use it for VCC, as documented in its note.
