# Aurora miniature icons

Original artwork generated with the built-in Imagegen tool on 2026-09-09.
standard.png and fruity.png are source sheets, not runtime assets.
Pack with `python scripts/pack_desktop_icons.py`; verify with `--check`.
Soft alpha is preserved. A gutter prevents adjacent cell bleed.

## Shared prompt

Create a production transparent PNG sprite sheet of exactly 64 tiny desktop action icons in an exact 8 columns by 8 rows grid. Square 1024 x 1024 canvas. Every cell 128 x 128; isolated icon centered in its cell, generous transparent gutter, no overlaps. True transparent alpha background, no checkerboard, no text, no captions, no grid lines, no numbers. These are crisp premium miniature objects legible when reduced to 16-24px: bold simple silhouettes, few details, front-facing orthographic, soft bevels, precise edge highlights, no cast shadows outside objects. Varied meaningful colors (gold folders, silver gears, blue documents, coral media, purple tools); never monochrome green. TOP LEFT LIGHT. All 64 symbols must be distinct and in exactly this row-major order:
Row1: new document with plus, new folder with plus, clipboard paste, two documents copy, scissors cut, trash bin, pencil edit, floppy disk save.
Row2: silver gear settings, colorful wallpaper landscape, four-pane window layout, app grid of colored tiles, magnifying glass search, upload tray, download tray, printer.
Row3: blue document, gold folder, blue home, monitor display, terminal screen, code brackets on document, archive zipper box, notebook.
Row4: chat bubbles, envelope mail, telephone handset, microphone, bell notification, two people contacts, shield security, brass key.
Row5: calendar, checklist, clock, bar chart, processor chip, network nodes, server tower, database cylinder.
Row6: photo landscape, camera, film strip video, music note, speaker, headphones, radio receiver, game controller.
Row7: paintbrush palette, eyedropper, crop tool, cube 3D, sliders controls, puzzle extension, package box, globe.
Row8: lightning run, bookmark ribbon, gold star, coral heart, paperclip attachment, chain link, life ring help, info badge.
STYLE:

Standard style: Aurora Workstation desktop, refined Ubuntu/Windows-inspired precision, colorful softly beveled enamel and satin metal miniatures, crisp compact geometric forms, controlled depth. Real usable icon sheet, not a UI mockup.

Fruity style: premium contemporary Apple-inspired rounded satin ceramic miniatures, sculpted gentle curves, softly frosted pastel enamel surfaces, simplified shapes, tiny precise highlights, delightful but not childish. Calm blue, apricot, lavender, pink, silver and warm yellow. More satin than glossy.

## Action-family extension

Generated with the built-in Imagegen tool on 2026-09-09. The original
`standard-actions.png` and `fruity-actions.png` add 16 motifs per theme.
The packer uses reviewed transparent crop lanes in these 1254 x 1254 sheets
and appends two atlas rows. The original 8x8 sheets also use reviewed lanes
to exclude fragments from adjacent rows. Existing motif coordinates stay unchanged.
Action artwork takes precedence over SVG fallbacks, including layout/widgets.
Arrows, plain checkmarks and window controls retain their structural SVGs.

### Standard action prompt

Use case: stylized-concept. Production asset: a TRUE TRANSPARENT PNG sprite sheet containing exactly 16 tiny desktop ACTION icons in a regular 4 columns x 4 rows grid on a 1024x1024 canvas. Every icon centered within its 256x256 cell, 40px transparent padding all around, no cells overlap, no background, no checkerboard, NO TEXT, no labels or grid lines. Optimize for 16-24px rendering: chunky filled silhouettes, very simple few details, front-facing orthographic, top-left light, fine bevel edge and calm colored enamel depth, no external cast shadows. Each icon must be colored, NOT white line art. Exact row-major order: Row 1: blue square tile with a crisp white checkmark (select all/complete); matching blue empty inset square tile (unchecked); blue descending three horizontal list bars with a coral down arrow (sort); turquoise-blue circular refresh arrow with chunky arrowhead. Row 2: lavender curved undo arrow pointing left; matching lavender redo arrow pointing right; blue rectangular list panel with three rows; four rounded tiles in blue, coral, gold and teal (grid). Row 3: blue two-column window panel; open blue eye with iris; matching blue eye crossed with coral diagonal slash; blue magnifying glass with plus. Row 4: blue magnifying glass with minus; blue square panel with coral arrow pointing top-right (open external); silver-blue tiny keyboard with only a few bold keys; half-blue half-ivory circle (contrast). All icons equal optical size, consistent restrained saturation and top-left lighting. Real usable assets, not a UI mockup. Style: Aurora Workstation Standard: refined Ubuntu/Windows precision, softly beveled enamel and satin metal miniatures, crisp compact geometric forms, controlled depth. Match gold folders and blue file icons from a premium desktop.

### Fruity action prompt

Use case: stylized-concept. Production asset: a TRUE TRANSPARENT PNG sprite sheet containing exactly 16 tiny desktop ACTION icons in a regular 4 columns x 4 rows grid on a 1024x1024 canvas. Every icon centered within its 256x256 cell, 40px transparent padding all around, no cells overlap, no background, no checkerboard, NO TEXT, no labels or grid lines. Optimize for 16-24px rendering: chunky filled silhouettes, very simple few details, front-facing orthographic, top-left light, fine bevel edge and calm colored enamel depth, no external cast shadows. Each icon must be colored, NOT white line art. Exact row-major order: Row 1: blue square tile with a crisp white checkmark (select all/complete); matching blue empty inset square tile (unchecked); blue descending three horizontal list bars with a coral down arrow (sort); turquoise-blue circular refresh arrow with chunky arrowhead. Row 2: lavender curved undo arrow pointing left; matching lavender redo arrow pointing right; blue rectangular list panel with three rows; four rounded tiles in blue, coral, gold and teal (grid). Row 3: blue two-column window panel; open blue eye with iris; matching blue eye crossed with coral diagonal slash; blue magnifying glass with plus. Row 4: blue magnifying glass with minus; blue square panel with coral arrow pointing top-right (open external); silver-blue tiny keyboard with only a few bold keys; half-blue half-ivory circle (contrast). All icons equal optical size, consistent restrained saturation and top-left lighting. Real usable assets, not a UI mockup. Style: Fruity premium Apple-inspired rounded satin ceramic miniatures, sculpted gentle curves, softly frosted pastel enamel surfaces, simplified shapes, tiny precise highlights, delightful but not childish. Calm periwinkle blue, apricot, lavender, pink, silver and warm yellow. More satin than glossy. Restrained polished desktop icons.
