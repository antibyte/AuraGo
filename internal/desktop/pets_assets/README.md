# Bundled Desktop Pets

These spritesheets are bundled with AuraGo's virtual desktop pet picker and are
seeded into the workspace when missing or invalid.

Original six pets: OpenPets by OpenPets, MIT licensed.

Upstream repository:
https://github.com/alvinunreal/openpets

The built-in fallback uses `apps/desktop/assets/default-pet-spritesheet.webp`.
The catalog pets mirror the OpenPets `apps/desktop/catalog.v2.fixture.json`
entries downloaded from the official ZIP URLs:

- `snoopy`
- `clippit`
- `tux`
- `wall-e`
- `dobby`

Keep these IDs stable because desktop settings can reference them as active pet
IDs.

## AuraGo personas

Twelve additional `aurago-*` pets are original Imagegen-generated sprite sheets
created from AuraGo persona references. The vampire has two tentacle arms.
They are selectable through the same catalog and missing-pet installation path.
Each sheet is lossless WebP with alpha: 1536x1872, 8x9 cells of 192x208 pixels.

Source art, prompts, reproducible exports and importable ZIPs are retained in the
sibling `personas/openpets/` project. These new sheets are not upstream OpenPets
assets. Rebuild the external web resource set and matching AuraGo binary to ship
new bundled pets; existing installations receive missing catalog pets on load.
