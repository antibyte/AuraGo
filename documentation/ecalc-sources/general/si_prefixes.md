# si_prefixes — SI prefixes / SI-Vorsätze

Kind: definitional
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| name | prefix name (English, as in the SI Brochure) | Name des Vorsatzes (englisch, wie in der SI-Broschüre) | – | no |
| name_de | prefix name in German (identical spelling except deca = Deka) | Name des Vorsatzes auf Deutsch | – | no |
| symbol | prefix symbol (case-sensitive; µ is U+00B5 MICRO SIGN in the brochure text; U+03BC is also common) | Vorsatzzeichen (Groß-/Kleinschreibung beachten) | – | no |
| exponent | power of ten n in 10^n | Zehnerpotenz n in 10^n | 1 | yes |
| factor | multiplying factor 10^n written as a number | Faktor 10^n als Zahl | 1 | yes |
| adopted | year the prefix was adopted by the CGPM (only given for the four 2022 prefixes, from the source) | Jahr der Aufnahme durch die CGPM (nur für die vier Vorsätze von 2022 belegt) | year | yes |

## Rows
```csv
name,name_de,symbol,exponent,factor,adopted
quetta,Quetta,Q,30,1e30,2022
ronna,Ronna,R,27,1e27,2022
yotta,Yotta,Y,24,1e24,
zetta,Zetta,Z,21,1e21,
exa,Exa,E,18,1e18,
peta,Peta,P,15,1e15,
tera,Tera,T,12,1e12,
giga,Giga,G,9,1e9,
mega,Mega,M,6,1e6,
kilo,Kilo,k,3,1e3,
hecto,Hekto,h,2,1e2,
deca,Deka,da,1,1e1,
deci,Dezi,d,-1,1e-1,
centi,Zenti,c,-2,1e-2,
milli,Milli,m,-3,1e-3,
micro,Mikro,µ,-6,1e-6,
nano,Nano,n,-9,1e-9,
pico,Piko,p,-12,1e-12,
femto,Femto,f,-15,1e-15,
atto,Atto,a,-18,1e-18,
zepto,Zepto,z,-21,1e-21,
yocto,Yokto,y,-24,1e-24,
ronto,Ronto,r,-27,1e-27,2022
quecto,Quekto,q,-30,1e-30,2022
```

## Sources
1. BIPM, *The International System of Units (SI)*, 9th edition 2019, document version V4.01 (June 2026), Table 7 "SI prefixes", https://www.bipm.org/documents/20126/41483022/SI-Brochure-9-EN.pdf (DOI 10.59161/AUEZ1291) — all 24 prefixes, names, symbols, factors; accessed 2026-11-02.
2. BIPM web page "SI prefixes", https://www.bipm.org/en/measurement-units/si-prefixes — same 24 prefixes (consistency check), accessed 2026-11-02.
3. BIPM, Resolution 3 of the 27th CGPM (2022) "On the extension of the range of SI prefixes", https://www.bipm.org/en/committees/cg/cgpm/27-2022/resolution-3 — adds ronna (R, 10^27), ronto (r, 10^-27), quetta (Q, 10^30), quecto (q, 10^-30), accessed 2026-11-02.

## Cross-check
Definitional; sources 1 and 2 list identical names, symbols and factors. The adoption year 2022 for the four new prefixes is from source 3 (and the brochure index line "27th CGPM, 2022: decides to add ronna, ronto, quetta, and quecto").
German names (`name_de`) are the usual German spellings (Hekto, Deka, Dezi, Zenti, Piko, Yokto, Quekto); they are not taken from a German standard I could read — if the implementer wants a cited German source, DIN 1301-1 is the reference (not accessed).

## Notes (en / de)
- en: Prefix symbols are case-sensitive (M = mega, m = milli). Prefixes are never combined (write nm, not mµm). The kilogram is the only base unit with a prefix in its name; prefixes are applied to the gram (mg, not µkg).
- de: Vorsatzzeichen unterscheiden Groß- und Kleinschreibung (M = Mega, m = Milli). Vorsätze werden nicht kombiniert (nm statt mµm). Beim Kilogramm wird der Vorsatz auf das Gramm angewendet (mg, nicht µkg).
- en: ECalc input accepts `u` for µ and `K` for k (see internal/ecalc/catalog/units.yaml); this table shows the official symbols only.
- de: ECalc akzeptiert bei der Eingabe `u` für µ und `K` für k; diese Tabelle zeigt nur die amtlichen Zeichen.

## Suggested finder / lookups
- Symbol → factor (e.g. "n" → 1e-9).
- Engineering notation: for a value x pick the prefix with exponent = 3·floor(log10(|x|)/3) (only the multiples of 3 are "engineering" prefixes; hecto/deca/deci/centi are excluded there).
