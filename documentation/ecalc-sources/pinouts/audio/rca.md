# rca — RCA (phono, cinch) / Cinch (RCA)

Kind: cross-checked reproductions (two makers' manuals)
Pins: 2; types: signal 1, ground 1

## View
`circular`, drawn as the plug's mating face: centre pin at radius 0, the shell as one contact on the rim. The loader allows radius 0 to 1, so the shell sits at the largest allowed radius (1, angle 180). The shell is a ring; the drawing shows it as one contact. The face is rotationally symmetric: no key, no mirror question. The views plug and socket exist because the shape is drawn as plug; both captions are generic.

An outline of the `jack` shape (phone plug) would be wrong for RCA: a phone plug is a rod with segments, an RCA plug is a pin inside a ring. Mackie calls the RCA plug "electrically equivalent to a 1/4" TS phone plug", which is why the assignment follows tip and sleeve.

## Sources
1. Mackie, 1620 owner's manual, Appendix B "Connections" (page 23), https://www.manualslib.com/manual/97163/Mackie-1620.html?page=23 — "RCA Unbalanced Wiring: Sleeve = Shield, Tip = Hot (+)"; "electrically equivalent to a 1/4" TS phone plug"; accessed 2026-11-24
2. Garmin, Primacy S3 Active Loudspeaker installation instructions, section "RCA or Unbalanced Input", https://www8.garmin.com/manuals/webhelp/GUID-D32FB5FE-755E-4DA5-B931-9D847AA75F5C/EN-US/GUID-978FF75B-9273-4C94-B133-03BC02DB0DDF.html — "Tip: Positive, Sleeve: Negative"; accessed 2026-11-24
3. RS Components, "A Complete Guide to RCA Connectors", 8 Sept 2026, https://uk.rs-online.com/web/content/product-knowledge/electrical-electronics/cables-connectors/rca-connectors-guide — plug = "protruding pin encircled by a raised ring", jack = "a hole in the centre of a corresponding ring"; colour codes; no pin assignment; accessed 2026-11-24

Not reached: a connector maker's RCA drawing with terminal labels (CUI 403; REAN drawing not opened); Audio-Technica's blog (certificate error); Monoprice guides (403).

## Cross-check
Mackie and Garmin, two independent makers, agree: the centre contact (both call it the tip) is the positive/hot signal, the outer contact (sleeve) is shield/negative. RS confirms the geometry (centre pin inside a ring) but not the assignment.

## Choices
- Catalog item: contact ids `T` (centre pin, "tip") and `S` (shell, "sleeve"), the terms of the Mackie and Garmin sources; no numbering exists. The longer ids `Pin`/`Shell` did not fit inside the drawn contact.
- `signal` for the centre pin, `ground` for the shell (Garmin's "Negative" is the signal return of an unbalanced line; Mackie's "Shield").
- Colour codes (RS: white left, red right, yellow video) are a convention, mentioned only in the item notes; no `wire`.

## Spot-check list
- Centre (tip) = Hot (+): Mackie 1620 Appendix B, page 23.
- Shell (sleeve) = Shield: Mackie 1620 Appendix B, page 23.
- Tip positive, sleeve negative: Garmin Primacy S3, "RCA or Unbalanced Input".
- Plug = pin encircled by a ring: RS Components guide.

Unverified: none for the assignment.
