# office_workbook

Use `office_workbook` for agent-safe spreadsheet work inside AuraGo's virtual desktop workspace. It reads and writes `.xlsx`, `.xlsm`, and `.csv` through AuraGo's Go Office backend; XLSX persistence uses Excelize behind a structured workbook JSON model.

The tool requires `tools.office_workbook.enabled`, `virtual_desktop.enabled`, and `virtual_desktop.allow_agent_control`. Paths are jailed to `virtual_desktop.workspace_dir`; do not expose secrets in spreadsheet contents unless the user explicitly asked for that data.

## Operations

- `read`: use `path`; returns `entry`, `workbook`, and `office_version`.
- `write`: use `path` and `workbook` shaped as `{sheets:[{name, rows:[[ {value, formula} ]]}]}`.
- `set_cell`: use `path`, `sheet`, `cell`, and either `value` or `formula`.
- `set_range`: use `path`, `sheet`, `start_cell`, and `values` as a 2D array of strings or `{value, formula}` cells.
- `evaluate_formula`: use `path`, `sheet`, and `formula`; supports AuraGo's safe formula subset (`SUM`, `AVG`/`AVERAGE`, `MIN`, `MAX`, `COUNT`, arithmetic, and same-sheet ranges).
- `export`: use `path`, `output_path`, and `format` (`xlsx` or `csv`); pass `sheet` for CSV export when needed.

## Examples

```json
{
  "operation": "set_range",
  "path": "Documents/budget.xlsx",
  "sheet": "Budget",
  "start_cell": "A1",
  "values": [
    ["Item", "Amount"],
    ["Coffee", "12.50"],
    ["Total", {"formula": "SUM(B2:B2)"}]
  ]
}
```

```json
{
  "operation": "evaluate_formula",
  "path": "Documents/budget.xlsx",
  "sheet": "Budget",
  "formula": "SUM(B2:B3)"
}
```

Python skills should call this native tool through the Tool Bridge by listing `office_workbook` in `internal_tools`; do not import Excelize directly.
