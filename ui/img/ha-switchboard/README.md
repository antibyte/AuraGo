# HA Switchboard artwork

Original artwork generated for AuraGo on 2026-09-10 with the built-in image
generation tool (no CLI or downloaded third-party stock). The user's reference
sets the walnut cabinet/material direction; its surrounding room is not included.
Repository-original contributions are distributed under the repository MIT license;
`LICENSE.txt` accompanies the runtime assets. No exclusive rights are claimed in
machine-generated output.

| File | Original dimensions | SHA-256 |
| --- | --- | --- |
| `switch-atlas.png` | 1536 × 1024, RGB | `a875c99488723ff9543320baff5ce64f76895f67e19d507076bae3aaf63bfd76` |
| `walnut.png` | 1254 × 1254, RGB | `0d411070088c3c2d289749148baa767b1905a1ec65bda597e4b1597faf7f28ed` |

These PNGs preserve the generator's original bytes. The atlas contains three
512-pixel-wide cells: on, intermediate, off. It did not provide true alpha;
`ha-switchboard.js` clips the hardware with window-local SVG paths. Do not replace
these files with a baked checkerboard image. CSS supplies the cabinet frame,
recesses, screws, lamps and analog dial. Text remains semantic DOM content.
The plaque material reuses the existing `../radio/metal.png` asset.

`icon.svg` is original hand-authored SVG, mirrored in the Papirus and WhiteSur
icon directories. Keep these copies and their manifests synchronized.

## Switch atlas generation prompt

Create a production game/UI sprite atlas of ONE vintage industrial SILVER CHROME
LEVER SWITCH in three mechanical positions, for a hyperrealistic walnut Home
Assistant switchboard. Photorealistic brushed stainless rectangular mounting plate
with four small slotted steel screw heads, a deep black vertical hinge recess,
substantial long polished chrome tapered lever with round bulb grip, very realistic
dark and bright metallic reflections, directional warm studio light from upper
left and contact shadows. Not a modern plastic toggle: an antique long mechanical
knife-switch style handle mounted in a pivot, elegant early 20th century control
panel. THREE full identical hardware assemblies side by side in equal-width cells
on one wide 1536x1024 TRANSPARENT RGBA canvas. Left cell ON lever tilted upwards;
middle cell half-way lever pointing toward viewer with foreshortening; right cell
OFF lever tilted downwards. Same brushed silver mounting plate and screw positions
in all three cells, same camera straight-on orthographic front view and identical
mounting plate dimensions and baseline. Center each assembly in its cell with
generous margin. The plate extends vertically from 20% to 80% of canvas height and
is 50% of cell width. Lever stays within each cell. True alpha transparency around
metal assembly, no black/white/checkerboard background, no wood background, no
text, no labels, no captions, no arrows, no surrounding scene. The output is ONE
coherent three-frame animation sprite atlas.

## Walnut generation prompt

Production material texture for a photorealistic antique electrical switchboard
cabinet. Fill the whole square 1024x1024 image edge to edge with richly figured
polished DARK AMERICAN WALNUT WOOD. Flat front orthographic macro photograph, warm
dark chocolate brown and reddish walnut grain, flowing horizontal wavy grain with
subtle burl figuring, pores, old shellac patina, restrained fine scratches. Deep
brown, not black, not orange. Uniform soft warm diffuse lighting, seamless tileable
on all edges, no perspective, no vignette, no directional cast shadow, no bevels,
no frame, no screws, no controls, no labels. Real cabinetry from 1920s precision
laboratory instrument. This is ONLY a seamless walnut texture, no panel object or
background.
