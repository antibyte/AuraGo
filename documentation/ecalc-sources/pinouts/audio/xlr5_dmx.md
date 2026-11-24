# xlr5_dmx — DMX512 on 5-pin XLR / DMX512 an 5-poligem XLR

Kind: cross-checked reproductions (two lighting makers + a 1999 working draft of the standard); geometry from Neutrik drawings
Pins: 5; types: ground 1, data 4

## View
Written for the male connector, looking at its mating face (`drawn: plug`), latch cut-out at 12 o'clock (`key: 0`), as in the `xlr` item. Positions (clockwise from 12 o'clock, male face), radius 0.55: pin 1 = 280, pin 2 = 230, pin 3 = 180, pin 4 = 130, pin 5 = 72 degrees. The female face follows from the mirror rule (1 = 80, 2 = 130, 3 = 180, 4 = 230, 5 = 288).

Derivation (read and measured):
- Neutrik drawing NC5MAH1 (drawing ID 20003685, rev. A, 24.07.2024), front view of the male chassis connector, latch window at the top: 1 upper left, 5 upper right, 2 lower left, 4 lower right, 3 straight below the centre.
- Neutrik drawing NC5FAH1 (drawing ID 20003668, rev. A, 24.07.2024), front view of the female chassis connector: exactly the mirror image. Measured on the rendered drawing (centroid of each contact ring): 1 = 79.6, 2 = 129.5, 3 = 179.5, 4 = 230.0, 5 = 288.2 degrees, all at about 0.55 of the insert radius. The male drawing agrees within about 5 degrees.
- The pattern is not symmetric: pins 1 to 4 about 50 degrees apart, pin 5 about 58 degrees from pin 4. Both drawings show it in mirror image, so it is real.

## Sources
1. ESTA / USITT, "BSR E1.11, Entertainment Technology - USITT DMX512 Asynchronous Serial Digital Data Transmission Standard for Controlling Lighting Equipment and Accessories", Revision 1.1a (working draft CP98/1031r1.1a), printed Nov 23, 1999, read in the copy at https://mikrocontroller.net/attachment/4868/DMX_512.PDF — table 2.8.2.2 "Signal designations summary", clauses 1.2.9, 10.1, 10.1.1; a work-in-progress document, not the published standard; accessed 2026-11-24
2. Leprecon, ULD-360DMX user manual, section 8 "DMX 512 Information" (page 16), read at https://www.manualsdir.com/manuals/777543/leprecon-uld-360dmx.html?page=16 — pins 1 to 5 and genders, "a summary of the USITT 1990 standard"; accessed 2026-11-24
3. Doug Fleenor Design, 2322DMX user manual, read at https://www.manualsdir.com/manuals/557970/doug-fleenor-design-2322dmx.html — pin 1 common, 2 DMX data minus, 3 DMX data plus, 4 and 5 passed from input to output; accessed 2026-11-24
4. ETC, Gadget setup manual, page 2, read at https://www.manualslib.com/manual/2102415/Etc-Gadget.html?page=2 — "DMX512 pinout for five-pin XLR female": Common (Shield), Data -, Data +, not connected, not connected (pin numbers only by list order in the text extraction); accessed 2026-11-24
5. Neutrik, drawing NC5MAH1, https://www.neutrik.com/media/20013/download/Drawing%20NC5MAH1.pdf?v=1 — male front view with pin numbers; accessed 2026-11-24
6. Neutrik, drawing NC5FAH1, https://www.neutrik.com/media/19982/download/Drawing%20NC5FAH1.pdf?v=1 — female front view with pin numbers; accessed 2026-11-24

Not reached: the published ANSI E1.11-2008 (R2018) (ESTA TSP and the ANSI preview answered 403); ETC's "DMX 512 Info" page shows the pinout only in images; Pathway's DMX wiring guide (502).

## Cross-check
Pins 1 to 3: the ESTA/USITT draft, Leprecon and Doug Fleenor Design agree (Common, Data 1-, Data 1+); ETC lists the same order. Pins 4 and 5: Leprecon "optional Data 2-/2+", the draft "Data 2-/Data 2+" for TYPE 2 ports but other uses for other TYPEs, DFD passes them through, ETC leaves them open: the names agree, the use differs.

Genders: draft clause 10.1 and Leprecon agree (female on transmitters and loop-through, male on receivers).

## Choices
- Catalog item: `ground` for pin 1 (shield and signal reference), `data` for the EIA-485 pairs.
- Pins 4 and 5 kept as `data` with the function "optional second data link (Data 2 -/+); often not connected". The 1999 draft lists other uses for older products by port type, including a supply for legacy products, which no second source confirmed; the catalog therefore says only "older equipment may use pins 4/5 differently; check the device manual", without numbers. The `nc` type would be the safe alternative.
- Names `Data 1-` etc. as Leprecon and the draft write them (plain hyphen).
- `drawn: plug` (male face) to match the `xlr` item; the notes say that DMX outputs are female and inputs male.
- No `wire`: no source names colours.

## Spot-check list
- Pin 1 = data link common: ESTA/USITT draft table 2.8.2.2; Leprecon page 16.
- Pin 2 = Data 1-, pin 3 = Data 1+: draft table 2.8.2.2; Leprecon page 16; DFD 2322DMX.
- Pins 4/5 = optional Data 2-/2+: Leprecon page 16; draft table 2.8.2.2 (TYPE 2).
- Female on transmitters, male on receivers: draft clause 10.1; Leprecon page 16.
- Pin 3 straight below the centre, 1 upper left, 5 upper right on the male face: Neutrik NC5MAH1 front view.

Unverified: the wording of the published ANSI E1.11 for pins 4 and 5. The pins 1 to 3 assignment does not depend on it.
