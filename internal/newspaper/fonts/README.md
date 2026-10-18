# Newspaper PDF fonts

The PDF renderer keeps the Go regular/bold fonts for editions they cover and
falls back to this Noto Sans SC pair when both weights cover the complete text.
This includes Chinese source titles in otherwise German editions. Both download
and Telegram use the same renderer. Fonts ship in the verified, version-bound
resource set under `newspaper/fonts`; no per-export download or system font
installation is needed. gofpdf subsets the embedded
fonts to the used glyphs in each PDF.

Source: [Noto CJK Sans 2.004](https://github.com/notofonts/noto-cjk/tree/523d033d6cb47f4a80c58a35753646f5c3608a78),
`Sans/Variable/TTF/Subset/NotoSansSC-VF.ttf`.
Source SHA-256: `d68bafcb48a2707749396aa12bbbd833cb70401f3a9a689fd2902c7e0d295964`.
The upstream SIL Open Font License is in `OFL.txt`; embedded font copyright
and name records are retained.

These are static 400/700 instances, restricted to the Unicode BMP supported by
gofpdf, without hinting or OpenType layout features. They add about 20 MB to the
expanded resource set, not the binary. Missing resources preserve Latin PDF
export and signal failure for CJK so Telegram can use its text fallback.
They use Simplified Chinese glyph forms, including for shared Han
characters in Japanese/Traditional Chinese text. This is glyph coverage, not a
language-specific shaping engine. Unsupported glyphs and scripts requiring the
currently excluded shaping paths still fail explicitly, retaining Telegram's
complete text fallback.

To regenerate with Python and `fonttools==4.62.1`, download the pinned source,
verify its hash, then run these commands (from a temporary working directory):

```sh
python -m fontTools.varLib.instancer NotoSansSC-VF.ttf wght=400 --update-name-table --no-recalc-timestamp --output regular.ttf
python -m fontTools.varLib.instancer NotoSansSC-VF.ttf wght=700 --update-name-table --no-recalc-timestamp --output bold.ttf
python -m fontTools.subset regular.ttf --unicodes=U+0000-FFFF --layout-features= --no-hinting --name-IDs='*' --name-languages='*' --output-file=NotoSansSC-Regular.ttf
python -m fontTools.subset bold.ttf --unicodes=U+0000-FFFF --layout-features= --no-hinting --name-IDs='*' --name-languages='*' --output-file=NotoSansSC-Bold.ttf
```

Generated SHA-256:

- Regular: `559df0dd02112015eaf3679f4b6475029b75e99f5a0ffdb24b790e46082718e0`
- Bold: `edb44c83bb1592cb0d3d9707f936dae0e23a0ecc47e5bad4164d215c01b00039`

Run `go test ./internal/newspaper` and
`go test ./internal/server -run TestNewspaper`. For font updates, also render a
mixed German/Chinese fixture, extract its text with an independent PDF reader,
and inspect regular body text, bold headings, sources and page footers.
