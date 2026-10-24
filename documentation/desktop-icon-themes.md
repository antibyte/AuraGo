# Desktop icon themes

Papirus remains the default. Settings → Appearance also offers WhiteSur,
Windows XP, Tela, Candy, Fluent and Buuf. Tela and Fluent each remember their
own folder color (`standard` initially). Selecting a complete Desktop design
still applies that design's icon preset atomically; later customizations persist.
Existing settings storage and API are unchanged; no database migration is needed.

## Sources and reproduction

The reviewed selection is in `assets/desktop-icon-themes/`. `sources.json` pins
full revisions and SHA-256 hashes, and each theme JSON maps individual semantic
names to source paths or original AuraGo supplements. No Linux installer runs.

| Theme | Revision | Imported artwork |
| --- | --- | --- |
| [Tela](https://github.com/vinceliuice/Tela-icon-theme) | `a1fffc5bfab716bd022dd228ee96fe3965cdb33d` | GPL-3.0 |
| [Candy](https://github.com/EliverLara/candy-icons) | `8d21022f3b77d6a3b44f9a4a1ed26d6a974c41f6` | GPL-3.0 |
| [Fluent](https://github.com/vinceliuice/Fluent-icon-theme) | `c8a244f571ef8fb47812bdc9dbce76546cedce69` | GPL-3.0 |
| [Buuf For Many Desktops](https://git.disroot.org/eudaimon/buuf-nestort) | `ba218523983aec90f1e9facaefeeeeecdcf6d6a5` | CC BY-NC-SA 2.5 |

Acquire the first three archives from
`https://codeload.github.com/<owner>/<repository>/tar.gz/<full-revision>` and name
them `tela.tar.gz`, `candy.tar.gz`, `fluent.tar.gz`. For Buuf, fetch its original
Git repository, then run `git archive --format=tar --output=buuf.tar <revision>`.
The importer refuses different checksums. Archive acquisition is a developer
step, never a browser or application startup operation.

```sh
python scripts/import-desktop-icon-themes.py --source-dir /path/to/archives
python scripts/import-desktop-icon-themes.py --check
python scripts/import-desktop-icon-themes.py --check --source-dir /path/to/archives
python scripts/test-desktop-icon-import.py
node scripts/test-desktop-icon-resolver.mjs
npm run build:ui
npm run check:ui
npm run test:ui-regressions
go test ./internal/desktop ./ui -run 'Desktop.*Icon|DesktopSDK' -count=1
AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run DesktopAdditionalIcons -count=1
go run ./cmd/assetpack -out disposable/_iconpack -stage disposable/_iconstage
```

The offline check verifies coverage, aliases, input/output hashes, local paths,
SVG safety, PNG transparency support, license notices and regular output files.
The archive-backed check additionally reproduces every output byte. Decoded PNG
transparency and backend/frontend palette parity are tested in Go. Generated
artwork is deduplicated across roles within the same theme/license; each use keeps
its source, link chain, original hash and modifications in `provenance.json`.
Reviewed control-only color corrections preserve upstream geometry while making
small Candy and Fluent controls legible on their intended surface.
All outputs and `.txt` license notices travel through the versioned web resource
set. The importer only prunes unreferenced generated images inside its four owned
output directories.

## License evidence

Tela/Fluent retain upstream COPYING and AUTHORS (including the Numix, Papirus
and Paper source credits). Candy retains its LICENSE and Eliver Lara attribution.
Buuf's original revision includes `index.theme`, which declares CC BY-NC-SA 2.5,
and a README crediting Paul Davey (Mattahan), OxayotlTheGreat and robson-66, with
eudaimon's remix. The [original artist's Buuf publication](https://www.deviantart.com/mattahan/art/Buuf-37966044)
also expressly describes the 2.5 license. Those original files and the full
[license text](https://creativecommons.org/licenses/by-nc-sa/2.5/legalcode.en)
are packaged beside the selected transparent PNGs. Inherited system icon themes
and unreviewed placeholder files are not part of the selection.

Buuf's noncommercial condition applies to the imported assets in this resource
set, including redistribution. They are not relicensed under AuraGo's MIT.
Original AuraGo additions are separately identified as MIT; they do not change
the upstream license. The allowlists identify rejected invalid upstream motifs.

## Rendering contract

Manifest version 2 keeps the existing `icons`/`aliases` shape and adds
`actions.light`, `actions.dark`, `action_aliases`, `folder_colors` and
`folder_variants.<color>`. Older manifests remain valid. The shared
`ui/js/desktop/icon-theme-resolver.js` is bundled into both the Desktop shell
and SDK, so aliases and roles use the same resolution rules.

`iconMarkup` and `AuraDesktop.ui.icon(name, {size, usage})` accept `icon`,
`folder` and `action`. File managers and dialogs mark directories explicitly;
a document file and a Documents directory can share a semantic name without
sharing their color treatment. SDK buttons request `action` automatically.
Changing icons never changes labels, focusable controls or their hit targets.
Arbitrary embedded third-party application interiors remain independent.

On theme/color/scheme changes the shell rerenders tracked decorative icons in
place, including open windows and menus. It broadcasts `aurago.desktop.appearance`
with a monotonically increasing revision through current document-bound
MessagePorts. The SDK clears context/icon caches, refreshes rendered icons,
rejects old-port messages and suppresses stale async render results. Failed
image loads and missing manifests reveal a readable local fallback; the previous
theme image is removed. SDK source is `sdk-runtime.js`; `aura-desktop-sdk.js` is
generated by the normal UI bundle builder.

## Acceptance evidence

`TestDesktopAdditionalIconsAppearanceBrowser` covers four themes, both schemes,
taskbar/dock, open Files/Settings/Writer/Sheets, folder/file separation, fast
settings writes, failed saves, missing assets/manifests, return to legacy sets,
and unchanged mobile hit targets/keyboard focus. The SDK channel test exercises
real MessageChannels, delayed context responses and replaced ports. Browser
screenshots are written to ignored `ui/reports/desktop-icon-themes/`.
