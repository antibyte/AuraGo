(function () {
    'use strict';

    const MARGIN = 0.04;
    const BOX = ['x', 'y', 'width', 'height'];
    const FONTS = ['Arial', 'Georgia', 'Trebuchet MS', 'Impact', 'Verdana', 'Courier New'];
    const round6 = value => Math.round(value * 1e6) / 1e6;
    const clamp = (value, min, max) => Math.max(min, Math.min(max, value));

    const Time = () => window.VideoStudioTime;
    const Effects = () => window.VideoStudioEffects;
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

    function slider(v, field, label, min, max, step, value, output, extra) {
        return `<label class="vs-slider"><span>${v.esc(label)}</span><input type="range" data-field="${field}" min="${min}" max="${max}" step="${step}" value="${value}"${extra || ''}><output>${v.esc(output)}</output></label>`;
    }
    // The control state of one path, or of several paths joined by spaces (a colour wheel's x and y):
    // - animated when any of them is;
    // - on a key when every animated one is;
    // - prev and next are the nearest previous and next key over the animated ones.
    function controlFor(kf, path) {
        const list = String(path).split(' ').map(item => kf && kf.controls[item]).filter(Boolean);
        if (!list.length) return null;
        if (list.length === 1) return list[0];
        const animated = list.filter(c => c.animated), pick = (values, fn) => { const known = values.filter(value => value != null); return known.length ? fn(...known) : null; };
        return { animated: animated.length > 0, onKey: animated.length > 0 && animated.every(c => c.onKey), prev: pick(animated.map(c => c.prev), Math.max), next: pick(animated.map(c => c.next), Math.min), value: null };
    }
    // Stopwatch, diamond and previous/next keyframe buttons of one keyframable value (spec §8.2); path may list several
    // paths joined by spaces (a colour wheel's x and y; controlFor). v.keyframes is
    // P.keyframeView(s) (video-studio-keyframes.js), which also keeps these buttons in sync while the playhead moves; they
    // are disabled while the playhead is outside the clip. Without v.keyframes nothing is drawn.
    function keyframeTools(v, path, name) {
        const kf = v.keyframes, c = kf && controlFor(kf, path);
        if (!c) return '';
        const text = (key, fallback) => v.esc(v.tr(key, fallback).split('{{name}}').join(name));
        const off = kf.locked || !kf.inside;
        const button = (action, cls, iconName, label, extra, disabled) => `<button type="button" class="vs-kf-button${cls}" data-kf-path="${v.esc(path)}" data-kf-action="${action}" aria-label="${label}" title="${label}"${extra}${disabled ? ' disabled' : ''}>${v.icon(iconName, 14)}</button>`;
        const stopwatch = c.animated ? text('keyframeStopwatchOff', 'Stop animating {{name}} (keeps the value at the playhead)') : text('keyframeStopwatch', 'Animate {{name}}');
        const key = c.onKey ? text('keyframeRemove', 'Remove the keyframe of {{name}} at the playhead') : text('keyframeAdd', 'Add a keyframe for {{name}} at the playhead');
        const outside = kf.inside ? '' : ` title="${v.esc(v.tr('keyframeOutside', 'Move the playhead into the clip to set keyframes.'))}"`;
        return `<span class="vs-kf-tools" role="group" aria-label="${v.esc(name)}"${outside}>${button('stopwatch', ' vs-kf-stopwatch', 'stopwatch', stopwatch, ` aria-pressed="${c.animated}"`, off)}${button('key', '', c.onKey ? 'diamondFilled' : 'diamond', key, ` aria-pressed="${c.onKey}" data-kf-on="${c.onKey}"`, off)}${button('prev', '', 'chevronLeft', text('keyframePrev', 'Previous keyframe of {{name}}'), '', off || c.prev == null)}${button('next', '', 'chevronRight', text('keyframeNext', 'Next keyframe of {{name}}'), '', off || c.next == null)}</span>`;
    }

    // Crop in the preview (2c): toggles the stage's crop overlay (P.setCropMode). It needs the playhead in the clip.
    function cropModeButton(v) {
        const kf = v.keyframes, outside = !!kf && !kf.inside;
        const name = v.tr('cropMode', 'Crop in the preview');
        const title = outside ? v.tr('cropModeOutside', 'Move the playhead into the selected clip to crop in the preview.') : name;
        return `<button type="button" class="vs-chip-button vs-crop-mode" data-action="crop-mode" aria-pressed="${!!v.cropMode}" title="${v.esc(title)}"${outside ? ' disabled' : ''}>${v.icon('crop', 14)}${v.esc(name)}</button>`;
    }

    // Crop and flip (spec §5.3, §8.1). The crop fields are in percent of the source picture, with keyframe tools. They show
    // the values at the playhead (v.keyframes.state) and freeze like the box while the playhead is outside and a side is
    // animated. The two flip toggles follow. The edits run in video-studio-inspector-effects.js; the crop-mode button
    // (cropModeButton) sits next to Full frame.
    const CROP = [['l', 'crop_l'], ['t', 'crop_t'], ['r', 'crop_r'], ['b', 'crop_b']];
    function cropMarkup(v, clip, at, lock) {
        const crop = (at && at.crop) || clip.crop || {};
        const fields = CROP.map(([side, key]) => { const name = v.tr(key, side); return `<div class="vs-kf-field"><label class="vs-field"><span>${v.esc(name)}</span><input type="number" data-crop="${side}" min="0" max="95" step="0.1" value="${round6((Number(crop[side]) || 0) * 100)}"${lock(['crop.' + side])}></label>${keyframeTools(v, 'crop.' + side, name)}</div>`; }).join('');
        const flip = (axis, key, iconName) => `<button type="button" class="vs-chip-button vs-fx-flip" data-flip="${axis}" aria-pressed="${!!clip['flip_' + axis]}">${v.icon(iconName, 14)}<span>${v.esc(v.tr(key, key))}</span></button>`;
        return `<div class="vs-fx-picture" role="group" aria-label="${v.esc(v.tr('crop', 'Crop'))}"><div class="vs-fx-subhead">${v.icon('crop', 14)}<span>${v.esc(v.tr('crop', 'Crop'))}</span></div><div class="vs-field-grid vs-fx-crop">${fields}</div><div class="vs-fx-flips" role="group" aria-label="${v.esc(v.tr('flip', 'Flip'))}">${flip('h', 'flip_h', 'flipH')}${flip('v', 'flip_v', 'flipV')}</div></div>`;
    }

    // Blend mode and mask (2d; spec §5.3, §8.1; plan index §4.22). Titles and stickers take both. The edits run in
    // video-studio-inspector-effects.js.
    // - The blend list follows the registry (VideoStudioEffects.blendModes). A mode the server's FFmpeg cannot export is
    //   disabled with a title, except the clip's own mode, which stays selectable so it still shows.
    // - blendJoins: during a transition between clips of different modes, the overlap composites Normal, and a hint says
    //   so.
    // - The mask block offers None, Rectangle or Ellipse. With a mask it also shows the six values with keyframe tools,
    //   Invert, and Edit mask in the preview (P.setMaskMode, video-studio-mask.js), which needs the playhead inside the
    //   clip. MASK_FIELDS: [path suffix, label key, field min, field max, factor from the stored value to the field].
    const MASK_FIELDS = Object.freeze([['x', 'mask_x', -100, 100, 100], ['y', 'mask_y', -100, 100, 100], ['w', 'mask_w', 1, 200, 100], ['h', 'mask_h', 1, 200, 100], ['rotation', 'mask_rotation', -360, 360, 1], ['feather', 'mask_feather', 0, 50, 100]]);
    const MASK_SHAPES = [['none', 'mask_none', 'None'], ['rect', 'mask_rect', 'Rectangle'], ['ellipse', 'mask_ellipse', 'Ellipse']];
    const blendOf = clip => (clip && clip.blend) || 'normal';
    function blendJoins(track, clip) {
        const ordered = ((track && track.clips) || []).slice().sort((a, b) => a.start - b.start), index = ordered.findIndex(item => item.id === clip.id);
        const joined = (out, into) => !!out && !!into && !!out.transition && !!out.transition.type && out.transition.type !== 'none' && blendOf(out) !== blendOf(into);
        return index >= 0 && (joined(ordered[index], ordered[index + 1]) || joined(ordered[index - 1], ordered[index]));
    }
    function blendMarkup(v, clip, track) {
        const current = blendOf(clip), unavailable = v.tr('blendUnavailable', "This server's FFmpeg cannot export this blend mode.");
        const options = Effects().blendModes().map(mode => `<option value="${v.esc(mode.id)}"${mode.id === current ? ' selected' : ''}${!mode.available && mode.id !== current ? ' disabled' : ''}${mode.available ? '' : ` title="${v.esc(unavailable)}"`}>${v.esc(v.tr(mode.label_key, mode.id))}</option>`).join('');
        const hint = blendJoins(track, clip) ? `<p class="vs-hint vs-blend-hint" data-blend-hint>${v.icon('info', 14)}<span>${v.esc(v.tr('blendTransitionHint', 'During a transition between clips with different blend modes, the overlap uses Normal.'))}</span></p>` : '';
        return `<div class="vs-fx-picture vs-blend"><label class="vs-blend-field"><span class="vs-fx-subhead">${v.icon('blend', 14)}<span>${v.esc(v.tr('blend', 'Blend mode'))}</span></span><select data-blend>${options}</select></label>${hint}</div>`;
    }
    function maskModeButton(v, clip) {
        const kf = v.keyframes, outside = !!kf && !kf.inside, name = v.tr('maskMode', 'Edit mask in the preview');
        const title = outside && clip.mask ? v.tr('maskModeOutside', 'Move the playhead into the selected clip to edit its mask in the preview.') : name;
        return `<button type="button" class="vs-chip-button vs-mask-mode" data-action="mask-mode" aria-pressed="${!!v.maskMode && !!clip.mask}" title="${v.esc(title)}"${!clip.mask || outside ? ' disabled' : ''}>${v.icon('mask', 14)}<span>${v.esc(name)}</span></button>`;
    }
    function maskMarkup(v, clip, at, lock) {
        const stored = clip.mask || null, mask = stored ? (at && at.mask) || stored : null, shape = stored ? stored.shape : 'none';
        const shapes = MASK_SHAPES.map(([id, key, fallback]) => `<button type="button" data-mask-set="${id}" aria-pressed="${shape === id}">${v.esc(v.tr(key, fallback))}</button>`).join('');
        const fields = mask ? MASK_FIELDS.map(([field, key, min, max, factor]) => { const name = v.tr(key, field), path = 'mask.' + field; return `<div class="vs-kf-field"><label class="vs-field"><span>${v.esc(name)}</span><input type="number" data-mask-field="${field}" min="${min}" max="${max}" step="0.1" value="${round6((Number(mask[field]) || 0) * factor)}"${lock([path])}></label>${keyframeTools(v, path, name)}</div>`; }).join('') : '';
        const values = fields ? `<div class="vs-field-grid vs-mask-fields">${fields}</div><p class="vs-hint">${v.esc(v.tr('maskFieldsHint', 'Position and size in percent of the clip box, rotation in degrees, feather in percent of the box height.'))}</p>` : '';
        const invert = stored ? `<button type="button" class="vs-chip-button vs-fx-flip" data-mask-invert aria-pressed="${!!stored.invert}">${v.esc(v.tr('mask_invert', 'Invert mask'))}</button>` : '';
        return `<div class="vs-fx-picture vs-mask" role="group" aria-label="${v.esc(v.tr('mask', 'Mask'))}"><div class="vs-fx-subhead">${v.icon('mask', 14)}<span>${v.esc(v.tr('mask', 'Mask'))}</span></div><div class="vs-segmented vs-mask-set" role="group" aria-label="${v.esc(v.tr('mask_shape', 'Mask shape'))}">${shapes}</div>${values}<div class="vs-mask-actions">${invert}${maskModeButton(v, clip)}</div></div>`;
    }

    // Marker colour dots use the timeline palette only; anything else is not drawn.
    function palette(color) { return !!color && window.VideoStudioTimeline.LABEL_COLORS.includes(color); }
    // The project view's marker list: jump to a marker, edit it, and the in and out points. Set in / Set out put a point at
    // the playhead for a mouse user (the transport hides its In and Out buttons in a narrow window); the app's data-action
    // handler runs them and a read-only editor disables them with the rest of the inspector.
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
        const setIn = v.tr('setIn', 'Set in'), setOut = v.tr('setOut', 'Set out');
        const setPoints = `<div class="vs-inout-actions"><button type="button" class="vs-chip-button" data-action="mark-in" title="${v.esc(setIn)} (I)"><span>${v.esc(setIn)}</span></button><button type="button" class="vs-chip-button" data-action="mark-out" title="${v.esc(setOut)} (O)"><span>${v.esc(setOut)}</span></button></div>`;
        return `<section class="vs-insp-section"><h3>${v.icon('flag', 14)}${v.esc(v.tr('markersList', 'Markers'))}</h3>${range}${setPoints}${rows ? `<ul class="vs-marker-list">${rows}</ul>` : `<p class="vs-hint">${v.esc(v.tr('noMarkers', 'No markers yet. Press M at the playhead to add one.'))}</p>`}</section>`;
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
        // Values at the playhead: v.keyframes (P.keyframeView) carries the clip's state with keyframes evaluated; without it
        // the clip's own fields show.
        const kf = v.keyframes, at = kf ? kf.state : clip;
        const num = (path, fallback) => { const value = Number(at[path]); return at[path] == null || !Number.isFinite(value) ? fallback : value; };
        // An animated value cannot change while the playhead is outside the clip: no keyframe can be set there.
        const frozen = paths => !!kf && !kf.inside && paths.some(path => kf.controls[path] && kf.controls[path].animated);
        const lock = paths => (frozen(paths) ? ` disabled title="${v.esc(v.tr('keyframeOutside', 'Move the playhead into the clip to set keyframes.'))}"` : '');
        const label = field => v.tr(field === 'x' || field === 'y' ? 'box_' + field : field, field.toUpperCase());
        const size = Math.round(num('width', 1) * 100), rotation = Math.round(num('rotation', 0)), opacity = Math.round(num('opacity', 1) * 100);
        const fine = BOX.map(field => `<div class="vs-kf-field"><label class="vs-field"><span>${v.esc(label(field))}</span><input type="number" data-field="${field}" min="${field === 'width' || field === 'height' ? 1 : 0}" max="100" step="0.5" value="${round6(num(field, field === 'width' || field === 'height' ? 1 : 0) * 100)}"${lock([field])}></label>${keyframeTools(v, field, label(field))}</div>`).join('');
        const boxLock = lock(BOX);
        return `<section class="vs-insp-section"><h3>${v.icon('image', 14)}${v.esc(v.tr('picture', 'Picture'))}</h3><div class="vs-position"><div class="vs-position-grid" role="group" aria-label="${v.esc(v.tr('position', 'Position'))}">${anchors.map(anchor => `<button type="button" data-position="${anchor}" title="${v.esc(v.tr('pos_' + anchor, anchor))}" aria-label="${v.esc(v.tr('pos_' + anchor, anchor))}"${frozen(BOX) ? ' disabled' : ''}><i></i></button>`).join('')}</div>${isText ? '' : `<button type="button" class="vs-chip-button" data-action="fill-frame"${boxLock}>${v.icon('fullscreen', 14)}${v.esc(v.tr('fillFrame', 'Full frame'))}</button>${cropModeButton(v)}`}</div>${isText ? '' : slider(v, 'size', v.tr('size', 'Size'), 5, 100, 1, size, size + ' %', boxLock)}<div class="vs-kf-line">${slider(v, 'rotation', v.tr('rotation', 'Rotation'), -180, 180, 1, rotation, rotation + '°', lock(['rotation']))}${keyframeTools(v, 'rotation', v.tr('rotation', 'Rotation'))}</div><div class="vs-kf-line">${slider(v, 'opacity', v.tr('opacity', 'Opacity'), 0, 100, 1, opacity, opacity + ' %', lock(['opacity']))}${keyframeTools(v, 'opacity', v.tr('opacity', 'Opacity'))}</div>${isText ? '' : `<div class="vs-segmented" role="group" aria-label="${v.esc(v.tr('fit', 'Fit'))}"><button type="button" data-fit="contain" aria-pressed="${clip.fit !== 'cover'}">${v.esc(v.tr('contain', 'Fit inside'))}</button><button type="button" data-fit="cover" aria-pressed="${clip.fit === 'cover'}">${v.esc(v.tr('cover', 'Fill'))}</button></div>`}${isText ? '' : cropMarkup(v, clip, at, lock)}${blendMarkup(v, clip, v.selected.track)}${maskMarkup(v, clip, at, lock)}<div class="vs-fine"><button type="button" class="vs-link-button vs-fine-toggle" data-action="toggle-fine" aria-expanded="${!!v.fineOpen}">${v.icon('chevronDown', 14)}<span>${v.esc(v.tr('fineTune', 'Fine tune (percent of the frame)'))}</span></button><div class="vs-field-grid" ${v.fineOpen ? '' : 'hidden'}>${fine}</div></div></section>`;
    }

    // In and out animations (2e; spec §5.6, §8.1; plan index §4.24) for every visual clip, titles and stickers included.
    // ANIM_SIDES rows are [side, clip field, label key, fallback]. Each side has:
    //   - a preset list: None plus the registry presets (VideoStudioEffects.animPresets). A preset the server's FFmpeg
    //     cannot export is disabled with a title, except the clip's own, which stays selectable so it still shows.
    //   - a length slider in timeline frames over VideoStudioAnim.frameRange (0.2 to 3 s), with its output in seconds.
    //     It is disabled while the side has no animation.
    //   - [data-anim-clamped]: while the stored length is longer than half the clip, the render plays it over half the
    //     clip (VideoStudioAnim.spans), and this hint says so (animClampText; U+00A0 keeps the number with its unit).
    // The edits run in video-studio-inspector-effects.js (P.animChange, P.liveAnimDuration). animationRows returns ''
    // without VideoStudioAnim. The app loads it before the inspector (§5); the guard keeps Node checks working that
    // load the inspector alone.
    const ANIM_SIDES = Object.freeze([['in', 'anim_in', 'animIn', 'In animation'], ['out', 'anim_out', 'animOut', 'Out animation']]);
    function animClampText(v, clip, side) {
        const anim = clip && clip['anim_' + side], played = anim ? window.VideoStudioAnim.spans(clip)[side] : 0;
        if (!anim || !(Number(anim.duration) > played)) return '';
        return v.tr('animClamped', 'This clip is short: the animation plays over {{seconds}}\xa0s, half the clip.').split('{{seconds}}').join(formatSeconds(played, v.locale, v.project));
    }
    function animationRows(v, clip) {
        const A = window.VideoStudioAnim;
        if (!A) return '';
        const range = A.frameRange(v.project), unavailable = v.tr('animUnavailable', "This server's FFmpeg cannot export this animation."), lengthName = v.tr('animLength', 'Length');
        return `<div class="vs-anim">${ANIM_SIDES.map(([side, key, labelKey, fallback]) => {
            const anim = clip[key] || null, current = anim ? anim.preset : 'none', name = v.tr(labelKey, fallback), clamped = animClampText(v, clip, side);
            const option = (id, label, available) => `<option value="${v.esc(id)}"${id === current ? ' selected' : ''}${!available && id !== current ? ' disabled' : ''}${available ? '' : ` title="${v.esc(unavailable)}"`}>${v.esc(label)}</option>`;
            const options = option('none', v.tr('none', 'None'), true) + Effects().animPresets().map(preset => option(preset.id, v.tr(preset.label_key, preset.id), preset.available)).join('');
            const frames = anim ? clamp(Math.round(Number(anim.duration)) || range.default, range.min, range.max) : range.default;
            return `<div class="vs-anim-side" data-anim-side="${side}"><label class="vs-field vs-anim-preset"><span class="vs-fx-subhead">${v.esc(name)}</span><select data-anim-preset="${side}">${options}</select></label><label class="vs-slider vs-anim-length"><span>${v.esc(lengthName)}</span><input type="range" data-anim-duration="${side}" min="${range.min}" max="${range.max}" step="1" value="${frames}" aria-label="${v.esc(name + ': ' + lengthName)}"${anim ? '' : ' disabled'}><output>${v.esc(formatSeconds(frames, v.locale, v.project))} s</output></label><p class="vs-hint vs-anim-clamped" data-anim-clamped="${side}"${clamped ? '' : ' hidden'}>${v.icon('info', 14)}<span>${v.esc(clamped)}</span></p></div>`;
        }).join('')}</div>`;
    }

    // Animation section presets (spec §8.1): [preset id, label key, glyph path on a 30×18 box, fast at the top].
    const RAMP_PRESET_BUTTONS = [['none', 'ramp_none', 'M2 9h26'], ['ramp_up', 'ramp_up', 'M2 15C12 15 18 3 28 3'], ['ramp_down', 'ramp_down', 'M2 3C12 3 18 15 28 15'], ['slow_middle', 'ramp_slow_middle', 'M2 4C8 4 9 14 13 14h4c4 0 5-10 11-10'], ['custom', 'ramp_custom', 'M2 12l8-6 8 7 10-8']];
    // Linked sound of a ramped video-track clip is muted in preview and export (index §4.18); detached sound is not.
    function rampNotice(v) {
        return `<p class="vs-hint vs-ramp-notice" data-ramp-notice>${v.icon('speakerOff', 14)}<span>${v.esc(v.tr('rampMuted', 'The clip\'s sound is muted while it has a speed ramp. Use Detach audio to hear it at normal speed.'))}</span></p>`;
    }
    // Animation (spec §8.1; index §4.18, §4.24): the in and out animations of every visual clip (2e, animationRows),
    // then the speed ramp presets for clips on video tracks with a video asset. Keep duration keeps the clip length, and
    // Edit in the timeline opens the speed lane. Audio clips have no Animation section.
    function animationSection(v, clip, asset, track, muted) {
        const rows = track.kind === 'audio' ? '' : animationRows(v, clip), rampable = track.kind === 'video' && asset.kind === 'video' && !clip.text;
        if (track.kind === 'audio' || (!rows && !rampable)) return '';
        const head = `<h3>${v.icon('sparkle', 14)}${v.esc(v.tr('animation', 'Animation'))}</h3>${rows}`;
        if (!rampable) return `<section class="vs-insp-section vs-anim-section">${head}</section>`;
        const ramped = Effects().hasRamp(clip);
        const presets = RAMP_PRESET_BUTTONS.map(([id, key, glyph]) => { const name = v.tr(key, id); return `<button type="button" data-ramp-preset="${id}" title="${v.esc(name)}"><svg class="vs-ramp-glyph" viewBox="0 0 30 18" aria-hidden="true" focusable="false"><path d="${glyph}"/></svg><span>${v.esc(name)}</span></button>`; }).join('');
        return `<section class="vs-insp-section vs-anim-section vs-ramp-section">${head}<div class="vs-fx-subhead vs-ramp-head">${v.icon('ramp', 14)}<span>${v.esc(v.tr('speedRamp', 'Speed ramp'))}</span></div><div class="vs-ramp-presets" role="group" aria-label="${v.esc(v.tr('speedRamp', 'Speed ramp'))}">${presets}</div><div class="vs-field-row vs-ramp-actions"><label class="vs-check"><input type="checkbox" data-ramp-keep><span>${v.esc(v.tr('keepDuration', 'Keep duration'))}</span></label><button type="button" class="vs-chip-button" data-action="ramp-edit" ${ramped ? '' : 'disabled'}>${v.icon('ramp', 14)}<span>${v.esc(v.tr('rampEdit', 'Edit in the timeline'))}</span></button></div>${muted ? rampNotice(v) : ''}</section>`;
    }

    function soundSection(v, clip, muted) {
        const volume = Math.round(Number(clip.volume == null ? 1 : clip.volume) * 100);
        const rate = Time().rate(v.project), step = Math.max(1, Math.round(rate / 10));
        const maxFade = Math.max(0, Math.min(clip.duration, Math.round(rate * 5)));
        return `<section class="vs-insp-section"><h3>${v.icon('speaker', 14)}${v.esc(v.tr('sound', 'Sound'))}</h3>${slider(v, 'volume', v.tr('volume', 'Volume'), 0, 200, 1, volume, volume + ' %')}${slider(v, 'fade_in', v.tr('fadeInSound', 'Fade in'), 0, maxFade, step, Math.min(maxFade, clip.fade_in || 0), formatSeconds(clip.fade_in || 0, v.locale, v.project) + ' s')}${slider(v, 'fade_out', v.tr('fadeOutSound', 'Fade out'), 0, maxFade, step, Math.min(maxFade, clip.fade_out || 0), formatSeconds(clip.fade_out || 0, v.locale, v.project) + ' s')}${muted ? rampNotice(v) : ''}</section>`;
    }

    // Transition length (index §4.9): the slider's step, its largest value for the pair, and the length a new transition
    // gets (half a second within that largest value; an existing transition keeps its own). The effects library uses
    // the same numbers (P.addTransitionTo, video-studio-effects-library.js).
    function transitionLength(project, clip, next) {
        const rate = Time().rate(project), step = Math.max(1, Math.round(rate / 10)), longest = Math.round(rate * 3);
        const max = next ? Math.max(step, Math.min(longest, Math.min(clip.duration, next.duration) - 1)) : longest;
        return { step, max, value: clip.transition && clip.transition.duration || Math.min(Math.round(rate / 2), max) };
    }

    // The transition picker (index §4.9):
    // - A summary button shows the current type's icon and label. It opens an inline panel (v.transitionPickerOpen, view
    //   state of video-studio.js) with 'none' and one group per registry category (VideoStudioEffects.transitionGroups,
    //   read at call time).
    // - A type the server's FFmpeg lacks is disabled with a title. The clip's current one stays enabled and pressed, so
    //   it still shows and can be replaced.
    // - Labels come from label_key, icons from VideoStudioIcons.transition. Without a following clip the summary is
    //   disabled.
    const NO_TRANSITION = Object.freeze({ id: 'none', label_key: 'none', available: true });
    function transitionIcon(id) { const Icons = window.VideoStudioIcons; return Icons && Icons.transition ? Icons.transition(id, 30) : ''; }
    function transitionChoice(v, spec, current) {
        const off = !spec.available && spec.id !== current ? ' disabled' : '';
        const title = spec.available ? '' : ` title="${v.esc(v.tr('transitionUnavailable', "Not available: this server's FFmpeg lacks this transition"))}"`;
        return `<button type="button" data-transition="${v.esc(spec.id)}" aria-pressed="${current === spec.id}"${title}${off}>${transitionIcon(spec.id)}<span>${v.esc(v.tr(spec.label_key, spec.id))}</span></button>`;
    }
    function transitionPicker(v, current, disabled) {
        const spec = current === 'none' ? NO_TRANSITION : Effects().transition(current) || { id: current, label_key: current };
        const name = v.tr('transitionPicker', 'Choose a transition'), label = v.tr(spec.label_key, spec.id), open = !!v.transitionPickerOpen && !disabled;
        const summary = `<button type="button" class="vs-tx-summary" data-action="transition-picker" aria-expanded="${open}" aria-label="${v.esc(name)}: ${v.esc(label)}" title="${v.esc(name)}"${disabled ? ' disabled' : ''}>${transitionIcon(spec.id)}<span>${v.esc(label)}</span>${v.icon('chevronDown', 14)}</button>`;
        if (!open) return summary;
        const groups = Effects().transitionGroups().map(group => { const heading = v.tr(group.label_key, group.category); return `<div class="vs-tx-group" role="group" aria-label="${v.esc(heading)}"><h4>${v.esc(heading)}</h4><div class="vs-tx-list">${group.items.map(item => transitionChoice(v, item, current)).join('')}</div></div>`; }).join('');
        return `${summary}<div class="vs-tx-picker" data-tx-picker><div class="vs-tx-list">${transitionChoice(v, NO_TRANSITION, current)}</div>${groups}</div>`;
    }

    // Arrow keys in the open transition picker (spec §8.6; plan index §4.26.9), DOM-free:
    // - lists are the picker's lists in DOM order, [{count, columns, disabled: [bool]}], with columns measured from the
    //   layout; current is {list, index};
    // - ArrowLeft/ArrowRight: the previous/next enabled type in DOM order;
    // - ArrowUp/ArrowDown: the same column in the previous/next row. Rows run on across the lists, and the column is
    //   clamped to the target row's length. A disabled target moves on to the next row in the key's direction;
    // - Home/End: the first/last enabled type.
    // Returns {list, index}. It returns null at the ends (no wrap), for other keys and for an unknown position.
    function pickerMove(lists, current, key) {
        const cells = [], rows = [], off = [];
        (Array.isArray(lists) ? lists : []).forEach((item, list) => {
            const count = Math.max(0, Math.floor(Number(item && item.count) || 0)), columns = Math.max(1, Math.floor(Number(item && item.columns) || 1));
            off[list] = item && Array.isArray(item.disabled) ? item.disabled : [];
            for (let index = 0; index < count; index++) cells.push({ list, index, disabled: !!off[list][index] });
            for (let start = 0; start < count; start += columns) rows.push({ list, start, length: Math.min(columns, count - start) });
        });
        const pick = cell => (cell ? { list: cell.list, index: cell.index } : null);
        if (key === 'Home') return pick(cells.find(cell => !cell.disabled));
        if (key === 'End') return pick(cells.slice().reverse().find(cell => !cell.disabled));
        const at = current ? cells.findIndex(cell => cell.list === current.list && cell.index === current.index) : -1;
        if (at < 0) return null;
        if (key === 'ArrowLeft' || key === 'ArrowRight') {
            const dir = key === 'ArrowRight' ? 1 : -1;
            for (let i = at + dir; i >= 0 && i < cells.length; i += dir) if (!cells[i].disabled) return pick(cells[i]);
            return null;
        }
        if (key !== 'ArrowUp' && key !== 'ArrowDown') return null;
        const here = cells[at], step = key === 'ArrowDown' ? 1 : -1;
        const row = rows.findIndex(r => r.list === here.list && here.index >= r.start && here.index < r.start + r.length), column = here.index - rows[row].start;
        for (let k = row + step; k >= 0 && k < rows.length; k += step) {
            const index = rows[k].start + Math.min(column, rows[k].length - 1);
            if (!off[rows[k].list][index]) return { list: rows[k].list, index };
        }
        return null;
    }

    function transitionSection(v, clip, info) {
        const current = clip.transition && clip.transition.type || 'none';
        const next = info && info.next, length = transitionLength(v.project, clip, next);
        return `<section class="vs-insp-section vs-tx-section"><h3>${v.icon('transition', 14)}${v.esc(v.tr('transitionNext', 'Transition to next clip'))}</h3><div class="vs-tx" role="group" aria-label="${v.esc(v.tr('transition', 'Transition'))}">${transitionPicker(v, current, !next)}</div><label class="vs-slider"><span>${v.esc(v.tr('transitionLength', 'Length'))}</span><input type="range" data-transition-duration min="${length.step}" max="${length.max}" step="1" value="${Math.min(length.value, length.max)}" ${next && current !== 'none' ? '' : 'disabled'}><output>${v.esc(formatSeconds(length.value, v.locale, v.project))} s</output></label>${next ? '' : `<p class="vs-hint">${v.esc(v.tr('transitionNeedsNextHint', 'Place another clip right after this one on the same track to add a transition.'))}</p>`}</section>`;
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
        // While a ramp drives the clip, the field is empty with the placeholder "Ramp"; a typed number replaces the ramp
        // through setSpeed (one undo step).
        const ramped = Effects().hasRamp(clip);
        const speedRow = timed ? `<div class="vs-field-row"><label class="vs-field"><span>${v.esc(v.tr('speed', 'Speed'))}</span><input type="number" min="0.25" max="16" step="0.05" data-field="speed" value="${ramped ? '' : speed}"${ramped ? ` placeholder="${v.esc(v.tr('speedRampActive', 'Ramp'))}"` : ''} aria-describedby="${hintId}"></label>${keep}</div><p class="vs-hint" id="${hintId}">${v.esc(v.tr('speedHint', 'Playback speed from 0.25 to 16×. The clip changes length unless Keep duration is on.'))}</p>${silent}` : '';
        const derived = asset.kind === 'video' ? `<div class="vs-clip-actions"><button type="button" data-action="clip-reverse" ${caps.reversible ? '' : 'disabled'}>${v.icon('reverse', 14)}<span>${v.esc(v.tr('reverse', 'Reverse'))}</span></button><button type="button" data-action="clip-freeze" ${caps.freezable ? '' : 'disabled'}>${v.icon('freeze', 14)}<span>${v.esc(v.tr('freezeFrame', 'Freeze frame'))}</span></button><button type="button" data-action="clip-export-frame" ${caps.exportable ? '' : 'disabled'}>${v.icon('download', 14)}<span>${v.esc(v.tr('exportFrame', 'Export frame as PNG'))}</span></button></div>` : '';
        const audio = track.kind === 'video' && asset.kind === 'video' && asset.has_audio ? `<button type="button" data-action="clip-detach" ${caps.detachable ? '' : 'disabled'}>${v.icon('speaker', 14)}<span>${v.esc(v.tr('detachAudio', 'Detach audio'))}</span></button><button type="button" data-action="clip-reattach" ${caps.reattachable ? '' : 'disabled'}>${v.icon('speaker', 14)}<span>${v.esc(v.tr('reattachAudio', 'Reattach audio'))}</span></button>` : '';
        return `<section class="vs-insp-section vs-clip-section"><h3>${v.icon('sliders', 14)}${v.esc(v.tr('clipSection', 'Clip'))}</h3>${speedRow}<div class="vs-swatches" role="group" aria-label="${v.esc(v.tr('label', 'Label'))}">${[''].concat(colors).map(swatch).join('')}</div><div class="vs-clip-actions"><button type="button" data-action="clip-link" aria-pressed="${!!caps.linked}" ${caps.linkable ? '' : 'disabled'}>${v.icon(caps.linked ? 'unlink' : 'link', 14)}<span>${v.esc(caps.linked ? v.tr('unlink', 'Unlink') : v.tr('link', 'Link'))}</span></button>${audio}<button type="button" data-action="clip-disable" aria-pressed="${!!clip.disabled}">${v.icon(clip.disabled ? 'eye' : 'eyeOff', 14)}<span>${v.esc(clip.disabled ? v.tr('enable', 'Enable') : v.tr('disable', 'Disable'))}</span></button></div>${derived}</section>`;
    }

    // Metadata of the selected library item (rows are prepared by the app part; this module only lays them out).
    function assetMarkup(v) {
        const info = v.libraryAsset;
        const thumb = info.thumb ? ` style="background-image:url('${v.esc(info.thumb)}')"` : '';
        const rows = info.rows.map(([label, value]) => `<div><dt>${v.esc(label)}</dt><dd title="${v.esc(value)}">${v.esc(value)}</dd></div>`).join('');
        const folder = info.folderPath || v.tr('rootFolder', 'Media (top level)');
        return `<div class="vs-insp-head"><span class="vs-insp-thumb vs-kind-${v.esc(info.kind)}"${thumb}>${info.thumb ? '' : v.icon(info.icon, 18)}</span><div class="vs-insp-title"><strong title="${v.esc(info.name)}">${v.esc(info.name)}</strong><span class="vs-insp-meta"><small title="${v.esc(folder)}">${v.icon('folder', 12)}${v.esc(folder)}</small></span></div></div><section class="vs-insp-section"><h3>${v.icon('info', 14)}${v.esc(v.tr('inspectorMedia', 'Media'))}</h3><dl class="vs-facts">${rows}</dl></section><p class="vs-hint">${v.esc(v.tr('libraryInspectorHint', 'Double-click to open it in the source monitor, or drag it onto the timeline.'))}</p>`;
    }

    function render(host, v) {
        if (!host) return;
        if (!v.selected) { host.innerHTML = v.libraryAsset ? assetMarkup(v) : emptyMarkup(v); return; }
        const { clip, track } = v.selected;
        const asset = v.asset || {};
        const kind = clip.text ? 'text' : asset.kind || track.kind;
        const icon = kind === 'text' ? 'text' : kind === 'audio' ? 'audio' : kind === 'image' ? 'image' : 'video';
        const title = clip.text ? clip.text.split(/\r?\n/)[0] : v.assetName || asset.name || v.tr('missingMedia', 'Missing media');
        const audioCapable = track.kind === 'audio' || asset.kind === 'video' && asset.has_audio && track.kind !== 'overlay';
        const visual = track.kind !== 'audio';
        const rampMuted = Effects().hasRamp(clip) && track.kind === 'video' && asset.kind === 'video' && !!asset.has_audio && !clip.audio_detached;
        const thumb = v.thumb ? ` style="background-image:url('${v.esc(v.thumb)}')"` : '';
        const lock = track.locked ? `<p class="vs-insp-locked">${v.icon('lock', 14)}${v.esc(v.tr('trackLocked', 'This track is locked. Unlock it in the timeline to edit.'))}</p>` : '';
        host.innerHTML = `<div class="vs-insp-head"><span class="vs-insp-thumb vs-kind-${v.esc(kind)}"${thumb}>${thumb ? '' : v.icon(icon, 18)}</span><div class="vs-insp-title"><strong title="${v.esc(title)} · ${v.esc(track.name)}">${v.esc(title)}</strong><span class="vs-insp-meta"><small title="${v.esc(track.name)}">${v.esc(formatTime(clip.start, v.project))} – ${v.esc(formatTime(clip.start + clip.duration, v.project))}</small><span class="vs-insp-actions"><button type="button" class="vs-icon-button" data-action="clip-split" title="${v.esc(v.tr('splitHintShort', 'Split at the playhead (S)'))}" aria-label="${v.esc(v.tr('split', 'Split'))}">${v.icon('scissors', 16)}</button><button type="button" class="vs-icon-button" data-action="clip-duplicate" title="${v.esc(v.tr('duplicate', 'Duplicate'))}" aria-label="${v.esc(v.tr('duplicate', 'Duplicate'))}">${v.icon('copy', 16)}</button><button type="button" class="vs-icon-button vs-danger" data-action="clip-delete" title="${v.esc(v.tr('deleteHint', 'Delete (Del)'))}" aria-label="${v.esc(v.tr('delete', 'Delete'))}">${v.icon('trash', 16)}</button></span></span></div></div>${lock}${clip.text ? textSection(v, clip) : ''}${timeSection(v, clip, asset)}${clip.text ? '' : clipSection(v, clip, asset, track)}${visual ? pictureSection(v, clip) : ''}${visual && window.VideoStudioInspectorEffects ? window.VideoStudioInspectorEffects.section(v, clip, track) : ''}${animationSection(v, clip, asset, track, rampMuted)}${audioCapable ? soundSection(v, clip, rampMuted) : ''}${visual ? transitionSection(v, clip, v.transitionInfo) : ''}`;
    }

    window.VideoStudioInspector = { render, formatTime, parseTime, formatSeconds, presetBox, scaleBox, clampBox, findOverlaySlot, displayTracks, insertIndexFor, FONTS, keyframeTools, controlFor, MASK_FIELDS, blendJoins, animClampText, transitionLength, pickerMove };
})();
