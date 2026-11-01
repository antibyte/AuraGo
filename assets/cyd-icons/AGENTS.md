# CYD icon artwork and exports

## Purpose

Own the original raster artwork, production prompts and reproducible display exports for the CYD Creator icon library.

## Ownership

- `sources/` and `prompts/`: ImageGen masters and exact production instructions. Rejected drafts stay outside the repository.
- `requirements.txt`, `export-manifest.json`, `preview.png`: authoring dependency pin, provenance/counts and native-size overview.
- Cross-tree: `scripts/export-cyd-icons.py` produces `ui/img/cyd-icons/`; `internal/cydcreator/icons.go` owns runtime IDs, categories and motion kinds. The domain, server and Desktop app guides retain runtime/API/UI ownership.

## Local Contracts

- All eight style/tone combinations share the same 110 motif IDs and four sizes (24/32/48/64). There are 32 six-frame animations and 78 single-frame motifs, totaling 8,640 PNGs and 1,760 still/animated selections.
- The original 60 IDs, their order, source masters and 3,840 runtime PNGs remain unchanged. The shipped-library test pins their combined path/digest checksum.
- Expansion 1 appends the five `EXPANSION_ROWS` in the exporter in order. Each style/tone has one `*-expansion-1.png` 10-column/5-row atlas and four `*-expansion-1-motion-{0..3}.png` 6-column/5-row atlases. Each animation row contains six chronological frames.
- All masters have real transparent alpha and empty gutters. Mono expansion masters use independent one-ink silhouettes with transparent interior detail; exports replace their ink with white and retain alpha for tinting. Never derive mono masks by desaturating colour artwork.
- Frame zero is the complete still/reduced-motion poster. Six frames play at 6 fps. Fixed parts retain their anchor/scale; the exporter uses a union bounding box and 70% maximum extent for the whole sequence.
- Runtime manifest/project/API formats remain version 1. Only selected variants enter firmware, under the existing flash budgets and import permissions.

## Work Guidance

- Use the existing style references and built-in ImageGen. Keep exact prompts, review all cells, replace defective drafts and retain only the selected final masters.
- New atlas columns are split in each row's transparent gutters; original atlas handling and reviewed rain-gutter exceptions remain intact.
- Use the pinned Pillow environment. Update Go catalog, exporter lists, all 16 locale sets and operator/user guides together; category labels must cover the catalog dynamically.
- Keep QA sheets, animation previews and hardware/compile evidence in ignored `reports/`. Do not claim hardware acceptance from browser fixtures or compiler success.

## Verification

- `python -m pip install -r assets/cyd-icons/requirements.txt`
- `python scripts/export-cyd-icons.py --check`; `--contact-sheets reports/cyd-icons/contact-sheets` also writes native-size light/dark review sheets.
- `python scripts/test_export_cyd_icons.py`
- `go test ./internal/cydcreator/...`; `go test ./ui -run '(?i)CYDCreator'`; server `TestCYDCreatorIconAPI`.
- Browser `TestDesktopCYDCreatorIconsBrowser` with `AURAGO_RUN_BROWSER_SMOKE=1`; strict builder matrix with `CYD_BUILDER_IT_ONLY=icons` as documented in the domain guide.

## Child DOX Index

None.
