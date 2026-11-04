# qwiic — Qwiic / STEMMA QT / Qwiic / STEMMA QT

Kind: authoritative maker document
Pins: 4; types: power 1, ground 1, data 2

## View
Written for the header on the board: `drawn: plug`, one column, pin 1 on top. A single column looks the same in both views, so only the sequence along the connector is stated, and that is what the sources fix: GND, 3.3V, SDA, SCL in consecutive positions.

Which end of a real connector is pin 1 is not stated by the drawing, on purpose. Both makers' side-entry footprints agree with each other: pad 1 at x = -1.5 mm to pad 4 at x = +1.5 mm, with the signal pads at the rear and the mounting tabs on the -y side (SparkFun JST04_1MM_RA; Adafruit JST_SH4 and JST_SH4_SKINNY). I did not turn this into a left/right claim on the mating face because I could not read JST's drawing to confirm which side the opening is on (see Sources). The connector is polarised, so a user only needs the order when making an adapter, and the wire colours settle it then.

## Sources
1. SparkFun, Qwiic Connect System, https://www.sparkfun.com/qwiic — pinout rule GND / 3.3V / SDA / SCL, colours, "Qwiic only supports 3.3V boards", part numbers SM04B-SRSS-TB and SHR-04V-S. Accessed 2026-11-04.
2. SparkFun, Qwiic cable assembly drawing ACCA-1571, rev 1, 2017-02-23, https://cdn.sparkfun.com/assets/e/9/8/9/8/Qwiic-Cable.pdf — pin assignments, rendered and read. Accessed 2026-11-04.
3. SparkFun Eagle library, https://raw.githubusercontent.com/sparkfun/SparkFun-Eagle-Libraries/master/SparkFun-Connectors.lbr — device QWIIC_CONNECTOR, symbol I2C_STANDARD, packages JST04_1MM_RA and JST04_1MM_VERT. Accessed 2026-11-04.
4. SparkFun, Qwiic Connect System page print, CAB-17261 technical data, https://static.mercateo.com/0e/34f4cafe25e94508b8eeb4d338befcb1/pdf/CAB-17261_eng_tds.pdf — colours in the order black, red, blue, yellow. Accessed 2026-11-04.
5. SparkFun Learn, I2C, https://learn.sparkfun.com/tutorials/i2c/qwiic-connect-system — "1 mm pitch, 4-pin JST", polarised. Accessed 2026-11-04.
6. Adafruit, Adafruit STEMMA & STEMMA QT, Technical Specs, https://learn.adafruit.com/introducing-adafruit-stemma-qt/technical-specs — colours, V+ 3 to 5 V, level shifting. Accessed 2026-11-04.
7. Same guide, What is STEMMA QT?, https://learn.adafruit.com/introducing-adafruit-stemma-qt/what-is-stemma-qt — JST SH, 1.0 mm, cross-compatibility. Accessed 2026-11-04.
8. Same guide, comparison page, https://learn.adafruit.com/introducing-adafruit-stemma-qt/sparkfun-qwiic — connector identical, same pin ordering, level shifting only on Qwiic controllers, jumper advice. Accessed 2026-11-04.
9. Adafruit QT Py PCB, https://github.com/adafruit/Adafruit-QT-Py-PCB, file "Adafruit QT Py.sch" — device STEMMA_I2C, variants _QT and _QTSKINNY: GND pad 1, V+ pad 2, SDA pad 3, SCL pad 4. Accessed 2026-11-04.

Not usable: JST's SH catalogue (https://www.jst-mfg.com/product/pdf/eng/eSH.pdf). It downloaded, but its text layer is scrambled and the Windows PDF renderer refused the file, so I could not read its drawings and do not cite it.

## Cross-check
Colours agree: black GND, red 3.3 V / V+, blue SDA, yellow SCL (sources 1, 2, 4, 6).

**Disagreement on the numbering, important:** sources 3 and 9 (the makers' schematic symbols) have pin 1 = GND up to pin 4 = SCL. Source 2 (SparkFun's cable supplier) lists position 1 = yellow, 2 = blue, 3 = red, 4 = black. The sequence is identical, only the counting direction differs. I chose 1 = GND: two makers independently use it on the boards, and it matches the rule "GND / 3.3V / SDA / SCL". The note warns about the other count. The SparkFun page's FAQ also lists the colours from yellow to black, without numbers.

Adafruit's sentence "the colors are slightly different for SDA/SCL but the pin order is the same" follows its list of the JST PH (green, white) and JST SH (blue, yellow) cables, so it compares its own two connectors, not Qwiic. A WebFetch summary presented Adafruit's list as numbered; the raw page has an unnumbered list, so I do not use it as evidence for the numbers. Pimoroni (a search result, not read) is reported to sell cables with other SDA/SCL colours, so colours are the makers' rule and not a physical guarantee.

Unverified: left/right of pin 1 on the mating face (not asserted anywhere in the item).

## Choices
- Name of pin 2 is `3.3V` (SparkFun's rule); V+ for STEMMA QT is in the function. SparkFun's symbol prints VCC.
- `type: data` for SDA and SCL by the catalog schema.
- One column instead of one row: avoids an unverifiable left/right statement and lets the page show names beside the contacts (the page rules).
- `drawn: plug` = header on the board: SparkFun calls it the board connector, Adafruit calls the cable end "JST-SH female".
- "Keep V+ at 3.3 V when a Qwiic device is on the bus" is my wording of Adafruit's advice to set a controller's jumper to 3 V and SparkFun's 3.3 V rule.
