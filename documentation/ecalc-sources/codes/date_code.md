# date_code — Component date codes (4-digit year/week, IEC 60062 year/month letter code) / Datumscodes auf Bauteilen (vierstellig Jahr/Woche, Jahr/Monat-Buchstabencode nach IEC 60062)

Kind: definitional (IEC 60062 date code clauses, read in the identical GOST adoption of the 2004 edition; ISO 8601-1
week definition; maker marking documents)
Guide-value note needed: yes (the week a maker counts is maker-dependent, see "Which maker uses which form")

What the codes are: the manufacturing date that makers print on a component body, next to the part number. Two forms
are covered here:

1. **Four digits, year and week.** IEC 60062 defines it as **YYWW**: the last two digits of the year, then the week
   number ("fifth week of 2006 = 0605"). Every maker document read prints YYWW (or a shortened YWW / YW). **No maker
   document read prints WWYY** (see UNVERIFIED); ECalc does not read it.
2. **Two characters, year letter and month (IEC 60062, twenty-year cycle).** First character = year letter
   (A B C D E F H J K L M N P R S T U V W X, repeating every 20 years), second character = month (1–9 = January to
   September, O = October, N = November, D = December). Example "J5" = May 2017.

The standard: IEC 60062:2016 *Marking codes for resistors and capacitors*, **Clause 8 "Date code system for
capacitors and resistors"**: 8.2 two-character codes for year and month (8.2.2 twenty-year cycle, Table 17 month
characters, Table 18 year letters; 8.2.3 ten-year cycle, Table 19), 8.3 four-character codes for year and week
(8.3.2 fully numerical four-numeral code, 8.3.3 alphanumerical twenty-year cycle code, 8.3.4 ten-year cycle code),
8.4 single-character code (Table 20). ISO 8601 is a normative reference (Clause 2). These clause and table titles are
from the 2016 contents page (iTeh preview); the clause text and tables of the 2016 edition are **not** in the preview.
The tables were read in the 2004 edition (IEC 60062:2004), whose date code is **Clause 6** and which is published
word for word as GOST IEC 60062-2014 (IDT) — see Sources.

## Rules

### Four-digit year/week code (IEC 60062:2004 clause 6.2.1; IEC 60062:2016 clause 8.3.2)
- GOST IEC 60062-2014 (= IEC 60062:2004) 6.2.1, translated: "Where the year and week of manufacture must be marked, a
  four-digit code system may be used. The first two digits shall be the last two digits of the year, and the last two
  digits shall be the week number. Example: fifth week of 2006 = 0605."
- Week numbering: 6.2.2 and 6.2.3 (the letter + "W" + week forms, e.g. "KW05" = week 5 of 1998) say "week numbering
  shall be in accordance with ISO 8601"; 6.2.1 itself does not repeat the sentence, but ISO 8601 is a normative
  reference of the whole standard (Clause 2 of both editions).
- **ISO 8601 week rules** (ISO 8601-1:2019 term 3.1.1.23 "week calendar", read in the iTeh preview; Monday start and
  the equivalent forms from two independent references, Kuhn and Wikipedia):
  - A week runs Monday (day 1) to Sunday (day 7).
  - Week 01 of a year is the week that contains the first Thursday of that year (equivalently: the week that contains
    4 January; the week with at least four of its days in the new year). ISO 8601-1:2019 3.1.1.23: "the first calendar
    week of a calendar year is the week including the first Thursday of that year, and … the last one is the week
    immediately preceding the first calendar week of the next calendar year"; Note 1: a week belongs to the year that
    holds the majority of its days.
  - So week 01 can start as early as Monday 29 December of the previous year, and the last week can end as late as
    Sunday 3 January of the next year.
  - A year has **53 weeks** when 1 January is a Thursday, or when it is a leap year and 1 January is a Wednesday
    (Wikipedia; equivalently: the year has 53 Thursdays). All other years have 52.
  - Long years computed with Python `date(y, 12, 28).isocalendar()[1]` (28 December always lies in the last week):
    2000–2099: **2004, 2009, 2015, 2020, 2026, 2032, 2037, 2043, 2048, 2054, 2060, 2065, 2071, 2076, 2082, 2088, 2093,
    2099** (identical to the list in Wikipedia's 400-year table); 1970–1999: 1970, 1976, 1981, 1987, 1992, 1998.
- Valid week numbers: 01 to 52, and 53 only in a long year. 00 and 54…99 are never valid.
- A date belongs to the **ISO week-year**, which can differ from its calendar year at the turn of the year: 1 January
  2021 is in 2020-W53 (code 2053), 29 December 2025 is in 2026-W01 (code 2601).
- Two-digit year: read as 2000–2099. A part can be from 1900–1999 (TI's own example: "Calendar week 48, 1999 = 9948");
  show that as a note, with the 1900s reading on request.

### Two-character year/month code (IEC 60062:2004 clause 6.1.1, Tables 10a/10b; IEC 60062:2016 clause 8.2.2, Tables 17/18)
- First character, year (Table 10a, twenty-year cycle; note under the table: "These codes, which show the year, repeat
  after every 20-year cycle"):

| Letter | Years (Table 10a, 2004 ed.) | Current cycle (TDK 2020 / Vishay 2024) |
|---|---|---|
| A | 1970, 1990 | 2010 (cycle); 2030 (Vishay) |
| B | 1971, 1991 | 2011 (cycle), 2031 (cycle) |
| C | 1972, 1992 | 2012 (TDK) |
| D | 1973, 1993 | 2013 (TDK) |
| E | 1974, 1994 | 2014 (TDK) |
| F | 1975, 1995 | 2015 (TDK) |
| H | 1976, 1996 | 2016 (TDK) |
| J | 1977, 1997 | 2017 (TDK; example "J5 = 2017 May") |
| K | 1978, 1998 | 2018 (TDK) |
| L | 1979, 1999 | 2019 (TDK) |
| M | 1980, 2000 | 2020 (TDK) |
| N | 1981, 2001 | 2021 (TDK) |
| P | 1982, 2002 | 2022 (TDK, Vishay) |
| R | 1983, 2003 | 2023 (TDK, Vishay) |
| S | 1984, 2004 | 2024 (Vishay) |
| T | 1985, 2005 | 2025 (Vishay) |
| U | 1986, 2006 | 2026 (Vishay) |
| V | 1987, 2007 | 2027 (Vishay) |
| W | 1988, 2008 | 2028 (Vishay) |
| X | 1989, 2009 | 2029 (Vishay) |

  Rule: letter index i in "ABCDEFHJKLMNPRSTUVWX" (A = 0 … X = 19) means every year with (year − 1970) mod 20 = i.
  **Not used as year letters: G, I, O, Q, Y, Z.** That set is read off Table 10a (the letters are simply absent); the
  standard text read does not give a reason. Wikipedia (RKM code, footnote) states the same six letters "to reduce the
  risk for read errors".
- Second character, month (Table 10b): 1 January, 2 February, 3 March, 4 April, 5 May, 6 June, 7 July, 8 August,
  9 September, **O October** (the letter O, not zero), **N November**, **D December**. Examples in the standard:
  "March 1998 = K3; November 1999 = LN".
- The same month characters are printed by TDK (2020) and Vishay (2024), see below.

## Which maker uses which form

| Maker / body | Form printed | What the document says about the week | Document |
|---|---|---|---|
| IEC 60062 (2004 ed. via GOST IDT; 2016 contents) | YYWW (4 digits), also year letter + "W" + week, year digit + "W" + week; year letter + month | ISO 8601 | Sources 1–2 |
| ECIA (US distributor/maker association) | YYWW, "the first two digits being the last two digits of the calendar year and the last two digits being the week of the year" | week 1 = first week in which the first Thursday falls; **first day of the week = Sunday**; week 53 when a year has 53 Thursdays | Source 5 |
| Texas Instruments (QML Class Q/V, per MIL-PRF-38535) | YYWW plus optional suffix (YYWWAA), "for example, 2010A, 2010B or 2010BA" | not stated | Source 6 |
| Texas Instruments / National Semiconductor | four-digit YYWW, three-digit YWW, two-digit YW, one-character alpha code | the week is "the starting calendar week of the six-week period during which the device was assembled" (example: calendar week 14, 2000 → 0012; week 32, 2001 → 0130; week 48, 1999 → 9948) | Source 7 |
| Microchip | YYWW (e.g. PDIP line "YYWW", examples 0432, 0746), YWW on small packages, YYWWNNN trace code | "Week code (week of January 1 is week '01')" — **not** the ISO rule in years where 1 January is a Friday, Saturday or Sunday | Source 8 |
| onsemi | YYWW (1N4001 "YYWWG"), YWW in "AWLYWWG" (LM317) | "YY = Year", "Y = Year", "WW = Work Week"; numbering not stated | Sources 9–10 |
| Vishay Dale (resistors) | YYWW with a location letter in front ("M1843"; WSL "1119") | "year, week code (YYWW)"; "Date code printed based upon week and year of manufacture"; numbering not stated | Source 11 |
| TDK Electronics (EPCOS film capacitors) | IEC 60062 year letter + month | "Codes for date of manufacture (to IEC 60062:2004)" | Source 3 |
| Vishay (rectifiers, TVS, small-signal diodes in SMF) | IEC year letter + month (+ optional factory letter) | "According EN 60062" (printed "EN 600626") | Source 4 |
| Vishay (film capacitors, current datasheets) | "year and week" in the marking | not detailed | Source 12 |

So: YYWW is stated by IEC 60062, ECIA, TI (two documents), Microchip, onsemi and Vishay Dale. **None of the documents
read uses WWYY.** The week itself is counted three different ways: ISO 8601 (IEC 60062; Monday–Sunday), ECIA
(Sunday–Saturday, same Thursday anchor, so the same week number but every week starts one day earlier), Microchip (the
week holding 1 January is week 01), and TI/National commercial parts mark the first week of a six-week period.

## Columns (suggested data shape)

Table A — forms

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| form | code form | Codeform | – | no |
| pattern | code pattern | Code-Muster | – | no |
| rule | how to read it | Leseregel | – | no (en\|de) |
| status | verification status | Prüfstatus | – | no (en\|de) |

Table B — year letters (twenty-year cycle)

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| letter | year letter | Jahresbuchstabe | – | no |
| index | position in the 20-year cycle (A = 0) | Stelle im 20-Jahres-Zyklus (A = 0) | – | yes |
| years | years in 1970–2049 | Jahre zwischen 1970 und 2049 | – | no |

Table C — month characters

| id | meaning (en) | Bedeutung (de) | unit | numeric? |
|---|---|---|---|---|
| char | month character | Monatszeichen | – | no |
| month | month number | Monatsnummer | – | yes |

## Rows

Table A

```csv
form,pattern,rule,status
yyww,DDDD,"last two digits of the year, then the ISO 8601 week (01–52, 53 only in a 53-week year); 0605 = week 5 of 2006|letzte zwei Ziffern des Jahres, dann die ISO-8601-Kalenderwoche (01–52, 53 nur in Jahren mit 53 Wochen); 0605 = KW 5/2006","IEC 60062:2004 6.2.1 (GOST IDT); TI, Microchip, onsemi, Vishay Dale, ECIA print YYWW|IEC 60062:2004 6.2.1 (GOST IDT); TI, Microchip, onsemi, Vishay Dale, ECIA drucken JJWW"
wwyy,DDDD,"week first, then the last two digits of the year (shown only as the second reading)|erst die Woche, dann die letzten zwei Ziffern des Jahres (nur als zweite Lesart)","UNVERIFIED: no maker document read uses it|UNVERIFIED: kein gelesenes Herstellerdokument verwendet es"
year_month,LD,"year letter (20-year cycle) + month 1–9, O, N, D; J5 = May 2017|Jahresbuchstabe (20-Jahres-Zyklus) + Monat 1–9, O, N, D; J5 = Mai 2017","IEC 60062:2004 6.1.1 Tables 10a/10b (GOST IDT); TDK 2020 and Vishay 2024 agree|IEC 60062:2004 6.1.1, Tabellen 10a/10b (GOST IDT); TDK 2020 und Vishay 2024 stimmen überein"
```

Table B

```csv
letter,index,years
A,0,"1970, 1990, 2010, 2030"
B,1,"1971, 1991, 2011, 2031"
C,2,"1972, 1992, 2012, 2032"
D,3,"1973, 1993, 2013, 2033"
E,4,"1974, 1994, 2014, 2034"
F,5,"1975, 1995, 2015, 2035"
H,6,"1976, 1996, 2016, 2036"
J,7,"1977, 1997, 2017, 2037"
K,8,"1978, 1998, 2018, 2038"
L,9,"1979, 1999, 2019, 2039"
M,10,"1980, 2000, 2020, 2040"
N,11,"1981, 2001, 2021, 2041"
P,12,"1982, 2002, 2022, 2042"
R,13,"1983, 2003, 2023, 2043"
S,14,"1984, 2004, 2024, 2044"
T,15,"1985, 2005, 2025, 2045"
U,16,"1986, 2006, 2026, 2046"
V,17,"1987, 2007, 2027, 2047"
W,18,"1988, 2008, 2028, 2048"
X,19,"1989, 2009, 2029, 2049"
```

Table C

```csv
char,month
1,1
2,2
3,3
4,4
5,5
6,6
7,7
8,8
9,9
O,10
N,11
D,12
```

## Sources
1. IEC 60062:2016 (edition 6.0, 2016-07), *Marking codes for resistors and capacitors* — iTeh Standards document preview,
   https://cdn.standards.iteh.ai/samples/20940/4ae014b83a7c49adb888d27fd4b35b6b/IEC-60062-2016.pdf (downloaded to
   `codes/pdf/iec60062_2016_iteh.pdf`). Read: contents (printed page 3: Clause 8 "Date code system for capacitors and
   resistors", 8.1–8.4 and Tables 17–20 as listed above), Clause 1 Scope ("It also provides date code systems suitable
   for the marking of small components"), Clause 2 normative references (IEC 60063, IEC 60757, **ISO 8601**), foreword
   (sixth edition replaces the 2004 fifth edition; the list of technical changes does not mention the date codes).
   Clause 8 itself and Tables 17–20 are **not** in the preview. Also seen: the IEC 60062:2016+AMD1:2019 CSV (ed. 6.1)
   VDE preview https://www.vde-verlag.de/iec-normen/preview-pdf/info_iec60062%7Bed6.1%7Db.pdf — the same Clause 8
   structure and Tables 17–20 in its contents. Accessed 2026-11-27.
2. GOST IEC 60062-2014, *Коды для маркировки резисторов и конденсаторов* (interstate standard, Rosstandart order
   1729-st of 24 Nov 2014, in force 2015-10-01), foreword item 5: "identical to the international standard IEC
   60062:2004" (IDT). Full text read at
   https://meganorm.ru/mega_doc/norm/gost_gosudarstvennyj-standart/18/gost_IEC_60062-2014_mezhgosudarstvennyy_standart_kody_dlya.html
   (saved as `codes/pdf/gost_iec60062_2014_meganorm.html/.txt`): Clause 6 "Date code system for capacitors and
   resistors": 6.1 two-character code (year/month), 6.1.1 twenty-year cycle with Table 10a (year letters 1970–2009, note
   "these codes repeat after every 20-year cycle") and Table 10b (months 1–9, O, N, D; examples "March 1998 = K3;
   November 1999 = LN"); 6.1.2 ten-year cycle (Tables 11a/11b); 6.2 four-character code (year/week): 6.2.1 four digits
   (example "fifth week of 2006 = 0605"), 6.2.2 twenty-year cycle code (letter + "W" + week, "KW05" = week 5 of 1998,
   "week numbering shall be in accordance with ISO 8601"), 6.2.3 ten-year cycle code ("8W05"); 6.3 single-character
   code (Table 12). Clause 2 lists ISO 8601:2000. (The web text shows W for 1988/2008 in lower case, "w" — a
   transcription slip; every other source has W.) Accessed 2026-11-27.
3. TDK Electronics AG, *Film Capacitors — Marking and ordering code system*, dated July 2020, 7 pages. Page 1:
   "The coded forms specified in IEC 60062:2004 are used to indicate the rated capacitance, capacitance tolerance and
   date of manufacture." Page 6: "Codes for date of manufacture (to IEC 60062:2004)": year letters 2012 C, 2013 D,
   2014 E, 2015 F, 2016 H, 2017 J, 2018 K, 2019 L, 2020 M, 2021 N, 2022 P, 2023 R; months January 1 … September 9,
   October O, November N, December D; "E.g.: J5 2017 May". Read as the copy attached on mikrocontroller.net,
   https://mikrocontroller.net/attachment/553432/pdf-markingandorderingcodesystem.pdf (the TDK original on
   tdk-electronics.tdk.com was not located). Accessed 2026-11-27.
4. Vishay, *Body Marking Information — SMF (DO-219AB)*, Document Number 98534, Revision 26-Nov-2024, page 1: date code
   for rectifiers (excluding FRED), TVS and small-signal products: Month "1 to 9 = January to September, O = October,
   N = November, D = December"; Year "e.g. P … 2022, R … 2023, S … 2024, T … 2025, U … 2026, V … 2027, W … 2028,
   X … 2029, A … 2030", printed next to the year list as "According EN 600626" (read: EN 60062, the trailing 6 is
   unexplained — possibly clause 6 of the 2004 edition).
   https://www.vishay.com/docs/98534/bodymarkinginformation_smf_do-219ab.pdf — accessed 2026-11-27.
5. Electronic Components Industry Association (ECIA), *ECIA Guideline for Date Code Marking Format*, NIGP 117.00,
   January 2015, pages 2–3, clause 4 "Date Code Format": "The date code format shall be a four-digit number. The first
   two digits being the last two digits of the calendar year and the last two digits being the week of the year. This
   is considered the YYWW format. The first week of the year shall be considered the first week in which the first
   Thursday of the year falls. The first day of the week shall be considered Sunday. It is possible to have 53
   Thursdays in a year. If this happens, the week will be marked as week 53." Reference documents: EIA-476 Date Code
   Marking, MIL-STD-1285. The ecianow.org URL now answers 404; read as the Internet Archive copy of 2023-05-13,
   https://web.archive.org/web/20230513124009id_/https://www.ecianow.org/assets/docs/GIPC/NIGP%20117%20Guideline%20for%20Date%20Code%20Marking%20Format.pdf
   — accessed 2026-11-27.
6. Texas Instruments, FAQ "Does TI.com offer date code selection for Qualified Manufacturers List (QML) Class Q &
   Class V devices?": "According to U.S. military performance specification MIL-PRF-38535, the lot date code is
   formatted as a four-character year-year-week-week (YYWW) plus an optional suffix (YYWWAA) … for example, 2010A,
   2010B or 2010BA." https://www.ti.com/ordering-resources/faqs/high-reliability-products/qml-classq-classv.html —
   accessed 2026-11-27.
7. Texas Instruments (National Semiconductor), application report *Device Marking Conventions*, SNOA039C, May 2004,
   revised July 2005. Section 2 "Date of Manufacture Codes" (pages 4–5): four-digit (YYWW), three-digit (YWW),
   two-digit (YW) and one-character codes (Table 4); "The year code is the last one or two digits of the calendar
   year"; "The week code is based on the starting calendar week of the six-week period during which the device was
   assembled" (Table 5); Table 6 examples: calendar week 48, 1999 → 9948 / 948 / 98; week 6, 2000 → 0006; week 14,
   2000 → 0012; week 32, 2001 → 0130. Read as the copy on TI's E2E forum,
   https://e2e.ti.com/cfs-file/__key/communityserver-discussions-components-files/14/device_2D00_marking_2D00_conventions_2D00_rev_2D00_c.pdf
   (www.ti.com/lit/an/snoa039c/snoa039c.pdf answered 404). Accessed 2026-11-27.
8. Microchip Technology, *MCP6001/1R/1U/2/4 1 MHz, Low-Power Op Amp* data sheet, DS20001733L (Revision L, March 2020),
   page 19 "Package Marking Information": 8-lead PDIP third line "YYWW" (examples 0432, 0746), SOIC "XXXXYYWW", DFN
   and MSOP "YWW"; legend "Y Year code (last digit of calendar year)", "YY Year code (last 2 digits of calendar
   year)", "WW Week code (week of January 1 is week '01')", "NNN Alphanumeric traceability code". The same legend is
   on Microchip's online MCP9808 page (section 7.1). https://ww1.microchip.com/downloads/en/DeviceDoc/MCP6001-1R-1U-2-4-1-MHz-Low-Power-Op-Amp-DS20001733L.pdf
   — accessed 2026-11-27.
9. onsemi, *1N4001 – 1N4007 Axial Lead Standard Recovery Rectifiers*, publication order number 1N4001/D, June 2024 –
   Rev. 18. Page 1 marking diagram "1N400x / YYWWG" with "YY = Year", "WW = Work Week"; page 6 (case 59-10,
   98ASB42045B) generic marking "YYWW". https://www.onsemi.com/pdf/datasheet/1n4001-d.pdf — accessed 2026-11-27.
10. onsemi, *LM317, NCV317* data sheet, LM317/D, September 2026 – Rev. 18, page 10 marking diagrams "AWLYWWG":
    "A = Assembly Location, WL = Wafer Lot, Y = Year, WW = Work Week, G = Pb-Free Package".
    https://www.onsemi.com/pdf/datasheet/lm317-d.pdf — accessed 2026-11-27.
11. Vishay Dale, *Product Marking — Power Metal Strip® and Power Metal Plate™ Series*, Document Number 30327, Revision
    22-May-2023, page 1: "Date code printed based upon week and year of manufacture" (WSL, example "1119");
    "date code consists of location (M); year, week code (YYWW)" (WSC, example "M1843"); WSHM "line 3 is date code
    YYWW". https://www.vishay.com/docs/30327/pms-pmpmarking.pdf — accessed 2026-11-27.
12. Vishay BCcomponents film capacitor datasheets MKP 339 X2 (doc 28166, https://www.vishay.com/docs/28166/f339mx2.pdf),
    MKP 379 (doc 28135, https://www.vishay.com/docs/28135/mkp379.pdf), F1778 X2 (doc 27610,
    https://www.vishay.com/docs/27610/f1778x2.pdf): marking "manufacturer location, year and week" / "year and week of
    manufacture" (page 1). Shows that Vishay's film capacitors now use year/week, not the IEC letter code. Accessed
    2026-11-27.
13. ISO 8601-1:2019 (first edition 2019-02), *Date and time — Representations for information interchange — Part 1:
    Basic rules* — iTeh preview, https://cdn.standards.iteh.ai/samples/70907/5e8d00e639cf4f849462bd63062f4cd8/ISO-8601-1-2019.pdf.
    Read: term **3.1.1.23 week calendar** (printed page 5) with Notes 1–2 (quoted in Rules); contents list 4.2.2 "The
    week calendar", 4.3.4 "Calendar week number and weeks duration", 5.2.4 "Week date" — those clauses are not in the
    preview (so the Monday start and the 53-week rule were not read in ISO itself). Accessed 2026-11-27.
14. Markus Kuhn (University of Cambridge Computer Laboratory), *A summary of the international standard date and time
    notation*, https://www.cl.cam.ac.uk/~mgk25/iso-time.html (created 1995, last modified 2020-06-16): "A week starts
    with Monday (day 1) and ends with Sunday (day 7)"; week 01 is the first week that has the Thursday in this year,
    "equivalent to the week that contains the fourth day of January"; week numbers 01 to 53; example "the first week of
    the year 1997 lasts from 1996-12-30 to 1997-01-05". Accessed 2026-11-27.
15. Wikipedia, *ISO week date*, https://en.wikipedia.org/wiki/ISO_week_date (secondary): equivalent definitions of
    week 01 (first Thursday, contains 4 January, Monday nearest 1 January), long-year rule (starts on Thursday, or leap
    year starting on Wednesday), the 400-year table of long years, boundary examples (29 Dec 2025 = 2026-W01-1;
    2009-W53-7 = 3 Jan 2010). Accessed 2026-11-27.
16. Wikipedia, *RKM code*, section on date codes, https://en.wikipedia.org/wiki/RKM_code (secondary): twenty-year cycle
    letters with years 1970–2049, months 1–9, O, N, D, example "J8" = August 2017, footnote that G, I, O, Q, Y, Z are
    not used "to reduce the risk for read errors". Accessed 2026-11-27.
17. DigiKey TechForum, A. Dahlen, *Determining the Age of Electronic Equipment Using a Component's 4-digit Date Code*,
    9 Dec 2024, https://forum.digikey.com/t/determining-the-age-of-electronic-equipment-using-a-component-s-4-digit-date-code/47120
    (secondary): "read as a YYWW with 2-digit year and a 2-digit week"; examples 7638 = week 38 of 1976, 2333 = week 33
    of 2023 (Microchip AVR32DD28). Mentions no WWYY maker. Accessed 2026-11-27.

Not used / not opened: Vishay KP 460 datasheet (doc 28129, "IEC 60062 clause 6" year letters 2003–2009) — only a
garbled distributor text copy found, vishay.com answers 404; KEMET film datasheets (no date code table in the one read);
Analog Devices date-code pages and PCN marking comparisons (analog.com reset every connection, WebFetch timed out);
Nexperia (no marking document found); EIA-476 and MIL-STD-1285 themselves (not freely available); ISO 8601-1 clauses
4.2.2/4.3.4 (not in the preview).

## Cross-check
- **YYWW, year first**: IEC 60062:2004 6.2.1 (standard text), ECIA NIGP 117, TI (FAQ citing MIL-PRF-38535, and
  SNOA039C), Microchip DS20001733L, onsemi 1N4001/D and LM317/D, Vishay Dale 30327 — seven documents from five
  organisations agree. Agreed.
- **WWYY**: not found in any document read. Secondary web guides claim "some US manufacturers" use it without naming
  one; telcontar.net's list of maker formats (GoldStar, Holtek, Intel, Mitsubishi, TI, Hitachi, Newtech) has none.
  → UNVERIFIED.
- **Week 01 = week with the first Thursday**: ISO 8601-1:2019 3.1.1.23 (read), Kuhn, Wikipedia, ECIA NIGP 117. Agreed.
- **Week starts Monday**: Kuhn and Wikipedia (two independent references; the ISO clause itself was not readable).
  Agreed for ISO 8601. **ECIA says Sunday** — a different week, see Edge cases.
- **53-week years**: Wikipedia's rule and its table = the Python computation (2004, 2009, 2015, 2020, 2026, …). ECIA:
  week 53 when a year has 53 Thursdays — the same set of years. Agreed.
- **IEC 60062 year letters**: Table 10a (standard text, 1970–2009) continued by its 20-year repetition note gives
  2010 A … 2029 X. TDK 2020 (2012 C … 2023 R) and Vishay 2024 (2022 P … 2029 X, 2030 A) are two independent maker
  documents that print the current cycle; they overlap on P = 2022, R = 2023 and agree with each other and with the
  standard. Wikipedia agrees letter by letter. **Confirmed.** Only B (2011/2031) and A = 2010 are not printed by a maker
  document read; they follow from the standard's cycle (and Vishay's A = 2030).
- **Month characters 1–9, O, N, D**: Table 10b (standard text), TDK, Vishay, Wikipedia. Confirmed.
- **Letters not used (G, I, O, Q, Y, Z)**: absent from Table 10a and from both maker lists; the reason is stated only by
  Wikipedia. Confirmed as "absent"; the reason is secondary.
- **2016 edition**: the clause and table titles exist (Clause 8, Tables 17 and 18); the table contents of the 2016
  edition were not read. Vishay's 2024 document ("According EN 60062") and TDK's 2020 document (citing the 2004
  edition) agree, so nothing indicates a change.

### Not cross-checked (only one source)
- Microchip's "week of January 1 is week '01'" (one maker; numbering differs from ISO, see Edge cases).
- TI/National's six-week-period week code (one document, 2005).
- The ten-year cycle code (IEC 60062:2004 6.1.2, Tables 11a/11b) and the single-character code (Table 12): read only in
  GOST; not shipped. Conflict noted: GOST Table 11b prints October as "0" (digit) for the ten-year cycle, Wikipedia
  gives X, Y, Z for October–December in the ten-year cycle, and the 2016 contents title Table 19 "Code letters for the
  year in a ten-year cycle" although the 2004 table uses digits. Not resolved; ECalc does not ship the ten-year code.

## Notes (en / de)
- en: Most makers print the date code as YYWW: 2448 = week 48 of 2024. de: Die meisten Hersteller drucken den
  Datumscode als JJWW: 2448 = Kalenderwoche 48/2024.
- en: Weeks follow ISO 8601 (IEC 60062): Monday to Sunday; week 01 is the week with the first Thursday of the year, so it
  can start in December of the previous year. de: Die Wochen zählen nach ISO 8601 (IEC 60062): Montag bis Sonntag;
  KW 01 ist die Woche mit dem ersten Donnerstag des Jahres und kann deshalb schon im Dezember des Vorjahres beginnen.
- en: Week 53 exists only in some years (2020, 2026, 2032 …); in other years it is not a valid code. de: Eine KW 53
  gibt es nur in manchen Jahren (2020, 2026, 2032 …); in allen anderen Jahren ist sie kein gültiger Code.
- en: Makers count weeks differently: ECIA (US) starts the week on Sunday, Microchip counts the week of 1 January as
  week 01, TI marks the first week of a six-week period. Dates can be off by a few days. de: Hersteller zählen die Wochen
  unterschiedlich: ECIA (USA) beginnt die Woche am Sonntag, Microchip zählt die Woche mit dem 1. Januar als KW 01, TI
  markiert die erste Woche eines Sechs-Wochen-Zeitraums. Die Daten können deshalb um einige Tage abweichen.
- en: A two-digit year is read as 20xx; old parts may be from 19xx. de: Eine zweistellige Jahreszahl wird als 20xx
  gelesen; ältere Bauteile können aus 19xx stammen.
- en: The IEC 60062 letter code repeats every 20 years: J5 is May 2017, but also May 1997. de: Der Buchstabencode nach
  IEC 60062 wiederholt sich alle 20 Jahre: J5 ist Mai 2017, aber auch Mai 1997.
- en: Not every four-digit number on a part is a date code (e.g. "7805" is a part number). de: Nicht jede vierstellige
  Zahl auf einem Bauteil ist ein Datumscode (z. B. ist „7805" eine Typenbezeichnung).

## Decode / encode rules
1. Normalise: trim, uppercase, remove spaces. A leading location letter (Vishay "M1843") or a trailing letter suffix
   (TI "2010A", onsemi "G") is not part of the date; the decoder takes exactly four digits.
2. Four digits `abcd`: reading YYWW = year 2000 + ab, week cd; reading WWYY = week ab, year 2000 + cd.
3. A reading is valid when 1 ≤ week ≤ weeks(year), where weeks(year) = 53 if 1 January is a Thursday, or the year is a
   leap year and 1 January is a Wednesday, else 52 (Python: `date(year, 12, 28).isocalendar()[1]`).
4. For a valid reading: Monday = `date.fromisocalendar(year, week, 1)`, Sunday = `date.fromisocalendar(year, week, 7)`.
5. Show YYWW first (the documented form); show WWYY only as a second reading, labelled "unusual order (no maker
   document found)"; if neither is valid, refuse with the reason (week 00, week above 53, week 53 in a 52-week year).
6. Note on every result: the year may also be 19xx (1900 + ab), and makers' week counting can differ by a few days.
7. Encoder (date → YYWW): take the ISO week-year y and week w of the date (`date.isocalendar()`), print `y mod 100`
   and `w` as two digits each. Use the ISO week-year, not the calendar year (1 January 2021 → 2053).
8. Two characters `Lm` (IEC 60062 twenty-year cycle): L must be one of ABCDEFHJKLMNPRSTUVWX (index i), m one of
   1–9, O, N, D. Years = 1970 + i + 20·k. Show the most recent such year that is not after the current year, then
   the one 20 years earlier. Reject G, I, O, Q, Y, Z as year letters, and 0 (zero), A–C, E–M, P–Z as month characters.
9. Encoder (year, month → Lm): letter = "ABCDEFHJKLMNPRSTUVWX"[(year − 1970) mod 20], month character from Table C.

## Edge cases
- **Week 01 starting in December**: 2026-W01 = Mon 29 Dec 2025 … Sun 4 Jan 2026; 2025-W01 starts 30 Dec 2024;
  2020-W01 starts 30 Dec 2019; 2004-W01 starts 29 Dec 2003. **Not 2021**: 2021-W01 = Mon 4 Jan … Sun 10 Jan 2021;
  1–3 January 2021 belong to 2020-W53 (contradicts the example in the task brief).
- **Week 53** valid in 2004, 2009, 2015, 2020, 2026 (…); refused in 2021, 2023, 2024, 2025 (…).
- **ECIA (Sunday-start) week**: same week number as ISO (both anchored on the same Thursday), but the week runs Sunday
  to Saturday, i.e. one day earlier: 2026 week 01 = Sun 28 Dec 2025 … Sat 3 Jan 2026 (ISO: Mon 29 Dec … Sun 4 Jan).
- **Microchip "week of January 1 is week 01"**: equals ISO when 1 January is Monday–Thursday; differs when it is
  Friday, Saturday or Sunday (2021, 2022, 2023, 2027, 2028): ISO puts 1 January 2021 in 2020-W53, Microchip calls the
  week of 1 January 2021 week 01. Microchip does not say which weekday its week starts on.
- **TI/National commercial parts**: WW is the first week of a six-week period: 0012 means weeks 12–17 of 2000 (Table 6:
  calendar week 14, 2000 → 0012).
- **IEC letter code ambiguity**: N and D are year letters and month characters; the position decides ("ND" = December
  2021, "DN" = November 2013 or 2033). "O" is a month (October), never a year letter.
- **Month "0"**: the twenty-year table uses the letter O for October; a digit 0 is refused (GOST's ten-year Table 11b
  prints "0" — not shipped, see Cross-check).

## Worked examples (test vectors)
Dates computed with `python -I` (`datetime.date.fromisocalendar`, `date.isocalendar`); weeks(year) from 28 December.

Four-digit decode:

| code | YYWW reading | WWYY reading | expected output | ref |
|---|---|---|---|---|
| 2448 | 2024 W48: 2024 has 52 weeks, 48 ≤ 52 → Mon 2024-11-25 … Sun 2024-12-01 | 2048 W24: 2048 has 53 weeks → Mon 2048-06-08 … Sun 2048-06-14 | both (normal case) | computed |
| 2601 | 2026 W01: 1 Jan 2026 is Thursday → W01 = Mon 2025-12-29 … Sun 2026-01-04 | 2001 W26 → Mon 2001-06-25 … Sun 2001-07-01 | both; YYWW week starts in the previous December | Wikipedia (29 Dec 2025 = 2026-W01-1) |
| 2101 | 2021 W01: 1 Jan 2021 is Friday → W01 = Mon 2021-01-04 … Sun 2021-01-10 | 2001 W21 → Mon 2001-05-21 … Sun 2001-05-27 | both; 2021-W01 does **not** start in December | computed |
| 2001 | 2020 W01: 1 Jan 2020 is Wednesday → Mon 2019-12-30 … Sun 2020-01-05 | 2001 W20 → Mon 2001-05-14 … Sun 2001-05-20 | both; YYWW week starts in December | computed |
| 2053 | 2020 W53: 2020 is a leap year starting Wednesday → 53 weeks → Mon 2020-12-28 … Sun 2021-01-03 | 2053 W20 → Mon 2053-05-12 … Sun 2053-05-18 | both | computed |
| 2653 | 2026 W53: 1 Jan 2026 Thursday → 53 weeks → Mon 2026-12-28 … Sun 2027-01-03 | 2053 W26 → Mon 2053-06-23 … Sun 2053-06-29 | both | computed |
| 2353 | 2023 W53: 1 Jan 2023 is Sunday, not a leap year → 52 weeks → **invalid** | 2053 W23 → Mon 2053-06-02 … Sun 2053-06-08 | WWYY only (valid in one reading) | computed |
| 5301 | 2053 W01 → Mon 2052-12-30 … Sun 2053-01-05 | 2001 W53: 1 Jan 2001 Monday → 52 weeks → **invalid** | YYWW only | computed |
| 9948 | 2099 W48 → Mon 2099-11-23 … Sun 2099-11-29 (note: as 1999 W48 → Mon 1999-11-29 … Sun 1999-12-05) | week 99 → **invalid** | YYWW only; 1900s note matters | TI SNOA039C Table 6 ("Calendar week 48, 1999 = 9948") |
| 0453 | 2004 W53: leap year starting Thursday → Mon 2004-12-27 … Sun 2005-01-02 | 2053 W04 → Mon 2053-01-20 … Sun 2053-01-26 | both | Wikipedia (2004-W53-7 = 2 Jan 2005) |
| 1553 | 2015 W53 → Mon 2015-12-28 … Sun 2016-01-03 | 2053 W15 → Mon 2053-04-07 … Sun 2053-04-13 | both | computed |
| 2400 | week 00 → **invalid** | 2000 W24 → Mon 2000-06-12 … Sun 2000-06-18 | WWYY only | computed |
| 0000 | week 00 → invalid | week 00 → invalid | refused: "week 00 does not exist" | computed |
| 2654 | week 54 → **invalid** | 2054 W26 → Mon 2054-06-22 … Sun 2054-06-28 | WWYY only | computed |
| 5454 | week 54 → invalid | week 54 → invalid | refused: "week above 53" | computed |
| 0605 | 2006 W05 → Mon 2006-01-30 … Sun 2006-02-05 | 2005 W06 → Mon 2005-02-07 … Sun 2005-02-13 | both; YYWW = the standard's example | IEC 60062:2004 6.2.1 ("fifth week of 2006 = 0605") |
| 0746 | 2007 W46 → Mon 2007-11-12 … Sun 2007-11-18 | 2046 W07 → Mon 2046-02-12 … Sun 2046-02-18 | both | Microchip DS20001733L p. 19 (example "0746") |
| 1843 | 2018 W43 → Mon 2018-10-22 … Sun 2018-10-28 | 2043 W18 → Mon 2043-04-27 … Sun 2043-05-03 | both | Vishay 30327 ("M1843") |
| 2010 | 2020 W10 → Mon 2020-03-02 … Sun 2020-03-08 | 2010 W20 → Mon 2010-05-17 … Sun 2010-05-23 | both | TI QML FAQ ("2010A") |
| 2615 | 2026 W15 → Mon 2026-04-06 … Sun 2026-04-12 | 2015 W26 → Mon 2015-06-22 … Sun 2015-06-28 | both (valid in both readings) | computed |

Four-digit encode (date → ISO week-year, week → YYWW):

| date | ISO week date | code |
|---|---|---|
| 2026-11-27 (Fri) | 2026-W48-5 | 2648 |
| 2026-01-01 (Thu) | 2026-W01-4 | 2601 |
| 2025-12-29 (Mon) | 2026-W01-1 | 2601 (week-year 2026, calendar year 2025) |
| 2021-01-01 (Fri) | 2020-W53-5 | 2053 |
| 2021-01-04 (Mon) | 2021-W01-1 | 2101 |
| 2027-01-01 (Fri) | 2026-W53-5 | 2653 |
| 2024-12-30 (Mon) | 2025-W01-1 | 2501 |
| 2023-01-01 (Sun) | 2022-W52-7 | 2252 |
| 2010-01-03 (Sun) | 2009-W53-7 | 0953 |
| 2006-02-01 (Wed) | 2006-W05-3 | 0605 (IEC example) |

IEC 60062 year/month decode and encode:

| code | expected | ref |
|---|---|---|
| J5 | May 2017 (or 1997) | TDK p. 6 ("J5 2017 May") |
| K3 | March 2018 (or 1998) | IEC 60062:2004 Table 10b example ("March 1998 = K3") |
| LN | November 2019 (or 1999) | IEC 60062:2004 Table 10b example ("November 1999 = LN") |
| PO | October 2022 (or 2002) | TDK (P = 2022), Vishay (P = 2022), O = October |
| UN | November 2026 (or 2006) | Vishay (U = 2026) |
| XD | December 2009 (2029 is later than today, 2026-11-27) | Table 10a (X = 2009), Vishay (X = 2029) |
| A1 | January 2010 (or 1990; A = 2030 per Vishay is still in the future) | Table 10a (A = 1990), cycle |
| ND | December 2021 (or 2001) | TDK (N = 2021); D = December |
| G5 | refused: G is not a year letter | Table 10a |
| Q1 | refused: Q is not a year letter | Table 10a |
| O3 | refused: O is not a year letter (it is the October character) | Table 10a/10b |
| S0 | refused: 0 (zero) is not a month character (October is the letter O) | Table 10b |
| SA | refused: A is not a month character | Table 10b |
| encode 2026-11 | UN | Vishay U = 2026; N = November |
| encode 2025-10 | TO | Vishay T = 2025; O = October |
| encode 2030-01 | A1 | Vishay A = 2030 |
| encode 2017-05 | J5 | TDK example |

## What the engine ships (catalog/codes/date_code.yaml, codes_date.go, ecalc-codes.js)
- Decode, four digits: YYWW only, year 2000 + YY, week 01 to 52 (53 only in a 53-week year; 00, 54 to 99 and a week 53 of a
  52-week year are `invalid_code` with the reason). Fields: `year`, `week`, `week_start` (Monday), `week_end` (Sunday), dates
  as YYYY-MM-DD. The WWYY reading of the research is **not shipped** (no maker document prints it); the 19xx reading is a note.
- Decode, two characters: year letter of A B C D E F H J K L M N P R S T U V W X (lower case accepted) and a month
  character 1 to 9, O, N, D. G, I, O, Q, Y, Z as year letter and anything else as month character are `invalid_code`. The
  decoder has no "today": `years` lists every year 1970 to 2049 of the letter (`J5` gives 1977, 1997, 2017, 2037) and
  `month` the English month name (the page translates it).
- Encode: `value` is a date YYYY-MM-DD (from 1583, a real calendar date). Result `code` is the YYWW of the ISO week-year (2021-01-01
  gives 2053), the fields are the week of the date and `letter_code` (year letter of the calendar year, month character).
  A date whose ISO week-year is outside 2000 to 2099 has no YYWW code: `code` is then the letter code and a warning says so.
  Dates are calendar days in UTC; no time zone, no clock.
- No number: `value` and `dim` stay empty, every field is text.
- Not shipped: the ten-year letter cycle (sources conflict), the single-character code, the week counts of ECIA, Microchip
  and TI (a note says they can be off by one).
- Vectors: 25 decode, 16 encode and 26 error cases in `testdata/vectors/codes.json`, computed independently with
  `datetime.date.isocalendar` / `fromisocalendar` (`python -I`).

UNVERIFIED: (1) **WWYY** — no maker document read prints it; the brief's assumption that some makers do is not backed
by a source. Ship it only as a labelled second reading, or drop it. (2) The 2016 edition's Clause 8 text and Tables
17–20 were not read (only their titles); the letter table is the 2004 edition's (GOST IDT), confirmed for 2012–2030 by
TDK and Vishay. (3) The ISO 8601-1 clause stating the Monday start and the 53-week rule was not readable; both come
from Kuhn and Wikipedia (and the Python standard library, which implements ISO weeks). (4) Microchip's and TI's week
counting rules are single-maker statements. (5) Analog Devices, Nexperia and Vishay-semiconductor YYWW documents were
not read (blocked or not found).
