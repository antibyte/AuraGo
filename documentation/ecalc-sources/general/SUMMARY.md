# SUMMARY — group "general"

All 10 topics done as 15 files (topic 5 → ip_codes + protection_classes; topic 7 → ism_bands, lora_regional_plans,
wifi_channels, amateur_bands_de; topic 8 → logic_levels + line_interface_levels). CSV blocks written with Python's csv
writer and parsed back. Helper scripts in a helper script that is not part of the repository; a helper script that is not part of the repository runs 81 arithmetic checks, all pass.
| file | topic | kind | rows | sources | guide note |
|---|---|---|---|---|---|
| si_prefixes.md | SI prefixes | definitional | 24 | 3 | no |
| si_units.md | SI base + derived units | definitional | 47 | 3 | no |
| physical_constants.md | constants (CODATA 2022 + 2018) | definitional + transcribed | 30 | 3 | no |
| greek_alphabet.md | Greek alphabet | definitional + typical (use column) | 24 | 6 | no |
| ip_codes.md | IP codes IEC 60529 | transcribed | 20 | 6 | yes |
| protection_classes.md | protection classes IEC 61140 | transcribed | 4 | 3 | yes |
| mains_by_country.md | mains V/Hz/plugs | transcribed (secondary) | 48 | 5 (1 unavailable) | yes |
| ism_bands.md | ISM (ITU) + EU SRD | transcribed | 20 | 5 | yes |
| lora_regional_plans.md | LoRaWAN EU868/US915/AS923 | transcribed | 11 | 5 | yes |
| wifi_channels.md | Wi-Fi 2.4/5 GHz | formula-derived + transcribed | 39 | 3 | yes |
| amateur_bands_de.md | AFuV Anlage 1 (BGBl. 2024 I Nr. 175) | transcribed | 45 | 3 | yes |
| logic_levels.md | TTL/LS/HC/HCT/CMOS/LVCMOS | transcribed (datasheets) | 8 | 7 | no |
| line_interface_levels.md | RS-232/RS-485/LVDS | transcribed | 18 | 9 | no |
| ascii.md | ASCII 0–127 | definitional | 128 | 2 | no |
| wire_colours.md | IEC/DE + US NEC | transcribed | 13 | 6 | yes |

## Open problems

### Sources not used
- IEC World Plugs is retired; web.archive.org was blocked. The mains table uses a three-way comparison of
  WorldStandards.eu, Wikipedia and Interpower (every cell backed by at least 2 of the 3).
- Not read (paywall, registration or download form): IEC 60529, IEC 61140, IEC 60445; ITU RR 5.138/5.150 (band list
  from FCC 47 CFR 18.301 and Wikipedia); JEDEC JESD8; TIA-232-F, TIA/EIA-485-A, TIA/EIA-644-A; NFPA 70, IEEE 802.11,
  DIN 1301; LoRa RP002-1.0.4/1.0.5 (values from 1.0.3).
- CODATA: brief asked for 2018; current set is 2022. Both given; recommendation: show 2022.

### UNVERIFIED or single-source cells
- ip_codes: the whole `test_hint` column (Wikipedia only); letters A–D and H/M/S/W omitted.
- protection_classes: IEC 60417-5019 for class I (one source).
- greek_alphabet: α "common-base gain" and θ/ϑ "angle; temperature" not backed by a source read.
- logic_levels: 74LS VOH 2.5 V and VOL 0.4/0.5 V (TI only); TTL outputs backed only by two TI documents.
- line_interface_levels: RS-232 3–7 kΩ load and 30 V/µs; RS-485 unit load 12 kΩ; LVDS VOD 247–454 mV and VOS
  1.125–1.375 V (one TI document each).
- wire_colours: PEN marking (one source); US "bare EGC" (one source); German colours before 1965 (weak popular sources;
  note text only).
- ism_bands: EU SRD power limits (433 MHz 10 mW/10 %, 2.4 GHz 10 mW, 5.8 GHz 25 mW) only from the binding EU legal text;
  Decision (EU) 2022/180 may have later amendments (check EUR-Lex).

### Choices on disagreement
- Mains voltage: CN, KR, AR 220 V (WorldStandards 230 V); CY 230 V (Wikipedia 240 V); RU 230 V (Interpower 220 V);
  MX 127 V.
- Plug types: only types listed by both WorldStandards and Wikipedia; extra types in the notes for DK, PT, SK, GB, AR,
  IN, SG.
- IP first numeral 2: 12.5 mm (BGN: 12 mm).
- LoRa EU868 RX2: DR0/SF12 per the spec (TTN uses SF9).
- Wi-Fi US channels 12/13: "no" per wireless-regdb (Wikipedia: allowed at low power).

### German text written by the agent
German names of the SI derived example units, the SI prefixes and the printable ASCII characters, plus the English IP
definitions (translated from Häwa's German). A German review pass is advisable.

### Subsets
48 countries; Wi-Fi without U-NII-4, 6 GHz and bonded channels; LoRa default channels only; ISM without the EU-only
169 MHz band.
