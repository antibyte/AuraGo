(function () {
    'use strict';

    // Keyframes in the editor (spec §8.2, plan index §4.18). Task 12: the stopwatch, diamond and previous/next buttons of
    // each keyframable value in the inspector's Picture section (markup in video-studio-inspector.js), their sync while
    // the playhead moves, and the keyframe shortcuts. Task 13 adds the timeline keyframe strip, Task 14 the speed lane and
    // the ramp section. Every project change runs a VideoStudioKeyframeEdit operation inside P.applyEdit: one undo step,
    // and a refusal explains itself in a toast.
    const P = window.VideoStudioParts;
    const T = (...args) => P.T(...args);
    const K = () => window.VideoStudioKeyframeEdit;
    const Fx = () => window.VideoStudioEffects;
    const icon = (...args) => P.icon(...args);
    const showNotice = (...args) => P.showNotice(...args);
    const tr = (...args) => P.tr(...args);
    const BOX = ['x', 'y', 'width', 'height'];
    const outsideText = s => tr(s, 'keyframeOutside', 'Move the playhead into the clip to set keyframes.');

    // Through the keyframe path store (2c): base values and crop sides in clip.keyframes, effect params in their entry.
    function isAnimated(clip, path) { return !!clip && K().keysOf(clip, path).length > 0; }
    function animatedPaths(clip) { return Fx().animatablePaths(clip).filter(path => isAnimated(clip, path)); }
    function inside(clip, frame) { return frame >= clip.start && frame < clip.start + clip.duration; }
    // The selected clip when it has a picture: keyframes live on visual tracks only.
    function visualSelection(s) {
        const found = s.project && s.selectedClipId ? T().clipFor(s.project, s.selectedClipId) : null;
        return found && found.track.kind !== 'audio' ? found : null;
    }

    // One value's keyframe state at a timeline frame: animated, a key on the playhead, the whole timeline frames of the
    // previous and next keys inside the clip (null when there is none), and the value shown.
    function controlState(project, clip, path, frame) {
        const animated = isAnimated(clip, path);
        return {
            animated,
            onKey: animated && !!K().atPlayhead(clip, path, frame),
            prev: animated ? K().neighbor(clip, [path], frame, -1) : null,
            next: animated ? K().neighbor(clip, [path], frame, 1) : null,
            value: Fx().valueAt(clip, path, frame, project)
        };
    }
    // What the inspector shows for the selected visual clip at the playhead, or null: the resolved state, one controlState
    // per animatable path (base values, crop sides and the clip's effect params), whether the playhead is inside the clip (keys are set only there), whether the editor may
    // change the clip, and whether a box value is animated (fine tune opens by itself then).
    function keyframeView(s) {
        const found = visualSelection(s);
        if (!found) return null;
        const clip = found.clip, frame = Math.round(Number(s.frame) || 0), controls = {};
        Fx().animatablePaths(clip).forEach(path => { controls[path] = controlState(s.project, clip, path, frame); });
        return {
            frame, inside: inside(clip, frame), locked: !!s.readonly || !!found.track.locked, controls,
            state: Fx().clipState(clip, frame, s.project), boxAnimated: BOX.some(path => controls[path] && controls[path].animated)
        };
    }

    // The stopwatch (spec §8.2): on sets the first keyframe at the playhead; off ends the animation and keeps the value at
    // the playhead in the field, clamped to a valid box.
    function applyAnimated(s, path, on) {
        const found = visualSelection(s);
        if (!found) return false;
        return P.applyEdit(s, on ? 'animate' : 'stop animation', project => K().setAnimated(project, found.clip.id, path, !!on, s.frame));
    }
    // An edit re-rendered the inspector: keyboard focus returns to the same keyframe button.
    function refocus(s, path, action) {
        const button = s.q(`[data-inspector] [data-kf-path="${CSS.escape(path)}"][data-kf-action="${CSS.escape(action)}"]`);
        if (button && !button.disabled) button.focus({ preventScroll: true });
    }
    // Inspector keyframe buttons ([data-kf-path][data-kf-action]) and the Animation section's ramp presets and
    // "Edit in the timeline"; true when the click was one of them.
    function handleKeyframeClick(s, event) {
        const preset = event.target.closest('[data-inspector] [data-ramp-preset]');
        if (preset) { if (!preset.disabled) applyRampPreset(s, preset.dataset.rampPreset); return true; }
        if (event.target.closest('[data-inspector] [data-action="ramp-edit"]')) { openSpeedLane(s); return true; }
        const button = event.target.closest('[data-kf-path][data-kf-action]');
        if (!button || !s.project) return false;
        // data-kf-path may hold several paths joined by spaces (a colour wheel's x and y): they act together.
        const found = visualSelection(s), path = button.dataset.kfPath, action = button.dataset.kfAction, paths = path.split(' ');
        if (!found || button.disabled) return true;
        if (action === 'prev' || action === 'next') {
            const frame = K().neighbor(found.clip, paths, s.frame, action === 'prev' ? -1 : 1);
            if (frame != null) P.seekTo(s, frame);
            // At the first or last key the button turns off; the keyboard moves to the other one.
            if (button.disabled && button.parentElement) button.parentElement.querySelector(`[data-kf-action="${action === 'prev' ? 'next' : 'prev'}"]:not(:disabled)`)?.focus({ preventScroll: true });
            return true;
        }
        if (action === 'stopwatch') applyAnimatedAll(s, found.clip.id, paths, !paths.some(item => isAnimated(found.clip, item)));
        else if (action === 'key') toggleKeysAll(s, found.clip, paths);
        else return false;
        refocus(s, path, action);
        return true;
    }
    // One stopwatch for several paths: they all start or all stop in one undo step (one path: applyAnimated).
    function applyAnimatedAll(s, clipId, paths, on) {
        if (paths.length === 1) return applyAnimated(s, paths[0], on);
        return P.applyEdit(s, on ? 'animate' : 'stop animation', project => {
            for (const path of paths) {
                if (isAnimated(T().clipFor(project, clipId).clip, path) === on) continue;
                const result = K().setAnimated(project, clipId, path, on, s.frame);
                if (result !== true) return result;
            }
            return true;
        });
    }
    // One diamond for several paths. When every animated one has a key at the playhead, those keys are removed;
    // otherwise every path without one gets one (a static path starts its animation). One undo step.
    function toggleKeysAll(s, clip, paths) {
        const animated = paths.filter(path => isAnimated(clip, path)), remove = animated.length > 0 && animated.every(path => K().atPlayhead(clip, path, s.frame));
        return P.applyEdit(s, 'keyframe', project => {
            for (const path of remove ? animated : paths) {
                if (!remove && K().atPlayhead(T().clipFor(project, clip.id).clip, path, s.frame)) continue;
                const result = K().toggleKeyframe(project, clip.id, path, s.frame);
                if (result !== true) return result;
            }
            return true;
        });
    }

    // One keyframe button follows the playhead: enabled state, aria-pressed, label and the diamond's icon.
    function syncButton(s, view, button) {
        const path = button.dataset.kfPath, action = button.dataset.kfAction, c = P.I().controlFor(view, path);
        if (!c) return;
        const name = (button.parentElement && button.parentElement.getAttribute('aria-label')) || path;
        const text = (key, fallback) => tr(s, key, fallback).split('{{name}}').join(name);
        const label = value => { button.setAttribute('aria-label', value); button.title = value; };
        button.disabled = view.locked || !view.inside || (action === 'prev' && c.prev == null) || (action === 'next' && c.next == null);
        if (action === 'stopwatch') {
            button.setAttribute('aria-pressed', String(c.animated));
            label(c.animated ? text('keyframeStopwatchOff', 'Stop animating {{name}} (keeps the value at the playhead)') : text('keyframeStopwatch', 'Animate {{name}}'));
        } else if (action === 'key') {
            button.setAttribute('aria-pressed', String(c.onKey));
            label(c.onKey ? text('keyframeRemove', 'Remove the keyframe of {{name}} at the playhead') : text('keyframeAdd', 'Add a keyframe for {{name}} at the playhead'));
            if (button.dataset.kfOn !== String(c.onKey)) { button.dataset.kfOn = String(c.onKey); button.innerHTML = icon(c.onKey ? 'diamondFilled' : 'diamond', 14); }
        }
    }
    // Keeps the Picture section on the playhead while it moves (paused; called from the frame and play-state callbacks in
    // video-studio.js): the values, the keyframe buttons, and the inputs of animated values, which cannot change while
    // the playhead is outside the clip. An input under the pointer or caret keeps what the user is doing.
    function syncKeyframeControls(s) {
        const host = s.q && s.q('[data-inspector]');
        const view = host && host.querySelector('[data-kf-path]') ? keyframeView(s) : null;
        if (!view) return;
        const outside = outsideText(s), active = document.activeElement, st = view.state, round6 = value => Math.round(value * 1e6) / 1e6;
        const frozen = paths => !view.inside && paths.some(path => view.controls[path] && view.controls[path].animated);
        host.querySelectorAll('.vs-kf-tools').forEach(group => { group.title = view.inside ? '' : outside; });
        host.querySelectorAll('[data-kf-path][data-kf-action]').forEach(button => syncButton(s, view, button));
        const put = (field, value, text, paths) => {
            const input = host.querySelector(`[data-field="${field}"]`);
            if (!input) return;
            input.disabled = view.locked || frozen(paths);
            input.title = frozen(paths) ? outside : '';
            if (input._vsBefore || (input === active && input.type !== 'range')) return;
            input.value = String(value);
            if (text) P.updateOutput(input, text);
        };
        put('size', Math.round(st.width * 100), P.percentText(st.width * 100), BOX);
        put('rotation', Math.round(st.rotation), Math.round(st.rotation) + '°', ['rotation']);
        put('opacity', Math.round(st.opacity * 100), P.percentText(st.opacity * 100), ['opacity']);
        BOX.forEach(path => put(path, round6(st[path] * 100), '', [path]));
        host.querySelectorAll('[data-position],[data-action="fill-frame"]').forEach(button => { button.disabled = view.locked || frozen(BOX); });
        if (P.syncEffectControls) P.syncEffectControls(s, view, host);
    }

    // Alt+K: every animated value of the selected clip gets a key at the playhead; when all of them already have one, they
    // are removed. One undo step.
    function toggleAllKeys(s) {
        const found = visualSelection(s);
        if (!found) return false;
        const clip = found.clip, frame = s.frame, paths = animatedPaths(clip);
        if (!paths.length) { showNotice(s, 'keyframeNeedsParam', 'Turn on the stopwatch of a value first to set keyframes.', false); return false; }
        if (!inside(clip, frame)) { showNotice(s, 'keyframeOutside', 'Move the playhead into the clip to set keyframes.', false); return false; }
        const allOn = paths.every(path => K().atPlayhead(clip, path, frame));
        return P.applyEdit(s, 'keyframe', project => {
            for (const path of paths) {
                if (!allOn && K().atPlayhead(T().clipFor(project, clip.id).clip, path, frame)) continue;
                const result = K().toggleKeyframe(project, clip.id, path, frame);
                if (result !== true) return result;
            }
            return true;
        });
    }
    // Alt+ArrowUp / Alt+ArrowDown: the previous / next key of any animated value of the selected clip, inside the clip.
    function jumpKey(s, direction) {
        const found = visualSelection(s);
        const frame = found ? K().neighbor(found.clip, animatedPaths(found.clip), s.frame, direction) : null;
        if (frame != null) P.seekTo(s, frame);
        return frame != null;
    }
    // The keyframe shortcuts (index §4.18), matched by event.code because macOS Option changes event.key; brackets are
    // avoided (AltGr on German layouts). They collide with no edit, navigation or source key
    // (TestVideoStudioKeyframeShortcuts); the source monitor keeps its own keys.
    function keyframeCommand(s, event) {
        if (s.stageMode === 'source' || !event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return null;
        if (event.code === 'KeyK') return { run: () => toggleAllKeys(s), repeat: false };
        if (event.code === 'ArrowUp' || event.code === 'ArrowDown') return { run: () => jumpKey(s, event.code === 'ArrowUp' ? -1 : 1) };
        return null;
    }
    function isKeyframeShortcut(s, event) { return !!keyframeCommand(s, event); }
    // Returns true when the key was a keyframe shortcut and has been handled. Holding Alt+K does not toggle repeatedly.
    function handleKeyframeKeys(s, event) {
        const command = keyframeCommand(s, event);
        if (!command) return false;
        event.preventDefault();
        if (command.repeat === false && event.repeat) return true;
        command.run();
        return true;
    }

    // ---- Timeline keyframe strip (2b, index §4.18). keyframeStrip draws it after the selected clip's track row
    // (timelineOptions.stripAfter) while that visual clip has keyframes or a speed ramp, or copied keyframes wait in
    // s.kfBoard (so they can be pasted onto a clip without keys). Its rows are .vs-kf-row: never data-track-row or
    // .vs-track-lane, so the timeline's drags, rubber band, drops, blade and menus pass them by. View state:
    // s.kfStrip = {clipId, expanded, selected}; selection ids are "<path>:<f>" with f in source frames.

    // Strip geometry for one clip, DOM-free and Node-tested.
    // - Every keyframe sits at timelineAt(clip, f) in lane pixels, through the clip's speed or ramp, so a diamond marks
    //   where the clip shows its source frame.
    // - Keys outside the clip's frames stay in the model with outside: true. They are kept, like trimmed media, but not
    //   drawn.
    // - The summary row has one diamond per source frame. It lists the paths keyed there, with e 'mixed' when their
    //   easings differ.
    // - Expanded adds one row per animated path, in VideoStudioEffects.animatablePaths order (base values, crop sides, then each effect entry's params).
    // view is {expanded} or null.
    function stripModel(project, clip, pps, view) {
        const Fx = window.VideoStudioEffects, fps = window.VideoStudioTime.rate(project);
        const first = clip.start, last = clip.start + clip.duration - 1, keysOf = path => window.VideoStudioKeyframeEdit.keysOf(clip, path);
        const place = f => {
            const frame = Fx.timelineAt(clip, f);
            return { frame, x: frame / fps * pps, outside: !(frame >= first - 1e-9 && frame <= last + 1e-9) };
        };
        const groups = new Map();
        const rows = Fx.animatablePaths(clip).filter(path => keysOf(path).length).map(path => ({
            id: path,
            keys: keysOf(path).map(key => {
                const at = place(key.f);
                if (!groups.has(key.f)) groups.set(key.f, Object.assign({ f: key.f, paths: [], easings: [] }, at));
                groups.get(key.f).paths.push(path);
                groups.get(key.f).easings.push(key.e);
                return Object.assign({ path, f: key.f, v: key.v, e: key.e }, at);
            })
        }));
        const summary = Array.from(groups.values()).sort((a, b) => a.f - b.f).map(group => ({
            f: group.f, frame: group.frame, x: group.x, outside: group.outside, paths: group.paths,
            e: group.easings.every(e => e === group.easings[0]) ? group.easings[0] : 'mixed'
        }));
        const expanded = !!(view && view.expanded);
        return { clipId: clip.id, left: clip.start / fps * pps, width: Math.max(2, clip.duration / fps * pps), expanded, rows: [{ id: 'summary', keys: summary }].concat(expanded ? rows : [], expanded && Fx.hasRamp(clip) ? [rampModelRow(clip, fps, pps)] : []) };
    }
    // One clip at a time: switching clips clears the selection and keeps the expanded state.
    function stripView(s, clipId) {
        const view = s.kfStrip || (s.kfStrip = { clipId: '', expanded: false, selected: [] });
        if (view.clipId !== clipId) { view.clipId = clipId; view.selected = []; }
        return view;
    }
    function stripSplit(id) { const at = id.lastIndexOf(':'); return { path: id.slice(0, at), f: Number(id.slice(at + 1)) }; }
    // The per-path ids a diamond stands for: a summary diamond holds every path keyed at its source frame.
    function stripIds(clip, path, f) {
        if (path !== 'summary') return [path + ':' + f];
        return K().keyedPaths(clip).filter(name => K().keysOf(clip, name).some(key => key.f === f)).map(name => name + ':' + f);
    }
    function stripKeys(s) { return ((s.kfStrip && s.kfStrip.selected) || []).map(stripSplit); }
    // After a move, unselected keys keep their frames and moved keys keep their order on each path. The new selection is
    // therefore read from the clip, whatever rounding moveKeys applied. Returns a map from old id to new id.
    function stripMoved(before, after, ids) {
        const map = new Map(), byPath = new Map();
        ids.forEach(id => { const { path, f } = stripSplit(id); if (!byPath.has(path)) byPath.set(path, []); byPath.get(path).push(f); });
        byPath.forEach((frames, path) => {
            const moving = new Set(frames);
            const kept = new Set(K().keysOf(before, path).map(key => key.f).filter(f => !moving.has(f)));
            const landed = K().keysOf(after, path).map(key => key.f).filter(f => !kept.has(f));
            frames.slice().sort((a, b) => a - b).forEach((f, index) => { if (landed[index] != null) map.set(path + ':' + f, path + ':' + landed[index]); });
        });
        return map;
    }
    // The id the pressed diamond has after a move (a summary diamond follows its keys).
    function stripAnchor(map, id) {
        if (!id) return '';
        const { path, f } = stripSplit(id);
        if (path !== 'summary') return map.get(id) || id;
        for (const [from, to] of map) if (stripSplit(from).f === f) return 'summary:' + stripSplit(to).f;
        return id;
    }
    // A re-render replaces the diamonds, and the timeline's focusSelector does not know them, so the strip restores
    // focus itself. The fallback order is the diamond, then the chevron, then the selected clip.
    function stripFocus(s, id) {
        const root = s.q('[data-timeline]');
        if (!root) return;
        const target = (id && root.querySelector(`[data-kf-strip] [data-kf-key="${CSS.escape(id)}"]`)) || root.querySelector('[data-kf-strip] [data-kf-action="expand"]') || (s.selectedClipId && root.querySelector(`[data-clip-id="${CSS.escape(s.selectedClipId)}"]`));
        if (target) target.focus({ preventScroll: true });
    }

    // ---- Speed lane (2b). Speed is drawn on a log scale, 0.25x at the bottom and 16x at the top. The height matches the
    // CSS of .vs-speed-lane.
    const RAMP_LANE = Object.freeze({ height: 48, pad: 6, low: -2, high: 4 });
    function rampY(speed) {
        const v = (Math.log2(Math.min(16, Math.max(0.25, speed))) - RAMP_LANE.low) / (RAMP_LANE.high - RAMP_LANE.low);
        return RAMP_LANE.pad + (1 - v) * (RAMP_LANE.height - 2 * RAMP_LANE.pad);
    }
    // Lane y to speed, clamped to 0.25..16, snapped to 1x within 5 % (in log2) and rounded to 3 decimals.
    function rampSpeedAtY(y) {
        const v = 1 - (y - RAMP_LANE.pad) / (RAMP_LANE.height - 2 * RAMP_LANE.pad);
        const speed = Math.min(16, Math.max(0.25, 2 ** (RAMP_LANE.low + v * (RAMP_LANE.high - RAMP_LANE.low))));
        return Math.abs(Math.log2(speed)) < 0.05 ? 1 : Math.round(speed * 1000) / 1000;
    }
    // The speed row of stripModel: ramp points in clip time, and the curve's nodes (one per ramp piece inside the
    // clip, since speed is linear inside a piece). Points outside the clip are flagged, like keys.
    function rampModelRow(clip, fps, pps) {
        const Fx = window.VideoStudioEffects, last = clip.duration - 1;
        const points = clip.ramp.map((point, index) => ({ index, f: point.f, speed: point.speed, e: point.e, frame: clip.start + point.f, x: (clip.start + point.f) / fps * pps, outside: point.f < 0 || point.f > last }));
        const pieces = Fx.rampPieces(clip.ramp, 0, clip.duration), node = (t, speed) => ({ t, speed, x: (clip.start + t) / fps * pps });
        const curve = pieces.map(piece => node(piece.t0, piece.a));
        if (pieces.length) { const end = pieces[pieces.length - 1].t1; curve.push(node(end, Fx.rampSpeed(pieces, end))); }
        return { id: 'speed', points, curve };
    }
    function rampRow(s, model, row, width) {
        const esc = value => P.esc(s, value), tr = (key, fallback) => P.tr(s, key, fallback), w = model.width;
        const path = row.curve.map((node, index) => (index ? 'L' : 'M') + (node.x - model.left).toFixed(2) + ' ' + rampY(node.speed).toFixed(2)).join(' ');
        const one = rampY(1).toFixed(2);
        const points = row.points.filter(point => !point.outside).map(point => {
            const text = tr('rampPointSpeed', 'Speed {{speed}}× at {{time}}').split('{{speed}}').join(String(Math.round(point.speed * 100) / 100)).split('{{time}}').join(P.I().formatTime(Math.round(point.frame), s.project)) + ' · ' + tr('easing_' + point.e, point.e);
            return `<button type="button" class="vs-ramp-point vs-kf-e-${esc(point.e)}" data-ramp-point="${point.index}" style="left:${point.x.toFixed(2)}px;top:${rampY(point.speed).toFixed(2)}px" aria-label="${esc(text)}" title="${esc(text)}"></button>`;
        }).join('');
        return `<div class="vs-kf-row vs-speed-lane" data-kf-row="speed"><div class="vs-kf-rowhead">${P.icon('ramp', 14)}<span class="vs-kf-rowname">${esc(tr('speedLane', 'Speed over time'))}</span></div><div class="vs-kf-track" style="width:${width}px"><span class="vs-kf-extent" style="left:${model.left.toFixed(2)}px;width:${w.toFixed(2)}px"></span><svg class="vs-speed-curve" style="left:${model.left.toFixed(2)}px;width:${w.toFixed(2)}px" viewBox="0 0 ${w.toFixed(2)} ${RAMP_LANE.height}" preserveAspectRatio="none" aria-hidden="true" focusable="false"><line class="vs-speed-one" x1="0" x2="${w.toFixed(2)}" y1="${one}" y2="${one}"/><path d="${path}"/></svg>${points}</div></div>`;
    }
    // Edit in the timeline, and the Custom preset: expand the strip of the selected ramped clip and focus its first
    // point. The narrow inspector drawer is modal, so it closes first.
    function openSpeedLane(s) {
        const found = s.project && s.selectedClipId ? P.T().clipFor(s.project, s.selectedClipId) : null;
        if (!found || !window.VideoStudioEffects.hasRamp(found.clip)) return false;
        if (s.app.classList.contains('vs-show-inspector') && s.app.getBoundingClientRect().width <= 920) P.setInspectorOpen(s, false, false);
        stripView(s, found.clip.id).expanded = true;
        P.renderTimeline(s);
        P.T().revealFrame(s.q('[data-timeline]'), s, found.clip.start);
        const point = s.q('[data-timeline] [data-kf-strip] [data-ramp-point]');
        if (point) { point.scrollIntoView({ block: 'nearest' }); point.focus({ preventScroll: true }); }
        return true;
    }
    // Animation section presets (index §4.18): the selected clips that take a ramp get the preset in one undo step.
    // "Keep duration" keeps their length, and Custom opens the speed lane.
    function applyRampPreset(s, preset) {
        if (s.readonly || !s.project) return false;
        const keep = !!s.q('[data-inspector] [data-ramp-keep]')?.checked;
        const ok = P.applyEdit(s, 'speed ramp', project => window.VideoStudioKeyframeEdit.applyRampPreset(project, P.selectedIds(s), preset, keep));
        if (ok && preset === 'custom') openSpeedLane(s);
        else s.q(`[data-inspector] [data-ramp-preset="${CSS.escape(preset)}"]`)?.focus({ preventScroll: true });
        return ok;
    }
    // timelineOptions.stripAfter: the strip markup after `track`'s row, or ''.
    function keyframeStrip(s, track, pps, width) {
        const Fx = window.VideoStudioEffects;
        const found = s.project && s.selectedClipId ? P.T().clipFor(s.project, s.selectedClipId) : null;
        if (!found || found.track.id !== track.id || track.kind === 'audio') return '';
        const clip = found.clip, keyed = K().keyedPaths(clip).length > 0;
        if (!keyed && !Fx.hasRamp(clip) && !s.kfBoard) return '';
        const view = stripView(s, clip.id), model = stripModel(s.project, clip, pps, view);
        const known = new Set();
        K().keyedPaths(clip).forEach(path => K().keysOf(clip, path).forEach(key => known.add(path + ':' + key.f)));
        view.selected = view.selected.filter(id => known.has(id));
        const selected = new Set(view.selected), readonly = !!s.readonly || !!track.locked;
        const esc = value => P.esc(s, value), tr = (key, fallback) => P.tr(s, key, fallback);
        // Row and diamond names: a base value's registry label, or "<entry name> · <param label>" for an effect param.
        const label = path => {
            const spec = Fx.paramSpec(clip, path), name = tr(spec ? spec.label_key : path, path), entry = /^effects\.(\d+)\./.exec(path);
            return entry && window.VideoStudioInspectorEffects ? window.VideoStudioInspectorEffects.entryName(clip, Number(entry[1]), tr) + ' · ' + name : name;
        };
        const time = frame => P.I().formatTime(Math.max(0, Math.round(frame)), s.project);
        const diamond = (row, key) => {
            const summary = row.id === 'summary', path = summary ? 'summary' : row.id;
            const ids = summary ? key.paths.map(name => name + ':' + key.f) : [path + ':' + key.f];
            const parts = [tr('keyframes', 'Keyframes') + ': ' + time(key.frame), summary ? key.paths.map(label).join(', ') : label(path)];
            if (key.e !== 'mixed') parts.push(tr('easing_' + key.e, key.e));
            const text = parts.join(' · ');
            return `<button type="button" class="vs-kf-diamond vs-kf-e-${esc(key.e)}" data-kf-key="${esc(path + ':' + key.f)}" data-kf-path="${esc(path)}" data-kf-f="${key.f}" style="left:${key.x.toFixed(2)}px" aria-pressed="${ids.length > 0 && ids.every(id => selected.has(id))}" aria-label="${esc(text)}" title="${esc(text)}"></button>`;
        };
        const toggle = model.expanded ? tr('keyframeCollapse', 'Show only the summary') : tr('keyframeExpand', 'Show keyframes per property');
        const extent = `<span class="vs-kf-extent" style="left:${model.left.toFixed(2)}px;width:${model.width.toFixed(2)}px"></span>`;
        const rowMarkup = row => {
            if (row.id === 'speed') return rampRow(s, model, row, width);
            const summary = row.id === 'summary';
            const head = summary
                ? `<button type="button" class="vs-kf-chevron" data-kf-action="expand" aria-expanded="${model.expanded}" title="${esc(toggle)}" aria-label="${esc(toggle)}">${P.icon(model.expanded ? 'chevronDown' : 'chevronRight', 14)}</button><span class="vs-kf-rowname">${esc(tr('keyframeSummary', 'All keyframes'))}</span>`
                : `<span class="vs-kf-rowname">${esc(label(row.id))}</span>`;
            const keys = (row.keys || []).filter(key => !key.outside).map(key => diamond(row, key)).join('');
            return `<div class="vs-kf-row${summary ? ' vs-kf-summary' : ''}" data-kf-row="${esc(row.id)}"><div class="vs-kf-rowhead">${head}</div><div class="vs-kf-track" style="width:${width}px">${extent}${keys}</div></div>`;
        };
        const asset = (s.project.assets || []).find(item => item.id === clip.asset_id);
        const name = clip.text ? clip.text.split(/\r?\n/)[0] : asset ? P.assetDisplayName(s, asset) : '';
        return `<div class="vs-kf-strip${readonly ? ' is-readonly' : ''}" data-kf-strip="${esc(clip.id)}" role="group" aria-label="${esc(tr('keyframes', 'Keyframes') + (name ? ' – ' + name : ''))}">${model.rows.map(rowMarkup).join('')}</div>`;
    }
    // Pointer, keyboard and menu handling of the strip. The listeners sit on the timeline root and end with s.abort. The
    // timeline's own handlers ignore strip targets, and these ignore everything else.
    // - Diamonds: click selects (Shift toggles).
    // - Drag moves in whole timeline frames, snapping to the playhead when snapping is on. The pressed diamond stays on
    //   the clip; a refused position keeps the last valid one.
    // - Enter/Space selects (Shift toggles), ←/→ move by one frame (Shift: ten), Del/Backspace deletes and stops the
    //   event so the clip stays, Escape clears, and double-click moves the playhead to the diamond.
    // - Right-click or Shift+F10 opens the easing / copy / paste / delete menu.
    function wireKeyframeStrip(s, root) {
        const signal = s.abort.signal;
        const Fx = () => window.VideoStudioEffects, KE = () => window.VideoStudioKeyframeEdit;
        const pps = () => P.T().BASE_PX_PER_SECOND * (s.zoom || 1);
        const fps = () => P.rateOf(s);
        const snapRange = () => Math.max(1, Math.round(8 / pps() * fps()));
        let drag = null, suppressClick = false;
        // The strip's clip, while it is still the selected clip.
        function current() {
            const found = s.kfStrip && s.project ? P.T().clipFor(s.project, s.kfStrip.clipId) : null;
            return found && found.clip.id === s.selectedClipId ? found : null;
        }
        const editable = found => !!found && !s.readonly && !found.track.locked;
        const idsOf = (found, button) => stripIds(found.clip, button.dataset.kfPath, Number(button.dataset.kfF));
        const pressed = (found, button) => { const ids = idsOf(found, button); return ids.length > 0 && ids.every(id => s.kfStrip.selected.includes(id)); };
        function select(found, button, add) {
            const ids = idsOf(found, button), view = s.kfStrip;
            if (!add) view.selected = ids;
            else if (ids.every(id => view.selected.includes(id))) view.selected = view.selected.filter(id => !ids.includes(id));
            else view.selected = Array.from(new Set(view.selected.concat(ids)));
        }
        // Selection-only changes update aria-pressed in place, so the focused diamond and the click target survive.
        function syncPressed(found) { root.querySelectorAll('[data-kf-strip] [data-kf-key]').forEach(button => button.setAttribute('aria-pressed', String(pressed(found, button)))); }
        function render(focusId) { P.renderTimeline(s); stripFocus(s, focusId); }
        function clampDelta(clip, anchor, delta) { return Math.max(clip.start - anchor, Math.min(clip.start + clip.duration - 1 - anchor, delta)); }
        // One undo step through applyEdit. A move reads the selection back from the clip; the ids are captured first,
        // because the render inside applyEdit drops ids that no longer exist.
        function edit(found, label, op, focusId, moved) {
            const before = P.clone(found.clip), ids = s.kfStrip.selected.slice();
            if (!P.applyEdit(s, label, op)) { stripFocus(s, focusId); return false; }
            const after = P.T().clipFor(s.project, found.clip.id);
            let focus = focusId;
            if (moved && after) {
                const map = stripMoved(before, after.clip, ids);
                s.kfStrip.selected = Array.from(map.values());
                focus = stripAnchor(map, focusId);
            }
            render(focus);
            return true;
        }
        function deleteKeys(found) {
            const keys = stripKeys(s);
            if (!keys.length || !editable(found)) return;
            if (P.applyEdit(s, 'delete keyframes', project => KE().removeKeys(project, found.clip.id, keys))) s.kfStrip.selected = [];
            render('');
        }
        function copyKeys(found) {
            const keys = stripKeys(s);
            if (!keys.length) return;
            const first = Math.min(...keys.map(key => Math.round(Fx().timelineAt(found.clip, key.f))));
            s.kfBoard = KE().copyKeys(found.clip, keys, first);
            P.showNotice(s, 'clipsCopied', 'Copied.', false);
        }
        function pasteKeys(found) {
            if (s.kfBoard && editable(found)) edit(found, 'paste keyframes', project => KE().pasteKeys(project, found.clip.id, s.kfBoard, s.frame), '');
        }
        function easingOf(clip, key) {
            const hit = KE().keysOf(clip, key.path).find(item => item.f === key.f);
            return hit ? hit.e : '';
        }
        // Easing (radio items; the easing all selected keys share is checked), copy, paste at the playhead, delete.
        function openStripMenu(found, x, y, focusId) {
            const ro = !editable(found), keys = stripKeys(s), easings = keys.map(key => easingOf(found.clip, key));
            const shared = easings.length && easings.every(e => e === easings[0]) ? easings[0] : '';
            const items = Fx().EASINGS.map(easing => ({ easing, label: P.tr(s, 'easing_' + easing, easing), icon: easing === shared ? 'check' : '', disabled: ro || !keys.length, run: () => edit(found, 'keyframe easing', project => KE().setEasing(project, found.clip.id, keys, easing), focusId) }))
                .concat([
                    { separator: true },
                    { label: P.tr(s, 'keyframeCopy', 'Copy keyframes'), icon: 'copy', disabled: !keys.length, run: () => copyKeys(found) },
                    { label: P.tr(s, 'keyframePaste', 'Paste keyframes at the playhead'), disabled: ro || !s.kfBoard, run: () => pasteKeys(found) },
                    { separator: true },
                    { label: P.tr(s, 'keyframeDelete', 'Delete the selected keyframes'), shortcut: P.tr(s, 'keyDel', 'Del'), icon: 'trash', disabled: ro || !keys.length, run: () => deleteKeys(found) }
                ]);
            const el = P.openMenu(s, x, y, items, P.tr(s, 'keyframes', 'Keyframes'), () => (focusId && s.q(`[data-timeline] [data-kf-key="${CSS.escape(focusId)}"]`)) || s.q('[data-timeline] [data-kf-strip] [data-kf-action="expand"]'));
            items.forEach((item, index) => {
                const button = item.easing && el ? el.querySelector(`[data-menu-index="${index}"]`) : null;
                if (button) { button.setAttribute('role', 'menuitemradio'); button.setAttribute('aria-checked', String(item.easing === shared)); }
            });
        }
        function down(event) {
            suppressClick = false;
            const rampPoint = event.button === 0 && event.target.closest ? event.target.closest('[data-kf-strip] [data-ramp-point]') : null;
            if (rampPoint) { rampDown(event, rampPoint); return; }
            const button = event.button === 0 && event.target.closest ? event.target.closest('[data-kf-strip] [data-kf-key]') : null;
            const found = button ? current() : null;
            if (!found) return;
            const id = button.dataset.kfKey;
            // The click after this press is ignored whatever it lands on (a drag re-renders the diamond under it).
            suppressClick = true;
            event.preventDefault();
            button.focus({ preventScroll: true });
            // Shift changes the selection and never starts a drag.
            if (event.shiftKey) { select(found, button, true); syncPressed(found); return; }
            const wasPressed = pressed(found, button);
            if (!wasPressed) select(found, button, false);
            syncPressed(found);
            if (!editable(found) || s.timelineDragging) return;
            drag = { clipId: found.clip.id, before: P.clone(s.project), selected: s.kfStrip.selected.slice(), x: event.clientX, anchor: Math.round(Fx().timelineAt(found.clip, Number(button.dataset.kfF))), delta: 0, moved: false, focus: id, current: id, collapse: wasPressed && s.kfStrip.selected.length > 1, epoch: s.projectEpoch };
            s.timelineDragging = true;
            try { root.setPointerCapture(event.pointerId); } catch (_) { /* synthetic or finished pointer */ }
        }
        function move(event) {
            if (rampDrag) { rampMove(event); return; }
            if (!drag) return;
            if (drag.epoch !== s.projectEpoch) { drag = null; s.timelineDragging = false; return; }
            const clip = P.T().clipFor(drag.before, drag.clipId).clip;
            let delta = Math.round((event.clientX - drag.x) / pps() * fps());
            if (s.snap !== false && Math.abs(drag.anchor + delta - s.frame) <= snapRange()) delta = s.frame - drag.anchor;
            delta = clampDelta(clip, drag.anchor, delta);
            if (delta === drag.delta) return;
            const project = P.clone(drag.before);
            if (delta && KE().moveKeys(project, drag.clipId, drag.selected.map(stripSplit), delta) !== true) return; // a refused position keeps the last valid one
            const map = delta ? stripMoved(clip, P.T().clipFor(project, drag.clipId).clip, drag.selected) : new Map(drag.selected.map(id => [id, id]));
            drag.delta = delta;
            drag.moved = true;
            drag.current = stripAnchor(map, drag.focus);
            s.project = project;
            s.kfStrip.selected = Array.from(map.values());
            render(drag.current);
            if (s.preview) s.preview.seek(s.frame);
        }
        function up(event) {
            if (rampDrag) { rampUp(event); return; }
            if (!drag) return;
            const done = drag;
            drag = null;
            s.timelineDragging = false;
            if (done.epoch !== s.projectEpoch) return;
            if (event.type === 'pointercancel') { s.project = done.before; s.kfStrip.selected = done.selected; render(done.focus); return; }
            if (done.moved && JSON.stringify(done.before) !== JSON.stringify(s.project)) P.recordChange(s, 'move keyframes', done.before);
            else if (!done.moved && done.collapse) {
                // A press without movement on a diamond of a multi-selection selects only that diamond.
                const { path, f } = stripSplit(done.focus), found = current();
                if (found) { s.kfStrip.selected = stripIds(found.clip, path, f); syncPressed(found); }
            }
            if (s.timelineOptions && s.timelineOptions.onDragEnd) s.timelineOptions.onDragEnd();
            stripFocus(s, done.current);
            // A click on the editor background after the release must not keep the focus.
            if (done.moved) window.setTimeout(() => { if (!s.disposed) stripFocus(s, done.current); }, 0);
        }
        const cancel = () => { if (drag || rampDrag) up({ type: 'pointercancel' }); };
        function click(event) {
            if (suppressClick) { suppressClick = false; return; }
            const strip = event.target.closest && event.target.closest('[data-kf-strip]');
            const found = strip ? current() : null;
            if (!found) return;
            if (event.target.closest('[data-kf-action="expand"]')) {
                s.kfStrip.expanded = !s.kfStrip.expanded;
                P.renderTimeline(s);
                s.q('[data-timeline] [data-kf-strip] [data-kf-action="expand"]')?.focus({ preventScroll: true });
                return;
            }
            // A click on empty strip space clears the key selection.
            if (!event.target.closest('[data-kf-key],[data-ramp-point]') && s.kfStrip.selected.length) { s.kfStrip.selected = []; syncPressed(found); }
        }
        function dblclick(event) {
            if (rampAdd(event)) return;
            const button = event.target.closest && event.target.closest('[data-kf-strip] [data-kf-key]');
            const found = button ? current() : null;
            if (!found || !s.preview) return;
            s.preview.pause();
            s.preview.seek(Math.round(Fx().timelineAt(found.clip, Number(button.dataset.kfF))));
        }
        function keydown(event) {
            suppressClick = false;
            if (rampKeys(event)) return;
            const button = event.target.closest && event.target.closest('[data-kf-strip] [data-kf-key]');
            // Alt+K and Alt+↑/↓ belong to P.handleKeyframeKeys; Ctrl/Cmd keys stay with the editor.
            if (!button || event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) return;
            const found = current();
            if (!found) return;
            const id = button.dataset.kfKey, key = event.key;
            if (key === 'Enter' || key === ' ') { select(found, button, event.shiftKey); syncPressed(found); }
            else if (key === 'ArrowLeft' || key === 'ArrowRight') {
                if (!editable(found)) return;
                if (!pressed(found, button)) select(found, button, false);
                const anchor = Math.round(Fx().timelineAt(found.clip, Number(button.dataset.kfF)));
                const delta = clampDelta(found.clip, anchor, (key === 'ArrowLeft' ? -1 : 1) * (event.shiftKey ? 10 : 1));
                const keys = stripKeys(s);
                if (delta) edit(found, 'move keyframes', project => KE().moveKeys(project, found.clip.id, keys, delta), id, true);
                else syncPressed(found);
            } else if (key === 'Delete' || key === 'Backspace') {
                if (editable(found)) { if (!pressed(found, button)) select(found, button, false); deleteKeys(found); }
            } else if (key === 'Escape' && s.kfStrip.selected.length) { s.kfStrip.selected = []; syncPressed(found); }
            else return;
            event.preventDefault();
            event.stopPropagation();
        }
        function contextmenu(event) {
            const strip = event.target.closest && event.target.closest('[data-kf-strip]');
            const found = strip ? current() : null;
            if (!found) return;
            event.preventDefault();
            event.stopPropagation();
            // A keyboard-opened menu has no pointer position inside the target, so it anchors below the target.
            const anchor = event.target.closest('[data-kf-key],[data-ramp-point],[data-kf-action],.vs-kf-rowhead,.vs-kf-track') || strip, rect = anchor.getBoundingClientRect();
            const inside = event.clientX >= rect.left && event.clientX <= rect.right && event.clientY >= rect.top && event.clientY <= rect.bottom;
            const x = inside ? event.clientX : rect.left + Math.min(24, rect.width / 2), y = inside ? event.clientY : rect.bottom;
            const rampPoint = event.target.closest('[data-ramp-point]');
            if (rampPoint) { openRampMenu(found, Number(rampPoint.dataset.rampPoint), x, y); return; }
            const button = event.target.closest('[data-kf-key]');
            if (button && !pressed(found, button)) { select(found, button, false); syncPressed(found); }
            openStripMenu(found, x, y, button ? button.dataset.kfKey : '');
        }
        // Ramp points in the speed lane.
        // - Drag sideways moves the point (whole frames, snapping to the playhead, kept on the clip); drag up or down
        //   changes its speed. An invalid fit keeps the last valid state.
        // - ←/→ move by one frame (Shift: ten); ↑/↓ change the speed by 2^(1/8) (Shift: by 2).
        // - Del/Backspace removes the point. Double-click on the lane adds one.
        // - Right-click or Shift+F10 opens the menu: easing, add at the playhead, delete.
        let rampDrag = null;
        function rampFocus(index) {
            const point = s.q(`[data-timeline] [data-kf-strip] [data-ramp-point="${index}"]`);
            if (point) point.focus({ preventScroll: true }); else stripFocus(s, '');
        }
        const rampClip = clip => (f => Math.max(0, Math.min(clip.duration - 1, f)));
        function rampDown(event, button) {
            const found = current(), index = Number(button.dataset.rampPoint);
            suppressClick = true;
            event.preventDefault();
            button.focus({ preventScroll: true });
            if (!found || !Fx().hasRamp(found.clip) || !found.clip.ramp[index] || !editable(found) || s.timelineDragging) return;
            const point = found.clip.ramp[index];
            rampDrag = { clipId: found.clip.id, index, before: P.clone(s.project), x: event.clientX, y: event.clientY, f: point.f, speed: point.speed, key: '', moved: false, epoch: s.projectEpoch };
            s.timelineDragging = true;
            try { root.setPointerCapture(event.pointerId); } catch (_) { /* synthetic or finished pointer */ }
        }
        function rampMove(event) {
            const d = rampDrag;
            if (d.epoch !== s.projectEpoch) { rampDrag = null; s.timelineDragging = false; return; }
            const clip = P.T().clipFor(d.before, d.clipId).clip;
            let f = d.f + Math.round((event.clientX - d.x) / pps() * fps());
            if (s.snap !== false && Math.abs(clip.start + f - s.frame) <= snapRange()) f = s.frame - clip.start;
            f = rampClip(clip)(f);
            const speed = event.clientY === d.y ? d.speed : rampSpeedAtY(rampY(d.speed) + event.clientY - d.y);
            const key = f + '|' + speed;
            if (key === d.key) return;
            const project = P.clone(d.before);
            if (window.VideoStudioKeyframeEdit.moveRampPoint(project, d.clipId, d.index, f, speed) !== true) return; // an invalid fit keeps the last valid state
            d.key = key;
            d.moved = true;
            s.project = project;
            P.renderTimeline(s);
            rampFocus(d.index);
            if (s.preview) s.preview.seek(s.frame);
        }
        function rampUp(event) {
            const d = rampDrag;
            rampDrag = null;
            s.timelineDragging = false;
            if (d.epoch !== s.projectEpoch) return;
            if (event.type === 'pointercancel') { s.project = d.before; P.renderTimeline(s); rampFocus(d.index); return; }
            if (d.moved && JSON.stringify(d.before) !== JSON.stringify(s.project)) P.recordChange(s, 'speed ramp', d.before);
            if (s.timelineOptions && s.timelineOptions.onDragEnd) s.timelineOptions.onDragEnd();
            rampFocus(d.index);
            if (d.moved) window.setTimeout(() => { if (!s.disposed) rampFocus(d.index); }, 0);
        }
        function rampAdd(event) {
            const lane = event.target.closest && !event.target.closest('[data-ramp-point]') ? event.target.closest('[data-kf-strip] [data-kf-row="speed"] .vs-kf-track') : null;
            const found = lane ? current() : null;
            if (!found) return false;
            if (!editable(found)) return true;
            const rect = lane.getBoundingClientRect(), f = Math.round((event.clientX - rect.left) / pps() * fps()) - found.clip.start;
            if (f < 0 || f > found.clip.duration - 1) return true;
            const speed = rampSpeedAtY(event.clientY - rect.top);
            if (P.applyEdit(s, 'speed ramp', project => window.VideoStudioKeyframeEdit.addRampPoint(project, found.clip.id, f, speed))) {
                P.renderTimeline(s);
                rampFocus((P.T().clipFor(s.project, found.clip.id).clip.ramp || []).findIndex(point => point.f === f));
            }
            return true;
        }
        function rampKeys(event) {
            const button = event.target.closest && event.target.closest('[data-kf-strip] [data-ramp-point]');
            if (!button || event.defaultPrevented || event.ctrlKey || event.metaKey || event.altKey) return false;
            const found = current(), index = Number(button.dataset.rampPoint);
            const point = found && Fx().hasRamp(found.clip) ? found.clip.ramp[index] : null;
            if (!point) return false;
            const key = event.key, KEr = window.VideoStudioKeyframeEdit, step = event.shiftKey ? 10 : 1, factor = event.shiftKey ? 2 : 2 ** (1 / 8);
            let op = null;
            if (key === 'ArrowLeft' || key === 'ArrowRight') op = project => KEr.moveRampPoint(project, found.clip.id, index, rampClip(found.clip)(point.f + (key === 'ArrowLeft' ? -step : step)), point.speed);
            else if (key === 'ArrowUp' || key === 'ArrowDown') op = project => KEr.moveRampPoint(project, found.clip.id, index, point.f, Math.round(Math.min(16, Math.max(0.25, key === 'ArrowUp' ? point.speed * factor : point.speed / factor)) * 1000) / 1000);
            else if (key === 'Delete' || key === 'Backspace') op = project => KEr.removeRampPoint(project, found.clip.id, index);
            else return false;
            event.preventDefault();
            event.stopPropagation();
            if (!editable(found)) return true;
            const removing = key === 'Delete' || key === 'Backspace';
            if (P.applyEdit(s, 'speed ramp', op)) P.renderTimeline(s);
            rampFocus(removing ? Math.max(0, index - 1) : index);
            return true;
        }
        function openRampMenu(found, index, x, y) {
            const point = found.clip.ramp && found.clip.ramp[index];
            if (!point) return;
            const ro = !editable(found), KEr = window.VideoStudioKeyframeEdit, clipId = found.clip.id;
            const inside = s.frame >= found.clip.start && s.frame < found.clip.start + found.clip.duration;
            const apply = op => { if (P.applyEdit(s, 'speed ramp', op)) P.renderTimeline(s); rampFocus(index); };
            const items = Fx().EASINGS.map(easing => ({ easing, label: P.tr(s, 'easing_' + easing, easing), icon: easing === point.e ? 'check' : '', disabled: ro, run: () => apply(project => KEr.setRamp(project, clipId, P.T().clipFor(project, clipId).clip.ramp.map((item, i) => (i === index ? Object.assign({}, item, { e: easing }) : item)))) }))
                .concat([
                    { separator: true },
                    { label: P.tr(s, 'keyframeAdd', 'Add a keyframe for {{name}} at the playhead').split('{{name}}').join(P.tr(s, 'speed', 'Speed')), icon: 'plus', disabled: ro || !inside, run: () => apply(project => KEr.addRampPoint(project, clipId, s.frame - found.clip.start)) },
                    { label: P.tr(s, 'keyframeDelete', 'Delete the selected keyframes'), shortcut: P.tr(s, 'keyDel', 'Del'), icon: 'trash', disabled: ro, run: () => apply(project => KEr.removeRampPoint(project, clipId, index)) }
                ]);
            const el = P.openMenu(s, x, y, items, P.tr(s, 'speedLane', 'Speed over time'), () => s.q(`[data-timeline] [data-kf-strip] [data-ramp-point="${index}"]`));
            items.forEach((item, i) => {
                const button = item.easing && el ? el.querySelector(`[data-menu-index="${i}"]`) : null;
                if (button) { button.setAttribute('role', 'menuitemradio'); button.setAttribute('aria-checked', String(item.easing === point.e)); }
            });
        }
        root.addEventListener('pointerdown', down, { signal });
        window.addEventListener('pointermove', move, { signal });
        window.addEventListener('pointerup', up, { signal });
        window.addEventListener('pointercancel', up, { signal });
        root.addEventListener('lostpointercapture', cancel, { signal });
        window.addEventListener('blur', cancel, { signal });
        root.addEventListener('click', click, { signal });
        root.addEventListener('dblclick', dblclick, { signal });
        root.addEventListener('keydown', keydown, { signal });
        root.addEventListener('contextmenu', contextmenu, { signal });
    }

    window.VideoStudioKeyframes = { controlState, stripModel };
    Object.assign(P, { keyframeView, handleKeyframeClick, syncKeyframeControls, applyAnimated, keyframeCommand, isKeyframeShortcut, handleKeyframeKeys, keyframeStrip, wireKeyframeStrip, applyRampPreset });
})();
