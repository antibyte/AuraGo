// Shared by the shell and the generated SDK bundle; manifests own semantic mappings.
(function () {
    'use strict';
    const themes = ['papirus', 'whitesur', 'windows_xp', 'tela', 'candy', 'fluent', 'buuf_nestort'];
    function normalize(name) { return String(name || '').trim().toLowerCase().replace(/[^a-z0-9:_-]+/g, '_'); }
    function path(manifests, settings, scheme, name, usage) {
        let key = normalize(name), theme = settings['appearance.icon_theme'] || 'papirus';
        if (!key || key.startsWith('sprite:')) return '';
        for (const id of themes) {
            if (key.startsWith(id + ':')) { theme = id; key = key.slice(id.length + 1); break; }
        }
        const manifest = (manifests || {})[theme];
        if (!manifest) return '';
        const aliases = manifest.aliases || {};
        const dashed = key.replaceAll('_', '-');
        const candidates = [...new Set([key, dashed, aliases[key], aliases[dashed], ({ 'arrow-left': 'chevron-left', 'arrow-right': 'chevron-right' })[key]].filter(Boolean))];
        let icons = manifest.icons || {};
        if (usage === 'action' && manifest.actions) {
            icons = manifest.actions[scheme === 'dark' ? 'dark' : 'light'] || {};
            const actionName = dashed.replace(/-symbolic$/, '');
            candidates.unshift((manifest.action_aliases || {})[actionName] || actionName);
        } else if (usage === 'folder' && manifest.folder_variants) {
            icons = manifest.folder_variants[settings['appearance.' + theme + '_folder_color']] || manifest.folder_variants.standard || {};
            candidates.push('folder');
        }
        for (const candidate of candidates) {
            const value = icons[candidate];
            // Manifest paths are local assets; never turn a malformed entry into a request.
            if (typeof value === 'string' && /^img\/[a-z0-9_/-]+\.(svg|png)$/.test(value) && !value.includes('..')) return '/' + value;
        }
        return '';
    }
    window.AuraDesktopIcons = Object.freeze({ themes, normalize, path });
}());
