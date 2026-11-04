# vga — VGA (DE-15) / VGA (DE-15)

Kind: cross-checked reproductions
Pins: 15; types: signal 5, nc 2, ground 5, power 1, data 2

## View
Written for the female connector on a computer or monitor, looking at its mating face (`drawn: socket`): 5 4 3 2 1 on top, 10 9 8 7 6 in the middle shifted half a pitch to the right, 15 14 13 12 11 at the bottom, wide side of the D shell at the top. The male cable connector is the mirror image.

Basis:
- Winford Engineering drawing BRKLD15HDF-SG-DIN rev A ("DB15HD Female Breakout", box "CONNECTOR PINOUT"): text layer `5 1`, `10 6`, `15 11` at the corners of the rows. Rendered and looked at: the middle row is shifted to the right.
- Wikipedia figure VGA_Port.svg, captioned "A female DE-15 socket": I read the coordinates in the SVG file. Top row 5 … 1, middle row 10 … 6 half a pitch further right, bottom row 15 … 11.
- pinouts.ru figures: female the same way, male with 1 … 5 on top and the middle row shifted to the left, which is the mirror image.

## Sources
1. Winford Engineering, drawing BRKLD15HDF-SG-DIN rev A, DB15HD Female Breakout, https://www.bipom.com/documents/peripherals/winford/brkld15hdf-sg-din_rev_a_dwg.pdf — face view with pin numbers. Accessed 2026-11-04.
2. Dell, SE2416H user's guide, page "Pin assignments", read at https://www.manualslib.com/manual/1121260/Dell-Se2416h.html?page=43 — full pin table of a monitor's D-SUB input. Accessed 2026-11-04.
3. Acer, B193 user's guide, page "Connector pin assignment", read at https://www.manualslib.com/manual/451319/Acer-B193.html?page=20 — full pin table. Accessed 2026-11-04.
4. Crypto Museum, D-sub connectors, https://cryptomuseum.com/ref/conn/dsub — DE-15 table. Accessed 2026-11-04.
5. Wikipedia, VGA connector, https://en.wikipedia.org/wiki/VGA_connector, with https://upload.wikimedia.org/wikipedia/commons/9/96/VGA_Port.svg — table with names, figure. Accessed 2026-11-04.
6. pinouts.ru, 15 pin highdensity D-SUB male connector, https://connector.pinouts.ru/15_pin_highdensity_D-SUB_male/ — figures dsubhi15m.gif and dsubhi15f.gif. Accessed 2026-11-04.

## Cross-check
Pins 1, 2, 3, 6, 7, 8, 12, 13, 14 and 15 agree in sources 2 to 5 (names differ, signals do not). Pin 10 is a ground in all four.

Disagreements. The pins differ between real devices, so none of them is an error:
- Pin 4: Dell GND, Acer monitor ground, Crypto Museum ID2, Wikipedia ID2/RES.
- Pin 5: Dell self-test, Acer DDC return, Crypto Museum and Wikipedia GND.
- Pin 9: Dell "Computer 5V/3.3V", Acer +5 V, Crypto Museum no pin (key), Wikipedia KEY/PWR (+5 V, once a key).
- Pin 11: Dell GND, Acer monitor ground, Crypto Museum ID0, Wikipedia ID0/RES.

Chosen: the names of the computer-side table (Wikipedia, with Crypto Museum for the ID history) for the number and name, and each variation in the pin `notes` and item `notes`. Dell and Acer describe monitor inputs.

The middle row's direction is confirmed by a maker drawing only for the female part; for the male part it follows from the mirror rule and the pinouts.ru figure.

UNVERIFIED: the VESA DDC standard itself (not free) was not read. Pins 12 and 15 as I²C rest on the sources above.

## Choices
- `drawn: socket`, since that is the part on a device and the part with a maker drawing. rs232_db9 uses `drawn: plug`, because a PC's port is male.
- Pins 4 and 11 are `nc` (reserved). Pin 9 is `power`, with a note that it may be missing.
- Pin 5 is `ground` as the computer-side tables have it, with the monitor variations in its note.
- Red, green and blue are `signal` (analogue).
- `offset` on the middle row because it is drawn from the female side; negative offsets do not exist in the catalog schema.
