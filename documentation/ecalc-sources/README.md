# ECalc sources

The research behind the ECalc reference tables, component codes and pinouts: what each item contains, where the values come
from and how they were cross-checked. The catalog itself (`internal/ecalc/catalog/`) only carries a short `source` citation per
item; the URLs, the comparison between sources and the reasons for each choice are here.

These files are a record for maintainers. The app does not read them, and they are not a substitute for the standards.

## Layout

| Folder | Catalog file | Content |
| --- | --- | --- |
| `conductors/` | `catalog/tables/conductors.yaml`, `catalog/calculators/finders.yaml` | Cable ampacity, correction factors, flexible cords, NEC, IEC 60228, IPC-2221 spacing, copper weight; the specs of the two finders |
| `components/` | `catalog/tables/components.yaml` | SMD sizes, MLCC dielectrics, fuses, batteries, DC barrel connectors |
| `materials/` | `catalog/tables/materials.yaml` | Resistivity, dielectric constants, thermocouples, solder alloys |
| `general/` | `catalog/tables/general.yaml` | SI, constants, Greek alphabet, IP codes, mains by country, frequency bands, logic levels, ASCII, wire colours |
| `codes/` | `catalog/codes/*.yaml` | Resistor and inductor colour bands, SMD resistor, capacitor and ceramic temperature codes, with decode rules and worked examples |
| `pinouts/` | `catalog/pinouts/*.yaml` | One file per connector, board or IC, in the folders of its list section: which view the drawing is written for and on what that rests, sources, cross-check, choices |

Each folder has a `SUMMARY.md` with the open problems of the group. Each table file has its columns, rows, sources,
cross-check and notes.

## Reading a file

- **Kind** says how far a value can be trusted: `definitional`, `transcribed standard`, `formula-derived` or
  `typical values`.
- **UNVERIFIED** marks a value that could not be confirmed. Transcribed standard values need two independent sources.
  Definitional data and binding legal text need one authoritative source. A search excerpt does not count as read.
- Sources were accessed on 2026-11-02 (pinouts: 2026-11-04) and have not been rechecked since. Most standards themselves are paywalled, so the
  values usually come from maker documents and published reproductions, as each file says.

## How the catalog uses this data

- Unverified cells are empty in the catalog, and the table's notes say so. A column without any value is hidden on the page.
- The Python helper scripts and the downloaded PDFs named as evidence are not in the repository: the PDFs are the makers'
  and publishers' copyrighted documents, and each source is cited so it can be fetched again.

The catalog deliberately differs from these files in a few places:

- **Aluminium ampacity:** only cells with two independent sources have values. The other columns exist but stay empty.
- **DIN versus IEC:** the ampacity tables carry the IEC values. A note names the DIN VDE 0298-4:2013 deviation.
- **Constants:** only CODATA 2022 ships, not the 2018 set the research also lists.
- **IP codes:** the `test_hint` column has one source and is left out.
- **EU SRD limits:** they ship, because Implementing Decision (EU) 2022/180 is the binding text itself.
- **CuSn6 α and the alumina upper ends:** left empty, because they were seen only as search excerpts.
- **Codes:** EIA-96 letters S and H are refused as ambiguous, `10R` reads as RKM 10 Ω, and single-source letters are refused
  as not cross-checked.
- **IPC-2221 Table 6-1:** the per-volt row has a text key and an empty `v_min`, so a numeric lookup cannot return it as a
  spacing.

## Changing a value

Change the catalog and this record together: update the table file (source, cross-check, date), the YAML and the vector file
in `internal/ecalc/testdata/vectors/tables/`, `testdata/vectors/pinouts/` or `testdata/vectors/codes.json`, whose `ref` names the source.
