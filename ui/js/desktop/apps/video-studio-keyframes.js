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

    function isAnimated(clip, path) { const list = clip && clip.keyframes && clip.keyframes[path]; return Array.isArray(list) && list.length > 0; }
    function animatedPaths(clip) { return Fx().keyframable().filter(path => isAnimated(clip, path)); }
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
    // per keyframable value, whether the playhead is inside the clip (keys are set only there), whether the editor may
    // change the clip, and whether a box value is animated (fine tune opens by itself then).
    function keyframeView(s) {
        const found = visualSelection(s);
        if (!found) return null;
        const clip = found.clip, frame = Math.round(Number(s.frame) || 0), controls = {};
        Fx().keyframable().forEach(path => { controls[path] = controlState(s.project, clip, path, frame); });
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
    // Inspector keyframe buttons ([data-kf-path][data-kf-action]); true when the click was one of them.
    function handleKeyframeClick(s, event) {
        const button = event.target.closest('[data-kf-path][data-kf-action]');
        if (!button || !s.project) return false;
        const found = visualSelection(s), path = button.dataset.kfPath, action = button.dataset.kfAction;
        if (!found || button.disabled) return true;
        if (action === 'prev' || action === 'next') {
            const frame = K().neighbor(found.clip, [path], s.frame, action === 'prev' ? -1 : 1);
            if (frame != null) P.seekTo(s, frame);
            // At the first or last key the button turns off; the keyboard moves to the other one.
            if (button.disabled && button.parentElement) button.parentElement.querySelector(`[data-kf-action="${action === 'prev' ? 'next' : 'prev'}"]:not(:disabled)`)?.focus({ preventScroll: true });
            return true;
        }
        if (action === 'stopwatch') applyAnimated(s, path, !isAnimated(found.clip, path));
        else if (action === 'key') P.applyEdit(s, 'keyframe', project => K().toggleKeyframe(project, found.clip.id, path, s.frame));
        else return false;
        refocus(s, path, action);
        return true;
    }

    // One keyframe button follows the playhead: enabled state, aria-pressed, label and the diamond's icon.
    function syncButton(s, view, button) {
        const path = button.dataset.kfPath, action = button.dataset.kfAction, c = view.controls[path];
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

    window.VideoStudioKeyframes = { controlState };
    Object.assign(P, { keyframeView, handleKeyframeClick, syncKeyframeControls, applyAnimated, keyframeCommand, isKeyframeShortcut, handleKeyframeKeys });
})();
