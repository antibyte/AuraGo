# displayport — DisplayPort / DisplayPort

Kind: cross-checked reproductions
Pins: 20; types: data 10, ground 6, signal 3, power 1 (each variant)

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): upper row 19 17 … 1, lower row 20 18 … 2, both left to right, cut corner of the shell at the lower left.

Basis, seen in three pictures (the VESA standard itself was not read):
- Barco Event Master user guide, image A-1 "DisplayPort connector": device port, numbered 19 … 1 above 20 … 2, cut corner at the lower left.
- Barco MDSC-8255 user guide, image 2-8, "Overview (sink side pin-out)": the same arrangement.
- University of Oregon page, picture beside its source-side table: the same arrangement.

The upper row is shifted to the right by about half a pitch against the lower: in the pictures 19 lies right of 20, and 1 right of 2. So `offset: 0.5` goes on the upper row.

The view is implied more than stated. Wikipedia captions the same arrangement "Pinout as looking at source side connector". Otherwise no text says "viewed from the mating side": the pictures are of device ports in device manuals, with the contacts on a central tongue, which is what a receptacle has. No maker drawing of a bare connector was found: a Molex drawing URL timed out, and the search turned up nothing else I could open.

## Sources
1. NCR Voyix, 7772 workstation online help, DisplayPort pinout, https://onlinehelp.ncrvoyix.com/Retail/Workstations/7772/HTML/Topics/CircuitBoard/PinoutDisplayPort.htm — pin table of a PC's output. Accessed 2026-11-04.
2. Barco, Event Master Devices user guide R5905948, A.24 Standard connector pinouts, https://www.barco.com/manuals/R5905948/eriqm1q9ogv8.html — pin table and image A-1. Accessed 2026-11-04.
3. Barco, MDSC-8255 user guide, 2.4.5 DisplayPort connector, https://www.barco.com/manuals/R5911693/yf14wdmancf9.html — "Overview (sink side pin-out)", list and picture. Accessed 2026-11-04.
4. Dell, P3424WEB user's guide, page "Pin assignments", read at https://www.manualslib.com/manual/3349470/Dell-P3424web.html?page=25 — table of a monitor's "DP port (in)". Accessed 2026-11-04.
5. University of Oregon, Projection Design Bootcamp, Display Port Information, https://blogs.uoregon.edu/projectiondesignpdw/?p=178 — source-side table, note on CONFIG pins, note on the sink side. Accessed 2026-11-04.
6. Wikipedia, DisplayPort, https://en.wikipedia.org/wiki/DisplayPort — caption of the pin diagram, only as a further check of the view. Accessed 2026-11-04.

## Cross-check
Source side: NCR, Barco Event Master and University of Oregon agree on all 20 pins.

Sink side: Barco MDSC-8255 and Dell agree on all 20 pins, and the University of Oregon note describes the same reversal: lane 3 on pins 1 and 3, lane 0 on pins 10 and 12.

Disagreements, all small:
- CONFIG1/CONFIG2. NCR and Dell give only the names. Barco Event Master says "connected to Ground". University of Oregon says directly to ground or through a pull-down device. The wider wording is used at the source side.
- UNVERIFIED: whether the same ground rule holds at a sink. Sink-side sources only give the names, so `function` there says only "Configuration pin".
- Spelling: Dell writes ML3(n), Barco ML_Lane 3 (n). Barco's and NCR's form is used. Barco MDSC writes "Hot Plug", the others "Hot Plug Detect".
- The University of Oregon page probably copies an older public table, so I do not count it as an independent witness; NCR and Barco are the two independent sources.

## Choices
- **Variants `source` and `sink` go beyond the catalog schema's list**, which names variants only for rj45. A single source-side list would be wrong by four lanes for anyone wiring a monitor input. The shape is shared, as the catalog schema requires.
- `Return` is `ground`: it is the return of DP_PWR.
- Not stated: how a cable connects the two ends. I read no source that spells out the wiring of a cable.
