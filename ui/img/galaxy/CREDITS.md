# Galaxy assets

Prepared for AuraGo on 2026-09-05. All runtime files are embedded and served
locally. This is an artistic orbit panorama, not a scientifically scaled or
time-accurate view of Earth and its neighboring galaxies.

| Files | Source and credit | Preparation |
| --- | --- | --- |
| `earth-day-4k.jpg`, `earth-day-2k.jpg` | [NASA Earth Observatory, Blue Marble Next Generation, January 2004](https://science.nasa.gov/earth/earth-observatory/blue-marble-next-generation/base-map/) | Original 5400x2700 RGB base map, resized to 4096x2048 and 2048x1024; JPEG quality 2. |
| `earth-night.png` | [NASA SVS 2916, Earth at Night](https://svs.gsfc.nasa.gov/2916/). Data: Marc Imhoff, NASA/GSFC; Christopher Elvidge, NOAA/NGDC. Image: Craig Mayhew and Robert Simmon, NASA/GSFC. | Original 2048x1024 PNG. Shader colors city light emission and masks it on the day side. |
| `earth-clouds.jpg` | [NASA/Goddard Space Flight Center Scientific Visualization Studio, water cycle flat maps](https://svs.gsfc.nasa.gov/3837/). Visualizer Cindy Starr; GEOS-5 data. | Original 1024x512 grayscale preview, used as an independently rotating opacity layer and cloud shadow mask. It is a model snapshot, not current weather. |
| `galaxy-detail.jpg` | [Hubble Andromeda panorama](https://science.nasa.gov/resource/hubbles-high-definition-panoramic-view-of-the-andromeda-galaxy/). NASA, ESA; J. Dalcanton, B.F. Williams, L.C. Johnson (University of Washington); PHAT team; R. Gendler. | Original 6000x1918 image resized to 4096x1310; JPEG quality 3. Fine stellar detail modulates the authored background on desktop. |
| `space-4k.webp`, `space-2k.webp` | Original AI-assisted composition created with OpenAI image generation, using the credited Hubble image as a visual reference. | Generated master is **1672x941**, resampled to 3840x2160 and 2048x1152, WebP quality 94/92. These are delivery sizes, not native 4K generated detail. |
| `poster-4k.webp`, `poster-2k.webp`, `poster-mobile.webp` | AuraGo renderer capture, with the above source credits retained. | The complete scene at simulation time zero, rendered at 3840x2160 and 1080x1920 portrait. 2K poster is resized to 2048x1152. WebP quality 94/92. Includes Earth, clouds, atmosphere, distant planet and stars. |
| `../chat-ui-icons/theme-galaxy.png` | Original AuraGo spiral icon. | Three rounded spiral arms and a shaded core, rendered with Canvas 2D at 128x128 with transparency. |
| `../../fonts/BarlowCondensed-SemiBold.ttf` | [The Barlow Project Authors / Google Fonts](https://github.com/google/fonts/tree/main/ofl/barlowcondensed) | Unmodified Semibold font; SIL Open Font License in `../../fonts/BarlowCondensed-LICENSE.txt`. Chat prose continues to use Geist. |

Direct original downloads:

- [Blue Marble base JPEG](https://assets.science.nasa.gov/content/dam/science/esd/eo/images/bmng/bmng-base/january/world.200401.3x5400x2700.jpg)
- [City light PNG](https://svs.gsfc.nasa.gov/vis/a000000/a002900/a002916/earthatnight-2048.png)
- [Cloud mask JPEG](https://svs.gsfc.nasa.gov/vis/a000000/a003800/a003837/clouds.0350_print.jpg)
- [Andromeda JPEG](https://assets.science.nasa.gov/content/dam/science/psd/solar/2023/09/h/HubbleAndromeda.jpg)
- [Font](https://raw.githubusercontent.com/google/fonts/main/ofl/barlowcondensed/BarlowCondensed-SemiBold.ttf) and [font license](https://raw.githubusercontent.com/google/fonts/main/ofl/barlowcondensed/OFL.txt)

NASA/ESA credits apply to the source imagery and derivatives; no affiliation
or endorsement is implied. The interface is an original orbital-glass design based on the user-supplied
Galaxy chat reference. No proprietary UI assets or fonts are included.

## Background generation prompt

Use case: cinematic game sky background asset. Create an exquisite photorealistic deep-space panorama, 3840x2160 landscape. Use the attached NASA/Hubble Andromeda photograph as an astronomical detail reference: intricate dusty filaments, countless resolved star clusters, pale gold galactic bulge and icy blue spiral arms. Compose a complete magnificent tilted spiral galaxy in the UPPER LEFT quarter, centered around 24% x, 25% y, its long axis rising diagonally left-to-right, extending over roughly 48% of image width. A second much smaller spiral galaxy sits at 74% x, 19% y, with subtle violet blue outer arms. Restrained faint indigo nebula filaments run along the far left edge and top edge. Vast velvet near-black space across the center and LOWER RIGHT with tiny delicate realistic stars of varied sizes and restrained brightness; an Earth-sized 3D planet will be composited at lower right later, so leave this whole lower-right area dark, without planets. A film-quality observatory view, deep luminous color, delicate dynamic range, extraordinarily crisp astronomical detail, no illustration/cartoon look, no repeated patterns, no excessive lens flare, no text, no UI, no spacecraft, no planets. This is a seamless-feeling immersive backdrop for a premium starship chat interface; focus on a stunning galaxy and elegant negative space.

## Rendering and refresh

Three.js r128 consumes the RGB maps in custom shaders. Daylight shading and ocean
specular operate in linear space; the final planet shader converts to display
gamma. The authored sky is already display-referred. No modern `colorSpace`
property or postprocessing pipeline is required. The low-frequency cloud map
is intentionally shared between quality levels.

`galaxy-scene.js` appends the current build version to textures, posters and
the font. `prepaint-theme.js` versions the saved Galaxy poster before rendering.
CSS has a `galaxy-2` revision fallback for immediate first-time selection; bump
that revision whenever posters are replaced. Refresh the posters with the same
camera, lighting and time-zero pose whenever the scene composition changes.

## Orbital-glass refresh (2026-09-09)

- `space-nebula-{4k,2k}.webp`: original OpenAI Imagegen panorama following the
  supplied composition. The generated master is 1672x941; 3840x2160 and
  2048x1152 are delivery sizes, not native generated 4K detail. Original master
  and generation prompt are retained in the local `disposable/galaxy-assets/space/`.
- `orb-companion.png`: Imagegen original, 1024x1024 RGBA, transparent glossy
  black spherical assistant with two blue-violet eyes and an illuminated orbit.
- `orbit-mark.png`: Imagegen original, 1024x1024 RGBA master, resized to 512x512.
  A glossy violet planet with a diagonal pink/cyan ring, transparent background.
  Masters and prompts for both transparent assets are retained locally under
  `disposable/galaxy-assets/orb/` and `disposable/galaxy-assets/mark/`.
- `control-symbols.svg`: unmodified path geometry from
  [Lucide 0.468.0](https://github.com/lucide-icons/lucide/tree/0.468.0/icons),
  assembled as SVG symbols with a shared 1.7px stroke. ISC license, including
  Feather attribution, in `LUCIDE-LICENSE.txt`. This is a static local asset;
  the interface does not load an icon library or CDN at runtime.

The existing NASA map credits above also apply to the refreshed night-side
Earth and regenerated fallback posters. The generated illustration assets are
artistic compositions, not scientific imagery.
