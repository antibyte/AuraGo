# microsd_card — microSD card, SD and SPI mode / microSD-Karte, SD- und SPI-Modus

Kind: cross-checked reproductions — three card makers (SanDisk, Kingston, Cactus Technologies); the microSD addendum of the SD specification is not free and was not read
Connector: microSD memory card, 8 contact pads in one row along the insertion edge. Drawn side: the card's contact side ("back view" / "bottom view").
Pins: 8 per variant. Types — `sd`: data 5 (CMD, DAT0–DAT3), signal 1 (CLK), power 1, ground 1. `spi`: data 2, signal 2, power 1, ground 1, nc 2.

## Pins

| Pin | `sd` name | `sd` function (en) / (de) | `sd` type | `spi` name | `spi` function (en) / (de) | `spi` type |
|---|---|---|---|---|---|---|
| 1 | DAT2 | Data line bit 2 / Datenleitung Bit 2 | data | RSV | Reserved / Reserviert | nc |
| 2 | CD/DAT3 | Card detect / data line bit 3 / Kartenerkennung / Datenleitung Bit 3 | data | CS | Chip select, active low / Chip-Select, aktiv low | signal |
| 3 | CMD | Command/response / Befehl/Antwort | data | DI | Data in (host to card) / Dateneingang (Host zur Karte) | data |
| 4 | VDD | Supply voltage / Versorgungsspannung | power | VDD | Supply voltage / Versorgungsspannung | power |
| 5 | CLK | Clock (host to card) / Takt (Host zur Karte) | signal | SCLK | Clock / Takt | signal |
| 6 | VSS | Supply ground / Versorgungsmasse | ground | VSS | Supply ground / Versorgungsmasse | ground |
| 7 | DAT0 | Data line bit 0 / Datenleitung Bit 0 | data | DO | Data out (card to host) / Datenausgang (Karte zum Host) | data |
| 8 | DAT1 | Data line bit 1 / Datenleitung Bit 1 | data | RSV | Reserved / Reserviert | nc |

Notes (verified):
- CD/DAT3 has a 50 kΩ pull-up in the card at power-up: card detection and mode selection; the host drives it low to select SPI mode; the pull-up is switched off with ACMD42 (SanDisk Table 4 note 3).
- The host should pull up all data lines even if only DAT0 is used, otherwise floating DAT1/DAT2 inputs can draw current (SanDisk note 3). In SPI mode the RSV lines "should be pulled up by host side with 10–100K ohm" (Kingston Table 3 note).
- DAT1 may be the SDIO interrupt, DAT2 the SDIO read-wait (SanDisk notes) — optional `notes`.
- Supply: Kingston states 2.7 V to 3.6 V operating voltage for its card; SanDisk refers to the SDA physical layer spec. Put a voltage in VDD's function only as "2.7–3.6 V (memory operation)", citing Kingston, or leave it out.

## View
Written for the **contact side of the card ("back view", "bottom view"), insertion edge with the contacts at the top**: the step (key notch) in the card's edge is on the **left**, and the pads read **1 2 3 4 5 6 7 8 from left to right**. Pads 4 (VDD) and 6 (VSS) are longer than the others (they reach closer to the insertion edge).

The label side ("top view") is the mirror image: step on the right, 8 … 1 left to right.

Basis:
- Kingston SDCIT spec 4900180-001.A00, Figure 1 "microSD Card Pin Assignment (Back View of microSD Card)": rendered and looked at: contacts at the top labelled "Pin 1" (left) … "Pin 8" (right), the step on the left side below the contacts, Pin 4 and Pin 6 drawn taller.
- SanDisk OEM Product Manual 3.1, Figure 5 "microSD Card Bottom View": contacts at the top, step on the left, the 4th and 6th contacts from the left drawn taller (no numbers in the figure; the long pads identify 4 = VDD and 6 = VSS, as in Kingston). Figure 4 "microSD Card Top View" has the step on the right — the mirror, as expected.

**For the drawing:** the `card` outline (microSD shape: step on one long side) with the pad row along the top edge, pads 4 and 6 longer.

## Sources
1. SanDisk Corporation, "SanDisk microSD, microSDHC and microSDXC cards OEM Product Manual", Revision 3.1, document no. 80-36-03335, November 2013 — §2.1.4 Figures 4 and 5 (page 9), §3.1 Table 4 "SD Bus Mode Pin Assignment" with SD and SPI columns and notes (page 10); "Pin assignments are provided by the SDA Physical Layer Specification, Version 3.01 and associated addendums (microSD)". Copy at https://cdn-shop.adafruit.com/product-files/1294/P1294+C11713-001SDSDQM-008G-Z35+%281%29.pdf. Accessed 2026-11-24.
2. Kingston Technology, microSDHC industrial card specification SDCIT/8GB–32GB, document 4900180-001.A00, §6.1 "microSD Card Pin", Figure 1 and Table 3 (page 3), https://www.kingston.com/datasheets/SDCIT-specsheet-8gb-32gb_us.pdf. Accessed 2026-11-24.
3. Cactus Technologies, "Industrial 3D pSLC -295M Series microSD Card Product Manual", Apr 2, 2025, §3.1 Table 3-5 "microSD interface Pin Assignments and Pin Type", copy at https://docs.rs-online.com/9f4b/A700000015847579.pdf. Accessed 2026-11-24.

## Cross-check
SD mode: SanDisk, Kingston and Cactus agree on all 8 names (1 DAT2, 2 CD/DAT3, 3 CMD, 4 VDD, 5 CLK, 6 VSS (Cactus "VSS2"), 7 DAT0, 8 DAT1).
SPI mode: all three agree on 2 CS, 3 data in, 4 VDD, 5 SCLK, 6 VSS, 7 data out, 8 reserved. Pin 1: SanDisk and Kingston "RSV" (reserved), Cactus "NC" (no connect) — same meaning for wiring; `nc` chosen. Names differ (Kingston DI/DO, Cactus SDI/SDO/CSN, SanDisk DataIn/DataOUT).
Slip in SanDisk Table 4: descriptions "Data Line [bit 1]" for DAT0 and "[bit 2]" for DAT1 contradict the names; Kingston and Cactus give bit 0 and bit 1. Names used.
View: Kingston (explicit numbers, "Back View") and SanDisk (geometry, "Bottom View") agree.

## Choices
- Catalog item: `data_av.yaml`, group `data`, outline `card`, `contact: blade`, one row 1..8; `drawn: plug`, because a card is the inserted part that carries the pads and the view of the record (contact side, step on the left) is the card itself, as the fixture `fx_pin_card` and the outline comment draw it; the second view caption is the label side, the mirror image (step on the right). Variants `sd` and `spi` with the same numbers; the reserved SPI pads 1 and 8 are `nc` named `RSV` with the pull-up note. The full-size SD card is not included (its drawing direction could not be verified).
- Two variants `sd` and `spi`; same numbering.
- `nc` for the reserved SPI pins 1 and 8 with the pull-up note.
- Contact-side drawing because the contacts are what one solders to or probes; the item notes say the label side is the mirror.

## Spot-check list
- Pin 1 = DAT2 (SPI reserved) — SanDisk Table 4; Kingston Table 3; Cactus Table 3-5.
- Pin 2 = CD/DAT3 (SPI CS) — SanDisk; Kingston; Cactus.
- Pin 4 = VDD, pin 6 = VSS — SanDisk; Kingston; Cactus; long pads in both figures.
- Pin 7 = DAT0 (SPI data out) — Kingston; Cactus (SanDisk name DAT0, "DataOUT").
- Back view: pin 1 on the step side (left), contacts up — Kingston Figure 1; SanDisk Figure 5.

UNVERIFIED: the microSD addendum of the SD specification (not free). UHS-II second-row pads and antenna pads are not covered. VDD range only from Kingston's own product.
