(function () {
    'use strict';

    const FPS = 30;
    const MARGIN = 0.04;
    const TRANSITIONS = ['none', 'dissolve', 'black', 'wipeleft', 'wiperight'];
    const FONTS = ['Arial', 'Georgia', 'Trebuchet MS', 'Impact', 'Verdana', 'Courier New'];
    const round6 = value => Math.round(value * 1e6) / 1e6;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));

    // Human time: m:ss.cc (centiseconds) instead of raw 30 fps frame numbers.
    function formatTime(frames) {
        const total = Math.max(0, Math.round(Number(frames) || 0));
        const seconds = Math.floor(total / FPS), cs = Math.min(99, Math.round((total % FPS) * 100 / FPS));
        const h = Math.floor(seconds / 3600), m = Math.floor(seconds % 3600 / 60), s = seconds % 60;
        const tail = String(s).padStart(2, '0') + '.' + String(cs).padStart(2, '0');
        return h ? `${h}:${String(m).padStart(2, '0')}:${tail}` : `${m}:${tail}`;
    }

    function parseTime(text) {
        const raw = String(text == null ? '' : text).trim().replace(',', '.');
        if (!raw || !/^\d+(:\d+){0,2}(\.\d+)?$/.test(raw)) return null;
        const parts = raw.split(':').map(Number);
        const seconds = parts.pop();
        if (parts.length && seconds >= 60) return null;
        if (parts.length === 2 && parts[1] >= 60) return null;
        let total = seconds, unit = 60;
        for (let i = parts.length - 1; i >= 0; i--) { total += parts[i] * unit; unit *= 60; }
        return Number.isFinite(total) ? Math.round(total * FPS) : null;
    }

    function formatSeconds(frames) { return String(Math.round((Number(frames) || 0) / FPS * 10) / 10); }

    function clampBox(box) {
        const width = clamp(Number(box.width) || 0, 0.01, 1), height = clamp(Number(box.height) || 0, 0.01, 1);
        return { x: round6(clamp(Number(box.x) || 0, 0, 1 - width)), y: round6(clamp(Number(box.y) || 0, 0, 1 - height)), width: round6(width), height: round6(height) };
    }

    function presetBox(anchor, box) {
        const w = Number(box.width) || 1, h = Number(box.height) || 1;
        const place = (code, size) => code === 'c' || code === 'm' ? (1 - size) / 2 : code === 't' || code === 'l' ? Math.min(MARGIN, 1 - size) : 1 - size - MARGIN;
        const row = String(anchor || 'mc')[0], col = String(anchor || 'mc')[1];
        return { x: round6(clamp(place(col, w), 0, Math.max(0, 1 - w))), y: round6(clamp(place(row, h), 0, Math.max(0, 1 - h))) };
    }

    function scaleBox(box, size, aspect) {
        const ratio = Number(aspect) > 0 ? Number(aspect) : (Number(box.height) || 1) / (Number(box.width) || 1);
        let width = clamp(Number(size) || 0.01, 0.01, 1), height = width * ratio;
        if (height > 1) { width /= height; height = 1; }
        const cx = (Number(box.x) || 0) + (Number(box.width) || 0) / 2, cy = (Number(box.y) || 0) + (Number(box.height) || 0) / 2;
        return clampBox({ x: cx - width / 2, y: cy - height / 2, width, height });
    }

    function findOverlaySlot(project, start, duration) {
        const overlays = (project.tracks || []).filter(track => track.kind === 'overlay');
        const end = start + duration;
        const free = overlays.find(track => !track.locked && (track.clips || []).every(clip => clip.start + clip.duration <= start || clip.start >= end));
        if (free) return { trackId: free.id };
        return overlays.length < 4 ? { create: true } : null;
    }

    // Front-most visual track on top (it paints last), audio tracks below.
    function displayTracks(tracks) {
        const list = tracks || [];
        return list.filter(track => track.kind !== 'audio').reverse().concat(list.filter(track => track.kind === 'audio'));
    }

    function insertIndexFor(tracks, kind) {
        const list = tracks || [];
        if (kind !== 'video') return list.length;
        for (let i = list.length - 1; i >= 0; i--) if (list[i].kind === 'video') return i + 1;
        const overlay = list.findIndex(track => track.kind === 'overlay');
        return overlay >= 0 ? overlay : 0;
    }

    function slider(v, field, label, min, max, step, value, output) {
        return `<label class="vs-slider"><span>${v.esc(label)}</span><input type="range" data-field="${field}" min="${min}" max="${max}" step="${step}" value="${value}"><output>${v.esc(output)}</output></label>`;
    }

    function emptyMarkup(v) {
        const p = v.project || {}, summary = v.summary || {};
        const ratio = p.width && p.height ? (p.width === p.height ? '1:1' : p.width > p.height ? '16:9' : '9:16') : '';
        const tips = [['tipImport', 'Import videos, music or pictures in the library on the left.'], ['tipPlace', 'Drag media onto the timeline or use + to place it at the playhead.'], ['tipSelect', 'Select a clip to adjust timing, picture, sound and transitions here.'], ['tipExport', 'Export creates an MP4 you can download.']];
        return `<div class="vs-insp-empty"><section class="vs-insp-section"><h3>${v.icon('project', 14)}${v.esc(v.tr('projectDetails', 'Project'))}</h3><dl class="vs-facts"><div><dt>${v.esc(v.tr('format', 'Format'))}</dt><dd>${v.esc(ratio)} · ${Number(p.width || 0)}×${Number(p.height || 0)}</dd></div><div><dt>${v.esc(v.tr('length', 'Length'))}</dt><dd>${v.esc(formatTime(summary.durationFrames || 0))}</dd></div><div><dt>${v.esc(v.tr('clips', 'Clips'))}</dt><dd>${Number(summary.clipCount || 0)}</dd></div></dl></section><section class="vs-insp-section vs-insp-tips"><h3>${v.icon('info', 14)}${v.esc(v.tr('howItWorks', 'How it works'))}</h3><ol>${tips.map(([key, fallback]) => `<li>${v.esc(v.tr(key, fallback))}</li>`).join('')}</ol><p class="vs-hint">${v.esc(v.tr('selectClipHint', 'Nothing selected. Click a clip on the timeline to edit it.'))}</p></section></div>`;
    }

    function timeSection(v, clip, asset) {
        const offset = asset.kind === 'image' || !!clip.text ? '' : `<label class="vs-field vs-field-wide"><span>${v.esc(v.tr('sourceStart', 'Starts in source at'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="offset" value="${v.esc(formatTime(clip.offset || 0))}"></label>`;
        return `<section class="vs-insp-section"><h3>${v.icon('clock', 14)}${v.esc(v.tr('timing', 'Timing'))}</h3><div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('start', 'Start'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="start" value="${v.esc(formatTime(clip.start))}"></label><label class="vs-field"><span>${v.esc(v.tr('duration', 'Duration'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="duration" value="${v.esc(formatTime(clip.duration))}"></label></div>${offset}<p class="vs-hint">${v.esc(v.tr('timeHint', 'Enter seconds (4,5) or minutes:seconds (1:04.5).'))}</p></section>`;
    }

    function pictureSection(v, clip) {
        const anchors = ['tl', 'tc', 'tr', 'ml', 'mc', 'mr', 'bl', 'bc', 'br'];
        const isText = !!clip.text;
        const size = Math.round(Number(clip.width || 1) * 100);
        const fine = ['x', 'y', 'width', 'height'].map(field => `<label class="vs-field"><span>${v.esc(v.tr('box_' + field, field.toUpperCase()))}</span><input type="number" data-field="${field}" min="${field === 'width' || field === 'height' ? 1 : 0}" max="100" step="0.5" value="${round6(Number(clip[field] || 0) * 100)}"></label>`).join('');
        return `<section class="vs-insp-section"><h3>${v.icon('image', 14)}${v.esc(v.tr('picture', 'Picture'))}</h3><div class="vs-position"><div class="vs-position-grid" role="group" aria-label="${v.esc(v.tr('position', 'Position'))}">${anchors.map(anchor => `<button type="button" data-position="${anchor}" title="${v.esc(v.tr('pos_' + anchor, anchor))}" aria-label="${v.esc(v.tr('pos_' + anchor, anchor))}"><i></i></button>`).join('')}</div>${isText ? '' : `<button type="button" class="vs-chip-button" data-action="fill-frame">${v.icon('fullscreen', 14)}${v.esc(v.tr('fillFrame', 'Full frame'))}</button>`}</div>${isText ? '' : slider(v, 'size', v.tr('size', 'Size'), 5, 100, 1, size, size + ' %')}${slider(v, 'rotation', v.tr('rotation', 'Rotation'), -180, 180, 1, Math.round(Number(clip.rotation || 0)), Math.round(Number(clip.rotation || 0)) + '°')}${slider(v, 'opacity', v.tr('opacity', 'Opacity'), 0, 100, 1, Math.round(Number(clip.opacity == null ? 1 : clip.opacity) * 100), Math.round(Number(clip.opacity == null ? 1 : clip.opacity) * 100) + ' %')}${isText ? '' : `<div class="vs-segmented" role="group" aria-label="${v.esc(v.tr('fit', 'Fit'))}"><button type="button" data-fit="contain" aria-pressed="${clip.fit !== 'cover'}">${v.esc(v.tr('contain', 'Fit inside'))}</button><button type="button" data-fit="cover" aria-pressed="${clip.fit === 'cover'}">${v.esc(v.tr('cover', 'Fill'))}</button></div>`}<details class="vs-fine"><summary>${v.esc(v.tr('fineTune', 'Fine tune (percent of the frame)'))}</summary><div class="vs-field-grid">${fine}</div></details></section>`;
    }

    function soundSection(v, clip) {
        const volume = Math.round(Number(clip.volume == null ? 1 : clip.volume) * 100);
        const maxFade = Math.max(0, Math.min(clip.duration, FPS * 5));
        return `<section class="vs-insp-section"><h3>${v.icon('speaker', 14)}${v.esc(v.tr('sound', 'Sound'))}</h3>${slider(v, 'volume', v.tr('volume', 'Volume'), 0, 200, 1, volume, volume + ' %')}${slider(v, 'fade_in', v.tr('fadeInSound', 'Fade in'), 0, maxFade, 3, Math.min(maxFade, clip.fade_in || 0), formatSeconds(clip.fade_in || 0) + ' s')}${slider(v, 'fade_out', v.tr('fadeOutSound', 'Fade out'), 0, maxFade, 3, Math.min(maxFade, clip.fade_out || 0), formatSeconds(clip.fade_out || 0) + ' s')}</section>`;
    }

    function transitionSection(v, clip, info) {
        const current = clip.transition && clip.transition.type || 'none';
        const next = info && info.next;
        const maxFrames = next ? Math.max(3, Math.min(90, Math.min(clip.duration, next.duration) - 1)) : 90;
        const duration = clip.transition && clip.transition.duration || Math.min(15, maxFrames);
        const disabled = next ? '' : 'disabled';
        return `<section class="vs-insp-section"><h3>${v.icon('transition', 14)}${v.esc(v.tr('transitionNext', 'Transition to next clip'))}</h3><div class="vs-transition-grid" role="group" aria-label="${v.esc(v.tr('transition', 'Transition'))}">${TRANSITIONS.map(type => `<button type="button" data-transition="${type}" aria-pressed="${current === type}" ${disabled}><i class="vs-tx-${type}"></i><span>${v.esc(v.tr(type, type))}</span></button>`).join('')}</div><label class="vs-slider"><span>${v.esc(v.tr('transitionLength', 'Length'))}</span><input type="range" data-transition-duration min="3" max="${maxFrames}" step="3" value="${Math.min(duration, maxFrames)}" ${next && current !== 'none' ? '' : 'disabled'}><output>${v.esc(formatSeconds(duration))} s</output></label>${next ? '' : `<p class="vs-hint">${v.esc(v.tr('transitionNeedsNextHint', 'Place another clip right after this one on the same track to add a transition.'))}</p>`}</section>`;
    }

    function textSection(v, clip) {
        const style = clip.text_style || {};
        const font = style.font_family || 'Arial';
        const opacity = style.background_opacity == null ? 0 : Number(style.background_opacity);
        return `<section class="vs-insp-section vs-text-group"><h3>${v.icon('text', 14)}${v.esc(v.tr('text', 'Text'))}</h3><label class="vs-field vs-field-wide"><span>${v.esc(v.tr('titleText', 'Text'))}</span><textarea data-text-field="text" rows="3">${v.esc(clip.text)}</textarea></label><div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('font', 'Font'))}</span><select data-text-style="font_family">${FONTS.map(name => `<option ${name === font ? 'selected' : ''}>${v.esc(name)}</option>`).join('')}</select></label><label class="vs-field vs-field-narrow"><span>${v.esc(v.tr('fontSize', 'Size'))}</span><input type="number" min="18" max="220" data-text-style="font_size" value="${Number(style.font_size || 72)}"></label></div><div class="vs-field-row vs-text-tools"><label class="vs-color"><span>${v.esc(v.tr('color', 'Color'))}</span><input type="color" data-text-style="color" value="${v.esc(style.color || '#ffffff')}"></label><div class="vs-segmented vs-icon-segmented" role="group" aria-label="${v.esc(v.tr('alignment', 'Alignment'))}">${[['left', 'alignLeft'], ['center', 'alignCenter'], ['right', 'alignRight']].map(([value, icon]) => `<button type="button" data-text-align="${value}" aria-pressed="${(style.alignment || 'center') === value}" title="${v.esc(v.tr('align_' + value, value))}" aria-label="${v.esc(v.tr('align_' + value, value))}">${v.icon(icon, 15)}</button>`).join('')}</div><div class="vs-segmented vs-icon-segmented" role="group"><button type="button" data-text-toggle="bold" aria-pressed="${!!style.bold}" title="${v.esc(v.tr('bold', 'Bold'))}" aria-label="${v.esc(v.tr('bold', 'Bold'))}"><b>B</b></button><button type="button" data-text-toggle="italic" aria-pressed="${!!style.italic}" title="${v.esc(v.tr('italic', 'Italic'))}" aria-label="${v.esc(v.tr('italic', 'Italic'))}"><i>I</i></button></div></div><div class="vs-field-row"><label class="vs-color"><span>${v.esc(v.tr('background', 'Background'))}</span><input type="color" data-text-style="background_color" value="${v.esc(style.background_color || '#101319')}"></label><label class="vs-slider vs-slider-compact"><span>${v.esc(v.tr('backgroundOpacity', 'Background opacity'))}</span><input type="range" min="0" max="1" step="0.05" data-text-style="background_opacity" value="${opacity}"><output>${Math.round(opacity * 100)} %</output></label></div><div class="vs-text-status" data-text-status><span>${v.esc(v.textPending ? v.tr('textUpdating', 'Updating the title…') : v.tr('textAuto', 'Changes are applied automatically.'))}</span><button type="button" data-action="apply-text" class="vs-link-button">${v.esc(v.tr('applyText', 'Apply now'))}</button></div></section>`;
    }

    function render(host, v) {
        if (!host) return;
        if (!v.selected) { host.innerHTML = emptyMarkup(v); return; }
        const { clip, track } = v.selected;
        const asset = v.asset || {};
        const kind = clip.text ? 'text' : asset.kind || track.kind;
        const icon = kind === 'text' ? 'text' : kind === 'audio' ? 'audio' : kind === 'image' ? 'image' : 'video';
        const title = clip.text ? clip.text.split(/\r?\n/)[0] : asset.name || v.tr('missingMedia', 'Missing media');
        const audioCapable = track.kind === 'audio' || asset.kind === 'video' && asset.has_audio && track.kind !== 'overlay';
        const visual = track.kind !== 'audio';
        const thumb = v.thumb ? ` style="background-image:url('${v.esc(v.thumb)}')"` : '';
        const lock = track.locked ? `<p class="vs-insp-locked">${v.icon('lock', 14)}${v.esc(v.tr('trackLocked', 'This track is locked. Unlock it in the timeline to edit.'))}</p>` : '';
        host.innerHTML = `<div class="vs-insp-head"><span class="vs-insp-thumb vs-kind-${v.esc(kind)}"${thumb}>${thumb ? '' : v.icon(icon, 18)}</span><div class="vs-insp-title"><strong title="${v.esc(title)}">${v.esc(title)}</strong><small>${v.esc(track.name)} · ${v.esc(formatTime(clip.start))} – ${v.esc(formatTime(clip.start + clip.duration))}</small></div></div><div class="vs-insp-actions"><button type="button" data-action="clip-split" title="${v.esc(v.tr('splitHintShort', 'Split at the playhead (S)'))}">${v.icon('scissors', 15)}<span>${v.esc(v.tr('split', 'Split'))}</span></button><button type="button" data-action="clip-duplicate">${v.icon('copy', 15)}<span>${v.esc(v.tr('duplicate', 'Duplicate'))}</span></button><button type="button" data-action="clip-delete" class="vs-danger">${v.icon('trash', 15)}<span>${v.esc(v.tr('delete', 'Delete'))}</span></button></div>${lock}${clip.text ? textSection(v, clip) : ''}${timeSection(v, clip, asset)}${visual ? pictureSection(v, clip) : ''}${audioCapable ? soundSection(v, clip) : ''}${visual ? transitionSection(v, clip, v.transitionInfo) : ''}`;
    }

    window.VideoStudioInspector = { render, formatTime, parseTime, formatSeconds, presetBox, scaleBox, clampBox, findOverlaySlot, displayTracks, insertIndexFor, FONTS };
})();
