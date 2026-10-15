(function () {
    'use strict';

    const MIN_ZOOM = 0.005;
    const MAX_ZOOM = 2.8;
    const BASE_PX_PER_SECOND = 52;
    const MAX_TRACKS_PER_KIND = 8;
    const MAX_CLIPS = 2000;
    const TICK_STEPS = [1, 2, 5, 10, 15, 30, 60, 120, 300, 600];
    const Time = () => window.VideoStudioTime;
    const fpsOf = project => Time().rate(project);
    const maxOf = project => Time().maxFrames(project);
    const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
    const clone = value => JSON.parse(JSON.stringify(value));

    function newId(prefix) {
        return prefix + '-' + (crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2));
    }

    function frameToClock(frame, project) {
        const fps = fpsOf(project), total = Math.max(0, frame);
        const seconds = Math.floor(total / fps + 1e-9);
        const frames = Math.floor(total - seconds * fps + 1e-9);
        const minutes = Math.floor(seconds / 60);
        return String(minutes).padStart(2, '0') + ':' + String(seconds % 60).padStart(2, '0') + ':' + String(frames).padStart(2, '0');
    }

    function shortClock(frame, project) { return Time().shortClock(frame, project); }

    function trackCapacity(project, kind) {
        return (project.tracks || []).filter(track => track.kind === kind).length < MAX_TRACKS_PER_KIND;
    }

    function trackFor(project, id) {
        return (project.tracks || []).find(track => track.id === id);
    }

    function clipFor(project, id) {
        for (const track of project.tracks || []) {
            const clip = (track.clips || []).find(item => item.id === id);
            if (clip) return { track, clip };
        }
        return null;
    }

    function makeClip(asset, start, id, project) {
        const max = maxOf(project);
        start = clamp(Math.round(start || 0), 0, max - 1);
        return {
            id: id || newId('clip'), asset_id: asset.id, start, offset: 0,
            duration: Math.max(1, Math.min(Number(asset.duration_frames || Math.round(fpsOf(project) * 5)), max - start)),
            x: 0, y: 0, width: 1, height: 1, rotation: 0, opacity: 1, volume: 1,
            fade_in: 0, fade_out: 0, fit: 'contain', text: '', text_style: null, transition: null
        };
    }

    function accepts(track, asset) {
        return !!asset && (track.kind === 'video' ? asset.kind === 'video' : track.kind === 'overlay' ? asset.kind === 'image' : asset.kind === 'audio' || asset.kind === 'video' && asset.has_audio);
    }

    function projectEnd(project) {
        return Math.max(0, ...(project.tracks || []).flatMap(track => (track.clips || []).map(clip => clip.start + clip.duration)));
    }

    // Mirror timing rules for immediate feedback; the server remains authoritative.
    function validTimeline(project) {
        if (!project || !Array.isArray(project.tracks) || !Array.isArray(project.assets)) return false;
        const max = maxOf(project);
        const assets = new Map(project.assets.map(asset => [asset.id, asset]));
        const ids = new Set(), tracks = new Set();
        const counts = { video: 0, audio: 0, overlay: 0 };
        for (const track of project.tracks) {
            if (!Object.hasOwn(counts, track.kind) || ++counts[track.kind] > MAX_TRACKS_PER_KIND || tracks.has(track.id) || !Array.isArray(track.clips)) return false;
            tracks.add(track.id);
            const clips = track.clips.slice().sort((a, b) => a.start - b.start);
            for (let i = 0; i < clips.length; i++) {
                const clip = clips[i], asset = assets.get(clip.asset_id), next = clips[i + 1];
                if (!accepts(track, asset) || ids.has(clip.id)) return false;
                ids.add(clip.id);
                if (ids.size > MAX_CLIPS) return false;
                if (![clip.start, clip.offset, clip.duration, clip.fade_in, clip.fade_out].every(Number.isSafeInteger) ||
                    clip.start < 0 || clip.duration <= 0 || clip.duration > max - clip.start || clip.offset < 0 ||
                    clip.fade_in < 0 || clip.fade_out < 0 || clip.fade_in > clip.duration || clip.fade_out > clip.duration) return false;
                if (asset.kind !== 'image' && clip.offset > asset.duration_frames - clip.duration) return false;
                if (!Number.isFinite(clip.volume) || clip.volume < 0 || clip.volume > 4) return false;
                if (track.kind !== 'audio' && (![clip.x, clip.y, clip.width, clip.height, clip.rotation, clip.opacity].every(Number.isFinite) ||
                    clip.x < 0 || clip.y < 0 || clip.width <= 0 || clip.height <= 0 || clip.x + clip.width > 1 || clip.y + clip.height > 1 ||
                    Math.abs(clip.rotation) > 360 || clip.opacity < 0 || clip.opacity > 1 || !['contain', 'cover'].includes(clip.fit))) return false;
                if (clip.transition) {
                    const duration = clip.transition.duration;
                    if (track.kind === 'audio' || !next || !['dissolve', 'black', 'wipeleft', 'wiperight'].includes(clip.transition.type) ||
                        !Number.isSafeInteger(duration) || duration <= 0 || duration >= Math.min(clip.duration, next.duration) ||
                        next.start <= clip.start || clip.start + clip.duration - next.start !== duration) return false;
                } else if (next && clip.start + clip.duration > next.start) return false;
                if (clips[i + 2] && clip.start + clip.duration > clips[i + 2].start) return false;
            }
        }
        return true;
    }

    // A transition is the overlap with the next clip. After an edit it follows that overlap, or
    // disappears when the clips no longer overlap (deleted, moved apart or trimmed away).
    function repairTransitions(project) {
        (project.tracks || []).forEach(track => {
            const clips = (track.clips || []).slice().sort((a, b) => a.start - b.start);
            clips.forEach((clip, index) => {
                if (!clip.transition) return;
                const next = clips[index + 1];
                const overlap = next ? clip.start + clip.duration - next.start : 0;
                if (track.kind === 'audio' || !next || overlap <= 0) clip.transition = null;
                else if (overlap < Math.min(clip.duration, next.duration)) clip.transition = Object.assign({}, clip.transition, { duration: overlap });
            });
        });
        return project;
    }

    // Sets, changes or removes the transition after a clip. The following clip and every later clip on
    // the track move together, so a shorter or removed transition never pushes into the next clip.
    function setTransition(project, trackId, clipId, type, duration) {
        const track = trackFor(project, trackId);
        if (!track || track.kind === 'audio') return false;
        const ordered = track.clips.slice().sort((a, b) => a.start - b.start);
        const index = ordered.findIndex(clip => clip.id === clipId), clip = ordered[index], following = ordered[index + 1];
        if (!clip || !following) return type === 'none' && !!clip && (clip.transition = null, true);
        const target = type === 'none' ? 0 : Math.max(1, Math.round(Number(duration) || 0));
        const shift = clip.start + clip.duration - target - following.start;
        ordered.slice(index + 1).forEach(item => { item.start += shift; });
        clip.transition = type === 'none' ? null : { type, duration: target };
        return true;
    }

    // Front-most visual track on top (it paints last), audio tracks below.
    function displayOrder(tracks) {
        const list = tracks || [];
        return list.filter(track => track.kind !== 'audio').reverse().concat(list.filter(track => track.kind === 'audio'));
    }

    function icon(options, name, size) { return options.icon ? options.icon(name, size || 15) : ''; }
    function label(options, key, fallback) { return options.label ? options.label(key, fallback) : fallback || key; }

    function trackHeader(options, project, track) {
        const esc = options.esc, kind = track.kind;
        const kindIcon = kind === 'audio' ? 'audio' : kind === 'overlay' ? 'layers' : 'video';
        const button = (action, on, iconOn, iconOff, keyOn, fallbackOn, keyOff, fallbackOff) => {
            const text = on ? label(options, keyOn, fallbackOn) : label(options, keyOff, fallbackOff);
            return `<button type="button" data-track-action="${action}" aria-pressed="${on}" title="${esc(text)}" aria-label="${esc(text)} – ${esc(track.name)}">${icon(options, on ? iconOn : iconOff, 15)}</button>`;
        };
        const actions = [
            kind !== 'overlay' ? button('mute', !!track.muted, 'speakerOff', 'speaker', 'unmute', 'Unmute', 'mute', 'Mute') : '',
            kind !== 'audio' ? button('hide', !!track.hidden, 'eyeOff', 'eye', 'show', 'Show', 'hide', 'Hide') : '',
            button('lock', !!track.locked, 'lock', 'unlock', 'unlock', 'Unlock', 'lock', 'Lock'),
            !(track.clips || []).length ? `<button type="button" data-track-action="remove" title="${esc(label(options, 'removeTrack', 'Remove empty track'))}" aria-label="${esc(label(options, 'removeTrack', 'Remove empty track'))} – ${esc(track.name)}">${icon(options, 'trash', 14)}</button>` : ''
        ].join('');
        return `<div class="vs-track-label" draggable="true" data-track-drag="${esc(track.id)}" title="${esc(label(options, 'dragTrack', 'Drag to reorder'))}"><span class="vs-track-kind vs-kind-${esc(kind)}">${icon(options, kindIcon, 14)}</span><span class="vs-track-name">${esc(track.name)}</span><span class="vs-track-actions">${actions}</span></div>`;
    }

    function clipMarkup(options, project, track, clip, pps) {
        const fps = fpsOf(project);
        const s = options.state, esc = options.esc;
        const asset = project.assets.find(a => a.id === clip.asset_id);
        const left = clip.start / fps * pps;
        const width = Math.max(2, clip.duration / fps * pps);
        const selected = options.isSelected ? options.isSelected(clip.id) : s.selectedClipId === clip.id;
        const kind = clip.text ? 'text' : track.kind === 'audio' ? 'audio' : track.kind === 'overlay' ? 'overlay' : 'video';
        const name = clip.text ? clip.text.split(/\r?\n/)[0] : asset ? (options.assetName ? options.assetName(asset) : asset.name) : label(options, 'missingMedia', 'Missing media');
        const media = options.clipMedia && asset ? options.clipMedia(asset, clip, track, pps / fps, width) : '';
        const fadeIn = clip.fade_in ? `<span class="vs-fade vs-fade-in" style="width:${Math.max(2, clip.fade_in / fps * pps)}px"></span>` : '';
        const fadeOut = clip.fade_out ? `<span class="vs-fade vs-fade-out" style="width:${Math.max(2, clip.fade_out / fps * pps)}px"></span>` : '';
        const transition = clip.transition && clip.transition.duration ? `<span class="vs-transition" style="width:${Math.max(4, clip.transition.duration / fps * pps)}px" title="${esc(label(options, clip.transition.type, clip.transition.type))}">${icon(options, 'transition', 12)}</span>` : '';
        const kindIcon = kind === 'text' ? 'text' : kind === 'audio' ? 'audio' : kind === 'overlay' ? 'image' : 'video';
        const range = `${shortClock(clip.start, project)}–${shortClock(clip.start + clip.duration, project)}`;
        return `<div class="vs-clip vs-clip-${kind}${selected ? ' is-selected' : ''}${width < 46 ? ' is-narrow' : ''}" data-clip-id="${esc(clip.id)}" data-track-id="${esc(track.id)}" title="${esc(name)} · ${esc(range)}" style="left:${left}px;width:${width}px" tabindex="0" role="button" aria-pressed="${selected}" aria-label="${esc(name)} ${esc(range)}"><span class="vs-clip-media">${media}</span>${fadeIn}${fadeOut}<span class="vs-clip-label">${icon(options, kindIcon, 12)}<span class="vs-clip-name">${esc(name)}</span><span class="vs-clip-dur">${esc(shortClock(clip.duration, project))}</span></span>${transition}<button type="button" class="vs-trim vs-trim-start" data-trim="start" tabindex="-1" aria-label="${esc(label(options, 'trimStart', 'Trim start'))}"></button><button type="button" class="vs-trim vs-trim-end" data-trim="end" tabindex="-1" aria-label="${esc(label(options, 'trimEnd', 'Trim end'))}"></button></div>`;
    }

    // Re-rendering replaces every control; remember which one had keyboard focus and restore it.
    function focusSelector(root) {
        const el = document.activeElement;
        if (!el || !root.contains(el) || !el.dataset) return '';
        const q = value => `"${CSS.escape(value)}"`;
        if (el.dataset.trackAction) return `[data-track-row=${q(el.closest('[data-track-row]').dataset.trackRow)}] [data-track-action=${q(el.dataset.trackAction)}]`;
        if (el.dataset.clipId) return `[data-clip-id=${q(el.dataset.clipId)}]`;
        if (el.dataset.addTrack) return `[data-add-track=${q(el.dataset.addTrack)}]`;
        if (el.dataset.action) return `[data-action=${q(el.dataset.action)}]`;
        if (el.matches('[data-zoom]')) return '[data-zoom]';
        if (el.matches('[data-playhead]')) return '[data-playhead]';
        return '';
    }
    function tickInterval(pps) {
        return TICK_STEPS.find(step => step * pps >= 72) || 600;
    }
    // Whole ruler seconds as m:ss, h:mm:ss from one hour on.
    function tickLabel(sec) {
        const h = Math.floor(sec / 3600), m = Math.floor(sec % 3600 / 60), rest = String(sec % 60).padStart(2, '0');
        return h ? `${h}:${String(m).padStart(2, '0')}:${rest}` : `${m}:${rest}`;
    }

    function render(root, options) {
        const s = options.state;
        const project = s.project;
        if (!project) { root.replaceChildren(); return; }
        const esc = options.esc;
        const priorScroll = root.querySelector('.vs-timeline-scroll');
        const scrollLeft = priorScroll ? priorScroll.scrollLeft : 0;
        const scrollTop = priorScroll ? priorScroll.scrollTop : 0;
        const menuOpen = !!root.querySelector('[data-track-menu]:not([hidden])');
        const focused = focusSelector(root);
        const zoom = s.zoom || 1;
        const pps = BASE_PX_PER_SECOND * zoom;
        const fps = fpsOf(project);
        const totalFrames = Math.max(fps * 20, projectEnd(project) + fps * 6, s.frame + fps * 4);
        const width = Math.ceil(totalFrames / fps * pps) + 120;
        const interval = tickInterval(pps);
        const ticks = [];
        for (let sec = 0; sec <= totalFrames / fps + interval; sec += interval) {
            ticks.push(`<span class="vs-tick" style="left:${Math.round(sec * pps)}px">${tickLabel(sec)}</span>`);
        }
        const hasClips = project.tracks.some(track => track.clips.length);
        const firstVisual = displayOrder(project.tracks).find(track => track.kind === 'video') || project.tracks[0];
        const rows = displayOrder(project.tracks).map(track => {
            const clips = track.clips.slice().sort((a, b) => a.start - b.start).map(clip => clipMarkup(options, project, track, clip, pps)).join('');
            const hint = !hasClips && track === firstVisual ? `<span class="vs-lane-hint">${icon(options, 'plus', 14)}${esc(label(options, 'laneHint', 'Drag media here or use + in the library'))}</span>` : '';
            const flags = (track.locked ? ' is-locked' : '') + (track.muted ? ' is-muted' : '') + (track.hidden ? ' is-hidden' : '');
            return `<div class="vs-track-row vs-row-${esc(track.kind)}${flags}" data-track-row="${esc(track.id)}">${trackHeader(options, project, track)}<div class="vs-track-lane" data-track-id="${esc(track.id)}" style="width:${width}px">${hint}${clips}</div></div>`;
        }).join('');
        const empty = project.tracks.length ? '' : `<div class="vs-tl-empty">${esc(label(options, 'noTracks', 'This project has no tracks yet.'))}</div>`;
        const selected = !!s.selectedClipId;
        const disabledEdit = selected && !s.readonly ? '' : 'disabled';
        const snap = s.snap !== false;
        const tool = s.readonly ? 'select' : s.tool || 'select';
        const toolButton = (name, iconName, key, fallback) => `<button type="button" data-action="tool-${name}" aria-pressed="${tool === name}" title="${esc(label(options, key, fallback))}" aria-label="${esc(label(options, key, fallback))}">${icon(options, iconName)}</button>`;
        const tools = `<div class="vs-tool-group" role="group" aria-label="${esc(label(options, 'tools', 'Edit tools'))}">${toolButton('select', 'pointer', 'toolSelect', 'Selection tool (V)')}${toolButton('trim', 'trim', 'toolTrim', 'Trim tool (T)')}${toolButton('blade', 'blade', 'toolBlade', 'Blade tool (B)')}</div>`;
        const modeKey = s.trimMode ? 'trim' + s.trimMode.charAt(0).toUpperCase() + s.trimMode.slice(1) : '';
        const badge = modeKey ? `<span class="vs-mode-badge">${esc(label(options, modeKey, s.trimMode))}</span>` : '';
        const magnetic = `<button type="button" class="vs-toggle" data-action="magnetic" aria-pressed="${!!s.magnetic}" title="${esc(label(options, 'magneticHint', 'Close gaps when clips are moved or deleted'))}">${icon(options, 'ripple')}<span>${esc(label(options, 'magnetic', 'Magnetic'))}</span></button>`;
        const addTrack = ['video', 'audio', 'overlay'].map(kind => `<button type="button" role="menuitem" data-add-track="${kind}" ${trackCapacity(project, kind) && !s.readonly ? '' : 'disabled'}>${icon(options, kind === 'audio' ? 'audio' : kind === 'overlay' ? 'layers' : 'video', 15)}${esc(label(options, kind + 'Track', kind))}</button>`).join('');
        root.innerHTML = `<div class="vs-timeline-toolbar"><div class="vs-edit-tools">${tools}${badge}<button type="button" data-action="split" ${disabledEdit} title="${esc(label(options, 'splitHintShort', 'Split at the playhead (S)'))}">${icon(options, 'scissors')}<span>${esc(label(options, 'split', 'Split'))}</span></button><button type="button" data-action="duplicate" ${disabledEdit}>${icon(options, 'copy')}<span>${esc(label(options, 'duplicate', 'Duplicate'))}</span></button><button type="button" data-action="delete" ${disabledEdit} title="${esc(label(options, 'deleteHint', 'Delete (Del)'))}">${icon(options, 'trash')}<span>${esc(label(options, 'delete', 'Delete'))}</span></button></div><div class="vs-timeline-tools"><div class="vs-menu-anchor"><button type="button" data-action="track-menu" aria-haspopup="menu" aria-expanded="${menuOpen}" ${s.readonly ? 'disabled' : ''}>${icon(options, 'plus')}<span>${esc(label(options, 'addTrack', 'Track'))}</span></button><div class="vs-popover vs-track-menu" data-track-menu role="menu" ${menuOpen ? '' : 'hidden'}>${addTrack}</div></div><button type="button" class="vs-toggle" data-action="snap" aria-pressed="${snap}" title="${esc(label(options, 'snapHint', 'Snap clips to edges and the playhead'))}">${icon(options, 'magnet')}<span>${esc(label(options, 'snap', 'Snap'))}</span></button>${magnetic}<div class="vs-timeline-zoom"><button type="button" data-action="zoom-out" aria-label="${esc(label(options, 'zoomOut', 'Zoom out'))}" title="${esc(label(options, 'zoomOut', 'Zoom out'))}">${icon(options, 'minus')}</button><input type="range" data-zoom aria-label="${esc(label(options, 'timelineZoom', 'Timeline zoom'))}"><button type="button" data-action="zoom-in" aria-label="${esc(label(options, 'zoomIn', 'Zoom in'))}" title="${esc(label(options, 'zoomIn', 'Zoom in'))}">${icon(options, 'plus')}</button><button type="button" data-action="zoom-fit" title="${esc(label(options, 'zoomFit', 'Show whole project'))}">${icon(options, 'fit')}<span>${esc(label(options, 'zoomFitShort', 'Fit'))}</span></button></div></div></div><div class="vs-timeline-scroll"><div class="vs-tl-content" style="width:calc(var(--vs-head-w) + ${width}px)"><div class="vs-ruler"><div class="vs-ruler-label">${icon(options, 'clock', 13)}<span data-timeline-time>${esc(shortClock(s.frame, project))}</span></div><div class="vs-ruler-scale" data-ruler style="width:${width}px;--vs-tick:${interval * pps}px">${ticks.join('')}</div></div><div class="vs-track-list">${rows}${empty}</div><div class="vs-playhead" style="left:calc(var(--vs-head-w) + ${s.frame / fps * pps}px)"><span class="vs-playhead-knob" data-playhead></span></div></div></div>`;
        const scroll = root.querySelector('.vs-timeline-scroll');
        scroll.scrollLeft = scrollLeft; scroll.scrollTop = scrollTop;
        const slider = root.querySelector('[data-zoom]');
        slider.min = String(MIN_ZOOM * 100); slider.max = String(MAX_ZOOM * 100); slider.step = '0.5'; slider.value = String(zoom * 100);
        root.querySelectorAll('[data-trim]').forEach(button => { button.disabled = s.readonly || !!trackFor(project, button.closest('[data-track-id]').dataset.trackId)?.locked; });
        if (s.readonly) {
            root.querySelectorAll('[data-track-action], [data-add-track], [data-action="split"], [data-action="duplicate"], [data-action="delete"], [data-action="tool-trim"], [data-action="tool-blade"], [data-action="magnetic"]').forEach(button => { button.disabled = true; });
            root.querySelectorAll('[draggable]').forEach(element => { element.draggable = false; });
        }
        root.dataset.tool = tool;
        if (focused) root.querySelector(focused)?.focus({ preventScroll: true });
    }

    function setPlayhead(root, state, follow) {
        const head = root.querySelector('.vs-playhead');
        if (!head) return;
        const pps = BASE_PX_PER_SECOND * (state.zoom || 1);
        const x = state.frame / fpsOf(state.project) * pps;
        head.style.left = `calc(var(--vs-head-w) + ${x}px)`;
        const time = root.querySelector('[data-timeline-time]');
        if (time) time.textContent = shortClock(state.frame, state.project);
        if (!follow) return;
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        if (!scroll || !labelEl) return;
        const visible = scroll.clientWidth - labelEl.offsetWidth;
        if (x < scroll.scrollLeft || x > scroll.scrollLeft + visible - 32) scroll.scrollLeft = Math.max(0, x - 48);
    }

    function revealFrame(root, state, frame) {
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        if (!scroll || !labelEl) return;
        const x = frame / fpsOf(state.project) * BASE_PX_PER_SECOND * (state.zoom || 1);
        const visible = scroll.clientWidth - labelEl.offsetWidth;
        if (x < scroll.scrollLeft || x > scroll.scrollLeft + visible - 32) scroll.scrollLeft = Math.max(0, x - 48);
    }

    function fitZoom(root, project) {
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        const visible = Math.max(120, (scroll ? scroll.clientWidth : 800) - (labelEl ? labelEl.offsetWidth : 160) - 40);
        const seconds = Math.max(5, projectEnd(project) / fpsOf(project));
        return clamp(visible / (seconds * BASE_PX_PER_SECOND), MIN_ZOOM, MAX_ZOOM);
    }

    function dragClip(before, clipId, mode, delta, targetTrackId, snapFrames, extraPoints) {
        const project = clone(before), source = clipFor(project, clipId);
        const max = maxOf(project);
        if (!source || source.track.locked) return null;
        const clip = source.clip, asset = project.assets.find(item => item.id === clip.asset_id);
        const extra = Array.isArray(extraPoints) ? extraPoints.filter(Number.isFinite) : [];
        if (mode === 'move') {
            const target = trackFor(project, targetTrackId) || source.track;
            if (target.locked || !accepts(target, asset)) return null;
            let start = clamp(clip.start + delta, 0, max - clip.duration);
            const edges = target.clips.filter(item => item.id !== clip.id).flatMap(item => [item.start, item.start + item.duration]).concat(extra);
            const positions = [0, ...edges, ...edges.map(edge => edge - clip.duration)];
            const nearby = positions.filter(point => point >= 0 && point <= max - clip.duration && Math.abs(point - start) <= snapFrames);
            nearby.sort((a, b) => Math.abs(a - start) - Math.abs(b - start));
            if (nearby.length) start = nearby[0];
            clip.start = start;
            if (target !== source.track) {
                source.track.clips = source.track.clips.filter(item => item.id !== clip.id);
                target.clips.push(clip);
            }
        } else if (mode === 'start') {
            let actual = clamp(delta, asset.kind === 'image' ? -clip.start : Math.max(-clip.start, -clip.offset), clip.duration - 1);
            const snapped = extra.find(point => Math.abs(point - (clip.start + actual)) <= snapFrames);
            if (snapped != null) actual = clamp(snapped - clip.start, asset.kind === 'image' ? -clip.start : Math.max(-clip.start, -clip.offset), clip.duration - 1);
            clip.start += actual; clip.offset = asset.kind === 'image' ? 0 : clip.offset + actual; clip.duration -= actual;
        } else {
            const sourceLimit = asset.kind === 'image' ? max : asset.duration_frames - clip.offset;
            let duration = clip.duration + delta;
            const snapped = extra.find(point => Math.abs(point - (clip.start + duration)) <= snapFrames);
            if (snapped != null) duration = snapped - clip.start;
            clip.duration = clamp(duration, 1, Math.min(sourceLimit, max - clip.start));
        }
        clip.fade_in = Math.min(clip.fade_in, clip.duration);
        clip.fade_out = Math.min(clip.fade_out, clip.duration);
        repairTransitions(project);
        return validTimeline(project) ? project : null;
    }

    function attach(root, options) {
        let drag = null, scrub = null, band = null, dropMarker = null, suppressClick = false;
        const s = () => options.state;
        const Edit = () => window.VideoStudioEdit;

        function pps() { return BASE_PX_PER_SECOND * (s().zoom || 1); }
        function frameAt(element, clientX) {
            const rect = element.getBoundingClientRect();
            return clamp(Math.round((clientX - rect.left) / pps() * fpsOf(s().project)), 0, maxOf(s().project));
        }
        function snapFrames() { return s().snap === false ? 0 : Math.max(1, Math.round(8 / pps() * fpsOf(s().project))); }
        function tool() { return s().readonly ? 'select' : s().tool || 'select'; }
        function setZoom(value, anchorClientX) {
            const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
            const before = pps(), offset = scroll && labelEl ? (anchorClientX == null ? (scroll.clientWidth - labelEl.offsetWidth) / 2 : anchorClientX - scroll.getBoundingClientRect().left - labelEl.offsetWidth) : 0;
            const time = scroll ? (scroll.scrollLeft + offset) / before : 0;
            s().zoom = clamp(value, MIN_ZOOM, MAX_ZOOM);
            render(root, options);
            const next = root.querySelector('.vs-timeline-scroll');
            if (next) next.scrollLeft = Math.max(0, time * pps() - offset);
            if (options.onZoom) options.onZoom(s().zoom);
        }
        function closeMenu() { const menu = root.querySelector('[data-track-menu]'); if (menu && !menu.hidden) { menu.hidden = true; root.querySelector('[data-action="track-menu"]')?.setAttribute('aria-expanded', 'false'); } }
        // The blade cuts at the pointer, or at the playhead when snapping is on and the pointer is near it.
        function bladeFrame(lane, clientX) {
            const frame = frameAt(lane, clientX), range = snapFrames();
            return range && Math.abs(frame - s().frame) <= range ? s().frame : frame;
        }
        // Dragging onto another track of the same kind moves the clips that many tracks within the kind.
        function trackShift(project, fromId, toId) {
            const from = trackFor(project, fromId), to = trackFor(project, toId);
            if (!from || !to || from.kind !== to.kind) return 0;
            const list = project.tracks.filter(track => track.kind === from.kind);
            return list.indexOf(to) - list.indexOf(from);
        }
        function endTrimMode() { s().trimMode = ''; delete root.dataset.trimMode; }

        // Applies the current gesture to a copy of the project taken when it started. Returns the copy, the
        // edit result and the frame the preview should show (-1 keeps the playhead). Edges snap to the playhead
        // from the gesture's start: the trim peek moves the playhead to the edited edge while dragging.
        function applyGesture(event, delta) {
            const E = Edit(), before = drag.before, project = clone(before), range = snapFrames(), extra = [drag.playhead];
            const source = clipFor(before, drag.clipId), anchor = source.clip, ownTrack = source.track.clips.map(clip => clip.id);
            const settled = id => clipFor(project, id)?.clip;
            if (drag.type === 'move') {
                const d = E.snapDelta([anchor.start, anchor.start + anchor.duration], delta, E.snapPoints(before, drag.ids, extra), range);
                const row = document.elementFromPoint(event.clientX, event.clientY)?.closest('[data-track-row]');
                const mode = event.ctrlKey || event.metaKey ? 'insert' : 'overwrite';
                return { project, result: E.moveClips(project, drag.ids, anchor.id, d, trackShift(before, drag.trackId, row?.dataset.trackRow), mode, { magnetic: !!s().magnetic }), peek: -1 };
            }
            if (drag.op === 'ripple') {
                const d = drag.edge === 'end' ? E.snapDelta([anchor.start + anchor.duration], delta, E.snapPoints(before, ownTrack, extra), range) : delta;
                const result = E.rippleTrim(project, [anchor.id], drag.edge, d), clip = settled(anchor.id);
                return { project, result, peek: clip ? (drag.edge === 'end' ? clip.start + clip.duration - 1 : clip.start) : -1 };
            }
            if (drag.op === 'roll') {
                const cut = clipFor(before, drag.pair[1]).clip.start;
                const result = E.roll(project, [drag.pair], E.snapDelta([cut], delta, E.snapPoints(before, drag.pair, extra), range)), right = settled(drag.pair[1]);
                return { project, result, peek: right ? right.start : -1 };
            }
            if (drag.op === 'slip') return { project, result: E.slip(project, [anchor.id], delta), peek: anchor.start };
            const result = E.slide(project, anchor.id, E.snapDelta([anchor.start, anchor.start + anchor.duration], delta, E.snapPoints(before, ownTrack, extra), range)), clip = settled(anchor.id);
            return { project, result, peek: clip ? clip.start : -1 };
        }

        function pointerMove(event) {
            if (scrub) { options.onFrame(frameAt(scrub.ruler, event.clientX)); return; }
            if (band) { moveBand(event); return; }
            if (!drag || s().readonly) return;
            if (drag.epoch !== s().projectEpoch || drag.projectId !== s().projectId) {
                drag = null; s().timelineDragging = false; endTrimMode(); return;
            }
            const delta = Math.round((event.clientX - drag.x) / pps() * fpsOf(s().project));
            if (drag.type === 'edge') {
                const next = dragClip(drag.before, drag.clipId, drag.mode, delta, '', snapFrames(), [s().frame]);
                drag.invalid = !next; drag.reason = '';
                if (!next) return;
                s().project = next;
            } else {
                const { project, result, peek } = applyGesture(event, delta);
                // Reaching a limit is not an error: the edit stops there, like hitting a wall.
                const unchanged = result === 'nothing' || result === 'bounds';
                drag.invalid = result !== true && !unchanged;
                drag.reason = drag.invalid ? result : '';
                if (drag.invalid) return;
                s().project = result === true ? project : clone(drag.before);
                if (result === true && peek >= 0 && options.onPeek) options.onPeek(peek);
            }
            drag.moved = drag.moved || delta !== 0;
            options.onPreviewChange();
            render(root, options);
        }

        // Rubber band on empty lane space: it starts after 4 px, highlights the clips it touches without
        // re-rendering and selects them on release (Shift/Ctrl adds to the selection).
        function moveBand(event) {
            const content = root.querySelector('.vs-tl-content');
            if (!content) return;
            if (!band.el) {
                if (Math.hypot(event.clientX - band.x, event.clientY - band.y) < 4) return;
                band.el = document.createElement('div');
                band.el.className = 'vs-band';
                content.append(band.el);
                capture(band);
            }
            const rect = content.getBoundingClientRect();
            const left = Math.min(band.x, event.clientX), right = Math.max(band.x, event.clientX), top = Math.min(band.y, event.clientY), bottom = Math.max(band.y, event.clientY);
            Object.assign(band.el.style, { left: left - rect.left + 'px', top: top - rect.top + 'px', width: right - left + 'px', height: bottom - top + 'px' });
            const hits = Array.from(root.querySelectorAll('.vs-clip')).filter(el => { const r = el.getBoundingClientRect(); return r.left < right && r.right > left && r.top < bottom && r.bottom > top; }).map(el => el.dataset.clipId);
            band.ids = band.additive ? Array.from(new Set(band.base.concat(hits))) : hits;
            root.querySelectorAll('.vs-clip').forEach(el => el.classList.toggle('is-selected', band.ids.includes(el.dataset.clipId)));
        }
        function finishBand(cancelled) {
            const done = band;
            band = null;
            if (!done.el) return; // a plain click: the click handler seeks and deselects
            done.el.remove();
            suppressClick = true;
            if (!cancelled) options.onSelectMany(done.ids, done.ids[done.ids.length - 1] || '');
            render(root, options);
        }

        // Gestures capture the pointer on the timeline container (clips re-render while they move), so the
        // release arrives even outside the window; a lost capture or a window switch cancels the gesture.
        function capture(event) { try { root.setPointerCapture(event.pointerId); } catch (_) { /* synthetic or finished pointer */ } }
        function cancelGesture() { if (drag || scrub || band) pointerUp({ type: 'pointercancel' }); }
        function pointerUp(event) {
            if (scrub) { scrub = null; return; }
            if (band) { finishBand(event?.type === 'pointercancel'); return; }
            if (!drag) return;
            const { before, invalid, reason, moved, type, clipId, ids } = drag;
            const current = drag.epoch === s().projectEpoch && drag.projectId === s().projectId;
            drag = null;
            s().timelineDragging = false;
            endTrimMode();
            if (type === 'trim' && options.onPeekEnd) options.onPeekEnd();
            if (!current) return;
            if (moved) suppressClick = true;
            if (event?.type === 'pointercancel' || s().readonly) s().project = before;
            else if (JSON.stringify(before) !== JSON.stringify(s().project)) options.onChange('timeline', before);
            // The click after a captured press never reaches the clip (the re-render replaces it), so a press
            // without movement on a clip of a multi-selection selects only that clip here.
            else if (type === 'move' && !moved && ids.length > 1) options.onSelect(clipId);
            if (invalid && options.onInvalid) options.onInvalid(reason);
            if (options.onDragEnd) options.onDragEnd();
            render(root, options);
        }

        function click(event) {
            // A drag or rubber band ends with a click on whatever lies under the pointer; ignore it once.
            if (suppressClick) { suppressClick = false; return; }
            if (!event.target.closest('.vs-menu-anchor')) closeMenu();
            const action = event.target.closest('[data-action]');
            if (action) {
                const id = s().selectedClipId, name = action.dataset.action;
                if (name === 'zoom-in' || name === 'zoom-out') setZoom((s().zoom || 1) * (name === 'zoom-in' ? 1.25 : 0.8));
                else if (name === 'zoom-fit') { setZoom(fitZoom(root, s().project)); const scroll = root.querySelector('.vs-timeline-scroll'); if (scroll) scroll.scrollLeft = 0; }
                else if (name === 'snap') { s().snap = s().snap === false; if (options.onSnap) options.onSnap(s().snap); render(root, options); }
                else if (name === 'track-menu') {
                    const menu = root.querySelector('[data-track-menu]');
                    menu.hidden = !menu.hidden; action.setAttribute('aria-expanded', String(!menu.hidden));
                    if (!menu.hidden) menu.querySelector('button:not(:disabled)')?.focus();
                } else if (options.onCommand && options.onCommand(name)) { /* tools, magnetic and selection edits */ }
                else if (id && !s().readonly) options.onAction(name, id);
                return;
            }
            const trackAction = event.target.closest('[data-track-action]');
            if (trackAction && !s().readonly) {
                const row = trackAction.closest('[data-track-row]');
                options.onTrackAction(row.dataset.trackRow, trackAction.dataset.trackAction);
                return;
            }
            const add = event.target.closest('[data-add-track]');
            if (add && !add.disabled && !s().readonly) { closeMenu(); options.onAddTrack(add.dataset.addTrack); return; }
            const clipEl = event.target.closest('[data-clip-id]');
            if (clipEl) {
                if (tool() === 'blade') {
                    if (options.onBlade) options.onBlade(clipEl.dataset.trackId, bladeFrame(clipEl.closest('.vs-track-lane'), event.clientX), event.shiftKey);
                    return;
                }
                if ((event.shiftKey || event.ctrlKey || event.metaKey) && options.onToggleSelect) options.onToggleSelect(clipEl.dataset.clipId);
                else options.onSelect(clipEl.dataset.clipId);
                render(root, options);
                return;
            }
            const lane = event.target.closest('.vs-track-lane');
            if (lane) {
                options.onFrame(frameAt(lane, event.clientX));
                if (s().selectedClipId) { options.onSelect(''); render(root, options); }
            }
        }

        function pointerDown(event) {
            suppressClick = false;
            if (event.button !== 0) return;
            const ruler = event.target.closest('[data-ruler]') || (event.target.closest('[data-playhead]') && root.querySelector('[data-ruler]'));
            if (ruler) {
                scrub = { ruler };
                capture(event);
                if (options.onScrubStart) options.onScrubStart();
                options.onFrame(frameAt(ruler, event.clientX));
                event.preventDefault();
                return;
            }
            const clipEl = event.target.closest('[data-clip-id]');
            if (!clipEl) {
                if (event.target.closest('.vs-track-lane') && !s().readonly && options.onSelectMany) band = { x: event.clientX, y: event.clientY, pointerId: event.pointerId, additive: event.shiftKey || event.ctrlKey || event.metaKey, base: options.selectedIds ? options.selectedIds() : [], ids: [], el: null };
                return;
            }
            if (s().readonly || tool() === 'blade') return;
            const target = clipFor(s().project, clipEl.dataset.clipId);
            if (!target || target.track.locked) return;
            const handle = event.target.closest('[data-trim]')?.dataset.trim || '';
            const selected = !!(options.isSelected && options.isSelected(target.clip.id));
            const common = { clipId: target.clip.id, x: event.clientX, playhead: s().frame, before: clone(s().project), epoch: s().projectEpoch, projectId: s().projectId, moved: false, invalid: false, reason: '' };
            if (tool() === 'trim') {
                // A cut between two clips rolls (Shift: ripple), a free edge ripples, the body slips (Alt: slides).
                const pair = handle && !event.shiftKey ? Edit().neighborAt(s().project, target.clip.id, handle) : null;
                const op = handle ? (pair ? 'roll' : 'ripple') : event.altKey ? 'slide' : 'slip';
                drag = Object.assign(common, { type: 'trim', op, edge: handle, pair });
                s().trimMode = op; root.dataset.trimMode = op;
            } else {
                // Shift/Ctrl+click changes the selection on click and never starts a drag.
                if (event.shiftKey || event.ctrlKey || event.metaKey) return;
                drag = handle ? Object.assign(common, { type: 'edge', mode: handle }) : Object.assign(common, { type: 'move', trackId: target.track.id, ids: selected && options.selectedIds ? options.selectedIds() : [target.clip.id] });
            }
            if (!selected) options.onSelect(target.clip.id);
            s().timelineDragging = true;
            capture(event);
            clearTimeout(s().autosaveTimer);
            event.preventDefault();
        }

        function dragStart(event) {
            if (s().readonly) { event.preventDefault(); return; }
            const trackLabel = event.target.closest('[data-track-drag]');
            if (trackLabel) {
                event.dataTransfer.setData('application/x-video-studio-track', trackLabel.dataset.trackDrag);
                event.dataTransfer.effectAllowed = 'move';
                return;
            }
            const card = event.target.closest('[data-asset-id]');
            if (card) {
                event.dataTransfer.setData('application/x-video-studio-asset', card.dataset.assetId);
                event.dataTransfer.effectAllowed = 'copy';
            }
        }
        function clearMarker() { if (dropMarker) { dropMarker.remove(); dropMarker = null; } root.querySelectorAll('.vs-track-lane.is-drop').forEach(el => el.classList.remove('is-drop')); }
        function dragOver(event) {
            const lane = event.target.closest('.vs-track-lane');
            if (s().readonly || !lane) return;
            event.preventDefault();
            const types = Array.from(event.dataTransfer.types || []);
            if (!types.includes('application/x-video-studio-asset')) { event.dataTransfer.dropEffect = types.includes('Files') ? 'copy' : 'move'; return; }
            event.dataTransfer.dropEffect = 'copy';
            if (!dropMarker) { dropMarker = document.createElement('span'); dropMarker.className = 'vs-drop-marker'; }
            if (dropMarker.parentElement !== lane) { clearMarker(); dropMarker = document.createElement('span'); dropMarker.className = 'vs-drop-marker'; lane.append(dropMarker); lane.classList.add('is-drop'); }
            dropMarker.style.left = Math.round(frameAt(lane, event.clientX) / fpsOf(s().project) * pps()) + 'px';
        }
        function dragLeave(event) { if (!root.contains(event.relatedTarget)) clearMarker(); }
        function drop(event) {
            const lane = event.target.closest('.vs-track-lane');
            clearMarker();
            if (!lane || s().readonly) return;
            event.preventDefault();
            const project = s().project;
            const track = trackFor(project, lane.dataset.trackId);
            if (!track || track.locked) return;
            const at = clamp(frameAt(lane, event.clientX), 0, maxOf(project) - 1);
            const assetId = event.dataTransfer.getData('application/x-video-studio-asset');
            const reorderId = event.dataTransfer.getData('application/x-video-studio-track');
            // Dropping media overwrites at the drop point; Ctrl inserts and moves later clips.
            if (assetId) options.onDropAsset(assetId, track.id, at, event.ctrlKey || event.metaKey ? 'insert' : 'overwrite');
            else if (reorderId && reorderId !== track.id) options.onReorderTrack(reorderId, track.id);
        }
        function zoomInput(event) {
            if (!event.target.matches('[data-zoom]')) return;
            setZoom(Number(event.target.value) / 100);
            root.querySelector('[data-zoom]')?.focus();
        }
        function wheel(event) {
            if (!(event.ctrlKey || event.metaKey) || !event.target.closest('.vs-timeline-scroll')) return;
            event.preventDefault();
            setZoom((s().zoom || 1) * (event.deltaY < 0 ? 1.15 : 1 / 1.15), event.clientX);
        }
        function keyDown(event) {
            suppressClick = false;
            if (event.key === 'Escape' && root.querySelector('[data-track-menu]:not([hidden])')) { closeMenu(); root.querySelector('[data-action="track-menu"]')?.focus(); event.preventDefault(); event.stopPropagation(); return; }
            const clip = event.target.closest('[data-clip-id]');
            if (clip && (event.key === 'Enter' || event.key === ' ') && !event.target.closest('button')) {
                event.preventDefault(); event.stopPropagation();
                options.onSelect(clip.dataset.clipId);
                render(root, options);
                root.querySelector(`[data-clip-id="${CSS.escape(clip.dataset.clipId)}"]`)?.focus();
            }
        }
        // The blade tool shows where it would cut.
        function hover(event) {
            let line = root.querySelector('.vs-blade-line');
            const lane = tool() === 'blade' && !drag && !band && !scrub ? event.target.closest('.vs-track-lane') : null;
            if (!lane) { if (line) line.remove(); return; }
            if (!line || line.parentElement !== lane) { if (line) line.remove(); line = document.createElement('span'); line.className = 'vs-blade-line'; lane.append(line); }
            line.style.left = Math.round(bladeFrame(lane, event.clientX) / fpsOf(s().project) * pps()) + 'px';
        }
        function leave() { root.querySelector('.vs-blade-line')?.remove(); }
        // Right-click, the context-menu key and Shift+F10 open the clip or lane menu. A keyboard-opened menu
        // has no pointer position inside the clip, so it anchors below the focused clip.
        function contextMenu(event) {
            const clipEl = event.target.closest('[data-clip-id]'), lane = event.target.closest('.vs-track-lane');
            if (!options.onContextMenu || !s().project || (!clipEl && !lane)) return;
            event.preventDefault(); event.stopPropagation();
            const anchor = clipEl || lane, rect = anchor.getBoundingClientRect();
            const inside = event.clientX >= rect.left && event.clientX <= rect.right && event.clientY >= rect.top && event.clientY <= rect.bottom;
            const x = inside ? event.clientX : rect.left + Math.min(24, rect.width / 2), y = inside ? event.clientY : rect.bottom;
            options.onContextMenu({ clipId: clipEl ? clipEl.dataset.clipId : '', trackId: anchor.dataset.trackId, frame: frameAt(lane || clipEl.closest('.vs-track-lane'), x), x, y, invoker: clipEl });
        }
        root.addEventListener('click', click);
        root.addEventListener('pointerdown', pointerDown);
        root.addEventListener('pointermove', hover);
        root.addEventListener('pointerleave', leave);
        root.addEventListener('contextmenu', contextMenu);
        root.addEventListener('dragstart', dragStart);
        root.addEventListener('dragover', dragOver);
        root.addEventListener('dragleave', dragLeave);
        root.addEventListener('drop', drop);
        root.addEventListener('input', zoomInput);
        root.addEventListener('keydown', keyDown);
        root.addEventListener('wheel', wheel, { passive: false });
        window.addEventListener('pointermove', pointerMove);
        window.addEventListener('pointerup', pointerUp);
        window.addEventListener('pointercancel', pointerUp);
        root.addEventListener('lostpointercapture', cancelGesture);
        window.addEventListener('blur', cancelGesture);
        window.addEventListener('dragend', clearMarker);
        return function detach() {
            drag = null; scrub = null; band = null; clearMarker(); s().timelineDragging = false; endTrimMode();
            root.removeEventListener('click', click);
            root.removeEventListener('pointerdown', pointerDown);
            root.removeEventListener('pointermove', hover);
            root.removeEventListener('pointerleave', leave);
            root.removeEventListener('contextmenu', contextMenu);
            root.removeEventListener('dragstart', dragStart);
            root.removeEventListener('dragover', dragOver);
            root.removeEventListener('dragleave', dragLeave);
            root.removeEventListener('drop', drop);
            root.removeEventListener('input', zoomInput);
            root.removeEventListener('keydown', keyDown);
            root.removeEventListener('wheel', wheel);
            window.removeEventListener('pointermove', pointerMove);
            window.removeEventListener('pointerup', pointerUp);
            window.removeEventListener('pointercancel', pointerUp);
            root.removeEventListener('lostpointercapture', cancelGesture);
            window.removeEventListener('blur', cancelGesture);
            window.removeEventListener('dragend', clearMarker);
        };
    }

    window.VideoStudioTimeline = { MIN_ZOOM, MAX_ZOOM, BASE_PX_PER_SECOND, MAX_TRACKS_PER_KIND, MAX_CLIPS, newId, frameToClock, shortClock, makeClip, trackCapacity, clipFor, accepts, projectEnd, validTimeline, repairTransitions, setTransition, dragClip, displayOrder, render, setPlayhead, revealFrame, fitZoom, attach };
})();
