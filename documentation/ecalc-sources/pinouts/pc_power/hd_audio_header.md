# hd_audio_header — HD Audio front panel header, 2x5 / HD-Audio-Frontpanel-Anschluss, 2x5

Kind: free official specification (Intel FPIO 2004) and Intel's own board document, cross-checked with two board makers
Pins: 9 listed on 10 positions; types: signal 8, ground 1

## View
Written for the header on the board, seen from above (pins pointing at the reader), pin 1 at the bottom left, odd pins in the bottom row (`drawn: socket`). In the `pc_power` convention (`fan_4` and the others: `plug` = cable end, `socket` = board header) this is the `socket` view, so record and group agree. Rows (top to bottom, each left to right):
- top row: 2, 4, 6, gap at position 8, 10
- bottom row: 1, 3, 5, 7, 9

Position 8 has no pin: a gap (`null`) in the top row, and the pin list has no entry for 8. The cable plug (looking into its holes; the hole at 8 is blocked) is the mirror image: 10, gap, 6, 4, 2 over 9, 7, 5, 3, 1; the catalog derives it.

Basis (read):
- Intel FPIO 2004 figure 3 "Front Panel Audio Header (Top View)" (page 21) and Intel FPIO 2018 figure 2-2 (page 17), the same drawing: 9 and 1 at the ends of the top row, 10 and 2 at the bottom row, pin 1 square top right, no pin at position 8 (second from the left in the bottom row).
- Intel FPIO 2004 figure 4 "Intel HD Audio - Analog Front Panel Header" (page 22): odd pins in the left column, even pins in the right column, no pin 8; the top view turned 90 degrees, with the HD Audio signal names at each pin.
- MSI JAUD1 drawing (page 35): 2 ... 10 above, 1 ... 9 below, pin 1 square bottom left, gap at 8. ASRock HD_AUDIO1 drawing (page 30): pin 1 marked bottom left. Both are Intel's top view turned by 180 degrees.

## Sources
1. Intel, "Front Panel I/O Connectivity Design Guide", Version 1.2, July 2004, A29286-004, copy at https://ftpmirror.infania.net/sites/intel/Motherboards/Workstation/Intel%20OR840/Docs/datasheets/A29286-004_frontPanelIO.pdf — section 2.3.5, table 7 "Intel HD Audio Front Panel Analog Header Signal Names" and figure 4 (page 22); accessed 2026-11-24
2. Intel, "Intel Desktop Board DX79SI Technical Product Specification", section 2.2.2.1, table 13 "Front Panel Audio Header for Intel HD Audio" and table 14 (AC'97), read in the copy at https://www.manualslib.com/manual/440790/Intel-Dx79si.html?page=50; accessed 2026-11-24
3. Intel, "Front Panel I/O Connectivity Design Guide", document 600569, Revision 1.1, July 2018, https://cdrdv2-public.intel.com/600569/600569-fpio-dg-rev1p1.pdf — figure 2-2 header layout (page 17), table 2-5 (AC'97 signals only, page 18); accessed 2026-11-24
4. MSI, B450 TOMAHAWK user guide, https://download-2.msi.com/archive/mnu_exe/mb/B450TOMAHAWK.pdf — page 35, "JAUD1: Front Audio Connector": 1 MIC L, 2 Ground, 3 MIC R, 4 NC, 5 Head Phone R, 6 MIC Detection, 7 SENSE_SEND, 8 No Pin, 9 Head Phone L, 10 Head Phone Detection; accessed 2026-11-24
5. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — page 30 (PDF page 36), "Front Panel Audio Header (9-pin HD_AUDIO1)": drawing with names and the AC'97 note; accessed 2026-11-24

## Cross-check
Intel FPIO 2004 / Intel DX79SI / MSI / ASRock:
- 1: PORT 1L / PORT_1L / MIC L / MIC2_L — agree
- 2: GND in all four — agree
- 3: PORT 1R / PORT_1R / MIC R / MIC2_R — agree
- 4: PRESENCE# / PRESENCE# / NC / PRESENCE# — MSI leaves it unconnected; same position
- 5: PORT 2R / PORT_2R / Head Phone R / OUT2_R — agree
- 6: SENSE1_RETURN / SENSE1_RETURN / MIC Detection / MIC_RET — agree
- 7: SENSE_SEND / SENSE_SEND / SENSE_SEND / J_SENSE — agree
- 8: KEY / KEY "No pin" / No Pin / empty — agree
- 9: PORT 2L / PORT_2L / Head Phone L / OUT2_L — agree
- 10: SENSE2_RETURN / SENSE2_RETURN / Head Phone Detection / OUT_RET — agree

The 2018 Intel FPIO revision covers only the AC'97 pinout; the HD Audio table is in the 2004 version and in Intel's board documents.

## Choices
- Catalog item: `rows` with two rows of five, `outline: rect`, `contact: square`, `drawn: socket`, no `key`; position 8 is a gap, the other nine are pins.
- Names as Intel's DX79SI TPS writes them (`PORT_1L` with underscore; FPIO 2004 writes "PORT 1L"). The functions say microphone and headphone.
- All audio and detection lines `signal`, pin 2 `ground`. Pin 4 PRESENCE#: Intel's meaning, with the MSI NC variant in the pin note.
- AC'97 only in the item note, not as a variant: the Intel sources disagree (FPIO 2018 table 2-5: 4 AUD_VCC, 6 AUD_RET_R, 7 HP_ON, 10 AUD_RET_L; FPIO 2004 table 6: 4 AUD_GND, 6 FP_RETURN_R, 7 AUD_5V, 10 FP_RETURN_L; DX79SI table 14: 4 PRESENCE#, 6 and 10 AUD_GND, 7 Reserved). Pins 1, 2, 3, 5, 8 and 9 agree in all three.
- No `wire`: no source names colours.

## Spot-check list
- Pin 1 = PORT 1L, analog port 1 left (microphone): Intel FPIO 2004 table 7, page 22.
- Pin 4 = PRESENCE#, active low: Intel FPIO 2004 table 7, page 22; DX79SI table 13.
- Pin 7 = SENSE_SEND: Intel FPIO 2004 table 7, page 22; MSI page 35.
- Pin 9 = PORT 2L (headphone), MSI "Head Phone L": Intel FPIO 2004 table 7; MSI page 35.
- Pin 8 = key, no pin: Intel FPIO 2004 table 7 and figure 4, page 22.
- Pin 1 at the top right of the top view: Intel FPIO 2018 figure 2-2, page 17.

UNVERIFIED: none for HD Audio. The AC'97 assignment is unresolved between Intel documents, which is why it is not a variant.
