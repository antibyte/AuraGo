// HomeDash line icons: 24 px grid, stroked by CSS (.hd-ico). M2-M4 set; ~70 glyphs complete in W4.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core) throw new Error('hd-icons.js: load hd-core.js first');
    const PATHS = Object.freeze({
        power: ['M12 3v8', 'M6.3 6.3a8 8 0 1 0 11.4 0'],
        bulb: ['M9 18h6', 'M10 21h4', 'M12 3a6 6 0 0 0-3.5 10.9c.9.7 1.5 1.6 1.5 2.6V17h4v-.5c0-1 .6-1.9 1.5-2.6A6 6 0 0 0 12 3z'],
        thermometer: ['M14 14.8V5a2 2 0 0 0-4 0v9.8a4 4 0 1 0 4 0z', 'M12 9v7'],
        flame: ['M12 3c1 3 5 5 5 10a5 5 0 0 1-10 0c0-2.5 1.5-4 2.5-5 .3 2 1.5 3 2.5 3 0-3-1-5 0-8z'],
        snowflake: ['M12 2v20', 'M3.3 7l17.4 10', 'M3.3 17l17.4-10', 'M9 4l3 2 3-2', 'M9 20l3-2 3 2'],
        gauge: ['M4.9 17a8 8 0 1 1 14.2 0', 'M12 13l4-4', 'M11 13a1 1 0 1 0 2 0a1 1 0 1 0-2 0'],
        droplet: ['M12 3s6 6.5 6 11a6 6 0 0 1-12 0c0-4.5 6-11 6-11z'], chart: ['M4 19h16', 'M5 15l4-4 3 3 6-6'],
        minus: ['M5 12h14'], plus: ['M12 5v14', 'M5 12h14'], alert: ['M12 4l9 16H3z', 'M12 10v4', 'M12 17h.01'],
        clock: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 7v5l3 2'], lock: ['M6 11h12v9H6z', 'M8 11V8a4 4 0 0 1 8 0v3'],
        check: ['M5 12.5l4.5 4.5L19 7'], more: ['M6 12h.01', 'M12 12h.01', 'M18 12h.01'], entity: ['M5 5h14v14H5z', 'M9 12h6'],
        'mode-auto': ['M5 19l7-14 7 14', 'M8.2 14h7.6'],
        plug: ['M9 3v5', 'M15 3v5', 'M6 8h12v3a6 6 0 0 1-12 0z', 'M12 17v4'],
        cpu: ['M7 7h10v10H7z', 'M10 10h4v4h-4z', 'M10 3v4', 'M14 3v4', 'M10 17v4', 'M14 17v4', 'M3 10h4', 'M3 14h4', 'M17 10h4', 'M17 14h4'],
        disk: ['M4 14h16v5H4z', 'M4 14l2.5-8.5h11L20 14', 'M7.5 16.5h.01', 'M11 16.5h5'],
        'fan-heater': ['M4 5h16v11H4z', 'M12 7.75a2.75 2.75 0 1 0 0 5.5a2.75 2.75 0 1 0 0-5.5z', 'M12 7.75v5.5', 'M9.25 10.5h5.5', 'M7 16v3', 'M17 16v3'],
        'trend-up': ['M7 17L17 7', 'M9 7h8v8'], 'trend-down': ['M7 7l10 10', 'M17 9v8H9'], 'trend-flat': ['M5 12h14', 'M14 7l5 5-5 5'],
        'arrow-left': ['M19 12H5', 'M11 18l-6-6 6-6'], trash: ['M4 7h16', 'M9 7V4h6v3', 'M6 7l1 13h10l1-13', 'M10 11v6', 'M14 11v6'],
        dashboard: ['M4 4h7v9H4z', 'M13 4h7v5h-7z', 'M13 11h7v9h-7z', 'M4 15h7v5H4z'],
        'signal-off': ['M3 3l18 18', 'M8.5 16.4a5 5 0 0 1 7 0', 'M5.2 12.9a10 10 0 0 1 4-2.3', 'M14.6 10.5a10 10 0 0 1 4.2 2.4', 'M2 8.8a15 15 0 0 1 4.3-2.7', 'M10.4 5.1A15 15 0 0 1 22 8.8', 'M12 20h.01'],
        info: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 11v5', 'M12 8h.01'], refresh: ['M20 11a8 8 0 1 0-2.3 5.7', 'M20 4v7h-7'],
        memory: ['M4 7h16v10H4z', 'M8 17v3', 'M12 17v3', 'M16 17v3', 'M7.5 10.5h2.5v3H7.5z', 'M14 10.5h2.5v3H14z'],
        download: ['M12 4v11', 'M7.5 10.5L12 15l4.5-4.5', 'M5 19.5h14'], upload: ['M12 15V4', 'M7.5 8.5L12 4l4.5 4.5', 'M5 19.5h14'],
        timer: ['M12 5.5a7.5 7.5 0 1 0 0 15a7.5 7.5 0 1 0 0-15z', 'M12 9.5V13l2.5 2', 'M10 2.5h4', 'M12 2.5v3', 'M18.4 6.6l1.4-1.4'],
        outlet: ['M6 3.5h12a2.5 2.5 0 0 1 2.5 2.5v12a2.5 2.5 0 0 1-2.5 2.5H6A2.5 2.5 0 0 1 3.5 18V6A2.5 2.5 0 0 1 6 3.5z', 'M9.5 9v2.5', 'M14.5 9v2.5', 'M10 16.5a2 2 0 0 1 4 0'],
        scene: ['M11 3.5l1.7 4.3L17 9.5l-4.3 1.7L11 15.5l-1.7-4.3L5 9.5l4.3-1.7z', 'M18 14l.9 2.1 2.1.9-2.1.9L18 20l-.9-2.1L15 17l2.1-.9z'],
        script: ['M6.5 3.5h8l4 4v13h-12z', 'M14.5 3.5v4h4', 'M10.5 11v5.5l4.5-2.75z'],
        button: ['M12 4a8 8 0 1 0 0 16a8 8 0 1 0 0-16z', 'M12 8.5a3.5 3.5 0 1 0 0 7a3.5 3.5 0 1 0 0-7z'],
        list: ['M9 6.5h11', 'M9 12h11', 'M9 17.5h11', 'M4.5 6.5h.01', 'M4.5 12h.01', 'M4.5 17.5h.01'], number: ['M5 9h14', 'M5 15h14', 'M10.5 4L8.5 20', 'M15.5 4l-2 16'],
        brightness: ['M12 8a4 4 0 1 0 0 8a4 4 0 1 0 0-8z', 'M12 2.5v2', 'M12 19.5v2', 'M2.5 12h2', 'M19.5 12h2', 'M5.3 5.3l1.4 1.4', 'M17.3 17.3l1.4 1.4', 'M5.3 18.7l1.4-1.4', 'M17.3 6.7l1.4-1.4'],
        'color-temp': ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 3v18', 'M12 7.5L7.8 11.7', 'M12 12.5l-3.5 3.5'],
        color: ['M12 3a9 9 0 0 0 0 18c1.4 0 2-.9 2-1.9 0-.6-.3-1-.6-1.4-.3-.4-.6-.8-.6-1.4 0-1.1.9-1.8 2-1.8h1.7A4.5 4.5 0 0 0 21 11c0-4.4-4-8-9-8z', 'M7.5 11.5h.01', 'M9.5 7.5h.01', 'M14.5 7.5h.01'],
        effect: ['M4 20L14.5 9.5', 'M16 3.5v4', 'M14 5.5h4', 'M19.5 9v3', 'M18 10.5h3', 'M9.5 3.5v2', 'M8.5 4.5h2'],
        'chevron-down': ['M6.5 9.5L12 15l5.5-5.5'], close: ['M6.5 6.5l11 11', 'M17.5 6.5l-11 11'], home: ['M3.5 11L12 4l8.5 7', 'M6 9.5V20h12V9.5', 'M10 20v-5.5h4V20'],
        undo: ['M9 14L4 9l5-5', 'M4 9h10.5a5.5 5.5 0 0 1 0 11H11'], redo: ['M15 14l5-5-5-5', 'M20 9H9.5a5.5 5.5 0 0 0 0 11H13'],
        copy: ['M9 9h11v11H9z', 'M5 15H4V4h11v1'], paste: ['M9 4h6v3H9z', 'M15 5.5h3V21H6V5.5h3', 'M9 12h6', 'M9 16h4'],
        cut: ['M6 3.5a2.5 2.5 0 1 0 0 5a2.5 2.5 0 1 0 0-5z', 'M6 15.5a2.5 2.5 0 1 0 0 5a2.5 2.5 0 1 0 0-5z', 'M8.1 7.6L20 18', 'M8.1 16.4L20 6'],
        edit: ['M4 20h4L19 9l-4-4L4 16z', 'M13.5 6.5l4 4'], duplicate: ['M8 8h12v12H8z', 'M4 16V4h12', 'M14 11v6', 'M11 14h6'],
        'panel-left': ['M4 5h16v14H4z', 'M9 5v14'], 'panel-right': ['M4 5h16v14H4z', 'M15 5v14'], window: ['M4 5h16v14H4z', 'M4 9h16', 'M7 7h.01'],
        grip: ['M9 6h.01', 'M15 6h.01', 'M9 12h.01', 'M15 12h.01', 'M9 18h.01', 'M15 18h.01'],
        look: ['M12 3a9 9 0 1 0 0 18a9 9 0 1 0 0-18z', 'M12 3v18', 'M12 7.5h4.5', 'M12 12h6', 'M12 16.5h4.5'],
        layers: ['M12 4l8.5 4.5L12 13 3.5 8.5z', 'M3.5 12.5L12 17l8.5-4.5', 'M3.5 16.5L12 21l8.5-4.5'],
        'align-left': ['M4 3v18', 'M8 7h11v4H8z', 'M8 14h7v4H8z'], 'align-top': ['M3 4h18', 'M7 8h4v11H7z', 'M14 8h4v7h-4z'],
        search: ['M10.5 4a6.5 6.5 0 1 0 0 13a6.5 6.5 0 1 0 0-13z', 'M15.5 15.5L20 20'],
        'device-desktop': ['M3.5 5h17v11h-17z', 'M9 20h6', 'M12 16v4'], 'device-tablet': ['M6 3.5h12v17H6z', 'M11 17.5h2'], 'device-phone': ['M8 3h8v18H8z', 'M11 18h2'],
        fullscreen: ['M4 9V4h5', 'M15 4h5v5', 'M20 15v5h-5', 'M9 20H4v-5'], sync: ['M20 11a8 8 0 0 0-14.3-4.9', 'M4 4v4h4', 'M4 13a8 8 0 0 0 14.3 4.9', 'M20 20v-4h-4'],
        'cloud-off': ['M3 3l18 18', 'M8 7.5A5 5 0 0 1 16.6 10H17a4 4 0 0 1 2.4 7.2', 'M16 19H7a4 4 0 0 1-1.3-7.8'], 'arrow-right': ['M5 12h14', 'M13 6l6 6-6 6']
    });
    const UI_GLYPHS = new Set(['more', 'check', 'close', 'chevron-down', 'arrow-left', 'arrow-right', 'minus', 'plus', 'trash', 'refresh', 'signal-off', 'alert', 'info', 'mode-auto', 'trend-up', 'trend-down', 'trend-flat', 'undo', 'redo', 'copy', 'cut', 'paste', 'edit', 'duplicate', 'panel-left', 'panel-right', 'window', 'grip', 'look', 'layers', 'align-left', 'align-top', 'search', 'device-desktop', 'device-tablet', 'device-phone', 'fullscreen', 'sync', 'cloud-off']);
    const has = name => Object.prototype.hasOwnProperty.call(PATHS, name);
    function icon(name) { const key = has(name) ? name : 'entity';
        const node = E.svg('svg', { class: 'hd-ico', viewBox: '0 0 24 24', 'aria-hidden': 'true', focusable: 'false', 'data-hd-icon': key });
        for (const d of PATHS[key]) node.appendChild(E.svg('path', { d })); return node; }
    E.icons = Object.freeze({ icon, has, names: () => Object.keys(PATHS), choices: () => Object.keys(PATHS).filter(n => !UI_GLYPHS.has(n)) });
})();
