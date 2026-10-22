# Chapter 31: Pixel – edit images in the Virtual Desktop

Pixel is the image editor in the Virtual Desktop. It works on layers with blend modes, masks, clipping masks and
selections. It has painting and retouch tools, 80 filters and adjustments, 43 generators for noise, textures, patterns
and mathematical images, a background remover, and a command palette. Projects can be saved as layered OpenRaster
files (`.ora`), which GIMP and Krita open. Pixel works on raster pictures: PNG, JPEG and `.ora` projects.

Background removal is on-device by default. The optional AI models run on your AuraGo server and are downloaded only when
you click for them. The older, provider-based features in the **AI** panel need an image-generation provider, as described
in [Chapter 4](./04-webui.md) and [Chapter 21](./21-api-reference.md#pixel-image-editor-api).

## Table of Contents
1. [Opening Pixel](#opening-pixel)
2. [Layout](#layout)
3. [Layers](#layers)
4. [Selections](#selections)
5. [Painting and Retouching](#painting-and-retouching)
6. [Shapes, Gradients and Fill](#shapes-gradients-and-fill)
7. [Move, Transform, Crop and View](#move-transform-crop-and-view)
8. [Filters and Adjustments](#filters-and-adjustments)
9. [Generators](#generators)
10. [Background Removal](#background-removal)
11. [Intelligence](#intelligence)
12. [Project Files](#project-files)
13. [Limits](#limits)
14. [Keyboard Shortcuts](#keyboard-shortcuts)
15. [Privacy and Safety](#privacy-and-safety)
16. [When Something Goes Wrong](#when-something-goes-wrong)

---

## Opening Pixel

Open **Pixel** from the start menu. A few other ways to start:

- **File → New image** creates a blank document. The generators (see [Generators](#generators)) can also create one.
- **Ctrl+O** (**File → Open**) opens a PNG, JPEG or `.ora` file. The dialog starts in the Photos folder. You can also drop
  an image or an `.ora` file onto the Pixel window.
- Photos from the Camera app open in Pixel.

**Ctrl+S** saves to the file you opened. **Ctrl+Shift+S** (**File → Save as**) saves under a new name. **File → Export**
downloads a flattened copy in the format you choose. In a read-only Virtual Desktop, saving is blocked.

## Layout

- **Menu bar:** File, Edit, Image, Layer, Select, Filter, Generate, View and AI.
- **Tool rail** (left): the tools, grouped as Select, Paint, Retouch, Fill, Shape, Type and View. Hover over a tool to see
  its name and shortcut.
- **Options bar** (above the canvas): the settings of the active tool, such as size, opacity, mode and tolerance.
- **Canvas** (centre): your picture on a checkerboard for transparency. Press Space and drag to pan, and Ctrl and the mouse
  wheel to zoom.
- **Inspector** (right, top): the tabs **Adjust**, **Filters**, **Colors**, **Brush**, **Transform**, **Generate**,
  **Cutout**, **AI** and **Stickers**. Only one tab is shown at a time.
- **Bottom dock** (right, below the inspector): **Layers** and **History**. Drag the splitter between the inspector and the
  dock to change their sizes.

The status line shows the size of a selection or shape while you drag. The command palette is one keystroke away
(**Ctrl+K**, see [Intelligence](#intelligence)).

## Layers

Every layer is a full-size picture, and the composition is the stack of all visible layers. The **Layers** dock shows the
settings of the active layer at the top, and the stack below it, top layer first.

- **Blend mode:** Normal, Multiply, Screen, Overlay, Darken, Lighten, Color dodge, Color burn, Hard light, Soft light,
  Difference, Exclusion, Hue, Saturation, Color and Luminosity.
- **Opacity:** a slider and a number field, from 0 to 100 %.
- **Locks:** **Lock pixels** (no painting or filtering), **Lock transparency** (the alpha channel stays as it is) and
  **Lock position** (no moving). A locked layer shows a lock badge, and an action on it is refused with "This layer is
  locked."
- **Clipping mask:** a clipping layer is clipped to the alpha of the nearest layer below it that is not clipping. Use the
  clip button in the Layers header, the Layer menu, or **Ctrl+Alt+G**.
- **Visibility:** click the eye to hide or show a layer. **Alt+click** the eye shows only that layer. Alt+click again shows
  all layers.
- **Rename:** double-click the name, or press **F2**. Press Enter to keep the name and Escape to cancel.
- **Reorder:** drag a row, or use **Ctrl+]** and **Ctrl+[** to move the active layer up and down.
- **Selection from a layer:** **Ctrl+click** a layer thumbnail to select the opaque pixels of that layer. Shift adds this to
  the selection and Alt subtracts it.
- **Masks:** a mask hides or reveals parts of a layer without changing its pixels. Add one from the footer or the Layer
  menu: reveal all, hide all, from the selection, or hide the selection. Click the mask thumbnail to paint on the mask:
  white reveals, black hides. Shift+click the mask thumbnail switches it on or off, and Ctrl+click makes a selection from
  it. The Layer menu also has **Invert mask**, **Apply mask** (writes the mask into the pixels, so the transparency is
  permanent) and **Delete mask**. Apply mask is refused on locked layers.

The footer of the Layers dock has buttons for **Add Layer** (Ctrl+Shift+Alt+N), **Layer via copy** (Ctrl+J), **Duplicate**,
**Add layer mask**, **Delete Layer**, **Merge Down** (Ctrl+E) and **More**. Right-click a row for the same actions as the
Layer menu:

- **Create:** New layer, New fill layer (in the primary colour), Layer via copy, Layer via cut (with a selection),
  Duplicate, Place image as layer…
- **Edit:** Rename layer, Layer properties (name, opacity, blend mode, locks and clipping in one dialog), Fill with primary
  or secondary color, Clear layer.
- **Combine:** Merge Down (Ctrl+E), Merge visible (Ctrl+Shift+E) and Flatten, which combines all layers into one layer
  named Background.
- **Arrange:** Bring to front (Ctrl+Shift+]), Bring forward (Ctrl+]), Send backward (Ctrl+[), Send to back (Ctrl+Shift+[),
  Select layer above (Alt+]) and Select layer below (Alt+[).
- **Align:** left, horizontal center, right, top, vertical center and bottom. Alignment uses the selection when there is
  one, otherwise the canvas.
- **Other:** Selection from layer, Rasterize sticker (for sticker layers), Export layer as PNG.

The last layer cannot be deleted. A document can hold up to 64 layers.

## Selections

| Tool | Shortcut | How it works |
|---|---|---|
| Rectangle select | `V` | Drag a rectangle. The style can be Normal, a fixed ratio (W : H) or a fixed size (W × H). |
| Ellipse select | `Shift+V` | Drag an ellipse. |
| Lasso | `L` | Drag a freehand outline. |
| Polygon lasso | `Shift+K` | Click the corners. Double-click or Enter closes the shape, Backspace removes the last point, Escape cancels. |
| Magic wand | `W` | Selects a region of similar colour. Options: Tolerance, Contiguous and All layers. |
| Quick selection | `Shift+W` | Paint over the subject. The selection grows through similar colours and stops at edges. Alt subtracts. |

All selection tools share four modes: **Replace**, **Add** (Shift), **Subtract** (Alt) and **Intersect** (Shift+Alt). The
options bar has the same choice as a segmented control. Feather and anti-aliasing apply to the selection's edge.

**Select → Color range…** selects by colour across the whole picture. Click the image to pick colours, Shift+click adds a
colour and Alt+click removes one, and a fuzziness slider sets how close a colour must be. A red overlay shows the result
before you apply it.

The **Select** menu also has:

- **All** (Ctrl+A), **Deselect** (Ctrl+D), **Reselect** (Ctrl+Shift+D) and **Inverse** (Ctrl+Shift+I).
- **Select subject**, which uses the cutout engine (see [Background Removal](#background-removal)).
- **Modify:** Grow and Shrink (1 to 250 px), Border (1 to 200 px), Smooth (1 to 100 px) and Feather (Shift+F6, 0.5 to 250 px).
- **Selection from layer transparency**, **Save selection as layer mask**, **Copy to new layer** and **Cut to new layer**
  (Ctrl+Shift+J).
- **Stroke selection…** draws a line along the outline, with a width and a location: Inside, Center or Outside.
- **Transform Selection** scales and rotates the selection outline only, not the pixels.

Fill shortcuts for the selection: **Alt+Backspace** fills with the primary colour and **Ctrl+Backspace** with the secondary
colour. **Shift+Backspace** opens content-aware fill (see [Intelligence](#intelligence)).

To move the outline, drag inside it with a selection tool. Arrow keys nudge the selection by 1 px, or by 10 px with Shift.
**Escape** removes the selection, or cancels an active crop.

## Painting and Retouching

### Brush engine

The **Brush** (`B`), **Pencil** (`P`), **Airbrush** (`A`) and **Eraser** (`E`) tools share one engine. Its settings are in the
options bar, and every setting is also in the **Brush** tab.

- **Size** from 1 to 1000 px. The keys `[` and `]` make the brush smaller and larger.
- **Opacity** and **Flow**. The keys 1 to 9 set the opacity to 10 to 90 %, and 0 sets 100 %.
- **Hardness** from soft to hard edge. The keys `Shift+[` and `Shift+]` change it.
- **Spacing**, **Smoothing** (a stabiliser that smooths the stroke), **Angle**, **Roundness** and **Follow stroke direction**.
- **Shape:** round, square, star or spray. **Scatter**, **Dabs per step** and size, angle and opacity **jitter** randomise the
  dabs. **Grain** adds paper or chalk texture, with a grain size.
- **Pen pressure:** Off, Size, Opacity or Both. Only a pen reports pressure. Mouse and touch strokes always use full pressure.
- **Symmetry:** Off, Vertical, Horizontal, Both, or Radial with 2 to 24 segments around the centre of the canvas. Guide
  lines show the mirror axes.
- **Eraser mode:** Brush (soft) or Block (square).

Ten presets set these values at once: Round hard, Round soft, Airbrush, Pencil, Ink, Marker, Chalk, Calligraphy, Spray and
Star scatter. Choose one from the preset list in the options bar.

Several details help while painting:

- **Shift+click** draws a straight line from the end of the previous stroke.
- **Alt+click** picks the colour under the pointer as the primary colour.
- **Escape** cancels the stroke that is in progress.
- The pencil draws hard single pixels without anti-aliasing.
- Strokes respect the selection (with feathering), the alpha lock and mask-edit mode. In mask-edit mode the brush paints
  white, which reveals, and the eraser writes black, which hides.

### Retouch tools

| Tool | Shortcut | What it does |
|---|---|---|
| Smudge | `Y` | Pushes colour along the stroke. The finger option paints with the primary colour. |
| Blur / sharpen brush | `N` | Blurs, or sharpens, the area under the brush. The Mode setting chooses. |
| Dodge and burn | `D` | Lightens (dodge) or darkens (burn). Saturate and Desaturate act like a sponge. Range sets Shadows, Midtones or Highlights. |
| Clone stamp | `S` | Alt+click sets the source. Aligned keeps the offset between source and brush. All layers samples the whole picture. |
| Healing brush | `Shift+J` | Clones from the source and matches the colour and brightness of the area under the brush. |
| Spot healing | `J` | Paint over a blemish and release. The area is filled from its surroundings. |
| Color replace | `Shift+B` | Gives similar colours the primary colour's hue and saturation. The brightness stays. |
| Red eye | `Shift+E` | Click a pupil. Search radius (5 to 200 px) and darkening (0 to 100 %) are adjustable. |
| Remove object | (none) | Paint over an object and release. Pixel fills the area (see [Intelligence](#intelligence)). |
| Cutout brush | (none) | Refines a background-removal mask (see [Background Removal](#background-removal)). |

## Shapes, Gradients and Fill

**Shapes** are drawn with an outline in the primary colour and an optional fill in the secondary colour:

- **Rectangle** (`R`), **Rounded rectangle** (`Shift+R`, corner radius 0 to 500 px), **Ellipse** (`O`), **Polygon** (`Shift+O`,
  3 to 24 sides), **Star** (`Shift+S`, 3 to 24 points, inner radius 10 to 95 %), **Line** (`Shift+L`) and **Arrow**
  (`Shift+A`, with a head at the start, the end or both, and a head size from 1 to 10).
- Stroke width from 1 to 200 px, a solid, dashed or dotted line style, line caps, anti-aliasing and opacity.
- Hold Shift to constrain: squares, circles and lines at 45-degree steps. Hold Alt to draw from the centre. Escape cancels
  the drag.

**Gradient** (`U`) has five types: Linear, Radial, Conic, Diamond and Reflected. Its colours are primary to secondary,
primary to transparent, or one of 18 presets. Reverse and Dither (which reduces banding) are options. Shift limits the
direction to 45 degrees, and the gradient stays inside the selection.

**Fill** (`G`) is the paint bucket. **Contiguous** (on by default) fills only the connected area under the click, and
turning it off fills every matching pixel. Tolerance runs from 0 to 255 (default 32). **All layers** samples the whole
picture for the colour. The fill colour is the primary colour or a pattern (checker, stripes, dots or grid). Anti-aliasing
and opacity are also available.

**Eyedropper** (`I`) sets the primary colour from the pixel under the pointer.

**Text** (`T`) places text on the canvas. Click, type in the box, and press Enter to place it. Shift+Enter starts a new
line, and Escape discards the text. Clicking elsewhere also places the text.

## Move, Transform, Crop and View

**Move** (`M`) moves the active layer. With a selection it moves the selected pixels, and the selection moves with them.

- **Alt+drag** copies the selection instead of moving it.
- **Ctrl+click** picks the layer under the cursor when there is no selection. The **Auto-select layer** option does this on
  every click.
- Arrow keys nudge by 1 px (10 px with Shift). Escape cancels a move in progress.
- **Snap to Guides** snaps the moved content to the canvas and layer edges and centers when it comes close to them.

**Free transform** (**Ctrl+T**, or **Ctrl+Alt+T**) changes the active layer or the selection:

- The corner and edge handles scale. Shift keeps the proportions and Alt scales from the centre.
- Drag outside the box to rotate. Shift rotates in 15-degree steps. Drag inside the box to move.
- The options bar has numeric fields for X, Y, Width, Height and Angle, and a lock for the aspect ratio.
- **Enter** applies the transform and **Escape** cancels it.
- **Edit → Transform Layer** flips the layer horizontally or vertically, and rotates it by 90 degrees clockwise or
  counter-clockwise.

**Crop** (`C`, also **Image → Crop** and the crop button in the **Transform** tab) shows handles on a crop rectangle. You can
drag the handles or move the rectangle. The ratio is Free, Original, Custom, 1:1, 4:3, 3:2, 16:9 or 9:16. The overlay is
rule of thirds (the default), a grid, or off. The area outside the crop is darkened. Enter applies the crop and Escape
cancels it.

**View tools:**

- **Hand** (`H`) pans by dragging.
- **Zoom** (`Z`): a click zooms in, Alt+click zooms out, and dragging a rectangle zooms to that area.
- Space and drag pans with any tool. Ctrl and the mouse wheel zoom.

The **View** menu has Zoom in (Ctrl++), Zoom out (Ctrl+-), Fit (Ctrl+0), Zoom 100 %, Fill Screen, **Pixel Grid** (shown at
800 % and above), **Grid** (8, 16, 32, 64 or 128 px) and **Snap to Guides**.

## Filters and Adjustments

Filters and adjustments apply to the active layer, and inside a selection they blend in at the feathered edge. Changing a
setting shows a live preview. The preview is computed in the background, and on large pictures it first uses a smaller
copy. **Apply** writes the result to the layer, and **Reset** and **Cancel** discard it.

### Filters tab

- The search box and the chips (All, Favorites and the eight categories) narrow the gallery. Click the star on a card to
  add the filter to **Favorites**.
- Click a card to open its settings. Below the parameters, **Strength** (0 to 100 %) and **Blend** fade the filter into the
  original. The blend modes are Normal, Multiply, Screen, Overlay, Soft light, Hard light, Darken, Lighten, Difference, Color
  and Luminosity.
- **Before / After** shows the original and the result, separated by a divider you can drag.
- **Repeat last filter** (**Ctrl+F**) runs the last filter again with its last settings. **Last filter…** (**Ctrl+Alt+F**)
  opens the last filter's dialog first.

The 80 filters, by category:

| Category | Count | Filters |
|---|---|---|
| Adjust | 22 | Levels, Curves, Hue/Saturation, Color balance, Vibrance, Channel mixer, Black & white, Photo filter, Gradient map, Threshold, Posterize, Invert, Equalize, Auto levels, Auto contrast, Auto white balance, Grayscale, Monochrome, Warm, Cool, Tint, High contrast |
| Blur | 8 | Gaussian blur, Box blur, Motion blur, Spin blur, Zoom blur, Lens blur, Surface blur, Median |
| Sharpen | 4 | Sharpen, Unsharp mask, High pass, Clarity |
| Distort | 10 | Twirl, Pinch / bulge, Wave, Ripple, Polar coordinates, Spherize, Kaleidoscope, Displace, Glass, Lens distortion |
| Stylize | 15 | Oil paint, Halftone, Dither, Pixel art, Toon, Crystallize, Pointillize, Emboss, Find edges, Glowing edges, Sketch, Comic, Neon, Mosaic, Pixelate |
| Noise | 4 | Add noise, Reduce noise, Dust & scratches, Film grain |
| Light | 7 | Vignette, Bloom, Glow, Light leak, Lens flare, Chromatic aberration, Tilt shift |
| Retro | 10 | Vintage, Fade, Cyanotype, Duotone, Sepia, VHS, CRT, Glitch, Solarize, Cross process |

Some filters have their own controls:

- **Levels:** channel (RGB, Red, Green or Blue), input black, gamma and input white, and output black and white.
- **Curves:** one curve per channel (RGB, R, G or B). Click to add a point, drag to move it, and double-click to remove it.
  Presets: Linear, Contrast, Strong contrast, Lighten, Darken, Matte, Negative and Cross process.
- **Dither:** Floyd–Steinberg, Atkinson, or ordered Bayer 2×2, 4×4 and 8×8. The palette is 1-bit, Game Boy, CGA, C64, PICO-8,
  EGA, Web-safe, or From image with 2 to 64 colours.
- **Gradient map** and **Duotone** use a colour gradient or two colours.

Use the **Filter** menu to reach every filter by category.

### Adjust tab

- A **histogram** at the top shows the image before and after your changes. Its buttons switch between RGB, Luma, Red,
  Green and Blue.
- **Sliders** from −100 to +100 in three groups: Light (exposure, brightness, contrast, highlights, shadows, whites, blacks),
  Color (temperature, tint, vibrance, saturation) and Detail (clarity, dehaze, sharpness, vignette).
- **Auto** sets levels, contrast, white balance and a little vibrance in one step. Press **Apply** to keep the result.
- **Presets:** Vivid, Matte, Black & white, Warm film, Cool fade, High key, Low key and Punch.
- **Compare** shows the before and after view, **Apply** writes the change and **Reset** clears the sliders.

**Auto enhance** (**Ctrl+Shift+L**, in the **Filter** menu) applies the automatic correction directly, without the sliders.

## Generators

The **Generate** tab, the **Generate** menu and the command palette (**Generate: …**) create new pictures from parameters.
Generators run on your device and nothing is downloaded. The same seed and settings always give the same picture.

| Category | Count | Generators |
|---|---|---|
| Noise | 9 | White noise, Value noise, Perlin noise, Simplex noise, Cellular (Worley), Fractal noise (fBm), Ridged multifractal, Turbulence, Domain-warped noise |
| Textures | 10 | Clouds, Marble, Wood, Plasma, Camouflage, Lava, Caustics, Nebula and stars, Paper and canvas, Brushed metal |
| Patterns | 10 | Checkerboard, Stripes, Grid, Polka dots, Hexagon tiles, Truchet tiles, Stained glass, Halftone gradient, Bricks and herringbone, Waves |
| Math | 14 | Gradient, Mandelbrot set, Julia set, Burning Ship, Newton fractal, Spirograph, Rose curve, Lissajous curve, Superformula, Phyllotaxis, Moiré and interference, Flow field, Reaction–diffusion, Maze |

Common controls:

- **Seed** and a dice button for a new random seed.
- **Gradient** colours the result with a preset gradient.
- **Tileable** (noise generators) makes the picture wrap at the edges, so it repeats seamlessly when placed side by side.
- **Presets:** one to four per generator. For example, Perlin noise offers Soft clouds, Hard rock and Smooth hills.
- **Preview on canvas** shows the result at document size on the canvas before you generate. Switching it off removes the preview.

Choose where the result goes:

- **New layer** (default) adds a layer above the active one. Its blend mode and opacity are set in the panel.
- **Replace layer** replaces the pixels of the active layer.
- **Into selection** fills only the selection. With no selection, the whole layer is filled.

**Generate** renders at full size and shows a progress bar. **Cancel** stops the render. With no document open, the panel
asks for a width and a height (16 to 8192 px, default 1024 × 1024) and creates a new document.

## Background Removal

Pixel removes backgrounds in two ways: the **classic** engine, which runs in your browser, and optional **AI** models, which
run on your AuraGo server. Both produce a **layer mask**. The original pixels stay in place, so you can refine the cutout
or undo it. The tools are in the **Cutout** tab and in the **AI** menu.

### Engines

| Engine | Where it runs | Notes |
|---|---|---|
| Automatic (default) | Depends on the choice | Uses the AI model if one is downloaded, otherwise the classic engine. |
| AI standard | AuraGo server | ISNet general-use (Apache-2.0), about 179 MB download. |
| AI precise | AuraGo server | BiRefNet general lite (MIT), about 224 MB download. |
| Classic (on device) | Your browser | Separates subject and background with colour models and a graph cut. Works well when the subject has a clear edge. |
| Color key | Your browser | Removes one colour. **Detect from the edges** (on by default) picks it, or set **Key color** yourself. Tolerance (0 to 100 %, default 40) and Softness (0 to 100 %, default 20) control the edge. |

### AI models

The AI models are not part of AuraGo. They are downloaded only when you click the button on the model card in the **Cutout**
tab, labelled **Download model (179 MB)** or **Download model (224 MB)**. The card then shows **Downloading** with a
percentage, and finally **Ready**. The first download also fetches the ONNX Runtime library for your platform if it is
missing. Pixel checks each download against its pinned size and SHA-256 hash before using it.

- The AI engines appear only where the server has a runtime for the platform. Otherwise the panel says that the AI cutout
  is not available on this device, and the classic engine still works.
- In a read-only Virtual Desktop, model downloads are disabled.
- Measured on a desktop CPU, an 800 × 533 photo took about 2 seconds with ISNet and about 16 seconds with BiRefNet. Your
  hardware will differ.

### Actions

- **Remove background** adds a layer mask to the active layer. Nothing is deleted, so you can refine or remove the mask later.
- **Select subject** creates a selection around the subject. The Select menu has the same command.
- **Cut out to new layer** copies the subject to a new layer, with the background transparent and colour fringes removed.
- **Blur background** adds a blurred copy of the background as a new layer. **Blur strength** (1 to 100 %) sets how strong the blur is.
- **Replace background** puts a solid colour (white by default), transparency, or a generated background under the subject.
  Generated backgrounds are the generators (clouds, gradients and similar) that suit a backdrop.

The **AI** menu contains **Remove background**, **Cut out to new layer**, **Blur background** and **Cutout** (which opens the
panel).

The AI panel also has a **Remove background** button. It is the older, provider-based version and needs a configured image
provider. The Cutout tab does not need one.

### Refine

Once the active layer has a mask, the **Refine** section of the **Cutout** tab is available:

- **Edge shift** (−10 to +10 px) grows or shrinks the mask. **Feather** (0 to 20 px) softens the edge, and **Smooth** (0 to 10)
  smooths its outline.
- **Remove color fringe** (on by default) takes the background colour out of the edge pixels. **Fringe radius** (1 to 20 px,
  default 6) sets how far from the edge it works.
- **View** shows the result as Normal, Checkerboard, On black, On white or Red overlay.
- **Cutout brush** paints on the mask. Keep adds the subject back and Remove takes it out, with a size from 2 to 300 px and a
  hardness. Hold Alt to switch between Keep and Remove. With **Smart** switched on, the engine runs again on release, and
  your strokes guide it.
- **Apply mask** bakes the refined mask into the layer's pixels, so its transparency is permanent. **Reset** restores the
  refinement settings to their defaults and rebuilds the mask from the raw result of the engine.

The **Cancel** button stops a running cutout.

## Intelligence

### Command palette

Press **Ctrl+K** (or **Ctrl+Shift+P**) to open the command palette. It lists every command: menu items, tools, panels,
filters and generators, with their shortcuts and groups.

- **Type to search.** Matching is fuzzy: the letters must appear in the same order, and matches at the start of a word rank
  higher.
- **English and German synonyms** work in both directions. For example, `freistellen` finds **Remove background**,
  `rauschen` finds the noise filters, `weichzeichnen` finds the blur filters, and `ebene` finds the layer commands.
- **Arrow keys** move through the list, **Enter** runs the highlighted command, and **Escape** closes the palette. Commands
  that are not available right now are shown as disabled.
- With an empty search, the palette shows your recent commands (the last 20), followed by suggestions for the current state.
  With a selection it suggests Content-aware fill, Feather and Copy to new layer. With a freshly opened photo it suggests Auto
  enhance and Remove background. With transparent edges it suggests Trim transparent pixels.

### Content-aware fill and removing objects

- **Edit → Content-aware fill…** (**Shift+Backspace**) fills the selection from its surroundings. **Sample area** is Auto or
  Around selection, with a margin from 16 to 512 px when you choose Around selection. **Expand selection** grows the area by
  0 to 24 px. **Quality** is Fast, Balanced or Best. The fill runs in the background with a progress bar and a Cancel button.
- **Remove object** (in the Retouch group of the tool rail) removes what you paint over. Paint across the object with a
  brush from 4 to 200 px and release. The area is then filled from its surroundings.

### Colors tab

- A colour square and a hue bar, a hex field and an opacity slider.
- **Harmonies** of the primary colour: Complementary, Analogous, Triadic, Split-complementary, Tetradic and Monochrome shades.
  Click one to use it.
- **Swatches** are saved colours, up to 48. Click one to set the primary colour, right-click to set the secondary colour, and
  Shift+click to remove it.
- **Palette from image** extracts the main colours of the picture. Choose 5, 6, 8, 10, 12 or 16 colours (default 8), then
  press **Extract**. The result can be added to the swatches, turned into a gradient, or placed as a palette layer.
- **Presets** and an eyedropper button are also in this tab.

### Image actions

The **Image** menu has four actions:

- **Trim transparent pixels** removes transparent edges.
- **Auto-crop to content** crops away the background, meaning the colour in the corners or transparency.
- **Canvas size…** (**Ctrl+Alt+C**) sets a new width and height, with the old picture placed by an anchor in a 3 × 3 grid. The new
  area is Transparent, Primary, Secondary, White or Black. The canvas can be up to 16384 px per side and 120 million pixels.
- **Fit canvas to layers** shrinks the canvas to the area that the layers cover.

## Project Files

Pixel saves three formats:

- **OpenRaster (`.ora`)** keeps the whole project: every layer with its name, order, opacity, visibility, blend mode, mask,
  clipping and locks. It is a standard OpenRaster file, so GIMP and Krita open the layers. A merged preview and a thumbnail
  are included. Locks and clipping are stored as extra attributes that other programs ignore. Sticker layers are saved as
  pictures.
- **PNG** and **JPEG** save the visible picture as one flattened image. The first time you save a multi-layer document this
  way, Pixel shows "Layers are flattened — save as .ora to keep them."

In the save dialog, OpenRaster is the first choice when the document has several layers or masks. Otherwise PNG and JPEG come
first. The open dialog offers images and OpenRaster projects. Dropping a `.ora` file onto the window opens it as well.

An OpenRaster project that has more than 64 layers cannot be opened, and one whose decoded layers would need more than 1 GiB
of memory is refused.

## Limits

| Item | Limit |
|---|---|
| Layers per document | 64 |
| Undo steps (History tab) | 60. Older steps are dropped, and the memory held by undo steps is capped at about 768 MiB. |
| Canvas size | 16384 px per side and 120 million pixels |
| New document from a generator | 16 to 8192 px per side (default 1024 × 1024) |
| PNG and JPEG files | Up to 32 MiB and 32 megapixels. A lower desktop file size limit applies when it is set. |
| OpenRaster project | Up to 32 MiB, up to 64 layers when opening, and up to 128 files inside the archive |
| AI cutout | The image is scaled to at most 2048 px on its longer side before upload. One run at a time, and a run is stopped after 180 seconds. |
| Filter previews | Previews run on a smaller copy of large pictures before the full-size result |

## Keyboard Shortcuts

Shortcuts in the table are the ones registered in the editor. Ctrl and Cmd are the same key.

### File, edit and selection

| Key | Action |
|---|---|
| `Ctrl+O`, `Ctrl+S`, `Ctrl+Shift+S` | Open, save, save as |
| `Ctrl+Z`, `Ctrl+Shift+Z` | Undo, redo |
| `Ctrl+C`, `Ctrl+X`, `Ctrl+V` | Copy, cut, paste |
| `Delete` | Delete the selection (in the layer list it deletes the layer) |
| `Ctrl+A`, `Ctrl+D`, `Ctrl+Shift+D`, `Ctrl+Shift+I` | Select all, deselect, reselect, invert selection |
| `Shift+F6` | Feather the selection |
| `Ctrl+J` | Layer via copy |
| `Ctrl+Shift+J` | Cut the selection to a new layer |
| `Alt+Backspace`, `Ctrl+Backspace` | Fill the selection with the primary, or the secondary, colour |
| `Shift+Backspace` | Content-aware fill |
| `Ctrl+T` or `Ctrl+Alt+T` | Free transform |
| `Escape` | Cancel the current action: a crop, a stroke, a shape, a transform, a move or a text entry; otherwise deselect |
| Arrow keys | Nudge the selection by 1 px; with Shift by 10 px |

### Tools

| Key | Tool |
|---|---|
| `V`, `Shift+V`, `L`, `Shift+K`, `W`, `Shift+W` | Rectangle select, ellipse select, lasso, polygon lasso, magic wand, quick selection |
| `M` | Move |
| `B`, `P`, `A`, `E` | Brush, pencil, airbrush, eraser |
| `S`, `Shift+J`, `J`, `Y`, `N`, `D`, `Shift+B`, `Shift+E` | Clone stamp, healing brush, spot healing, smudge, blur / sharpen brush, dodge and burn, color replace, red eye |
| `G`, `U`, `I` | Fill, gradient, eyedropper |
| `R`, `Shift+R`, `O`, `Shift+O`, `Shift+S`, `Shift+L`, `Shift+A` | Rectangle, rounded rectangle, ellipse, polygon, star, line, arrow |
| `T` | Text |
| `H`, `Z`, `C` | Hand, zoom, crop |

### Brush settings

| Key | Action |
|---|---|
| `[`, `]` | Smaller and larger brush |
| `Shift+[`, `Shift+]` | Softer and harder edge |
| `1` to `9`, `0` | Opacity 10 % to 90 %, and 100 % for `0` |
| Shift+click | Straight line from the end of the previous stroke |
| Alt+click | Pick the colour under the pointer (paint tools) |

### View, commands and filters

| Key | Action |
|---|---|
| `Space` and drag | Pan |
| `Ctrl` and mouse wheel | Zoom |
| `Ctrl++`, `Ctrl+-`, `Ctrl+0` | Zoom in, zoom out, fit |
| `Ctrl+K` or `Ctrl+Shift+P` | Command palette |
| `?` or `F1` | Show the keyboard shortcuts |
| `Ctrl+Alt+C` | Canvas size |
| `Ctrl+F` | Repeat the last filter |
| `Ctrl+Alt+F` | Last filter, with its dialog |
| `Ctrl+Shift+L` | Auto enhance |

### Layers

| Key | Action |
|---|---|
| `Ctrl+Shift+Alt+N` | Add layer |
| `F2` | Rename layer |
| `Ctrl+E`, `Ctrl+Shift+E` | Merge down, merge visible |
| `Ctrl+Alt+G` | Clipping mask |
| `Ctrl+]`, `Ctrl+[` | Bring forward, send backward (also `Ctrl+Shift+ArrowUp` and `Ctrl+Shift+ArrowDown`) |
| `Ctrl+Shift+]`, `Ctrl+Shift+[` | Bring to front, send to back |
| `Alt+]`, `Alt+[` | Select the layer above or below (also `Alt+ArrowUp` and `Alt+ArrowDown`) |

With the layer list focused, the arrow keys select layers, Home and End go to the top and bottom layer, Space shows or hides
the active layer, and Enter opens its properties.

## Privacy and Safety

- **On your device.** Painting, filters, generators, the classic background engine, layers and saving run in your browser
  and on your Virtual Desktop files. Pixel loads no external fonts or libraries.
- **AI background removal.** Your image is sent only to your AuraGo server, which runs the model locally. The model files
  come from the rembg release on GitHub, and only when you click **Download model**. Each file is checked by its size and
  SHA-256 hash.
- **Provider features.** Generate, Enhance, Remove background and Upscale in the **AI** panel send your prompt and image to
  the image provider configured under **Config**. Without a provider they are unavailable.
- **Where files go.** Saved pictures are written to the Virtual Desktop folder you choose. The AI models are stored on the
  server in the AuraGo data directory.
- **Read-only.** In a read-only Virtual Desktop, saving and model downloads are blocked. You can still look at and edit a
  picture in the window, but the changes cannot be saved.

## When Something Goes Wrong

| Message | What helps |
|---|---|
| "This layer is locked." | Turn off the lock on the layer in the Layers header. |
| "Maximum number of layers reached (64)." | Merge or delete layers. |
| "The last layer cannot be deleted." | A document always keeps one layer. |
| "Make a selection first." | Make a selection, or press **Ctrl+A** to select everything. |
| "A cutout is already running." | Wait for it to finish, or press **Cancel** in the **Cutout** tab. |
| "The AI cutout failed (…). Using the classic engine instead." | The reason is in the brackets: the model is not downloaded (download it in the **Cutout** tab), it is not available on this device, another cutout is running, it took too long, the Virtual Desktop is read-only, or the server could not run it. |
| "No red eye found here." | Click closer to the pupil, or increase the search radius. |
| "There is nothing around the selection to copy from. Select a smaller area." | Select a smaller area for the content-aware fill. |
| "The image is completely transparent." | Trimming found no visible pixels, so there is nothing to keep. |
| "That canvas size is not possible." | Choose a smaller width or height (up to 16384 px per side and 120 million pixels). |
| "Generator failed" | Check the width and height of the new document, or try another seed. |
| "The image could not be placed." | Check that the file is a PNG or JPEG image. |
| "The project file is not a valid OpenRaster archive." | The file is damaged or is not an OpenRaster file. Save it again from the program that made it. |

---

**See also:** [Chapter 4: Web UI](./04-webui.md) · [Chapter 21: API reference](./21-api-reference.md#pixel-image-editor-api) · [Chapter 14: Security](./14-security.md) · [Chapter 16: Troubleshooting](./16-troubleshooting.md)

---

**Previous Chapter:** [Chapter 30: RuView](./30-ruview.md)
