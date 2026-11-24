# pcie_12vhpwr — PCIe 12V-2x6 / 12VHPWR, 16-pin (12 + 4) / PCIe 12V-2x6 / 12VHPWR, 16-polig (12 + 4)

Kind: free official specification
Pins: 16; types: power 6, ground 6, signal 4

## View
Written for the cable plug, looking at its mating face (`drawn: plug`), latch on top: the contact holes face the reader.

Rows (top to bottom, each left to right):
- top row (latch side): 7, 8, 9, 10, 11, 12
- middle row: 1, 2, 3, 4, 5, 6
- bottom row (sideband), shifted by one pitch so it sits under the middle of the power rows: S1, S2, S3, S4

The header on the card or in a modular supply (`socket`) is the mirror image x to W - x, latch staying on top: 12 ... 7 over 6 ... 1 over S4 ... S1.

Basis (read, both in Intel 336521 Rev. 2.1a):
- Figure 5-4 "12V-2x6 Cable Assembly", bottom drawing (page 60, zoomed): the plug face with the latch at the top centre; leader "PIN 1" to the left cavity of the lower power row, "PIN 2" to the second cavity of the same row, "PIN 12" to the right cavity of the upper row; the sideband row below the lower power row, "S1" at its left end and "S4" at its right end. In third-angle projection this bottom view looks at the plug's mating end.
- Figure 5-6 "12V-2x6 PCB Header Dimensions - Front View" (page 63, zoomed): "PIN 12" upper left, "PIN 7" upper right, "PIN 6" lower left, "PIN 1" lower right, "S4" left and "S1" right in the sideband row below: exactly the mirror image of the plug face, as a mating pair must be. Figure 5-5 (page 62) shows the latch ramp on top of the header housing with the sideband row at the bottom.
- So pins 1 to 6 (+12 V) are the row next to the sideband contacts, pins 7 to 12 (COM) the row on the latch side.

## Sources
1. Intel, "ATX Version 3 Multi Rail Desktop Platform Power Supply Design Guide", document 336521, Revision 2.1a, November 2023, https://cdrdv2-public.intel.com/336521/336521_Rev2p1a.pdf — table 5-7 "PCIe* 12V-2x6 Auxiliary Power Connector Pin Assignment (600 watts)" (page 66: pins, signals, colours); sections 3.2, 3.3, 3.3.1 to 3.3.3 and tables 3-5, 3-6 (pages 28 to 34: limits and sideband meaning); section 5.2.2.4.3 (pages 58 to 65: current rating, wires, H++/H+, 12VHPWR vs 12V-2x6); figures 5-4, 5-5, 5-6 (pages 60, 62, 63: numbering and view); accessed 2026-11-24
2. Corsair, "Evolving Standards: 12VHPWR and 12V-2x6", https://www.corsair.com/us/en/explorer/diy-builder/power-supply-units/corsair-12vhpwr-connector/ — "the new 12V-2x6 connector has shorter sensing pins (1.5mm) while the conductor terminals are 0.25mm longer", 600 W; no pin table; accessed 2026-11-24

Not reached / not read: PCI-SIG CEM 5.0 / 5.1 (members only; no free copy found); the Seasonic cable-compatibility page (HTTP 403; its table was seen only as a search snippet and does not count); the Amphenol 10160920 drawing (HTTP 403).

## Cross-check
Inside the Intel guide, table 5-7 (pins and colours), figure 5-4 (plug numbering) and figure 5-6 (header numbering, the mirror) agree. No second independent primary document with a pin table was read; Intel's guide is the free official specification for this connector on the supply side, which meets the rule. Corsair confirms only the 12V-2x6 mechanical change.

Pin notes (all read in Intel 336521 Rev. 2.1a):
- SENSE0/SENSE1, table 3-6 "PCI Express* 12V-2x6 Connector Power Limits" (initial permitted power at power-up / maximum sustained power after software configuration): Ground, Ground 375 W / 600 W; Open, Ground 225 W / 450 W; Ground, Open 150 W / 300 W; SENSE0 and SENSE1 shorted to each other (floating against ground) 100 W / 150 W; Open, Open 0 W / 0 W. Note 1: a card must monitor SENSE0 and SENSE1 to draw any power through the connector; the SENSE signals must not change state while the card edge has main +3.3 V.
- CARD_PWR_STABLE, section 3.3.2: optional; open collector / open drain on the card; in the supply a 4.7 kΩ pull-up to +3.3 V; open = card rails within limits, driven low (at least 100 ms) = fault.
- CARD_CBL_PRES#, section 3.3.3: required on every card (4.7 kΩ pull-down at the card), optional in the supply; low = cable attached to a card. It does not tell the card its power level.
- Section 5.2.2.4.3: 9.2 A per pin, at most 30 °C rise above ambient at +12 V DC with all twelve contacts energized; the body must show "H++" (12V-2x6; the older 12VHPWR was "H+"); wires 16 AWG, all 12 pins connected "with no depopulation"; sideband wires 28 AWG. Table 3-5: 600 W level, 55 A maximum RMS on +12 V.

## Choices
- Catalog item: `rows` with three rows (7-12 over 1-6 over S1-S4 at `offset: 1`), `outline: minifit`, `contact: square`, `drawn: plug`. One item for 12VHPWR and 12V-2x6 (same pins and numbering; the differences are mechanical and the SENSE coding), and the item note says so.
- Approximation, stated in the item note: the sideband contacts are drawn at power-contact size on the `minifit` grid, with the row centred under the middle of the power rows. The real sideband contacts are smaller and at a finer pitch; no dedicated outline was made for this.
- Sideband names and meanings exactly as Intel table 5-7 and sections 3.3.1 to 3.3.3 read them (CARD_PWR_STABLE, CARD_CBL_PRES#, SENSE0, SENSE1); all four `signal`, wire blue as Intel names it.
- Left out on purpose: what the SENSE coding meant on the older 12VHPWR (CEM 5.0 not read; Intel only says Open/Open "was not present in the 12VHPWR connector"). In particular the 150 W meaning of Open/Open is not in the data. The table in the S3 note is the 12V-2x6 one.
- Name `+12V` for pins 1 to 6 rather than Intel's `+12V3/V4` (the rail numbering of a multi-rail supply, which the card side does not see), as `pcie_8`. `COM` as Intel writes it.

## Spot-check list
- Pin 1 = +12V3/V4, yellow; pin 7 = COM, black: Intel 336521 table 5-7, page 66.
- S1 = CARD_PWR_STABLE, S2 = CARD_CBL_PRES#, S3 = SENSE0, S4 = SENSE1, all blue: Intel 336521 table 5-7, page 66.
- Ground/Ground = 600 W, Open/Open = 0 W, SENSE0-SENSE1 short = 150 W: Intel 336521 table 3-6, page 31.
- 9.2 A per pin, 30 °C rise, all twelve contacts energized: Intel 336521 section 5.2.2.4.3, page 59.
- PIN 1 lower left, PIN 12 upper right, S1 left on the plug face: Intel 336521 figure 5-4, page 60.
- PIN 1 lower right, PIN 12 upper left, S1 right on the header front view: Intel 336521 figure 5-6, page 63.

UNVERIFIED: the SENSE coding of the original 12VHPWR (CEM 5.0) was not read in a primary source and is not in the data.
