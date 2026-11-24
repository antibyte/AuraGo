# can_db9 — CAN on D-Sub 9 (CiA 303-1 / CiA 106) / CAN auf D-Sub 9 (CiA 303-1 / CiA 106)

Kind: cross-checked reproductions (CiA 303-1 v2.0.1 and its successor CiA 106 v1.1.0 need a CiA login and were not read)
Connector: 9-way D-subminiature (DE-9), the de-facto CAN/CANopen connector. CAN interfaces usually carry the male plug (PEAK "D-Sub (m)"), the bus cable the female socket. Drawn side: the male connector, looking at its pins (`drawn: plug`), as in `rs232_db9`.
Pins: 9; types: data 2, ground 3, power 1, nc 3

## Pins

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | Reserved | Reserved (some interfaces: optional +5 V) | Reserviert (manche Interfaces: optional +5 V) | nc |
| 2 | CAN_L | CAN bus line, dominant low | CAN-Busleitung, dominant low | data |
| 3 | CAN_GND | CAN ground | CAN-Masse | ground |
| 4 | Reserved | Reserved, not connected | Reserviert, nicht belegt | nc |
| 5 | CAN_SHLD | Optional cable shield | Optionaler Kabelschirm | ground |
| 6 | GND | Optional CAN ground (connected to pin 3) | Optionale CAN-Masse (mit Pin 3 verbunden) | ground |
| 7 | CAN_H | CAN bus line, dominant high | CAN-Busleitung, dominant high | data |
| 8 | Reserved | Reserved, not connected | Reserviert, nicht belegt | nc |
| 9 | CAN_V+ | Optional external supply for the transceivers | Optionale externe Versorgung der Transceiver | power |

Pin notes for the app:
- Pin 1: PEAK PCAN-USB can put 5 V on pin 1 and/or 9 via solder bridges (not fitted at delivery); HMS says "Reserved", Beckhoff "not used". So: `nc`, with the note.
- Pin 5: HMS "Optional shield", Beckhoff "Shield", PEAK "Not connected".
- Pin 9: HMS "Optional external voltage supply Vcc", ABB FCAN-01 lists CAN_V+ on 9, PEAK "CAN_V+ (optional)" (5 V from the interface), Beckhoff "not used"; Beckhoff's text on its device family says up to 30 V DC on pin 9 for supplying transceivers (from search excerpt only — not read, so no voltage is stated). Measure before connecting anything to pin 9.
- Pin 6: HMS "Optional CAN ground", Beckhoff "CAN Ground (internally connected to pin 3)", PEAK "CAN_GND".

## View
Written for the **male connector, looking at its pins, wide side of the D shell at the top** (`drawn: plug`): 1 2 3 4 5 on top, 6 7 8 9 below shifted half a pitch to the right — identical to `rs232_db9`. The female connector (cable socket) is the mirror image: 5 4 3 2 1 over 9 8 7 6.

Basis:
- PEAK-System PCAN-USB user manual 3.2.0, section 4.1, figure in the column "D-Sub plug": 1 2 3 4 5 over the wide row, 6 7 8 9 below, pins drawn as dots in a D shell with the wide side up. The technical data call the connector "D-Sub (m)". So this is the male face.
- Black Box Corporation, "Pinouts for Serial Interfaces" (1997), section 1B (DB9): "All connectors are shown in front view"; "1 Male 5 / 6 9" and "5 Female 1 / 9 6". Same geometry.
- Repo record `rs232_db9` (same connector, `drawn: plug`).

## Sources
1. PEAK-System Technik GmbH, PCAN-USB User Manual 3.2.0, © 2024, section 4.1 "Connection over D-Sub Connector" (page 16: pin table and D-Sub plug figure, "corresponds to the specification CiA 106") and Appendix D quick reference (page 38), https://www.peak-system.com/produktcd/Pdf/English/PCAN-USB_UserMan_eng.pdf. Accessed 2026-11-24.
2. HMS Networks (Ixxat), knowledge base article "CAN-Steckerbelegung" (pin assignment reference for Ixxat CAN interfaces: Sub-D9, RJ45, header), https://www.hms-networks.com/de/support/tech-support/kb-articles/26602569633426-CAN-Steckerbelegung — Sub-D9 table "nach der CiA DS 102". Accessed 2026-11-24.
3. Beckhoff Automation, CX7050 documentation, "D-sub connector (X003)", Fig. 4 "CANopen interface X003", https://infosys.beckhoff.com/content/1033/cx7050/14426931339.html. Accessed 2026-11-24.
4. Black Box Corporation, "Technical Reference: Pinouts for Serial Interfaces", 1997, copy at https://ftp1.digi.com/support/images/PinoutStndrds.pdf — DE-9 front-view numbering only. Accessed 2026-11-24.
5. CAN in Automation, "CiA 106: Connector pin assignment" (overview page), https://can-cia.org/can-knowledge/cia-106-connector-pin-assignment — says the recommendations moved from CiA 303-1 (v2.0.1, 2023-03-27) to CiA 106 (v1.1.0, 2023-07-11); both behind a login, not read. Accessed 2026-11-24.

## Cross-check
CAN_L on 2, CAN_GND on 3, CAN_H on 7: PEAK, HMS and Beckhoff agree. Pin 6 ground: all three (HMS "optional"). Pins 4 and 8 unused: all three. Differences on 1, 5 and 9 are listed above; they are device options, not errors — the main table follows HMS (which cites the CiA document) with the variations in the notes.

## Choices
- Catalog item: `data_av.yaml`, group `data`, outline `dsub`, `contact: round`, `drawn: plug` (male, as `rs232_db9`), rows 1..5 / 6..9 with `offset: 0.5`; pins 2, 3, 6, 7 as agreed by all sources, pins 1, 5, 9 with the makers variations in pin notes (pin 1 `nc`, pin 9 `power`, pin 5 `ground`); the CiA documents were not read and the item notes say so.
- `drawn: plug` (male), like `rs232_db9` — the interface/device side in the PEAK figure.
- CAN_H/CAN_L typed `data` (differential bus lines). Pin 9 `power` with the "optional, measure first" note; pin 1 `nc` with the PEAK note.
- Names as in HMS/CiA style (CAN_L, CAN_GND, CAN_SHLD, CAN_H, CAN_V+), all ≤ 24 characters.
- The 120 Ω bus termination is not a pin matter; `doc` may say "terminate both bus ends with 120 Ω" only if the app wants it — not verified here, so leave out.

## Spot-check list
- Pin 2 = CAN_L — PEAK 4.1 table; HMS Sub-D9 table; Beckhoff X003 table.
- Pin 7 = CAN_H — PEAK; HMS; Beckhoff.
- Pin 3 = CAN_GND — PEAK; HMS; Beckhoff.
- Pin 9 = CAN_V+ (optional) — PEAK; HMS.
- Male face: 1–5 on top, 6–9 below — PEAK figure "D-Sub plug"; Black Box 1B.

UNVERIFIED: CiA 303-1 / CiA 106 themselves (login). Voltage on CAN_V+ (PEAK 5 V on its own product; others not stated). The CiA wording for pins 1, 4, 8.
