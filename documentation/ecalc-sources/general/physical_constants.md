# physical_constants — Physical constants for electronics / Physikalische Konstanten für die Elektronik

Kind: definitional (exact constants) + transcribed standard values (CODATA adjustments)
Guide-value note needed: no

## Columns
| id | meaning (en) | Bedeutung (de) | unit as in source | numeric? |
|---|---|---|---|---|
| id | short id for lookups | Kurzname | – | no |
| name | name "en\|de" (en = NIST wording) | Name "en\|de" | – | no |
| symbol | usual symbol | übliches Formelzeichen | – | no |
| exact | `yes` = exact by SI definition (2019), `derived` = exact because computed from exact constants (NIST prints it truncated with "..."), `no` = measured | `yes` = per Definition exakt, `derived` = aus exakten Konstanten berechnet (gekürzt), `no` = gemessen | – | no |
| value_2022 | CODATA 2022 recommended value (current NIST listing) | CODATA-2022-Wert (aktuell bei NIST) | see `unit` | yes |
| unc_2022 | CODATA 2022 standard uncertainty (empty = exact) | Standardunsicherheit CODATA 2022 (leer = exakt) | see `unit` | yes |
| value_2018 | CODATA 2018 recommended value | CODATA-2018-Wert | see `unit` | yes |
| unc_2018 | CODATA 2018 standard uncertainty (empty = exact) | Standardunsicherheit CODATA 2018 | see `unit` | yes |
| unit | unit exactly as NIST writes it (ASCII: `^-1` = ⁻¹) | Einheit wie bei NIST | – | no |

## Rows
```csv
id,name,symbol,exact,value_2022,unc_2022,value_2018,unc_2018,unit
c,speed of light in vacuum|Lichtgeschwindigkeit im Vakuum,c,yes,299792458,,299792458,,m s^-1
h,Planck constant|Planck-Konstante (Plancksches Wirkungsquantum),h,yes,6.62607015e-34,,6.62607015e-34,,J Hz^-1
hbar,reduced Planck constant|reduzierte Planck-Konstante,ħ,derived,1.054571817e-34,,1.054571817e-34,,J s
e,elementary charge|Elementarladung,e,yes,1.602176634e-19,,1.602176634e-19,,C
k,Boltzmann constant|Boltzmann-Konstante,k,yes,1.380649e-23,,1.380649e-23,,J K^-1
k_ev,Boltzmann constant in eV/K|Boltzmann-Konstante in eV/K,k/e,derived,8.617333262e-5,,8.617333262e-5,,eV K^-1
na,Avogadro constant|Avogadro-Konstante,NA,yes,6.02214076e23,,6.02214076e23,,mol^-1
dnu_cs,hyperfine transition frequency of Cs-133|Hyperfeinstruktur-Übergangsfrequenz von Cs-133,ΔνCs,yes,9192631770,,9192631770,,Hz
kcd,luminous efficacy (540 THz)|Photometrisches Strahlungsäquivalent (540 THz),Kcd,yes,683,,683,,lm W^-1
mu0,vacuum magnetic permeability|magnetische Feldkonstante (Vakuumpermeabilität),µ0,no,1.25663706127e-6,0.00000000020e-6,1.25663706212e-6,0.00000000019e-6,N A^-2
eps0,vacuum electric permittivity|elektrische Feldkonstante (Vakuumpermittivität),ε0,no,8.8541878188e-12,0.0000000014e-12,8.8541878128e-12,0.0000000013e-12,F m^-1
z0,characteristic impedance of vacuum|Wellenwiderstand des Vakuums (Feldwellenwiderstand),Z0,no,376.730313412,0.000000059,376.730313668,0.000000057,ohm
me,electron mass|Elektronenmasse (Ruhemasse),me,no,9.1093837139e-31,0.0000000028e-31,9.1093837015e-31,0.0000000028e-31,kg
me_mev,electron mass energy equivalent in MeV|Ruheenergie des Elektrons in MeV,me c²,no,0.51099895069,0.00000000016,0.51099895000,0.00000000015,MeV
mp,proton mass|Protonenmasse,mp,no,1.67262192595e-27,0.00000000052e-27,1.67262192369e-27,0.00000000051e-27,kg
mn,neutron mass|Neutronenmasse,mn,no,1.67492750056e-27,0.00000000085e-27,1.67492749804e-27,0.00000000095e-27,kg
e_me,electron charge to mass quotient|spezifische Ladung des Elektrons,-e/me,no,-1.75882000838e11,0.00000000055e11,-1.75882001076e11,0.00000000053e11,C kg^-1
ev,electron volt|Elektronvolt,eV,yes,1.602176634e-19,,1.602176634e-19,,J
faraday,Faraday constant|Faraday-Konstante,F,derived,96485.33212,,96485.33212,,C mol^-1
r_gas,molar gas constant|universelle (molare) Gaskonstante,R,derived,8.314462618,,8.314462618,,J mol^-1 K^-1
sigma_sb,Stefan-Boltzmann constant|Stefan-Boltzmann-Konstante,σ,derived,5.670374419e-8,,5.670374419e-8,,W m^-2 K^-4
phi0,magnetic flux quantum|magnetisches Flussquantum,Φ0,derived,2.067833848e-15,,2.067833848e-15,,Wb
kj,Josephson constant|Josephson-Konstante,KJ,derived,483597.8484e9,,483597.8484e9,,Hz V^-1
rk,von Klitzing constant|von-Klitzing-Konstante,RK,derived,25812.80745,,25812.80745,,ohm
g0,conductance quantum|Leitwertquantum,G0,derived,7.748091729e-5,,7.748091729e-5,,S
alpha,fine-structure constant|Feinstrukturkonstante,α,no,7.2973525643e-3,0.0000000011e-3,7.2973525693e-3,0.0000000011e-3,1
bohr_mag,Bohr magneton|Bohrsches Magneton,µB,no,9.2740100657e-24,0.0000000029e-24,9.2740100783e-24,0.0000000028e-24,J T^-1
g_n,standard acceleration of gravity|Normfallbeschleunigung,gn,yes,9.80665,,9.80665,,m s^-2
atm,standard atmosphere|Normatmosphäre (Normdruck),atm,yes,101325,,101325,,Pa
grav,Newtonian constant of gravitation|Gravitationskonstante,G,no,6.67430e-11,0.00015e-11,6.67430e-11,0.00015e-11,m^3 kg^-1 s^-2
```

## Sources
1. NIST, "Fundamental Physical Constants — Complete Listing, 2022 CODATA adjustment", https://physics.nist.gov/cuu/Constants/Table/allascii.txt — all `value_2022`/`unc_2022`; accessed 2026-11-02.
2. NIST, "Fundamental Physical Constants — Complete Listing, 2018 CODATA adjustment", https://physics.nist.gov/cuu/Constants/ArchiveASCII/allascii_2018.txt — all `value_2018`/`unc_2018`; accessed 2026-11-02.
3. BIPM SI Brochure, 9th ed. 2019 (V4.01, June 2026), Table 1 "The seven defining constants of the SI", https://www.bipm.org/documents/20126/41483022/SI-Brochure-9-EN.pdf — confirms the exact values of ΔνCs, c, h, e, k, NA, Kcd and that µ0 is no longer exactly 4π×10⁻⁷ H/m; accessed 2026-11-02.

## Cross-check
- Definitional/standard data from the authoritative publisher (NIST CODATA). The exact constants agree between NIST (sources 1, 2) and the BIPM brochure (source 3).
- Internal consistency checked with a helper script that is not part of the repository: for both adjustments ε0 = 1/(µ0 c²) and Z0 = µ0 c reproduce the listed values within 2·10⁻¹⁰ relative; ħ = h/2π, F = NA·e, R = NA·k, k/e, Φ0 = h/2e, RK = h/e², KJ = 2e/h and G0 = 2e²/h reproduce the NIST "derived exact" values (all checks pass). µ0/(4π·10⁻⁷ N/A²) − 1 is of order 10⁻¹⁰ for both adjustments.
- **Edition decision:** the brief asked for CODATA 2018. NIST's current listing is the **2022** adjustment (2018 is archived). Both are given; I recommend that ECalc shows 2022 as the current value and keeps 2018 only if a reference to the older value is wanted. Values differ only in the measured constants (µ0, ε0, Z0, masses, α, µB).
- Exact "derived" values are printed by NIST truncated with "..." (e.g. ħ = 1.054 571 817... × 10⁻³⁴); the table keeps NIST's printed digits. A calculator should compute them from h, e, k, NA instead of using the truncated digits.

## Notes (en / de)
- en: Since the 2019 SI revision h, e, k and NA are exact; µ0 and ε0 are measured quantities (µ0 ≈ 4π × 10⁻⁷ N/A² to about 1 part in 10¹⁰).
- de: Seit der SI-Reform 2019 sind h, e, k und NA exakt; µ0 und ε0 sind Messgrößen (µ0 ≈ 4π × 10⁻⁷ N/A² auf etwa 10⁻¹⁰ genau).
- en: The thermal voltage kT/e is 25.852 mV at 300 K and 25.693 mV at 25 °C (computed from exact constants).
- de: Die Temperaturspannung kT/e beträgt 25,852 mV bei 300 K und 25,693 mV bei 25 °C (aus exakten Konstanten berechnet).
- en: Uncertainties are standard uncertainties (1σ) in the same unit as the value.
- de: Unsicherheiten sind Standardunsicherheiten (1σ) in derselben Einheit wie der Wert.

## Suggested finder / lookups
- `id` → value (2022) for formulas (e.g. thermal voltage, skin depth with µ0, free-space wavelength with c, Z0 for antenna formulas).
