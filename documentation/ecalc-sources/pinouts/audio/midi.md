# midi — MIDI (DIN-5, 180°) / MIDI (DIN-5, 180°)

Kind: free official specification
Pins: 5; types: nc 2, ground 1, data 2

## View
Written for the female device jack, looking at its mating face from the front (`drawn: socket`), notch at 12 o'clock.

Angles (clockwise from 12 o'clock, jack face): **pin 1 = 90°, pin 4 = 135°, pin 2 = 180°, pin 5 = 225°, pin 3 = 270°, radius 0.53; key = 0°.** The plug face follows from the mirror rule: pin 1 = 270°, pin 4 = 225°, pin 2 = 180°, pin 5 = 135°, pin 3 = 90°.

Derivation:
- binder catalogue, series 423/723, page 34, "5 pol Stereo (DIN 41524)", with coordinates in mm. "Buchseneinsatz (Steckseite) / Female insert (mating side)": 1 (-3.50, 0), 2 (0, 3.50), 3 (3.50, 0), 4 (-2.47, 2.47), 5 (2.47, 2.47). "Stifteinsatz (Steckseite) / Male insert (mating side)": 1 (3.50, 0), 2 (0, 3.50), 3 (-3.50, 0), 4 (2.47, 2.47), 5 (-2.47, 2.47). The notch is at the bottom. Turned by 180° (a rotation, no mirror) the female values give the angles above.
- Switchcraft catalogue: the arrangement drawings for 5 pins at 180° (pin arrangement 5, contact arrangement F) have the notch at 12 o'clock and the contacts in the lower half; I keep that orientation. The "Recommended PC board layout (component side)" of the 57PC5F, the part family CA-033 names, shows the tails labelled in the order (1), 4, 2, 5, 3 from the top of the drawing to the bottom with the panel to the right. Seen from the front of the panel, the top of that drawing is on the viewer's right. So pin 1 is on the right of the jack's front face, which agrees with binder. (The digit 1 itself is not legible in the print; the other four are.)
- Radius: 3.5 mm (binder; Switchcraft: pin circle 7.00 mm) of a 6.6 mm shell radius (Switchcraft: 13.20 mm), so 0.53.

**The common mistake:** CA-033 figures 1 and 2 draw the jack with 1 at the left and 3 at the right, and figure 3 uses the same handedness for the plugs. CA-033 does not name the view. It matches the plug's mating face and the jack's solder side, and it does not match the jack's mating face. Taken for the front of a jack, it would be mirrored.

## Sources
1. MMA/AMEI, CA-033, "MIDI 1.0 Electrical Specification Update [2014]", copy on the MIDI Association site, https://midi.org/wp-content/uploads/wpforo/default_attachments/1709416667-ca33-MIDI-10-Electrical-Specification-Update.pdf — jacks and cable: "DIN 5-pin (180 degree) female panel-mount", pins 1 and 3 unused, pin 2 grounded at the transmitter only, current loop and figures 1 to 3, accessed 2026-11-04
2. binder, catalogue M16 IP 67, "Miniatursteckverbinder Serie 423 - 723" contact arrangements (page 34), copy at https://static.chipdip.ru/lib/959/DOC027959862.pdf — coordinates of male and female inserts, each labelled mating side, accessed 2026-11-04
3. Switchcraft, catalogue pages 46 to 49, "DIN connectors and plugs" and "DIN receptacles", copy at https://assets.sourcengine.com/datasheets/80db8068-6e03-493b-82e0-1ecaa5f26323.pdf — arrangement drawings and the 57PC5F board layout, accessed 2026-11-04

## Cross-check
One free official specification is enough for the functions. For the geometry, binder and Switchcraft are independent and agree. I found no disagreement in what I read. Pages that assign pin 4 as sink at MIDI Out were seen only as search excerpts and not used; CA-033 figure 1 is clear that pin 4 goes to VTX through RA and pin 5 to the UART through RC.

## Choices
- The MIDI spec gives pins 4 and 5 no names. `Loop +` and `Loop -` are mine: the current flows from pin 4 through the receiver's opto-isolator back to pin 5 (figure 2).
- `N/C` is how CA-033 writes it. CA-033 adds that a cable may have wires on pins 1 and 3.
- `data` for pins 4 and 5: a serial data line.
- `ground` for pin 2: a shield. At MIDI In it is not grounded; the function and the notes say so.
- Item names are the same in all locales: MIDI and DIN are proper names, and there is no generic word in the name.
