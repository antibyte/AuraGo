# usb_micro_b — USB Micro-B / USB Micro-B

Kind: free official specification
Pins: 5; types: power 1, ground 1, data 2, signal 1

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): 1 2 3 4 5 left to right, wider side of the shell at the top.

Basis: part seen, part derived. Figure 4-7, "Micro-B Plug Interface" (extracted from the PDF and looked at), has the front view of the plug with the callouts "No.5" on the left and "No.1" on the right, wider side at the top. Figure 4-10, "Micro-B receptacle interface", shows the receptacle the same way up but does not label the contacts. So the receptacle order is derived: it is the mirror image of the plug, because the two faces look at each other. The plug view of the item gives exactly the labelled figure 4-7.

## Sources
1. Universal Serial Bus Micro-USB Cables and Connectors Specification, Revision 1.01, 4 April 2007, file Micro-USB_1_01.pdf in usb_20_20250603.zip, https://www.usb.org/document-library/usb-20-specification — table 4-2 "Micro-A Plug Pin Assignments" and the text below it on the ID pin, figures 4-7 and 4-10. Accessed 2026-11-04.

## Cross-check
One free official specification is enough. No disagreement.

The only table in the document is written for the Micro-A plug. Its numbers, names and colours are carried to Micro-B for pins 1, 2, 3 and 5, because the same contact numbers appear on the Micro-B drawing. Pin 4 differs: the text under the table describes it for both plugs, and note 1 of figure 4-7 says pin 4 is not connected to pin 5 inside the plug.

The PDF text layer lost the ohm sign in "Ra_PLUG_ID (10 MAX)"; I read it as 10 Ω and "100k MIN" as 100 kΩ.

## Choices
- Pin 4 is `signal`: on a Micro-B plug it may be open, but the receptacle contact is the sense input of an On-The-Go device.
- Shell left out of the pins and named in `notes`.
- `drawn: socket` although the labelled figure is the plug, so that all USB items open on the receptacle.
