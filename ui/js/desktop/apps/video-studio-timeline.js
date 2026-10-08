(function () {
    'use strict';

    const FPS = 30;
    const MIN_ZOOM = 0.025;
    const MAX_ZOOM = 2.8;
    const BASE_PX_PER_SECOND = 52;
    const MAX_FRAME = 600 * FPS;
    const TICK_STEPS = [1, 2, 5, 10, 15, 30, 60, 120, 300];
    const clamp = (v, lo, hi) => Math.max(lo, Math.min(hi, v));
    const clone = value => JSON.parse(JSON.stringify(value));

    function newId(prefix) {
        return prefix + '-' + (crypto.randomUUID ? crypto.randomUUID() : Math.random().toString(36).slice(2));
    }

    function frameToClock(frame) {
        const seconds = Math.floor(Math.max(0, frame) / FPS);
        const frames = Math.floor(Math.max(0, frame) % FPS);
        const minutes = Math.floor(seconds / 60);
        return String(minutes).padStart(2, '0') + ':' + String(seconds % 60).padStart(2, '0') + ':' + String(frames).padStart(2, '0');
    }

    function shortClock(frame) {
        const seconds = Math.floor(Math.max(0, frame) / FPS);
        return Math.floor(seconds / 60) + ':' + String(seconds % 60).padStart(2, '0');
    }

    function trackCapacity(project, kind) {
        return (project.tracks || []).filter(track => track.kind === kind).length < 4;
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

    function makeClip(asset, start, id) {
        start = clamp(Math.round(start || 0), 0, MAX_FRAME - 1);
        return {
            id: id || newId('clip'), asset_id: asset.id, start: Math.max(0, Math.round(start)), offset: 0,
            duration: Math.max(1, Math.min(Number(asset.duration_frames || FPS * 5), MAX_FRAME - start)),
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
        const assets = new Map(project.assets.map(asset => [asset.id, asset]));
        const ids = new Set(), tracks = new Set(), used = new Set();
        const counts = { video: 0, audio: 0, overlay: 0 };
        for (const track of project.tracks) {
            if (!Object.hasOwn(counts, track.kind) || ++counts[track.kind] > 4 || tracks.has(track.id) || !Array.isArray(track.clips)) return false;
            tracks.add(track.id);
            const clips = track.clips.slice().sort((a, b) => a.start - b.start);
            for (let i = 0; i < clips.length; i++) {
                const clip = clips[i], asset = assets.get(clip.asset_id), next = clips[i + 1];
                if (!accepts(track, asset) || ids.has(clip.id)) return false;
                ids.add(clip.id); used.add(clip.asset_id);
                if (ids.size > 256 || used.size > 32) return false;
                if (![clip.start, clip.offset, clip.duration, clip.fade_in, clip.fade_out].every(Number.isSafeInteger) ||
                    clip.start < 0 || clip.duration <= 0 || clip.duration > MAX_FRAME - clip.start || clip.offset < 0 ||
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
        const s = options.state, esc = options.esc;
        const asset = project.assets.find(a => a.id === clip.asset_id);
        const left = clip.start / FPS * pps;
        const width = Math.max(2, clip.duration / FPS * pps);
        const selected = s.selectedClipId === clip.id;
        const kind = clip.text ? 'text' : track.kind === 'audio' ? 'audio' : track.kind === 'overlay' ? 'overlay' : 'video';
        const name = clip.text ? clip.text.split(/\r?\n/)[0] : asset ? (options.assetName ? options.assetName(asset) : asset.name) : label(options, 'missingMedia', 'Missing media');
        const media = options.clipMedia && asset ? options.clipMedia(asset, clip, track, pps / FPS, width) : '';
        const fadeIn = clip.fade_in ? `<span class="vs-fade vs-fade-in" style="width:${Math.max(2, clip.fade_in / FPS * pps)}px"></span>` : '';
        const fadeOut = clip.fade_out ? `<span class="vs-fade vs-fade-out" style="width:${Math.max(2, clip.fade_out / FPS * pps)}px"></span>` : '';
        const transition = clip.transition && clip.transition.duration ? `<span class="vs-transition" style="width:${Math.max(4, clip.transition.duration / FPS * pps)}px" title="${esc(label(options, clip.transition.type, clip.transition.type))}">${icon(options, 'transition', 12)}</span>` : '';
        const kindIcon = kind === 'text' ? 'text' : kind === 'audio' ? 'audio' : kind === 'overlay' ? 'image' : 'video';
        const range = `${shortClock(clip.start)}–${shortClock(clip.start + clip.duration)}`;
        return `<div class="vs-clip vs-clip-${kind}${selected ? ' is-selected' : ''}${width < 46 ? ' is-narrow' : ''}" data-clip-id="${esc(clip.id)}" data-track-id="${esc(track.id)}" title="${esc(name)} · ${esc(range)}" style="left:${left}px;width:${width}px" tabindex="0" role="button" aria-pressed="${selected}" aria-label="${esc(name)} ${esc(range)}"><span class="vs-clip-media">${media}</span>${fadeIn}${fadeOut}<span class="vs-clip-label">${icon(options, kindIcon, 12)}<span class="vs-clip-name">${esc(name)}</span><span class="vs-clip-dur">${esc(shortClock(clip.duration))}</span></span>${transition}<button type="button" class="vs-trim vs-trim-start" data-trim="start" tabindex="-1" aria-label="${esc(label(options, 'trimStart', 'Trim start'))}"></button><button type="button" class="vs-trim vs-trim-end" data-trim="end" tabindex="-1" aria-label="${esc(label(options, 'trimEnd', 'Trim end'))}"></button></div>`;
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
        const totalFrames = Math.max(FPS * 20, projectEnd(project) + FPS * 6, s.frame + FPS * 4);
        const width = Math.ceil(totalFrames / FPS * pps) + 120;
        const interval = tickInterval(pps);
        const ticks = [];
        for (let sec = 0; sec <= totalFrames / FPS + interval; sec += interval) {
            ticks.push(`<span class="vs-tick" style="left:${Math.round(sec * pps)}px">${Math.floor(sec / 60)}:${String(sec % 60).padStart(2, '0')}</span>`);
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
        const addTrack = ['video', 'audio', 'overlay'].map(kind => `<button type="button" role="menuitem" data-add-track="${kind}" ${trackCapacity(project, kind) && !s.readonly ? '' : 'disabled'}>${icon(options, kind === 'audio' ? 'audio' : kind === 'overlay' ? 'layers' : 'video', 15)}${esc(label(options, kind + 'Track', kind))}</button>`).join('');
        root.innerHTML = `<div class="vs-timeline-toolbar"><div class="vs-edit-tools"><button type="button" data-action="split" ${disabledEdit} title="${esc(label(options, 'splitHintShort', 'Split at the playhead (S)'))}">${icon(options, 'scissors')}<span>${esc(label(options, 'split', 'Split'))}</span></button><button type="button" data-action="duplicate" ${disabledEdit}>${icon(options, 'copy')}<span>${esc(label(options, 'duplicate', 'Duplicate'))}</span></button><button type="button" data-action="delete" ${disabledEdit} title="${esc(label(options, 'deleteHint', 'Delete (Del)'))}">${icon(options, 'trash')}<span>${esc(label(options, 'delete', 'Delete'))}</span></button></div><div class="vs-timeline-tools"><div class="vs-menu-anchor"><button type="button" data-action="track-menu" aria-haspopup="menu" aria-expanded="${menuOpen}" ${s.readonly ? 'disabled' : ''}>${icon(options, 'plus')}<span>${esc(label(options, 'addTrack', 'Track'))}</span></button><div class="vs-popover vs-track-menu" data-track-menu role="menu" ${menuOpen ? '' : 'hidden'}>${addTrack}</div></div><button type="button" class="vs-toggle" data-action="snap" aria-pressed="${snap}" title="${esc(label(options, 'snapHint', 'Snap clips to edges and the playhead'))}">${icon(options, 'magnet')}<span>${esc(label(options, 'snap', 'Snap'))}</span></button><div class="vs-timeline-zoom"><button type="button" data-action="zoom-out" aria-label="${esc(label(options, 'zoomOut', 'Zoom out'))}" title="${esc(label(options, 'zoomOut', 'Zoom out'))}">${icon(options, 'minus')}</button><input type="range" data-zoom aria-label="${esc(label(options, 'timelineZoom', 'Timeline zoom'))}"><button type="button" data-action="zoom-in" aria-label="${esc(label(options, 'zoomIn', 'Zoom in'))}" title="${esc(label(options, 'zoomIn', 'Zoom in'))}">${icon(options, 'plus')}</button><button type="button" data-action="zoom-fit" title="${esc(label(options, 'zoomFit', 'Show whole project'))}">${icon(options, 'fit')}<span>${esc(label(options, 'zoomFitShort', 'Fit'))}</span></button></div></div></div><div class="vs-timeline-scroll"><div class="vs-tl-content" style="width:calc(var(--vs-head-w) + ${width}px)"><div class="vs-ruler"><div class="vs-ruler-label">${icon(options, 'clock', 13)}<span data-timeline-time>${esc(shortClock(s.frame))}</span></div><div class="vs-ruler-scale" data-ruler style="width:${width}px;--vs-tick:${interval * pps}px">${ticks.join('')}</div></div><div class="vs-track-list">${rows}${empty}</div><div class="vs-playhead" style="left:calc(var(--vs-head-w) + ${s.frame / FPS * pps}px)"><span class="vs-playhead-knob" data-playhead></span></div></div></div>`;
        const scroll = root.querySelector('.vs-timeline-scroll');
        scroll.scrollLeft = scrollLeft; scroll.scrollTop = scrollTop;
        const slider = root.querySelector('[data-zoom]');
        slider.min = String(MIN_ZOOM * 100); slider.max = String(MAX_ZOOM * 100); slider.step = '0.5'; slider.value = String(zoom * 100);
        root.querySelectorAll('[data-trim]').forEach(button => { button.disabled = s.readonly || !!trackFor(project, button.closest('[data-track-id]').dataset.trackId)?.locked; });
        if (s.readonly) {
            root.querySelectorAll('[data-track-action], [data-add-track], [data-action="split"], [data-action="duplicate"], [data-action="delete"]').forEach(button => { button.disabled = true; });
            root.querySelectorAll('[draggable]').forEach(element => { element.draggable = false; });
        }
        if (focused) root.querySelector(focused)?.focus({ preventScroll: true });
    }

    function setPlayhead(root, state, follow) {
        const head = root.querySelector('.vs-playhead');
        if (!head) return;
        const pps = BASE_PX_PER_SECOND * (state.zoom || 1);
        const x = state.frame / FPS * pps;
        head.style.left = `calc(var(--vs-head-w) + ${x}px)`;
        const time = root.querySelector('[data-timeline-time]');
        if (time) time.textContent = shortClock(state.frame);
        if (!follow) return;
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        if (!scroll || !labelEl) return;
        const visible = scroll.clientWidth - labelEl.offsetWidth;
        if (x < scroll.scrollLeft || x > scroll.scrollLeft + visible - 32) scroll.scrollLeft = Math.max(0, x - 48);
    }

    function revealFrame(root, state, frame) {
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        if (!scroll || !labelEl) return;
        const x = frame / FPS * BASE_PX_PER_SECOND * (state.zoom || 1);
        const visible = scroll.clientWidth - labelEl.offsetWidth;
        if (x < scroll.scrollLeft || x > scroll.scrollLeft + visible - 32) scroll.scrollLeft = Math.max(0, x - 48);
    }

    function fitZoom(root, project) {
        const scroll = root.querySelector('.vs-timeline-scroll'), labelEl = root.querySelector('.vs-ruler-label');
        const visible = Math.max(120, (scroll ? scroll.clientWidth : 800) - (labelEl ? labelEl.offsetWidth : 160) - 40);
        const seconds = Math.max(5, projectEnd(project) / FPS);
        return clamp(visible / (seconds * BASE_PX_PER_SECOND), MIN_ZOOM, MAX_ZOOM);
    }

    function dragClip(before, clipId, mode, delta, targetTrackId, snapFrames, extraPoints) {
        const project = clone(before), source = clipFor(project, clipId);
        if (!source || source.track.locked) return null;
        const clip = source.clip, asset = project.assets.find(item => item.id === clip.asset_id);
        const extra = Array.isArray(extraPoints) ? extraPoints.filter(Number.isFinite) : [];
        if (mode === 'move') {
            const target = trackFor(project, targetTrackId) || source.track;
            if (target.locked || !accepts(target, asset)) return null;
            let start = clamp(clip.start + delta, 0, MAX_FRAME - clip.duration);
            const edges = target.clips.filter(item => item.id !== clip.id).flatMap(item => [item.start, item.start + item.duration]).concat(extra);
            const positions = [0, ...edges, ...edges.map(edge => edge - clip.duration)];
            const nearby = positions.filter(point => point >= 0 && point <= MAX_FRAME - clip.duration && Math.abs(point - start) <= snapFrames);
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
            const sourceLimit = asset.kind === 'image' ? MAX_FRAME : asset.duration_frames - clip.offset;
            let duration = clip.duration + delta;
            const snapped = extra.find(point => Math.abs(point - (clip.start + duration)) <= snapFrames);
            if (snapped != null) duration = snapped - clip.start;
            clip.duration = clamp(duration, 1, Math.min(sourceLimit, MAX_FRAME - clip.start));
        }
        clip.fade_in = Math.min(clip.fade_in, clip.duration);
        clip.fade_out = Math.min(clip.fade_out, clip.duration);
        repairTransitions(project);
        return validTimeline(project) ? project : null;
    }

    function attach(root, options) {
        let drag = null, scrub = null, dropMarker = null;
        const s = () => options.state;

        function pps() { return BASE_PX_PER_SECOND * (s().zoom || 1); }
        function frameAt(element, clientX) {
            const rect = element.getBoundingClientRect();
            return clamp(Math.round((clientX - rect.left) / pps() * FPS), 0, MAX_FRAME);
        }
        function snapFrames() { return s().snap === false ? 0 : Math.max(1, Math.round(8 / pps() * FPS)); }
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

        function pointerMove(event) {
            if (scrub) { options.onFrame(frameAt(scrub.ruler, event.clientX)); return; }
            if (!drag || s().readonly) return;
            if (drag.epoch !== s().projectEpoch || drag.projectId !== s().projectId) {
                drag = null; s().timelineDragging = false; return;
            }
            const delta = Math.round((event.clientX - drag.x) / pps() * FPS);
            const row = document.elementFromPoint(event.clientX, event.clientY)?.closest('[data-track-row]');
            const next = dragClip(drag.before, drag.clipId, drag.mode, delta, row?.dataset.trackRow, snapFrames(), [s().frame]);
            drag.invalid = !next;
            if (!next) return;
            s().project = next;
            options.onPreviewChange();
            render(root, options);
        }

        // Gestures capture the pointer on the timeline container (clips re-render while they move), so the
        // release arrives even outside the window; a lost capture or a window switch cancels the gesture.
        function capture(event) { try { root.setPointerCapture(event.pointerId); } catch (_) { /* synthetic or finished pointer */ } }
        function cancelGesture() { if (drag || scrub) pointerUp({ type: 'pointercancel' }); }
        function pointerUp(event) {
            if (scrub) { scrub = null; return; }
            if (drag) {
                const before = drag.before, invalid = drag.invalid;
                const current = drag.epoch === s().projectEpoch && drag.projectId === s().projectId;
                drag = null;
                s().timelineDragging = false;
                if (!current) return;
                if (event?.type === 'pointercancel' || s().readonly) s().project = before;
                else if (JSON.stringify(before) !== JSON.stringify(s().project)) options.onChange('timeline', before);
                if (invalid && options.onInvalid) options.onInvalid();
                if (options.onDragEnd) options.onDragEnd();
                render(root, options);
            }
        }

        function click(event) {
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
                } else if (id && !s().readonly) options.onAction(name, id);
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
            if (clipEl) { options.onSelect(clipEl.dataset.clipId); render(root, options); return; }
            const lane = event.target.closest('.vs-track-lane');
            if (lane) {
                options.onFrame(frameAt(lane, event.clientX));
                if (s().selectedClipId) { options.onSelect(''); render(root, options); }
            }
        }

        function pointerDown(event) {
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
            if (!clipEl || s().readonly) return;
            const target = clipFor(s().project, clipEl.dataset.clipId);
            if (!target || target.track.locked) return;
            const mode = event.target.closest('[data-trim]')?.dataset.trim;
            drag = { clipId: target.clip.id, mode: mode === 'start' ? 'start' : mode === 'end' ? 'end' : 'move', x: event.clientX, before: clone(s().project), epoch: s().projectEpoch, projectId: s().projectId };
            s().timelineDragging = true;
            capture(event);
            clearTimeout(s().autosaveTimer);
            options.onSelect(target.clip.id);
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
            dropMarker.style.left = Math.round(frameAt(lane, event.clientX) / FPS * pps()) + 'px';
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
            const at = clamp(frameAt(lane, event.clientX), 0, MAX_FRAME - 1);
            const assetId = event.dataTransfer.getData('application/x-video-studio-asset');
            const reorderId = event.dataTransfer.getData('application/x-video-studio-track');
            if (assetId) options.onDropAsset(assetId, track.id, at);
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
            if (event.key === 'Escape' && root.querySelector('[data-track-menu]:not([hidden])')) { closeMenu(); root.querySelector('[data-action="track-menu"]')?.focus(); event.preventDefault(); event.stopPropagation(); return; }
            const clip = event.target.closest('[data-clip-id]');
            if (clip && (event.key === 'Enter' || event.key === ' ') && !event.target.closest('button')) {
                event.preventDefault(); event.stopPropagation();
                options.onSelect(clip.dataset.clipId);
                render(root, options);
                root.querySelector(`[data-clip-id="${CSS.escape(clip.dataset.clipId)}"]`)?.focus();
            }
        }
        root.addEventListener('click', click);
        root.addEventListener('pointerdown', pointerDown);
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
            drag = null; scrub = null; clearMarker(); s().timelineDragging = false;
            root.removeEventListener('click', click);
            root.removeEventListener('pointerdown', pointerDown);
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

    window.VideoStudioTimeline = { FPS, MIN_ZOOM, MAX_ZOOM, BASE_PX_PER_SECOND, newId, frameToClock, shortClock, makeClip, trackCapacity, clipFor, accepts, projectEnd, validTimeline, repairTransitions, setTransition, dragClip, displayOrder, render, setPlayhead, revealFrame, fitZoom, attach };
})();
