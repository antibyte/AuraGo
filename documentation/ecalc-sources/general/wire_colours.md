# wire_colours — Conductor colour codes (IEC 60445 / DIN VDE and US NEC) / Aderfarben (IEC 60445 / DIN VDE und US-NEC)

Kind: transcribed standard
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| system | `IEC/DE` = IEC 60445 / DIN VDE (Germany, EU harmonised); `US/NEC` = USA, NFPA 70 | Normensystem | – | no |
| conductor | conductor (designation, or "en\|de" text) | Leiter (Kurzzeichen oder "en\|de") | – | no |
| colour | insulation colour "en\|de" | Aderfarbe "en\|de" | – | no |
| status | mandatory / recommended / customary "en\|de" | verbindlich / empfohlen / üblich "en\|de" | – | no |
| reference | clause or document the rule comes from (as cited by the sources) | Fundstelle (wie in den Quellen zitiert) | – | no |
| note | remark "en\|de" | Hinweis "en\|de" | – | no |
| check | cross-check record | Quellenabgleich | – | no |

## Rows
```csv
system,conductor,colour,status,reference,note,check
IEC/DE,L1,brown|braun,recommended assignment|empfohlene Zuordnung,DIN VDE 0100-510; ZVEH recommendation; BS 7671 (UK),"outer conductors may be brown, black or grey; Germany has no normative assignment to L1/L2/L3 (ZVEH recommends brown–black–grey)|Außenleiter dürfen braun, schwarz oder grau sein; in Deutschland gibt es keine normative Zuordnung zu L1/L2/L3 (ZVEH empfiehlt braun–schwarz–grau)",elektro.net (DIN VDE 0100-510 + ZVEH table) = Prysmian UK (L1 brown) = Kiefer (outer conductors brown/black/grey)
IEC/DE,L2,black|schwarz,recommended assignment|empfohlene Zuordnung,DIN VDE 0100-510; ZVEH recommendation; BS 7671 (UK),,elektro.net (ZVEH) = Prysmian UK (L2 black)
IEC/DE,L3,grey|grau,recommended assignment|empfohlene Zuordnung,DIN VDE 0100-510; ZVEH recommendation; BS 7671 (UK),,elektro.net (ZVEH) = Prysmian UK (L3 grey)
IEC/DE,L (single-phase)|L (einphasig),brown|braun,cable core colour|Aderfarbe im Kabel,DIN VDE 0293-308 (HD 308 S2) Table 1,"3-core cable with PE: green-yellow, blue, brown|3-adrige Leitung mit Schutzleiter: grün-gelb, blau, braun",elektro.net (DIN VDE 0293-308:2003-01 Table 1) = Prysmian UK (single-phase brown/blue)
IEC/DE,N,blue|blau,mandatory|verbindlich,DIN VDE 0100-510 514.3.1; IEC 60445 (DIN EN 60445 VDE 0197),"named ""light blue"" (hellblau) until 2003; colour may be used for other conductors only if no neutral exists in the circuit|bis 2003 als ""hellblau"" bezeichnet; Blau darf nur dann für andere Leiter verwendet werden, wenn der Stromkreis keinen Neutralleiter hat",Kiefer (VDE 0100 und die Praxis) = elektro.net = Prysmian UK
IEC/DE,PE,green-yellow|grün-gelb,mandatory|verbindlich,DIN VDE 0100-510 514.3.1; IEC 60445,green-yellow must not be used for any other conductor|Grün-Gelb darf für keinen anderen Leiter verwendet werden,Kiefer = elektro.net = Prysmian UK
IEC/DE,PEN,green-yellow with blue markings at the ends|grün-gelb mit blauer Markierung an den Enden,mandatory (Germany)|verbindlich (Deutschland),DIN VDE 0100-510 514.3.2,internationally also blue with green-yellow markings at the ends; Germany permits only the green-yellow variant (except public distribution networks)|international auch blau mit grün-gelber Markierung an den Enden; in Deutschland nur die grün-gelbe Variante (Ausnahme: öffentliche Verteilungsnetze),Kiefer only (UNVERIFIED)
US/NEC,grounded conductor (neutral)|geerdeter Leiter (Neutralleiter),white or gray|weiß oder grau,mandatory (NEC 200.6)|verbindlich (NEC 200.6),NFPA 70 (NEC) 200.6,,"EC&M (Mike Holt, 2023) = ExpertCE (2025)"
US/NEC,equipment grounding conductor (PE)|Schutzleiter (PE),green or green with yellow stripe(s); may be bare|grün oder grün mit gelbem Streifen; darf auch blank sein,mandatory (NEC 250.119)|verbindlich (NEC 250.119),NFPA 70 (NEC) 250.119,,green / green-yellow: EC&M = ExpertCE; bare: ExpertCE only
US/NEC,ungrounded conductors (hot)|Außenleiter (nicht geerdet),"any colour except white, gray or green|jede Farbe außer weiß, grau oder grün","mandatory (NEC 200.7, 250.119)|verbindlich (NEC 200.7, 250.119)","NFPA 70 (NEC) 200.7, 250.119, 210.5(C)",branch circuits: the identification method must be documented/posted (210.5(C))|Endstromkreise: Kennzeichnungsmethode muss dokumentiert/ausgehängt sein (210.5(C)),EC&M (any colour but white or green) = ExpertCE (210.5(C) distinguishable)
US/NEC,L1 / L2 / L3 at 120/208 V or 120/240 V|L1 / L2 / L3 bei 120/208 V oder 120/240 V,black / red / blue|schwarz / rot / blau,"customary, not required by NEC|üblich, vom NEC nicht vorgeschrieben",trade practice|Praxis,neutral white|Neutralleiter weiß,EC&M = ExpertCE
US/NEC,L1 / L2 / L3 at 277/480 V|L1 / L2 / L3 bei 277/480 V,brown / orange / yellow|braun / orange / gelb,"customary, not required by NEC|üblich, vom NEC nicht vorgeschrieben",trade practice|Praxis,neutral gray; EC&M also lists brown / purple / yellow|Neutralleiter grau; EC&M nennt auch braun / violett / gelb,EC&M = ExpertCE
US/NEC,high leg of a 4-wire delta (120/240 V)|hochgelegter Außenleiter im 4-Leiter-Dreiecknetz (120/240 V),orange|orange,required marking (NEC 110.15)|vorgeschriebene Kennzeichnung (NEC 110.15),NFPA 70 (NEC) 110.15,marked at terminations and accessible points|an Anschlüssen und zugänglichen Stellen gekennzeichnet,"ExpertCE (110.15, commonly orange) = EC&M (120/240 V high-leg scheme black, orange, blue, white)"
```

## Sources
1. W. Hörmann, "Farben für Einzeladern", *de – das elektrohandwerk* 5/2015, pp. 15–16, https://www.elektro.net/file/show/80378/e70522/015-016_DE_2015_5_PP1.pdf — DIN VDE 0100-510:2011-03 514.3: PE green-yellow, N (light) blue, outer conductors brown, black or grey, no normative L1/L2/L3 assignment; Tables 1/2 from DIN VDE 0293-308:2003-01 (3-core: green-yellow, blue, brown; 5-core: green-yellow, blue, brown, black, grey); Table 3 ZVEH recommendation L1 brown, L2 black, L3 grey; accessed 2026-11-02.
2. G. Kiefer et al., *VDE 0100 und die Praxis*, VDE Verlag, pp. 697–701 (section 19.9 "Farbige Kennzeichnung …", Table 19.10), excerpt hosted by Voltimum, https://content.voltimum.com/de/128010/vde_0100_und_die_praxis_603616_seite_697-701.pdf — PE green-yellow; N blue ("hellblau" in VDE 0100:1973 and VDE 0100-510:1995, changed to "blau" by DIN VDE 0100-510 Bbl 1:2003-06 following HD 308); other conductors brown, black or grey; PEN green-yellow with blue end markings (only variant allowed in Germany); DIN EN 60446 (VDE 0198) merged into DIN EN 60445 (VDE 0197):2011-10; accessed 2026-11-02.
3. Prysmian UK, "Understanding core colours" (09/07/2025), https://uk.prysmian.com/media/news/understanding-core-colours — L1 brown, L2 black, L3 grey, N blue, earth green/yellow; old UK colours (red/yellow/blue phases, black neutral); change by BS 7671 Amendment 2 (2004), both allowed April 2004 – March 2006; accessed 2026-11-02.
4. M. Holt, "Code Q&A: NEC Requirements for Conductor Identification", *EC&M*, 23 March 2023, https://ecmweb.com/national-electrical-code/qa/article/21262479/code-qa-nec-requirements-for-conductor-identification — neutral white or gray (200.6), EGC green or green with yellow stripe (250.119), phase conductors any colour but white or green (200.7, 250.119), customary 120/208 V black/red/blue/white and 277/480 V brown/orange/yellow/gray (or brown/purple/yellow/gray), 120/240 V high-leg scheme black/orange/blue/white; accessed 2026-11-02.
5. ExpertCE, "NEC wire color codes" (3 Nov 2025), https://expertce.com/learn-articles/nec-wire-color-codes/ — neutral continuous white or gray (200.6), EGC bare or green (green with yellow stripes allowed) (250.119), 210.5(C) identification, high leg 110.15 "commonly orange", customary black/red/blue and brown/orange/yellow; accessed 2026-11-02.
6. code-knacker.de "Stromleitung – Adernkennzeichnungen" as reproduced in an elektrotanya.com PDF (2011), https://elektrotanya.com/sites/default/files/forum/2011/10/halozati%20vezetekek%20szinei.pdf — "bis 31.12.02: schwarz oder braun" for outer conductors (weak hobby source, used only for the historical note); accessed 2026-11-02.

## Cross-check
- IEC 60445 itself (paywalled) and NFPA 70 (free read-only access needs an account) were **not** read; values come from the sources above, which quote them.
- IEC/DE: PE green-yellow, N blue, outer conductors brown/black/grey — sources 1, 2, 3 agree. L1 brown / L2 black / L3 grey: source 1 (ZVEH recommendation, "keine normative Zuordnung" in Germany) and source 3 (UK practice under BS 7671) agree on the assignment; status is therefore "recommended", not "mandatory", for Germany.
- PEN green-yellow with blue end markings: source 2 only → **UNVERIFIED** (single source, though a VDE Verlag handbook).
- US: neutral white/gray and EGC green / green-yellow agree in sources 4 and 5; "bare EGC permitted" only in source 5 (single). Customary phase colours agree (4, 5). Three white stripes as an alternative neutral marking (200.6) appeared only in a search snippet, not in a page I read → not included.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Errichtungsbestimmungen.
- en: Never rely on colours alone in an existing installation — measure before you touch. Old installations may use other colours.
- de: In bestehenden Anlagen nie allein auf Farben verlassen – vor dem Berühren messen. Altanlagen können andere Farben haben.
- en: Historical Germany (note only, weak sources): before the harmonisation by DIN VDE 0293-308:2003-01 (HD 308 S2:2001) outer conductors were black or brown (no grey; source 6: "bis 31.12.02: schwarz oder braun"); the neutral was called "light blue" from VDE 0100:1973 (source 2); before about 1965 the protective conductor was often red and the neutral grey (popular sources, UNVERIFIED).
- de: Historisch Deutschland (nur Hinweis, schwache Quellen): vor der Harmonisierung durch DIN VDE 0293-308:2003-01 (HD 308 S2:2001) waren Außenleiter schwarz oder braun (kein Grau; Quelle 6: "bis 31.12.02: schwarz oder braun"); der Neutralleiter hieß ab VDE 0100:1973 "hellblau" (Quelle 2); vor etwa 1965 war der Schutzleiter oft rot und der Neutralleiter grau (populäre Quellen, UNVERIFIED).
- en: Historical UK (source 3): phases red / yellow / blue, neutral black; harmonised colours mandatory for new work from April 2006.
- de: Historisch UK (Quelle 3): Außenleiter rot / gelb / blau, Neutralleiter schwarz; harmonisierte Farben für Neuanlagen ab April 2006 verbindlich.
- en: Do not confuse the systems: in the US the neutral is white or gray and plain green marks the grounding conductor; in IEC countries the neutral is blue and PE is always green-yellow.
- de: Systeme nicht verwechseln: In den USA ist der Neutralleiter weiß oder grau und reines Grün kennzeichnet den Schutzleiter; in IEC-Ländern ist der Neutralleiter blau und der Schutzleiter immer grün-gelb.

## Suggested finder / lookups
- Conductor → colour for both systems side by side ("PE: green-yellow | US: green / green-yellow").
