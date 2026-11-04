# hdmi_a — HDMI Type A / HDMI Type A

Kind: cross-checked reproductions
Pins: 19; types: data 11, ground 5, power 1, signal 2

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): upper row 19 17 … 1, lower row 18 16 … 2, both left to right, wider side of the shell at the top.

Basis, seen in figures and read in one text layer (the HDMI specification itself was not read):
- TRU Components 15 71 072, "HDMI Terminal Block Adaptor, Type A, Receptacle": beside the face view its text layer has `19 17 15 13 11 9 7 5 3 1` above `18 16 14 12 10 8 6 4 2`.
- Sony VPL-PHZ50 help guide, picture of the "HDMI, Female" terminal: 19 upper left, 1 upper right, 18 lower left, 2 lower right, chamfers at the bottom.
- Barco Event Master user guide, image A-4 "HDMI connector" (a device port): the same rows, numbered in the picture.
- Amphenol drawing 191-PLC-1038-00: receptacle face drawn upright, "Pin 1" and "Pin 19" on the wide side of the tongue, "Pin 2" and "Pin 18" on the chamfered side, pins 1 and 2 at the top. Turned a quarter clockwise so that the wide side is up, this is the same arrangement. This part has a receptacle at both ends, so the view is a receptacle face in either case.

The half-pitch offset of the lower row is seen in the Barco picture: 18 lies between 19 and 17.

## Sources
1. Amphenol Socapex, drawing 191-PLC-1038-00, "HDMIF TV 7 x x", receptacle, rev A, https://mm.digikey.com/Volume0/opasdata/d220001/medias/docus/2593/HDMI%20FTV%207%20x%20x.pdf — pin table, face drawing with pins 1, 2, 18, 19. Accessed 2026-11-04.
2. Rohde & Schwarz, HDMI poster, 3606.8489.82, version 03.02, June 2014, https://scdn.rohde-schwarz.com/ur/pws/dl_downloads/dl_common_library/dl_brochures_and_datasheets/pdf_1/HDMI_po_en_3606-8489-82_v0302.pdf — table "Pin assignment of HDMI connectors (type A and E)". Accessed 2026-11-04.
3. Schlegel, RRJ_HDMI_STB data sheet, 17.05.2022, https://schlegelusa.com/wp-content/uploads/product-documents/RRJ_HDMI_STB-Datasheet.pdf — pin table with HEC notes. Accessed 2026-11-04.
4. TRU Components (Conrad), 15 71 072 HDMI Terminal Block Adaptor, V1_07092021_01_en, https://static.rapidonline.com/pdf/20-2627_v1.pdf — pin table, rows. Accessed 2026-11-04.
5. Barco, Event Master Devices user guide R5905948, A.24 Standard connector pinouts, https://www.barco.com/manuals/R5905948/eriqm1q9ogv8.html — HDMI table and image A-4. Accessed 2026-11-04.
6. Sony, VPL-PHZ50 help guide, Pin Assignment (HDMI Terminal), https://helpguide.sony.net/vpl/phz50/v1/en/contents/TP1000002596.html — table and picture. Accessed 2026-11-04.
7. Sony Corp, patent EP2405654B1, https://patents.google.com/patent/EP2405654B1/en — names the HPD line (HEAC- line) on pin 19 and the utility line (HEAC+ line) on pin 14. Accessed 2026-11-04.
8. Wikipedia, HDMI, https://en.wikipedia.org/wiki/HDMI — pin table, only as a further check of pins 14 and 19. Accessed 2026-11-04.

## Cross-check
Pins 1 to 12, 15, 16, 17 and 18 agree in sources 1 to 6 (spelling aside).

Disagreements:
- HEAC polarity. Schlegel, the Sony patent and Wikipedia: + on pin 14, - on pin 19. Rohde & Schwarz: HEC Data- on 14 and HEC Data+ on 19. Barco: HEC Data+ on 19, pin 14 blank. I chose + on 14, - on 19: three sources, among them a patent by an HDMI founder that names both lines explicitly. Rohde & Schwarz and Barco are the minority here and may share an origin.
- Pin 13. Sony's projector page says "N.C."; all others say CEC. That page describes one product that does not use CEC. CEC chosen.
- Pin 14. Amphenol "Utility"; TRU and Sony "Reserved"; Rohde & Schwarz and Schlegel "reserved up to 1.3, HEC from 1.4". All compatible; name `Utility` chosen, the history is in `function`.
- +5 V current. Rohde & Schwarz and Schlegel name 55 mA, Barco and Wikipedia 50 mA. Not resolved from what I read (the specification is paywalled), so `notes` gives both and `function` no number.

## Choices
- Names as the sources write them ("TMDS Data2+"), not the catalog schema's sample "TMDS D2+". A name fits in 24 characters.
- CEC is `data` (a one-wire bus). `Utility` is `signal` because it has no one bus role across versions. It is not `nc`: from HDMI 1.4 it can be in use.
- Shields are `ground` by the catalog schema.
- Name kept as "HDMI Type A" in all locales: treated as a proper name, like USB Type-A in the plan.
