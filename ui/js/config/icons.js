// Inline SVG icons for config sections: cfgIcon(name, {label, cls, size}).
// Stroke icons on a 24 grid, colour from currentColor, no external assets. Without a
// label the icon is decorative (aria-hidden); with one it is role="img" + aria-label.
// Names follow the shared AuraIcons set (js/precision/icons.js) where meanings overlap;
// when that set is loaded and knows the name, it renders the icon.
(function () {
    'use strict';

    const P = {
        // shared names
        trash: 'M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7',
        edit: 'm4 16 12-12 4 4L8 20H4v-4Zm10-10 4 4',
        play: 'M7 4v16l13-8L7 4Z',
        stop: 'M6 6h12v12H6z',
        pause: 'M8 5v14M16 5v14',
        download: 'M12 3v12m-5-5 5 5 5-5M4 20h16',
        upload: 'M12 16V4m-5 5 5-5 5 5M4 20h16',
        copy: 'M9 9h11v11H9zM5 15V4h11',
        plus: 'M12 4v16M4 12h16',
        x: 'm6 6 12 12M6 18 18 6',
        check: 'm4 12 5 5L20 6',
        alert: 'M12 3 2 21h20L12 3Zm0 7v5m0 3v.01',
        info: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0-10v6m0-9v.01',
        lock: 'M5 11h14v10H5zM8 11V7a4 4 0 0 1 8 0v4',
        unlock: 'M5 11h14v10H5zM8 11V7a4 4 0 0 1 7-2.6',
        external: 'M14 4h6v6M20 4l-9 9M18 14v6H4V6h6',
        'chevron-left': 'm15 5-7 7 7 7',
        'chevron-right': 'm9 5 7 7-7 7',
        'layout-grid': 'M4 4h7v7H4zM13 4h7v7h-7zM4 13h7v7H4zM13 13h7v7h-7z',
        list: 'M8 6h13M8 12h13M8 18h13M3 6h.01M3 12h.01M3 18h.01',
        // config set
        settings: 'M4 7h10M18 7h2M4 17h2M10 17h10M16 7a2 2 0 1 0-4 0 2 2 0 0 0 4 0ZM12 17a2 2 0 1 0-4 0 2 2 0 0 0 4 0Z',
        activity: 'M3 12h4l3-8 4 16 3-8h4',
        plug: 'M8 3v5M16 3v5M6 8h12v3a6 6 0 0 1-12 0V8Zm6 9v4',
        key: 'M14 10a5 5 0 1 0-4 4l1 1h2v2h2v2h3v-3l-4-4a5 5 0 0 0 0-2Zm-6.5-2v.01',
        shield: 'M12 3 4 6v6c0 5 3.5 8 8 9 4.5-1 8-4 8-9V6l-8-3Z',
        server: 'M4 4h16v6H4zM4 14h16v6H4zM8 7v.01M8 17v.01',
        database: 'M4 6c0-1.7 3.6-3 8-3s8 1.3 8 3-3.6 3-8 3-8-1.3-8-3Zm0 0v12c0 1.7 3.6 3 8 3s8-1.3 8-3V6M4 12c0 1.7 3.6 3 8 3s8-1.3 8-3',
        folder: 'M3 6h6l2 2h10v11H3z',
        'folder-open': 'M3 19V6h6l2 2h8v3M3 19l3-8h16l-3 8H3Z',
        file: 'M6 3h8l5 5v13H6zM14 3v5h5M9 13h6M9 17h6',
        cloud: 'M7 18a4 4 0 0 1-.5-8A6 6 0 0 1 18 9a4.5 4.5 0 0 1 0 9H7Z',
        globe: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18ZM3 12h18M12 3c3 3 3 15 0 18M12 3c-3 3-3 15 0 18',
        network: 'M12 3v6M12 9H5v6M12 9h7v6M3 15h4v4H3zM17 15h4v4h-4zM10 3h4v4h-4zM10 15h4v4h-4z',
        mail: 'M3 5h18v14H3zM3 7l9 7 9-7',
        chat: 'M4 4h16v12H9l-5 4V4Z',
        bell: 'M6 16V11a6 6 0 1 1 12 0v5l2 2H4l2-2Zm4 4a2 2 0 0 0 4 0',
        refresh: 'M20 4v6h-6M4 20v-6h6M5 8a8 8 0 0 1 14-3M19 16a8 8 0 0 1-14 3',
        search: 'M11 18a7 7 0 1 0 0-14 7 7 0 0 0 0 14Zm5-2 5 5',
        clock: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0-14v5l3 2',
        chart: 'M4 20V4M4 20h16M8 16v-4M12 16V8M16 16v-6',
        user: 'M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm-7 9c0-4 3-6 7-6s7 2 7 6',
        users: 'M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Zm-6 9c0-3.5 2.5-5.5 6-5.5s6 2 6 5.5M16 4.5a3.5 3.5 0 0 1 0 6.5M18 14.5c2 .6 3 2.2 3 5.5',
        link: 'M10 14a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1M14 10a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1',
        mic: 'M9 4a3 3 0 0 1 6 0v7a3 3 0 0 1-6 0V4ZM5 11a7 7 0 0 0 14 0M12 18v3',
        speaker: 'M4 9h4l5-4v14l-5-4H4zM16 9a4 4 0 0 1 0 6M18.5 6.5a8 8 0 0 1 0 11',
        camera: 'M3 7h4l2-3h6l2 3h4v13H3zM12 17a4 4 0 1 0 0-8 4 4 0 0 0 0 8Z',
        video: 'M3 6h13v12H3zM16 10l5-3v10l-5-3',
        image: 'M3 4h18v16H3zM3 16l5-5 4 4 3-3 6 6M8 9v.01',
        music: 'M9 18V5l11-2v13M9 18a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm11-2a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
        phone: 'M5 3h4l2 5-2.5 1.5a11 11 0 0 0 6 6L16 13l5 2v4a2 2 0 0 1-2 2A16 16 0 0 1 3 5a2 2 0 0 1 2-2Z',
        home: 'M3 11 12 3l9 8M5 10v11h14V10M10 21v-6h4v6',
        cpu: 'M7 7h10v10H7zM10 10h4v4h-4zM9 3v4M15 3v4M9 17v4M15 17v4M3 9h4M3 15h4M17 9h4M17 15h4',
        terminal: 'M3 4h18v16H3zM7 9l3 3-3 3M13 15h4',
        monitor: 'M3 4h18v12H3zM8 20h8M12 16v4',
        save: 'M5 3h12l4 4v14H3V3h2Zm2 0v6h10V3M7 21v-8h10v8',
        eye: 'M2 12s4-7 10-7 10 7 10 7-4 7-10 7S2 12 2 12Zm10 3a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
        box: 'M12 3 3 7.5v9L12 21l9-4.5v-9L12 3ZM3 7.5l9 4.5 9-4.5M12 12v9',
        puzzle: 'M10 4a2 2 0 1 1 4 0v2h4v4h-2a2 2 0 1 0 0 4h2v4h-4v-2a2 2 0 1 0-4 0v2H6v-4H4a2 2 0 1 1 0-4h2V6h4V4Z',
        bot: 'M5 8h14v11H5zM12 8V4M9 13v.01M15 13v.01M9 16.5h6M3 12v4M21 12v4',
        zap: 'M13 2 4 14h7l-1 8 9-12h-7l1-8Z',
        wrench: 'M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.770-3.770a6 6 0 0 1-7.940 7.940l-6.910 6.910a2.120 2.120 0 0 1-3-3l6.910-6.910a6 6 0 0 1 7.940-7.940l-3.760 3.760Z',
        flask: 'M9 3h6M10 3v6l-6 10a2 2 0 0 0 1.7 3h12.6a2 2 0 0 0 1.7-3l-6-10V3M7 15h10',
        wifi: 'M2 9a15 15 0 0 1 20 0M5 13a10 10 0 0 1 14 0M8.5 16.5a5 5 0 0 1 7 0M12 20v.01',
        palette: 'M12 3a9 9 0 0 0 0 18c1.5 0 2-1 1.5-2-.6-1.2.2-2.5 1.5-2.5H17a4 4 0 0 0 4-4c0-5-4-9.5-9-9.5ZM7.5 12v.01M10 7.5v.01M15 7.5v.01',
        code: 'm8 7-5 5 5 5M16 7l5 5-5 5M14 4l-4 16',
        bulb: 'M9 18h6M10 21h4M12 3a6 6 0 0 0-4 10.5c.7.7 1 1.5 1 2.5h6c0-1 .3-1.800 1-2.500A6 6 0 0 0 12 3Z',
        star: 'm12 3 2.800 5.700 6.200.9-4.500 4.400 1.100 6.200L12 17.200 6.400 20.200l1.100-6.200L3 9.600l6.200-.9L12 3Z',
        moon: 'M20 15A9 9 0 0 1 9 4a9 9 0 1 0 11 11Z',
        sun: 'M12 3v2M12 19v2M3 12h2M19 12h2M5.600 5.600 7 7M17 17l1.400 1.400M5.600 18.400 7 17M17 7l1.400-1.400M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0Z',
        rocket: 'M5 19c0-3 1-4 3-4l-1-3c3-6 8-9 13-9 0 5-3 10-9 13l-3-1c0 2-1 3-3 4Zm9-10v.01',
        gamepad: 'M6 8h12a4 4 0 0 1 4 4v3a3 3 0 0 1-5 2l-1.500-2h-7L7 17a3 3 0 0 1-5-2v-3a4 4 0 0 1 4-4ZM8 10v4M6 12h4M16 11v.01M18 13v.01',
        compass: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm3.500-12.500-2 5-5 2 2-5 5-2Z',
        pin: 'M12 21s7-6 7-12a7 7 0 0 0-14 0c0 6 7 12 7 12Zm0-9a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
        'arrow-left': 'M19 12H5m6-6-6 6 6 6',
        'arrow-right': 'M5 12h14m-6-6 6 6-6 6',
        'arrow-down': 'M12 5v14m-6-6 6 6 6-6',
        'arrow-up': 'M12 19V5m-6 6 6-6 6 6',
        loader: 'M12 3v4M12 17v4M3 12h4M17 12h4M5.600 5.600l2.800 2.800M15.600 15.600l2.800 2.800M5.600 18.400l2.800-2.800M15.600 8.400l2.800-2.800',
        circle: 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z',
        'circle-dot': 'M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Zm0-6a3 3 0 1 0 0-6 3 3 0 0 0 0 6Z',
        sparkles: 'M12 3l1.800 5.200L19 10l-5.200 1.800L12 17l-1.800-5.200L5 10l5.200-1.800L12 3ZM19 16v4M17 18h4',
        building: 'M5 21V4h9v17M14 9h5v12M3 21h18M8 8h3M8 12h3M8 16h3',
        tag: 'M3 3h8l10 10-8 8L3 11V3Zm4 4v.01',
        heart: 'M12 20s-8-5-8-11a4.500 4.500 0 0 1 8-2.800A4.500 4.500 0 0 1 20 9c0 6-8 11-8 11Z'
    };

    // Emoji (and old text symbols) used in ui/cfg so far -> icon name.
    const EMOJI = {
        '🔌': 'plug', '⚠': 'alert', '🔐': 'lock', '🔒': 'lock', '🔓': 'unlock', '❌': 'x', '✕': 'x', '✅': 'check', '✓': 'check',
        '✔': 'check', '🌐': 'globe', '🌍': 'globe', '🗑': 'trash', '🔄': 'refresh', '↻': 'refresh', '🔑': 'key', '⚙': 'settings',
        '🛠': 'settings', '🔧': 'wrench', '⏳': 'clock', '⏱': 'clock', '✏': 'edit', '📋': 'list', '🔍': 'search', '🔎': 'search',
        '▶': 'play', '⏹': 'stop', '⏸': 'pause', '🛡': 'shield', '🔗': 'link', '⚡': 'zap', '🧪': 'flask', '📡': 'wifi', '📶': 'wifi',
        '🛰': 'wifi', '💾': 'save', '📄': 'file', '📝': 'edit', '📖': 'file', '🐳': 'box', '📊': 'chart', '📥': 'download',
        '⬇': 'download', '📤': 'upload', '💡': 'bulb', '📁': 'folder', '📂': 'folder-open', '🗄': 'database', '🔴': 'circle-dot',
        '🟢': 'circle-dot', '🟡': 'circle-dot', '⚪': 'circle', '⭕': 'circle', '●': 'circle-dot', '○': 'circle', '◎': 'circle-dot',
        '🖥': 'monitor', '💻': 'monitor', '📺': 'monitor', '🖧': 'server', '🧭': 'compass', '📦': 'box', '🧩': 'puzzle', '🖼': 'image',
        '🎬': 'video', '🎥': 'video', '🎞': 'video', '🎵': 'music', '♪': 'music', '🎤': 'mic', '🗣': 'speaker', '🏠': 'home',
        '🏢': 'building', '🎯': 'compass', '🧠': 'cpu', '📱': 'phone', '📞': 'phone', '🎮': 'gamepad', '🤖': 'bot', '⭐': 'star',
        '✦': 'sparkles', '☀': 'sun', '🌙': 'moon', '🚀': 'rocket', '🎨': 'palette', '🗜': 'box', '🎭': 'users', '💰': 'tag',
        '🍎': 'box', '🐚': 'terminal', '🐍': 'code', '🐧': 'terminal', '🐘': 'database', '🐬': 'database', '🦙': 'bot', '🕷': 'bot',
        '👁': 'eye', '→': 'arrow-right', '←': 'arrow-left', '↗': 'external', '🆓': 'tag', '🆕': 'sparkles', '🚫': 'x', '🔷': 'circle',
        '🪟': 'monitor', '💗': 'heart', '💔': 'heart', '🗃': 'database', '⚔': 'shield', '＋': 'plus', '➕': 'plus', '▣': 'box'
    };

    function esc(value) {
        return String(value == null ? '' : value).replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[ch]));
    }

    function has(name) {
        return Object.prototype.hasOwnProperty.call(P, name);
    }

    // -> SVG markup string. Unknown names fall back to "info" so layouts never collapse.
    function cfgIcon(name, opts) {
        opts = opts || {};
        if (window.AuraIcons && typeof window.AuraIcons.has === 'function' && window.AuraIcons.has(name)) {
            return window.AuraIcons.svg(name, { label: opts.label, cls: opts.cls, size: opts.size });
        }
        const path = has(name) ? P[name] : P.info;
        const size = opts.size ? ' width="' + esc(opts.size) + '" height="' + esc(opts.size) + '"' : '';
        const cls = 'cfg-ui-icon' + (opts.cls ? ' ' + esc(opts.cls) : '');
        const a11y = opts.label ? ' role="img" aria-label="' + esc(opts.label) + '"' : ' aria-hidden="true" focusable="false"';
        return '<svg class="' + cls + '" data-icon="' + esc(name) + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"' + size + a11y + '><path d="' + path + '"/></svg>';
    }

    function cfgIconForEmoji(symbol) {
        const key = String(symbol || '').replace(/️/g, '').trim();
        return Object.prototype.hasOwnProperty.call(EMOJI, key) ? EMOJI[key] : '';
    }

    window.cfgIcon = cfgIcon;
    window.cfgIconForEmoji = cfgIconForEmoji;
    window.AuraConfigIcons = { svg: cfgIcon, has, names: () => Object.keys(P), forEmoji: cfgIconForEmoji };
})();
