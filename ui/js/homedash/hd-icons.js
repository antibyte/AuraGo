// HomeDash line icons: 24 px grid, stroked by CSS (.hd-ico). Starter set for the eight hero controls; ~70 glyphs in W4.
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
        'trend-up': ['M7 17L17 7', 'M9 7h8v8'], 'trend-down': ['M7 7l10 10', 'M17 9v8H9'], 'trend-flat': ['M5 12h14', 'M14 7l5 5-5 5']
    });
    const has = name => Object.prototype.hasOwnProperty.call(PATHS, name);
    function icon(name) { const key = has(name) ? name : 'entity';
        const node = E.svg('svg', { class: 'hd-ico', viewBox: '0 0 24 24', 'aria-hidden': 'true', focusable: 'false', 'data-hd-icon': key });
        for (const d of PATHS[key]) node.appendChild(E.svg('path', { d })); return node; }
    E.icons = Object.freeze({ icon, has, names: () => Object.keys(PATHS) });
})();
