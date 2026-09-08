(function () {
    'use strict';

    const STORAGE_KEY = 'aurago.desktop.terminal.style';
    const IDS = [
        'modern', 'amber', 'green', 'apple2', 'commodore64',
        'ibm3278', 'vintage', 'mono-green', 'transparent-green'
    ];

    const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';
    const PIXEL = '"Press Start 2P", "Geist Mono", ui-monospace, monospace';
    const GEIST = '"Aura Terminal", "Geist Mono", ui-monospace, monospace';

    const PROFILES = {
        modern: {
            id: 'modern',
            retro: false,
            fontFamily: MONO,
            fontSize: 13,
            lineHeight: 1.2,
            theme: { background: '#0f172a', foreground: '#e2e8f0', cursor: '#93c5fd', selectionBackground: '#1e3a5f' },
            crt: { phosphor: [0.89, 0.91, 0.94], curve: 0, bloom: 0, burn: 0, noise: 0, flicker: 0, mask: 0, alpha: 1, scan: 0 }
        },
        amber: {
            id: 'amber',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#120c04', foreground: '#ffb000', cursor: '#ffd27a', selectionBackground: '#5a3a10' },
            crt: { phosphor: [1.0, 0.66, 0.22], curve: 0.32, bloom: 0.65, burn: 0.28, noise: 0.065, flicker: 0.012, mask: 0.22, alpha: 1, scan: 0.72 }
        },
        green: {
            id: 'green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#020802', foreground: '#5fff3a', cursor: '#9cff7a', selectionBackground: '#14580c' },
            crt: { phosphor: [0.38, 1.0, 0.32], curve: 0.3, bloom: 0.58, burn: 0.24, noise: 0.05, flicker: 0.01, mask: 0.2, alpha: 1, scan: 0.68 }
        },
        apple2: {
            id: 'apple2',
            retro: true,
            fontFamily: PIXEL,
            fontSize: 12,
            lineHeight: 1.35,
            theme: { background: '#001400', foreground: '#33ff33', cursor: '#66ff66', selectionBackground: '#0b4a0b' },
            crt: { phosphor: [0.35, 1.0, 0.28], curve: 0.22, bloom: 0.35, burn: 0.18, noise: 0.055, flicker: 0.008, mask: 0.15, alpha: 1, scan: 0.8 }
        },
        commodore64: {
            id: 'commodore64',
            retro: true,
            fontFamily: PIXEL,
            fontSize: 13,
            lineHeight: 1.3,
            theme: { background: '#241b59', foreground: '#a99aff', cursor: '#c4baff', selectionBackground: '#5040a0' },
            crt: { phosphor: [0.65, 0.59, 1.0], curve: 0.24, bloom: 0.24, burn: 0.12, noise: 0.035, flicker: 0.008, mask: 0.2, alpha: 1, scan: 0.55 }
        },
        ibm3278: {
            id: 'ibm3278',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.1,
            theme: { background: '#010301', foreground: '#2adf2a', cursor: '#7cff7c', selectionBackground: '#0d3d0d' },
            crt: { phosphor: [0.25, 1.0, 0.3], curve: 0.18, bloom: 0.32, burn: 0.14, noise: 0.025, flicker: 0.006, mask: 0.18, alpha: 1, scan: 0.52 }
        },
        vintage: {
            id: 'vintage',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.2,
            theme: { background: '#0a0804', foreground: '#d4b06a', cursor: '#f0d090', selectionBackground: '#4a3818' },
            crt: { phosphor: [1.0, 0.73, 0.39], curve: 0.42, bloom: 0.85, burn: 0.46, noise: 0.12, flicker: 0.023, mask: 0.32, alpha: 1, scan: 0.85 }
        },
        'mono-green': {
            id: 'mono-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.12,
            theme: { background: '#000000', foreground: '#00ff66', cursor: '#9dffc0', selectionBackground: '#00331a' },
            crt: { phosphor: [0.18, 1.0, 0.48], curve: 0.22, bloom: 0.38, burn: 0.18, noise: 0.03, flicker: 0.008, mask: 0.25, alpha: 1, scan: 0.78 }
        },
        'transparent-green': {
            id: 'transparent-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 17,
            lineHeight: 1.15,
            theme: { background: '#001108', foreground: '#7CFF6A', cursor: '#b6ffaa', selectionBackground: '#145820' },
            crt: { phosphor: [0.49, 1.0, 0.42], curve: 0.26, bloom: 0.52, burn: 0.2, noise: 0.035, flicker: 0.01, mask: 0.2, alpha: 0.82, scan: 0.65 }
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

    function profile(id) {
        return PROFILES[normalize(id)] || PROFILES.modern;
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

    window.TerminalStyles = {
        ids: IDS.slice(),
        normalize: normalize,
        load: load,
        save: save,
        profile: profile,
        applyXterm: applyXterm
    };
})();
