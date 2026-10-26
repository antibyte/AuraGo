// Pixel filters and adjustments (WP5): the filter catalog, the Filters and Adjust panels, live previews through the worker
// (pixel-lib-fx.js), the curves editor, favourites, repeat-last-filter and the Filter menu.
// Every pixel change goes through commitLayerPixels; previews are layer previews (setLayerPreview), never layer pixels.
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    const FAV_KEY = 'pixel.filter_favorites';
    const PROXY_EDGE = 1024;              // long edge of the preview proxy for big documents
    const FULL_PREVIEW_PIXELS = 2000000;  // up to this size the preview runs at full resolution straight away
    const SYNC_PIXELS = 160000;           // up to this size immediate previews run on the main thread (instant, no round trip)
    const THUMB_EDGE = 96;
    const THUMB_SCALE = 0.3;              // thumbnails pretend to be a ~300 px wide image so pixel-sized effects stay visible

    // --- labels ---

    const snake = value => String(value).replace(/[^a-z0-9]+/gi, '_').replace(/^_+|_+$/g, '').toLowerCase();

    const FILTER_NAMES = {
        levels: 'Levels', curves: 'Curves', 'hue-saturation': 'Hue/Saturation', 'color-balance': 'Color balance', vibrance: 'Vibrance', 'channel-mixer': 'Channel mixer',
        'black-white': 'Black & white', 'photo-filter': 'Photo filter', 'gradient-map': 'Gradient map', threshold: 'Threshold', posterize: 'Posterize', invert: 'Invert',
        equalize: 'Equalize', 'auto-levels': 'Auto levels', 'auto-contrast': 'Auto contrast', 'auto-white-balance': 'Auto white balance', grayscale: 'Grayscale',
        monochrome: 'Monochrome', warm: 'Warm', cool: 'Cool', tint: 'Tint', 'high-contrast': 'High contrast',
        blur: 'Gaussian blur', 'box-blur': 'Box blur', 'motion-blur': 'Motion blur', 'radial-blur': 'Spin blur', 'zoom-blur': 'Zoom blur', 'lens-blur': 'Lens blur',
        'surface-blur': 'Surface blur', median: 'Median',
        sharpen: 'Sharpen', 'unsharp-mask': 'Unsharp mask', 'high-pass': 'High pass', clarity: 'Clarity',
        twirl: 'Twirl', pinch: 'Pinch / bulge', wave: 'Wave', ripple: 'Ripple', polar: 'Polar coordinates', spherize: 'Spherize', kaleidoscope: 'Kaleidoscope',
        displace: 'Displace', glass: 'Glass', 'lens-distortion': 'Lens distortion',
        'oil-paint': 'Oil paint', halftone: 'Halftone', dither: 'Dither', 'pixel-art': 'Pixel art', toon: 'Toon', crystallize: 'Crystallize', pointillize: 'Pointillize',
        emboss: 'Emboss', edge: 'Find edges', 'glowing-edges': 'Glowing edges', sketch: 'Sketch', comic: 'Comic', neon: 'Neon', mosaic: 'Mosaic', pixelate: 'Pixelate',
        'add-noise': 'Add noise', denoise: 'Reduce noise', 'dust-scratches': 'Dust & scratches', grain: 'Film grain',
        vignette: 'Vignette', bloom: 'Bloom', glow: 'Glow', 'light-leak': 'Light leak', 'lens-flare': 'Lens flare', aberration: 'Chromatic aberration', 'tilt-shift': 'Tilt shift',
        vintage: 'Vintage', fade: 'Fade', cyanotype: 'Cyanotype', duotone: 'Duotone', sepia: 'Sepia', vhs: 'VHS', crt: 'CRT', glitch: 'Glitch', solarize: 'Solarize',
        'cross-process': 'Cross process'
    };

    const CATEGORY_NAMES = { adjust: 'Adjust', blur: 'Blur', sharpen: 'Sharpen', distort: 'Distort', stylize: 'Stylize', noise: 'Noise', light: 'Light', retro: 'Retro' };

    // Parameter labels are shared by key (pixel.fxp_<key>); option labels by value (pixel.fxo_<value>).
    const PARAM_NAMES = {
        amount: 'Amount', radius: 'Radius', angle: 'Angle', distance: 'Distance', threshold: 'Threshold', seed: 'Seed', size: 'Size', strength: 'Strength', level: 'Level',
        levels: 'Levels', method: 'Method', mode: 'Mode', mono: 'Monochrome', color: 'Color', position: 'Position', center: 'Center', gamma: 'Gamma', channel: 'Channel',
        in_black: 'Input black', in_white: 'Input white', out_black: 'Output black', out_white: 'Output white', hue: 'Hue', saturation: 'Saturation', lightness: 'Lightness',
        colorize: 'Colorize', tone_hue: 'Tone hue', tone_sat: 'Tone saturation', preserve_luma: 'Preserve luminosity', vibrance: 'Vibrance',
        shadows_cr: 'Shadows: cyan - red', shadows_mg: 'Shadows: magenta - green', shadows_yb: 'Shadows: yellow - blue',
        mid_cr: 'Midtones: cyan - red', mid_mg: 'Midtones: magenta - green', mid_yb: 'Midtones: yellow - blue',
        high_cr: 'Highlights: cyan - red', high_mg: 'Highlights: magenta - green', high_yb: 'Highlights: yellow - blue',
        mix_rr: 'Red from red', mix_rg: 'Red from green', mix_rb: 'Red from blue', mix_gr: 'Green from red', mix_gg: 'Green from green', mix_gb: 'Green from blue',
        mix_br: 'Blue from red', mix_bg: 'Blue from green', mix_bb: 'Blue from blue', mono_r: 'Gray from red', mono_g: 'Gray from green', mono_b: 'Gray from blue',
        reds: 'Reds', yellows: 'Yellows', greens: 'Greens', cyans: 'Cyans', blues: 'Blues', magentas: 'Magentas', tint: 'Tint', tint_color: 'Tint color', tint_amount: 'Tint amount',
        density: 'Density', gradient: 'Gradient', reverse: 'Reverse', clip: 'Clip', contrast: 'Contrast', brightness: 'Brightness', warmth: 'Warmth', coolness: 'Coolness',
        twist: 'Twist', amplitude: 'Amplitude', wavelength: 'Wavelength', phase: 'Phase', segments: 'Segments', rotation: 'Rotation', zoom: 'Zoom', texture: 'Texture', scale: 'Scale',
        gain: 'Gain', detail: 'Detail', ink: 'Ink color', paper: 'Paper color', palette: 'Palette', colors: 'Colors', block: 'Block size', edge: 'Edges', smooth: 'Smoothing', cell: 'Cell size',
        height: 'Height', keep_color: 'Keep color', invert: 'Invert', width: 'Width', smoothness: 'Smoothness', darkness: 'Darkness', outline: 'Outline', dots: 'Dot size', glow: 'Glow',
        intensity: 'Intensity', darken: 'Darken', grout: 'Grout', grout_color: 'Grout color', variation: 'Variation', distribution: 'Distribution', roundness: 'Roundness',
        feather: 'Feather', stretch: 'Stretch', ghosts: 'Ghosts', rays: 'Rays', blur: 'Blur', sepia: 'Sepia', fade: 'Fade', vignette: 'Vignette', grain: 'Grain', desaturate: 'Desaturate',
        lift: 'Lift blacks', shadow: 'Shadow color', highlight: 'Highlight color', balance: 'Balance', scanlines: 'Scanlines', rgb_shift: 'RGB shift', noise: 'Noise', smear: 'Smear',
        tracking: 'Tracking', curvature: 'Curvature', line_size: 'Line size', mask: 'Mask', mask_amount: 'Mask strength', shift: 'Shift', rgb_split: 'RGB split', style: 'Style',
        curves: 'Curves', exposure: 'Exposure', highlights: 'Highlights', shadows: 'Shadows', whites: 'Whites', blacks: 'Blacks', temperature: 'Temperature', clarity: 'Clarity',
        dehaze: 'Dehaze', sharpness: 'Sharpness'
    };

    const OPTION_NAMES = {
        rgb: 'RGB', r: 'Red', g: 'Green', b: 'Blue', luma: 'Luma', average: 'Average', lightness: 'Lightness', to_polar: 'To polar', to_rect: 'To rectangular',
        frosted: 'Frosted', ribs: 'Ribs', blocks: 'Blocks', mono: 'Mono', color: 'Color', floyd_steinberg: 'Floyd-Steinberg', atkinson: 'Atkinson', bayer2: 'Bayer 2x2',
        bayer4: 'Bayer 4x4', bayer8: 'Bayer 8x8', '1bit': '1-bit', gameboy: 'Game Boy', cga: 'CGA', c64: 'C64', pico8: 'PICO-8', ega: 'EGA', websafe: 'Web-safe',
        custom: 'From image', none: 'None', sobel: 'Sobel', laplacian: 'Laplacian', gaussian: 'Gaussian', uniform: 'Uniform', bilateral: 'Bilateral', median: 'Median',
        lateral: 'Lateral', radial: 'Radial', aperture: 'Aperture grille', shadow: 'Shadow mask', shadows: 'Shadows', highlights: 'Highlights', classic: 'Classic', cool: 'Cool', warm: 'Warm',
        normal: 'Normal', multiply: 'Multiply', screen: 'Screen', overlay: 'Overlay', soft_light: 'Soft light', hard_light: 'Hard light', darken: 'Darken', lighten: 'Lighten',
        difference: 'Difference', luminosity: 'Luminosity'
    };

    const opt = value => {
        const key = snake(value);
        return { value, labelKey: 'pixel.fxo_' + key, fallback: OPTION_NAMES[key] || String(value) };
    };

    // --- schema helpers (labels and defaults are added by finalize below; defaults come from the library) ---

    const R = (key, min, max, step, extra) => Object.assign({ key, type: 'range', min, max, step: step || 1 }, extra);
    const SEL = (key, values, extra) => Object.assign({ key, type: 'select', options: values.map(opt) }, extra);
    const SEG = (key, values, extra) => Object.assign({ key, type: 'segmented', options: values.map(opt) }, extra);
    const BOOL = (key, extra) => Object.assign({ key, type: 'bool' }, extra);
    const COL = (key, extra) => Object.assign({ key, type: 'color' }, extra);
    const ANG = (key, extra) => Object.assign({ key, type: 'angle' }, extra);
    const PT = (key, extra) => Object.assign({ key, type: 'point' }, extra);
    const SEED = extra => Object.assign({ key: 'seed', type: 'seed' }, extra);
    const GRAD = (key, extra) => Object.assign({ key, type: 'gradient' }, extra);
    const PCT = { unit: '%' }, PX = { unit: 'px' }, DEG = { unit: '°' };
    const unit = (u, extra) => Object.assign({}, u, extra);

    const PALETTES = ['1bit', 'gameboy', 'cga', 'c64', 'pico8', 'ega', 'websafe', 'custom'];
    const mix = key => R('mix_' + key, -200, 200, 1, { showIf: v => !v.mono });
    const tone = (key) => R(key, -100, 100);

    // The catalog. cat: adjust | blur | sharpen | distort | stylize | noise | light | retro. preview: 'fast' | 'slow' (debounce).
    // thumb: parameters used for the gallery thumbnail when the defaults would not show an effect.
    Pixel.FILTER_CATEGORIES = ['adjust', 'blur', 'sharpen', 'distort', 'stylize', 'noise', 'light', 'retro'];
    Pixel.FILTERS = [
        // adjust
        { id: 'levels', cat: 'adjust', preview: 'fast', thumb: { in_black: 28, gamma: 1.25, in_white: 226 }, params: [SEL('channel', ['rgb', 'r', 'g', 'b']), R('in_black', 0, 254), R('gamma', 0.1, 5, 0.01), R('in_white', 1, 255), R('out_black', 0, 255), R('out_white', 0, 255)] },
        { id: 'curves', cat: 'adjust', preview: 'fast', thumb: { curves: { rgb: [[0, 0], [64, 46], [192, 210], [255, 255]], r: [[0, 0], [255, 255]], g: [[0, 0], [255, 255]], b: [[0, 0], [255, 255]] } }, params: [{ key: 'curves', type: 'custom', factory: 'curves' }] },
        { id: 'hue-saturation', cat: 'adjust', preview: 'fast', thumb: { hue: 40, saturation: 30 }, params: [R('hue', -180, 180, 1, DEG), R('saturation', -100, 100), R('lightness', -100, 100), BOOL('colorize'), R('tone_hue', 0, 360, 1, unit(DEG, { showIf: v => v.colorize })), R('tone_sat', 0, 100, 1, unit(PCT, { showIf: v => v.colorize }))] },
        { id: 'color-balance', cat: 'adjust', preview: 'fast', thumb: { shadows_cr: -25, mid_yb: -25, high_cr: 30 }, params: [tone('shadows_cr'), tone('shadows_mg'), tone('shadows_yb'), tone('mid_cr'), tone('mid_mg'), tone('mid_yb'), tone('high_cr'), tone('high_mg'), tone('high_yb'), BOOL('preserve_luma')] },
        { id: 'vibrance', cat: 'adjust', preview: 'fast', params: [R('vibrance', -100, 100), R('saturation', -100, 100)] },
        { id: 'channel-mixer', cat: 'adjust', preview: 'fast', thumb: { mix_rr: 20, mix_rb: 80, mix_br: 80, mix_bb: 20 }, params: [BOOL('mono'), mix('rr'), mix('rg'), mix('rb'), mix('gr'), mix('gg'), mix('gb'), mix('br'), mix('bg'), mix('bb'), R('mono_r', -200, 200, 1, { showIf: v => v.mono }), R('mono_g', -200, 200, 1, { showIf: v => v.mono }), R('mono_b', -200, 200, 1, { showIf: v => v.mono })] },
        { id: 'black-white', cat: 'adjust', preview: 'fast', params: [R('reds', -200, 300), R('yellows', -200, 300), R('greens', -200, 300), R('cyans', -200, 300), R('blues', -200, 300), R('magentas', -200, 300), BOOL('tint'), COL('tint_color', { showIf: v => v.tint }), R('tint_amount', 0, 100, 1, unit(PCT, { showIf: v => v.tint }))] },
        { id: 'photo-filter', cat: 'adjust', preview: 'fast', params: [COL('color'), R('density', 0, 100, 1, PCT), BOOL('preserve_luma')] },
        { id: 'gradient-map', cat: 'adjust', preview: 'fast', params: [GRAD('gradient'), BOOL('reverse')] },
        { id: 'threshold', cat: 'adjust', preview: 'fast', params: [R('level', 0, 255)] },
        { id: 'posterize', cat: 'adjust', preview: 'fast', params: [R('levels', 2, 32)] },
        { id: 'invert', cat: 'adjust', preview: 'fast', params: [] },
        { id: 'equalize', cat: 'adjust', preview: 'fast', params: [R('amount', 0, 100, 1, PCT)] },
        { id: 'auto-levels', cat: 'adjust', preview: 'fast', params: [R('clip', 0, 5, 0.1, PCT)] },
        { id: 'auto-contrast', cat: 'adjust', preview: 'fast', params: [R('clip', 0, 5, 0.1, PCT)] },
        { id: 'auto-white-balance', cat: 'adjust', preview: 'fast', params: [R('strength', 0, 100, 1, PCT)] },
        { id: 'grayscale', cat: 'adjust', preview: 'fast', params: [SEG('method', ['luma', 'average', 'lightness'])] },
        { id: 'monochrome', cat: 'adjust', preview: 'fast', params: [R('contrast', -50, 100)] },
        { id: 'warm', cat: 'adjust', preview: 'fast', params: [R('warmth', 0, 100), R('saturation', -50, 100)] },
        { id: 'cool', cat: 'adjust', preview: 'fast', params: [R('coolness', 0, 100), R('saturation', -50, 100)] },
        { id: 'tint', cat: 'adjust', preview: 'fast', params: [R('hue', 0, 360, 1, DEG), R('saturation', -50, 100)] },
        { id: 'high-contrast', cat: 'adjust', preview: 'fast', params: [R('contrast', 0, 100), R('brightness', -50, 50)] },
        // blur
        { id: 'blur', cat: 'blur', preview: 'fast', params: [R('radius', 0, 60, 0.5, PX)] },
        { id: 'box-blur', cat: 'blur', preview: 'fast', params: [R('radius', 0, 60, 1, PX)] },
        { id: 'motion-blur', cat: 'blur', preview: 'fast', params: [ANG('angle'), R('distance', 0, 200, 1, PX)] },
        { id: 'radial-blur', cat: 'blur', preview: 'slow', params: [R('amount', 0, 90, 1, DEG), PT('center')] },
        { id: 'zoom-blur', cat: 'blur', preview: 'slow', params: [R('amount', 0, 100, 1, PCT), PT('center')] },
        { id: 'lens-blur', cat: 'blur', preview: 'slow', params: [R('radius', 0, 60, 1, PX), R('brightness', 0, 100, 1, PCT), R('threshold', 0, 254)] },
        { id: 'surface-blur', cat: 'blur', preview: 'slow', params: [R('radius', 1, 30, 1, PX), R('threshold', 1, 100)] },
        { id: 'median', cat: 'blur', preview: 'slow', params: [R('radius', 1, 20, 1, PX)] },
        // sharpen
        { id: 'sharpen', cat: 'sharpen', preview: 'fast', params: [R('amount', 0, 200, 1, PCT)] },
        { id: 'unsharp-mask', cat: 'sharpen', preview: 'fast', params: [R('amount', 0, 500, 1, PCT), R('radius', 0.1, 50, 0.1, PX), R('threshold', 0, 255)] },
        { id: 'high-pass', cat: 'sharpen', preview: 'fast', params: [R('radius', 0.5, 100, 0.5, PX), R('gain', 0.2, 4, 0.1)] },
        { id: 'clarity', cat: 'sharpen', preview: 'fast', params: [R('amount', 0, 200, 1, PCT), R('radius', 5, 100, 1, PX)] },
        // distort
        { id: 'twirl', cat: 'distort', preview: 'slow', params: [R('twist', -720, 720, 1, DEG), R('radius', 10, 200, 1, PCT), PT('center')] },
        { id: 'pinch', cat: 'distort', preview: 'slow', params: [R('amount', -100, 100, 1, PCT), R('radius', 10, 200, 1, PCT), PT('center')] },
        { id: 'wave', cat: 'distort', preview: 'slow', params: [R('amplitude', 0, 100, 1, PX), R('wavelength', 4, 300, 1, PX), ANG('angle')] },
        { id: 'ripple', cat: 'distort', preview: 'slow', params: [R('amplitude', 0, 60, 1, PX), R('wavelength', 6, 200, 1, PX), ANG('phase'), PT('center')] },
        { id: 'polar', cat: 'distort', preview: 'slow', params: [SEG('mode', ['to_polar', 'to_rect'])] },
        { id: 'spherize', cat: 'distort', preview: 'slow', params: [R('amount', -100, 100, 1, PCT), R('radius', 10, 200, 1, PCT), PT('center')] },
        { id: 'kaleidoscope', cat: 'distort', preview: 'slow', params: [R('segments', 2, 24, 1), ANG('rotation'), R('zoom', 20, 300, 1, PCT), PT('center')] },
        { id: 'displace', cat: 'distort', preview: 'slow', params: [R('amount', 0, 200, 1, PX), R('scale', 4, 300, 1, PX), SEED()] },
        { id: 'glass', cat: 'distort', preview: 'slow', params: [SEG('texture', ['frosted', 'ribs', 'blocks']), R('amount', 0, 60, 1, PX), R('scale', 3, 80, 1, unit(PX, { showIf: v => v.texture !== 'frosted' })), SEED({ showIf: v => v.texture !== 'ribs' })] },
        { id: 'lens-distortion', cat: 'distort', preview: 'slow', params: [R('amount', -100, 100, 1, PCT), R('zoom', 50, 200, 1, PCT)] },
        // stylize
        { id: 'oil-paint', cat: 'stylize', preview: 'slow', params: [R('radius', 1, 12, 1, PX), R('detail', 0, 100, 1, PCT)] },
        { id: 'halftone', cat: 'stylize', preview: 'slow', params: [R('size', 3, 40, 1, PX), ANG('angle'), SEG('mode', ['mono', 'color']), COL('ink', { showIf: v => v.mode === 'mono' }), COL('paper', { showIf: v => v.mode === 'mono' })] },
        { id: 'dither', cat: 'stylize', preview: 'slow', params: [SEL('method', ['floyd-steinberg', 'atkinson', 'bayer2', 'bayer4', 'bayer8']), SEL('palette', PALETTES), R('colors', 2, 64, 1, { showIf: v => v.palette === 'custom' }), R('strength', 0, 100, 1, unit(PCT, { showIf: v => /^bayer/.test(v.method) }))] },
        { id: 'pixel-art', cat: 'stylize', preview: 'slow', params: [R('block', 2, 64, 1, PX), SEL('palette', ['none'].concat(PALETTES)), R('colors', 2, 64, 1, { showIf: v => v.palette === 'custom' })] },
        { id: 'toon', cat: 'stylize', preview: 'slow', params: [R('levels', 2, 10), R('edge', 0, 100, 1, PCT), R('smooth', 0, 12, 1, PX)] },
        { id: 'crystallize', cat: 'stylize', preview: 'slow', params: [R('cell', 4, 120, 1, PX), SEED()] },
        { id: 'pointillize', cat: 'stylize', preview: 'slow', params: [R('size', 4, 60, 1, PX), SEED(), COL('paper')] },
        { id: 'emboss', cat: 'stylize', preview: 'fast', params: [ANG('angle'), R('height', 0, 10, 0.1, PX), R('amount', 0, 300, 1, PCT), BOOL('keep_color')] },
        { id: 'edge', cat: 'stylize', preview: 'fast', params: [SEG('method', ['sobel', 'laplacian']), R('gain', 0.2, 6, 0.1), BOOL('invert'), BOOL('color')] },
        { id: 'glowing-edges', cat: 'stylize', preview: 'slow', params: [R('width', 0, 20, 1, PX), R('brightness', 0, 200, 1, PCT), R('smoothness', 0, 30)] },
        { id: 'sketch', cat: 'stylize', preview: 'fast', params: [R('radius', 1, 40, 1, PX), R('darkness', 0, 100), BOOL('keep_color')] },
        { id: 'comic', cat: 'stylize', preview: 'slow', params: [R('levels', 2, 8), R('outline', 0, 100, 1, PCT), R('dots', 0, 30, 1, PX)] },
        { id: 'neon', cat: 'stylize', preview: 'slow', params: [R('glow', 0, 40, 1, PX), R('intensity', 0, 200, 1, PCT), R('darken', 0, 100, 1, PCT)] },
        { id: 'mosaic', cat: 'stylize', preview: 'fast', params: [R('size', 6, 80, 1, PX), R('grout', 0, 8, 0.5, PX), COL('grout_color'), R('variation', 0, 100, 1, PCT), SEED()] },
        { id: 'pixelate', cat: 'stylize', preview: 'fast', params: [R('size', 2, 100, 1, PX)] },
        // noise
        { id: 'add-noise', cat: 'noise', preview: 'fast', params: [R('amount', 0, 100, 1, PCT), SEG('distribution', ['gaussian', 'uniform']), BOOL('mono'), SEED()] },
        { id: 'denoise', cat: 'noise', preview: 'slow', params: [SEG('method', ['bilateral', 'median']), R('radius', 1, 8, 1, PX), R('threshold', 1, 100, 1, { showIf: v => v.method === 'bilateral' })] },
        { id: 'dust-scratches', cat: 'noise', preview: 'slow', params: [R('radius', 1, 16, 1, PX), R('threshold', 0, 255)] },
        { id: 'grain', cat: 'noise', preview: 'fast', params: [R('amount', 0, 100, 1, PCT), R('size', 0.5, 6, 0.1, PX), BOOL('mono'), SEED()] },
        // light
        { id: 'vignette', cat: 'light', preview: 'fast', params: [R('amount', -100, 100), R('size', 0, 100), R('roundness', -100, 100), R('feather', 0, 100)] },
        { id: 'bloom', cat: 'light', preview: 'fast', params: [R('threshold', 0, 254), R('radius', 1, 80, 1, PX), R('intensity', 0, 200, 1, PCT)] },
        { id: 'glow', cat: 'light', preview: 'fast', params: [R('radius', 1, 80, 1, PX), R('intensity', 0, 200, 1, PCT)] },
        { id: 'light-leak', cat: 'light', preview: 'slow', params: [COL('color'), PT('position'), R('size', 5, 200, 1, PCT), R('intensity', 0, 100, 1, PCT), ANG('angle'), R('stretch', 1, 5, 0.1)] },
        { id: 'lens-flare', cat: 'light', preview: 'slow', params: [PT('position'), R('brightness', 10, 300, 1, PCT), R('ghosts', 0, 8), BOOL('rays')] },
        { id: 'aberration', cat: 'light', preview: 'fast', params: [SEG('mode', ['lateral', 'radial']), R('amount', 0, 40, 0.5, PX), ANG('angle', { showIf: v => v.mode === 'lateral' })] },
        { id: 'tilt-shift', cat: 'light', preview: 'slow', params: [R('position', 0, 100, 1, PCT), R('size', 0, 100, 1, PCT), R('blur', 0, 40, 1, PX), ANG('angle'), R('saturation', -50, 100, 1, PCT)] },
        // retro
        { id: 'vintage', cat: 'retro', preview: 'slow', params: [R('sepia', 0, 100, 1, PCT), R('fade', 0, 100, 1, PCT), R('brightness', -50, 50), R('vignette', 0, 100), R('grain', 0, 100), SEED()] },
        { id: 'fade', cat: 'retro', preview: 'fast', params: [R('desaturate', 0, 100, 1, PCT), R('lift', 0, 100, 1, PCT), R('brightness', -50, 50)] },
        { id: 'cyanotype', cat: 'retro', preview: 'fast', params: [R('contrast', -50, 50), R('brightness', -50, 50)] },
        { id: 'duotone', cat: 'retro', preview: 'fast', params: [COL('shadow', { palette: 'secondary' }), COL('highlight', { palette: 'primary' }), R('balance', -100, 100)] },
        { id: 'sepia', cat: 'retro', preview: 'fast', params: [R('amount', 0, 100, 1, PCT)] },
        { id: 'vhs', cat: 'retro', preview: 'slow', params: [R('scanlines', 0, 100, 1, PCT), R('rgb_shift', 0, 20, 1, PX), R('noise', 0, 100, 1, PCT), R('smear', 0, 10, 0.5, PX), R('tracking', 0, 100, 1, PCT), SEED()] },
        { id: 'crt', cat: 'retro', preview: 'slow', params: [R('curvature', 0, 100, 1, PCT), R('scanlines', 0, 100, 1, PCT), R('line_size', 2, 8, 1, PX), SEL('mask', ['aperture', 'shadow', 'none']), R('mask_amount', 0, 100, 1, unit(PCT, { showIf: v => v.mask !== 'none' })), R('glow', 0, 100, 1, PCT)] },
        { id: 'glitch', cat: 'retro', preview: 'slow', params: [R('amount', 0, 100, 1, PCT), R('shift', 0, 200, 1, PX), R('rgb_split', 0, 30, 1, PX), SEED()] },
        { id: 'solarize', cat: 'retro', preview: 'fast', params: [R('threshold', 0, 255), SEG('mode', ['shadows', 'highlights'])] },
        { id: 'cross-process', cat: 'retro', preview: 'fast', params: [R('amount', 0, 100, 1, PCT), SEG('style', ['classic', 'cool', 'warm'])] }
    ];

    // Adds labels to every parameter and the library defaults; the op is always fx.apply (the work runs in the worker).
    Pixel.FILTERS.forEach(def => {
        const lib = window.AuraPixelLib && window.AuraPixelLib.fx;
        const defaults = lib ? lib.defaults(def.id) : null;
        def.op = 'fx.apply';
        def.labelKey = 'pixel.filter_' + snake(def.id);
        def.fallback = FILTER_NAMES[def.id] || def.id;
        def.params.forEach(param => {
            if (!param.labelKey) {
                param.labelKey = 'pixel.fxp_' + param.key;
                param.fallback = PARAM_NAMES[param.key] || param.key;
            }
            if (defaults && defaults[param.key] !== undefined && param.default === undefined) param.default = defaults[param.key];
        });
    });

    // --- adjust panel: sliders, presets ---

    const ADJUST_GROUPS = [
        { id: 'light', labelKey: 'pixel.adjust_group_light', fallback: 'Light', keys: ['exposure', 'brightness', 'contrast', 'highlights', 'shadows', 'whites', 'blacks'] },
        { id: 'color', labelKey: 'pixel.adjust_group_color', fallback: 'Color', keys: ['temperature', 'tint', 'vibrance', 'saturation'] },
        { id: 'detail', labelKey: 'pixel.adjust_group_detail', fallback: 'Detail', keys: ['clarity', 'dehaze', 'sharpness', 'vignette'] }
    ];
    const ADJUST_LEGACY_KEYS = ['brightness', 'contrast', 'saturation', 'exposure', 'sharpness', 'temperature', 'shadows', 'highlights'];
    const ADJUST_KEYS = ADJUST_GROUPS.reduce((all, group) => all.concat(group.keys), []);
    const ADJUST_NAMES = { exposure: 'Exposure', brightness: 'Brightness', contrast: 'Contrast', highlights: 'Highlights', shadows: 'Shadows', whites: 'Whites', blacks: 'Blacks', temperature: 'Temperature', tint: 'Tint', vibrance: 'Vibrance', saturation: 'Saturation', clarity: 'Clarity', dehaze: 'Dehaze', sharpness: 'Sharpness', vignette: 'Vignette' };
    const adjustSchema = keys => keys.map(key => ({
        key, type: 'range', min: -100, max: 100, step: 1, default: 0,
        labelKey: ADJUST_LEGACY_KEYS.includes(key) ? 'pixel.' + key : 'pixel.fxp_' + key, fallback: ADJUST_NAMES[key]
    }));
    const ADJUST_PRESETS = [
        { id: 'vivid', fallback: 'Vivid', values: { contrast: 15, vibrance: 35, saturation: 15, clarity: 15 } },
        { id: 'matte', fallback: 'Matte', values: { blacks: 35, contrast: -15, saturation: -12, highlights: -10 } },
        { id: 'bw', fallback: 'Black & white', values: { saturation: -100, contrast: 18, clarity: 10 } },
        { id: 'warm_film', fallback: 'Warm film', values: { temperature: 28, tint: 6, blacks: 18, contrast: -6, saturation: -5, vignette: -12 } },
        { id: 'cool_fade', fallback: 'Cool fade', values: { temperature: -25, blacks: 25, saturation: -18, contrast: -8, exposure: 5 } },
        { id: 'high_key', fallback: 'High key', values: { exposure: 22, highlights: -15, shadows: 20, contrast: -10, saturation: -8 } },
        { id: 'low_key', fallback: 'Low key', values: { exposure: -20, contrast: 18, shadows: -10, vignette: -20 } },
        { id: 'punch', fallback: 'Punch', values: { contrast: 28, vibrance: 30, clarity: 25, sharpness: 20, saturation: 8 } }
    ];

    // --- curves editor (the custom control of the Curves filter) ---

    const CURVE_PRESETS = [
        { id: 'linear', fallback: 'Linear', curves: { rgb: [[0, 0], [255, 255]] } },
        { id: 'contrast', fallback: 'Contrast', curves: { rgb: [[0, 0], [64, 48], [192, 208], [255, 255]] } },
        { id: 'strong_contrast', fallback: 'Strong contrast', curves: { rgb: [[0, 0], [64, 32], [192, 224], [255, 255]] } },
        { id: 'lighten', fallback: 'Lighten', curves: { rgb: [[0, 0], [128, 152], [255, 255]] } },
        { id: 'darken', fallback: 'Darken', curves: { rgb: [[0, 0], [128, 104], [255, 255]] } },
        { id: 'matte', fallback: 'Matte', curves: { rgb: [[0, 32], [255, 232]] } },
        { id: 'negative', fallback: 'Negative', curves: { rgb: [[0, 255], [255, 0]] } },
        { id: 'cross', fallback: 'Cross process', curves: { r: [[0, 0], [64, 40], [192, 226], [255, 255]], g: [[0, 0], [128, 138], [255, 255]], b: [[0, 40], [128, 124], [255, 214]] } }
    ];
    const CURVE_CHANNELS = [['rgb', 'RGB', '#e8eef7'], ['r', 'R', '#ff6b6b'], ['g', 'G', '#5be37d'], ['b', 'B', '#6aa7ff']];
    const flatCurve = () => [[0, 0], [255, 255]];

    function normalizeCurves(values) {
        const cv = values.curves && typeof values.curves === 'object' ? values.curves : (values.curves = {});
        CURVE_CHANNELS.forEach(([key]) => { if (!Array.isArray(cv[key]) || cv[key].length < 2) cv[key] = flatCurve(); });
        return cv;
    }

    function curvesControl(r) {
        return {
            render(values) {
                normalizeCurves(values);
                const chans = CURVE_CHANNELS.map(([key, text]) => `<button type="button" class="pixel-param-seg-btn${key === 'rgb' ? ' active' : ''}" data-curve-chan="${key}" aria-pressed="${key === 'rgb'}">${r.esc(text)}</button>`).join('');
                const presets = CURVE_PRESETS.map(p => `<option value="${p.id}">${r.esc(r.t('pixel.curve_preset_' + p.id, p.fallback))}</option>`).join('');
                return `<div class="pixel-fx-curves" data-fx-curves>
                    <div class="pixel-param-segmented" role="group">${chans}</div>
                    <canvas class="pixel-fx-curves-canvas" data-curve-canvas width="256" height="256"></canvas>
                    <div class="pixel-fx-curves-row">
                        <select class="pixel-select" data-curve-preset><option value="">${r.esc(r.t('pixel.curve_presets'))}</option>${presets}</select>
                        <button type="button" class="pixel-param-mini" data-curve-reset>${r.esc(r.t('pixel.reset'))}</button>
                    </div>
                    <p class="pixel-draw-hint">${r.esc(r.t('pixel.curve_hint'))}</p>
                </div>`;
            },
            wire(el, values, change) {
                const canvas = el.querySelector('[data-curve-canvas]');
                if (!canvas) return;
                const ctx = canvas.getContext('2d'), lib = window.AuraPixelLib && window.AuraPixelLib.fx;
                let chan = 'rgb', drag = -1;
                const pts = () => normalizeCurves(values)[chan];
                const draw = () => {
                    ctx.clearRect(0, 0, 256, 256);
                    ctx.fillStyle = 'rgba(0,0,0,0.35)';
                    ctx.fillRect(0, 0, 256, 256);
                    ctx.strokeStyle = 'rgba(255,255,255,0.14)';
                    ctx.lineWidth = 1;
                    ctx.beginPath();
                    [64, 128, 192].forEach(g => { ctx.moveTo(g + 0.5, 0); ctx.lineTo(g + 0.5, 256); ctx.moveTo(0, g + 0.5); ctx.lineTo(256, g + 0.5); });
                    ctx.moveTo(0, 256); ctx.lineTo(256, 0);
                    ctx.stroke();
                    const line = (points, color, width) => {
                        const lut = lib.splineLut(points);
                        ctx.strokeStyle = color;
                        ctx.lineWidth = width;
                        ctx.beginPath();
                        for (let x = 0; x < 256; x++) { if (x === 0) ctx.moveTo(x, 255 - lut[x]); else ctx.lineTo(x, 255 - lut[x]); }
                        ctx.stroke();
                    };
                    const cv = normalizeCurves(values);
                    CURVE_CHANNELS.forEach(([key, , color]) => { if (key !== chan) line(cv[key], color + '55', 1); });
                    const current = CURVE_CHANNELS.find(c => c[0] === chan);
                    line(cv[chan], current[2], 2);
                    ctx.fillStyle = current[2];
                    cv[chan].forEach(p => { ctx.beginPath(); ctx.arc(p[0], 255 - p[1], 4, 0, Math.PI * 2); ctx.fill(); });
                };
                const at = e => {
                    const rect = canvas.getBoundingClientRect(), k = rect.width > 0 ? 255 / rect.width : 1;
                    return { x: (e.clientX - rect.left) * k, y: 255 - (e.clientY - rect.top) * (rect.height > 0 ? 255 / rect.height : 1), tol: 9 * k };
                };
                const nearest = pos => {
                    let best = -1, bd = pos.tol * pos.tol;
                    pts().forEach((p, i) => { const d = (p[0] - pos.x) * (p[0] - pos.x) + (p[1] - pos.y) * (p[1] - pos.y); if (d <= bd) { bd = d; best = i; } });
                    return best;
                };
                const commit = () => { draw(); change(values.curves); };
                canvas.addEventListener('pointerdown', e => {
                    if (e.button !== 0) return;
                    const pos = at(e), list = pts();
                    let i = nearest(pos);
                    if (i < 0) {
                        const x = Math.round(Math.max(1, Math.min(254, pos.x)));
                        if (list.some(p => Math.abs(p[0] - x) < 2)) return;
                        list.push([x, Math.round(Math.max(0, Math.min(255, pos.y)))]);
                        list.sort((a, b) => a[0] - b[0]);
                        i = list.findIndex(p => p[0] === x);
                        commit();
                    }
                    drag = i;
                    try { canvas.setPointerCapture(e.pointerId); } catch (_) { /* synthetic pointer */ }
                    e.preventDefault();
                });
                canvas.addEventListener('pointermove', e => {
                    if (drag < 0) return;
                    const pos = at(e), list = pts(), p = list[drag];
                    if (!p) return;
                    p[1] = Math.round(Math.max(0, Math.min(255, pos.y)));
                    if (drag > 0 && drag < list.length - 1) p[0] = Math.round(Math.max(list[drag - 1][0] + 1, Math.min(list[drag + 1][0] - 1, pos.x)));
                    commit();
                });
                const stop = () => { drag = -1; };
                canvas.addEventListener('pointerup', stop);
                canvas.addEventListener('pointercancel', stop);
                const remove = e => {
                    e.preventDefault();
                    const list = pts(), i = nearest(at(e));
                    if (i > 0 && i < list.length - 1) { list.splice(i, 1); commit(); }
                };
                canvas.addEventListener('dblclick', remove);
                canvas.addEventListener('contextmenu', remove);
                el.querySelectorAll('[data-curve-chan]').forEach(btn => btn.addEventListener('click', () => {
                    chan = btn.dataset.curveChan;
                    el.querySelectorAll('[data-curve-chan]').forEach(b => { b.classList.toggle('active', b === btn); b.setAttribute('aria-pressed', b === btn ? 'true' : 'false'); });
                    draw();
                }));
                const select = el.querySelector('[data-curve-preset]');
                if (select) {
                    select.addEventListener('change', () => {
                        const preset = CURVE_PRESETS.find(p => p.id === select.value);
                        select.value = '';
                        if (!preset) return;
                        const next = {};
                        CURVE_CHANNELS.forEach(([key]) => { next[key] = (preset.curves[key] || flatCurve()).map(p => p.slice()); });
                        values.curves = next;
                        commit();
                    });
                }
                const reset = el.querySelector('[data-curve-reset]');
                if (reset) reset.addEventListener('click', () => { values.curves = { rgb: flatCurve(), r: flatCurve(), g: flatCurve(), b: flatCurve() }; commit(); });
                draw();
            }
        };
    }

    const CUSTOM_CONTROLS = { curves: curvesControl };

    // --- helpers on plain data ---

    const cloneParams = values => JSON.parse(JSON.stringify(values || {}));
    const isAbort = err => !!err && (err.name === 'AbortError' || /abort/i.test(String(err.message || '')));
    const toImageData = img => (typeof ImageData === 'function' ? new ImageData(img.data, img.width, img.height) : { width: img.width, height: img.height, data: img.data });

    // Nearest-neighbour resample of a document-sized selection mask to the proxy size.
    function resampleMask(alpha, sw, sh, tw, th) {
        const out = new Uint8ClampedArray(tw * th);
        for (let y = 0; y < th; y++) {
            const sy = Math.min(sh - 1, Math.floor((y + 0.5) * sh / th)) * sw;
            for (let x = 0; x < tw; x++) out[y * tw + x] = alpha[sy + Math.min(sw - 1, Math.floor((x + 0.5) * sw / tw))];
        }
        return out;
    }

    Pixel.installFilters = function installFilters(runtime) {
        const r = runtime;
        const lib = () => window.AuraPixelLib && window.AuraPixelLib.fx;
        const alive = () => !r.state.disposed;
        let sess = null;                 // the one live preview session { kind: 'filter' | 'adjust', layer, ... }
        let docStamp = 0;                // changes with the document and its history (thumbnail validity)
        let filterSearch = '';
        let thumbAbort = null, thumbBase = null, thumbBaseStamp = -1;
        let histCache = null, histTimer = 0, histData = { base: null, after: null, baseKey: '' };
        let histMode = 'rgb';
        let adjustHandles = [];
        let detailWire = null;
        const paramStore = new Map();    // filter id -> parameter values (kept while the window lives)
        const schemaCache = new Map();

        // --- compare (shared by both panels; the draggable divider lives in pixel-canvas.js / pixel-events.js) ---

        function enterCompare() {
            if (r.compareMode || !r.canvas.width) return;
            r.compareMode = true;
            r.comparePos = 0.5;
            r.compareOrigData = r.flattenDocument();
            if (r.compareDivider) r.compareDivider.hidden = false;
            r.renderCompare();
        }

        function leaveCompare() {
            if (!r.compareMode) return;
            r.compareMode = false;
            r.compareOrigData = null;
            r.filterCompareMode = false;
            if (r.compareDivider) r.compareDivider.hidden = true;
            syncCompareButtons();
            r.requestOverlay();
            r.compositeLayers();
        }

        function syncCompareButtons() {
            r.host.querySelectorAll('[data-action="compare-toggle"],[data-action="filter-compare"]').forEach(btn => btn.classList.toggle('active', !!r.compareMode));
        }

        // --- sources, masks, recipes ---

        function buildSource(layer) {
            const w = r.imgWidth, h = r.imgHeight, canvas = layer.canvas;
            if (w * h <= FULL_PREVIEW_PIXELS) {
                const full = r.imageFromCanvas(canvas);
                return { full, proxy: full, scale: 1 };
            }
            const s = PROXY_EDGE / Math.max(w, h), pw = Math.max(1, Math.round(w * s)), ph = Math.max(1, Math.round(h * s));
            const tmp = r.acquireTempCanvas(pw, ph), ctx = tmp.getContext('2d');
            ctx.imageSmoothingQuality = 'high';
            ctx.drawImage(canvas, 0, 0, pw, ph);
            const proxy = r.imageFromCanvas(tmp);
            r.releaseTempCanvas(tmp);
            return { full: null, proxy, scale: pw / w };
        }

        function ensureFull(s) {
            if (!s.src.full) s.src.full = r.imageFromCanvas(s.layer.canvas);
            return s.src.full;
        }

        // Selection alpha at the size of the image the job works on (null without a selection).
        function maskFor(s, tw, th) {
            if (!r.selection) return null;
            const m = r.getSelectionMask();
            if (!m) return new Uint8ClampedArray(tw * th);
            if (tw === s.w && th === s.h) return m.alpha;
            if (!s.maskCache || s.maskCache.source !== m || s.maskCache.w !== tw || s.maskCache.h !== th) {
                s.maskCache = { source: m, w: tw, h: th, alpha: resampleMask(m.alpha, s.w, s.h, tw, th) };
            }
            return s.maskCache.alpha;
        }

        function fadeOf() {
            const strength = Math.max(0, Math.min(100, Number(r.filterStrength)));
            const mode = r.filterBlend || 'normal';
            return strength >= 100 && mode === 'normal' ? null : { mode, strength: Number.isFinite(strength) ? strength : 100 };
        }

        function recipe(s) {
            if (s.kind === 'adjust') return { id: 'adjust', params: cloneParams(r.adjustments), fade: null };
            return { id: s.filterId, params: cloneParams(s.params), fade: fadeOf() };
        }

        // --- session ---

        function setBusy(s, busy, label) {
            r.host.querySelectorAll('[data-fx-status]').forEach(el => {
                el.textContent = busy ? (label || r.t('pixel.fx_rendering')) : '';
                el.classList.toggle('active', !!busy);
            });
            r.host.querySelectorAll('[data-action="apply-filter"],[data-action="apply-adjust"]').forEach(btn => { btn.disabled = !!(busy && s && s.applying); });
        }

        function clearTimers(s) {
            if (s.timer) clearTimeout(s.timer);
            if (s.fullTimer) clearTimeout(s.fullTimer);
            s.timer = s.fullTimer = 0;
        }

        function end(s) {
            if (!s) return;
            clearTimers(s);
            try { s.abort.abort(); } catch (_) { /* nothing to cancel */ }
            const wasFilter = sess === s && s.kind === 'filter';
            if (sess === s) {
                sess = null;
                r.filterPreview = null;
                r.adjustPreview = null;
                setBusy(null, false);
            }
            if (s.surface && s.layer.preview === s.surface) r.setLayerPreview(s.layer, null);
            s.surface = null;
            s.src = null;
            s.raw = s.rawProxy = null;
            if (wasFilter) {
                r.activeFilterId = null;
                r.filterStrength = 100;
                r.filterBlend = 'normal';
                renderFilterDetail();
            }
        }

        function begin(kind) {
            const layer = r.activeLayer();
            if (!layer || !r.imgWidth || !r.requireRasterLayer(layer)) return null;
            if (r.editMaskMode) r.leaveMaskEdit();
            if (layer.lockPixels) {
                r.notify({ type: 'info', message: r.t('pixel.layer_locked') });
                return null;
            }
            if (sess && (sess.kind !== kind || sess.layer !== layer || sess.w !== r.imgWidth || sess.h !== r.imgHeight || sess.rev !== layer.rev)) {
                // Another kind of edit starts: a pending adjustment is baked in (as the old panels did), a filter preview is dropped.
                if (sess.kind === 'adjust' && sess.layer === layer && sess.rev === layer.rev && r.hasPendingAdjustments()) commit(sess, 'adjust');
                else end(sess);
            }
            if (sess) return sess;
            const s = {
                kind, layer, rev: layer.rev, w: r.imgWidth, h: r.imgHeight, big: r.imgWidth * r.imgHeight > FULL_PREVIEW_PIXELS,
                src: buildSource(layer), seq: 0, pseq: 0, timer: 0, fullTimer: 0, surface: null, abort: new AbortController(),
                raw: null, rawFull: false, rawP: -1, rawProxy: null, rawProxyP: -1, shownFull: false, applying: 0, slow: false, filterId: null, params: null
            };
            sess = s;
            if (kind === 'filter') r.filterPreview = s; else r.adjustPreview = s;
            return s;
        }

        function toDocCanvas(s, img) {
            if (!s.surface) s.surface = r.makeCanvas(s.w, s.h);
            const ctx = s.surface.getContext('2d');
            if (img.width === s.w && img.height === s.h) {
                ctx.putImageData(toImageData(img), 0, 0);
            } else {
                const tmp = r.canvasFromImage(img);
                ctx.clearRect(0, 0, s.w, s.h);
                ctx.imageSmoothingEnabled = true;
                ctx.imageSmoothingQuality = 'high';
                ctx.drawImage(tmp, 0, 0, s.w, s.h);
            }
            return s.surface;
        }

        function showResult(s, img) {
            if (sess !== s) return;
            r.setLayerPreview(s.layer, toDocCanvas(s, img));
            if (r.compareMode) r.renderCompare();
            scheduleHistogram();
        }

        // Full-resolution render on the main thread: small documents (instant previews) and the commit fallback.
        function renderNow(s) {
            const full = ensureFull(s), rc = recipe(s), p = s.pseq;
            const res = lib().render({ image: full, id: rc.id, params: rc.params, scale: 1, fade: rc.fade, mask: maskFor(s, s.w, s.h), keepRaw: true }, {});
            s.raw = res.raw; s.rawFull = true; s.rawP = p;
            if (s.big) { s.rawProxy = null; }
            s.shownFull = true;
            showResult(s, res.image);
        }

        // Fade / strength changes reuse the cached filter result: only the blend runs again.
        function composeFromCache(s) {
            const rc = recipe(s), lb = lib();
            let base = null, raw = null;
            if (s.raw && s.rawFull && s.rawP === s.pseq) { base = ensureFull(s); raw = s.raw; }
            else if (s.rawProxy && s.rawProxyP === s.pseq) { base = s.src.proxy; raw = s.rawProxy; }
            if (!raw) return false;
            const out = lb.compose(base, raw, rc.fade, maskFor(s, base.width, base.height));
            s.shownFull = base.width === s.w && base.height === s.h;
            showResult(s, out);
            return true;
        }

        function runPreview(s, seq) {
            if (sess !== s || seq !== s.seq || !alive()) return;
            const rc = recipe(s), img = s.src.proxy, p = s.pseq;
            setBusy(s, true);
            r.runJob('fx.render', { image: img, id: rc.id, params: rc.params, scale: s.src.scale, fade: rc.fade, mask: maskFor(s, img.width, img.height), keepRaw: true }, { key: 'fx-preview', signal: s.abort.signal })
                .then(res => {
                    if (sess !== s || seq !== s.seq) return;
                    s.rawProxy = res.raw; s.rawProxyP = p;
                    if (!s.big) { s.raw = res.raw; s.rawFull = true; s.rawP = p; }
                    s.shownFull = !s.big;
                    showResult(s, res.image);
                    if (s.big) scheduleFull(s, seq); else setBusy(s, false);
                })
                .catch(err => { if (!isAbort(err)) { setBusy(s, false); r.notify({ type: 'error', message: String(err && err.message || err) }); } });
        }

        // Big documents: the proxy preview is shown first, the exact full-resolution render follows once the sliders rest.
        function scheduleFull(s, seq) {
            if (s.fullTimer) clearTimeout(s.fullTimer);
            s.fullTimer = setTimeout(() => {
                s.fullTimer = 0;
                if (sess !== s || seq !== s.seq || !alive()) return;
                const rc = recipe(s), full = ensureFull(s), p = s.pseq, keep = s.w * s.h <= 16000000;
                setBusy(s, true);
                r.runJob('fx.render', { image: full, id: rc.id, params: rc.params, scale: 1, fade: rc.fade, mask: maskFor(s, s.w, s.h), keepRaw: keep }, { key: 'fx-preview-full', signal: s.abort.signal })
                    .then(res => {
                        if (sess !== s || seq !== s.seq) return;
                        if (keep) { s.raw = res.raw; s.rawFull = true; s.rawP = p; }
                        s.shownFull = true;
                        showResult(s, res.image);
                        setBusy(s, false);
                    })
                    .catch(err => { if (!isAbort(err)) { setBusy(s, false); r.notify({ type: 'error', message: String(err && err.message || err) }); } });
            }, 450);
        }

        // opts: { immediate } skips the debounce (and runs on the main thread for small documents), { fadeOnly } reuses the cached result.
        function requestPreview(s, opts) {
            if (!s || sess !== s || s.applying || !alive()) return;
            const o = opts || {};
            s.seq++;
            clearTimers(s);
            if (o.fadeOnly && composeFromCache(s)) { if (s.big) scheduleFull(s, s.seq); return; }
            if (o.immediate && s.w * s.h <= SYNC_PIXELS) { renderNow(s); return; }
            s.timer = setTimeout(() => { s.timer = 0; runPreview(s, s.seq); }, o.immediate ? 0 : (s.slow ? 280 : 90));
        }

        // The full-resolution result that is written into the layer (without the selection: commitLayerPixels applies it).
        function finalImage(s) {
            const full = ensureFull(s), rc = recipe(s), lb = lib();
            const raw = s.raw && s.rawFull && s.rawP === s.pseq ? s.raw : lb.apply(full, rc.id, rc.params, { scale: 1 });
            return rc.fade ? lb.compose(full, raw, rc.fade, null) : raw;
        }

        function finish(s, img, label) {
            const canvas = r.canvasFromImage(img);
            if (r.compareMode) leaveCompare();
            const ok = r.commitLayerPixels(s.layer, canvas, { label });
            if (ok && s.kind === 'filter') r.lastFilter = { id: s.filterId, params: cloneParams(s.params), strength: r.filterStrength, blend: r.filterBlend };
            end(s);
            return ok;
        }

        function stillValid(s) {
            if (sess !== s) return false;
            if (!r.layers.includes(s.layer) || s.layer.rev !== s.rev || s.w !== r.imgWidth || s.h !== r.imgHeight) { end(s); return false; }
            return true;
        }

        // Bakes the preview into its layer. opts.async: big documents render in the worker with progress and the Promise resolves later.
        function commit(s, label, opts) {
            if (!stillValid(s)) return false;
            clearTimers(s);
            const cachedFull = s.raw && s.rawFull && s.rawP === s.pseq;
            if (opts && opts.async && s.big && !cachedFull) {
                const rc = recipe(s), token = ++s.applying;
                s.seq++;
                setBusy(s, true, r.t('pixel.fx_applying'));
                return r.runJob('fx.render', { image: ensureFull(s), id: rc.id, params: rc.params, scale: 1, fade: rc.fade, mask: null, keepRaw: false }, {
                    key: 'fx-apply', signal: s.abort.signal,
                    onProgress: f => r.host.querySelectorAll('[data-fx-status]').forEach(el => { el.textContent = r.t('pixel.fx_applying') + ' ' + Math.round(f * 100) + '%'; })
                }).then(res => (sess === s && s.applying === token && stillValid(s) ? finish(s, res.image, label) : false))
                    .catch(err => {
                        if (sess === s && s.applying === token) { s.applying = 0; setBusy(s, false); }
                        if (!isAbort(err)) r.notify({ type: 'error', message: String(err && err.message || err) });
                        return false;
                    });
            }
            return finish(s, finalImage(s), label);
        }

        // --- filters: parameters and the detail view ---

        function schemaFor(def) {
            if (!schemaCache.has(def.id)) {
                schemaCache.set(def.id, def.params.map(p => (p.factory && CUSTOM_CONTROLS[p.factory] ? Object.assign({}, p, CUSTOM_CONTROLS[p.factory](r)) : p)));
            }
            return schemaCache.get(def.id);
        }

        function paramsFor(def, given) {
            const base = r.defaultParams(schemaFor(def));
            if (given) {
                const merged = Object.assign(base, cloneParams(given));
                paramStore.set(def.id, merged);
                return merged;
            }
            if (paramStore.has(def.id)) return paramStore.get(def.id);
            def.params.forEach(p => {
                if (p.palette === 'primary') base[p.key] = r.primaryColor;
                else if (p.palette === 'secondary') base[p.key] = r.secondaryColor;
            });
            paramStore.set(def.id, base);
            return base;
        }

        const filterLabel = def => r.t(def.labelKey, def.fallback);
        const noop = () => {};

        function syncFadeControls() {
            const slider = r.host.querySelector('[data-filter-strength]'), value = r.host.querySelector('[data-filter-strength-val]'), blend = r.host.querySelector('[data-filter-blend]');
            if (slider) slider.value = String(r.filterStrength);
            if (value) value.textContent = Math.round(r.filterStrength) + '%';
            if (blend) blend.value = r.filterBlend || 'normal';
        }

        function renderFilterDetail() {
            const host = r.host, detail = host.querySelector('[data-filter-detail]');
            if (!detail) return;
            if (detailWire) { detailWire.dispose(); detailWire = null; }
            const s = sess && sess.kind === 'filter' ? sess : null;
            host.querySelectorAll('[data-filter-id]').forEach(card => card.classList.toggle('active', !!s && card.dataset.filterId === s.filterId));
            if (!s || !s.filterId) { detail.hidden = true; return; }
            const def = r.filterDef(s.filterId), schema = schemaFor(def);
            const title = host.querySelector('[data-filter-title]'), paramsEl = host.querySelector('[data-filter-params]'), star = host.querySelector('[data-filter-detail-fav]');
            if (title) title.textContent = filterLabel(def);
            if (star) star.textContent = (r.filterFavorites || []).includes(def.id) ? '★' : '☆';
            if (paramsEl) {
                paramsEl.innerHTML = schema.length ? r.buildParamControls(schema, s.params, { idPrefix: 'pixel-fx-' + snake(r.windowId) }) : `<p class="pixel-draw-hint">${r.esc(r.t('pixel.fx_no_options'))}</p>`;
                if (schema.length) detailWire = r.wireParamControls(paramsEl, schema, s.params, () => { s.pseq++; requestPreview(s, {}); });
            }
            syncFadeControls();
            syncCompareButtons();
            detail.hidden = false;
            if (detail.scrollIntoView) detail.scrollIntoView({ block: 'nearest' });
        }

        // The gallery shows the cards of the active category, the search text and the favourites chip.
        function applyGalleryFilter() {
            const favs = new Set(r.filterFavorites || []), q = filterSearch.trim().toLowerCase(), cat = r.activeFilterCat || 'all';
            let shown = 0;
            r.host.querySelectorAll('[data-filter-id]').forEach(card => {
                const def = r.filterDef(card.dataset.filterId);
                let ok = cat === 'all' || (cat === 'favorites' ? favs.has(def.id) : def.cat === cat);
                if (ok && q) ok = (filterLabel(def) + ' ' + def.id + ' ' + r.t('pixel.cat_' + def.cat, CATEGORY_NAMES[def.cat])).toLowerCase().includes(q);
                card.hidden = !ok;
                if (ok) shown++;
            });
            const empty = r.host.querySelector('[data-filter-empty]');
            if (empty) empty.hidden = shown > 0;
        }

        // --- thumbnails (rendered in the worker on a tiny copy of the document, only for the cards on screen) ---

        function sampleProxy(layer, edge) {
            const w = layer.canvas.width, h = layer.canvas.height, k = Math.min(1, edge / Math.max(w, h));
            const pw = Math.max(1, Math.round(w * k)), ph = Math.max(1, Math.round(h * k)), tmp = r.acquireTempCanvas(pw, ph), ctx = tmp.getContext('2d');
            ctx.imageSmoothingQuality = 'high';
            ctx.drawImage(layer.canvas, 0, 0, pw, ph);
            const img = r.imageFromCanvas(tmp);
            r.releaseTempCanvas(tmp);
            return img;
        }

        function thumbSource() {
            if (thumbBase && thumbBaseStamp === docStamp) return thumbBase;
            const flat = r.flattenDocument(), k = Math.min(1, THUMB_EDGE / Math.max(flat.width, flat.height));
            const tw = Math.max(1, Math.round(flat.width * k)), th = Math.max(1, Math.round(flat.height * k)), tmp = r.acquireTempCanvas(tw, th), ctx = tmp.getContext('2d');
            ctx.imageSmoothingQuality = 'high';
            ctx.drawImage(flat, 0, 0, tw, th);
            thumbBase = r.imageFromCanvas(tmp);
            thumbBaseStamp = docStamp;
            r.releaseTempCanvas(tmp);
            return thumbBase;
        }

        function refreshThumbnails() {
            const gallery = r.host.querySelector('[data-filter-gallery]');
            if (!gallery || !r.canvas.width || !lib()) return;
            const stamp = docStamp, cards = Array.from(gallery.querySelectorAll('[data-filter-id]')).filter(card => !card.hidden && card.dataset.thumbStamp !== String(stamp));
            if (!cards.length) return;
            if (thumbAbort) thumbAbort.abort();
            thumbAbort = new AbortController();
            const signal = thumbAbort.signal, base = thumbSource();
            cards.forEach(card => {
                const def = r.filterDef(card.dataset.filterId), canvas = card.querySelector('[data-filter-thumb]');
                if (!def || !canvas) return;
                const params = Object.assign(lib().defaults(def.id) || {}, def.thumb || {});
                r.runJob('fx.apply', { image: base, id: def.id, params, scale: THUMB_SCALE }, { signal }).then(res => {
                    if (!alive()) return;
                    canvas.width = res.image.width;
                    canvas.height = res.image.height;
                    canvas.getContext('2d').putImageData(toImageData(res.image), 0, 0);
                    card.dataset.thumbStamp = String(stamp);
                }).catch(noop);
            });
        }

        // --- histogram of the adjust panel ---

        function histSample(layer) {
            const key = layer.id + ':' + layer.rev + ':' + layer.canvas.width + 'x' + layer.canvas.height;
            if (!histCache || histCache.key !== key) histCache = { key, img: sampleProxy(layer, 256) };
            return histCache;
        }

        function drawHistogram() {
            const canvas = r.host.querySelector('[data-fx-hist]');
            if (!canvas || !canvas.getContext) return;
            const ctx = canvas.getContext('2d'), W = 256, H = 72;
            ctx.clearRect(0, 0, W, H);
            const sets = histMode === 'luma' ? [['l', '207,214,224']] : (histMode === 'rgb' ? [['r', '255,90,90'], ['g', '70,220,110'], ['b', '95,150,255']] : [[histMode, histMode === 'r' ? '255,90,90' : (histMode === 'g' ? '70,220,110' : '95,150,255')]]);
            const sources = [histData.base, histData.after].filter(Boolean);
            if (!sources.length) return;
            let peak = 1;
            sources.forEach(data => sets.forEach(([key]) => { const bins = data[key]; for (let i = 1; i < 255; i++) if (bins[i] > peak) peak = bins[i]; }));
            const trace = (bins, color, alpha, stroke) => {
                ctx.beginPath();
                ctx.moveTo(0, H);
                for (let i = 0; i < 256; i++) ctx.lineTo(i, H - Math.min(1, bins[i] / peak) * (H - 2));
                ctx.lineTo(255, H);
                ctx.closePath();
                if (stroke) { ctx.strokeStyle = 'rgba(' + color + ',' + alpha + ')'; ctx.lineWidth = 1; ctx.stroke(); } else { ctx.fillStyle = 'rgba(' + color + ',' + alpha + ')'; ctx.fill(); }
            };
            ctx.globalCompositeOperation = 'lighter';
            if (histData.after && histData.base) sets.forEach(([key, color]) => trace(histData.base[key], color, 0.5, true));
            sets.forEach(([key, color]) => trace((histData.after || histData.base)[key], color, histMode === 'rgb' ? 0.55 : 0.8, false));
            ctx.globalCompositeOperation = 'source-over';
        }

        function refreshHistogram() {
            if (!alive() || !r.canvas.width || !lib()) return;
            if (!r.host.querySelector('[data-fx-hist]')) return;
            if (r.activePanels && r.activePanels.inspector !== 'adjust') return;
            const layer = r.activeLayer();
            if (!layer || layer.sticker || !layer.canvas.width) return;
            const src = histSample(layer);
            if (histData.baseKey !== src.key) {
                histData.baseKey = src.key;
                r.runJob('fx.histogram', { image: src.img }, { key: 'fx-hist-base' }).then(res => { if (alive() && histData.baseKey === src.key) { histData.base = res; drawHistogram(); } }).catch(noop);
            }
            if (r.hasPendingAdjustments()) {
                r.runJob('fx.histogram', { image: src.img, id: 'adjust', params: cloneParams(r.adjustments), scale: 1 }, { key: 'fx-hist' }).then(res => { if (alive()) { histData.after = res; drawHistogram(); } }).catch(noop);
            } else {
                histData.after = null;
                drawHistogram();
            }
        }

        function scheduleHistogram() {
            if (histTimer || !alive()) return;
            histTimer = setTimeout(() => { histTimer = 0; refreshHistogram(); }, 90);
        }

        // --- adjust: sliders drive one session; the Apply button / next edit commits it ---

        function dropAdjust() {
            if (sess && sess.kind === 'adjust') end(sess);
        }

        function onAdjustChange() {
            if (!r.hasPendingAdjustments()) { dropAdjust(); scheduleHistogram(); return; }
            const s = begin('adjust');
            if (!s) { r.resetAdjustmentControls(); return; }
            s.pseq++;
            requestPreview(s, {});
            scheduleHistogram();
        }

        function setAdjustValues(values) {
            ADJUST_KEYS.forEach(key => { r.adjustments[key] = 0; });
            Object.keys(values || {}).forEach(key => { if (ADJUST_KEYS.includes(key)) r.adjustments[key] = values[key]; });
            adjustHandles.forEach(handle => handle.refresh());
            onAdjustChange();
        }

        // Invalidates a session whose layer changed underneath it (undo, another tool, a new document).
        function validate() {
            if (!sess) return;
            const kind = sess.kind;
            if (!stillValid(sess) && kind === 'adjust') r.resetAdjustmentControls();
        }

        function applyAdjustValues(label) {
            const s = sess;
            if (!s || s.kind !== 'adjust' || !r.hasPendingAdjustments()) return false;
            if (r.compareMode) leaveCompare();
            const ok = commit(s, label || 'adjust', { async: true });
            if (ok && typeof ok.then === 'function') return ok.then(done => { if (done) r.resetAdjustmentControls(); return done; });
            r.resetAdjustmentControls();
            return ok;
        }

        Object.assign(runtime, {
            filterBlend: 'normal',
            lastFilter: null,

            filterDef: Pixel.bindRuntime(runtime, function filterDef(id) {
                return Pixel.FILTERS.find(f => f.id === id) || null;
            }),
            loadFilterFavorites: Pixel.bindRuntime(runtime, function loadFilterFavorites() {
                try {
                    const stored = JSON.parse(localStorage.getItem(FAV_KEY) || '[]');
                    this.filterFavorites = Array.isArray(stored) ? stored.filter(id => typeof id === 'string') : [];
                } catch (_) { this.filterFavorites = []; }
            }),
            toggleFilterFavorite: Pixel.bindRuntime(runtime, function toggleFilterFavorite(id) {
                if (!id) return;
                const set = new Set(this.filterFavorites || []);
                if (set.has(id)) set.delete(id); else set.add(id);
                this.filterFavorites = Array.from(set);
                try { localStorage.setItem(FAV_KEY, JSON.stringify(this.filterFavorites)); } catch (_) { /* storage may be blocked */ }
                const fav = set.has(id);
                this.host.querySelectorAll('[data-filter-id]').forEach(card => {
                    if (card.dataset.filterId !== id) return;
                    card.classList.toggle('is-favorite', fav);
                    card.dataset.filterFav = fav ? '1' : '0';
                    const star = card.querySelector('[data-filter-fav-toggle]');
                    if (star) star.textContent = fav ? '★' : '☆';
                });
                const detailStar = this.host.querySelector('[data-filter-detail-fav]');
                if (detailStar && sess && sess.kind === 'filter' && sess.filterId === id) detailStar.textContent = fav ? '★' : '☆';
                if (this.activeFilterCat === 'favorites') applyGalleryFilter();
            }),
            toggleFilterCompare: Pixel.bindRuntime(runtime, function toggleFilterCompare() {
                if (!sess || sess.kind !== 'filter') return;
                this.filterCompareMode = !this.filterCompareMode;
                this.previewFilterBlend();
            }),
            // Selects a filter and starts its live preview. options: { params, strength, blend, silent }.
            selectFilter: Pixel.bindRuntime(runtime, function selectFilter(id, options) {
                const def = this.filterDef(id);
                if (!def || !this.canvas.width) return;
                if (!this.requireRasterLayer()) return;
                const s = begin('filter');
                if (!s) return;
                const o = options || {}, changed = s.filterId !== id;
                s.filterId = id;
                s.slow = def.preview === 'slow';
                s.params = paramsFor(def, o.params);
                if (o.strength != null) this.filterStrength = o.strength; else if (changed) this.filterStrength = 100;
                if (o.blend) this.filterBlend = o.blend; else if (changed) this.filterBlend = 'normal';
                s.pseq++;
                s.raw = s.rawProxy = null;
                s.rawFull = false;
                this.activeFilterId = id;
                if (changed || o.params) renderFilterDetail(); else syncFadeControls();
                if (!o.silent) requestPreview(s, { immediate: true });
            }),
            // Blend of the filtered result into the layer (strength, mode) and the before/after view: no filter recompute.
            previewFilterBlend: Pixel.bindRuntime(runtime, function previewFilterBlend() {
                const s = sess;
                if (!s || s.kind !== 'filter') return;
                if (this.filterCompareMode && !this.compareMode) enterCompare();
                else if (!this.filterCompareMode && this.compareMode) leaveCompare();
                syncCompareButtons();
                requestPreview(s, { immediate: true, fadeOnly: true });
            }),
            applyFilter: Pixel.bindRuntime(runtime, function applyFilter() {
                const s = sess;
                if (!s || s.kind !== 'filter') return false;
                return commit(s, 'filter:' + s.filterId, { async: true });
            }),
            resetFilterPreview: Pixel.bindRuntime(runtime, function resetFilterPreview() {
                if (!sess || sess.kind !== 'filter') return;
                if (this.compareMode) leaveCompare();
                end(sess);
            }),
            // Puts the parameters of the selected filter back to their defaults (the filter stays selected).
            resetFilterParams: Pixel.bindRuntime(runtime, function resetFilterParams() {
                const s = sess;
                if (!s || s.kind !== 'filter') return;
                paramStore.delete(s.filterId);
                this.filterStrength = 100;
                this.filterBlend = 'normal';
                this.selectFilter(s.filterId);
                renderFilterDetail();
            }),
            applyFilterPreview: Pixel.bindRuntime(runtime, function applyFilterPreview(name, options) {
                if (!this.canvas.width) return false;
                this.selectFilter(name, Object.assign({ strength: 100, blend: 'normal' }, options || {}));
                return this.applyFilter();
            }),
            // Opens the Filters panel with the filter selected (menu items, commands, "last filter with dialog").
            openFilter: Pixel.bindRuntime(runtime, function openFilter(id, options) {
                this.showPanel('filters');
                const def = this.filterDef(id);
                if (def && this.activeFilterCat !== 'all' && this.activeFilterCat !== def.cat && this.activeFilterCat !== 'favorites') { this.activeFilterCat = 'all'; this.host.querySelectorAll('[data-filter-cat]').forEach(chip => chip.classList.toggle('active', chip.dataset.filterCat === 'all')); applyGalleryFilter(); }
                this.selectFilter(id, options);
            }),
            // Ctrl+F: applies the last filter again with the same settings; withDialog reopens its panel instead.
            repeatLastFilter: Pixel.bindRuntime(runtime, function repeatLastFilter(withDialog) {
                const last = this.lastFilter;
                if (!this.canvas.width) return false;
                if (!last || !this.filterDef(last.id)) {
                    this.notify({ type: 'info', message: this.t('pixel.fx_no_last') });
                    return false;
                }
                this.commitPendingPreviews();
                if (withDialog) { this.openFilter(last.id, { params: last.params, strength: last.strength, blend: last.blend }); return true; }
                this.selectFilter(last.id, { params: last.params, strength: last.strength, blend: last.blend, silent: true });
                return this.applyFilter();
            }),
            refreshFilterThumbnails: Pixel.bindRuntime(runtime, function refreshFilterThumbnails() {
                refreshThumbnails();
            }),

            // --- adjust (the sliders of the Adjust panel; legacy entry points kept for the other modules) ---
            hasPendingAdjustments: Pixel.bindRuntime(runtime, function hasPendingAdjustments() {
                const lb = lib();
                return lb ? lb.adjustHasEffect(this.adjustments) : Object.keys(this.adjustments).some(k => Number(this.adjustments[k]) !== 0);
            }),
            captureAdjustmentSource: Pixel.bindRuntime(runtime, function captureAdjustmentSource() {
                validate();
            }),
            dropAdjustPreview: Pixel.bindRuntime(runtime, function dropAdjustPreview() {
                dropAdjust();
            }),
            applyAdjustmentsPreview: Pixel.bindRuntime(runtime, function applyAdjustmentsPreview() {
                if (!this.requireRasterLayer() || !this.canvas.width) return;
                if (!this.hasPendingAdjustments()) { dropAdjust(); return; }
                const s = begin('adjust');
                if (!s) return;
                s.pseq++;
                requestPreview(s, { immediate: true });
            }),
            applyAdjustments: Pixel.bindRuntime(runtime, function applyAdjustments() {
                if (!this.requireRasterLayer()) return false;
                return applyAdjustValues('adjust');
            }),
            resetAdjustmentControls: Pixel.bindRuntime(runtime, function resetAdjustmentControls() {
                Object.keys(this.adjustments).forEach(key => { this.adjustments[key] = 0; });
                adjustHandles.forEach(handle => handle.refresh());
                dropAdjust();
                scheduleHistogram();
            }),
            resetAdjustments: Pixel.bindRuntime(runtime, function resetAdjustments() {
                if (!this.requireRasterLayer()) return;
                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                this.resetAdjustmentControls();
            }),
            toggleCompare: Pixel.bindRuntime(runtime, function toggleCompare() {
                if (!this.requireRasterLayer() || !this.canvas.width) return;
                if (this.compareMode) { this.exitCompareMode({ preservePreview: true }); return; }
                if (this.hasPendingAdjustments()) this.applyAdjustmentsPreview();
                enterCompare();
                syncCompareButtons();
            }),
            exitCompareMode: Pixel.bindRuntime(runtime, function exitCompareMode(options) {
                if (!this.compareMode) return;
                leaveCompare();
                if (!(options && options.preservePreview) && sess) end(sess);
            }),
            // Bakes a pending adjust or filter preview into its layer so the next edit starts from what is on screen.
            commitPendingPreviews: Pixel.bindRuntime(runtime, function commitPendingPreviews() {
                if (this.compareMode) this.exitCompareMode({ preservePreview: true });
                const s = sess;
                if (!s) return false;
                if (s.kind === 'adjust' && !this.hasPendingAdjustments()) { end(s); return false; }
                if (!stillValid(s)) return false;
                const ok = commit(s, s.kind === 'filter' ? 'filter:' + s.filterId : 'adjust');
                if (s.kind === 'adjust') this.resetAdjustmentControls();
                return !!ok;
            }),
            // Pending adjust/filter changes are thrown away (layer about to disappear or be replaced).
            discardPendingPreviews: Pixel.bindRuntime(runtime, function discardPendingPreviews() {
                this.resetFilterPreview();
                if (this.compareMode) this.exitCompareMode({ preservePreview: false });
                this.resetAdjustmentControls();
            }),
            // One-click enhance from fx.autoTone: levels + contrast + white balance + mild vibrance on the sliders.
            // options.apply writes it straight into the layer, otherwise the sliders show it as a preview.
            autoEnhance: Pixel.bindRuntime(runtime, function autoEnhance(options) {
                if (!this.canvas.width || !this.requireRasterLayer()) return Promise.resolve(false);
                this.commitPendingPreviews();
                const layer = this.activeLayer();
                if (!layer || layer.lockPixels) return Promise.resolve(false);
                const img = sampleProxy(layer, 512), apply = !!(options && options.apply);
                return this.runJob('fx.autoTone', { image: img }, { key: 'fx-auto' }).then(res => {
                    if (!alive() || this.activeLayer() !== layer || layer.rev === undefined) return false;
                    setAdjustValues(res.params);
                    if (!apply) return true;
                    const s = sess;
                    if (!s) return false;
                    return applyAdjustValues('auto-enhance');
                }).catch(err => { if (!isAbort(err)) this.notify({ type: 'error', message: String(err && err.message || err) }); return false; });
            }),
            refreshAdjustHistogram: Pixel.bindRuntime(runtime, function refreshAdjustHistogram() {
                scheduleHistogram();
            })
        });

        ADJUST_KEYS.forEach(key => { if (runtime.adjustments[key] === undefined) runtime.adjustments[key] = 0; });

        // --- panel wiring ---

        function mountFilters(el) {
            r.loadFilterFavorites();
            el.addEventListener('click', e => {
                const target = e.target;
                if (!target || !target.closest) return;
                const star = target.closest('[data-filter-fav-toggle]');
                if (star) { e.stopPropagation(); r.toggleFilterFavorite(star.dataset.filterFavToggle); return; }
                const detailStar = target.closest('[data-filter-detail-fav]');
                if (detailStar) { if (sess && sess.kind === 'filter') r.toggleFilterFavorite(sess.filterId); return; }
                const chip = target.closest('[data-filter-cat]');
                if (chip) {
                    r.activeFilterCat = chip.dataset.filterCat;
                    el.querySelectorAll('[data-filter-cat]').forEach(b => b.classList.toggle('active', b === chip));
                    applyGalleryFilter();
                    refreshThumbnails();
                    return;
                }
                const card = target.closest('[data-filter-id]');
                if (card) { r.selectFilter(card.dataset.filterId); return; }
                const action = target.closest('[data-action]');
                if (!action) return;
                switch (action.dataset.action) {
                    case 'apply-filter': r.applyFilter(); break;
                    case 'reset-filter': r.resetFilterParams(); break;
                    case 'cancel-filter': r.resetFilterPreview(); break;
                    case 'filter-compare': r.toggleFilterCompare(); break;
                    default: break;
                }
            });
            const search = el.querySelector('[data-filter-search]');
            if (search) {
                let timer = 0;
                search.addEventListener('input', () => {
                    filterSearch = search.value;
                    applyGalleryFilter();
                    if (timer) clearTimeout(timer);
                    timer = setTimeout(() => { timer = 0; if (alive()) refreshThumbnails(); }, 200);
                });
            }
            let fadeFrame = 0;
            const fadeNow = () => { fadeFrame = 0; if (alive()) r.previewFilterBlend(); };
            const slider = el.querySelector('[data-filter-strength]');
            if (slider) {
                slider.addEventListener('input', () => {
                    r.filterStrength = Number(slider.value);
                    const val = el.querySelector('[data-filter-strength-val]');
                    if (val) val.textContent = Math.round(r.filterStrength) + '%';
                    if (!fadeFrame) fadeFrame = requestAnimationFrame(fadeNow);
                });
                slider.addEventListener('dblclick', () => { slider.value = '100'; slider.dispatchEvent(new Event('input')); });
            }
            const blend = el.querySelector('[data-filter-blend]');
            if (blend) blend.addEventListener('change', () => { r.filterBlend = blend.value; r.previewFilterBlend(); });
            applyGalleryFilter();
        }

        function mountAdjust(el) {
            ADJUST_GROUPS.forEach(group => {
                const root = el.querySelector('[data-adjust-group="' + group.id + '"] [data-param-root]');
                if (!root) return;
                const handle = r.wireParamControls(root, adjustSchema(group.keys), r.adjustments, onAdjustChange);
                if (handle) adjustHandles.push(handle);
            });
            el.addEventListener('click', e => {
                const target = e.target;
                if (!target || !target.closest) return;
                const mode = target.closest('[data-hist-mode]');
                if (mode) {
                    histMode = mode.dataset.histMode;
                    el.querySelectorAll('[data-hist-mode]').forEach(b => b.classList.toggle('active', b === mode));
                    drawHistogram();
                    return;
                }
                const action = target.closest('[data-action]');
                if (!action) return;
                switch (action.dataset.action) {
                    case 'apply-adjust': r.applyAdjustments(); break;
                    case 'reset-adjust': r.resetAdjustments(); break;
                    case 'compare-toggle': r.toggleCompare(); break;
                    case 'adjust-auto': r.autoEnhance({ apply: false }); break;
                    default: break;
                }
            });
            const presets = el.querySelector('[data-adjust-preset]');
            if (presets) {
                presets.addEventListener('change', () => {
                    const preset = ADJUST_PRESETS.find(p => p.id === presets.value);
                    presets.value = '';
                    if (preset) setAdjustValues(preset.values);
                });
            }
            drawHistogram();
        }

        function buildFilters() {
            if (!r.filterFavorites || !r.filterFavorites.length) r.loadFilterFavorites();
            const favs = new Set(r.filterFavorites || []), t = (key, fallback) => r.esc(r.t(key, fallback));
            const cats = ['all', 'favorites'].concat(Pixel.FILTER_CATEGORIES);
            const chips = cats.map(cat => `<button class="pixel-chip${(r.activeFilterCat || 'all') === cat ? ' active' : ''}" type="button" data-filter-cat="${cat}">${t('pixel.cat_' + cat, CATEGORY_NAMES[cat] || cat)}</button>`).join('');
            const cards = Pixel.FILTERS.map(def => {
                const fav = favs.has(def.id), name = r.esc(filterLabel(def));
                return `<button class="pixel-filter-card${fav ? ' is-favorite' : ''}" type="button" data-filter-id="${r.esc(def.id)}" data-filter-card-cat="${def.cat}" data-filter-fav="${fav ? '1' : '0'}" title="${name}">
                    <span class="pixel-filter-fav" data-filter-fav-toggle="${r.esc(def.id)}" title="${t('pixel.filter_favorite')}">${fav ? '★' : '☆'}</span>
                    <canvas class="pixel-filter-thumb" data-filter-thumb="${r.esc(def.id)}" width="${THUMB_EDGE}" height="64"></canvas>
                    <span class="pixel-filter-name">${name}</span>
                </button>`;
            }).join('');
            const blends = (window.AuraPixelLib && window.AuraPixelLib.fx ? window.AuraPixelLib.fx.fadeModes : ['normal']).map(mode => `<option value="${mode}">${t('pixel.fxo_' + snake(mode), OPTION_NAMES[snake(mode)] || mode)}</option>`).join('');
            return `<input type="search" class="pixel-fx-search" data-filter-search placeholder="${t('pixel.fx_search')}" aria-label="${t('pixel.fx_search')}" autocomplete="off" spellcheck="false">
                <div class="pixel-filter-chips" data-filter-chips>${chips}</div>
                <div class="pixel-filter-gallery pixel-fx-gallery" data-filter-gallery>${cards}</div>
                <p class="pixel-draw-hint" data-filter-empty hidden>${t('pixel.fx_no_match')}</p>
                <div class="pixel-fx-detail" data-filter-detail hidden>
                    <div class="pixel-fx-detail-head"><strong class="pixel-fx-title" data-filter-title></strong><button class="pixel-fx-star" type="button" data-filter-detail-fav title="${t('pixel.filter_favorite')}">☆</button></div>
                    <div class="pixel-fx-params" data-filter-params></div>
                    <div class="pixel-fx-fade">
                        <div class="pixel-slider-row"><span class="pixel-slider-label">${t('pixel.filter_strength')}</span><input type="range" class="pixel-slider" data-filter-strength min="0" max="100" value="100"><span class="pixel-slider-value" data-filter-strength-val>100%</span></div>
                        <div class="pixel-slider-row"><span class="pixel-slider-label">${t('pixel.fx_blend')}</span><select class="pixel-select" data-filter-blend>${blends}</select></div>
                    </div>
                    <div class="pixel-fx-status" data-fx-status role="status"></div>
                    <div class="pixel-panel-actions">
                        <button class="pixel-btn" type="button" data-action="filter-compare">${t('pixel.filter_compare')}</button>
                        <button class="pixel-btn" type="button" data-action="reset-filter">${t('pixel.reset')}</button>
                        <button class="pixel-btn" type="button" data-action="cancel-filter">${t('pixel.cancel')}</button>
                    </div>
                    <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-filter">${t('pixel.apply_filter')}</button></div>
                </div>`;
        }

        function buildAdjust() {
            const t = (key, fallback) => r.esc(r.t(key, fallback));
            const prefix = 'pixel-adj-' + snake(r.windowId);
            const groups = ADJUST_GROUPS.map(group => `<div class="pixel-fx-group" data-adjust-group="${group.id}">
                    <div class="pixel-fx-group-title">${t(group.labelKey, group.fallback)}</div>
                    ${r.buildParamControls(adjustSchema(group.keys), r.adjustments, { idPrefix: prefix + '-' + group.id })}
                </div>`).join('');
            const modes = [['rgb', 'RGB'], ['luma', 'Luma'], ['r', 'R'], ['g', 'G'], ['b', 'B']].map(([id, text]) => `<button type="button" class="pixel-param-seg-btn${id === histMode ? ' active' : ''}" data-hist-mode="${id}">${r.esc(text)}</button>`).join('');
            const presets = ADJUST_PRESETS.map(p => `<option value="${p.id}">${t('pixel.adjust_preset_' + p.id, p.fallback)}</option>`).join('');
            return `<div class="pixel-fx-hist-wrap"><canvas class="pixel-fx-hist" data-fx-hist width="256" height="72" aria-label="${t('pixel.histogram')}"></canvas><div class="pixel-param-segmented pixel-fx-hist-modes" role="group">${modes}</div></div>
                <div class="pixel-fx-auto-row">
                    <button class="pixel-btn" type="button" data-action="adjust-auto" title="${t('pixel.adjust_auto_hint')}">${t('pixel.adjust_auto')}</button>
                    <select class="pixel-select" data-adjust-preset aria-label="${t('pixel.adjust_presets')}"><option value="">${t('pixel.adjust_presets')}</option>${presets}</select>
                </div>
                ${groups}
                <div class="pixel-fx-status" data-fx-status role="status"></div>
                <div class="pixel-panel-actions"><button class="pixel-btn pixel-btn-primary" type="button" data-action="apply-adjust">${t('pixel.apply')}</button><button class="pixel-btn" type="button" data-action="reset-adjust">${t('pixel.reset')}</button><button class="pixel-btn" type="button" data-action="compare-toggle">${t('pixel.compare')}</button></div>`;
        }

        // Panel registrations call these through the runtime so every window has its own state.
        Object.assign(runtime, {
            buildFilterPanelHTML: Pixel.bindRuntime(runtime, function buildFilterPanelHTML() { return buildFilters(); }),
            mountFilterPanelSection: Pixel.bindRuntime(runtime, function mountFilterPanelSection(el) { mountFilters(el); }),
            buildAdjustPanelHTML: Pixel.bindRuntime(runtime, function buildAdjustPanelHTML() { return buildAdjust(); }),
            mountAdjustPanelSection: Pixel.bindRuntime(runtime, function mountAdjustPanelSection(el) { mountAdjust(el); }),
            // Names the built-in panel code of pixel-view.js and older modules still call.
            buildFilterGalleryHTML: Pixel.bindRuntime(runtime, function buildFilterGalleryHTML() { return buildFilters(); }),
            mountFilterPanel: Pixel.bindRuntime(runtime, function mountFilterPanel(el) { mountFilters(el); }),
            wireFilterPanel: Pixel.bindRuntime(runtime, function wireFilterPanel() { this.loadFilterFavorites(); applyGalleryFilter(); })
        });

        // --- events ---

        // A new document or a history step makes the thumbnails stale; the preview of the old layers is gone with them.
        function onHistory() {
            docStamp++;
            if (r.host.querySelector('[data-filter-gallery]') && r.activePanels && r.activePanels.inspector === 'filters') {
                if (r._thumbTimer) clearTimeout(r._thumbTimer);
                r._thumbTimer = setTimeout(() => { r._thumbTimer = 0; if (alive()) refreshThumbnails(); }, 400);
            }
        }
        runtime.on('history-changed', onHistory);
        runtime.on('document-changed', () => { onHistory(); histData = { base: null, after: null, baseKey: '' }; validate(); scheduleHistogram(); });
        runtime.on('layers-changed', () => { validate(); scheduleHistogram(); });
        runtime.on('selection-changed', () => { if (sess) requestPreview(sess, {}); });
        runtime.on('active-layer-changed', () => {
            const s = sess;
            if (!s || r.activeLayer() === s.layer) { scheduleHistogram(); return; }
            // Switching layers (as the old layers panel did): a pending adjustment is baked in, a filter preview dropped.
            if (r.compareMode) leaveCompare();
            if (s.kind === 'adjust' && r.hasPendingAdjustments() && stillValid(s)) { commit(s, 'adjust'); r.resetAdjustmentControls(); } else { end(s); if (s.kind === 'adjust') r.resetAdjustmentControls(); }
            scheduleHistogram();
        });
    };

    // --- registrations: panels (replace the built-in ones of pixel-view.js), menus and commands ---

    Pixel.registerPanel({ id: 'adjust',
        build: r => r.buildAdjustPanelHTML(),
        mount: (r, el) => r.mountAdjustPanelSection(el),
        onShow: r => { r.refreshAdjustHistogram(); },
        onHide: r => { if (r.compareMode) r.exitCompareMode({ preservePreview: true }); }
    });
    Pixel.registerPanel({ id: 'filters',
        build: r => r.buildFilterPanelHTML(),
        mount: (r, el) => r.mountFilterPanelSection(el),
        onShow: r => r.refreshFilterThumbnails(),
        onHide: r => r.resetFilterPreview()
    });

    // The Filter menu: repeat / dialog / auto enhance, then one submenu per category. Items carry ids, so every filter is also
    // a command (menu:filter:cat-<category>/f-<id>) for the command palette.
    function filterMenuItems(r) {
        const t = (key, fallback) => r.t(key, fallback);
        const head = [
            { id: 'repeat-last', labelKey: 'pixel.fx_repeat_last', label: t('pixel.fx_repeat_last'), shortcut: 'Ctrl+F', keywords: ['filter', 'again'], action: rt => rt.repeatLastFilter(false) },
            { id: 'last-dialog', labelKey: 'pixel.fx_last_dialog', label: t('pixel.fx_last_dialog'), shortcut: 'Ctrl+Alt+F', keywords: ['filter', 'dialog'], action: rt => rt.repeatLastFilter(true) },
            { id: 'auto-enhance', labelKey: 'pixel.fx_auto_enhance', label: t('pixel.fx_auto_enhance'), shortcut: 'Ctrl+Shift+L', keywords: ['auto', 'levels', 'contrast', 'white balance'], action: rt => rt.autoEnhance({ apply: true }) },
            { type: 'separator' }
        ];
        const groups = Pixel.FILTER_CATEGORIES.map(cat => ({
            id: 'cat-' + cat, labelKey: 'pixel.cat_' + cat, label: t('pixel.cat_' + cat, CATEGORY_NAMES[cat]),
            items: Pixel.FILTERS.filter(def => def.cat === cat).map(def => ({
                id: 'f-' + def.id, labelKey: def.labelKey, label: t(def.labelKey, def.fallback), keywords: ['filter', def.id, def.cat],
                action: rt => rt.openFilter(def.id)
            }))
        }));
        return head.concat(groups);
    }
    Pixel.registerMenuItems('filter', filterMenuItems, { order: 100 });
})();
