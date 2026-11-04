# fan_3 — Fan, 3-pin / Lüfter, 3-polig

Kind: cross-checked reproductions
Pins: 3; types: ground 1, power 1, signal 1

## View
Written for the fan's cable plug, looking at its mating face (`drawn: plug`), with the guide rails on top (`key: top`). Left to right: 1, 2, 3.

Derived from the 4-wire family, as no source draws the 3-pin plug with a stated view:
- Intel's 4-wire specification, figure 6 ("Connector Housing"), and figure 7 ("Baseboard Connector") are read in the 4-wire record. The rails of the plug span positions 1 to 3, so they take the same wall of the header as a 3-pin plug does.
- ASRock B450 Steel Legend: "If you plan to connect a 3-Pin chassis water cooler fan, please connect it to Pin 1-3", drawn on a header reading 4 3 2 1 with the wall on top. Its mirror image is the plug view 1 2 3 (4).

## Sources
1. Noctua, FAQ "What pin configuration do Noctua fans use?", https://www.noctua.at/en/support/faqs/what-pin-configuration-do-noctua-fans-use — alt text of the illustration for 12 V fans: 3-pin Yellow RPM speed signal, Red +12V, Black ground; 4-pin colours; states the same pin assignment as specified by Intel and AMD, accessed 2026-11-04
2. Sunon, "Fan 3rd Wire Signal", in the PMD1212PTB3-A datasheet, copy at https://pdf2.datasheet.su/sunon%20fans/pmd1212ptb3-a.(2).gn.pdf — three lead wires: "+:Red", "-:Black", third wire "F Type : Yellow", "R Type : White"; two to four pulses per rotation by motor, accessed 2026-11-04
3. ASRock, B450 Steel Legend motherboard manual, https://download.asrock.com/Manual/B450%20Steel%20Legend.pdf — pages 30 and 31: pin 1 GND, 2 FAN_VOLTAGE_CONTROL, 3 FAN_SPEED, 3-pin fans on pins 1 to 3, accessed 2026-11-04
4. Intel, "4-Wire Pulse Width Modulation (PWM) Controlled Fans Specification", Revision 1.2, July 2004, copy at https://www.mikrocontroller.net/attachment/670442/4-Wire_PWM_Controlled_Fans.pdf — figures 6 and 7, key and view of the family, accessed 2026-11-04

## Cross-check
Noctua and Sunon are independent fan makers and agree on black, red and yellow (Sunon's F type). Noctua's text is the image description of its page; I read that, not the picture.

Sunon's R type has a white third wire, and on 4-wire fans yellow is +12 V. The notes warn about both.

**UNVERIFIED, left out of the YAML:** search excerpts of Intel desktop board specifications show other 3-pin chassis headers (DP35DP and D945GNT with a control signal on pin 1, DG41BI with Sense on pin 1 and Ground on pin 3). I could not open those pages (403), so they do not count as read. A wiring user with an old Intel board should check that board's manual. The caller may want this followed up.

## Choices
- `RPM` from Noctua's wording. Pin 1 is `GND`, pin 2 `+12V`.
- Pin 2's function mentions the voltage a board varies, as ASRock names the pin FAN_VOLTAGE_CONTROL.
- `key: top` is centred by the outline, although the real rails sit around the three contacts.
