# obd2 — OBD-II (SAE J1962) / OBD-II (SAE J1962)

Kind: cross-checked reproductions
Pins: 16; types: power 1, ground 2, data 6, nc 7

## View
Written for the vehicle's socket, looking into it, wide side up: `drawn: socket`, top row 1 to 8 from left to right, bottom row 9 to 16 from left to right. The other view (mirror) is the tester's plug: 8 to 1 on top.

How I know, from two independent drawings:
- ELM327 datasheet, page 79, "Figure 8. The J1962 Vehicle Connector" (rendered and looked at): trapezoid with the wide side on top, mounting lugs, "1" top left, "8" top right, "9" bottom left, "16" bottom right. The next paragraph says that your circuit must connect "by way of a matching male J1962 connector", so the drawn part is the vehicle's female one.
- AESTECHNO's figure j1962.svg (labels read from the SVG): numbers 1 to 8 at rising x on the upper row and 9 to 16 at rising x on the lower row, shell path wider at the top. Its caption describes the vehicle connector with female contacts, "seen from the mating side, facing the connector", and refers to SAE J1962 figure 3.
- Wikipedia's caption agrees on the view: "Female OBD-II type A connector pinout – front view".

For the outline: the JS drawing of `obd2` must put the wide side at the top with this row order.

## Sources
1. Elm Electronics, ELM327 datasheet, ELM327DSK, https://www.elmelectronics.com/wp-content/uploads/2017/01/ELM327DS.pdf — page 79 (figure 8 and the pin 5 remark), page 81 (figure 9: pins 2, 5, 6, 7, 10, 14, 15, 16). Accessed 2026-11-04.
2. AESTECHNO (electronic design house, France), OBD2 pinout (OBD-II port): J1962 16-pin, CAN, K-line, J1850, https://www.aestechno.com/en/resources/pinouts/obd2-obd-ii/ with figure j1962.svg — full table, states that the allocation comes from SAE J1962 table 1 and section 6.3. Accessed 2026-11-04.
3. Wikipedia, On-board diagnostics, https://en.wikipedia.org/wiki/On-board_diagnostics — table of all 16 pins, type A and B. Accessed 2026-11-04.
4. CSS Electronics, OBD2 Explained, https://www.csselectronics.com/pages/obd2-explained-simple-intro — pin 16, pins 6 and 14, type A 12 V and type B 24 V. Accessed 2026-11-04.

Not read: SAE J1962 and ISO 15031-3 (paywalled). Also seen but not used: Gendan's page (https://m.gendan.co.uk/article_3.html, same pins, no view statement).

## Cross-check
Sources 1, 2 and 3 agree on every pin they name. ELM327 does not draw pin 4 or the manufacturer pins. Details:
- **Differs from the assignment:** the assignment says that all pins apart from 4, 5, 16, 6, 14, 7 and 15 are undefined. Sources 1, 2 and 3 all give pin 2 as J1850 Bus+ and pin 10 as J1850 Bus-. I followed the sources: pins 2 and 10 are `data`, and only 1, 3, 8, 9, 11, 12 and 13 are manufacturer pins (AESTECHNO: "The other seven are left to the manufacturer"; Wikipedia: manufacturer discretion).
- Pin 10: Gendan writes only "J1850 Bus". ELM327 and AESTECHNO write Bus-. Wikipedia and AESTECHNO agree that VPW uses pin 2 alone, so pin 10 is PWM only.
- Pin 15: optional per Wikipedia, and ELM327 says that many vehicles do not need it.
- Pin 16: AESTECHNO and Wikipedia give 12 V for type A and 24 V for type B, and CSS gives the same; AESTECHNO says that J1962 defines the pin as permanent, and CSS says "often" even with the ignition off. I wrote "stays live with the ignition off".
- Fuse advice: AESTECHNO says to fuse or current-limit the adapter itself.
- Pin 5 missing on some vehicles: ELM327 only, worded as "reported".

## Choices
- `type: nc` for the seven manufacturer pins, with the function saying that they are manufacturer-specific and can be live. They may carry signals, but none that the standard defines, and the catalog schema's `nc` includes "reserved". The dashed look must not be read as "free to use"; the function says so. This is a judgement call.
- All seven have the same name `OEM`. Manufacturer uses (Wikipedia lists some for GM, Ford, BMW and others) are left out, they vary too much for a pin table.
- Names follow AESTECHNO's table, except `OEM` (source: "Discretionary").
- `contact: blade`: the drawing style for the flat contacts of this connector, shown as rectangles in the ELM327 figure. No source calls them blades, so the plug caption "blades" is my word for the male contacts.
- Type B has the same pin positions; the item shows type A only, with the voltage difference in the notes.
- Item names stay in Latin script in all locales: only proper names, no generic word.
