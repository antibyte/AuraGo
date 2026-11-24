# trailer_7 — 7-pin trailer socket ISO 1724 (12N) / 7-polige Anhängersteckdose ISO 1724 (12N)

Kind: cross-checked maker documents (two connector makers: Erich Jaeger, M&K)
Pins: 7; types: power 6, ground 1
Status: complete for contacts, functions, designations, colours and positions; key position not verified
Group: vehicle_mains

## Pins
| No. | Name | Function (en) | Function (de) | Type | Wire (core colour, "for information") |
| --- | --- | --- | --- | --- | --- |
| 1 | L | Left-hand direction indicator | Fahrtrichtungsanzeiger links | power | yellow |
| 2 | 54g | Rear fog light | Nebelschlussleuchte | power | blue |
| 3 | 31 | Earth (common return), 2.5 mm² | Masse (gemeinsame Rückleitung), 2,5 mm² | ground | white |
| 4 | R | Right-hand direction indicator | Fahrtrichtungsanzeiger rechts | power | green |
| 5 | 58R | Right-hand rear position and marker lights, number-plate light | Rechte Schlussleuchte, Umriss- und Begrenzungsleuchte, Kennzeichenleuchte | power | brown |
| 6 | 54 | Stop (brake) lights | Bremsleuchten | power | red |
| 7 | 58L | Left-hand rear position and marker lights, number-plate light | Linke Schlussleuchte, Umriss- und Begrenzungsleuchte, Kennzeichenleuchte | power | black |

Cross-sections: 1.5 mm² for all lamp circuits, 2.5 mm² for contact 3 (both sources).

## View
Written for the vehicle's socket, looking into its mating face (cover open), contact 1 at 12 o'clock: `drawn: socket`. Circle positions, angle clockwise from 12 o'clock as seen on the socket face, radius relative to the contact insert:

| Contact | Angle | Radius |
| --- | --- | --- |
| 1 (L) | 0° | 0.6 |
| 2 (54g) | 60° | 0.6 |
| 3 (31) | 120° | 0.6 |
| 4 (R) | 180° | 0.6 |
| 5 (58R) | 240° | 0.6 |
| 6 (54) | 300° | 0.6 |
| 7 (58L) | centre | 0 |

The other view (mirror) is the trailer's plug face (pins) and also the socket seen from its wiring side: 2 at 300°, 3 at 240°, 5 at 120°, 6 at 60°; 1, 4 and 7 unchanged.

Basis (Jaeger, three figures, all rendered and looked at, mutually consistent):
- "Contact allocation of the 7-pin socket according to ISO 1724 with color coding (rear view)": 1 yellow at the top, 2 blue upper left, 3 white lower left, 4 green at the bottom, 5 brown lower right, 6 red upper right, 7 black in the centre. Measured from the centre: the six outer contacts at the same radius (125 px), 60° apart.
- "Contact allocation of the 7-pin plug according to ISO 1724 with color coding" (plug face, pins): the same arrangement as the socket rear view (1 top, 2 upper left, 3 lower left, 4 bottom, 5 lower right, 6 upper right, 7 centre), with the labels "1 L, 2 54g, 3 31, 4 R, 5 58R, 6 54, 7 58L".
- "Trailer socket according to ISO 1724 with contact allocation in the cover": the cover print reads 1/L top, 2/54g upper right, 3/31 lower right, 4/R bottom, 5/58R lower left, 6/54 upper left, 7/58L centre, which is the mirror of the rear view, i.e. the socket face. This confirms the drawn view directly.
- Radius 0.6: the outer contacts sit at about 125 px of an insert radius of about 190 px in the rear view (0.66) and about 0.55 of the shell in the plug render; 0.6 is an approximate drawing value, not a dimension from the standard.

## Sources
1. Erich Jaeger GmbH + Co. KG (trailer connector maker), "ISO 1724: The international standard for 7-pin trailer connectors" (scope and application page), https://www.erich-jaeger.com/en/products/standards/iso-1724-type-n/iso-1724-type-n-scope-and-application — contact table (contact no., function, cross-section, core colour, DIN 72552 designation), "12 V nominal supply voltage", and the three figures: https://www.erich-jaeger.com/fileadmin/_processed_/f/8/csm_ISO1724-Socket-Front_a44bc1bc9e.png, .../6/2/csm_ISO1724-Socket-Rear_964ab65d76.png, .../3/f/csm_ISO1724-Plug-Front_d5b0448bee.png. Accessed 2026-11-24.
2. M&K (connector maker, logo "M&K"; catalogue pages 14 and 15, "7-polige 12-Volt-Steckvorrichtung ISO 1724 / ISO 3732", distributed by Conrad as item 808849), https://asset.conrad.com/media10/add/160267/c1/-/GL/000808849DS01/000808849DS01-GL.pdf — "Kontaktbelegung / Contact allocation, Connector N-type according to ISO 1724": contact no./designation (designation "nach DIN 72 577"), mm², circuit, core insulation colour "(for information)"; ISO 3732 S-type table; plug part numbers 6.00774.000 etc. Rendered and read. Accessed 2026-11-24.

Not read: ISO 1724 itself (paywalled). Hella's partner catalogue (https://www.hella.com/partnerworld/assets/documents/10044799a_de.pdf) was too large for the fetch tool (over 10 MB).

## Cross-check
- Contacts 1 to 7: functions, designations, cross-sections and colours agree between Jaeger and M&K, pin by pin.
- Designation standard: Jaeger calls the designations "DIN 72552", M&K "nach DIN 72 577". Same designations either way.
- Footnote on the number-plate light: M&K's English and French footnote says no lamp may be connected to both contacts 5 and 7; the German footnote on the same page says "Kontakten 2 und 6", an error in the German text. Do not copy the German footnote.
- Positions: only Jaeger draws them (three consistent figures). M&K's page shows only outer dimensions. One maker's figures for the positions, consistent with the maker's own cover print.
- Voltage: Jaeger "12 V nominal"; M&K title "12-Volt".

## Choices
- Catalog item: `circular`, `contact: round`, `drawn: socket`, no `key` (the key position is not verified, so no notch is drawn; the notes say so); positions as above with contact 7 at radius 0 (the shape accepts 0). The item notes carry the one-sentence safety line (qualified people, check the actual connector), the 12N/12S distinction, the common return on 3 and the number-plate-light remark on 5 and 7; the pin notes are empty. Wire colours are the core colours of both sources.
- Names are the DIN designations (L, 54g, 31, R, 58R, 54, 58L); Jaeger writes "54g" with a lower-case g, M&K the same.
- Lamp circuits `type: power` (they carry the lamp current from the towing vehicle); contact 3 `ground`. Alternative: `signal` for the indicator and brake circuits, but they are lamp supplies, not logic signals.
- `wire` given for all seven: both sources state the core colours, marked "for information" (M&K) and "Core color" (Jaeger).
- `type: circular`, `contact: round`, `drawn: socket`, positions as above.
- `key`: not set from a source. Jaeger's renders show no clearly identifiable key on the socket face; the plug render shows a groove near contact 4, which I could not confirm in text. Recommend drawing without a key or confirming from ISO 1724 or another maker drawing before setting `key: 180`.
- Recommended notes: "12 V system (ISO 1724, 12N). Not the same as the white 12S socket (ISO 3732), which carries supply and reversing light on the same contact numbers. Contact 3 is the common return for all lamps, 2.5 mm². Contacts 5 and 7 are the two sides of the tail lights; the number-plate light must not bridge them." The 12S allocation (M&K): 1 reversing light, 2 reserved, 3 earth for 4, 4 permanent supply, 5 not allocated, 6 ignition-switched supply, 7 earth for 6.

## Safety note
Trailer lighting is road-safety equipment: a swapped brake, indicator or tail-light contact can leave the trailer without working signals and cause an accident; check every function with the trailer connected before driving. The connector is for a 12 V nominal system (Jaeger). Work on the vehicle wiring only if you are competent to do so; vehicles with trailer control modules may need coding.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| 1 | L | power | Jaeger table "1 Left-hand direction indicator light, 1.5 mm², Yellow, L"; M&K "1 / L 1,5 Fahrtrichtungsanzeiger, links gelb" |
| 3 | 31 | ground | Jaeger "3 Common return 2.5 mm² White 31"; M&K "3 / 31 2,5 Masse weiß" |
| 6 | 54 | power | Jaeger "6 Brake lights 1.5 mm² Red 54"; M&K "6 / 54 1,5 Bremsleuchten rot" |
| 7 | 58L | power | Jaeger "7 Left-hand rear, marker, position light and license plate lamp, Black, 58L"; M&K "7 / 58L ... schwarz" |
| 2 | 54g | power | Jaeger rear view: blue "2" upper left (rear), cover print "2/54g" upper right (front) |

UNVERIFIED: the key position; exact radius (approximate from renders); ISO 1724 text not read. Australian/NZ 7-pin variants (different use of contacts) not researched and not covered.
