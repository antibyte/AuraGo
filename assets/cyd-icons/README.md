# CYD icon library

1,760 selectable variants: 110 shared motif IDs × four styles (`playful`, `modern`,
`comic`, `sober`) × two colour modes (`color`, `mono`) × still/animated.
The four source resolutions are 24, 32, 48 and 64 pixels. Image widgets scale
them proportionally with their canvas frame; changing the selected icon or
resolution preserves the frame. Higher resolutions use more image memory.

![All eight base sets at 32 pixels](preview.png)

For dark device surfaces, the tintable monochrome variants offer the clearest
contrast; modern colour icons deliberately retain dark navy details. Check
the picker against the actual background before using those at 24 pixels.

## Artwork and reproducibility

The original masters in `sources/` were generated with the built-in ImageGen tool on
2026-10-15; expansion 1 uses the same tool and style references (2026-11-01).
`style-reference.png` establishes home, settings, Wi-Fi, bulb, fan
and warning for each style. The eight 10×6 motif atlases reference this board;
each pair of 6×6 motion atlases references its matching motif atlas. Exact
generation prompts are in `prompts/`. `playful-mono-download-repair.png` is an
ImageGen edit of `playful-mono-motion-0.png`; only its corrected third row is
used. The original atlas is retained for provenance. Expansion 1 adds eight 10×5
motif atlases and 32 animation atlases, each with five rows of six chronological
frames. Their names contain `-expansion-1`. All original source and runtime PNG
bytes are preserved. Expansion monochrome masters use independently drawn navy
silhouettes with transparent interior details; only alpha enters the white,
tintable exports.

Install `assets/cyd-icons/requirements.txt` in a development environment, then run
`python scripts/export-cyd-icons.py` from the repository root. Pillow 13.0.0 is
pinned to preserve the original PNG bytes.
`--check` regenerates in memory and compares every byte and manifest hash.
`--contact-sheets reports/cyd-icons/contact-sheets` writes every combination at
native size on light and dark backgrounds. Reviewed gutter exceptions in the
exporter keep detached rain drops with their cloud. All frames of a motif use
one union bounding box, scale and centre. Artwork occupies at most 70% of the
square, leaving room for rotation and small translations. No generated master
is shipped to the device.

The 8,640 runtime PNGs and strict SHA-256 manifest live in `ui/img/cyd-icons/`,
part of the verified external web resource pack. `export-manifest.json` records
source hashes, dimensions, counts and the runtime manifest hash. PNGs are
authoring/preview/portable-project files. Firmware stores constant RGB565,
RGB565A8 or A8 arrays; it needs no PNG/GIF decoder. Monochrome masters are
independent single-ink silhouettes with transparent negative space, not grayscale
conversions of the colour set. Exports preserve their alpha as a tintable mask.

## Motion and device budgets

Wi-Fi, upload, download, bulb, fan, charging battery, rain, snow, wind, volume,
microphone and bell, plus the 20 expansion animations below, have six ImageGen
frames at six frames per second. Frame
zero is also the still/reduced-motion poster. Other icons use pulse, bob or
rotation; sober icons use a restrained pulse. Native firmware updates run at
84 ms (less than 12 Hz); invisible/off-screen icons do no animation updates,
and deleting a widget deletes its timer and state. The frame arrays stay in
flash. Only the displayed image and timer state consume runtime memory.

32 px is the default below 800×480, 48 px on 800×480. The existing board budget
is 256 KiB for flash profiles below 8 MB and 4 MiB otherwise, regardless of
PSRAM. A 32 px transparent colour image uses 3,072 bytes, A8 uses 1,024;
six distinct frames use 18,432 or 6,144 bytes, plus small descriptors and
runtime state. Content hashes deduplicate repeated imports and shared frames.
Only imported sizes/variants enter a project, never the entire library.

## Expansion 1 motifs

Each row adds six single-frame motifs and four six-frame animations in every
style and tone. Single-frame motifs retain the optional pulse effect.

| Area | Single-frame motifs | Frame animations |
| --- | --- | --- |
| Home | garage, air_quality, air_conditioner, dishwasher, faucet, smoke_detector | blinds, washing_machine, vacuum_robot, coffee_machine |
| Energy, weather and nature | sunrise, sunset, fog, leaf, flower, tree | ev_charger, wind_turbine, water_pump, thunderstorm |
| Network and system | antenna, ssd, memory, usb, shield, key | router, database, printer, bug |
| Media and everyday life | music_note, headphones, calendar, location, heart, shopping_cart | radio, gamepad, alarm_clock, hourglass |
| Characters and animals | cat, dog, fish, smile, sad, sleep | gopher, robot, bird, rocket |

## Acceptance

Automated coverage includes all 8,640 manifest files, the original 3,840 file
hashes, catalog/export parity, visible motion at every size, alpha/dimensions, A8
conversion, exact budgets, content deduplication, partial failure, revision
restore, ZIP round trips, route permissions and old static-image goldens.
The icon fixture is generated for all nine profiles and is included in the
opt-in builder compile matrix (`CYD_BUILDER_IT_ONLY=icons`). Browser coverage
uses Standard/Fruity, both motion preferences, keyboard selection, images and
buttons, tint, style changes, undo/redo, delayed imports and budget recovery.

Firmware compilation requires the Docker builder. Hardware acceptance still
requires a classic CYD without PSRAM and an S3 with PSRAM: check small-icon
recognition on both backgrounds, touch latency, free/minimum heap, Wi-Fi
activity, repeated screen switches, hiding and deleting animated widgets.
Existing community board labels remain unchanged until device verification.

LVGL references: [images](https://lvgl.io/docs/open/9.5/widgets/image.html),
[image sequences](https://lvgl.io/docs/open/9.5/widgets/animimg.html).
