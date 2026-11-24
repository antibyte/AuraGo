# eurorack_power — Eurorack / Doepfer A-100 bus power, 16-pin (2x8) / Eurorack / Doepfer A-100 Busstrom, 16-polig (2x8)

Kind: maker document (Doepfer) for the signal order and pin 1; pin numbers per pair from a second maker (Elby Designs)
Pins: 16; types: power 6, ground 6, signal 4
Status: complete for pins and functions; partial for the drawing (which column holds the odd pins is not stated)

## View
Written for the module's (or bus board's) pin header as Doepfer draws it: `rows`, outline `rect`, `drawn: plug`, `key: none`, 8 rows of 2, -12 V row at the bottom. Rows (top to bottom), each row one signal pair: 15/16 Gate, 13/14 CV, 11/12 +5V, 9/10 +12V, 7/8 GND, 5/6 GND, 3/4 GND, 1/2 -12V (red stripe).

Odd pins are written in the left column; this is a placeholder (see Unverified). Because both contacts of every row carry the same signal, the mirror view (cable socket) swaps only the two columns and no function changes; -12 V stays at the bottom. The item notes say so.

Basis (read):
- Doepfer, figure a100b_e.gif "bus connectors of A-100 modules" (A-100 technical details page): 16-pin version drawn as 8 rows of two contacts, labels from the top: Gate, CV, +5V, +12V, GND, GND, GND, -12V; 10-pin version: +12V, GND, GND, GND, -12V; both placed at the "right lower corner of PC board".
- Doepfer, figure a100t_3.gif "The A-100 system bus": bus connectors 1 to 14, each 8 rows of two, rows from the top INT. Gate, INT. CV, +5 V, +12 V, GND (three rows), -12 V.
- Doepfer FAQ: "pin "1" = red wire" and "colored strip = -12V = bottom". So pin 1 is in the bottom (-12 V) row.
- Elby Designs, "EuroSynth - Power Distribution": "Pins 1,2 -12V; 3,4 0V; 5,6 0V; 7,8 0V; 9,10 +12V; 11,12 +5V; 13,14 CV; 15,16 GATE" for "a 16-pin IDC connector" of the Doepfer A-100.

## Sources
1. Doepfer Musikelektronik, "A-100 Technical Details" (a100t_e.htm), https://www.doepfer.de/a100_man/a100t_e.htm — text on the bus, red stripe, 10/16-pin versions, power supply ratings; figure https://www.doepfer.de/a100_man/a100b_e.gif (module connectors) and https://www.doepfer.de/a100_man/a100t_3.gif (system bus); accessed 2026-11-24
2. Doepfer Musikelektronik, A-100 FAQ, https://doepfer.de/faq/a100_faq.htm — sections "Orientation of bus connection cables" ("It is essential that: colored strip = -12V = bottom") and "Notes concerning bus boards with boxed pin headers" ("pin "1" = red wire / nose = right when red wire is at the bottom"); accessed 2026-11-24
3. Elby Designs, "EuroSynth - Power Distribution", https://elby-designs.com/webtek/documents/eurosynth/power_distribution.htm — 16-pin A-100 pin numbers per pair; 10-pin EuroSynth layout 1,2 -12V, 3 to 8 0V, 9,10 +12V; "All module cables must assign pin 1 of the IDC socket to electrical pin 1 (i.e. -12V)"; accessed 2026-11-24

## Cross-check
Doepfer (order of the signal rows, -12 V = red wire = pin 1 = bottom) and Elby Designs (pin numbers per pair, -12 V on 1 and 2) agree: counting pairs up from the -12 V row gives exactly Elby's table. Elby's 10-pin layout (1, 2 -12V; 3 to 8 0V; 9, 10 +12V) matches Doepfer's 10-pin drawing (five rows, -12V at the bottom, +12V at the top).

Elby also gives wire colours for its own 10-pin proposal (-12V blue, 0V black, +12V red); these are Elby's, not Doepfer's, and conflict with Doepfer's red stripe = -12 V, so no `wire` is used.

## Choices
- Catalog item: one row per signal pair, -12 V at the bottom, as Doepfer draws both the module connector and the bus; `key: none` (the record names no key; the Doepfer FAQ's "nose" belongs to boxed headers and is only mentioned in the notes as a code gap that can force the cable the wrong way).
- `power` for -12V, +12V, +5V; `ground` for GND; `signal` for CV and Gate.
- Names `-12V`, `GND`, `+12V`, `+5V`, `CV`, `Gate` as Doepfer writes them.
- 10-pin version in the notes only (it is the lower five rows of this item; a 10-pin plug sits on the -12 V end).
- No `wire`: only "colored (typically red)" for the -12 V conductor of the ribbon; the pin 1 function text says it.

## Spot-check list
- Row order Gate, CV, +5V, +12V, GND, GND, GND, -12V from the top: Doepfer a100b_e.gif.
- Pin 1 = red wire = -12 V, at the bottom: Doepfer FAQ.
- Pins 9, 10 = +12V: Elby Designs, Power Distribution table.
- Pins 15, 16 = GATE; 13, 14 = CV: Elby Designs, Power Distribution table.
- 10-pin carries only -12V, GND, +12V: Doepfer a100t_e.htm.

Unverified: which of the two columns holds the odd pins (1, 3, ... 15) in a stated view. Neither Doepfer nor Elby draws pin numbers in a view; the left column is a placeholder. It changes no wiring because both contacts of every row carry the same signal.
