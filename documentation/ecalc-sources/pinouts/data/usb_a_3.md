# usb_a_3 — USB 3.x Type-A / USB 3.x Type-A

Kind: free official specification
Pins: 9; types: power 1, ground 2, data 6

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): upper row 9 8 7 6 5, lower row 1 2 3 4, both left to right.

Basis, seen in the figures (extracted from the PDF and looked at): figure 5-4, detail G, is the front view of the Standard-A plug and labels `5 6 7 8 9` left to right above `4 3 2 1` left to right. The receptacle is its mirror image, which gives the order above. Check against a second drawing: figure 5-5 (example footprint "PCB layout, standard mount type", connector front edge at the bottom of the drawing) has pins 9 8 7 6 5 in the rear row and 1 2 3 4 in the front row, left to right. Read from the component side, which the drawing does not state, that puts pin 1 on the left seen from the front of the receptacle, in line with detail G. Figure 5-1 shows the receptacle with the tongue in the upper half.

The half-pitch offset follows the dimensions of detail G: pins 5 to 9 at ±4.00, ±2.00 and 0 mm, pins 1 to 4 at ±3.50 and ±1.00 mm from the centre line.

The rows are not at one depth. Top and bottom in the drawing follow detail G and are a face-on schematic; `notes` says so.

## Sources
1. USB 3.1 Legacy Cable and Connector Specification, Revision 1.0, 22 September 2017, USB 3.0 Promoter Group, https://www.usb.org/document-library/usb-31-legacy-cable-and-connector-revision-10 — table 5-2 (pins, signal names, mating sequence), figures 5-1, 5-4 and 5-5. Accessed 2026-11-04.

## Cross-check
One free official specification is enough. Pins 1 to 4 agree with USB 2.0 table 6-1.

UNVERIFIED: no wire colours. Table 5-6 of this revision lists wires by number and signal only, so `wire` is left out everywhere in this item, also on pins 1 to 4.

## Choices
- Title "3.x": chapter 5 of USB 3.2 Revision 1.1 (usb_32_202206.zip, read) refers to this legacy document for the Standard-A connector, so the pinout also holds for USB 3.2 Type-A.
- Outline `usb_a` as assigned.
- Shell left out of the pins (table 5-2 lists it as "Shield") and named in `notes`.
- Note 2 of table 5-2 says Tx and Rx are defined from the host perspective; repeated in `notes`.
- VBUS: the table says only "Power", so no voltage in `function`.
