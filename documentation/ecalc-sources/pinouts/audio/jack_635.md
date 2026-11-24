# jack_635 — 6.35 mm (1/4 inch) jack, TRS / 6,35-mm-Klinke (1/4 Zoll), TRS

Kind: cross-checked reproductions (maker documents: Rane, Mackie, Yamaha)
Pins: 3 per variant; variants: balanced, stereo, insert (same numbers T, R, S); types in each variant: signal 2, ground 1

## View
The catalog schema's `jack` shape: a plug from the side, tip at the left, segments from tip to sleeve (T, R, S). One fixed view, no mirror question (as `audio_trs`).

## Sources
1. Rane Corporation, RaneNote 102 "Analog I/O Standards", Dennis Bohn, 1982, https://www.ranecommercial.com/legacy/pdf/old/note102.pdf — section "1/4" PHONE JACKS": TRS 1/4" jacks Tip = Positive, Ring = Negative, Sleeve = Ground; TS Tip = Positive, Sleeve = Ground; headphone jacks Tip = Left Positive, Ring = Right Positive, Sleeve = Common Ground; accessed 2026-11-24
2. Mackie, 1620 owner's manual, Appendix B "Connections" (page 23), https://www.manualslib.com/manual/97163/Mackie-1620.html?page=23 — "1/4" TRS Balanced Mono Wiring: Sleeve = Shield, Tip = Hot (+), Ring = Cold (-)"; "1/4" TS Unbalanced Wiring"; "1/4" TRS Stereo Unbalanced Wiring: Sleeve = Shield, Tip = Left, Ring = Right"; accessed 2026-11-24
3. Mackie, 1620 owner's manual, page 24 "TRS Send/Receive Insert Jacks", https://www.manualslib.com/manual/97163/Mackie-1620.html?page=24 — sleeve common ground, send on the tip, return on the ring, "(TIP = SEND to effect, RING = RETURN from effect)"; accessed 2026-11-24
4. Mackie, ProFX12 owner's manual, page 28, https://www.manualslib.com/manual/754368/Mackie-Profx12.html?page=28 — same insert wording (same maker, repeat only); accessed 2026-11-24
5. Yamaha, "How to Use Inserts", Steve La Cerra, December 28, 2022, https://hub.yamaha.com/proaudio/livesound/how-to-use-inserts/ — send out through the tip, return on the ring, sleeves shared; "some do it vice-versa"; accessed 2026-11-24

## Cross-check
- balanced: Rane (Tip Positive, Ring Negative, Sleeve Ground) and Mackie (Tip Hot +, Ring Cold -, Sleeve Shield) agree.
- stereo: Rane (Tip Left, Ring Right, Sleeve Common Ground) and Mackie (Tip Left, Ring Right, Sleeve Shield) agree; the `audio_trs` item (3.5 mm) has the same assignment from TI.
- insert: Mackie (two manuals) and Yamaha agree on Tip = Send, Ring = Return, Sleeve = ground.
- **Opposite insert convention:** Yamaha states that some manufacturers use ring = send, tip = return. No maker document with that wiring was read, so the insert variant's notes say that some mixers swap send and return and name no maker.

## Choices
- Catalog item: three variants with the same numbers T, R, S; the mono TS plug appears only in the item notes (a two-contact plug does not fit the three-segment variants). The insert is unbalanced (Mackie); the send/return swap is a pin note of T and R of the insert variant.
- `Hot (+)` and `Cold (-)` as in the `xlr` item, plain hyphen.
- `signal` for all audio contacts, `ground` for the sleeve.
- No `wire`: no colours named.

## Spot-check list
- balanced T = Positive, R = Negative, S = Ground: Rane Note 102, "1/4" PHONE JACKS".
- stereo T = Left, R = Right, S = Shield: Mackie 1620 Appendix B, page 23.
- insert T = Send, R = Return: Mackie 1620 page 24; Yamaha "How to Use Inserts".
- TS: Tip = Hot (+), Sleeve = Shield: Mackie 1620 page 23; Rane Note 102.

Unverified: which makers use the reversed insert wiring (ring = send); none is named.
