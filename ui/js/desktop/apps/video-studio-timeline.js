(function () {
    'use strict';

    const FPS = 30;
    const MIN_ZOOM = 0.025;
    const MAX_ZOOM = 2.8;
    const BASE_PX_PER_SECOND = 52;
    const MAX_FRAME = 600 * FPS;
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

    function render(root, options) {
        const s = options.state;
        const project = s.project;
        if (!project) { root.replaceChildren(); return; }
        const priorScroll = root.querySelector('.vs-timeline-scroll');
        const scrollLeft = priorScroll ? priorScroll.scrollLeft : 0;
        const scrollTop = priorScroll ? priorScroll.scrollTop : 0;
        const zoom = s.zoom || 1;
        const pps = BASE_PX_PER_SECOND * zoom;
        const totalFrames = Math.max(FPS * 15, ...project.tracks.flatMap(track => track.clips.map(c => c.start + c.duration)), s.frame + FPS * 2);
        const width = Math.ceil(totalFrames / FPS * pps) + 100;
        const ticks = [];
        const interval = zoom < 0.1 ? 60 : zoom < 0.25 ? 30 : zoom < 0.65 ? 10 : zoom < 1.1 ? 5 : 2;
        for (let sec = 0; sec <= totalFrames / FPS + 1; sec += interval) {
            ticks.push(`<span class="vs-tick" style="left:${Math.round(sec * pps)}px">${String(Math.floor(sec / 60)).padStart(2, '0')}:${String(sec % 60).padStart(2, '0')}</span>`);
        }
        const trackRows = project.tracks.map((track, trackIndex) => {
            const color = track.kind === 'audio' ? 'audio' : track.kind === 'overlay' ? 'overlay' : 'video';
            const clips = track.clips.slice().sort((a, b) => a.start - b.start).map(clip => {
                const asset = project.assets.find(a => a.id === clip.asset_id);
                const left = clip.start / FPS * pps;
                const clipWidth = Math.max(2, clip.duration / FPS * pps);
                const selected = s.selectedClipId === clip.id ? ' is-selected' : '';
                const title = asset ? asset.name : options.label('missingMedia');
                const wave = color === 'audio' ? '<span class="vs-audio-mark" aria-hidden="true">♫</span>' : '';
                const transition = clip.transition && clip.transition.duration ? `<span class="vs-transition" style="width:${Math.max(2, clip.transition.duration / FPS * pps)}px" title="${options.esc(options.label('transition'))}"></span>` : '';
                return `<div class="vs-clip vs-clip-${color}${selected}" data-clip-id="${options.esc(clip.id)}" data-track-id="${options.esc(track.id)}" title="${options.esc(title)}" style="left:${left}px;width:${clipWidth}px" tabindex="0" role="button" aria-label="${options.esc(title)} ${frameToClock(clip.start)}–${frameToClock(clip.start + clip.duration)}"><button class="vs-trim vs-trim-start" data-trim="start" aria-label="${options.esc(options.label('trimStart'))}"></button><span class="vs-clip-title">${options.esc(title)}</span>${wave}${transition}<button class="vs-trim vs-trim-end" data-trim="end" aria-label="${options.esc(options.label('trimEnd'))}"></button></div>`;
            }).join('');
            const locked = track.locked ? ' is-locked' : '';
            const muted = track.muted ? ' is-muted' : '';
            const hidden = track.hidden ? ' is-hidden' : '';
            return `<div class="vs-track-row${locked}${muted}${hidden}" data-track-row="${options.esc(track.id)}"><div class="vs-track-label" draggable="true" data-track-drag="${options.esc(track.id)}"><span class="vs-track-kind">${color === 'audio' ? '♫' : color === 'overlay' ? 'T' : '▧'}</span><strong>${options.esc(track.name)}</strong><div class="vs-track-actions"><button type="button" data-track-action="mute" title="${options.esc(options.label(track.muted ? 'unmute' : 'mute'))}" aria-label="${options.esc(options.label(track.muted ? 'unmute' : 'mute'))}">${track.muted ? '◌' : '◉'}</button><button type="button" data-track-action="hide" title="${options.esc(options.label(track.hidden ? 'show' : 'hide'))}" aria-label="${options.esc(options.label(track.hidden ? 'show' : 'hide'))}">${track.hidden ? '⊘' : '◉'}</button><button type="button" data-track-action="lock" title="${options.esc(options.label(track.locked ? 'unlock' : 'lock'))}" aria-label="${options.esc(options.label(track.locked ? 'unlock' : 'lock'))}">${track.locked ? '▣' : '▢'}</button></div></div><div class="vs-track-lane" data-track-id="${options.esc(track.id)}" style="width:${width}px">${clips}</div></div>`;
        }).join('');
        root.innerHTML = `<div class="vs-timeline-toolbar"><div class="vs-edit-tools"><button type="button" data-action="split" ${s.selectedClipId ? '' : 'disabled'}>${options.icon('scissors')}<span>${options.esc(options.label('split'))}</span></button><button type="button" data-action="duplicate" ${s.selectedClipId ? '' : 'disabled'}>${options.icon('copy')}<span>${options.esc(options.label('duplicate'))}</span></button><button type="button" data-action="delete" ${s.selectedClipId ? '' : 'disabled'}>${options.icon('trash')}<span>${options.esc(options.label('delete'))}</span></button></div><div class="vs-timeline-zoom"><button type="button" data-action="zoom-out" aria-label="${options.esc(options.label('zoomOut'))}">−</button><input type="range" min="45" max="280" value="${Math.round(zoom * 100)}" data-zoom aria-label="${options.esc(options.label('timelineZoom'))}"><button type="button" data-action="zoom-in" aria-label="${options.esc(options.label('zoomIn'))}">+</button><span>${Math.round(zoom * 100)}%</span></div></div><div class="vs-timeline-scroll"><div class="vs-ruler" style="width:${width}px"><div class="vs-ruler-label">${options.esc(options.label('timeline'))}</div><div class="vs-ruler-scale" data-ruler style="width:${width}px">${ticks.join('')}<i class="vs-playhead" style="left:${s.frame / FPS * pps}px"></i></div></div><div class="vs-track-list">${trackRows}</div></div><div class="vs-track-add">${['video', 'audio', 'overlay'].map(kind => `<button type="button" data-add-track="${kind}" ${trackCapacity(project, kind) ? '' : 'disabled'}>＋ ${options.esc(options.label(kind + 'Track'))}</button>`).join('')}<span>${options.esc(options.label('snapOn'))}</span></div>`;
        const scroll = root.querySelector('.vs-timeline-scroll');
        scroll.scrollLeft = scrollLeft; scroll.scrollTop = scrollTop;
        const slider = root.querySelector('[data-zoom]');
        slider.min = String(MIN_ZOOM * 100); slider.step = '0.5'; slider.value = String(zoom * 100);
        root.querySelectorAll('[data-trim]').forEach(button => { button.disabled = s.readonly || !!trackFor(project, button.closest('[data-track-id]').dataset.trackId)?.locked; });
        if (s.readonly) {
            root.querySelectorAll('[data-track-action], [data-add-track], [data-action="split"], [data-action="duplicate"], [data-action="delete"]').forEach(button => { button.disabled = true; });
            root.querySelectorAll('[draggable]').forEach(element => { element.draggable = false; });
        }
    }

    function dragClip(before, clipId, mode, delta, targetTrackId, snapFrames) {
        const project = clone(before), source = clipFor(project, clipId);
        if (!source || source.track.locked) return null;
        const clip = source.clip, asset = project.assets.find(item => item.id === clip.asset_id);
        if (mode === 'move') {
            const target = trackFor(project, targetTrackId) || source.track;
            if (target.locked || !accepts(target, asset)) return null;
            let start = clamp(clip.start + delta, 0, MAX_FRAME - clip.duration);
            const edges = target.clips.filter(item => item.id !== clip.id).flatMap(item => [item.start, item.start + item.duration]);
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
            const actual = clamp(delta, asset.kind === 'image' ? -clip.start : Math.max(-clip.start, -clip.offset), clip.duration - 1);
            clip.start += actual; clip.offset = asset.kind === 'image' ? 0 : clip.offset + actual; clip.duration -= actual;
        } else {
            const sourceLimit = asset.kind === 'image' ? MAX_FRAME : asset.duration_frames - clip.offset;
            clip.duration = clamp(clip.duration + delta, 1, Math.min(sourceLimit, MAX_FRAME - clip.start));
        }
        clip.fade_in = Math.min(clip.fade_in, clip.duration);
        clip.fade_out = Math.min(clip.fade_out, clip.duration);
        return validTimeline(project) ? project : null;
    }

    function attach(root, options) {
        let drag = null;
        let trackDrag = null;
        let zoom = options.state.zoom || 1;

        function setFrameFromPointer(event) {
            const ruler = event.target.closest('[data-ruler]');
            if (!ruler) return false;
            const rect = ruler.getBoundingClientRect();
            const pps = BASE_PX_PER_SECOND * (options.state.zoom || 1);
            const value = clamp(Math.round((event.clientX - rect.left) / pps * FPS), 0, MAX_FRAME);
            options.onFrame(value);
            return true;
        }

        function pointerMove(event) {
            if (!drag || options.state.readonly) return;
            if (drag.epoch !== options.state.projectEpoch || drag.projectId !== options.state.projectId) {
                drag = null; options.state.timelineDragging = false; return;
            }
            const pps = BASE_PX_PER_SECOND * (options.state.zoom || 1);
            const delta = Math.round((event.clientX - drag.x) / pps * FPS);
            const row = document.elementFromPoint(event.clientX, event.clientY)?.closest('[data-track-row]');
            const next = dragClip(drag.before, drag.clipId, drag.mode, delta, row?.dataset.trackRow, Math.max(1, Math.round(6 / pps * FPS)));
            drag.invalid = !next;
            if (!next) return;
            options.state.project = next;
            options.onPreviewChange();
            render(root, options);
        }

        function pointerUp(event) {
            if (drag) {
                const before = drag.before, invalid = drag.invalid;
                const current = drag.epoch === options.state.projectEpoch && drag.projectId === options.state.projectId;
                drag = null;
                options.state.timelineDragging = false;
                if (!current) return;
                if (event?.type === 'pointercancel' || options.state.readonly) options.state.project = before;
                else if (JSON.stringify(before) !== JSON.stringify(options.state.project)) options.onChange('timeline', before);
                if (invalid && options.onInvalid) options.onInvalid();
                if (options.onDragEnd) options.onDragEnd();
                render(root, options);
            }
            trackDrag = null;
        }

        function click(event) {
            const action = event.target.closest('[data-action]');
            if (action) {
                const id = options.state.selectedClipId;
                if (action.dataset.action === 'zoom-in' || action.dataset.action === 'zoom-out') {
                    zoom = clamp((options.state.zoom || 1) + (action.dataset.action === 'zoom-in' ? 0.15 : -0.15), MIN_ZOOM, MAX_ZOOM);
                    options.state.zoom = zoom;
                    render(root, options);
                } else if (id && !options.state.readonly) options.onAction(action.dataset.action, id);
                return;
            }
            const trackAction = event.target.closest('[data-track-action]');
            if (trackAction && !options.state.readonly) {
                const row = trackAction.closest('[data-track-row]');
                options.onTrackAction(row.dataset.trackRow, trackAction.dataset.trackAction);
                return;
            }
            const add = event.target.closest('[data-add-track]');
            if (add && !add.disabled && !options.state.readonly) { options.onAddTrack(add.dataset.addTrack); return; }
            const clipEl = event.target.closest('[data-clip-id]');
            if (clipEl) { options.onSelect(clipEl.dataset.clipId); render(root, options); }
            setFrameFromPointer(event);
        }

        function pointerDown(event) {
            const clipEl = event.target.closest('[data-clip-id]');
            if (!clipEl || event.button !== 0 || options.state.readonly) return;
            const target = clipFor(options.state.project, clipEl.dataset.clipId);
            if (!target || target.track.locked) return;
            const mode = event.target.closest('[data-trim]')?.dataset.trim;
            drag = { clipId: target.clip.id, mode: mode === 'start' ? 'start' : mode === 'end' ? 'end' : 'move', x: event.clientX, before: clone(options.state.project), epoch: options.state.projectEpoch, projectId: options.state.projectId };
            options.state.timelineDragging = true;
            clearTimeout(options.state.autosaveTimer);
            options.onSelect(target.clip.id);
            event.preventDefault();
        }

        function dragStart(event) {
            if (options.state.readonly) { event.preventDefault(); return; }
            const label = event.target.closest('[data-track-drag]');
            if (label) {
                trackDrag = label.dataset.trackDrag;
                event.dataTransfer.setData('application/x-video-studio-track', trackDrag);
                event.dataTransfer.effectAllowed = 'move';
                return;
            }
            const card = event.target.closest('[data-asset-id]');
            if (card) {
                event.dataTransfer.setData('application/x-video-studio-asset', card.dataset.assetId);
                event.dataTransfer.effectAllowed = 'copy';
            }
        }
        function dragOver(event) {
            if (!options.state.readonly && event.target.closest('.vs-track-lane')) { event.preventDefault(); event.dataTransfer.dropEffect = 'copy'; }
        }
        function drop(event) {
            const lane = event.target.closest('.vs-track-lane');
            if (!lane || options.state.readonly) return;
            event.preventDefault();
            const project = options.state.project;
            const track = trackFor(project, lane.dataset.trackId);
            if (!track || track.locked) return;
            const rect = lane.getBoundingClientRect();
            const at = clamp(Math.round((event.clientX - rect.left) / (BASE_PX_PER_SECOND * (options.state.zoom || 1)) * FPS), 0, MAX_FRAME - 1);
            const assetId = event.dataTransfer.getData('application/x-video-studio-asset');
            const reorderId = event.dataTransfer.getData('application/x-video-studio-track');
            if (assetId) options.onDropAsset(assetId, track.id, at);
            else if (reorderId && reorderId !== track.id) options.onReorderTrack(reorderId, track.id);
        }
        function zoomInput(event) {
            if (!event.target.matches('[data-zoom]')) return;
            options.state.zoom = clamp(Number(event.target.value) / 100, MIN_ZOOM, MAX_ZOOM);
            zoom = options.state.zoom;
            render(root, options);
        }
        function keyDown(event) {
            const clip = event.target.closest('[data-clip-id]');
            if (clip && event.key === 'Enter' && !event.target.closest('button')) {
                event.preventDefault(); event.stopPropagation();
                options.onSelect(clip.dataset.clipId);
                render(root, options);
            }
        }
        root.addEventListener('click', click);
        root.addEventListener('pointerdown', pointerDown);
        root.addEventListener('dragstart', dragStart);
        root.addEventListener('dragover', dragOver);
        root.addEventListener('drop', drop);
        root.addEventListener('input', zoomInput);
        root.addEventListener('keydown', keyDown);
        window.addEventListener('pointermove', pointerMove);
        window.addEventListener('pointerup', pointerUp);
        window.addEventListener('pointercancel', pointerUp);
        return function detach() {
            drag = null; options.state.timelineDragging = false;
            root.removeEventListener('click', click);
            root.removeEventListener('pointerdown', pointerDown);
            root.removeEventListener('dragstart', dragStart);
            root.removeEventListener('dragover', dragOver);
            root.removeEventListener('drop', drop);
            root.removeEventListener('input', zoomInput);
            root.removeEventListener('keydown', keyDown);
            window.removeEventListener('pointermove', pointerMove);
            window.removeEventListener('pointerup', pointerUp);
            window.removeEventListener('pointercancel', pointerUp);
        };
    }

    window.VideoStudioTimeline = { FPS, MIN_ZOOM, MAX_ZOOM, newId, frameToClock, makeClip, trackCapacity, clipFor, validTimeline, dragClip, render, attach };
})();
