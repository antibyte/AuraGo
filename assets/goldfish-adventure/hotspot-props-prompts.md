# Hotspot artwork corrections

These ImageGen assets make existing authored interactions visibly identifiable.
They add no story actions or localized bitmap text. The original scene versions
remain in Git; `art-layout.json` owns placement and native atlas cell extraction.

## R06 emergency coffee machine

- Tool: built-in `image_gen`, opaque background, referenced-image edit.
- Input: `masters/scenes/R06.png`, SHA-256
  `ce35698bb9ad1173dccbfd86cacd1aed9cfd93b4e7c849ea849bf1e61a863f01`.
- Output: `exec-b7e96dcd-ee2c-44cb-88a9-9e586e0ffe61.png`, retained as the
  R06 master and byte-identical runtime scene; native 1672 by 941 pixels.

```text
Edit this original hand-painted 16:9 point-and-click adventure background with ONE precise addition. Preserve the exact camera, framing, aspect ratio, room dimensions, floor, engine, disconnected coupling, canister shelves, hanging lamps, wall monitor, clipboards, and articulated brass read head. Add a small, unmistakable quirky emergency coffee machine on the FAR RIGHT end of the existing wooden workbench: centered around normalized image x=0.945, base at y=0.565, about 0.085 of image width and 0.145 of image height. It must sit solidly on the tabletop, to the RIGHT of the articulated read head, clearly separate from it, and must not cover the blank papers above. The machine has a squat petrol-blue enamel and brass housing, three distinct small brass selector buttons, a little spout, a cream coffee mug under the spout with subtle steam, and a tiny amber indicator. Match the existing exquisite painterly cartoon material, warm copper light and coherent slightly crooked perspective. This coffee machine is an interactable object, so its silhouette and mug must be legible at game scale. Keep every other scene object in its original position and retain all existing fine painted details. No words, letters, labels, numerals, logos, characters, faces, robots or animals. Empty environment only. Return one full scene at the original landscape 16:9 framing, not a crop or collage.
```

## Transparent interaction props

- Tool: built-in `image_gen`, transparent background, new atlas.
- Output: `exec-07af80c5-a4de-4603-9426-8f577229b3f9.png`, retained as
  `masters/props/audit-props.png`, native 1774 by 887 pixels.
- Four columns, two rows. Native unscaled cells are exported by `build-assets.ps1`.
  The return-circuit cell is an unused source alternative: R13 already depicts
  its unplugged lead and only needs a corrected clickable region.
- Alpha review: saturated preview-background pixels have zero/near-zero alpha;
  inspect composited gameplay, not the hidden RGB preview.
- R05 additionally reuses the existing ImageGen `own_record` paper sprite for
  its freight invoice. Original inventory semantics are unchanged.

```text
Generate an original transparent-background 2D adventure-game PROP SPRITE ATLAS, exactly FOUR equal columns by TWO equal rows, eight separate isolated complete objects. Landscape canvas. Uniform cell grid, each object strictly contained in its own cell with clear transparent margin; no object touches a neighboring cell. No grid lines, no captions, no words, letters, numerals, labels, logos or watermark. Richly hand-painted high-end cartoon adventure art: warm ochre/brass highlights, petrol-blue enamel, aged walnut wood, slightly crooked coherent perspective, beautiful fine purposeful wear and strong readable silhouettes. Fixed front three-quarter camera with modest visible tops, all objects lit warmly from upper-left. Preserve genuine alpha transparency, no floor or environment, only tiny soft local contact shadow. Top row left to right: (1) A small battered wooden repair chair, empty seat, one tilted leg wrapped with brass repair bracket, no character or face. (2) A small countertop menu/IOU stand: dark unlettered slate in worn wooden frame with two loose cream receipts pinned at its side, only subtle illegible abstract strokes, no writing. (3) A short visibly wobbly three-legged walnut stool with mismatched brass feet, empty, no character. (4) A compact open weather ledger on a small wooden stand, cream pages with only simple cloud/sun/droplet pictograms and lines, a tiny seed envelope at the base, no text. Bottom row left to right: (5) A small tilted clipboard with a visible schematic of condensation: blue droplet symbols, copper pipe shapes and arrows, a tucked logbook, no words. (6) A compact petrol-blue/brass wall control panel with exactly THREE large separated round push buttons, clearly distinguishable amber/teal/red colors, no lettering. (7) A separate tiny wall-mounted return-circuit apparatus: glowing amber confirmation bulb at left connected by visible copper wire across a narrow blue mounting plate, the wire ends in a clearly disconnected brass plug at the right beside an empty receiving socket. (8) A compact desktop test-data apparatus: empty translucent glass cartridge slot, two tactile brass toggles and a clearly visible physical lock lever on a small petrol-blue base, tiny teal/red indicators, no face. Eight whole objects, all eight visibly separated. Chairs/stool fully shown including legs; gadgets complete including attachments. Extremely clean game-ready alpha cutouts, no scenery or characters.
```
