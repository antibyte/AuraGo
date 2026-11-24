# dsub25 — D-Sub 25 (DB-25): parallel port and RS-232 / D-Sub 25 (DB-25): Parallelport und RS-232

Kind: cross-checked reproductions (IEEE 1284 and TIA-232-F are paywalled and were not read; the EIA-232-D/E table was read in Datapro's reproduction)
Connector: 25-way D-subminiature (DB-25), two rows: 13 on the wide side, 12 on the narrow side. Same numbering for both variants. Drawn side: the female connector (`drawn: socket`) — the PC's parallel port is female; an RS-232 DTE (PC, terminal) has the male one, which is the other view.
Pins: 25 per variant. Types — `parallel`: data 8, signal 9, ground 8. `rs232`: data 4, signal 16, ground 2, nc 3.

## View (both variants)
Written for the **female connector, looking at its mating face, wide side of the D shell at the top** (`drawn: socket`):
- Top row: 13 12 11 10 9 8 7 6 5 4 3 2 1 (left to right).
- Bottom row: 25 24 23 22 21 20 19 18 17 16 15 14, each between two top-row positions (25 between 13 and 12, 14 between 2 and 1) → `offset: 0.5`.

The male connector seen from its pins is the mirror image: 1 … 13 on top, 14 … 25 below, pin 1 at the upper left.

Basis:
- Winford Engineering PBC25F Physical Layout, "DB25 Female Pinout": text layer and render: 13 upper left, 1 upper right, 25 lower left, 14 lower right, 13 holes in the upper row, board edge at the bottom. PBC25M ("DB25 Male Pinout"): 1 upper left, 13 upper right, 14 lower left, 25 lower right.
- Black Box Corporation, Technical Reference "Pinouts for Serial Interfaces" (1997): "All connectors are shown in front view (wires exit from the rear)"; DB25 drawings "1 Male 13 / 14 25" and "13 Female 1 / 25 14".
- Dell PowerEdge 8450 User's Guide, Figure 3 "Pin Numbers for the Parallel Port Connector" (the female port on the server, drawn upright): 13 positions on the wide side numbered 13 (top) to 1 (bottom), 12 on the narrow side 25 to 14. Turned by 90° (wide side up) this is the female face above.

## Variant `parallel` — PC parallel (printer) port, IEEE 1284 compatibility-mode signals, female on the PC
Directions are for the PC (host).

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | nStrobe | Strobe, active low, from the PC: latches the data into the printer | Strobe, aktiv low, vom PC: übernimmt die Daten in den Drucker | signal |
| 2 | D0 | Data bit 0 (LSB) | Datenbit 0 (LSB) | data |
| 3 | D1 | Data bit 1 | Datenbit 1 | data |
| 4 | D2 | Data bit 2 | Datenbit 2 | data |
| 5 | D3 | Data bit 3 | Datenbit 3 | data |
| 6 | D4 | Data bit 4 | Datenbit 4 | data |
| 7 | D5 | Data bit 5 | Datenbit 5 | data |
| 8 | D6 | Data bit 6 | Datenbit 6 | data |
| 9 | D7 | Data bit 7 (MSB) | Datenbit 7 (MSB) | data |
| 10 | nAck | Acknowledge, active low, from the printer | Bestätigung, aktiv low, vom Drucker | signal |
| 11 | Busy | Busy, from the printer | Belegt, vom Drucker | signal |
| 12 | PError | Paper end / paper error, from the printer | Papierende / Papierfehler, vom Drucker | signal |
| 13 | Select | Printer selected (online), from the printer | Drucker ausgewählt (online), vom Drucker | signal |
| 14 | nAutoFd | Auto feed, active low, from the PC | Automatischer Zeilenvorschub, aktiv low, vom PC | signal |
| 15 | nError | Error (fault), active low, from the printer | Fehler, aktiv low, vom Drucker | signal |
| 16 | nInit | Initialize printer, active low, from the PC | Drucker initialisieren, aktiv low, vom PC | signal |
| 17 | nSelectIn | Select in, active low, from the PC | Auswahl, aktiv low, vom PC | signal |
| 18–25 | GND | Signal ground (eight pins, 18 to 25) | Signalmasse (acht Pins, 18 bis 25) | ground |

(Eight separate pin entries 18 … 25, each `GND`, `ground`.)

## Variant `rs232` — EIA-232-D/E (TIA-232) on DB-25
Directions: signals named from the DTE's point of view; "(DTE)" = driven by the DTE (PC/terminal), "(DCE)" = driven by the DCE (modem). A pin carries the same signal at both ends.

| Pin | Name | Function (en) | Funktion (de) | Type |
|---|---|---|---|---|
| 1 | Shield | Shield / protective (frame) ground | Schirm / Schutzerde (Gehäusemasse) | ground |
| 2 | TXD | Transmitted data, from the DTE | Sendedaten, von der DTE | data |
| 3 | RXD | Received data, from the DCE | Empfangsdaten, von der DCE | data |
| 4 | RTS | Request to send, from the DTE | Sendeanforderung, von der DTE | signal |
| 5 | CTS | Clear to send, from the DCE | Sendefreigabe, von der DCE | signal |
| 6 | DSR | DCE ready (data set ready), from the DCE | Modem bereit (DSR), von der DCE | signal |
| 7 | GND | Signal ground (common return) | Signalmasse (gemeinsame Rückleitung) | ground |
| 8 | DCD | Received line signal detector (carrier detect), from the DCE | Trägersignal erkannt (DCD), von der DCE | signal |
| 9 | Test | Reserved for testing | Reserviert für Tests | nc |
| 10 | Test | Reserved for testing | Reserviert für Tests | nc |
| 11 | Unassigned | Unassigned | Nicht belegt | nc |
| 12 | SDCD / DRS | Secondary carrier detect, or data signal rate selector, from the DCE | Sekundäres Trägersignal oder Datenraten-Wahl, von der DCE | signal |
| 13 | SCTS | Secondary clear to send, from the DCE | Sekundäre Sendefreigabe, von der DCE | signal |
| 14 | STXD | Secondary transmitted data, from the DTE | Sekundäre Sendedaten, von der DTE | data |
| 15 | TXC (DCE) | Transmitter clock (DCE source) | Sendetakt (von der DCE) | signal |
| 16 | SRXD | Secondary received data, from the DCE | Sekundäre Empfangsdaten, von der DCE | data |
| 17 | RXC | Receiver clock (DCE source) | Empfangstakt (von der DCE) | signal |
| 18 | LL | Local loopback, from the DTE | Lokale Schleife, von der DTE | signal |
| 19 | SRTS | Secondary request to send, from the DTE | Sekundäre Sendeanforderung, von der DTE | signal |
| 20 | DTR | DTE ready (data terminal ready), from the DTE | Endgerät bereit (DTR), von der DTE | signal |
| 21 | RL / SQD | Remote loopback (from the DTE) or signal quality detector (from the DCE) | Ferne Schleife (von der DTE) oder Signalqualität (von der DCE) | signal |
| 22 | RI | Ring indicator, from the DCE | Klingelsignal, von der DCE | signal |
| 23 | DRS | Data signal rate selector (DTE or DCE source) | Datenraten-Wahl (von DTE oder DCE) | signal |
| 24 | TXC (DTE) | Transmitter clock (DTE source, external clock) | Sendetakt (von der DTE, externer Takt) | signal |
| 25 | TM | Test mode, from the DCE | Testbetrieb, von der DCE | signal |

Type count: data 2, 3, 14, 16 = 4; ground 1, 7 = 2; nc 9, 10, 11 = 3; signal 16.

Notes for the app: an asynchronous PC port uses only 2, 3, 4, 5, 6, 7, 8, 20, 22 (Black Box: 7-pin async 2–4, 6–8, 20; Datapro: "all 25 pins are not usually used"). Black Box says pins 9 and 10 carry +12 V and −12 V on some equipment ("9 = +12 VDC reserved for testing, 10 = −12 VDC") — a reason to keep them `nc` and never tie them to anything.

## Sources
1. Winford Engineering, "PBC25F Physical Layout" with "DB25 Female Pinout", https://www.bipom.com/documents/peripherals/winford/pbc25f_layout.pdf, and "PBC25M Physical Layout" with "DB25 Male Pinout", https://www.bipom.com/documents/peripherals/winford/pbc25m_layout.pdf. Accessed 2026-11-24.
2. Black Box Corporation, "Technical Reference: Pinouts for Serial Interfaces", © 1997, section 1A "Original standard pinout on DB25 connectors" (EIA-232-D, V.24/V.28, ISO 2110), copy at https://ftp1.digi.com/support/images/PinoutStndrds.pdf. Accessed 2026-11-24.
3. Datapro Information Services Group (McGraw-Hill), report 2741 "ANSI EIA/TIA-232-E Interface Standard", 1995, page 3 table "EIA-232-D Interface Connector Pin Assignments" (pin, function, circuit and source, signal type, CCITT equivalent) with notes 1–4, scanned copy at https://ftp.mirrorservice.org/sites/www.bitsavers.org/pdf/datapro/communications_standards/2741_EIA_RS-232-E.pdf. Accessed 2026-11-24.
4. Dell, PowerEdge 8450 Systems User's Guide, "I/O Ports and Connectors": Figure 3 "Pin Numbers for the Parallel Port Connector" (io-3.gif) and Table 2 "Pin Assignments for the Parallel Port Connector", copy at https://cs.uwaterloo.ca/~brecht/servers/docs/PowerEdge-2600/en/Pe8450/UG/io.htm. Accessed 2026-11-24.
5. Sun Microsystems (Oracle), Sun Enterprise 220R Server Service Manual 806-1081-11, A.5.2 "Parallel Port Signals", https://docs.oracle.com/cd/E19088-01/220r.srvr/806-1081-11/z400027e1451238/index.html. Accessed 2026-11-24.
6. HP, 8924E CDMA Mobile Station Test Set User Guide, "Connectors That Begin with the Letter P" (parallel port), read at https://www.manualslib.com/manual/2689199/Hp-8924e.html?page=101 — IEEE 1284 names nStrobe, nAck, Busy, PError, Select, nAutoFd, nFault, nInit, nSelectIn. Accessed 2026-11-24.

## Cross-check
`parallel`: Dell and Sun agree on every pin (1 strobe, 2–9 data 0–7, 10 ack, 11 busy, 12 paper error, 13 select, 14 auto feed, 15 error, 16 init, 17 select in, 18–25 ground). HP agrees on the signals, names pin 15 nFault and numbers the data bits Data 1 … Data 8 (pin 9 "Data 8 (Most Significant Bit)"). Directions: Dell gives I/O for 1–9, I for 10–13 and 15, O for 14, 16, 17; Sun's descriptions give the same drivers. Name differences, not conflicts: Sun "PERROR … Parity Error" (Dell "PE Paper end", HP "PError"; Sun's own description is about the paper path, so "Parity" is a slip), Sun "SELECT_L" vs Dell "SLCT" (active level not agreed; I name it plain "Select" as HP and Dell do).

`rs232`: Datapro's EIA-232-D table and Black Box agree on all 25 pins, including the alternatives on 12 (secondary RLSD or data signal rate selector, DCE source), 21 (remote loopback or signal quality detector) and 23 (data signal rate selector, DTE or DCE source). Differences: Datapro lists pin 20 "DTE Ready CD (DCE)" — the "(DCE)" contradicts its own name and Black Box ("Control, to DCE"); taken as a slip, pin 20 is driven by the DTE. Black Box adds the ±12 V test voltages on 9 and 10. Black Box names pin 6 "DCE Ready (DSR)", Datapro "DCE Ready".

View: Winford, Black Box and Dell agree (female: 13 … 1 over 25 … 14, wide side up); male is the mirror (Winford, Black Box).

## Choices
- Catalog item: `data_av.yaml`, group `data`, outline `dsub`, `contact: round`, `drawn: socket` (female, wide side up, as `vga`), rows 13..1 and 25..14 with `offset: 0.5`; variants `parallel` and `rs232` with the same numbers; pins 9 and 10 of `rs232` stay `nc` and carry the Black Box remark about +12 V / -12 V on some equipment as a pin note, nothing more; IEEE 1284 and TIA-232-F were not read and the notes say so.
- One item with two variants, `drawn: socket`, rows `["13" … "1"]` and `["25" … "14"]` with `offset: 0.5` — the same direction as `vga` (female, wide side up).
- `views`: socket = "Female (PC parallel port; modem/DCE RS-232)", plug = "Male (RS-232 DTE port; parallel cable end)". The RS-232 gender rule (DTE male) is the usual one with exceptions, as in `rs232_db9`.
- Names: IEEE 1284-style for the parallel port (nStrobe …) because the item asks for IEEE 1284; the standard itself was not read, so they rest on HP and Dell/Sun.
- RS-232 names: the de-facto abbreviations (Black Box says the standard defines none); TXD/RXD as in `rs232_db9`.
- Timing pins (15, 17, 24) typed `signal`; pins 9, 10, 11 `nc`.

## Spot-check list
- `parallel` pin 1 = strobe (active low) — Dell Table 2 "STB#"; Sun A.5.2 "Data_Strobe_L".
- `parallel` pin 11 = Busy, pin 12 = paper end — Dell Table 2; Sun A.5.2.
- `parallel` pins 18–25 = ground — Dell Table 2; Sun A.5.2.
- `rs232` pin 2 = TXD from the DTE, pin 3 = RXD — Datapro page 3; Black Box 1A.
- `rs232` pin 7 = signal ground, pin 1 = shield — Datapro; Black Box.
- `rs232` pin 20 = DTR, pin 22 = RI — Datapro; Black Box.
- Female face 13 upper left … 1 upper right, 25 lower left … 14 lower right — Winford PBC25F; Black Box front view.

UNVERIFIED: IEEE 1284 and TIA-232-F were not read; the active level of pin 13 (Select) differs in Sun's name; on bidirectional (EPP/ECP) ports pins 2–9 and several control lines change roles — only compatibility mode is given.
