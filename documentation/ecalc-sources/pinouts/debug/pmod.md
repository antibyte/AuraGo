# pmod — Digilent Pmod (2x6, 12-pin) / Digilent Pmod (2x6, 12-polig)

Kind: authoritative maker document (Digilent's own specification, 2011 edition), current edition not opened
Pins: 12; types: power 2, ground 2, io 8
Scope: the generic 12-pin layout only (positions, GND and VCC pins, eight signals as general I/O). The per-type signal tables are not in the catalog.

## Pins (generic GPIO layout, as both editions agree)
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| 1 | IO1 | I/O 1 (on an SPI module: CS) | E/A 1 (bei einem SPI-Modul: CS) | io |
| 2 | IO2 | I/O 2 | E/A 2 | io |
| 3 | IO3 | I/O 3 | E/A 3 | io |
| 4 | IO4 | I/O 4 | E/A 4 | io |
| 5 | GND | Ground | Masse | ground |
| 6 | VCC | Supply from the host, usually 3.3 V | Versorgung vom Host, meist 3,3 V | power |
| 7 | IO5 | I/O 5 | E/A 5 | io |
| 8 | IO6 | I/O 6 | E/A 6 | io |
| 9 | IO7 | I/O 7 | E/A 7 | io |
| 10 | IO8 | I/O 8 | E/A 8 | io |
| 11 | GND | Ground | Masse | ground |
| 12 | VCC | Supply from the host, same voltage as pin 6 | Versorgung vom Host, gleiche Spannung wie Pin 6 | power |

## View
Written as Digilent draws it: the host (system board) seen from above, connector at the right-hand board edge, `drawn: socket`. Rows (left, right), top to bottom:
- [1, 7]
- [2, 8]
- [3, 9]
- [4, 10]
- [5, 11]
- [6, 12]

Pin 1 is the square pad at the top of the column farther from the board edge. The other view (mirror) is the peripheral module seen from above with its board edge at the left: [7, 1] ... [12, 6].

Basis: specification 2011, page 4, "Fig 2: Standard Twelve-Pin Connector Placement" (rendered and read): "Host" drawn as "1 □ ○ 7 / 2 ○ ○ 8 / ... / 6 ○ ○ 12", a vertical line (board edge) to the right, "0.45"", "Right angle female connector"; "Peripheral" drawn as "7 ○ □ 1 / ... / 12 ○ ○ 6", board edge to the left, "0.20"", "Right angle male connector". Page 3 text: "Note that the pin numbering conventions for the 2x6 connectors are non-standard and are mirrored between the host connector and the peripheral board connector." Page 6 text, SPI type: "When this interface is placed on a 12-pin connector on a host, it should use pins 1-6 (i.e. the upper row of pins)."

So in the mating face of a right-angle host connector, pins 1 to 6 form the upper row. No front view of the mating face was read, so the record stays in the top view that is drawn.

## Sources
1. Digilent, "Digilent Pmod Interface Specification", Doc 510-002, revision November 20, 2011, copy at https://cdn.hackaday.io/files/21333912711072/Digilent-Pmod_%20Interface_Specification.pdf — page 1 (six-pin: "four digital I/O signal pins, one power pin and one ground pin"; twelve-pin: "two of the six-pin interfaces stacked"), page 2 (power: "switch the voltage on the power pins between 5.0V and 3.3V", reduced subset 3.3 V only; "both power supply pins switch together"; about 100 mA), page 3 (fig 1, mirror note), page 4 (fig 2), pages 6 to 10 (type tables). Rendered and read. Accessed 2026-11-24.
2. Wikipedia, "Pmod Interface", https://en.wikipedia.org/wiki/Pmod_Interface — type table of the current numbering (pins 1 to 6, second set 7 to 12 in brackets; pin 5 GND, pin 6 Vcc, 11 GND, 12 Vcc) and the list of specification editions 1.1.0 (July 2017), 1.2.0 (October 2017), 1.3.0 (September 2020), 1.3.1 (October 2020). Accessed 2026-11-24.

Not opened: Digilent Pmod Interface Specification 1.3.1 (and 1.1.0, 1.2.0) at digilent.com/reference/_media/reference/pmod/ — HTTP 403 to the fetch tool. Digilent's blog posts about 1.1.0 and the I2C type — HTTP 403. The Digi-Key copy — HTTP 410.

## Cross-check
- Generic layout: the 2011 specification and Wikipedia agree: 1 to 4 signals, 5 GND, 6 VCC, 7 to 10 signals, 11 GND, 12 VCC. The 2011 type 2A table (page 7) lists 11 GND and 12 VCC explicitly.
- **Types differ between editions.** 2011 edition: type 3 UART (deprecated, 1 CTS, 2 RTS, 3 RXD, 4 TXD), type 4 UART (1 CTS, 2 TXD, 3 RXD, 4 RTS), type 5 H-bridge, type 6 dual H-bridge, and I2C on a separate 2x4 header. Wikipedia for the current edition: type 3 UART, type 4 H-bridge, type 5 dual H-bridge, type 6 I2C on the 6-pin connector. The SPI type 2 is the same in both: 1 SS/CS, 2 MOSI, 3 MISO, 4 SCK. Only the 2011 edition was read; the current-edition type numbers rest on Wikipedia alone. That is why the catalog carries no per-type table.
- Direction: the specification names directions from the host ("'Out' means from the host to the peripheral").

## Choices
- Catalog item: the generic layout, named "Pmod (2×6)". Pins IO1 to IO8 (the 2011 type 1 table names 1 to 4 IO1 to IO4; IO5 to IO8 for 7 to 10 follow Wikipedia's type 1A). No per-type names on the pins, because the same connector carries SPI, UART, I2C or GPIO; the item note says that the module's Pmod type decides the signal use and that the type tables differ between editions. Only the SPI order (1 CS, 2 MOSI, 3 MISO, 4 SCK), which both editions agree on, is in the note, and the function of pin 1 says "on an SPI module: CS". UART and I2C pin orders are not in the item (the I2C order is Wikipedia only).
- `rows` with two pins per row, `outline: rect`, `contact: square`, `drawn: socket`, no key (the square pad at pin 1 marks the orientation, and the note says so).
- Supply text "usually 3.3 V" from page 1 ("the general expectation is that a 3.3V logic power supply will be used") and page 2.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | IO1 | io | Digilent 510-002 (2011), page 4 fig 2 (square pad, host top left); page 6 type 1 table "1 IO1 In/Out" |
| 5 | GND | ground | page 6 type 1 table "5 GND"; Wikipedia type table |
| 6 | VCC | power | page 6 type 1 table "6 VCC"; Wikipedia type table |
| 7 | IO5 | io | page 4 fig 2 ("1 □ ○ 7"); page 7 type 2A "7 INT In" shows 7 is a signal pin |
| 11 | GND | ground | page 7 type 2A table "11 GND"; Wikipedia (12-pin: 2 GND) |
| 12 | VCC | power | page 7 type 2A table "12 VCC" |

UNVERIFIED: current Digilent specification 1.3.1 not opened (403); its figures, its I2C (type 6) and UART (type 3) pin orders and whether pin 2 is marked "PWM" rest on Wikipedia only and stay out of the pin data. The front (mating-face) view of a right-angle host socket was not read.
