# type2 — IEC 62196-2 Type 2 (vehicle inlet / connector) / IEC 62196-2 Typ 2 (Fahrzeugstecker / Ladedose)

Kind: authoritative maker document (TE Connectivity product specification for its own Type 2 assemblies); no second maker drawing read
Pins: 7; types: power 4, ground 1, signal 2
Status: complete for names, positions and CP/PP basics from one maker document; second maker document not read (Phoenix Contact answered 403)
Group: vehicle_mains

## Pins
| No. | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- |
| PE | PE | Protective earth, centre contact; mates first, breaks last | Schutzleiter, mittlerer Kontakt; schließt zuerst, trennt zuletzt | ground |
| L1 | L1 | Phase 1 (single-phase charging uses L1 and N) | Außenleiter 1 (einphasiges Laden über L1 und N) | power |
| L2 | L2 | Phase 2 (three-phase only) | Außenleiter 2 (nur dreiphasig) | power |
| L3 | L3 | Phase 3 (three-phase only) | Außenleiter 3 (nur dreiphasig) | power |
| N | N | Neutral | Neutralleiter | power |
| CP | CP | Control pilot, signal contact (rated 2 A, 30 V); connects last when plugging in | Control Pilot, Signalkontakt (Bemessung 2 A, 30 V); schließt beim Stecken zuletzt | signal |
| PP | PP | Proximity: resistor Rc to PE in the plug codes the cable's current capacity (13 A 1.5 kΩ, 20 A 680 Ω, 32 A 220 Ω) and shows a plug is present | Proximity: Widerstand Rc nach PE im Stecker codiert die Strombelastbarkeit des Kabels (13 A 1,5 kΩ, 20 A 680 Ω, 32 A 220 Ω) und meldet einen gesteckten Stecker | signal |

## View
Written for the pin side: the vehicle inlet on the car, or a station-side plug, looking at its mating face with the flat edge (signal contacts) at the top: `drawn: plug`. Angle clockwise from 12 o'clock, radius relative to the housing; PE in the centre, the other six on a ring about 60° apart.

| Contact | Angle (pin face) | Radius |
| --- | --- | --- |
| PE | centre | 0 |
| CP | 30° | 0.55 |
| N | 90° | 0.55 |
| L3 | 150° | 0.55 |
| L2 | 210° | 0.55 |
| L1 | 270° | 0.55 |
| PP | 330° | 0.55 |

In words, on the pin face: top row PP (left), CP (right); middle row L1, PE, N; bottom row L2 (left), L3 (right). The other view (mirror) is the contact-tube side: the vehicle connector on the cable and a socket-outlet on a wallbox: top row CP (left), PP (right); middle N, PE, L1; bottom L3 (left), L2 (right).

Basis: TE 108-94323, page 2, two labelled renders (looked at and measured):
- "2.1.2. Car-side Connector ... All contacts are receptacle contacts": labels control pilot (CP) top left, proximity (PP) top right, N middle left, earth contact (PE) centre, L1 middle right, L3 bottom left, L2 bottom right.
- "2.1.3. Station-side plug ... All contacts are pin contacts": proximity (PP) top left, control pilot (CP) top right, L1 middle left, PE centre, N middle right, L2 bottom left, L3 bottom right.
- The two are exact mirror images, as plug face and socket face must be. The vehicle inlet carries pin contacts ("Vehicle inlet is assembled on the electrical vehicle and contains pin contacts", page 1), so its face is the pin-face layout.
- Angles measured on the plug render from the PE centre: PP 326°, CP 34°, N 90°, L3 152°, L2 208°, L1 270°, radii 27 to 32 px of a housing radius of about 55 px. Rounded to 30° steps and 0.55. Measured on a small render: about ±5°. Both renders show the straight edge of the housing at the top, above CP and PP.

## Sources
1. TE Connectivity, Product Specification 108-94323 Rev A, 01 FEB 16 (revision record 18.04.16), "IEC 62196 Electric Vehicle Charge Connector Assembly (Type 2 for Mode 2 and 3)", 22 pages, copy at https://www.dalroad.com/wp-content/uploads/2016/08/Type-II-connector-product-spec.pdf — page 1 (definitions: socket-outlet, station side plug with pin contacts, car side/vehicle connector, vehicle inlet with pin contacts), page 2 (contacts L1, PE, N, PP, CP; three-phase "with the L2 and L3 Phases line in place"; renders), page 2-3 (power and earth contacts 6 mm, signal contacts 3 mm, "IEC 62196-2, Standard Sheet 2-IIb"), page 5 (table 4-1 ratings: AC power "20A up to three phases / 32A one phase", 250 VAC; Control Pilot 2 A 30 V; Proximity Detection 2 A 30 V), page 7 (4.1.8 and table 4-7 resistor coding Rc between proximity contact and PE: 13 A 1.5 kΩ, 20 A 680 Ω, 32 A 220 Ω, tolerance ±3 %, "IEC 61851-22, Annex B"; "The resistor is also used for proximity detection"), pages 11-13 (table 4-11 sequence of connections: inserting: PE, then N, then L1/L2/L3, then CP and PP; withdrawing in reverse; "Proximity connection can be made before the control connection"). Rendered and read. Accessed 2026-11-24.

Not read: IEC 62196-2 standard sheets (the VDE-Verlag preview of edition 3.0, 2022-10, was opened but ends before the standard sheets; it confirms the standard sheets for "Configuration type 2" start on page 27 of the standard). Phoenix Contact product PDFs (1271965 etc.): HTTP 403. Farnell datasheets: time-out. IEC 61851-1 (CP PWM signalling): not read, so CP's PWM duty-cycle meaning is not stated here.

## Cross-check
- One maker document; inside it the connector face, the plug face and the connection-sequence renders (pages 11-13) agree on every position (e.g. L1 at the right of the connector face in steps 3 and 2 of insertion and withdrawal).
- Wikipedia "Type 2 connector" (secondary, opened): top row two signal contacts PP and CP, middle row with PE in the centre and two power contacts, bottom row two power contacts. Agrees in structure; its text gives no per-contact positions.
- Ratings: TE's 250 VAC / 20 A three-phase / 32 A single-phase are TE's assembly ratings, not the limits of the Type 2 system; the item must not present them as general ratings.

## Choices
- Catalog item: `circular`, `contact: round`, `drawn: plug` (pin face), `key: 0` (the flat edge at the top, seen in both renders); PE at radius 0 (the loader accepts 0 to 1), the ring at 0.55. The item notes carry the one-sentence safety line, the CP/PP rules (do not bridge, contact sequence, Rc values), and the statement that TE's 250 V / 20 A / 32 A / 2 A 30 V are TE's assembly ratings, not the limits of the Type 2 system; the CP function text names the 2 A, 30 V as TE's. CP's PWM signalling is not covered. No `wire`.
- `type: circular` with PE at radius 0 and a ring at 0.55; `contact: round`; `drawn: plug` (the car's inlet is what most users look at); `key: 0` for the flat edge at the top (from the renders).
- N `power` (current-carrying), as for the CEE items.
- Recommended notes: "Single-phase cables and inlets use L1 and N only. Contact sequence: PE first, N, then the phases, CP and PP last; CP and PP break first when unplugging (TE table 4-11). PP resistor values per TE table 4-7 (from IEC 61851-22 Annex B)." Do not add CP PWM values until IEC 61851-1 or a maker's controller document is read.
- A second maker drawing (Mennekes, Phoenix Contact, ABL) is wanted before shipping, because this is a mains connector.

## Safety note
Type 2 carries mains voltage, up to three-phase, at high currents; the charging station switches the power on only after the CP/PP handshake. Never apply power to an inlet or cable outside a certified charging station, never bridge CP or PP to fake a vehicle, and leave work on chargers and vehicle inlets to qualified electricians. Voltages and currents here are only those read in TE's specification for its own assemblies.

## Spot-check
| No. | Name | Type | Reference |
| --- | --- | --- | --- |
| PE | PE | ground | TE 108-94323 p. 2, both renders "earth contact (PE)" in the centre; p. 11 table 4-11 step 1 |
| L1 | L1 | power | p. 2 station-side plug render, L1 middle left; car-side connector render, L1 middle right |
| CP | CP | signal | p. 2 renders "control pilot (CP)" top right (plug) / top left (connector); p. 5 table 4-1 "Control Pilot 2 A 30 V" |
| PP | PP | signal | p. 2 renders "proximity (PP)"; p. 7 table 4-7 resistor coding |
| L3 | L3 | power | p. 2 plug render bottom right; p. 11 step 3 "L3" |

UNVERIFIED: exact angles (±5° from a render); a second maker's drawing; CP PWM signalling; current and voltage limits of the Type 2 system beyond TE's own ratings.
