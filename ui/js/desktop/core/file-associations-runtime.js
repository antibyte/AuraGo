(function () {
    'use strict';

    const AUDIO_EXTENSIONS = new Set(['mp3', 'wav', 'flac', 'ogg', 'm4a', 'opus']);
    const MEDIA_EXTENSIONS = new Set(['mp4', 'mkv', 'avi', 'mov', 'webm', ...AUDIO_EXTENSIONS]);

    function extensionForName(name) {
        const value = String(name || '').toLowerCase();
        if (value.endsWith('.fb2.zip')) return 'fb2.zip';
        if (value.endsWith('.txt.gz')) return 'txt.gz';
        const base = value.split(/[\\/]/).pop() || '';
        const dot = base.lastIndexOf('.');
        return dot <= 0 ? '' : base.slice(dot + 1);
    }

    function parse(value) {
        let input = value;
        if (typeof input === 'string') {
            try { input = JSON.parse(input); } catch (_) { return {}; }
        }
        if (!input || typeof input !== 'object' || Array.isArray(input)) return {};
        const result = {};
        Object.keys(input).forEach(extension => {
            const appId = input[extension];
            if (typeof appId !== 'string' || !appId.trim()) return;
            result[String(extension).toLowerCase().replace(/^\./, '')] = appId.trim();
        });
        return result;
    }

    function resolveCandidates(entry, helpers) {
        const h = helpers || {};
        const name = String(entry && (entry.name || entry.path) || '').split(/[\\/]/).pop() || '';
        const lower = name.toLowerCase();
        const extension = extensionForName(name);
        const apps = [];
        const add = (appId, labelKey, label) => apps.push({ appId, labelKey: labelKey || '', label: label || '' });
        const isText = /\.(txt|log)$/i.test(name);
        const isImage = typeof h.isImageFile === 'function' && h.isImageFile(name);
        const mediaKind = String(entry && (entry.media_kind || entry.mediaKind) || '').toLowerCase();
        const isMedia = (typeof h.isMediaFile === 'function' ? h.isMediaFile(name) : MEDIA_EXTENSIONS.has(extension)) || mediaKind === 'audio' || mediaKind === 'video';
        const isBookshelfFile = /\.(epub|pdf|mobi|azw|azw3|fb2(?:\.zip)?|cbz|txt(?:\.gz)?|html?|cbr|rtf|docx|odt|djvu|lit|pdb|lrf)$/i.test(name);

        if (isText) {
            add('editor', 'desktop.app_editor');
            add('code-studio', 'desktop.app_code_studio');
            add('viewer', 'desktop.app_viewer');
        } else if (/\.ora$/i.test(name)) {
            add('pixel', 'desktop.app_pixel');
        } else if (isImage) {
            add('gallery', 'desktop.app_gallery');
            add('viewer', 'desktop.app_viewer');
            add('code-studio', 'desktop.app_code_studio');
        } else if (isMedia) {
            if (AUDIO_EXTENSIONS.has(extension) || mediaKind === 'audio') {
                add('audio-player', 'desktop.app_audio_player');
                add('music-player', 'desktop.app_music_player');
            }
            add('gallery', 'desktop.app_gallery');
            add('viewer', 'desktop.app_viewer');
        } else if (typeof h.isWriterFile === 'function' && h.isWriterFile(entry)) {
            add('writer', 'desktop.app_writer');
            add('viewer', 'desktop.app_viewer');
        } else if (typeof h.isSheetsFile === 'function' && h.isSheetsFile(entry)) {
            add('sheets', 'desktop.app_sheets');
            add('viewer', 'desktop.app_viewer');
        } else if (/\.lyl$/i.test(name)) {
            add('layerling', '', 'Layerling');
        } else if (/\.md$/i.test(name)) {
            add('notes', 'desktop.app_notes');
            add('editor', 'desktop.app_editor');
            add('viewer', 'desktop.app_viewer');
        } else if (/\.aurasynth$/i.test(name)) {
            add('synth-studio', 'desktop.app_synth_studio');
            add('viewer', 'desktop.app_viewer');
        } else if (typeof h.isViewerFile === 'function' && h.isViewerFile(entry)) {
            add('viewer', 'desktop.app_viewer');
        } else if (typeof h.isArchiveFile === 'function' && h.isArchiveFile(name)) {
            add('zipper', 'desktop.app_zipper');
        } else {
            add('viewer', 'desktop.app_viewer');
            add('code-studio', 'desktop.app_code_studio');
        }

        if (isBookshelfFile) apps.unshift({ appId: 'bookshelf', labelKey: 'desktop.app_bookshelf', label: '' });
        if (/\.kicad_pro$/i.test(name) && String(entry && entry.path || '').startsWith('Shared/')) {
            const allApps = typeof h.allApps === 'function' ? h.allApps() : [];
            apps.unshift(allApps.some(app => app.id === 'store-ziro-designer')
                ? { appId: 'store-ziro-designer', labelKey: '', label: 'Ziro Designer' }
                : { appId: 'software-store', labelKey: 'desktop.app_software_store', label: '' });
        }
        if (/\.(stl|obj|3mf|step|stp|svg)$/i.test(name)) add('layerling', '', 'Layerling');

        const seen = new Set();
        const unique = apps.filter(app => {
            if (!app.appId || seen.has(app.appId)) return false;
            seen.add(app.appId);
            return true;
        }).map(app => Object.assign({ extension }, app));
        const availableApps = Array.isArray(h.availableApps) ? h.availableApps : typeof h.allApps === 'function' ? h.allApps() : null;
        return availableCandidates(unique, availableApps);
    }

    function list(value, apps, candidatesForExtension) {
        const associations = parse(value);
        const availableApps = Array.isArray(apps) ? apps : [];
        return Object.keys(associations).sort((a, b) => a.localeCompare(b)).map(extension => {
            const appId = associations[extension];
            const app = availableApps.find(candidate => candidate && candidate.id === appId) || null;
            const candidates = availableCandidates(typeof candidatesForExtension === 'function' ? candidatesForExtension(extension) : [], availableApps);
            return {
                extension,
                appId,
                app,
                available: Boolean(app),
                candidates: Array.isArray(candidates) ? candidates : []
            };
        });
    }

    function availableCandidates(candidates, apps) {
        if (!Array.isArray(candidates)) return [];
        if (!Array.isArray(apps)) return candidates.slice();
        const available = new Set(apps.filter(app => app && app.id).map(app => app.id));
        return candidates.filter(candidate => candidate && available.has(candidate.appId));
    }

    window.AuraDesktopFileAssociations = Object.freeze({ extensionForName, parse, resolveCandidates, availableCandidates, list });
}());
