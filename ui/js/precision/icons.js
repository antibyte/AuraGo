// ══════════════════════════════════════════════════════════════════════════════
// AuraGo shared icon set (operational pages)
// Inline stroke SVGs in the dashboard style, no sprite or network dependency.
//   AuraIcons.svg('play', {cls, label, size}) -> '<svg class="pw-ic pw-ic-play" ...>'
//   AuraIcons.el(name, opts)  -> SVGElement (empty span for unknown names)
//   AuraIcons.has(name) / names() / paths() / injectSprite(prefix)
//   pwIcon('play', 'extra-class') is the shorthand for AuraIcons.svg.
// With data-i18n buttons put the icon OUTSIDE the translated <span>: i18n
// replaces the span text, and the icon would be wiped otherwise.
// dash-icons.js is a shim on top of this file (dashIcon, #ic-* sprite, dash-ic).
// ══════════════════════════════════════════════════════════════════════════════

(function () {
    'use strict';

    if (window.AuraIcons) return;

    /** 24x24 viewBox, stroke-based paths; stroke="currentColor" is set on the root. */
    const ICON_PATHS = {
        // Navigation / Tabs
        'home':       '<path d="M3 9l9-7 9 7v11a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>',
        'brain':      '<path d="M12 2a3 3 0 0 0-3 3v.5A3.5 3.5 0 0 0 5.5 9c0 1 .4 1.9 1 2.5A3.5 3.5 0 0 0 5 15.5c0 1.9 1.3 3.5 3 3.8V21a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-1.7c1.7-.3 3-1.9 3-3.8a3.5 3.5 0 0 0-1.5-4c.6-.6 1-1.5 1-2.5A3.5 3.5 0 0 0 15 5.5V5a3 3 0 0 0-3-3z"/>',
        'user':       '<circle cx="12" cy="8" r="4"/><path d="M4 21v-1a7 7 0 0 1 14 0v1"/>',
        'graph':      '<circle cx="12" cy="12" r="2"/><circle cx="5" cy="5" r="1.5"/><circle cx="19" cy="5" r="1.5"/><circle cx="5" cy="19" r="1.5"/><circle cx="19" cy="19" r="1.5"/><line x1="6.5" y1="6.5" x2="10.5" y2="10.5"/><line x1="13.5" y1="10.5" x2="17.5" y2="6.5"/><line x1="6.5" y1="17.5" x2="10.5" y2="13.5"/><line x1="13.5" y1="13.5" x2="17.5" y2="17.5"/>',
        'filesync':   '<path d="M12 2v6m0 0L8 6m4 2l4-2"/><rect x="3" y="8" width="18" height="13" rx="2"/><path d="M7 13h2v4H7zm8 0h2v4h-2z" fill="currentColor" stroke="none"/>',
        'audit':      '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><path d="M14 2v6h6"/><path d="M8 13h8M8 17h5" stroke-linecap="round"/>',
        'cron':       '<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14" stroke-linecap="round" stroke-linejoin="round"/>',
        'settings':   '<circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M4.9 4.9l2.1 2.1M17 17l2.1 2.1M4.9 19.1L7 17M17 7l2.1-2.1" stroke-linecap="round"/>',

        // Logo
        'chart':      '<path d="M3 3v18h18"/><rect x="7" y="11" width="3" height="6" fill="currentColor" stroke="none"/><rect x="13" y="7" width="3" height="10" fill="currentColor" stroke="none"/><rect x="19" y="13" width="0.1" height="4" fill="currentColor" stroke="none"/>',

        // Card icons — overview
        'server':     '<rect x="2" y="3" width="20" height="8" rx="2"/><rect x="2" y="13" width="20" height="8" rx="2"/><circle cx="6" cy="7" r="1" fill="currentColor" stroke="none"/><circle cx="6" cy="17" r="1" fill="currentColor" stroke="none"/>',
        'bolt':       '<path d="M13 2L3 14h7l-1 8 10-12h-7z" fill="currentColor" stroke="none"/>',
        'wallet':     '<rect x="2" y="6" width="20" height="14" rx="2"/><path d="M2 10h20"/><circle cx="17" cy="15" r="1.5" fill="currentColor" stroke="none"/>',
        'sliders':    '<line x1="4" y1="6" x2="4" y2="10"/><line x1="4" y1="14" x2="4" y2="18"/><line x1="12" y1="6" x2="12" y2="8"/><line x1="12" y1="12" x2="12" y2="18"/><line x1="20" y1="6" x2="20" y2="12"/><line x1="20" y1="16" x2="20" y2="18"/><circle cx="4" cy="12" r="2"/><circle cx="12" cy="10" r="2"/><circle cx="20" cy="14" r="2"/>',
        'compress':   '<path d="M9 9V3M15 9V3M9 15v6M15 15v6M3 9h6M15 9h6M3 15h6M15 15h6" stroke-linecap="round"/>',
        'rocket':     '<path d="M12 2c3 2 5 5 5 9l-2 4h-6l-2-4c0-4 2-7 5-9z" fill="currentColor" stroke="none"/><path d="M9 15l-3 3c0-2 1-4 3-5M15 15l3 3c0-2-1-4-3-5"/><circle cx="12" cy="8" r="1.5" fill="none" stroke="currentColor"/>',

        // Card icons — agent
        'drama':      '<path d="M12 3a9 9 0 1 0 9 9 9 9 0 0 0-9-9z"/><path d="M8 11l2 2-2 2M16 11l-2 2 2 2" stroke-linecap="round"/><circle cx="9" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="9" r="1" fill="currentColor" stroke="none"/>',
        'puzzle':     '<path d="M10 3h4v3a2 2 0 1 0 0 4v4h-4v-4a2 2 0 1 1 0-4z" fill="currentColor" stroke="none"/>',
        'emotions':   '<circle cx="12" cy="12" r="9"/><path d="M8 14s1.5 2 4 2 4-2 4-2" stroke-linecap="round"/><circle cx="9" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="9" r="1" fill="currentColor" stroke="none"/>',
        'trend-up':   '<polyline points="3 17 9 11 13 15 21 7" stroke-linecap="round" stroke-linejoin="round"/><polyline points="14 7 21 7 21 14" stroke-linecap="round" stroke-linejoin="round"/>',

        // Card icons — user
        'calendar':   '<rect x="3" y="4" width="18" height="18" rx="2"/><line x1="3" y1="10" x2="21" y2="10"/><line x1="8" y1="2" x2="8" y2="6"/><line x1="16" y1="2" x2="16" y2="6"/>',
        'book':       '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z" fill="none"/>',

        // Card icons — knowledge
        'stethoscope':'<path d="M4 3v6a5 5 0 0 0 10 0V3" stroke-linecap="round"/><circle cx="19" cy="14" r="3"/><path d="M9 14v3a4 4 0 0 0 4 4" stroke-linecap="round"/>',
        'search':     '<circle cx="11" cy="11" r="7"/><line x1="16" y1="16" x2="21" y2="21" stroke-linecap="round"/>',
        'map':        '<path d="M9 3L3 6v15l6-3 6 3 6-3V3l-6 3-6-3z" fill="none"/><line x1="9" y1="3" x2="9" y2="18"/><line x1="15" y1="6" x2="15" y2="21"/>',
        'compass':    '<circle cx="12" cy="12" r="9"/><polygon points="16 8 14 14 8 16 10 10" fill="currentColor" stroke="none"/>',

        // Card icons — system
        'shield':     '<path d="M12 2L4 5v6c0 5 3.5 9 8 11 4.5-2 8-6 8-11V5z" fill="none"/>',
        'daemon':     '<path d="M12 2l8 4v6c0 5-3.5 9-8 11-4.5-2-8-6-8-11V6z" fill="none"/><circle cx="12" cy="11" r="3" fill="currentColor" stroke="none"/>',
        'feather':    '<path d="M20 4c-4 0-9 2-12 5l-4 4v3l4-1c3-3 5-7 5-11z" fill="none"/><path d="M16 8L8 16M14 10l-4 4M12 12l-2 2" stroke-linecap="round"/>',
        'tools':      '<path d="M14 7l3-3 3 3-3 3zM5 12l3-3 4 4-3 3z" fill="none"/><path d="M5 12L2 15l4 4 3-3M14 7l3 3" stroke-linecap="round"/>',
        'telemetry':  '<path d="M3 12h4l3-7 4 14 3-7h4" stroke-linecap="round" stroke-linejoin="round"/>',

        // Card icons — operations/integrations
        'octopus':    '<path d="M12 3a5 5 0 0 0-5 5v3a5 5 0 0 0 10 0V8a5 5 0 0 0-5-5z" fill="none"/><path d="M7 11c-3 0-5 2-5 5v2M12 11v6c0 2-1 4-3 4M17 11c3 0 5 2 5 5v2" stroke-linecap="round"/>',
        'clipboard':  '<rect x="8" y="2" width="8" height="4" rx="1"/><path d="M8 4H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V6a2 2 0 0 0-2-2h-2"/>',
        'note':       '<path d="M4 4h16v12l-4 4H4z" fill="none"/><path d="M16 20v-4h4" stroke-linecap="round"/><line x1="8" y1="9" x2="16" y2="9" stroke-linecap="round"/><line x1="8" y1="13" x2="12" y2="13" stroke-linecap="round"/>',

        // Action icons
        'refresh':    '<path d="M21 12a9 9 0 1 1-3-6.7L21 8" fill="none"/><polyline points="21 3 21 8 16 8" stroke-linecap="round" stroke-linejoin="round"/>',
        'arrow-down': '<line x1="12" y1="5" x2="12" y2="19" stroke-linecap="round"/><polyline points="6 13 12 19 18 13" stroke-linecap="round" stroke-linejoin="round"/>',
        'edit':       '<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.1 2.1 0 0 1 3 3L12 15l-4 1 1-4z" fill="none"/>',
        'trash':      '<polyline points="3 6 5 6 21 6" stroke-linecap="round"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" stroke-linecap="round"/>',
        'check':      '<polyline points="4 12 10 18 20 6" stroke-linecap="round" stroke-linejoin="round"/>',
        'x':          '<line x1="6" y1="6" x2="18" y2="18" stroke-linecap="round"/><line x1="18" y1="6" x2="6" y2="18" stroke-linecap="round"/>',
        'warning':    '<path d="M12 2L1 21h22z" fill="none"/><line x1="12" y1="9" x2="12" y2="14" stroke-linecap="round"/><circle cx="12" cy="17" r="0.5" fill="currentColor" stroke="none"/>',
        'lock':       '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 8 0v4" fill="none"/>',
        'globe':      '<circle cx="12" cy="12" r="9"/><line x1="3" y1="12" x2="21" y2="12"/><path d="M12 3a14 14 0 0 1 0 18 14 14 0 0 1 0-18z" fill="none"/>',
        'plug':       '<path d="M9 2v6M15 2v6M6 8h12v4a6 6 0 0 1-12 0z" fill="none"/><path d="M12 18v4" stroke-linecap="round"/>',
        'chat':       '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z" fill="none"/>',
        'wrench':     '<path d="M14 7l3-3 3 3-3 3z" fill="none"/><path d="M5 12l3-3 4 4-3 3zM14 7l3 3" stroke-linecap="round"/>',
        'ruler':      '<path d="M3 8l5-5 13 13-5 5z" fill="none"/><path d="M7 5l2 2M9 7l2 2M11 9l2 2M13 11l2 2" stroke-linecap="round"/>',

        // Status icons
        'circle-dot': '<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="3" fill="currentColor" stroke="none"/>',
        'play':       '<polygon points="6 4 20 12 6 20" fill="currentColor" stroke="none"/>',
        'stop':       '<rect x="6" y="6" width="12" height="12" rx="2" fill="currentColor" stroke="none"/>',
        'ban':        '<circle cx="12" cy="12" r="9"/><line x1="5" y1="5" x2="19" y2="19" stroke-linecap="round"/>',

        // Misc
        'trophy':     '<path d="M6 4h12v4a6 6 0 0 1-12 0z" fill="none"/><path d="M6 4H4v2a3 3 0 0 0 3 3M18 4h2v2a3 3 0 0 1-3 3M10 14v4M14 14v4M8 20h8" stroke-linecap="round"/>',
        'bot':        '<rect x="4" y="8" width="16" height="12" rx="2"/><circle cx="9" cy="14" r="1.5" fill="currentColor" stroke="none"/><circle cx="15" cy="14" r="1.5" fill="currentColor" stroke="none"/><path d="M12 4v4M9 2h6" stroke-linecap="round"/>',
        'hourglass':  '<path d="M6 2h12v6l-6 4-6-4z" fill="none"/><path d="M6 22h12v-6l-6-4-6 4z" fill="none"/>',
        'link':       '<path d="M10 13a5 5 0 0 0 7 0l3-3a5 5 0 0 0-7-7l-1 1" fill="none"/><path d="M14 11a5 5 0 0 0-7 0l-3 3a5 5 0 0 0 7 7l1-1" fill="none"/>',
        'folder':     '<path d="M4 6a2 2 0 0 1 2-2h4l2 2h6a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2z" fill="none"/>',
        'device':     '<rect x="7" y="2" width="10" height="20" rx="2"/><line x1="11" y1="18" x2="13" y2="18" stroke-linecap="round"/>',
        'key':        '<circle cx="8" cy="15" r="4"/><path d="M11 12l9-9M16 7l2 2" stroke-linecap="round"/>',
        'cloud':      '<path d="M6 18a4 4 0 0 1 0-8 5 5 0 0 1 9-2 4 4 0 0 1 1 8z" fill="none"/>',
        'docker':     '<rect x="2" y="9" width="4" height="4" fill="currentColor" stroke="none"/><rect x="7" y="9" width="4" height="4" fill="currentColor" stroke="none"/><rect x="12" y="9" width="4" height="4" fill="currentColor" stroke="none"/><path d="M2 14c0 3 3 4 8 4s10-2 10-4z" fill="none"/>',
        'email':      '<rect x="2" y="4" width="20" height="16" rx="2"/><polyline points="2 6 12 13 22 6" stroke-linecap="round" stroke-linejoin="round"/>',
        'monitor':    '<rect x="2" y="3" width="20" height="14" rx="2"/><line x1="8" y1="21" x2="16" y2="21" stroke-linecap="round"/><line x1="12" y1="17" x2="12" y2="21" stroke-linecap="round"/>',
        'printer':    '<rect x="6" y="2" width="12" height="6"/><rect x="6" y="14" width="12" height="8"/><path d="M6 8H4a2 2 0 0 0-2 2v6a2 2 0 0 0 2 2h2M18 8h2a2 2 0 0 1 2 2v6a2 2 0 0 1-2 2h-2" fill="none"/>',
        'video':      '<polygon points="23 7 16 12 23 17 23 7" fill="currentColor" stroke="none"/><rect x="1" y="5" width="15" height="14" rx="2"/>',
        'tts':        '<path d="M11 5L6 9H2v6h4l5 4z" fill="none"/><path d="M15 9a3 3 0 0 1 0 6M19 5a8 8 0 0 1 0 14" stroke-linecap="round"/>',
        'package':    '<path d="M12 2l9 5v10l-9 5-9-5V7z" fill="none"/><path d="M12 12l9-5M12 12v10M12 12L3 7" stroke-linecap="round"/>',
        'firewall':   '<rect x="2" y="6" width="20" height="12" rx="1"/><line x1="2" y1="10" x2="22" y2="10"/><line x1="2" y1="14" x2="22" y2="14"/><line x1="7" y1="6" x2="7" y2="10"/><line x1="12" y1="6" x2="12" y2="10"/><line x1="17" y1="10" x2="17" y2="14"/><line x1="10" y1="14" x2="10" y2="18"/><line x1="15" y1="14" x2="15" y2="18"/>',
        'spider':     '<circle cx="12" cy="12" r="3"/><path d="M12 9V4M12 15v5M9 11L5 7M9 13L5 17M15 11l4-4M15 13l4 4" stroke-linecap="round"/>',
        'virus':      '<circle cx="12" cy="12" r="4"/><line x1="12" y1="2" x2="12" y2="8" stroke-linecap="round"/><line x1="12" y1="16" x2="12" y2="22" stroke-linecap="round"/><line x1="2" y1="12" x2="8" y2="12" stroke-linecap="round"/><line x1="16" y1="12" x2="22" y2="12" stroke-linecap="round"/><line x1="5" y1="5" x2="9" y2="9" stroke-linecap="round"/><line x1="15" y1="15" x2="19" y2="19" stroke-linecap="round"/><line x1="19" y1="5" x2="15" y2="9" stroke-linecap="round"/><line x1="9" y1="15" x2="5" y2="19" stroke-linecap="round"/>',

        // Journal / sentiment
        'smile':      '<circle cx="12" cy="12" r="9"/><path d="M8 14s1.5 2 4 2 4-2 4-2" stroke-linecap="round"/><circle cx="9" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="9" r="1" fill="currentColor" stroke="none"/>',
        'meh':        '<circle cx="12" cy="12" r="9"/><line x1="8" y1="15" x2="16" y2="15" stroke-linecap="round"/><circle cx="9" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="9" r="1" fill="currentColor" stroke="none"/>',
        'frown':      '<circle cx="12" cy="12" r="9"/><path d="M8 16s1.5-2 4-2 4 2 4 2" stroke-linecap="round"/><circle cx="9" cy="9" r="1" fill="currentColor" stroke="none"/><circle cx="15" cy="9" r="1" fill="currentColor" stroke="none"/>',

        // Misc operational
        'egg':        '<ellipse cx="12" cy="12" rx="6" ry="9" fill="none"/>',
        'clock':      '<circle cx="12" cy="12" r="9"/><polyline points="12 7 12 12 15 14" stroke-linecap="round" stroke-linejoin="round"/>',
        'star':       '<polygon points="12 2 15 9 22 9 16 14 18 21 12 17 6 21 8 14 2 9 9 9" fill="currentColor" stroke="none"/>',
        'target':     '<circle cx="12" cy="12" r="9"/><circle cx="12" cy="12" r="5"/><circle cx="12" cy="12" r="1" fill="currentColor" stroke="none"/>',
        'code':       '<polyline points="8 6 2 12 8 18" stroke-linecap="round" stroke-linejoin="round"/><polyline points="16 6 22 12 16 18" stroke-linecap="round" stroke-linejoin="round"/>',
        'pen':        '<path d="M12 19l7-7 3 3-7 7-3-1zM18 13l-1.5-3L2 2l2 14.5L7 18" fill="none"/><path d="M2 2l7.5 7.5" stroke-linecap="round"/>',
        'pin':        '<path d="M12 2v6M12 8l5 4v4l-5-2-5 2v-4z" fill="none"/><circle cx="12" cy="18" r="3" fill="none"/>',
        'flag':       '<path d="M4 22V4M4 4l12 2-2 6 2 6H4" fill="none"/>',
        'box':        '<path d="M3 6l9-4 9 4v12l-9 4-9-4z" fill="none"/>',
        'bucket':     '<path d="M4 7l1-2h14l1 2v2a5 5 0 0 1-10 0 5 5 0 0 1-10 0z" fill="none"/><path d="M4 9v8a4 4 0 0 0 16 0v-8" fill="none"/>',
        'net':        '<path d="M12 2a10 10 0 1 0 0 20 10 10 0 0 0 0-20z" fill="none"/><path d="M2 12h20M12 2v20M4 7h16M4 17h16M7 4v16M17 4v16" stroke-opacity="0.5"/>',
        'speaker':    '<path d="M11 5L6 9H3v6h3l5 4z" fill="none"/><path d="M15 9a3 3 0 0 1 0 6" stroke-linecap="round"/>',
        'doc':        '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" fill="none"/><path d="M14 2v6h6M8 13h8M8 17h5" stroke-linecap="round"/>',
        'palette':    '<circle cx="12" cy="12" r="9"/><circle cx="8" cy="10" r="1" fill="currentColor" stroke="none"/><circle cx="16" cy="10" r="1" fill="currentColor" stroke="none"/><circle cx="12" cy="15" r="1" fill="currentColor" stroke="none"/>',
        'eye':        '<path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12z" fill="none"/><circle cx="12" cy="12" r="3"/>',
        'wifi':       '<path d="M5 13a10 10 0 0 1 14 0M8 16a5 5 0 0 1 8 0" stroke-linecap="round"/><circle cx="12" cy="20" r="1" fill="currentColor" stroke="none"/>',
        'database':   '<ellipse cx="12" cy="5" rx="9" ry="3" fill="none"/><path d="M3 5v14c0 1.7 4 3 9 3s9-1.3 9-3V5" fill="none"/><path d="M3 12c0 1.7 4 3 9 3s9-1.3 9-3" fill="none"/>',
        'cube':       '<path d="M12 2l9 5v10l-9 5-9-5V7z" fill="none"/>',

        // Shared additions (operational pages; pwIcon / AuraIcons)
        'pause':         '<rect x="6" y="4" width="4" height="16" rx="1"/><rect x="14" y="4" width="4" height="16" rx="1"/>',
        'unlock':        '<rect x="4" y="11" width="16" height="10" rx="2"/><path d="M8 11V7a4 4 0 0 1 7.5-2"/>',
        'download':      '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>',
        'upload':        '<path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/><polyline points="17 8 12 3 7 8"/><line x1="12" y1="3" x2="12" y2="15"/>',
        'copy':          '<rect x="9" y="9" width="12" height="12" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/>',
        'undo':          '<path d="M3 7v6h6"/><path d="M21 17a9 9 0 0 0-9-9 9 9 0 0 0-6 2.3L3 13"/>',
        'plus':          '<line x1="12" y1="5" x2="12" y2="19"/><line x1="5" y1="12" x2="19" y2="12"/>',
        'minus':         '<line x1="5" y1="12" x2="19" y2="12"/>',
        'chevron-left':  '<polyline points="15 18 9 12 15 6"/>',
        'chevron-right': '<polyline points="9 18 15 12 9 6"/>',
        'chevrons-left': '<polyline points="11 17 6 12 11 7"/><polyline points="18 17 13 12 18 7"/>',
        'chevrons-right':'<polyline points="13 17 18 12 13 7"/><polyline points="6 17 11 12 6 7"/>',
        'layout-grid':   '<rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/>',
        'list':          '<line x1="8" y1="6" x2="21" y2="6"/><line x1="8" y1="12" x2="21" y2="12"/><line x1="8" y1="18" x2="21" y2="18"/><line x1="3" y1="6" x2="3.01" y2="6"/><line x1="3" y1="12" x2="3.01" y2="12"/><line x1="3" y1="18" x2="3.01" y2="18"/>',
        'external':      '<path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/><polyline points="15 3 21 3 21 9"/><line x1="10" y1="14" x2="21" y2="3"/>',
        'paperclip':     '<path d="M21.4 11.1l-9.2 9.2a6 6 0 0 1-8.5-8.5l9.2-9.2a4 4 0 0 1 5.7 5.7l-9.2 9.2a2 2 0 0 1-2.8-2.8l8.5-8.5"/>',
        'bell':          '<path d="M18 8a6 6 0 0 0-12 0c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.7 21a2 2 0 0 1-3.4 0"/>',
        'users':         '<path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 0 0-3-3.9M16 3.1a4 4 0 0 1 0 7.8"/>',
        'pointer':       '<path d="M5 3l14 7-6 2-2 6z"/>',
        'info':          '<circle cx="12" cy="12" r="9"/><line x1="12" y1="11" x2="12" y2="16"/><line x1="12" y1="8" x2="12.01" y2="8"/>',
        'alert':         '<circle cx="12" cy="12" r="9"/><line x1="12" y1="7" x2="12" y2="13"/><line x1="12" y1="16.5" x2="12.01" y2="16.5"/>',
        'check-circle':  '<circle cx="12" cy="12" r="9"/><polyline points="8 12 11 15 16 9"/>',
        'x-circle':      '<circle cx="12" cy="12" r="9"/><line x1="9" y1="9" x2="15" y2="15"/><line x1="15" y1="9" x2="9" y2="15"/>',
        'cpu':           '<rect x="5" y="5" width="14" height="14" rx="2"/><rect x="9" y="9" width="6" height="6"/><path d="M9 2v3M15 2v3M9 19v3M15 19v3M2 9h3M2 15h3M19 9h3M19 15h3"/>',
        'nest':          '<path d="M3 12c0 5 4 8 9 8s9-3 9-8z"/><path d="M6 12c3-2 9-2 12 0"/><ellipse cx="12" cy="8" rx="3" ry="2.5"/>',
        'image':         '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8.5" cy="8.5" r="1.5"/><polyline points="21 15 16 10 5 21"/>',
        'music':         '<path d="M9 18V5l12-2v13"/><circle cx="6" cy="18" r="3"/><circle cx="18" cy="16" r="3"/>',
        'phone':         '<path d="M22 16.9v3a2 2 0 0 1-2.2 2 19.8 19.8 0 0 1-8.6-3.1 19.5 19.5 0 0 1-6-6A19.8 19.8 0 0 1 2.1 4.2 2 2 0 0 1 4.1 2h3a2 2 0 0 1 2 1.7c.1 1 .4 1.9.7 2.8a2 2 0 0 1-.5 2.1L8.1 9.9a16 16 0 0 0 6 6l1.3-1.3a2 2 0 0 1 2.1-.4c.9.3 1.8.6 2.8.7a2 2 0 0 1 1.7 2z"/>',
        'camera':        '<path d="M23 19a2 2 0 0 1-2 2H3a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h4l2-3h6l2 3h4a2 2 0 0 1 2 2z"/><circle cx="12" cy="13" r="4"/>',
        'radio':         '<circle cx="12" cy="12" r="2"/><path d="M16.2 7.8a6 6 0 0 1 0 8.4M7.8 16.2a6 6 0 0 1 0-8.4M19.1 4.9a10 10 0 0 1 0 14.2M4.9 19.1a10 10 0 0 1 0-14.2"/>',
        'check-square':  '<polyline points="9 11 12 14 22 4"/><path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11"/>',
        'file-code':     '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><polyline points="10 13 8 15 10 17"/><polyline points="14 13 16 15 14 17"/>',
        'table':         '<rect x="3" y="3" width="18" height="18" rx="2"/><line x1="3" y1="9" x2="21" y2="9"/><line x1="3" y1="15" x2="21" y2="15"/><line x1="9" y1="3" x2="9" y2="21"/>',
    };

    const STROKE_ATTRS = 'viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"';

    function escAttr(value) {
        return String(value == null ? '' : value)
            .replace(/&/g, '&amp;').replace(/"/g, '&quot;')
            .replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/'/g, '&#39;');
    }

    function has(name) {
        return Object.prototype.hasOwnProperty.call(ICON_PATHS, name);
    }

    function names() {
        return Object.keys(ICON_PATHS);
    }

    function paths() {
        return ICON_PATHS;
    }

    /**
     * Inline icon markup. Unknown names return ''.
     * @param {string} name
     * @param {{cls?: string, label?: string, size?: (string|number)}} [opts]
     */
    function svg(name, opts) {
        if (!has(name)) return '';
        if (typeof opts === 'string') opts = { cls: opts };
        opts = opts || {};
        let cls = 'pw-ic pw-ic-' + name;
        if (opts.cls) cls += ' ' + escAttr(opts.cls);
        let attrs = 'class="' + cls + '" ' + STROKE_ATTRS + ' focusable="false"';
        if (opts.size) {
            const size = typeof opts.size === 'number' ? opts.size + 'px' : escAttr(opts.size);
            attrs += ' style="width:' + size + ';height:' + size + '"';
        }
        if (opts.label) {
            attrs += ' role="img" aria-label="' + escAttr(opts.label) + '"';
        } else {
            attrs += ' aria-hidden="true"';
        }
        return '<svg ' + attrs + '>' + ICON_PATHS[name] + '</svg>';
    }

    function el(name, opts) {
        const markup = svg(name, opts);
        if (!markup) {
            const fallback = document.createElement('span');
            fallback.setAttribute('aria-hidden', 'true');
            return fallback;
        }
        const temp = document.createElement('div');
        temp.innerHTML = markup;
        return temp.firstElementChild;
    }

    let spriteInjected = false;
    /** Hidden <symbol> sprite for `<use href="#<prefix>NAME">` consumers (dashboard shim). */
    function injectSprite(prefix) {
        if (spriteInjected) return;
        const idPrefix = prefix || 'ic-';
        const symbols = Object.keys(ICON_PATHS).map(function (name) {
            return '<symbol id="' + idPrefix + name + '" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">' + ICON_PATHS[name] + '</symbol>';
        }).join('');
        const holder = document.createElement('div');
        holder.setAttribute('aria-hidden', 'true');
        holder.style.cssText = 'position:absolute;width:0;height:0;overflow:hidden';
        holder.innerHTML = '<svg xmlns="http://www.w3.org/2000/svg">' + symbols + '</svg>';
        document.body.insertBefore(holder, document.body.firstChild);
        spriteInjected = true;
    }

    window.AuraIcons = { svg: svg, el: el, has: has, names: names, paths: paths, injectSprite: injectSprite };
    window.pwIcon = function pwIcon(name, cls) { return svg(name, { cls: cls }); };
})();
