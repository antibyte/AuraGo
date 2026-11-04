# ftdi_6 — FTDI TTL-232R (6-pin) / FTDI TTL-232R (6-polig)

Kind: authoritative maker document
Pins: 6; types: power 1, ground 1, signal 2, data 2

## View
Written for the connector at the cable end: `drawn: socket`, one column of 6, pin 1 on top. A single column looks the same in both views, so a mirror error cannot occur; only the sequence along the connector is a fact here.

Basis: figure 4.1 (rendered and looked at) draws the header as one vertical bar with contacts 1 to 6 from top to bottom and the colours in the same order, captioned "6 Way Header Pin Out". Table 4.1 gives the same numbers. Figure 4.2 shows a row of six funnel-shaped openings on the end face and a triangle labelled "Pin 1". That is why I treat the part as a socket: the text only calls it a "6 way, 0.1", Single-In-Line (SIL) connector" and "6 pin header", it never says female.

## Sources
1. Connective Peripherals, TTL-232R TTL to USB Serial Converter Range of Cables Datasheet, CP_000069, version 1.1, copy at https://cdn.shopify.com/s/files/1/0434/1376/6295/files/DS_TTL-232R_CABLES.pdf — page 9 and 10: figures 4.1 and 4.2, tables 4.1 to 4.5. Accessed 2026-11-04.
2. Future Technology Devices International, TTL-232R datasheet, FT_000054, version 2.01, 2008-08-28, copy at https://www.cl.cam.ac.uk/teaching/0910/P31/docs/usbseriallead.pdf — section 4: same figure and table. Accessed 2026-11-04.

FTDI's own download URLs (ftdichip.com) answered 403 to my request, so both documents were read in the copies above. The first document carries the Connective Peripherals name, which now issues this datasheet; the second is FTDI's own issue.

## Cross-check
Both editions agree on numbers, names and colours: 1 GND black, 2 CTS# brown, 3 VCC red, 4 TXD orange, 5 RXD yellow, 6 RTS# green. They also agree that VCC is +5 V on the 3V3 cable ("signals only, VCC= +5V"). 75 mA is the output power current of tables 4.2 and 4.4.

Unverified: nothing about the pins. Whether the housing is female is taken from figure 4.2, as described above.

## Choices
- "Do not feed a 3.3 V board from it directly" is my conclusion from VCC = +5 V; the datasheet does not phrase it as a warning.
- Direction of TXD and RXD is given from the cable's side, as in table 4.1. The advice to cross them to the target is my wording of what output and input mean.
- `type: signal` for CTS# and RTS# (RTS is the catalog schema's example for `signal`), `data` for TXD and RXD.
- I keep the # in the names as the source writes it. It must stay quoted in YAML.
- The note about other versions: the datasheet's sections 5 to 7 describe audio jack, wire end and 2 mm (8-way) versions. Their tables are not in this item.
