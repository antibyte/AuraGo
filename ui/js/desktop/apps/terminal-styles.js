(function () {
    'use strict';

    const STORAGE_KEY = 'aurago.desktop.terminal.style';
    const IDS = [
        'modern', 'amber', 'green', 'apple2', 'commodore64',
        'ibm3278', 'vintage', 'mono-green', 'transparent-green'
    ];

    const MONO = 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';
    const PIXEL = '"Press Start 2P", "Geist Mono", ui-monospace, monospace';
    const GEIST = '"Geist Mono", ui-monospace, SFMono-Regular, Menlo, Consolas, monospace';

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
            fontSize: 14,
            lineHeight: 1.15,
            theme: { background: '#120c04', foreground: '#ffb000', cursor: '#ffd27a', selectionBackground: '#5a3a10' },
            crt: { phosphor: [1.0, 0.69, 0.0], curve: 0.18, bloom: 0.55, burn: 0.22, noise: 0.08, flicker: 0.04, mask: 0.35, alpha: 1, scan: 0.85 }
        },
        green: {
            id: 'green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 14,
            lineHeight: 1.15,
            theme: { background: '#020802', foreground: '#5fff3a', cursor: '#9cff7a', selectionBackground: '#14580c' },
            crt: { phosphor: [0.37, 1.0, 0.23], curve: 0.16, bloom: 0.5, burn: 0.18, noise: 0.06, flicker: 0.035, mask: 0.32, alpha: 1, scan: 0.8 }
        },
        apple2: {
            id: 'apple2',
            retro: true,
            fontFamily: PIXEL,
            fontSize: 12,
            lineHeight: 1.35,
            theme: { background: '#001400', foreground: '#33ff33', cursor: '#66ff66', selectionBackground: '#0b4a0b' },
            crt: { phosphor: [0.2, 1.0, 0.2], curve: 0.08, bloom: 0.18, burn: 0.08, noise: 0.1, flicker: 0.02, mask: 0.15, alpha: 1, scan: 0.45 }
        },
        commodore64: {
            id: 'commodore64',
            retro: true,
            fontFamily: PIXEL,
            fontSize: 13,
            lineHeight: 1.3,
            theme: { background: '#352879', foreground: '#6c5eb5', cursor: '#9b8cff', selectionBackground: '#5040a0' },
            crt: { phosphor: [0.55, 0.48, 0.85], curve: 0.12, bloom: 0.32, burn: 0.12, noise: 0.07, flicker: 0.025, mask: 0.22, alpha: 1, scan: 0.55 }
        },
        ibm3278: {
            id: 'ibm3278',
            retro: true,
            fontFamily: GEIST,
            fontSize: 14,
            lineHeight: 1.1,
            theme: { background: '#010301', foreground: '#2adf2a', cursor: '#7cff7c', selectionBackground: '#0d3d0d' },
            crt: { phosphor: [0.16, 0.87, 0.16], curve: 0.1, bloom: 0.22, burn: 0.06, noise: 0.03, flicker: 0.015, mask: 0.18, alpha: 1, scan: 0.4 }
        },
        vintage: {
            id: 'vintage',
            retro: true,
            fontFamily: GEIST,
            fontSize: 13,
            lineHeight: 1.2,
            theme: { background: '#0a0804', foreground: '#d4b06a', cursor: '#f0d090', selectionBackground: '#4a3818' },
            crt: { phosphor: [0.83, 0.69, 0.42], curve: 0.28, bloom: 0.62, burn: 0.42, noise: 0.16, flicker: 0.07, mask: 0.4, alpha: 1, scan: 0.9 }
        },
        'mono-green': {
            id: 'mono-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 14,
            lineHeight: 1.12,
            theme: { background: '#000000', foreground: '#00ff66', cursor: '#9dffc0', selectionBackground: '#00331a' },
            crt: { phosphor: [0.0, 1.0, 0.4], curve: 0.14, bloom: 0.28, burn: 0.1, noise: 0.04, flicker: 0.02, mask: 0.25, alpha: 1, scan: 0.95 }
        },
        'transparent-green': {
            id: 'transparent-green',
            retro: true,
            fontFamily: GEIST,
            fontSize: 14,
            lineHeight: 1.15,
            theme: { background: '#001108', foreground: '#7CFF6A', cursor: '#b6ffaa', selectionBackground: '#145820' },
            crt: { phosphor: [0.49, 1.0, 0.42], curve: 0.15, bloom: 0.4, burn: 0.14, noise: 0.05, flicker: 0.03, mask: 0.28, alpha: 0.72, scan: 0.7 }
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
