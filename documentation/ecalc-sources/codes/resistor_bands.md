# resistor_bands — Resistor colour code (IEC 60062) / Widerstands-Farbcode (IEC 60062)

Kind: definitional
Guide-value note needed: no

Edition followed: **IEC 60062:2016 (edition 6.0, 2016-07) incl. COR1:2016 and AMD1:2019 (consolidated
edition 6.1, 2019-08)**. Table 1 "Code colour prescriptions" is identical in the 6.0 sample and the 6.1 CSV
(redline) sample I read, i.e. AMD1 did not change the colour table.

## Columns

Table A — colours (one row per colour)

| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| colour_id | stable id of the colour | stabile Kennung der Farbe | – | no |
| iec60757 | colour code letters (IEC 60757, as printed in IEC 60062 Table 1) | Farbkurzzeichen nach IEC 60757 | – | no |
| digit | significant figure | Ziffer (signifikante Stelle) | – | yes (empty = not allowed) |
| multiplier | multiplier | Multiplikator | factor (×) | yes |
| tol_pct | tolerance, symmetric ± | Toleranz ± | % | yes |
| tol_letter | tolerance letter code (IEC 60062 clause 5; see cross-check) | Toleranz-Kennbuchstabe | – | no |
| tcr_ppm | temperature coefficient of resistance, ± | Temperaturkoeffizient ± | 10⁻⁶/K (= ppm/K) | yes |
| legacy_tol_pct | tolerance printed in older / EIA-style charts that differ from IEC 60062:2016 | Toleranz in älteren / EIA-Tabellen, falls abweichend | % | yes |
| hex | suggested drawing colour (display choice, NOT normative) | Zeichenfarbe (Darstellungsvorschlag, nicht normativ) | sRGB hex | no |
| name | colour name | Farbname | – | no (en\|de) |

Table B — band layouts

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| layout_id | id | Kennung | – | no |
| bands | number of bands | Anzahl Ringe | – | yes |
| sequence | band meanings in reading order | Bedeutung der Ringe in Lesereihenfolge | – | no (en\|de) |
| when | when the layout is used | wann verwendet | – | no (en\|de) |

Table C — colour names in the 16 UI locales (see bottom).

## Rows

Table A — colours (IEC 60062:2016+AMD1:2019, Table 1; `tol_letter` and `hex` see notes)

```csv
colour_id,iec60757,digit,multiplier,tol_pct,tol_letter,tcr_ppm,legacy_tol_pct,hex,name
none,,,,20,M,,,transparent,"no band|kein Ring"
pink,PK,,0.001,,,,,#FF8FB1,"pink|rosa"
silver,SR,,0.01,10,K,,,#C0C0C0,"silver|silber"
gold,GD,,0.1,5,J,,,#CFA93A,"gold|gold"
black,BK,0,1,,,250,,#000000,"black|schwarz"
brown,BN,1,10,1,F,100,,#8B4513,"brown|braun"
red,RD,2,100,2,G,50,,#D62828,"red|rot"
orange,OG,3,1000,0.05,W,15,(none),#F77F00,"orange|orange"
yellow,YE,4,10000,0.02,P,25,(none),#FCD116,"yellow|gelb"
green,GN,5,100000,0.5,D,20,,#2E9E3E,"green|grün"
blue,BU,6,1000000,0.25,C,10,,#1F5FD6,"blue|blau"
violet,VT,7,10000000,0.1,B,5,,#7B2CBF,"violet|violett"
grey,GY,8,100000000,0.01,L,1,0.05,#8C8C8C,"grey|grau"
white,WH,9,1000000000,,,,,#FFFFFF,"white|weiß"
```

Empty cell = the colour is not assigned to that function in IEC 60062:2016 (shown as "--" in Table 1).
`legacy_tol_pct` "(none)" = the older chart assigns no tolerance to this colour.

Table B — band layouts (IEC 60062:2016 clauses 3.3, 3.4)

```csv
layout_id,bands,sequence,when
b3,3,"digit 1, digit 2, multiplier (no tolerance band = ±20 %)|Ziffer 1, Ziffer 2, Multiplikator (kein Toleranzring = ±20 %)","2 significant figures, tolerance ±20 % (IEC 3.3.1)|2 signifikante Stellen, Toleranz ±20 % (IEC 3.3.1)"
b4,4,"digit 1, digit 2, multiplier, tolerance|Ziffer 1, Ziffer 2, Multiplikator, Toleranz","2 significant figures, tolerance tighter than ±20 % (IEC 3.3.2)|2 signifikante Stellen, Toleranz enger als ±20 % (IEC 3.3.2)"
b5,5,"digit 1, digit 2, digit 3, multiplier, tolerance|Ziffer 1, Ziffer 2, Ziffer 3, Multiplikator, Toleranz","3 significant figures (IEC 3.3.3)|3 signifikante Stellen (IEC 3.3.3)"
b6,6,"digit 1, digit 2, digit 3, multiplier, tolerance, TCR|Ziffer 1, Ziffer 2, Ziffer 3, Multiplikator, Toleranz, TK","3 significant figures plus TCR; TCR only together with 3 significant figures (IEC 3.4). The TCR may also be an interrupted 6th band or dots between the bands.|3 signifikante Stellen plus TK; TK nur zusammen mit 3 signifikanten Stellen (IEC 3.4). Der TK kann auch als unterbrochener 6. Ring oder als Punkte zwischen den Ringen erscheinen."
```

## Sources
1. IEC 60062:2016 (edition 6.0, 2016-07), *Marking codes for resistors and capacitors* — iTeh Standards document preview
   (contents, foreword, clauses 1–4.2.2 incl. Table 1, Figures 1–6, Tables 2–4),
   https://cdn.standards.iteh.ai/samples/20940/4ae014b83a7c49adb888d27fd4b35b6b/IEC-60062-2016.pdf — accessed 2026-11-02.
2. IEC 60062:2016+AMD1:2019 CSV (edition 6.1, 2019-08, redline) — iTeh Standards document preview (same clauses,
   Table 1), https://cdn.standards.iteh.ai/samples/20940/b5fb592a1a1749c79f33e4d987f7fefb/IEC-60062-2016.pdf — accessed 2026-11-02.
3. EN 60062:2016/AC:2016-12 = IEC 60062:2016/COR1:2016 (published as STN EN 60062/AC:2017, Slovak preview):
   tolerance band ≥ 1.5 × width of the other bands; silver, gold and pink also code multiplier, tolerance and TCR,
   https://normy.normoff.gov.sk/norma/124871/nahlad — accessed 2026-11-02.
4. TT Electronics, *Technical Note TN004 – Methods for Marking Values on Resistors* (09.24), colour table with tolerance
   letters, https://www.ttelectronics.com/TTElectronics/media/ProductFiles/Application-Note/TN004-Methods-for-Marking-Values-on-Resistors.pdf — accessed 2026-11-02.
5. Panasonic Industry, *Resistor Color Codes: Thorough Explanations of How to Interpret, Calculate, and Remember Them!*
   (technical article dated 2025-10-24), https://industrial.panasonic.com/ww/ds/ss/technical/b27 — accessed 2026-11-02.
6. Wikipedia, *Electronic color code* (secondary; used only for the tolerance letters W/P/L and edition history),
   https://en.wikipedia.org/wiki/Electronic_color_code — accessed 2026-11-02.
7. Vishay Dale, *FRJ – Metal Film Resistors, Industrial, Zero Ohm Jumper Resistor*, document 31016, rev. 06-Oct-05
   ("Marking: Single Black"), copy at https://pdf.utmel.com/r/datasheets/vishay-frj50e36-datasheets-9419.pdf — accessed 2026-11-02.

## Cross-check
- **Digits, multipliers, tolerances, TCR**: taken from IEC 60062 Table 1 (sources 1 and 2 agree cell by cell).
  TCR column cross-checked with Panasonic (source 5): identical for all nine colours (250/100/50/15/25/20/10/5/1).
- **Edition / chart differences (important)**: Panasonic (2025, source 5) and TT Electronics (2024, source 4) still
  print the older convention: **grey = ±0.05 %**, orange and yellow = no tolerance, no pink. IEC 60062:2016 Table 1
  gives **orange ±0.05 %, yellow ±0.02 %, grey ±0.01 %, pink ×10⁻³**. I follow IEC 60062:2016; the old grey value is
  kept in `legacy_tol_pct`. A decoder should show both for a grey tolerance band.
- **What changed in 2016**: the IEC 2016 foreword lists as the only new colour "pink for the coding of the multiplier
  10⁻³", and (note under 3.4) that the 6th (TCR) band is **no longer** prescribed to be the wider band (earlier editions
  did that, which confused users). Wikipedia (source 6) says the orange/yellow/grey tolerances were also assigned by the
  2016 edition; the IEC foreword does not list that, and I could not read the 2004 (5th edition) Table 1 (the 2004
  preview ends before it). **UNVERIFIED**: in which edition orange ±0.05 %, yellow ±0.02 %, grey ±0.01 % first appeared.
- **Tolerance letters**: F, G, D, C, B, J, K, M agree between TT TN004 (source 4) and the capacitor/resistor maker
  tables used in `capacitor.md` (KEMET, TDK, Murata, Vishay). **W (±0.05 %), P (±0.02 %), L (±0.01 %)**: only from
  Wikipedia (source 6, which cites IEC 60062:2016 Table 12); IEC Table 12 itself is not in the preview. TT uses **A**
  for ±0.05 % in its own part numbers. → W/P/L are **UNVERIFIED** (single secondary source); show them as
  "per IEC 60062 (unverified)" or omit.
- **Zero-ohm**: single black band — Vishay FRJ (source 7, "Marking: Single Black"); Wikipedia agrees.
- **hex**: my own display choice (IEC Table 1 NOTE 2 says its example colours are not normative). Not a standard value.
- **TCR letter codes** (IEC Table 16) are not transcribed: not in the preview, secondary sources disagree.

## Notes (en / de)
- en: Read from the end where the bands start; the tolerance band is at least 1.5× as wide as the others (IEC 60062,
  corrigendum 2016). Gold, silver and pink never stand first.
  de: Lesen ab dem Ende, an dem die Ringe beginnen; der Toleranzring ist mindestens 1,5-mal so breit wie die anderen
  (IEC 60062, Berichtigung 2016). Gold, Silber und Rosa stehen nie an erster Stelle.
- en: Older charts use grey = ±0.05 % and give orange/yellow no tolerance; IEC 60062:2016 uses grey ±0.01 %, orange ±0.05 %,
  yellow ±0.02 %. Check the datasheet for precision resistors.
  de: Ältere Tabellen verwenden Grau = ±0,05 % und geben Orange/Gelb keine Toleranz; IEC 60062:2016 verwendet Grau
  ±0,01 %, Orange ±0,05 %, Gelb ±0,02 %. Bei Präzisionswiderständen das Datenblatt prüfen.
- en: Pink (×0.001) is new in IEC 60062:2016 for very low resistances. de: Rosa (×0,001) ist seit IEC 60062:2016 neu, für
  sehr kleine Widerstandswerte.
- en: A single black band marks a zero-ohm jumper. de: Ein einzelner schwarzer Ring kennzeichnet eine Null-Ohm-Brücke.
- en: Displayed colours are illustrations only. de: Die angezeigten Farben sind nur Illustrationen.

## Decode / encode rules (for the implementer)
1. Value = (digits as integer) × multiplier, in Ω. 3 or 4 bands → 2 digits; 5 or 6 bands → 3 digits (IEC 3.3.1–3.3.3,
   the "four–five rule").
2. 3 bands: no tolerance band → ±20 %. 4 bands: band 4 = tolerance. 5 bands: band 5 = tolerance. 6 bands: band 6 = TCR.
3. Digit bands may only be black…white. Multiplier band: black…white, gold, silver, pink. Tolerance band: brown, red,
   orange, yellow, green, blue, violet, grey, gold, silver. TCR band: black…grey (white has no TCR).
   Any other colour in a position → invalid input.
4. Encoding a value V with n significant digits: write V = N × 10^k with N an integer of exactly 2 (4-band) or 3
   (5/6-band) digits; k must be in {−3 … 9} (−3 pink, −2 silver, −1 gold, 0 black … 9 white). If V needs more
   significant digits than the layout has, or k is outside −3…9, it cannot be coded → error, do not round silently.
   Ranges: 4-band 0.010 Ω (10 × 10⁻³) … 99 GΩ; 5-band 0.100 Ω (100 × 10⁻³) … 999 GΩ.
5. A value with 3 significant figures (e.g. 249 kΩ, E96) needs 5 bands. A 2-figure value may still be written with 5
   bands (e.g. 47 kΩ 1 % = yellow-violet-black-red-brown, TT TN004 example 1).
6. TCR coding is only defined together with 3 significant figures (6 bands). A 5-band code is always
   3 digits + multiplier + tolerance in IEC 60062.
7. Direction heuristic for a decoder that gets bands in either order: the reading is invalid if the first band is gold,
   silver, pink or none; prefer the orientation whose last band is a valid tolerance colour.

## Edge cases
- Single black band = 0 Ω jumper (Vishay FRJ). Treat a 1-band "black" input as 0 Ω, tolerance not applicable.
- Grey tolerance band: show "±0.01 % (IEC 60062:2016); older charts ±0.05 %".
- Orange/yellow as tolerance: valid only under IEC 60062:2016; older parts do not use them.
- Military/legacy resistors may carry an extra reliability (failure-rate) band; it is not part of IEC 60062 and makes
  band counting unreliable (Wikipedia, secondary) — mention only, do not decode.
- White as multiplier = 10⁹ (valid per IEC), rarely seen.

## Worked examples (test vectors)
| bands | expected | source |
|---|---|---|
| blue, grey, red | 6.8 kΩ ±20 % | IEC 60062:2016 Figure 1 |
| violet, green, yellow, gold | 750 kΩ ±5 % | IEC 60062:2016 Figure 2 |
| red, yellow, white, orange, brown | 249 kΩ ±1 % | IEC 60062:2016 Figure 3 |
| red, yellow, white, orange, brown, red | 249 kΩ ±1 %, ±50 ppm/K | IEC 60062:2016 Figure 4 |
| yellow, violet, black, red, brown | 47 kΩ ±1 % | TT TN004 example 1 |
| grey, red, orange, gold | 82 kΩ ±5 % | TT TN004 example 3 |
| brown, black, gold, gold | 1.0 Ω ±5 % | computed from Table 1 |
| brown, black, black, pink, brown | 0.100 Ω ±1 % | computed from Table 1 (pink ×10⁻³) |
| brown, black, black, red, grey | 10.0 kΩ ±0.01 % (legacy charts: ±0.05 %) | computed from Table 1 |
| green, blue, black, silver, orange | 5.60 Ω ±0.05 % | computed from Table 1 |
| orange, orange, black, black, violet, brown | 330 Ω ±0.1 %, ±100 ppm/K | computed from Table 1 |
| black (single band) | 0 Ω jumper | Vishay FRJ |

Computed rows were checked with a helper script that is not part of the repository.

## Suggested finder / lookups
- Decoder: bands → value, tolerance, TCR (rules above). Encoder: value + tolerance (+ TCR) → bands, picking 4 bands
  when the value has ≤ 2 significant figures and no TCR is asked, else 5/6 bands.
- Nearest-E-series hint: if the decoded value is not in E24/E96, flag "check reading direction".

## Table C — colour names (16 locales)

Natural words as used for the colour code; Portuguese follows the UI's pt-BR majority (pt-PT: castanho, cinzento).
Japanese uses the single-kanji colour-code names (黒茶赤橙黄緑青紫灰白金銀); pink as ピンク.

```csv
colour_id,cs,da,de,el,en,es,fr,hi,it,ja,nl,no,pl,pt,sv,zh
black,černá,sort,schwarz,μαύρο,black,negro,noir,काला,nero,黒,zwart,svart,czarny,preto,svart,黑色
brown,hnědá,brun,braun,καφέ,brown,marrón,marron,भूरा,marrone,茶,bruin,brun,brązowy,marrom,brun,棕色
red,červená,rød,rot,κόκκινο,red,rojo,rouge,लाल,rosso,赤,rood,rød,czerwony,vermelho,röd,红色
orange,oranžová,orange,orange,πορτοκαλί,orange,naranja,orange,नारंगी,arancione,橙,oranje,oransje,pomarańczowy,laranja,orange,橙色
yellow,žlutá,gul,gelb,κίτρινο,yellow,amarillo,jaune,पीला,giallo,黄,geel,gul,żółty,amarelo,gul,黄色
green,zelená,grøn,grün,πράσινο,green,verde,vert,हरा,verde,緑,groen,grønn,zielony,verde,grön,绿色
blue,modrá,blå,blau,μπλε,blue,azul,bleu,नीला,blu,青,blauw,blå,niebieski,azul,blå,蓝色
violet,fialová,violet,violett,βιολετί,violet,violeta,violet,बैंगनी,viola,紫,violet,fiolett,fioletowy,violeta,violett,紫色
grey,šedá,grå,grau,γκρι,grey,gris,gris,स्लेटी,grigio,灰,grijs,grå,szary,cinza,grå,灰色
white,bílá,hvid,weiß,λευκό,white,blanco,blanc,सफ़ेद,bianco,白,wit,hvit,biały,branco,vit,白色
gold,zlatá,guld,gold,χρυσό,gold,dorado,or,सुनहरा,oro,金,goud,gull,złoty,dourado,guld,金色
silver,stříbrná,sølv,silber,ασημί,silver,plateado,argent,रुपहला,argento,銀,zilver,sølv,srebrny,prateado,silver,银色
pink,růžová,lyserød,rosa,ροζ,pink,rosa,rose,गुलाबी,rosa,ピンク,roze,rosa,różowy,rosa,rosa,粉红色
none,bez proužku,ingen ring,kein Ring,χωρίς δακτύλιο,no band,sin banda,sans anneau,कोई पट्टी नहीं,nessuna fascia,帯なし,geen ring,ingen ring,brak paska,sem faixa,ingen ring,无色环
```

Translations are my own (no source needed for common colour words); a native-speaker check is advisable for hi
(स्लेटी vs धूसर for grey, रुपहला vs चाँदी for silver).
