(function () {
    'use strict';

    const MARGIN = 0.04;
    const TRANSITIONS = ['none', 'dissolve', 'black', 'wipeleft', 'wiperight'];
    const FONTS = ['Arial', 'Georgia', 'Trebuchet MS', 'Impact', 'Verdana', 'Courier New'];
    const round6 = value => Math.round(value * 1e6) / 1e6;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));

    const Time = () => window.VideoStudioTime;
    // Kept for app parts and tests; the project is optional (30 fps when missing).
    function formatTime(frames, project) { return Time().formatTime(frames, project); }
    function parseTime(text, project) { return Time().parseTime(text, project); }
    function formatSeconds(frames, locale, project) { return Time().formatSeconds(frames, project, locale); }
    function aspectLabel(width, height) {
        const gcd = (a, b) => b ? gcd(b, a % b) : a, d = gcd(width, height) || 1;
        return width && height ? `${width / d}:${height / d}` : '';
    }

    // Rounds the size first and keeps position + size within the frame even after float rounding.
    function fitAxis(position, size) {
        let value = round6(clamp(Number(position) || 0, 0, 1 - size));
        for (let i = 0; i < 4 && value + size > 1; i++) value = Math.max(0, round6(value - 1e-6));
        return value;
    }
    function clampBox(box) {
        const width = round6(clamp(Number(box.width) || 0, 0.01, 1)), height = round6(clamp(Number(box.height) || 0, 0.01, 1));
        return { x: fitAxis(box.x, width), y: fitAxis(box.y, height), width, height };
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

    // Marker colour dots use the timeline palette only; anything else is not drawn.
    function palette(color) { return !!color && window.VideoStudioTimeline.LABEL_COLORS.includes(color); }
    // The project view's marker list: jump to a marker, edit it, and the in and out points.
    function markersSection(v) {
        const markers = v.markers || [], io = v.inOut || {}, at = frame => (frame == null ? '–' : formatTime(frame, v.project));
        const rows = markers.map(marker => {
            const name = marker.name || v.tr('marker', 'Marker'), dot = palette(marker.color) ? ` style="--vs-marker-color:var(--vs-label-${marker.color})"` : '';
            const edit = v.tr('markerEdit', 'Edit marker…');
            return `<li><button type="button" class="vs-chip-button" data-marker-jump="${v.esc(marker.id)}"><span class="vs-marker-dot"${dot}></span><span>${v.esc(formatTime(marker.frame, v.project))}</span><span>${v.esc(name)}</span></button><button type="button" class="vs-icon-button" data-marker-edit="${v.esc(marker.id)}" title="${v.esc(edit)}" aria-label="${v.esc(edit)}">${v.icon('edit', 14)}</button></li>`;
        }).join('');
        const range = io.in == null && io.out == null
            ? `<p class="vs-hint">${v.esc(v.tr('noInOut', 'No in or out point set.'))}</p>`
            : `<p class="vs-hint">${v.esc(v.tr('markIn', 'In'))} ${v.esc(at(io.in))} · ${v.esc(v.tr('markOut', 'Out'))} ${v.esc(at(io.out))} <button type="button" class="vs-link-button" data-action="clear-inout">${v.esc(v.tr('clearInOut', 'Clear in and out'))}</button></p>`;
        return `<section class="vs-insp-section"><h3>${v.icon('flag', 14)}${v.esc(v.tr('markersList', 'Markers'))}</h3>${range}${rows ? `<ul class="vs-marker-list">${rows}</ul>` : `<p class="vs-hint">${v.esc(v.tr('noMarkers', 'No markers yet. Press M at the playhead to add one.'))}</p>`}</section>`;
    }

    function emptyMarkup(v) {
        const p = v.project || {}, summary = v.summary || {};
        const ratio = aspectLabel(Number(p.width) || 0, Number(p.height) || 0);
        const tips = [['tipImport', 'Import videos, music or pictures in the library on the left.'], ['tipPlace', 'Drag media onto the timeline or use + to place it at the playhead.'], ['tipSelect', 'Select a clip to adjust timing, picture, sound and transitions here.'], ['tipExport', 'Export creates an MP4 you can download.']];
        return `<div class="vs-insp-empty"><section class="vs-insp-section"><h3>${v.icon('project', 14)}${v.esc(v.tr('projectDetails', 'Project'))}</h3><dl class="vs-facts"><div><dt>${v.esc(v.tr('format', 'Format'))}</dt><dd>${v.esc(ratio)} · ${Number(p.width || 0)}×${Number(p.height || 0)} · ${v.esc(Time().rateLabel(p, v.locale))}</dd></div><div><dt>${v.esc(v.tr('length', 'Length'))}</dt><dd>${v.esc(formatTime(summary.durationFrames || 0, v.project))}</dd></div><div><dt>${v.esc(v.tr('clips', 'Clips'))}</dt><dd>${Number(summary.clipCount || 0)}</dd></div></dl></section>${markersSection(v)}<section class="vs-insp-section vs-insp-tips"><h3>${v.icon('info', 14)}${v.esc(v.tr('howItWorks', 'How it works'))}</h3><ol>${tips.map(([key, fallback]) => `<li>${v.esc(v.tr(key, fallback))}</li>`).join('')}</ol><p class="vs-hint">${v.esc(v.tr('selectClipHint', 'Nothing selected. Click a clip on the timeline to edit it.'))}</p></section></div>`;
    }

    function timeSection(v, clip, asset) {
        const offset = asset.kind === 'image' || !!clip.text ? '' : `<label class="vs-field vs-field-wide"><span>${v.esc(v.tr('sourceStart', 'Starts in source at'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="offset" value="${v.esc(formatTime(clip.offset || 0, v.project))}"></label>`;
        return `<section class="vs-insp-section"><h3>${v.icon('clock', 14)}${v.esc(v.tr('timing', 'Timing'))}</h3><div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('start', 'Start'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="start" value="${v.esc(formatTime(clip.start, v.project))}"></label><label class="vs-field"><span>${v.esc(v.tr('duration', 'Duration'))}</span><input type="text" inputmode="decimal" spellcheck="false" data-field="duration" value="${v.esc(formatTime(clip.duration, v.project))}"></label></div>${offset}<p class="vs-hint">${v.esc(v.tr('timeHint', 'Enter seconds (4,5) or minutes:seconds (1:04.5).'))}</p></section>`;
    }

    function pictureSection(v, clip) {
        const anchors = ['tl', 'tc', 'tr', 'ml', 'mc', 'mr', 'bl', 'bc', 'br'];
        const isText = !!clip.text;
        const size = Math.round(Number(clip.width || 1) * 100);
        const fine = ['x', 'y', 'width', 'height'].map(field => `<label class="vs-field"><span>${v.esc(v.tr(field === 'x' || field === 'y' ? 'box_' + field : field, field.toUpperCase()))}</span><input type="number" data-field="${field}" min="${field === 'width' || field === 'height' ? 1 : 0}" max="100" step="0.5" value="${round6(Number(clip[field] || 0) * 100)}"></label>`).join('');
        return `<section class="vs-insp-section"><h3>${v.icon('image', 14)}${v.esc(v.tr('picture', 'Picture'))}</h3><div class="vs-position"><div class="vs-position-grid" role="group" aria-label="${v.esc(v.tr('position', 'Position'))}">${anchors.map(anchor => `<button type="button" data-position="${anchor}" title="${v.esc(v.tr('pos_' + anchor, anchor))}" aria-label="${v.esc(v.tr('pos_' + anchor, anchor))}"><i></i></button>`).join('')}</div>${isText ? '' : `<button type="button" class="vs-chip-button" data-action="fill-frame">${v.icon('fullscreen', 14)}${v.esc(v.tr('fillFrame', 'Full frame'))}</button>`}</div>${isText ? '' : slider(v, 'size', v.tr('size', 'Size'), 5, 100, 1, size, size + ' %')}${slider(v, 'rotation', v.tr('rotation', 'Rotation'), -180, 180, 1, Math.round(Number(clip.rotation || 0)), Math.round(Number(clip.rotation || 0)) + '°')}${slider(v, 'opacity', v.tr('opacity', 'Opacity'), 0, 100, 1, Math.round(Number(clip.opacity == null ? 1 : clip.opacity) * 100), Math.round(Number(clip.opacity == null ? 1 : clip.opacity) * 100) + ' %')}${isText ? '' : `<div class="vs-segmented" role="group" aria-label="${v.esc(v.tr('fit', 'Fit'))}"><button type="button" data-fit="contain" aria-pressed="${clip.fit !== 'cover'}">${v.esc(v.tr('contain', 'Fit inside'))}</button><button type="button" data-fit="cover" aria-pressed="${clip.fit === 'cover'}">${v.esc(v.tr('cover', 'Fill'))}</button></div>`}<div class="vs-fine"><button type="button" class="vs-link-button vs-fine-toggle" data-action="toggle-fine" aria-expanded="${!!v.fineOpen}">${v.icon('chevronDown', 14)}<span>${v.esc(v.tr('fineTune', 'Fine tune (percent of the frame)'))}</span></button><div class="vs-field-grid" ${v.fineOpen ? '' : 'hidden'}>${fine}</div></div></section>`;
    }

    function soundSection(v, clip) {
        const volume = Math.round(Number(clip.volume == null ? 1 : clip.volume) * 100);
        const rate = Time().rate(v.project), step = Math.max(1, Math.round(rate / 10));
        const maxFade = Math.max(0, Math.min(clip.duration, Math.round(rate * 5)));
        return `<section class="vs-insp-section"><h3>${v.icon('speaker', 14)}${v.esc(v.tr('sound', 'Sound'))}</h3>${slider(v, 'volume', v.tr('volume', 'Volume'), 0, 200, 1, volume, volume + ' %')}${slider(v, 'fade_in', v.tr('fadeInSound', 'Fade in'), 0, maxFade, step, Math.min(maxFade, clip.fade_in || 0), formatSeconds(clip.fade_in || 0, v.locale, v.project) + ' s')}${slider(v, 'fade_out', v.tr('fadeOutSound', 'Fade out'), 0, maxFade, step, Math.min(maxFade, clip.fade_out || 0), formatSeconds(clip.fade_out || 0, v.locale, v.project) + ' s')}</section>`;
    }

    function transitionSection(v, clip, info) {
        const current = clip.transition && clip.transition.type || 'none';
        const next = info && info.next;
        const rate = Time().rate(v.project), step = Math.max(1, Math.round(rate / 10)), longest = Math.round(rate * 3);
        const maxFrames = next ? Math.max(step, Math.min(longest, Math.min(clip.duration, next.duration) - 1)) : longest;
        const duration = clip.transition && clip.transition.duration || Math.min(Math.round(rate / 2), maxFrames);
        const disabled = next ? '' : 'disabled';
        return `<section class="vs-insp-section"><h3>${v.icon('transition', 14)}${v.esc(v.tr('transitionNext', 'Transition to next clip'))}</h3><div class="vs-transition-grid" role="group" aria-label="${v.esc(v.tr('transition', 'Transition'))}">${TRANSITIONS.map(type => `<button type="button" data-transition="${type}" aria-pressed="${current === type}" ${disabled}><i class="vs-tx-${type}"></i><span>${v.esc(v.tr(type, type))}</span></button>`).join('')}</div><label class="vs-slider"><span>${v.esc(v.tr('transitionLength', 'Length'))}</span><input type="range" data-transition-duration min="${step}" max="${maxFrames}" step="1" value="${Math.min(duration, maxFrames)}" ${next && current !== 'none' ? '' : 'disabled'}><output>${v.esc(formatSeconds(duration, v.locale, v.project))} s</output></label>${next ? '' : `<p class="vs-hint">${v.esc(v.tr('transitionNeedsNextHint', 'Place another clip right after this one on the same track to add a transition.'))}</p>`}</section>`;
    }

    function textSection(v, clip) {
        const style = clip.text_style || {};
        const font = style.font_family || 'Arial';
        const opacity = style.background_opacity == null ? 0 : Number(style.background_opacity);
        return `<section class="vs-insp-section vs-text-group"><h3>${v.icon('text', 14)}${v.esc(v.tr('text', 'Text'))}</h3><label class="vs-field vs-field-wide"><span>${v.esc(v.tr('titleText', 'Text'))}</span><textarea data-text-field="text" rows="3">${v.esc(clip.text)}</textarea></label><div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('font', 'Font'))}</span><select data-text-style="font_family">${FONTS.map(name => `<option ${name === font ? 'selected' : ''}>${v.esc(name)}</option>`).join('')}</select></label><label class="vs-field vs-field-narrow"><span>${v.esc(v.tr('fontSize', 'Size'))}</span><input type="number" min="18" max="220" data-text-style="font_size" value="${Number(style.font_size || 72)}"></label></div><div class="vs-field-row vs-text-tools"><label class="vs-color"><span>${v.esc(v.tr('color', 'Color'))}</span><input type="color" data-text-style="color" value="${v.esc(style.color || '#ffffff')}"></label><div class="vs-segmented vs-icon-segmented" role="group" aria-label="${v.esc(v.tr('alignment', 'Alignment'))}">${[['left', 'alignLeft'], ['center', 'alignCenter'], ['right', 'alignRight']].map(([value, icon]) => `<button type="button" data-text-align="${value}" aria-pressed="${(style.alignment || 'center') === value}" title="${v.esc(v.tr(icon, value))}" aria-label="${v.esc(v.tr(icon, value))}">${v.icon(icon, 15)}</button>`).join('')}</div><div class="vs-segmented vs-icon-segmented" role="group"><button type="button" data-text-toggle="bold" aria-pressed="${!!style.bold}" title="${v.esc(v.tr('bold', 'Bold'))}" aria-label="${v.esc(v.tr('bold', 'Bold'))}"><b>B</b></button><button type="button" data-text-toggle="italic" aria-pressed="${!!style.italic}" title="${v.esc(v.tr('italic', 'Italic'))}" aria-label="${v.esc(v.tr('italic', 'Italic'))}"><i>I</i></button></div></div><div class="vs-field-row vs-bg-row"><label class="vs-color"><span>${v.esc(v.tr('background', 'Background'))}</span><input type="color" data-text-style="background_color" value="${v.esc(style.background_color || '#101319')}"></label><label class="vs-slider vs-slider-compact"><span>${v.esc(v.tr('backgroundOpacity', 'Background opacity'))}</span><input type="range" min="0" max="1" step="0.05" data-text-style="background_opacity" value="${opacity}"><output>${Math.round(opacity * 100)} %</output></label></div><div class="vs-text-status" data-text-status><span>${v.esc(v.textPending ? v.tr('textUpdating', 'Updating the title…') : v.tr('textAuto', 'Changes are applied automatically.'))}</span><button type="button" data-action="apply-text" class="vs-link-button">${v.esc(v.tr('applyTextNow', 'Apply now'))}</button></div></section>`;
    }

    // Clip settings (spec 4.1, 4.3, 4.4): speed, colour labels, enable state, link and detached audio, and the derived
    // media actions. A button is disabled when the editor would refuse it (P.clipCapabilities). Stills have no speed.
    function clipSection(v, clip, asset, track) {
        const caps = v.caps || {}, speed = Number(clip.speed) > 0 ? Number(clip.speed) : 1;
        const colors = (window.VideoStudioParts && window.VideoStudioParts.LABEL_COLORS) || [];
        const swatch = color => {
            const name = color ? v.tr('color_' + color, color) : v.tr('labelNone', 'No label');
            return `<button type="button" class="vs-swatch" data-action="clip-label" data-value="${v.esc(color)}"${color ? ` style="--vs-swatch:var(--vs-label-${v.esc(color)})"` : ''} aria-pressed="${(clip.label || '') === color}" title="${v.esc(name)}" aria-label="${v.esc(name)}"></button>`;
        };
        // One clip is shown at a time; the clip id keeps the hint id unique even with two editor windows.
        const hintId = 'vs-speed-hint-' + v.esc(clip.id);
        const keep = `<label class="vs-check"><input type="checkbox" data-speed-keep><span>${v.esc(v.tr('keepDuration', 'Keep duration'))}</span></label>`;
        const silent = speed > 4 ? `<p class="vs-hint">${v.esc(v.tr('speedSilent', 'Audio is silent above 4× in preview and export.'))}</p>` : '';
        const timed = asset.kind === 'video' || asset.kind === 'audio';
        const speedRow = timed ? `<div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('speed', 'Speed'))}</span><input type="number" min="0.25" max="16" step="0.05" data-field="speed" value="${speed}" aria-describedby="${hintId}"></label>${keep}</div><p class="vs-hint" id="${hintId}">${v.esc(v.tr('speedHint', 'Playback speed from 0.25 to 16×. The clip changes length unless Keep duration is on.'))}</p>${silent}` : '';
        const derived = asset.kind === 'video' ? `<div class="vs-clip-actions"><button type="button" data-action="clip-reverse" ${caps.reversible ? '' : 'disabled'}>${v.icon('reverse', 14)}<span>${v.esc(v.tr('reverse', 'Reverse'))}</span></button><button type="button" data-action="clip-freeze" ${caps.freezable ? '' : 'disabled'}>${v.icon('freeze', 14)}<span>${v.esc(v.tr('freezeFrame', 'Freeze frame'))}</span></button><button type="button" data-action="clip-export-frame" ${caps.exportable ? '' : 'disabled'}>${v.icon('download', 14)}<span>${v.esc(v.tr('exportFrame', 'Export frame as PNG'))}</span></button></div>` : '';
        const audio = track.kind === 'video' && asset.kind === 'video' && asset.has_audio ? `<button type="button" data-action="clip-detach" ${caps.detachable ? '' : 'disabled'}>${v.icon('speaker', 14)}<span>${v.esc(v.tr('detachAudio', 'Detach audio'))}</span></button><button type="button" data-action="clip-reattach" ${caps.reattachable ? '' : 'disabled'}>${v.icon('speaker', 14)}<span>${v.esc(v.tr('reattachAudio', 'Reattach audio'))}</span></button>` : '';
        return `<section class="vs-insp-section vs-clip-section"><h3>${v.icon('sliders', 14)}${v.esc(v.tr('clipSection', 'Clip'))}</h3>${speedRow}<div class="vs-swatches" role="group" aria-label="${v.esc(v.tr('label', 'Label'))}">${[''].concat(colors).map(swatch).join('')}</div><div class="vs-clip-actions"><button type="button" data-action="clip-link" aria-pressed="${!!caps.linked}" ${caps.linkable ? '' : 'disabled'}>${v.icon(caps.linked ? 'unlink' : 'link', 14)}<span>${v.esc(caps.linked ? v.tr('unlink', 'Unlink') : v.tr('link', 'Link'))}</span></button>${audio}<button type="button" data-action="clip-disable" aria-pressed="${!!clip.disabled}">${v.icon(clip.disabled ? 'eye' : 'eyeOff', 14)}<span>${v.esc(clip.disabled ? v.tr('enable', 'Enable') : v.tr('disable', 'Disable'))}</span></button></div>${derived}</section>`;
    }

    function render(host, v) {
        if (!host) return;
        if (!v.selected) { host.innerHTML = emptyMarkup(v); return; }
        const { clip, track } = v.selected;
        const asset = v.asset || {};
        const kind = clip.text ? 'text' : asset.kind || track.kind;
        const icon = kind === 'text' ? 'text' : kind === 'audio' ? 'audio' : kind === 'image' ? 'image' : 'video';
        const title = clip.text ? clip.text.split(/\r?\n/)[0] : v.assetName || asset.name || v.tr('missingMedia', 'Missing media');
        const audioCapable = track.kind === 'audio' || asset.kind === 'video' && asset.has_audio && track.kind !== 'overlay';
        const visual = track.kind !== 'audio';
        const thumb = v.thumb ? ` style="background-image:url('${v.esc(v.thumb)}')"` : '';
        const lock = track.locked ? `<p class="vs-insp-locked">${v.icon('lock', 14)}${v.esc(v.tr('trackLocked', 'This track is locked. Unlock it in the timeline to edit.'))}</p>` : '';
        host.innerHTML = `<div class="vs-insp-head"><span class="vs-insp-thumb vs-kind-${v.esc(kind)}"${thumb}>${thumb ? '' : v.icon(icon, 18)}</span><div class="vs-insp-title"><strong title="${v.esc(title)} · ${v.esc(track.name)}">${v.esc(title)}</strong><span class="vs-insp-meta"><small title="${v.esc(track.name)}">${v.esc(formatTime(clip.start, v.project))} – ${v.esc(formatTime(clip.start + clip.duration, v.project))}</small><span class="vs-insp-actions"><button type="button" class="vs-icon-button" data-action="clip-split" title="${v.esc(v.tr('splitHintShort', 'Split at the playhead (S)'))}" aria-label="${v.esc(v.tr('split', 'Split'))}">${v.icon('scissors', 16)}</button><button type="button" class="vs-icon-button" data-action="clip-duplicate" title="${v.esc(v.tr('duplicate', 'Duplicate'))}" aria-label="${v.esc(v.tr('duplicate', 'Duplicate'))}">${v.icon('copy', 16)}</button><button type="button" class="vs-icon-button vs-danger" data-action="clip-delete" title="${v.esc(v.tr('deleteHint', 'Delete (Del)'))}" aria-label="${v.esc(v.tr('delete', 'Delete'))}">${v.icon('trash', 16)}</button></span></span></div></div>${lock}${clip.text ? textSection(v, clip) : ''}${timeSection(v, clip, asset)}${clip.text ? '' : clipSection(v, clip, asset, track)}${visual ? pictureSection(v, clip) : ''}${audioCapable ? soundSection(v, clip) : ''}${visual ? transitionSection(v, clip, v.transitionInfo) : ''}`;
    }

    window.VideoStudioInspector = { render, formatTime, parseTime, formatSeconds, presetBox, scaleBox, clampBox, findOverlaySlot, displayTracks, insertIndexFor, FONTS };
})();
