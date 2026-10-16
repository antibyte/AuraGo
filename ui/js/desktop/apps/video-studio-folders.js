(function () {
    'use strict';

    // DOM-free folder model of the media bin (spec §4.8): path rules, the folder tree, usage counts, sorting and the
    // folder and name edits. Each edit changes the project in place and returns true or a REASON code. The project
    // document owns folders: asset.folder and project.folders (empty folders persist there).
    const LIMITS = Object.freeze({ depth: 4, segment: 64, path: 255, folders: 128 });
    const REASON = Object.freeze({ invalid: 'folder_invalid', exists: 'folder_exists', limit: 'folder_limit', inside: 'folder_inside', name: 'name_invalid', nothing: 'nothing' });
    const CONTROL = /[\x00-\x1f\x7f-\x9f]/;
    const collator = { numeric: true, sensitivity: 'base' };
    const runes = value => Array.from(value).length;

    // The canonical folder path: '' for the top level, null when the input is not a valid folder path.
    function normalizeFolder(raw) {
        const text = String(raw == null ? '' : raw).trim();
        if (!text) return '';
        const parts = text.split('/').map(part => part.trim());
        if (parts.length > LIMITS.depth || parts.some(part => !part || part === '.' || part === '..' || runes(part) > LIMITS.segment || CONTROL.test(part))) return null;
        const path = parts.join('/');
        return runes(path) > LIMITS.path ? null : path;
    }
    function parentOf(path) {
        const text = String(path || '');
        const index = text.lastIndexOf('/');
        return index < 0 ? '' : text.slice(0, index);
    }
    function nameOf(path) {
        const text = String(path || '');
        return text.slice(text.lastIndexOf('/') + 1);
    }
    function joinFolder(parent, name) { return normalizeFolder(parent ? parent + '/' + name : name); }
    // Every folder that exists: the project's own list, every asset's folder and all their ancestors.
    function folderPaths(project) {
        const set = new Set();
        const add = path => { for (let p = path; p; p = parentOf(p)) set.add(p); };
        (project.folders || []).forEach(add);
        (project.assets || []).forEach(asset => add(asset.folder || ''));
        return Array.from(set).sort((a, b) => a.localeCompare(b, undefined, collator));
    }
    function childFolders(project, parent) {
        const base = parent || '';
        return folderPaths(project).filter(path => parentOf(path) === base);
    }
    // Assets directly in a folder, or everything below it when recursive (the top level with recursive is all assets).
    function assetsIn(project, folder, recursive) {
        const base = folder || '';
        return (project.assets || []).filter(asset => {
            const at = asset.folder || '';
            return recursive ? !base || at === base || at.startsWith(base + '/') : at === base;
        });
    }
    // How many clips of the project use each asset.
    function usageCounts(project) {
        const counts = new Map();
        (project.tracks || []).forEach(track => (track.clips || []).forEach(clip => counts.set(clip.asset_id, (counts.get(clip.asset_id) || 0) + 1)));
        return counts;
    }
    function frameRateOf(asset) {
        const [num, den] = String(asset.source_fps || '').split('/').map(Number);
        return num > 0 && den > 0 ? num / den : 0;
    }
    function sortValue(asset, key, usage, fps) {
        switch (key) {
            case 'duration': return Number(asset.duration_us) || (Number(asset.duration_frames) || 0) * 1e6 / fps;
            case 'resolution': return (Number(asset.width) || 0) * (Number(asset.height) || 0);
            case 'frame_rate': return frameRateOf(asset);
            case 'codec': return String(asset.codec || '');
            case 'size': return Number(asset.size_bytes) || 0;
            case 'usage': return usage.get(asset.id) || 0;
            default: return String(asset.name || '');
        }
    }
    // A sorted copy by one list column (name, duration, resolution, frame_rate, codec, size, usage). Ties keep name order.
    function sortAssets(assets, key, direction, ctx) {
        const context = ctx || {}, usage = context.usage || new Map(), fps = Number(context.fps) || 30;
        const sign = direction === 'desc' ? -1 : 1;
        const byName = (a, b) => String(a.name || '').localeCompare(String(b.name || ''), undefined, collator);
        return assets.slice().sort((a, b) => {
            const x = sortValue(a, key, usage, fps), y = sortValue(b, key, usage, fps);
            const order = typeof x === 'string' ? x.localeCompare(y, undefined, collator) : x - y;
            return order ? order * sign : byName(a, b);
        });
    }
    function ensureFolders(project) {
        if (!Array.isArray(project.folders)) project.folders = [];
        return project.folders;
    }
    function createFolder(project, path) {
        const folder = normalizeFolder(path);
        if (!folder) return REASON.invalid;
        if (folderPaths(project).includes(folder)) return REASON.exists;
        const list = ensureFolders(project);
        if (list.length >= LIMITS.folders) return REASON.limit;
        list.push(folder);
        return true;
    }
    // Re-roots a path from `from` to `to` when it is `from` or lies below it. An empty `to` (the top level) drops the
    // leading segments. Other paths stay as they are.
    function moved(path, from, to) {
        if (path === from) return to;
        if (!path.startsWith(from + '/')) return path;
        return to ? to + path.slice(from.length) : path.slice(from.length + 1);
    }
    // True when every folder at or below `from` is still a valid path after the move (depth and length limits).
    function fitsAfterMove(project, from, to) {
        const below = path => path === from || path.startsWith(from + '/');
        const paths = (project.folders || []).concat((project.assets || []).map(asset => asset.folder || '')).filter(below);
        return paths.every(path => { const next = moved(path, from, to); return normalizeFolder(next) === next; });
    }
    // Applies a folder move to every asset and to project.folders. Deleting a folder also drops its own entry.
    function applyMove(project, from, to, removeSelf) {
        (project.assets || []).forEach(asset => {
            const next = moved(asset.folder || '', from, to);
            if (next) asset.folder = next; else delete asset.folder;
        });
        const list = ensureFolders(project).filter(path => !(removeSelf && path === from));
        project.folders = Array.from(new Set(list.map(path => moved(path, from, to)).filter(Boolean)));
    }
    function renameFolder(project, from, to) {
        const target = normalizeFolder(to);
        if (!target) return REASON.invalid;
        if (target === from || !folderPaths(project).includes(from)) return REASON.nothing;
        if (target.startsWith(from + '/')) return REASON.inside;
        if (folderPaths(project).includes(target)) return REASON.exists;
        if (!fitsAfterMove(project, from, target)) return REASON.invalid;
        applyMove(project, from, target, false);
        return true;
    }
    function deleteFolder(project, path) {
        if (!path || !folderPaths(project).includes(path)) return REASON.nothing;
        applyMove(project, path, parentOf(path), true);
        return true;
    }
    // Moves assets into a folder ('' is the top level). A folder exists once it holds an asset or is listed.
    function moveAssets(project, ids, folder) {
        const target = normalizeFolder(folder);
        if (target === null) return REASON.invalid;
        const wanted = new Set(ids || []);
        let changed = false;
        (project.assets || []).forEach(asset => {
            if (!wanted.has(asset.id) || (asset.folder || '') === target) return;
            if (target) asset.folder = target; else delete asset.folder;
            changed = true;
        });
        return changed ? true : REASON.nothing;
    }
    function renameAsset(project, id, name) {
        const value = String(name == null ? '' : name).trim();
        const asset = (project.assets || []).find(item => item.id === id);
        if (!asset || !value || runes(value) > 255) return REASON.name;
        if (asset.name === value) return REASON.nothing;
        asset.name = value;
        return true;
    }

    window.VideoStudioFolders = { LIMITS, REASON, normalizeFolder, parentOf, nameOf, joinFolder, folderPaths, childFolders, assetsIn, usageCounts, sortAssets, createFolder, renameFolder, deleteFolder, moveAssets, renameAsset };
})();
