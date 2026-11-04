# usb_a — USB 2.0 Type-A / USB 2.0 Type-A

Kind: free official specification
Pins: 4; types: power 1, ground 1, data 2

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): 1 2 3 4 left to right, contact tongue in the upper half of the shell, contacts on its underside.

Basis: the text layer of the interface drawings, read by position. USB 2.0 figure 6-7 ("USB Series "A" Receptacle Interface") has the labels `1 23 4` left to right under the face view. Figure 6-9 (Series A plug, "Section A - A") has `4 32 1` left to right. The two are mirror images, which is what the catalog schema's other view produces. I rendered the same receptacle sheet from ECN #1 (page 17) and the USB 3.1 legacy figure 5-1 and looked at them: tongue on top, contacts below it.

## Sources
1. Universal Serial Bus Specification, Revision 2.0, USB-IF, file usb_20.pdf in usb_20_20250603.zip, https://www.usb.org/document-library/usb-20-specification — table 6-1 "USB Connector Termination Assignment" (contacts, signal names, typical wire colours), figures 6-7 and 6-9. Accessed 2026-11-04.

## Cross-check
One free official specification is enough. No disagreement. The USB 3.1 legacy specification, table 5-2 (read for usb_a_3), repeats pins 1 to 4.

## Choices
- The shell (table 6-1: "Shield", "Drain Wire") is not a pin in the list; it is in `notes`.
- Colours are the column "Typical Wiring Assignment".
- `VBUS` function "nominally +5 V" comes from section 4.2.1 of the same specification ("nominally +5 V at the source").
