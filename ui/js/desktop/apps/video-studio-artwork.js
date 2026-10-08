(function () {
    'use strict';

    // Titles and stickers: cropped text PNGs, sticker art and overlay placement.
    const P = window.VideoStudioParts;
    const { API, FPS, MAX_ACTIVE_ASSETS, MAX_CLIPS } = P;
    const I = (...args) => P.I(...args);
    const T = (...args) => P.T(...args);
    const activeAssetCount = (...args) => P.activeAssetCount(...args);
    const addTrackTo = (...args) => P.addTrackTo(...args);
    const clamp = (...args) => P.clamp(...args);
    const clone = (...args) => P.clone(...args);
    const idempotencyKey = (...args) => P.idempotencyKey(...args);
    const maxFrames = (...args) => P.maxFrames(...args);
    const mutate = (...args) => P.mutate(...args);
    const pollJob = (...args) => P.pollJob(...args);
    const refreshProject = (...args) => P.refreshProject(...args);
    const renderInspector = (...args) => P.renderInspector(...args);
    const renderJobs = (...args) => P.renderJobs(...args);
    const request = (...args) => P.request(...args);
    const scheduleSave = (...args) => P.scheduleSave(...args);
    const setInspectorOpen = (...args) => P.setInspectorOpen(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const totalClipCount = (...args) => P.totalClipCount(...args);
    const tr = (...args) => P.tr(...args);

    const STICKERS = ['sparkle', 'heart', 'sun', 'star', 'flower', 'burst'];
    const TITLE_PRESETS = {
        title: { textKey: 'defaultTitle', text: 'Your title', seconds: 4, anchor: 'mc', style: { font_family: 'Georgia', font_size: 104, color: '#ffffff', bold: true, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0 } },
        subtitle: { textKey: 'defaultSubtitle', text: 'Your subtitle', seconds: 4, anchor: 'bc', style: { font_family: 'Arial', font_size: 50, color: '#ffffff', bold: false, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0.55 } },
        lower: { textKey: 'defaultLowerThird', text: 'Name Surname\nRole', seconds: 5, anchor: 'bl', style: { font_family: 'Arial', font_size: 44, color: '#ffffff', bold: true, italic: false, alignment: 'left', background_color: '#101319', background_opacity: 0.72 } },
        credits: { textKey: 'defaultCredits', text: 'Directed by\nYour name\n\nThanks for watching', seconds: 6, anchor: 'mc', style: { font_family: 'Georgia', font_size: 52, color: '#ffffff', bold: false, italic: false, alignment: 'center', background_color: '#101319', background_opacity: 0 } }
    };
    function scaledFont(s, size) {
        const base = Math.min(Number(s.project.width || 1280), Number(s.project.height || 720));
        return Math.round(clamp(size * base / 720, 18, 220));
    }
    async function addTitle(s, presetName) {
        if (s.readonly || !s.project || !s.projectId) return;
        if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
        if (activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) return showNotice(s, 'assetLimit', 'This project reached its media limit.', true);
        const preset = TITLE_PRESETS[presetName] || TITLE_PRESETS.title;
        const projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame;
        const duration = Math.min(preset.seconds * FPS, maxFrames(s) - startFrame);
        if (duration < FPS) return showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true);
        if (!I().findOverlaySlot(s.project, startFrame, duration)) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        const text = tr(s, preset.textKey, preset.text);
        const style = Object.assign({}, preset.style, { font_size: scaledFont(s, preset.style.font_size) });
        try {
            const art = await renderTextPNG(s, text, style);
            const asset = await createArtworkAsset(s, art.file, text, projectId, epoch);
            if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            s.artworkKeys.set(asset.id, artworkKey(text, style));
            const box = { width: art.width / s.project.width, height: art.height / s.project.height };
            placeOverlay(s, asset, startFrame, duration, clip => {
                Object.assign(clip, I().clampBox(Object.assign(box, I().presetBox(preset.anchor, I().clampBox(Object.assign({ x: 0, y: 0 }, box))))));
                clip.text = text; clip.text_style = style;
            });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not create the title artwork.', true); }
    }
    function placeOverlay(s, asset, start, duration, configure) {
        const slot = I().findOverlaySlot(s.project, start, duration);
        if (!slot) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        const ok = mutate(s, 'add overlay', project => {
            const track = slot.trackId ? project.tracks.find(item => item.id === slot.trackId) : addTrackTo(s, project, 'overlay');
            const clip = T().makeClip(asset, start);
            clip.duration = duration;
            configure(clip);
            track.clips.push(clip);
            s.selectedClipId = clip.id; s.selectionRevision++;
        });
        if (ok) { renderInspector(s); T().revealFrame(s.q('[data-timeline]'), s, start); if (s.app.getBoundingClientRect().width <= 920) setInspectorOpen(s, true, false); }
    }
    function fontFor(style) { return `${style.italic ? 'italic ' : ''}${style.bold ? '700 ' : ''}${Number(style.font_size || 72)}px "${String(style.font_family || 'Arial').replace(/["\\]/g, '')}"`; }
    // Titles are PNGs cropped to their text block and positioned with the clip box like stickers.
    function renderTextPNG(s, text, style) {
        const W = Number(s.project.width || 1280), H = Number(s.project.height || 720);
        const size = Number(style.font_size || 72), font = fontFor(style);
        const measure = document.createElement('canvas').getContext('2d');
        measure.font = font;
        const maxWidth = W * 0.84;
        const wrapped = String(text || ' ').split(/\r?\n/).slice(0, 8).flatMap(line => wrapCanvasText(measure, line, maxWidth));
        const lineHeight = size * 1.2;
        const background = Number(style.background_opacity || 0) > 0;
        const padX = Math.round(size * (background ? 0.5 : 0.25)), padY = Math.round(size * (background ? 0.3 : 0.2));
        const textWidth = Math.max(1, ...wrapped.map(line => measure.measureText(line).width));
        const canvas = document.createElement('canvas');
        canvas.width = Math.ceil(Math.min(W, textWidth + padX * 2)); canvas.height = Math.ceil(Math.min(H, wrapped.length * lineHeight + padY * 2));
        const ctx = canvas.getContext('2d');
        if (background) { ctx.fillStyle = hexWithAlpha(style.background_color || '#101319', Number(style.background_opacity || 0.45)); ctx.fillRect(0, 0, canvas.width, canvas.height); }
        ctx.font = font; ctx.textBaseline = 'middle';
        ctx.textAlign = ['left', 'right'].includes(style.alignment) ? style.alignment : 'center';
        const x = style.alignment === 'left' ? padX : style.alignment === 'right' ? canvas.width - padX : canvas.width / 2;
        ctx.fillStyle = style.color || '#fff'; ctx.shadowColor = 'rgba(0,0,0,.6)'; ctx.shadowBlur = Math.max(2, size * 0.08);
        wrapped.forEach((line, index) => ctx.fillText(line, x, padY + lineHeight * (index + 0.5), canvas.width - padX * 2));
        return new Promise((resolve, reject) => canvas.toBlob(blob => blob ? resolve({ file: new File([blob], 'title-' + Date.now() + '.png', { type: 'image/png' }), width: canvas.width, height: canvas.height }) : reject(new Error('canvas_export_failed')), 'image/png'));
    }
    function wrapCanvasText(ctx, text, maxWidth) {
        const words = String(text || ' ').split(/\s+/), lines = []; let line = '';
        words.forEach(word => {
            const candidate = line ? line + ' ' + word : word;
            if (line && ctx.measureText(candidate).width > maxWidth) { lines.push(line); line = word; } else line = candidate;
        });
        lines.push(line || ' '); return lines;
    }
    function hexWithAlpha(hex, alpha) {
        const value = /^#[0-9a-f]{6}$/i.test(hex) ? hex : '#101319';
        return `rgba(${parseInt(value.slice(1, 3), 16)},${parseInt(value.slice(3, 5), 16)},${parseInt(value.slice(5, 7), 16)},${clamp(alpha, 0, 1)})`;
    }
    function textStyleKey(style) { const defaults = { font_family: 'Arial', font_size: 72, color: '#ffffff', bold: false, italic: false, alignment: 'center', outline_color: '', outline_width: 0, background_color: '#101319', background_opacity: 0 }; return Object.keys(defaults).map(key => style && style[key] || defaults[key]).join('|'); }
    // A title's PNG is derived from its text and style. Edits mark the clip stale and a per-clip
    // debounce regenerates it; the result amends the edit that is already on the undo stack.
    function artworkKey(text, style) { return String(text) + '\u0001' + textStyleKey(style); }
    function artworkPending(s, clipId) {
        return !!clipId && (s.staleArtwork.has(clipId) || s.artworkInFlight.has(clipId) || s.textTimers.has(clipId) || s.pendingArtwork.has(clipId));
    }
    function updateTextStatus(s) {
        const span = s.q('[data-text-status] span');
        if (!span) return;
        const pending = artworkPending(s, s.selectedClipId), failed = !pending && s.failedArtwork.has(s.selectedClipId);
        span.textContent = pending ? tr(s, 'textUpdating', 'Updating the title…') : failed ? tr(s, 'artworkFailed', 'Could not create or update the title artwork.') : tr(s, 'textAuto', 'Changes are applied automatically.');
        span.parentElement.classList.toggle('is-error', failed);
    }
    function markTextStale(s, clipId) {
        if (!clipId || s.readonly) return;
        s.failedArtwork.delete(clipId);
        s.staleArtwork.add(clipId);
        clearTimeout(s.textTimers.get(clipId));
        s.textTimers.set(clipId, window.setTimeout(() => { s.textTimers.delete(clipId); applyTextArtwork(s, clipId); }, 900));
        updateTextStatus(s);
    }
    function cancelTextApplies(s) {
        s.textTimers.forEach(timer => clearTimeout(timer));
        s.textTimers.clear(); s.staleArtwork.clear(); s.pendingArtwork.clear(); s.failedArtwork.clear();
    }
    // Undo/Redo invalidate pending applies; restored titles whose PNG no longer matches are regenerated.
    function resyncArtwork(s) {
        s.artworkRevision++;
        cancelTextApplies(s);
        (s.project ? s.project.tracks : []).forEach(track => track.clips.forEach(clip => {
            const key = clip.text && s.artworkKeys.get(clip.asset_id);
            if (key && key !== artworkKey(clip.text, clip.text_style || {})) markTextStale(s, clip.id);
        }));
        updateTextStatus(s);
    }
    function applyTextArtwork(s, clipId) {
        clipId = clipId || s.selectedClipId;
        clearTimeout(s.textTimers.get(clipId)); s.textTimers.delete(clipId);
        const found = s.project && clipId ? T().clipFor(s.project, clipId) : null;
        if (!found || !found.clip.text || s.readonly || found.track.locked) { s.staleArtwork.delete(clipId); updateTextStatus(s); return Promise.resolve(); }
        const projectId = s.projectId, epoch = s.projectEpoch;
        const clipRevision = (s.clipArtRevisions.get(clipId) || 0) + 1;
        s.clipArtRevisions.set(clipId, clipRevision);
        s.pendingArtwork.delete(clipId);
        s.failedArtwork.delete(clipId);
        const style = clone(found.clip.text_style || {});
        const result = { clipId, clipRevision, global: s.artworkRevision, originalAssetId: found.clip.asset_id, text: found.clip.text, style, styleKey: textStyleKey(style) };
        s.artworkInFlight.add(clipId);
        updateTextStatus(s);
        const job = (async () => {
            try {
                await Promise.resolve();
                const art = await renderTextPNG(s, result.text, result.style);
                const asset = await createArtworkAsset(s, art.file, 'Title · ' + result.text.slice(0, 40), projectId, epoch);
                if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
                s.artworkKeys.set(asset.id, artworkKey(result.text, result.style));
                Object.assign(result, { asset, width: art.width, height: art.height });
                // Drags and slider gestures own the clip box until they end; the result waits for them.
                if (s.timelineDragging || s.fieldGesture) s.pendingArtwork.set(clipId, result);
                else commitArtwork(s, result);
            } catch (_) {
                if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) {
                    s.staleArtwork.delete(clipId); s.failedArtwork.add(clipId);
                    showNotice(s, 'artworkFailed', 'Could not update the title artwork.', true);
                }
            }
            finally {
                if (s.artworkJobs.get(clipId) === job) { s.artworkJobs.delete(clipId); s.artworkInFlight.delete(clipId); }
                if (!s.disposed) updateTextStatus(s);
            }
        })();
        s.artworkJobs.set(clipId, job);
        return job;
    }
    // Commits only if the project, clip, text, normalized style, source PNG and both Apply revisions still match.
    function commitArtwork(s, result) {
        if (s.disposed || !s.project || result.global !== s.artworkRevision || s.clipArtRevisions.get(result.clipId) !== result.clipRevision) return;
        const current = T().clipFor(s.project, result.clipId);
        if (!current || current.clip.text !== result.text || textStyleKey(current.clip.text_style || {}) !== result.styleKey || current.clip.asset_id !== result.originalAssetId) return;
        const clip = current.clip, project = s.project, previous = project.assets.find(item => item.id === clip.asset_id);
        const before = { asset_id: clip.asset_id, x: clip.x, y: clip.y, width: clip.width, height: clip.height };
        const scale = previous && previous.width ? clamp(clip.width / (previous.width / project.width), 0.1, 10) : 1;
        const width = result.width / project.width * scale, height = result.height / project.height * scale;
        clip.asset_id = result.asset.id;
        Object.assign(clip, I().clampBox({ x: clip.x + clip.width / 2 - width / 2, y: clip.y + clip.height / 2 - height / 2, width, height }));
        if (!T().validTimeline(project)) { Object.assign(clip, before); return; }
        s.staleArtwork.delete(result.clipId);
        scheduleSave(s);
        T().render(s.q('[data-timeline]'), s.timelineOptions);
        renderInspector(s);
        s.preview.seek(s.frame);
        P.updateTransform(s);
    }
    // Results that arrived during a drag are committed once the gesture ends.
    function retryArtwork(s) {
        const pending = Array.from(s.pendingArtwork.values());
        s.pendingArtwork.clear();
        pending.forEach(result => commitArtwork(s, result));
        updateTextStatus(s);
    }
    // Close, project switches and export wait for debounced and running title applies.
    async function flushArtwork(s) {
        if (s.timelineDragging || s.fieldGesture || s.disposed) return;
        retryArtwork(s);
        new Set([...s.textTimers.keys(), ...s.staleArtwork]).forEach(id => { if (!s.artworkInFlight.has(id)) applyTextArtwork(s, id); });
        await Promise.allSettled(Array.from(s.artworkJobs.values()));
        retryArtwork(s);
    }
    async function createArtworkAsset(s, file, name, expectedProjectId, expectedEpoch) {
        const projectId = expectedProjectId || s.projectId, epoch = expectedEpoch == null ? s.projectEpoch : expectedEpoch;
        if (!projectId || projectId !== s.projectId || epoch !== s.projectEpoch || s.readonly) throw new Error('project_changed');
        const form = new FormData(); form.append('file', file, file.name || 'overlay.png');
        const response = await request(API + '/projects/' + encodeURIComponent(projectId) + '/media', { method: 'POST', headers: { 'Idempotency-Key': idempotencyKey() }, body: form, signal: s.abort.signal });
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        if (!response.body.job) return response.body.asset;
        s.jobs.unshift(response.body.job); renderJobs(s);
        const job = await pollJob(s, response.body.job.id, projectId, epoch);
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        if (!job || job.status !== 'succeeded') throw new Error('probe_failed');
        await refreshProject(s);
        if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) throw new Error('project_changed');
        const assetId = job.result && job.result.asset && job.result.asset.id || response.body.asset && response.body.asset.id;
        const canonical = s.project.assets.find(item => item.id === assetId);
        if (canonical) return canonical;
        throw new Error('asset_not_ready');
    }
    function stickerSVG(name, size) {
        const art = {
            sparkle: '<path d="M48 4 58 37 92 48 58 59 48 92 38 59 4 48 38 37z"/><circle cx="78" cy="22" r="5"/><circle cx="20" cy="78" r="4"/>',
            heart: '<path d="M48 82 15 51C-2 34 10 10 29 13c9 1 15 7 19 15 4-8 11-14 20-15 19-3 31 21 14 38z"/>',
            sun: '<circle cx="48" cy="48" r="19"/><path d="M48 5v17m0 52v17M5 48h17m52 0h17M18 18l12 12m36 36 12 12m4-60L70 30M30 66 18 78"/>',
            star: '<path d="m48 6 11 28 30 2-23 19 8 30-26-17-26 17 8-30L7 36l30-2z"/>',
            flower: '<g><ellipse cx="48" cy="24" rx="12" ry="23"/><ellipse cx="48" cy="72" rx="12" ry="23"/><ellipse cx="24" cy="48" rx="23" ry="12"/><ellipse cx="72" cy="48" rx="23" ry="12"/><circle cx="48" cy="48" r="11"/></g>',
            burst: '<path d="M48 4 56 35 82 16 65 42 96 48 65 55 82 80 56 62 48 94 41 62 15 80 32 55 1 48 32 42 15 16 41 35z"/>'
        };
        const fill = name === 'heart' ? '#fb7185' : name === 'sun' ? '#fbbf24' : name === 'flower' ? '#f472b6' : name === 'star' ? '#facc15' : '#a78bfa';
        const px = size || 512;
        return `<svg xmlns="http://www.w3.org/2000/svg" width="${px}" height="${px}" viewBox="0 0 96 96"><g fill="${fill}" stroke="#fff" stroke-width="2" stroke-linejoin="round">${art[name] || art.sparkle}</g></svg>`;
    }
    async function stickerPNG(name) {
        const blob = new Blob([stickerSVG(name, 512)], { type: 'image/svg+xml' }); const url = URL.createObjectURL(blob);
        try {
            const image = new Image(); image.src = url; await image.decode();
            const canvas = document.createElement('canvas'); canvas.width = 512; canvas.height = 512; canvas.getContext('2d').drawImage(image, 0, 0);
            return await new Promise((resolve, reject) => canvas.toBlob(blobValue => blobValue ? resolve(new File([blobValue], 'sticker-' + name + '.png', { type: 'image/png' })) : reject(new Error('canvas_export_failed')), 'image/png'));
        } finally { URL.revokeObjectURL(url); }
    }
    async function addSticker(s, name) {
        if (s.readonly || !s.project || !s.projectId) return;
        if (totalClipCount(s.project) >= MAX_CLIPS) return showNotice(s, 'clipLimit', 'This project reached its clip limit.', true);
        if (activeAssetCount(s.project) >= MAX_ACTIVE_ASSETS) return showNotice(s, 'assetLimit', 'This project reached its media limit.', true);
        const projectId = s.projectId, epoch = s.projectEpoch, startFrame = s.frame;
        const duration = Math.min(FPS * 4, maxFrames(s) - startFrame);
        if (duration < FPS) return showNotice(s, 'projectLimit', 'The project reached its 10-minute limit.', true);
        if (!I().findOverlaySlot(s.project, startFrame, duration)) return showNotice(s, 'overlayFull', 'Every overlay track is busy at the playhead. Move the playhead or free a track.', true);
        try {
            const file = await stickerPNG(name);
            if (s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const asset = await createArtworkAsset(s, file, tr(s, 'sticker_' + name, name), projectId, epoch);
            if (!asset || s.disposed || epoch !== s.projectEpoch || projectId !== s.projectId) return;
            const side = 0.24 * Math.min(s.project.width, s.project.height);
            const box = { width: side / s.project.width, height: side / s.project.height };
            placeOverlay(s, asset, startFrame, duration, clip => { Object.assign(clip, box, I().presetBox('tr', box)); });
        } catch (_) { if (!s.disposed && epoch === s.projectEpoch && projectId === s.projectId) showNotice(s, 'artworkFailed', 'Could not add this sticker.', true); }
    }

    Object.assign(P, { STICKERS, TITLE_PRESETS, scaledFont, addTitle, placeOverlay, fontFor, renderTextPNG, wrapCanvasText, hexWithAlpha, textStyleKey, applyTextArtwork, createArtworkAsset, stickerSVG, stickerPNG, addSticker, artworkKey, artworkPending, updateTextStatus, markTextStale, cancelTextApplies, resyncArtwork, commitArtwork, retryArtwork, flushArtwork });
})();
