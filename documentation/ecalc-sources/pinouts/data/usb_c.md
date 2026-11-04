# usb_c — USB Type-C / USB Type-C

Kind: free official specification
Pins: 24; types: power 4, ground 4, data 12, signal 4

## View
Written for the receptacle, looking at its mating face (`drawn: socket`): upper row A1 … A12 left to right, lower row B12 … B1 left to right.

Basis, seen in the figures (extracted from the PDF and looked at): figure 2-1 is captioned "USB Type-C Receptacle Interface (Front View)" and shows exactly these rows. Figure 2-2, "USB Full-Featured Type-C Plug Interface (Front View)", shows A12 … A1 above B1 … B12, which is the mirror image the catalog schema's other view produces.

## Sources
1. USB Type-C Cable and Connector Specification, Release 2.5, March 2026, USB 3.0 Promoter Group, https://www.usb.org/document-library/usb-type-cr-cable-and-connector-specification-release-25 — figures 2-1 and 2-2; table 3-4 (pins and descriptions, with notes on B6/B7 and tied contacts); table 3-5 (USB 2.0 only); table 4-27 "Source CC Termination (Rp) Requirements"; table 4-28 "Sink CC Termination (Rd) Requirements". Accessed 2026-11-04.

## Cross-check
One free official specification is enough. Figure 2-1 and table 3-4 agree on every pin.

Values from table 4-27: default USB power 56 kΩ ±20% (to 4.75–5.5 V), 36 kΩ ±20% (to 3.3 V ±5%); 1.5 A at 5 V 22 kΩ ±5%, 12 kΩ ±5%; 3.0 A at 5 V 10 kΩ ±5%, 4.7 kΩ ±5%. Note 1: 56 kΩ ±5% in a plug on a Type-C to legacy Standard-A assembly, a Type-C to Micro-B receptacle adapter or a captive cable to a host.

Values from table 4-28: ±20% resistor 5.1 kΩ cannot detect power capability, ±10% resistor 5.1 kΩ can.

Search excerpts of a vendor page (Infineon) showed the same Rp and Rd values; I did not use it as a source.

## Choices
- Names follow figure 2-1 (`TX1+`, `D+`, `CC1`), not the table's `TXp1`, `Dp1`: people look for the former. The table's names are in the vector `ref`. ASCII hyphen instead of the figure's minus sign, so a search finds them.
- `SBU` is `signal`: the table gives only "Sideband Use", with no fixed bus. The catalog schema names CC as `signal`.
- The other two ways of building a source (current source 80 µA, 180 µA, 330 µA) and the 1.1 V clamp as Rd are left out of `notes`.
- Plug differences are in the pin `notes` of A5, B5, B6 and B7: figure 2-2 labels A5 "CC", B5 "VCONN", and B6/B7 empty; table 3-4 note 1 says B6 and B7 should not be present in the plug.
- "Thunderbolt" in the tags is a search term only, not from the specification.
