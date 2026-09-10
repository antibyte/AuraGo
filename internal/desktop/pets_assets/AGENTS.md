# Desktop Pet Assets

## Purpose

OpenPets-compatible sprite artwork shipped in AuraGo's external resource set.

## Ownership

- `service_pets.go:bundledDefaultPets` owns the selectable catalog.
- Six original OpenPets sheets and twelve `aurago-*` persona sheets live here.
- Persona generation sources, prompts, export script and importable packages
  live in the sibling `personas/openpets/` project, outside runtime resources.

## Local Contracts

- Each runtime sheet is lossless RGBA WebP, 1536x1872: 8 columns, 9 rows,
  192x208 per frame. Row frame counts are 6,8,8,4,5,8,6,6,6, matching
  `ui/js/desktop/core/pet-runtime.js`.
- Preserve existing IDs, selected pets and user-installed files. Add reviewed
  personas through the current catalog and missing-pet repair path.
- The vampire (`aurago-evil`) has two tentacle arms and ordinary legs.
- Keep source images and intermediate exports outside the production directory.
  `assets/web-assets.json` already packages `.webp` files from this subtree;
  never add Go embeds or a production source-directory fallback.
- The original OpenPets provenance in README applies to its six original
  sheets, not to the newly generated AuraGo persona artwork.

## Work Guidance

Re-export reviewed art with the sibling project's `pack-sprites.cjs`, then copy
only final sheets here. Check transparent borders, full-body alignment and real
frame changes on both light and dark backgrounds before updating assets.

## Verification

`go test ./internal/desktop -run 'TestBundledPersonaSprites|TestInstallBundledDefaultPets|TestServiceBootstrapIncludesAllBundledDefaultPets'`
checks actual sheet pixels and catalog installation. Existing
`go test ./ui -run DesktopPet` covers the shared picker/runtime contract.

## Child DOX Index

None.
