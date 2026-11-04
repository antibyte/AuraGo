# usb_mini_b — USB Mini-B / USB Mini-B

Kind: free official specification
Pins: 5; types: power 1, ground 1, data 2, signal 1

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): 1 2 3 4 5 left to right, wider side of the shell at the top.

Basis, seen in the drawings (rendered from the PDF and looked at): figure 6-11, "Series "Mini-B" Receptacle Interface Drawing", front view with the callouts "Pin 1" on the left and "Pin 5" on the right, shell wider at the top. Figure 6-15, "Series 'Mini-B' Plug Interface Drawing", front view with "Pin 5" on the left and "Pin 1" on the right, same way up. The two are mirror images.

## Sources
1. USB 2.0 Specification Engineering Change Notice (ECN) #1: Mini-B connector, 20 October 2000, file ecn1-usb20-miniB-revd.pdf in usb_20_20250603.zip, https://www.usb.org/document-library/usb-20-specification — table 6-2 "USB Series "mini-B" Connector Termination Assignment", figures 6-11 and 6-15 with note 1 of the plug drawing. Accessed 2026-11-04.

## Cross-check
One free official specification is enough. No disagreement. Table 6-2 lists contact 4 as "ID", wiring "not connected"; note 1 of figure 6-15 says pin 4 is installed but goes to no wire of the cable.

## Choices
- Pin 4 is `signal`, not `nc`: the contact is real and has a name, only the cable leaves it open.
- No `wire` on pin 4, the table names no colour for it.
- Shell left out of the pins and named in `notes`.
