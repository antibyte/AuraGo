# nec_ampacity_awg — NEC allowable ampacities (Table 310.16) / Zulässige Strombelastbarkeit nach NEC (Tabelle 310.16)

Kind: transcribed standard (NFPA 70, National Electrical Code 2023, Table 310.16 — identical values in NEC 2017 Table 310.15(B)(16))
Guide-value note needed: yes

Conditions: insulated conductors rated 0–2000 V, not more than three current-carrying conductors in raceway, cable or earth (directly buried), ambient 30 °C (86 °F). Columns by conductor temperature rating 60 °C (types TW, UF), 75 °C (RHW, THHW, THW, THWN, XHHW, USE, ZW …), 90 °C (THHN, THWN-2, XHHW-2, RHH, RHW-2, USE-2 …).
Bedingungen: isolierte Leiter 0–2000 V, höchstens drei stromführende Leiter im Rohr, Kabel oder Erdreich, Umgebung 30 °C.

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| size | Conductor size, AWG or kcmil (text code) | Leitergröße, AWG oder kcmil (Text) | AWG / kcmil | no |
| size_kind | "AWG" or "kcmil" | "AWG" oder "kcmil" | — | no |
| cu_60 | Copper, 60 °C rated insulation | Kupfer, Isolierung 60 °C | A | yes |
| cu_75 | Copper, 75 °C rated insulation | Kupfer, Isolierung 75 °C | A | yes |
| cu_90 | Copper, 90 °C rated insulation | Kupfer, Isolierung 90 °C | A | yes |
| al_60 | Aluminium or copper-clad aluminium, 60 °C | Aluminium oder kupferkaschiertes Aluminium, 60 °C | A | yes |
| al_75 | Aluminium / CCA, 75 °C | Aluminium / CCA, 75 °C | A | yes |
| al_90 | Aluminium / CCA, 90 °C | Aluminium / CCA, 90 °C | A | yes |
| ocpd_note | Footnote mark: overcurrent protection limited by NEC 240.4(D) | Fußnote: Überstromschutz begrenzt nach NEC 240.4(D) | — | no |

## Rows
```csv
size,size_kind,cu_60,cu_75,cu_90,al_60,al_75,al_90,ocpd_note
18,AWG,,,14,,,,240.4(D)
16,AWG,,,18,,,,240.4(D)
14,AWG,15,20,25,,,,240.4(D)
12,AWG,20,25,30,15,20,25,240.4(D)
10,AWG,30,35,40,25,30,35,240.4(D)
8,AWG,40,50,55,35,40,45,
6,AWG,55,65,75,40,50,55,
4,AWG,70,85,95,55,65,75,
3,AWG,85,100,115,65,75,85,
2,AWG,95,115,130,75,90,100,
1,AWG,110,130,145,85,100,115,
1/0,AWG,125,150,170,100,120,135,
2/0,AWG,145,175,195,115,135,150,
3/0,AWG,165,200,225,130,155,175,
4/0,AWG,195,230,260,150,180,205,
250,kcmil,215,255,290,170,205,230,
300,kcmil,240,285,320,195,230,260,
350,kcmil,260,310,350,210,250,280,
400,kcmil,280,335,380,225,270,305,
500,kcmil,320,380,430,260,310,350,
```

No UNVERIFIED cells.

## Sources
1. NFPA, NEC Code-Making Panel 6, "Working draft of panel meeting output, January 18–20, 2024" (First Draft report for the 2026 NEC cycle), which reproduces the current NEC 2023 Table 310.16 in Public Input 1016 and First Revisions FR-8036/FR-8257: https://docinfofiles.nfpa.org/files/AboutTheCodes/70/70_A2025_NEC_P06_FD_PrelimFR.pdf — full table 18 AWG–2000 kcmil, Cu and Al; accessed 2026-11-02.
2. Omni Cable, "NEC Ampacity Data" (05-01-2018), Table 310.15(B)(16) (formerly 310.16), NEC 2017: https://www.omnicable.com/docs/default-source/technical-resources/nec-ampacity-data.pdf?sfvrsn=2 — full table; accessed 2026-11-02.
3. Cerrowire, "Ampacity Charts" ("Ampacities are based on the 2017 NEC"): https://www.cerrowire.com/products/resources/tables-calculators/ampacity-charts/ — Cu 60/75/90 (60 °C only to 2 AWG), Al 75/90, no 400 kcmil; accessed 2026-11-02.
4. Lapp (Miltronic Denmark), "Selection Table T13: Power Rating according to the National Electrical Code of the USA", Table 13-1 "Abstract of NEC Tabelle 310-16" (copper only, print 2010): https://t3.lappcdn.com/fileadmin/DAM/Miltronic_Denmark/4_servicecenter/T13.pdf — older NEC edition; accessed 2026-11-02.

## Cross-check
- Sources 1 (NEC 2023 text) and 2 (NEC 2017) agree on every cell in the table (Cu and Al, all three columns). Source 3 agrees on every cell it lists.
- Source 4 reflects an older NEC edition and differs in: 14 AWG Cu 20*/20*/25* (now 15/20/25), 12 AWG Cu 25*/25*/30* (now 20/25/30), 3 AWG Cu 90 °C 110 (now 115), 1 AWG Cu 90 °C 150 (now 145), 600 kcmil Cu 60 °C 355 (now 350), and 400 kcmil Cu 75 °C 355 (all other sources 335; probably a misprint). Chosen: current values (sources 1–3).
- NEC 2026 outlook: First Revision FR-8257 (source 1) adds 10 A at 60 °C for 16 AWG copper and 14 AWG copper-clad aluminium; not included because the final 2026 text was not checked.
- 240.4(D) limits: source 4 note gives 15 A for 14 AWG, 20 A for 12 AWG and 30 A for 10 AWG (copper); the table only carries the reference.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. US view (NEC); for IEC/DIN installations use the cable_ampacity_* tables. The overcurrent device for 14/12/10 AWG copper is limited to 15/20/30 A (NEC 240.4(D)) regardless of the table value; terminal temperature ratings (usually 60 °C or 75 °C) limit which column may be used. Correct for ambient temperature (NEC 310.15(B)) and for more than three current-carrying conductors (NEC 310.15(C)(1)).
- de: Richtwerte — maßgebend ist die Norm; die örtlich geltenden Errichtungsbestimmungen sind einzuhalten. US-Sicht (NEC); für IEC/DIN-Anlagen die Tabellen cable_ampacity_* verwenden. Absicherung für 14/12/10 AWG Kupfer höchstens 15/20/30 A (NEC 240.4(D)); die Temperaturfestigkeit der Klemmen (meist 60 °C oder 75 °C) bestimmt die nutzbare Spalte.

## Suggested finder / lookups
Smallest size with column value ≥ load current; size order is the row order. AWG area/diameter come from ECalc's AWG formula (not from this table).
