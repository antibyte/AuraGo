# Studio stickers

100 original Imagegen illustrations shared by Video Studio and Pixel. Each
motif was generated separately with a transparent background. The ten styles
cycle by one position between categories, giving every category all ten styles.
`production.json` records the stable IDs, subjects, style assignments and prompts.

Open [the contact sheet](contact-sheet.html) to inspect every sticker against
light and dark backgrounds; names link to the full-resolution PNGs.

The delivered files are 1024 × 1024 RGBA PNGs in `ui/img/stickers/`. Imagegen
returned 1280px sources, which were downsampled using Lanczos resampling.
Thumbnails in `thumbs/` are 192 × 192, with a 188px rendering and a two-pixel
transparent safety margin. Full-resolution files load only when inserted.
Lossless PNG compression uses adaptive filtering; decoded RGBA pixels were
compared before and after compression. The originals total approximately
113 MiB; both apps reuse this single copy.
1024px gives useful headroom for typical 250–500px stickers in 1080p projects,
but does not make them resolution-independent.

Video Studio copies the selected original into the project through its existing
media importer. Pixel retains the original and affine transform in the current
session; saving/exporting produces a flat image without flattening the open
document. There is no new project format, migration, service or remote asset URL.

Regenerate the catalog and contact sheet after changing the manifest:

```sh
node scripts/build-stickers.mjs
node scripts/build-stickers.mjs --check
go test ./ui -run TestDesktopStickerAssets
go test ./internal/videostudio -run TestRenderBundledSticker1080p
```

The asset test decodes all 200 PNGs, verifies dimensions, actual transparency,
visible content and clear boundary pixels. Inspect new artwork visually before
accepting it; alpha checks cannot detect malformed anatomy or incorrect motifs.
