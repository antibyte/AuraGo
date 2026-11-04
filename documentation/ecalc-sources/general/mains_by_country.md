# mains_by_country — Mains voltage, frequency and plug types by country / Netzspannung, Frequenz und Steckertypen nach Land

Kind: transcribed standard (reference data from secondary sources)
Guide-value note needed: yes

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| iso2 | ISO 3166-1 alpha-2 country code | Ländercode nach ISO 3166-1 alpha-2 | – | no |
| country | country name "en\|de" | Ländername "en\|de" | – | no |
| voltage | nominal single-phase (line-to-neutral) household voltage; "127 / 220" = two regional values | Nennspannung einphasig (Außenleiter–Neutralleiter) im Haushalt; "127 / 220" = regional unterschiedlich | V | yes (except BR) |
| frequency | mains frequency; "50 / 60" = both in use (Japan) | Netzfrequenz; "50 / 60" = beide in Gebrauch (Japan) | Hz | yes (except JP) |
| plugs | plug/socket types (IEC letter designations A–O), comma-separated | Stecker-/Steckdosentypen (IEC-Buchstaben A–O), kommagetrennt | – | no |
| note | remark "en\|de" (source in brackets) | Hinweis "en\|de" (Quelle in Klammern) | – | no |
| check | cross-check record: WS = WorldStandards, WP = Wikipedia, IP = Interpower (IP only for V and Hz) | Abgleich der Quellen | – | no |

## Rows
```csv
iso2,country,voltage,frequency,plugs,note,check
AT,Austria|Österreich,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
BE,Belgium|Belgien,230,50,"C, E",some areas use a 3 × 230 V delta network without neutral (WP)|in einigen Gebieten 3 × 230 V Dreieck-Netz ohne Neutralleiter (WP),V/Hz/plugs: WS = WP = IP
BG,Bulgaria|Bulgarien,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
HR,Croatia|Kroatien,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
CY,Cyprus|Zypern,230,50,G,,"V: WS = IP 230, WP 240/415; Hz/plugs: WS = WP"
CZ,Czechia|Tschechien,230,50,"C, E",,V/Hz/plugs: WS = WP = IP
DK,Denmark|Dänemark,230,50,"C, F, K","type E sockets are also legal but rare (WP)|Steckdosen Typ E sind ebenfalls zulässig, aber selten (WP)","plugs: WS C/F/K, WP C/E/F/K; V/Hz: WS = WP = IP"
EE,Estonia|Estland,230,50,"C, F",,"V: WS = WP 230, IP 220–230; Hz/plugs: WS = WP"
FI,Finland|Finnland,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
FR,France|Frankreich,230,50,"C, E",,V/Hz/plugs: WS = WP = IP
DE,Germany|Deutschland,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
GR,Greece|Griechenland,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
HU,Hungary|Ungarn,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
IE,Ireland|Irland,230,50,G,,V/Hz/plugs: WS = WP = IP
IT,Italy|Italien,230,50,"C, F, L",type L in 10 A and 16 A versions; hybrid sockets are common (WP)|Typ L in 10-A- und 16-A-Ausführung; Kombisteckdosen verbreitet (WP),V/Hz/plugs: WS = WP = IP
LV,Latvia|Lettland,230,50,"C, F",,"V: WS = WP 230, IP 220; Hz/plugs: WS = WP"
LT,Lithuania|Litauen,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
LU,Luxembourg|Luxemburg,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
MT,Malta|Malta,230,50,G,,V/Hz/plugs: WS = WP = IP
NL,Netherlands|Niederlande,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
PL,Poland|Polen,230,50,"C, E",,V/Hz/plugs: WS = WP = IP
PT,Portugal|Portugal,230,50,"C, F",type E only in very old installations (WP)|Typ E nur in sehr alten Installationen (WP),"plugs: WS C/F, WP C/E/F; V/Hz: WS = WP = IP"
RO,Romania|Rumänien,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
SK,Slovakia|Slowakei,230,50,"C, E",WP also lists type F|WP nennt zusätzlich Typ F,"plugs: WS C/E, WP C/E/F; V/Hz: WS = WP = IP"
SI,Slovenia|Slowenien,230,50,"C, F",,"V: WS = WP 230, IP 220; Hz/plugs: WS = WP"
ES,Spain|Spanien,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
SE,Sweden|Schweden,230,50,"C, F",,V/Hz/plugs: WS = WP = IP
GB,United Kingdom|Vereinigtes Königreich,230,50,G,"types D and M only for special uses (lighting circuits, stage) (WP)|Typen D und M nur für Sonderzwecke (Lichtkreise, Bühne) (WP)","plugs: WS G, WP G/D/M; V/Hz: WS = WP = IP"
CH,Switzerland|Schweiz,230,50,"C, J",,V/Hz/plugs: WS = WP = IP
NO,Norway|Norwegen,230,50,"C, F",many homes use a 230 V IT network (no neutral to earth) (WP)|viele Haushalte haben ein 230-V-IT-Netz (WP),V/Hz/plugs: WS = WP = IP
US,United States|Vereinigte Staaten,120,60,"A, B",homes get 120/240 V split-phase; 240 V outlets (NEMA 14-30/14-50) for dryers and stoves (WP)|Haushalte haben 120/240 V (Split-Phase); 240-V-Steckdosen (NEMA 14-30/14-50) für Trockner und Herde (WP),V/Hz/plugs: WS = WP = IP
CA,Canada|Kanada,120,60,"A, B",homes get 120/240 V split-phase (WP)|Haushalte haben 120/240 V (Split-Phase) (WP),V/Hz/plugs: WS = WP = IP
MX,Mexico|Mexiko,127,60,"A, B",both 127/220 V three-phase and 120/240 V split-phase are used (WP)|sowohl 127/220 V Drehstrom als auch 120/240 V Split-Phase (WP),"V: WS = IP 127, WP 127/220 and 120/240; Hz/plugs: WS = WP"
BR,Brazil|Brasilien,127 / 220,60,"C, N","127 V or 220 V depending on the region (WS, WP)|je nach Region 127 V oder 220 V (WS, WP)","V: WS 127/220, WP 127/220 and 220/380, IP 127–220; Hz/plugs: WS = WP"
AR,Argentina|Argentinien,220,50,I,WP also lists type C; Argentine type I has live and neutral reversed vs. AU/CN (WP)|WP nennt zusätzlich Typ C; argentinischer Typ I hat L und N gegenüber AU/CN vertauscht (WP),"V: WP = IP 220, WS 230; plugs: WS I, WP C/I; Hz: WS = WP = IP"
CN,China|China,220,50,"A, I",Hong Kong and Macau use type G (WS)|Hongkong und Macau verwenden Typ G (WS),"V: WP = IP 220, WS 230; Hz/plugs: WS = WP"
JP,Japan|Japan,100,50 / 60,"A, B","east Japan 50 Hz (e.g. Tokyo), west Japan 60 Hz (e.g. Osaka) (WP)|Ostjapan 50 Hz (z. B. Tokio), Westjapan 60 Hz (z. B. Osaka) (WP)",V/Hz/plugs: WS = WP = IP
KR,South Korea|Südkorea,220,60,"C, F",,"V: WP = IP 220, WS 230; Hz/plugs: WS = WP = IP"
IN,India|Indien,230,50,"D, M","WP also lists type C (Europlug fits D sockets, not recommended)|WP nennt zusätzlich Typ C (Eurostecker passt in D-Steckdosen, nicht empfohlen)","plugs: WS D/M, WP C/D/M; V/Hz: WS = WP = IP"
AU,Australia|Australien,230,50,I,"officially 230 V, in practice often 240 V; Western Australia 240/415 V (WS, WP)|offiziell 230 V, in der Praxis oft 240 V; Westaustralien 240/415 V (WS, WP)",V/Hz/plugs: WS = WP = IP
NZ,New Zealand|Neuseeland,230,50,I,,V/Hz/plugs: WS = WP = IP
ZA,South Africa|Südafrika,230,50,"C, M, N",,"V: WS = WP 230, IP 220–250; Hz/plugs: WS = WP"
RU,Russia|Russland,230,50,"C, F",older networks 220 V (WP: 230/400 and 220/380 V)|ältere Netze 220 V (WP: 230/400 und 220/380 V),"V: WS 230, WP 230 and 220, IP 220; Hz/plugs: WS = WP = IP"
TR,Türkiye|Türkei,230,50,"C, F",,"V: WS = WP 230, IP 220; Hz/plugs: WS = WP"
IL,Israel|Israel,230,50,"C, H",,V/Hz/plugs: WS = WP = IP
AE,United Arab Emirates|Vereinigte Arabische Emirate,230,50,G,,"V: WS = WP 230, IP 220–240; Hz/plugs: WS = WP"
SG,Singapore|Singapur,230,50,G,WP also lists C and M (M for air conditioners)|WP nennt zusätzlich C und M (M für Klimageräte),"plugs: WS G, WP C/G/M; V/Hz: WS = WP = IP"
TH,Thailand|Thailand,230,50,"A, B, C, O",hybrid A/B/C sockets are most common (WP)|Kombisteckdosen A/B/C am häufigsten (WP),"V: WS = WP 230, IP 220; Hz/plugs: WS = WP"
```

## Sources
1. **IEC World Plugs (iec.ch/world-plugs) — NOT AVAILABLE.** The page now says "The interactive guide to world plugs is no longer available! We have retired this feature …" (read via the in-app browser, 2026-11-02). Archived copies (web.archive.org) could not be opened (blocked). The brief's primary source therefore could not be used.
2. WorldStandards.eu, "Plug, socket & voltage by country", https://www.worldstandards.eu/electricity/plug-voltage-by-country/ ("Last updated: 7 September 2025") — single-phase voltage, frequency, plug types (WS); accessed 2026-11-02.
3. en.wikipedia, "Mains electricity by country", https://en.wikipedia.org/wiki/Mains_electricity_by_country — plug types, national plug standard, nominal system voltage (phase/line), frequency, notes (WP); accessed 2026-11-02.
4. Interpower Corporation (cord-set maker), "International Configurations guide", https://www.interpower.com/ic/guide.html (redirects to /cgi-bin/ic.cgi/guidelist.p) — voltage and frequency per country (IP); its plug patterns use Interpower's own codes and are not used for the IEC letters; accessed 2026-11-02.
5. WorldStandards.eu, "Plug & socket types", https://www.worldstandards.eu/electricity/plugs-and-sockets/ — legend for types A–O (see notes); accessed 2026-11-02.

## Cross-check
Tables 2 and 3 were downloaded and parsed with a helper script that is not part of the repository (no LLM summarising); Interpower rows were extracted the same way.
- **Frequency:** all three sources agree for all 48 countries.
- **Voltage disagreements (chosen value, why):**
  - China, South Korea, Argentina: WS 230 V vs WP and IP 220 V → **220 V** (two of three; WS lists "230 V" for nearly every 220–240 V country).
  - Cyprus: WS and IP 230 V vs WP 240/415 V → **230 V**.
  - Russia: WS 230 V, WP 230/400 V and 220/380 V, IP 220 V → **230 V** (WS + WP), note "older networks 220 V".
  - Mexico: WS and IP 127 V, WP 127/220 V and 120/240 V → **127 V**.
  - Latvia, Slovenia, Türkiye, Thailand: WS and WP 230 V vs IP 220 V → **230 V**. Estonia, South Africa, UAE: IP gives a range (220–230 / 220–250 / 220–240 V) → **230 V** (WS + WP).
- **Plug-type disagreements:** only types listed by **both** WS and WP are in `plugs`; extra types from one source are in `note`: Denmark (WP adds E), Portugal (WP adds E, "very old installations"), Slovakia (WP adds F), UK (WP adds D, M, special uses), Argentina (WP adds C), India (WP adds C), Singapore (WP adds C, M).
- **Not independent of IEC:** WP cites the (now retired) IEC World Plugs pages for many rows; WS and IP are independent compilations. No row is UNVERIFIED: every chosen voltage, frequency and plug type is backed by at least two of the three sources.
- **Subset:** 48 countries (EU 27 + GB, CH, NO, US, CA, MX, BR, AR, CN, JP, KR, IN, AU, NZ, ZA, RU, TR, IL, AE, SG, TH). The full world list (~230 entries in WS/WP) would need the same three-way comparison; WS and WP already parse cleanly with the scripts, so extending is mechanical, but every disagreement must be reviewed by hand.

## Notes (en / de)
- en: Guide values — the standard is authoritative; follow the installation rules that apply locally.
- de: Richtwerte – maßgeblich ist die Norm; beachten Sie die örtlich geltenden Errichtungsbestimmungen.
- en: Nominal voltages have tolerances; the permitted band is set by the local grid code (not part of this table). Many countries listed as 220 V are converging on 230 V.
- de: Nennspannungen haben Toleranzen; das zulässige Band legt der örtliche Netzbetreiber bzw. die Norm fest (nicht Teil dieser Tabelle). Viele 220-V-Länder nähern sich 230 V an.
- en: Plug type legend (WorldStandards, source 5 — pin count, earthing, rated current, main region; single source, typical): A 2 pins, unearthed, 15 A (US/CA/MX/JP); B 3 pins, earthed, 15 A (US/CA/MX); C 2 pins, unearthed, 2.5 A / 10 A / 16 A (Europe and many others); D 3 pins, earthed, 6 A (India region); E 2 pins + earth, 16 A (FR/BE/PL/SK/CZ); F 2 pins + earth, 16 A (most of Europe, RU); G 3 pins, earthed, 13 A (UK/IE/MT/SG/Arabian Peninsula); H 3 pins, earthed, 16 A (Israel); I 2 or 3 pins, 10 A (AU/NZ/CN/AR); J 3 pins, earthed, 10 A (CH/LI); K 3 pins, earthed, 16 A (DK); L 3 pins, earthed, 10 A / 16 A (IT/CL); M 3 pins, earthed, 16 A (ZA); N 3 pins, earthed, 10 A / 16 A / 20 A (BR/ZA); O 3 pins, earthed, 16 A (TH).
- de: Legende Steckertypen (WorldStandards, Quelle 5 — Stiftzahl, Schutzleiter, Bemessungsstrom, Hauptregion; eine Quelle, typisch): A 2 Stifte, ohne Schutzleiter, 15 A (US/CA/MX/JP); B 3 Stifte, mit Schutzleiter, 15 A (US/CA/MX); C 2 Stifte, ohne Schutzleiter, 2,5 A / 10 A / 16 A (Europa und viele andere); D 3 Stifte, mit Schutzleiter, 6 A (Indien und Nachbarländer); E 2 Stifte + Schutzkontakt, 16 A (FR/BE/PL/SK/CZ); F 2 Stifte + Schutzkontakt, 16 A (fast ganz Europa, RU); G 3 Stifte, mit Schutzleiter, 13 A (UK/IE/MT/SG/Arabische Halbinsel); H 3 Stifte, mit Schutzleiter, 16 A (Israel); I 2 oder 3 Stifte, 10 A (AU/NZ/CN/AR); J 3 Stifte, mit Schutzleiter, 10 A (CH/LI); K 3 Stifte, mit Schutzleiter, 16 A (DK); L 3 Stifte, mit Schutzleiter, 10 A / 16 A (IT/CL); M 3 Stifte, mit Schutzleiter, 16 A (ZA); N 3 Stifte, mit Schutzleiter, 10 A / 16 A / 20 A (BR/ZA); O 3 Stifte, mit Schutzleiter, 16 A (TH).

## Suggested finder / lookups
- Country (name en/de or ISO code) → voltage, frequency, plug types.
- Reverse: plug type → list of countries; "is my 230 V / 50 Hz device usable in X?" → compare voltage and frequency.
