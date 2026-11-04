# servo — Hobby servo (3-pin) / Modellbau-Servo (3-polig)

Kind: cross-checked reproductions
Pins: 3; types: power 1, ground 1, signal 1

## View
Written for the plug of the servo lead: `drawn: socket` (SparkFun: "a female, 3-pin, 0.1" pitch header"), one column in the order signal, V+, ground. A single column looks the same in both views, and the order is symmetric around V+, so a mirror error cannot change which pin is in the middle. Only the sequence is stated. A plug turned by 180° reads ground, V+, signal, as on a real connector.

Basis: Parallax's table lists "1 (white) Signal", "2 (red) Vservo", "3 (black) Ground". Hitec writes "the Red wire ( center ) is 'power'". Pololu says the power wire is in the middle, with ground and signal on either side. Derivation from these, as none of them states a view.

## Sources
1. Parallax, Standard Servo (#900-00005), product guide v3.0, 8/10/2022, https://mm.digikey.com/Volume0/opasdata/d220001/medias/docus/5728/900-00005_Guide.pdf — pages 3 and 4: connection diagram, voltage table. Accessed 2026-11-04.
2. Parallax, Standard Servo (#900-00005), datasheet v2.0, copy at https://cs.umb.edu/cs341/Lab09/900-00005-StdServo-v2.0.pdf — same table; "Manufactured for Parallax exclusively by Futaba". Accessed 2026-11-04.
3. Hitec RCD, General Servo Information, copy at https://people.ece.cornell.edu/land/courses/ece4760/FinalProjects/f2012/jmv87/site/files/Servomanual.pdf — "Wire Color Meanings", voltage range. Accessed 2026-11-04.
4. Pololu (Jan), Electrical characteristics of servos and introduction to the servo control interface, 2011, https://www.pololu.com/blog/16/electrical-characteristics-of-servos-and-introduction-to-the-servo-control-interface — wire order, colour schemes, reversed power warning. Accessed 2026-11-04.
5. SparkFun, Hobby Servo Tutorial, https://learn.sparkfun.com/tutorials/hobby-servo-tutorial/all — connector type, colour table. Accessed 2026-11-04.
6. Spektrum SPMAR6310 User Manual, page 7, copy at https://www.manualsdir.com/manuals/429160/spektrum-spmar6310.html?page=7 — JST B3B-ZR micro connectors, colours, polarity warning. Accessed 2026-11-04.
7. ProgressiveRC, JR/Hitec servo extension, https://www.progressiverc.com/jr-hitec-universal-servo-extension.html — brown, red, orange. Accessed 2026-11-04.
8. RobotPlatform, Major Servo Manufacturers, https://robotplatform.com/knowledge/servo/servo_manufacturers.html — pin order per maker, Airtronics remark. Accessed 2026-11-04.

Not reached: ProgressiveRC's Futaba lead page (HTTP 429 twice). No original Futaba or JR document was found.

## Cross-check
Order signal, V+, ground: sources 1, 2, 3, 4 and 8 agree (two makers' documents, Parallax and Hitec, plus Pololu). Disagreements and thin spots:
- Numbering: Parallax has 1 = signal, 3 = ground; SparkFun has 1 = ground, 3 = signal. No common numbering exists, so I use S, + and - rather than inventing numbers.
- SparkFun's colour table is damaged: rows 2 and 3 have one cell too many in its HTML (row 2: Red, Brown, Red, Red; row 3: Yellow, White, Orange, White), so columns do not line up under the headings Hitec, JR, Futaba. I only use its first column (Hitec: black, red, yellow) and take the JR and Futaba colours from elsewhere, where the table agrees if the stray cells are ignored.
- Futaba scheme (drawn): white, red, black from Parallax's servo made by Futaba. Brand and colours are linked only through that sentence and through SparkFun's last column. This is the `wire` choice I trust least in this item, although Pololu lists black, red, white as a common scheme.
- JR scheme: ProgressiveRC for a JR/Hitec lead and Spektrum for its leads. Pololu lists the same colours without a brand.
- Hitec scheme: Hitec names only black and red (centre) and "third wire", and its HS-311 sheet (https://mm.digikey.com/Volume0/opasdata/d220001/medias/docus/489/HS-311.pdf, searched for colours, none named) adds nothing. Yellow comes from SparkFun alone, so the note says "usually yellow". ProgressiveRC sells orange leads as JR/Hitec.
- **Exception to the middle pin:** Spektrum's AR6310 manual says "Brown Wire (middle wire): Negative" for its JST micro plugs and warns that some JST servos have another polarity. A search result (not read) reports the same mismatch on Spektrum telemetry cables. Kept as an exception in the note, it applies to JST micro plugs and not to the 0.1 inch plug.
- Airtronics: RobotPlatform alone says that power and ground are swapped, Pololu only names "Airtronics Z" as a connector type. The note says "is reported".

Why the middle matters: this part is my own derivation. Turning a three-in-line plug keeps the centre contact in place, so V+ stays on V+ and only the outer contacts swap. The remark about the ground on the signal pin follows from the same swap. Pololu's warning covers real reversal of power.

## Choices
- Numbers `S`, `+`, `-` are text and quoted. They appear in `data-pin` and in searches; the catalog schema allows 1 to 8 characters and has examples such as "T". If these characters cause trouble, change them in the YAML and vector together.
- `wire` values are the Futaba scheme only, as the assignment asks. The note names the scheme.
- The pulse timing is left out (it is not part of a pinout, and values differ per maker).
- Kind is "cross-checked reproductions" because no standard exists at all: this is a de facto layout confirmed by makers' documents.
