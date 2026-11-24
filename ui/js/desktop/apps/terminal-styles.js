(function () {
    'use strict';

    const STORAGE_KEY = 'aurago.desktop.terminal.style';
    const EFFECTS_KEY = 'aurago.desktop.terminal.effects.v1';
    // Store percentages per style; scale converts controls to shader units.
    const EFFECTS = [
        { key: 'intensity', value: 100 }, { key: 'brightness', value: 100, min: 50, max: 150 },
        { key: 'bloom' }, { key: 'scan' }, { key: 'curve', scale: 0.22, advanced: true },
        { key: 'burn', advanced: true }, { key: 'mask', advanced: true },
        { key: 'vignette', value: 48 }, { key: 'reflection', value: 16 },
        { key: 'noise', scale: 0.25, advanced: true }, { key: 'flicker', scale: 0.06, advanced: true },
        { key: 'jitter', value: 0, advanced: true }, { key: 'interference', value: 0, advanced: true },
        { key: 'chromatic', value: 0, advanced: true }
    ];
    const IDS = [
        'modern', 'amber', 'green', 'apple2', 'commodore64',
        'ibm3278', 'vintage', 'mono-green', 'transparent-green', 'color-monitor'
    ];

    const MONO = 'ui-monospace, "Cascadia Mono", "SF Mono", SFMono-Regular, Menlo, Consolas, "DejaVu Sans Mono", "Liberation Mono", monospace';
    // Modern: a curated 16-colour palette that stays readable on every desktop theme's dark terminal background.
    const MODERN_BG = '#0f172a';
    const MODERN_THEME = {
        background: MODERN_BG, foreground: '#e2e8f0', cursor: '#93c5fd', cursorAccent: MODERN_BG, selectionBackground: '#1e3a5f',
        // Normal colours sit one step deeper so they also work as backgrounds (white on blue bars); bright ones stay vivid.
        black: '#334155', red: '#ef4444', green: '#22c55e', yellow: '#eab308',
        blue: '#3b82f6', magenta: '#b472f8', cyan: '#06b6d4', white: '#cbd5e1',
        brightBlack: '#7b8ba3', brightRed: '#f87171', brightGreen: '#4ade80', brightYellow: '#fde047',
        brightBlue: '#60a5fa', brightMagenta: '#c084fc', brightCyan: '#67e8f9', brightWhite: '#f8fafc'
    };
    const PIXEL = '"Press Start 2P", "Geist Mono", ui-monospace, monospace';
    const GEIST = '"Aura Terminal", "Geist Mono", ui-monospace, monospace';
    const VGA = '"Aura VGA", "Aura Terminal", ui-monospace, monospace';

    // The 16 CGA colours, as a classic PC colour monitor shows them.
    const CGA = {
        black: '#000000', red: '#aa0000', green: '#00aa00', yellow: '#aa5500',
        blue: '#0000aa', magenta: '#aa00aa', cyan: '#00aaaa', white: '#aaaaaa',
        brightBlack: '#555555', brightRed: '#ff5555', brightGreen: '#55ff55', brightYellow: '#ffff55',
        brightBlue: '#5555ff', brightMagenta: '#ff55ff', brightCyan: '#55ffff', brightWhite: '#ffffff'
    };

    const PROFILES = {
        modern: {
            id: 'modern',
            retro: false,
            fontFamily: MONO,
            fontSize: 13,
            lineHeight: 1.2,
            theme: MODERN_THEME,
            crt: { phosphor: [0.89, 0.91, 0.94], curve: 0, bloom: 0, burn: 0, noise: 0, flicker: 0, mask: 0, alpha: 1, scan: 0 }
        },
        amber: {
            id: 'amber',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#120c04', foreground: '#ffb000', cursor: '#ffd27a', selectionBackground: '#5a3a10' },
            crt: { phosphor: [1.0, 0.66, 0.22], curve: 0.1, bloom: 0.65, burn: 0.28, noise: 0.065, flicker: 0.012, mask: 0.22, alpha: 1, scan: 0.72 }
        },
        green: {
            id: 'green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#020802', foreground: '#5fff3a', cursor: '#9cff7a', selectionBackground: '#14580c' },
            crt: { phosphor: [0.38, 1.0, 0.32], curve: 0.1, bloom: 0.58, burn: 0.24, noise: 0.05, flicker: 0.01, mask: 0.2, alpha: 1, scan: 0.68 }
        },
        apple2: {
            id: 'apple2',
            retro: true,
            fontFamily: PIXEL,
            fontSize: 12,
            lineHeight: 1.35,
            theme: { background: '#001400', foreground: '#33ff33', cursor: '#66ff66', selectionBackground: '#0b4a0b' },
            crt: { phosphor: [0.35, 1.0, 0.28], curve: 0.07, bloom: 0.35, burn: 0.18, noise: 0.055, flicker: 0.008, mask: 0.15, alpha: 1, scan: 0.8 }
        },
        commodore64: {
            id: 'commodore64',
            retro: true,
            color: true,
            fontFamily: PIXEL,
            fontSize: 13,
            lineHeight: 1.3,
            theme: { background: '#241b59', foreground: '#a99aff', cursor: '#c4baff', selectionBackground: '#5040a0' },
            crt: { phosphor: [0.65, 0.59, 1.0], curve: 0.08, bloom: 0.24, burn: 0.12, noise: 0.035, flicker: 0.008, mask: 0.2, alpha: 1, scan: 0.55 }
        },
        ibm3278: {
            id: 'ibm3278',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.1,
            theme: { background: '#010301', foreground: '#2adf2a', cursor: '#7cff7c', selectionBackground: '#0d3d0d' },
            crt: { phosphor: [0.25, 1.0, 0.3], curve: 0.06, bloom: 0.32, burn: 0.14, noise: 0.025, flicker: 0.006, mask: 0.18, alpha: 1, scan: 0.52 }
        },
        vintage: {
            id: 'vintage',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.2,
            theme: { background: '#0a0804', foreground: '#d4b06a', cursor: '#f0d090', selectionBackground: '#4a3818' },
            crt: { phosphor: [1.0, 0.73, 0.39], curve: 0.14, bloom: 0.85, burn: 0.46, noise: 0.12, flicker: 0.023, mask: 0.32, alpha: 1, scan: 0.85 }
        },
        'mono-green': {
            id: 'mono-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.12,
            theme: { background: '#000000', foreground: '#00ff66', cursor: '#9dffc0', selectionBackground: '#00331a' },
            crt: { phosphor: [0.18, 1.0, 0.48], curve: 0.07, bloom: 0.38, burn: 0.18, noise: 0.03, flicker: 0.008, mask: 0.25, alpha: 1, scan: 0.78 }
        },
        'transparent-green': {
            id: 'transparent-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#001108', foreground: '#7CFF6A', cursor: '#b6ffaa', selectionBackground: '#145820' },
            crt: { phosphor: [0.49, 1.0, 0.42], curve: 0.09, bloom: 0.52, burn: 0.2, noise: 0.035, flicker: 0.01, mask: 0.2, alpha: 0.82, scan: 0.65 }
        },
        'color-monitor': {
            id: 'color-monitor',
            retro: true,
            color: true,
            fontFamily: VGA,
            fontSize: 16,
            lineHeight: 1,
            theme: Object.assign({ background: '#000000', foreground: '#aaaaaa', cursor: '#ffffff', cursorAccent: '#000000', selectionBackground: '#0000aa', selectionForeground: '#ffffff' }, CGA),
            crt: { phosphor: [0.92, 0.95, 1.0], curve: 0.1, bloom: 0.42, burn: 0.1, noise: 0.03, flicker: 0.006, mask: 0.45, alpha: 1, scan: 0.5, triad: 1 }
        }
    };

    function normalize(value) {
        const id = String(value || '').trim();
        return IDS.indexOf(id) >= 0 ? id : 'modern';
    }

    function load() {
        try {
            return normalize(window.localStorage.getItem(STORAGE_KEY));
        } catch (e) {
            return 'modern';
        }
    }

    function save(id) {
        const next = normalize(id);
        try {
            window.localStorage.setItem(STORAGE_KEY, next);
        } catch (e) {}
        return next;
    }

    function normalizeEffects(id, values) {
        const base = PROFILES[normalize(id)].crt;
        const result = {};
        EFFECTS.forEach(function (effect) {
            const value = values && Object.prototype.hasOwnProperty.call(values, effect.key) ? values[effect.key] : undefined;
            const fallback = effect.value === undefined ? Math.round(base[effect.key] / (effect.scale || 1) * 100) : effect.value;
            result[effect.key] = typeof value === 'number' && Number.isFinite(value)
                ? Math.round(Math.max(effect.min || 0, Math.min(effect.max || 100, value))) : fallback;
        });
        return result;
    }

    function readEffects() {
        try {
            const saved = JSON.parse(window.localStorage.getItem(EFFECTS_KEY));
            if (saved && typeof saved === 'object' && !Array.isArray(saved)) return saved;
        } catch (_) {}
        return {};
    }

    function loadEffects(id) {
        return normalizeEffects(id, readEffects()[normalize(id)]);
    }

    function saveEffects(id, values) {
        const next = normalizeEffects(id, values);
        const saved = readEffects();
        const clean = {};
        IDS.forEach(function (style) {
            if (Object.prototype.hasOwnProperty.call(saved, style)) clean[style] = normalizeEffects(style, saved[style]);
        });
        clean[normalize(id)] = next;
        try { window.localStorage.setItem(EFFECTS_KEY, JSON.stringify(clean)); } catch (_) {}
        return next;
    }

    function resetEffects(id) {
        return saveEffects(id, null);
    }

    function profile(id, effects) {
        const base = PROFILES[normalize(id)];
        const values = effects ? normalizeEffects(id, effects) : loadEffects(id);
        const crt = Object.assign({}, base.crt);
        EFFECTS.forEach(function (effect) { crt[effect.key] = values[effect.key] / 100 * (effect.scale || 1); });
        return Object.assign({}, base, { crt: crt });
    }

    function applyXterm(term, nextProfile) {
        if (!term || !nextProfile) return;
        const profile = nextProfile.id ? nextProfile : window.TerminalStyles.profile(nextProfile);
        try {
            term.options.fontFamily = profile.fontFamily;
            term.options.fontSize = profile.fontSize;
            term.options.lineHeight = profile.lineHeight;
            term.options.theme = profile.theme;
        } catch (e) {}
    }

    // ---- Toolbar icons and markup (16x16 line icons, inline, decorative) ----

    const ICONS = {
        monitor: '<rect x="2" y="2.5" width="12" height="8.5" rx="1.5"/><path d="M5.5 13.5h5M8 11v2.5"/>',
        gauge: '<path d="M2.5 12a5.5 5.5 0 1 1 11 0"/><path d="M8 12l2.6-3.6"/><path d="M2.5 12h1.2M12.3 12h1.2"/>',
        book: '<rect x="3" y="2" width="10" height="12" rx="1.5"/><path d="M6 5.5h4M6 8.5h4"/>',
        hangup: '<path d="M1.8 9.6C3.6 7.4 5.7 6.5 8 6.5s4.4.9 6.2 3.1c.3.4.2.9-.2 1.2l-1.5 1c-.4.3-.9.1-1.1-.3l-.5-1.3c-.1-.3-.4-.5-.7-.6-1.4-.4-2.8-.4-4.2 0-.3.1-.6.3-.7.6l-.5 1.3c-.2.4-.7.6-1.1.3l-1.5-1c-.4-.3-.5-.8-.2-1.2z"/>',
        sliders: '<path d="M2.5 5h1.9M7.6 5h5.9M2.5 11h6.4M12.1 11h1.4"/><circle cx="6" cy="5" r="1.6"/><circle cx="10.5" cy="11" r="1.6"/>',
        soundOn: '<path d="M2.5 6.5h2L8 3.8v8.4L4.5 9.5h-2z"/><path d="M10.4 6a3 3 0 0 1 0 4M12.3 4.3a5.6 5.6 0 0 1 0 7.4"/>',
        soundOff: '<path d="M2.5 6.5h2L8 3.8v8.4L4.5 9.5h-2z"/><path d="M10.5 6.5l3 3M13.5 6.5l-3 3"/>'
    };

    ICONS.power = '<path d="M8 2v5.5"/><path d="M4.9 4.4a5.2 5.2 0 1 0 6.2 0"/>';
    ICONS.close ='<path d="M4 4l8 8M12 4l-8 8"/>';

    // The slider's filled part: its value as a percentage in --vd-fill (the track gradient reads it).
    function syncRange(input) {
        const min = Number(input.min || 0), max = Number(input.max || 100);
        const pct = max > min ? (Number(input.value) - min) / (max - min) * 100 : 0;
        input.style.setProperty('--vd-fill', Math.max(0, Math.min(100, pct)) + '%');
    }

    function icon(name) {
        return '<svg class="vd-terminal-icon" viewBox="0 0 16 16" width="16" height="16" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">' +
            (ICONS[name] || '') + '</svg>';
    }

    // Icon plus a label span (textContent, so server or translated text never becomes markup); the title and the
    // aria-label keep the name when the label collapses on a narrow window.
    function setButton(button, name, label, title) {
        if (!button) return;
        button.innerHTML = icon(name) + '<span class="vd-terminal-btn-label"></span>';
        button.lastChild.textContent = label;
        button.setAttribute('aria-label', label);
        button.title = title || label;
    }

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, function (m) {
            return ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[m];
        });
    }

    // The console header. `o` carries plain translated texts (escaped here); the option lists are markup built by the caller.
    function toolbarMarkup(o) {
        const text = { loading: esc(o.loading), baud: esc(o.baud), style: esc(o.style), effects: esc(o.effects), audio: esc(o.audio), audioTitle: esc(o.audioTitle) };
        function field(cls, hook, name, label, options, attrs) {
            return '<label class="vd-terminal-select ' + cls + '"' + (hook || '') + '>' + icon(name) +
                '<span class="vd-sr-only">' + label + '</span>' +
                '<select ' + attrs + ' aria-label="' + label + '">' + options + '</select></label>';
        }
        function button(attrs, name, label) {
            return '<button type="button" ' + attrs + ' aria-label="' + label + '" title="' + label + '">' + icon(name) +
                '<span class="vd-terminal-btn-label">' + label + '</span></button>';
        }
        return '<div class="vd-terminal-toolbar">' +
            '<div class="vd-terminal-toolbar-status">' +
            '<span class="vd-terminal-status-dot" aria-hidden="true"></span>' +
            '<span class="vd-terminal-status" data-terminal-status role="status">' + text.loading + '</span>' +
            '<span class="vd-terminal-status-detail" data-terminal-status-detail></span>' +
            '<span class="vd-terminal-grid" data-terminal-grid aria-hidden="true"></span></div>' +
            '<div class="vd-terminal-toolbar-actions">' +
            '<button type="button" data-terminal-retronet-action="directory" hidden></button>' +
            field('vd-terminal-baud-label', ' data-terminal-baud-label hidden', 'gauge', text.baud, o.baudOptions, 'data-terminal-baud') +
            '<span class="vd-terminal-toolbar-sep" aria-hidden="true"></span>' +
            field('vd-terminal-style-label', '', 'monitor', text.style, o.styleOptions, 'data-terminal-style') +
            button('data-terminal-effects aria-haspopup="dialog"', 'sliders', text.effects) +
            '<button type="button" data-terminal-audio aria-pressed="' + (o.muted ? 'false' : 'true') + '" aria-label="' + text.audio +
            '" title="' + text.audioTitle + '">' + icon(o.muted ? 'soundOff' : 'soundOn') + '<span class="vd-terminal-btn-label">' + text.audio + '</span></button>' +
            '</div></div>';
    }

    // ---- Theme-aware Modern colours ----

    const LUMA_CAP = 0.012;

    function toLinear(c) {
        c /= 255;
        return c <= 0.04045 ? c / 12.92 : Math.pow((c + 0.055) / 1.055, 2.4);
    }

    function fromLinear(l) {
        const v = l <= 0.0031308 ? l * 12.92 : 1.055 * Math.pow(l, 1 / 2.4) - 0.055;
        return Math.round(Math.max(0, Math.min(1, v)) * 255);
    }

    function luminance(rgb) {
        const l = rgb.map(toLinear);
        return 0.2126 * l[0] + 0.7152 * l[1] + 0.0722 * l[2];
    }

    function contrast(a, b) {
        const x = luminance(a), y = luminance(b);
        return (Math.max(x, y) + 0.05) / (Math.min(x, y) + 0.05);
    }

    function toHex(rgb) {
        return '#' + rgb.map(function (v) { return ('0' + Math.round(v).toString(16)).slice(-2); }).join('');
    }

    function parseHex(hex) {
        return [1, 3, 5].map(function (i) { return parseInt(hex.slice(i, i + 2), 16); });
    }

    // Computed colours of custom properties as [r, g, b] (null when unset, transparent or unparsed; alpha blends over black): a hidden probe gives the computed colour,
    // a 1x1 canvas turns color-mix() and oklch() results into rgb.
    function resolveColors(el, names) {
        const probe = document.createElement('span');
        probe.style.display = 'none';
        el.appendChild(probe);
        try {
            const canvas = document.createElement('canvas');
            canvas.width = 1;
            canvas.height = 1;
            const ctx = canvas.getContext('2d', { willReadFrequently: true });
            const styles = window.getComputedStyle(el);
            return names.map(function (name) {
                if (!ctx || !styles.getPropertyValue(name).trim()) return null;
                // background-color: a gradient or garbage token computes to transparent (never the inherited text colour).
                probe.style.backgroundColor = '';
                probe.style.backgroundColor = 'var(' + name + ')';
                ctx.clearRect(0, 0, 1, 1);
                ctx.fillStyle = 'rgba(0, 0, 0, 0)';
                ctx.fillStyle = window.getComputedStyle(probe).backgroundColor;
                ctx.fillRect(0, 0, 1, 1);
                const d = ctx.getImageData(0, 0, 1, 1).data;
                if (!d[3]) return null;
                return [d[0], d[1], d[2]].map(function (v) { return v * d[3] / 255; });
            });
        } catch (_) {
            return names.map(function () { return null; });
        } finally {
            probe.remove();
        }
    }

    // The Modern theme for the app root `el`: the desktop theme's well colour, capped to a dark luminance (a darker
    // well stays as it is, a light theme gets a neutral dark in its hue), cursor and selection from the accent.
    function modernTheme(el) {
        const theme = Object.assign({}, MODERN_THEME);
        try {
            const found = resolveColors(el, ['--vd-mat-well-color', '--vd-accent']);
            const well = found[0], accent = found[1];
            if (well) {
                const lin = well.map(toLinear);
                const l = 0.2126 * lin[0] + 0.7152 * lin[1] + 0.0722 * lin[2];
                const k = l > LUMA_CAP ? LUMA_CAP / l : 1;
                theme.background = toHex(lin.map(function (v) { return fromLinear(v * k); }));
                theme.cursorAccent = theme.background;
            }
            const bg = parseHex(theme.background);
            if (accent && contrast(accent, bg) >= 3) {
                theme.cursor = toHex(accent);
                theme.selectionBackground = toHex(bg.map(function (v, i) { return v * 0.65 + accent[i] * 0.35; }));
            }
            theme.selectionSgr = barSgr(bg, accent && contrast(accent, bg) >= 3 ? accent : null);
        } catch (_) {}
        if (!theme.selectionSgr) theme.selectionSgr = barSgr(parseHex(theme.background), null);
        return theme;
    }

    // The directory's selected row (an SGR prefix: background plus bright white text): the accent mixed into the
    // background as strongly as keeps white text at 4.5:1 or better, else a quiet blue.
    function barSgr(bg, accent) {
        const white = [255, 255, 255];
        let bar = parseHex('#1e3a5f');
        if (accent) {
            for (let t = 0.6; t >= 0.2; t -= 0.05) {
                const mix = bg.map(function (v, i) { return v * (1 - t) + accent[i] * t; });
                if (contrast(white, mix) >= 4.5) { bar = mix; break; }
            }
        }
        return '\x1b[48;2;' + bar.map(Math.round).join(';') + 'm\x1b[97m';
    }

    window.TerminalStyles = {
        icon: icon,
        syncRange: syncRange,
        setButton: setButton,
        toolbarMarkup: toolbarMarkup,
        modernTheme: modernTheme,
        ids: IDS.slice(),
        normalize: normalize,
        load: load,
        save: save,
        effectControls: EFFECTS,
        loadEffects: loadEffects,
        saveEffects: saveEffects,
        resetEffects: resetEffects,
        profile: profile,
        applyXterm: applyXterm
    };
})();
