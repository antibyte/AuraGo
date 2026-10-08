(function () {
    'use strict';

    // Stroke icons drawn on a 24 px grid; they inherit currentColor so both Desktop themes tint them.
    const PATHS = {
        play: '<path d="M7 4.5v15l12-7.5z" fill="currentColor" stroke="none"/>',
        pause: '<rect x="6.5" y="4.5" width="4" height="15" rx="1" fill="currentColor" stroke="none"/><rect x="13.5" y="4.5" width="4" height="15" rx="1" fill="currentColor" stroke="none"/>',
        skipStart: '<path d="M6 5v14"/><path d="M18 5 9 12l9 7z"/>',
        skipEnd: '<path d="M18 5v14"/><path d="M6 5l9 7-9 7z"/>',
        stepBack: '<path d="M15 6l-6 6 6 6"/>',
        stepForward: '<path d="M9 6l6 6-6 6"/>',
        undo: '<path d="M9 14 4 9l5-5"/><path d="M4 9h10.5a5.5 5.5 0 0 1 0 11H11"/>',
        redo: '<path d="m15 14 5-5-5-5"/><path d="M20 9H9.5a5.5 5.5 0 0 0 0 11H13"/>',
        upload: '<path d="M12 15V3"/><path d="m7 8 5-5 5 5"/><path d="M4 15v4a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-4"/>',
        folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
        search: '<circle cx="11" cy="11" r="6.5"/><path d="m20 20-4.2-4.2"/>',
        plus: '<path d="M12 5v14M5 12h14"/>',
        minus: '<path d="M5 12h14"/>',
        scissors: '<circle cx="6" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M20 4 8.1 15.9M14.5 14.5 20 20M8.1 8.1 12 12"/>',
        copy: '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h10a1 1 0 0 1 1 1v1"/>',
        trash: '<path d="M4 7h16"/><path d="M10 11v6M14 11v6"/><path d="M6 7l1 12a2 2 0 0 0 2 2h6a2 2 0 0 0 2-2l1-12"/><path d="M9 7V4h6v3"/>',
        magnet: '<path d="M6 3v8a6 6 0 0 0 12 0V3"/><path d="M6 3h4v8a2 2 0 0 0 4 0V3h4"/><path d="M6 7h4M14 7h4"/>',
        fit: '<path d="M4 9V5a1 1 0 0 1 1-1h4M15 4h4a1 1 0 0 1 1 1v4M20 15v4a1 1 0 0 1-1 1h-4M9 20H5a1 1 0 0 1-1-1v-4"/><path d="M8 12h8M8 12l2-2M8 12l2 2M16 12l-2-2M16 12l-2 2"/>',
        speaker: '<path d="M4 9v6h4l5 4V5L8 9z"/><path d="M16.5 8.5a5 5 0 0 1 0 7M19 6a8.5 8.5 0 0 1 0 12"/>',
        speakerOff: '<path d="M4 9v6h4l5 4V5L8 9z"/><path d="m17 9 5 6M22 9l-5 6"/>',
        eye: '<path d="M2 12s3.6-7 10-7 10 7 10 7-3.6 7-10 7S2 12 2 12z"/><circle cx="12" cy="12" r="3"/>',
        eyeOff: '<path d="M10.6 5.1A10.4 10.4 0 0 1 12 5c6.4 0 10 7 10 7a17.6 17.6 0 0 1-3.2 4.1M6.6 6.6C3.7 8.4 2 12 2 12s3.6 7 10 7a9.7 9.7 0 0 0 5.4-1.6"/><path d="M3 3l18 18"/><path d="M9.9 9.9a3 3 0 0 0 4.2 4.2"/>',
        lock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 8 0v3"/>',
        unlock: '<rect x="5" y="11" width="14" height="10" rx="2"/><path d="M8 11V8a4 4 0 0 1 7.7-1.5"/>',
        video: '<rect x="3" y="6" width="13" height="12" rx="2"/><path d="m16 10 5-3v10l-5-3z"/>',
        audio: '<path d="M9 18V5l11-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="17" cy="16" r="3"/>',
        image: '<rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="2"/><path d="m21 16-5-5-9 9"/>',
        text: '<path d="M5 6V4h14v2"/><path d="M12 4v16"/><path d="M9 20h6"/>',
        sticker: '<path d="M15.5 3H6a3 3 0 0 0-3 3v12a3 3 0 0 0 3 3h7l8-8V6a3 3 0 0 0-3-3z"/><path d="M13 21v-5a3 3 0 0 1 3-3h5"/><path d="M8.5 9.5h.01M14.5 9.5h.01"/>',
        sparkle: '<path d="M12 3l1.8 5.2L19 10l-5.2 1.8L12 17l-1.8-5.2L5 10l5.2-1.8z"/><path d="M19 15l.8 2.2L22 18l-2.2.8L19 21l-.8-2.2L16 18l2.2-.8z"/>',
        download: '<path d="M12 3v12"/><path d="m7 10 5 5 5-5"/><path d="M4 17v2a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-2"/>',
        export: '<path d="M12 15V3"/><path d="m7 8 5-5 5 5"/><rect x="3" y="13" width="18" height="8" rx="2"/>',
        check: '<path d="m5 12.5 4.5 4.5L19 7"/>',
        alert: '<path d="M12 3 2 20h20z"/><path d="M12 10v4M12 17h.01"/>',
        close: '<path d="M6 6l12 12M18 6 6 18"/>',
        chevronDown: '<path d="m6 9 6 6 6-6"/>',
        keyboard: '<rect x="2.5" y="6" width="19" height="12" rx="2"/><path d="M6 10h.01M10 10h.01M14 10h.01M18 10h.01M7 14h10"/>',
        fullscreen: '<path d="M4 9V4h5M15 4h5v5M20 15v5h-5M9 20H4v-5"/>',
        sliders: '<path d="M4 6h10M18 6h2M4 12h4M12 12h8M4 18h12M20 18h0"/><circle cx="16" cy="6" r="2"/><circle cx="10" cy="12" r="2"/><circle cx="18" cy="18" r="2"/>',
        film: '<rect x="3" y="3" width="18" height="18" rx="2"/><path d="M7 3v18M17 3v18M3 8h4M3 12h4M3 16h4M17 8h4M17 12h4M17 16h4"/>',
        spinner: '<path d="M12 3a9 9 0 1 0 9 9" />',
        grip: '<path d="M9 6h.01M15 6h.01M9 12h.01M15 12h.01M9 18h.01M15 18h.01" stroke-width="2.6"/>',
        info: '<circle cx="12" cy="12" r="9"/><path d="M12 11v5M12 8h.01"/>',
        alignLeft: '<path d="M4 6h16M4 10h10M4 14h16M4 18h10"/>',
        alignCenter: '<path d="M4 6h16M7 10h10M4 14h16M7 18h10"/>',
        alignRight: '<path d="M4 6h16M10 10h10M4 14h16M10 18h10"/>',
        edit: '<path d="M4 20h4L19 9l-4-4L4 16z"/><path d="m14 6 4 4"/>',
        layers: '<path d="m12 3 9 5-9 5-9-5z"/><path d="m3 13 9 5 9-5"/>',
        clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
        transition: '<rect x="3" y="6" width="9" height="12" rx="1.5"/><rect x="12" y="6" width="9" height="12" rx="1.5" stroke-dasharray="2 2"/>',
        project: '<rect x="3" y="5" width="18" height="14" rx="2"/><path d="m10 9 5 3-5 3z"/>'
    };

    function svg(name, size, cls) {
        const px = Number(size) || 16;
        return `<svg class="${cls || 'vs-icon'}" width="${px}" height="${px}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true" focusable="false">${PATHS[name] || PATHS.info}</svg>`;
    }

    window.VideoStudioIcons = { svg, names: Object.keys(PATHS) };
})();
