# fan_4 — Fan, 4-pin PWM / Lüfter, 4-polig PWM

Kind: free official specification
Pins: 4; types: ground 1, power 1, signal 2

## View
Written for the fan's cable plug, looking at its mating face (`drawn: plug`), with the guide rails on top (`key: top`). Left to right: 1, 2, 3, 4.

Basis: Intel's specification, read from the drawings in third-angle projection.
- Figure 6 "Connector Housing": the view of the four square openings has the two rails on top; the rails span the three left positions and the right one is outside them. The view below it marks "PIN ONE" with a triangle and the digit 1 at the left. In third-angle projection the horizontal direction stays the same between these two views, so pin 1 is the left opening. The fourth contact is outside the rails, which is why a 3-pin plug fits positions 1 to 3.
- Figure 7 "Baseboard Connector": the top view of the header has the wall on top over the three right pins, and the 3-D view marks PIN 1 at the right end. So the header reads 4 3 2 1 with the wall on top, the mirror image of the plug, as it must be.
- Boards confirm it: ASRock draws "4 3 2 1" with the wall on top, and Gigabyte draws pin 1 at the right end with the wall on top.

## Sources
1. Intel, "4-Wire Pulse Width Modulation (PWM) Controlled Fans Specification", Revision 1.2, July 2004, copy at https://www.mikrocontroller.net/attachment/670442/4-Wire_PWM_Controlled_Fans.pdf — table 1 pins and colours, section 2 tachometer and PWM values, section 3.3 no signal means full speed, section 4.1.5 housing Molex 47054-1000 and header 47053-1000, figures 6 and 7, accessed 2026-11-04
2. Noctua, FAQ "What pin configuration do Noctua fans use?", https://www.noctua.at/en/support/faqs/what-pin-configuration-do-noctua-fans-use — alt text of the illustration for 12 V fans: 4-pin Blue PWM, Green RPM, Yellow +12V, Black ground, accessed 2026-11-04
3. Gigabyte, B450M K motherboard user manual, https://download.gigabyte.com/FileList/Manual/mb_manual_b450m-k_e.pdf — page 17: pin table and drawing of the header, accessed 2026-11-04
4. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — pages 30 and 31: pin table and drawing of the header, accessed 2026-11-04

## Cross-check
Intel table 1, Noctua and both boards agree on pins. Intel and Noctua agree on colours.

The copy read is Revision 1.2 (July 2004), a copy of Intel's PDF. A Revision 1.3 (September 2005) exists, but I could only see it on Scribd and did not read it. Table 1 did not change as far as secondary mentions show, but I have not verified this.

Boards name pin 2 differently: Gigabyte "Voltage Speed Control", ASRock "FAN_VOLTAGE_CONTROL". This is the same pin; the note says that some boards vary it.

## Choices
- Names `GND`, `12 V`, `Sense`, `Control` exactly as in Intel's table 1.
- Sense and Control are both `signal`, as the catalog schema names TACH and PWM.
- `key: top` is centred by the outline, although the real rails span pins 1 to 3.
- Noctua's chromax fans have all black wires; not noted, as `wire` is the standard colour.
