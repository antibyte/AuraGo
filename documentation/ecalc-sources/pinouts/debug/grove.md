# grove — Grove (4-pin) / Grove (4-polig)

Kind: authoritative maker document
Pins: 4; types: power 1, ground 1, signal 2

## View
Written for the connector on the module: `drawn: socket`, one column, pin 1 on top. A single column looks the same in both views, so only the sequence along the connector is stated: pin 1 yellow, pin 2 white, pin 3 red, pin 4 black, as Seeed lists them.

Seeed gives no statement on which physical end of the connector is pin 1 (the text has none; I did not look at its mechanical drawing ZIP files), so the item makes no left/right claim. The connector is keyed and the wire colours identify the pins. The derivation is the numbering itself; this is not a `GEOMETRY UNVERIFIED` case because the single column asserts nothing beyond the sequence.

## Sources
1. Seeed Studio Wiki, Grove Ecosystem Introduction, https://wiki.seeedstudio.com/Grove_System/ — colours list, the four interface tables, the UART viewpoint sentence, I2C voltage remark, "Grove Female Header" products with 4-pin, 2.0 mm pitch, "Grove plug". Accessed 2026-11-04.
2. Adafruit, Adafruit STEMMA & STEMMA QT, STEMMA / Grove comparison, https://learn.adafruit.com/introducing-adafruit-stemma-qt/seeed-studio-grove — HY-style housing by ZJDLL, "not JST PH". Accessed 2026-11-04.

## Cross-check
Only one maker's document is needed and all pin facts come from it. Seeed's page says of this section that it "may be outdated". The tables are internally consistent. I did not search for a second Seeed table to compare them with.

Comparison with Qwiic (own derivation from the two tables): Grove I2C runs SCL, SDA, VCC, GND from pin 1, Qwiic runs GND, 3.3V, SDA, SCL from pin 1 in the makers' numbering.

Unverified: which end of the housing is pin 1, as above. Left out of the notes.

## Choices
- Pin names `SIG1` and `SIG2` are mine and neutral, as the assignment asks. Seeed's names (Dn, An, RX, SCL and Dn+1, An+1, TX, SDA) are in the functions, and the meanings per interface are in the notes.
- `type: signal` for pins 1 and 2 because they differ per module, although they are data lines on I2C and UART.
- No variants: the assignment asks for one list with the meanings in the notes.
- `plug` and `socket` follow Seeed's words: "Grove plug" for the cable end and "Grove Female Header" for the board part. The board part holds the pins, despite that name, so the caption avoids the word pins.
- Outline `jst` as assigned, although Adafruit says that the housing is not a true JST. The note says so.
- The advice "check the module" reflects Seeed's remark that a few I2C modules take only 3.3 V or only 5 V.
