# sot23 — SOT-23 examples (3 pins, TO-236AB) / SOT-23: Beispiele (3 Anschlüsse, TO-236AB)

Kind: authoritative maker documents (onsemi primary for every variant, Diodes Incorporated cross-check for every variant)
Pins: 3 per variant, 4 variants; types: bc847, mmbt3904, q2n7002, bss84: signal 3 each

## Package
SOT-23, 3 leads, onsemi "SOT−23 (TO−236) 2.90x1.30x1.00 1.90P, CASE 318, ISSUE AU, DATE 14 AUG 2024". Two leads on one
long side (pins 1 and 2, 1.90 mm apart), one lead alone in the middle of the other long side (pin 3).
Suggested item: an "examples" item like `to92`, shape `to`, package `sot23`, `pins: ["1", "2", "3"]`, with variants.
Suggested name `SOT-23: examples` / `SOT-23: Beispiele`.

Pin order inside the package is **not** fixed by the package. onsemi's case 318 outline lists 26 "styles" for the same
body (page 2 of the outline): STYLE 6 = 1 Base, 2 Emitter, 3 Collector; STYLE 7 = 1 Emitter, 2 Base, 3 Collector;
STYLE 8 = 1 Anode, 2 No connection, 3 Cathode; STYLE 21 = 1 Gate, 2 Source, 3 Drain; STYLE 10 = 1 Drain, 2 Source …;
and more. The doc text must say: check the datasheet of the exact part.

## View
Shape `to`, package `sot23`, as defined in `internal/ecalc/pinouts.go`: seen from above, marking side up, pin 1 lower
left, pin 2 lower right, pin 3 top centre. Basis:
- onsemi case 318 outline, "TOP VIEW" (MMBT3904LT1/D PDF page 9, rendered and looked at): body drawn wide, the single
  lead at the top centre labelled 3, the two lower leads labelled 1 (left) and 2 (right). This is exactly the catalog
  view; no mapping needed.
- onsemi "MARKING DIAGRAM" on page 1 of each onsemi sheet (marking text visible, so seen from above): single lead at
  the top, "1" under the lower-left lead. The BSS84 diagram labels all three: "3 Drain" top, "1 Gate" lower left,
  "2 Source" lower right.
- Diodes Inc "Top View" / "Top View Pinout" figures (page 1 of each Diodes sheet): single lead at the top, two leads
  below. Diodes writes the function letters (C/B/E, D/G/S) at the leads but no pin numbers; the positions are the same
  as onsemi's numbered drawing.

## Variants

### bc847 — BC847 (NPN, 45 V)
| No. | Position | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | lower left | B | Base | Basis | signal |
| 2 | lower right | E | Emitter | Emitter | signal |
| 3 | top centre | C | Collector | Kollektor | signal |

onsemi BC846ALT1/D page 1: symbol with "1 BASE", "2 EMITTER", "3 COLLECTOR"; "SOT-23 CASE 318 STYLE 6"; the series covers
BC847A, B, C (ordering table: BC847ALT1G, BC847BLT1G …). Diodes DS11108 (BC846A–BC848C) page 1 "Top View Pinout": B lower
left, E lower right, C top.

### mmbt3904 — MMBT3904 (NPN, 40 V; the SOT-23 sibling of the 2N3904)
| No. | Position | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | lower left | B | Base | Basis | signal |
| 2 | lower right | E | Emitter | Emitter | signal |
| 3 | top centre | C | Collector | Kollektor | signal |

onsemi MMBT3904LT1/D page 1: "1 BASE", "2 EMITTER", "3 COLLECTOR", "SOT-23 (TO-236) CASE 318 STYLE 6". Diodes DS30036
page 1 "Top View Pinout": B lower left, E lower right, C top.

### q2n7002 — 2N7002 (N-channel MOSFET, 60 V)
| No. | Position | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | lower left | G | Gate | Gate | signal |
| 2 | lower right | S | Source of the N-channel MOSFET | Source des N-Kanal-MOSFET | signal |
| 3 | top centre | D | Drain | Drain | signal |

onsemi NDS7002A/D (2N7000, 2N7002, NDS7002A) page 1: SOT−23 CASE 318−08 drawing with "1 – Gate, 2 – Source, 3 – Drain"
(the same page gives the TO-92 2N7000 as 1 – Source, 2 – Gate, 3 – Drain). Diodes DS11303 page 1 "Top View": G lower
left, S lower right, D top.

### bss84 — BSS84 (P-channel MOSFET, −50 V) — the P-channel example of the task
| No. | Position | Name | Function (en) | Function (de) | Type |
| --- | --- | --- | --- | --- | --- |
| 1 | lower left | G | Gate | Gate | signal |
| 2 | lower right | S | Source of the P-channel MOSFET, normally the more positive side | Source des P-Kanal-MOSFET, normalerweise die positivere Seite | signal |
| 3 | top centre | D | Drain | Drain | signal |

onsemi BSS84/D page 1 "MARKING DIAGRAM": "3 Drain" (top), "1 Gate" (lower left), "2 Source" (lower right),
"SOT−23−3 CASE 318−08". Diodes DS30149 page 1 "Top View": G lower left, S lower right, D top.

Suggested item note (en/de): "The pin order of a SOT-23 part depends on the part, not on the package: transistors,
MOSFETs, diodes and regulators in the same case use different orders. The examples here all have pin 3 alone on one
side. The 2N7002 in SOT-23 (gate, source, drain) runs differently from the 2N7000 in TO-92 (source, gate, drain), and
the MMBT3904 (base, emitter, collector) differently from the 2N3904 in TO-92 (emitter, base, collector)." /
"Die Pin-Reihenfolge eines SOT-23-Bauteils hängt vom Bauteil ab, nicht vom Gehäuse: Transistoren, MOSFETs, Dioden und
Regler im selben Gehäuse haben verschiedene Reihenfolgen. Bei allen Beispielen hier steht Pin 3 allein auf einer Seite.
Der 2N7002 im SOT-23 (Gate, Source, Drain) ist anders belegt als der 2N7000 im TO-92 (Source, Gate, Drain), der MMBT3904
(Basis, Emitter, Kollektor) anders als der 2N3904 im TO-92 (Emitter, Basis, Kollektor)."

## Sources
1. onsemi, "General Purpose Transistors NPN Silicon, BC846ALT1G Series", Publication Order Number BC846ALT1/D,
   Rev. 21 (September 2026), page 1 (symbol with pin numbers, "SOT-23 CASE 318 STYLE 6", marking diagram), ordering
   table, case 318 outline with style list.
   https://www.onsemi.com/pdf/datasheet/bc846alt1-d.pdf — read page 1 rendered, style list as text, accessed 2026-11-24.
2. onsemi, "General Purpose Transistor NPN Silicon, MMBT3904L, SMMBT3904L", MMBT3904LT1/D, Rev. 15 (September 2026),
   page 1 (symbol "1 BASE, 2 EMITTER, 3 COLLECTOR", "SOT-23 (TO-236) CASE 318 STYLE 6", marking diagram); PDF page 9
   case 318 ISSUE AU outline "TOP VIEW"; PDF page 10 style list (STYLE 6, STYLE 21).
   https://www.onsemi.com/pdf/datasheet/mmbt3904lt1-d.pdf — read pages 1 and 9 rendered, page 10 as text, accessed 2026-11-24.
3. onsemi, "N-Channel Enhancement Mode Field Effect Transistor 2N7000, 2N7002, NDS7002A", NDS7002A/D, Rev. 11
   (March 2025), page 1 (SOT−23 CASE 318−08 with "1 – Gate, 2 – Source, 3 – Drain").
   https://www.onsemi.com/pdf/datasheet/nds7002a-d.pdf — read page 1 rendered, accessed 2026-11-24.
4. onsemi, "P-Channel Enhancement Mode Field-Effect Transistor BSS84", BSS84/D, Rev. 5 (October 2021), page 1
   (symbol, "SOT−23−3 CASE 318−08", marking diagram with "1 Gate, 2 Source, 3 Drain").
   https://www.onsemi.com/pdf/datasheet/bss84-d.pdf — read page 1 rendered, accessed 2026-11-24.
5. Diodes Incorporated, "BC846A–BC848C NPN Small-Signal Transistor in SOT23", Document number DS11108 Rev. 28 - 2
   (May 2025), page 1 "Top View Pinout". https://www.diodes.com/datasheet/download/BC847B.pdf (served the BC846A–BC848C
   sheet) — read page 1 rendered, accessed 2026-11-24.
6. Diodes Incorporated, "MMBT3904 40V NPN Small-Signal Transistor in SOT23", DS30036 Rev. 27 - 2 (March 2025), page 1
   "Top View Pinout". https://www.diodes.com/datasheet/download/MMBT3904.pdf — read page 1 rendered, accessed 2026-11-24.
7. Diodes Incorporated, "2N7002 N-Channel Enhancement Mode Field Effect Transistor", DS11303 Rev. 40 - 2 (July 2024),
   page 1 "Top View". https://www.diodes.com/datasheet/download/2N7002.pdf — read page 1 rendered, accessed 2026-11-24.
8. Diodes Incorporated, "BSS84 P-Channel Enhancement Mode MOSFET", DS30149 Rev. 27 - 2 (May 2026), page 1 "Top View".
   https://www.diodes.com/datasheet/download/BSS84.pdf — read page 1 rendered, accessed 2026-11-24.

Nexperia (BC847, 2N7002) not opened: nexperia.com answered HTTP 403; Diodes Incorporated used as the second maker.

## Cross-check
- bc847: onsemi (numbers + names) and Diodes (names at positions) agree: base lower left, emitter lower right,
  collector top.
- mmbt3904: onsemi and Diodes agree: B lower left, E lower right, C top.
- q2n7002: onsemi and Diodes agree: G lower left, S lower right, D top.
- bss84: onsemi and Diodes agree: G lower left, S lower right, D top.
- Package numbering: onsemi case 318 "TOP VIEW" numbers 1 lower left, 2 lower right, 3 top; onsemi's marking diagrams
  put "1" at the lower left. Diodes' figures carry no pin numbers, so the pin **numbers** rest on onsemi alone; the
  **positions** of the functions are confirmed by both makers.
- No maker-to-maker difference found for these four parts.

## Choices
- Catalog item: examples item `sot23` (name translated in all 16 locales like `to92`) with the variants `bc847`, `mmbt3904`, `q2n7002` (leading digit gets `q`, as `q2n7000`) and `bss84`; names B/E/C and G/S/D, all `signal`. The pin numbers rest on onsemi (Diodes' figures carry none); the positions are confirmed by both makers.
- Variant ids: `bc847`, `mmbt3904`, `q2n7002` (a leading digit gets `q`, as `q2n7000` in `to92`), `bss84`.
- Names are the short letters (B, E, C, G, S, D) as in the `to92` item; the words are in the function.
- All transistor leads are `signal`, as in `to92`.
- BSS84 chosen as the P-channel example because both makers' sheets were readable; no diode variant added.
- The function text of S at the BSS84 says "normally the more positive side" — my wording for P-channel use, not a
  datasheet sentence; it can be dropped.

## Spot-check
| Variant | No. | Name | Ref |
| --- | --- | --- | --- |
| bc847 | 1 | B | onsemi BC846ALT1/D page 1 symbol "1 BASE"; Diodes DS11108 p.1 B lower left |
| bc847 | 2 | E | onsemi BC846ALT1/D page 1 "2 EMITTER"; Diodes DS11108 p.1 E lower right |
| bc847 | 3 | C | onsemi BC846ALT1/D page 1 "3 COLLECTOR"; Diodes DS11108 p.1 C top |
| mmbt3904 | 1 | B | onsemi MMBT3904LT1/D page 1 "1 BASE"; Diodes DS30036 p.1 |
| mmbt3904 | 2 | E | onsemi MMBT3904LT1/D page 1 "2 EMITTER"; Diodes DS30036 p.1 |
| mmbt3904 | 3 | C | onsemi MMBT3904LT1/D page 1 "3 COLLECTOR"; Diodes DS30036 p.1 |
| q2n7002 | 1 | G | onsemi NDS7002A/D page 1 "1 – Gate"; Diodes DS11303 p.1 |
| q2n7002 | 2 | S | onsemi NDS7002A/D page 1 "2 – Source"; Diodes DS11303 p.1 |
| q2n7002 | 3 | D | onsemi NDS7002A/D page 1 "3 – Drain"; Diodes DS11303 p.1 |
| bss84 | 1 | G | onsemi BSS84/D page 1 marking diagram "1 Gate"; Diodes DS30149 p.1 |
| bss84 | 2 | S | onsemi BSS84/D page 1 "2 Source"; Diodes DS30149 p.1 |
| bss84 | 3 | D | onsemi BSS84/D page 1 "3 Drain"; Diodes DS30149 p.1 |
| package | 1/2/3 | – | onsemi case 318 ISSUE AU "TOP VIEW", MMBT3904LT1/D PDF page 9 |

UNVERIFIED: Nexperia sheets (BC847, 2N7002) not read (HTTP 403). Diodes' figures name the leads by function at their
positions but print no pin numbers, so pin numbering is from onsemi only. JEDEC TO-236AB itself was not read; onsemi
names the case "SOT−23 (TO−236)" without the "AB" suffix. Every pin and position above was read.
