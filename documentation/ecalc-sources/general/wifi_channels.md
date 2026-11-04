# wifi_channels — Wi-Fi channels 2.4 GHz and 5 GHz / WLAN-Kanäle 2,4 GHz und 5 GHz

Kind: formula-derived (centre frequencies) + transcribed standard (regulatory availability)
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| band | `2.4 GHz` or `5 GHz` | Frequenzband | – | no |
| channel | IEEE 802.11 channel number | Kanalnummer nach IEEE 802.11 | – | yes |
| centre | centre frequency: 2.4 GHz: 2407 + 5·ch (ch 1–13), ch 14 = 2484; 5 GHz: 5000 + 5·ch | Mittenfrequenz | MHz | yes |
| f_low | lower edge of the 20 MHz channel (centre − 10) | untere Grenze des 20-MHz-Kanals | MHz | yes |
| f_high | upper edge of the 20 MHz channel (centre + 10) | obere Grenze des 20-MHz-Kanals | MHz | yes |
| us_band | US FCC U-NII band name (5 GHz only) | US-FCC-U-NII-Bandname (nur 5 GHz) | – | no |
| de | availability for WLAN in Germany/EU "en\|de" (`indoor` = indoor only, `DFS` = dynamic frequency selection required) | Verfügbarkeit für WLAN in Deutschland/EU "en\|de" | – | no |
| us | availability in the USA "en\|de" | Verfügbarkeit in den USA "en\|de" | – | no |
| note | remark "en\|de" | Hinweis "en\|de" | – | no |

## Rows
```csv
band,channel,centre,f_low,f_high,us_band,de,us,note
2.4 GHz,1,2412,2402,2422,,yes|ja,yes|ja,
2.4 GHz,2,2417,2407,2427,,yes|ja,yes|ja,
2.4 GHz,3,2422,2412,2432,,yes|ja,yes|ja,
2.4 GHz,4,2427,2417,2437,,yes|ja,yes|ja,
2.4 GHz,5,2432,2422,2442,,yes|ja,yes|ja,
2.4 GHz,6,2437,2427,2447,,yes|ja,yes|ja,
2.4 GHz,7,2442,2432,2452,,yes|ja,yes|ja,
2.4 GHz,8,2447,2437,2457,,yes|ja,yes|ja,
2.4 GHz,9,2452,2442,2462,,yes|ja,yes|ja,
2.4 GHz,10,2457,2447,2467,,yes|ja,yes|ja,
2.4 GHz,11,2462,2452,2472,,yes|ja,yes|ja,
2.4 GHz,12,2467,2457,2477,,yes|ja,no|nein,US: regdb does not allow it; Wikipedia: allowed only at low power and normally avoided|USA: laut regdb nicht erlaubt; laut Wikipedia nur mit geringer Leistung erlaubt und meist gemieden
2.4 GHz,13,2472,2462,2482,,yes|ja,no|nein,US: regdb does not allow it; Wikipedia: allowed only at low power and normally avoided|USA: laut regdb nicht erlaubt; laut Wikipedia nur mit geringer Leistung erlaubt und meist gemieden
2.4 GHz,14,2484,2474,2494,,no|nein,no|nein,"Japan only, 802.11b (DSSS/CCK) only|nur Japan, nur 802.11b (DSSS/CCK)"
5 GHz,36,5180,5170,5190,U-NII-1,indoor|nur innen,yes|ja,
5 GHz,40,5200,5190,5210,U-NII-1,indoor|nur innen,yes|ja,
5 GHz,44,5220,5210,5230,U-NII-1,indoor|nur innen,yes|ja,
5 GHz,48,5240,5230,5250,U-NII-1,indoor|nur innen,yes|ja,
5 GHz,52,5260,5250,5270,U-NII-2A,"indoor, DFS|nur innen, DFS",DFS|DFS,
5 GHz,56,5280,5270,5290,U-NII-2A,"indoor, DFS|nur innen, DFS",DFS|DFS,
5 GHz,60,5300,5290,5310,U-NII-2A,"indoor, DFS|nur innen, DFS",DFS|DFS,
5 GHz,64,5320,5310,5330,U-NII-2A,"indoor, DFS|nur innen, DFS",DFS|DFS,
5 GHz,100,5500,5490,5510,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,104,5520,5510,5530,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,108,5540,5530,5550,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,112,5560,5550,5570,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,116,5580,5570,5590,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,120,5600,5590,5610,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,124,5620,5610,5630,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,128,5640,5630,5650,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,132,5660,5650,5670,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,136,5680,5670,5690,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,140,5700,5690,5710,U-NII-2C,DFS|DFS,DFS|DFS,
5 GHz,144,5720,5710,5730,U-NII-2C/3,no|nein,DFS|DFS,5710–5730 MHz exceeds the EU WLAN band end 5725 MHz|5710–5730 MHz überschreitet das EU-WLAN-Bandende 5725 MHz
5 GHz,149,5745,5735,5755,U-NII-3,"SRD only, 25 mW|nur SRD, 25 mW",yes|ja,
5 GHz,153,5765,5755,5775,U-NII-3,"SRD only, 25 mW|nur SRD, 25 mW",yes|ja,
5 GHz,157,5785,5775,5795,U-NII-3,"SRD only, 25 mW|nur SRD, 25 mW",yes|ja,
5 GHz,161,5805,5795,5815,U-NII-3,"SRD only, 25 mW|nur SRD, 25 mW",yes|ja,
5 GHz,165,5825,5815,5835,U-NII-3,"SRD only, 25 mW|nur SRD, 25 mW",yes|ja,
```

## Sources
1. Linux kernel, `net/wireless/util.c`, function `ieee80211_channel_to_freq_khz()` (comment: "see 802.11 17.3.8.3.2 and Annex J"), https://raw.githubusercontent.com/torvalds/linux/master/net/wireless/util.c — 2.4 GHz: ch 14 → 2484 MHz, ch < 14 → 2407 + 5·ch; 5 GHz: 5000 + 5·ch (182–196: 4000 + 5·ch, not used here); accessed 2026-11-02.
2. en.wikipedia, "List of WLAN channels", https://en.wikipedia.org/wiki/List_of_WLAN_channels — 2.4 GHz table (F0 2412…2484 MHz; channel 14 Japan 802.11b only; channels 12/13 in North America only at low power), 5 GHz table (F0, frequency ranges, U-NII band names); accessed 2026-11-02.
3. wireless-regdb `db.txt` (Linux regulatory database), https://git.kernel.org/pub/scm/linux/kernel/git/wens/wireless-regdb.git/plain/db.txt — rules for country DE (2400–2483.5; 5150–5250 NO-OUTDOOR; 5250–5350 NO-OUTDOOR DFS; 5470–5725 DFS; 5725–5875 25 mW "short range devices (ETSI EN 300 440-1)") and US (2400–2472; 5150–5250; 5250–5350 DFS; 5470–5730 DFS; 5730–5850); accessed 2026-11-02.

## Cross-check
- **Centre frequencies:** formula (source 1) = Wikipedia table (source 2) for all 14 channels at 2.4 GHz and all listed 5 GHz channels (checked in a helper script that is not part of the repository, 45 channel checks pass). The IEEE 802.11 standard itself was not read (paywalled); source 1 cites its clause.
- **Availability columns:** derived from wireless-regdb (source 3) by checking whether the 20 MHz channel lies inside an allowed range. Source 2 agrees on: 2.4 GHz channels 1–13 in Europe, 1–11 normal in the US, 14 Japan-only/11b-only, U-NII band names, and DFS for 5250–5350 and 5470–5725 MHz. **Disagreement:** US channels 12/13 — regdb: not allowed; Wikipedia: allowed under low-power conditions but avoided → table says "no" with a note.
- 5 GHz channels 169–177 (U-NII-4, US indoor since 2021) and the 6 GHz band are not included (subset).
- Bonded 40/80/160 MHz channels are not listed.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally. Regulatory details (power limits, indoor rules, DFS) change; check the current national rules (in Germany: Bundesnetzagentur).
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Regeln. Regulatorische Details (Leistung, Innenraum, DFS) ändern sich; maßgeblich sind die aktuellen nationalen Vorgaben (in Deutschland: Bundesnetzagentur).
- en: At 2.4 GHz neighbouring channels are only 5 MHz apart, so 20 MHz channels overlap.
- de: Bei 2,4 GHz liegen benachbarte Kanäle nur 5 MHz auseinander; 20-MHz-Kanäle überlappen sich deshalb.

## Suggested finder / lookups
- Channel → centre frequency (formula); frequency → channel (inverse: (f − 2407)/5 or (f − 5000)/5).
- Overlap check: two 2.4 GHz channels overlap when |ch1 − ch2| < 4 for 20 MHz OFDM.
