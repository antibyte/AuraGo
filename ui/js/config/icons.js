// Inline SVG icons for config sections: cfgIcon(name, {label, cls, size}).
// Stroke icons on a 24 grid, colour from currentColor, no external assets. Without a
// label the icon is decorative (aria-hidden); with one it is role="img" + aria-label.
// A thin wrapper over the shared AuraIcons set: js/precision/icons.js must be loaded first.
(function () {
    'use strict';

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
        return !!(window.AuraIcons && window.AuraIcons.has(name));
    }

    // -> SVG markup string. Unknown names fall back to "info" so layouts never collapse.
    function cfgIcon(name, opts) {
        opts = opts || {};
        if (!window.AuraIcons) return '';
        const markup = window.AuraIcons.svg(has(name) ? name : 'info', { label: opts.label, cls: opts.cls });
        const size = opts.size ? ' width="' + esc(opts.size) + '" height="' + esc(opts.size) + '"' : '';
        return markup.replace('<svg class="', '<svg class="cfg-ui-icon ').replace('<svg ', '<svg data-icon="' + esc(name) + '"' + size + ' ');
    }

    function cfgIconForEmoji(symbol) {
        const key = String(symbol || '').replace(/️/g, '').trim();
        return Object.prototype.hasOwnProperty.call(EMOJI, key) ? EMOJI[key] : '';
    }

    window.cfgIcon = cfgIcon;
    window.cfgIconForEmoji = cfgIconForEmoji;
    window.AuraConfigIcons = { svg: cfgIcon, has, names: () => window.AuraIcons.names(), forEmoji: cfgIconForEmoji };
})();
