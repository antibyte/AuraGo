// Storymaker app scene strip and transport. The strip lists the scenes with the thumbnails the player renders (the `thumb`
// message), the narration snippet or id, the duration of the compile report and the add buttons; it moves with the arrow
// keys. The transport is play/pause, the time and the scrubber: one block per scene of the compile report, the narration waveform, a playhead, pointer
// dragging and the keyboard (arrows step a frame, Shift a second, Home/End the ends). The scene texts are untrusted and are
// set as text only. Classic script, publishes S.createStrip and S.createTransport.
(function () {
    'use strict';

    const S = window.StorymakerParts = window.StorymakerParts || {};

    function drawBitmap(canvas, bitmap) {
        if (!bitmap) return;
        if (canvas.width !== bitmap.width) canvas.width = bitmap.width;
        if (canvas.height !== bitmap.height) canvas.height = bitmap.height;
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.drawImage(bitmap, 0, 0);
        canvas.dataset.drawn = 'true';
    }

    function snippet(text, max) {
        const one = String(text || '').replace(/\s+/g, ' ').trim();
        return one.length > max ? one.slice(0, max - 1) + '…' : one;
    }

    // inst: {t, bus, story, selected, report, thumbs, readonly, select(i, opts), addScene(kind)}
    function createStrip(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const list = el('ol', { class: 'stm-scene-list', dataset: { stm: 'scene-list' }, ariaLabel: t('storymaker.strip.title') });
        const addScene = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'add-scene' }, onClick: () => inst.addScene('storyboard') }, icon('plus'), el('span', { text: t('storymaker.strip.add_storyboard') }));
        const addCode = el('button', { type: 'button', class: 'stm-btn stm-btn-quiet', dataset: { stm: 'add-code-scene' }, onClick: () => inst.addScene('code') }, icon('code'), el('span', { text: t('storymaker.strip.add_code') }));
        const root = el('aside', { class: 'stm-strip', ariaLabel: t('storymaker.strip.title') },
            el('h3', { class: 'stm-panel-title', text: t('storymaker.strip.title') }),
            list,
            el('div', { class: 'stm-strip-add' }, addScene, addCode));

        function durationText(index) {
            const report = inst.report;
            const scene = report && report.scenes ? report.scenes[index] : null;
            if (!scene || !report.fps) return '';
            return t('storymaker.strip.duration_s', { seconds: S.format.seconds((scene.end_frame - scene.start_frame) / report.fps) });
        }

        function item(scene, index) {
            const selected = index === inst.selected;
            const isCode = scene.kind === 'code';
            const canvas = el('canvas', { class: 'stm-thumb', width: '192', height: '108', dataset: { scene: scene.id } });
            const bitmap = inst.thumbs.get(scene.id);
            if (bitmap) drawBitmap(canvas, bitmap);
            const label = snippet(scene.narration, 48) || scene.id;
            const button = el('button', {
                type: 'button', class: 'stm-scene-btn', tabindex: selected ? '0' : '-1', dataset: { stm: 'scene', index, scene: scene.id },
                ariaCurrent: selected ? 'true' : null,
                ariaLabel: t('storymaker.strip.scene_n', { n: index + 1 }) + ': ' + label,
                onClick: () => inst.select(index, { seek: true }),
                onKeydown: event => {
                    const next = event.key === 'ArrowDown' || event.key === 'ArrowRight' ? index + 1 : event.key === 'ArrowUp' || event.key === 'ArrowLeft' ? index - 1 : event.key === 'Home' ? 0 : event.key === 'End' ? S.model.scenesOf(inst.story).length - 1 : -1;
                    if (next < 0 || next >= S.model.scenesOf(inst.story).length) return;
                    event.preventDefault();
                    inst.select(next, { seek: true });
                    const target = list.querySelector('[data-index="' + next + '"]');
                    if (target) target.focus();
                }
            },
                canvas,
                el('span', { class: 'stm-scene-num', text: String(index + 1) }),
                isCode ? el('span', { class: 'stm-scene-kind', title: t('storymaker.strip.kind_code') }, icon('code')) : null,
                el('span', { class: 'stm-scene-name', text: label }),
                el('span', { class: 'stm-scene-dur', text: durationText(index) }));
            return el('li', { class: 'stm-scene', dataset: { selected: String(selected), kind: isCode ? 'code' : 'storyboard' } }, button);
        }

        function paint() {
            const focused = list.contains(document.activeElement) ? document.activeElement.getAttribute('data-index') : null;
            S.clear(list);
            S.model.scenesOf(inst.story).forEach((scene, index) => list.appendChild(item(scene, index)));
            const off = inst.readonly;
            addScene.disabled = off;
            addCode.disabled = off;
            if (focused !== null) {
                const again = list.querySelector('[data-index="' + focused + '"]');
                if (again) again.focus();
            }
            const current = list.querySelector('[aria-current="true"]');
            if (current && typeof current.scrollIntoView === 'function') current.scrollIntoView({ block: 'nearest', inline: 'nearest' });
        }

        function drawScene(sceneId) {
            const bitmap = inst.thumbs.get(sceneId);
            list.querySelectorAll('canvas[data-scene]').forEach(canvas => { if (canvas.dataset.scene === sceneId) drawBitmap(canvas, bitmap); });
        }

        const off = [
            inst.bus.on('story', paint),
            inst.bus.on('selection', paint),
            inst.bus.on('report', paint),
            inst.bus.on('readonly', paint),
            inst.bus.on('thumb', drawScene)
        ];
        paint();
        return { el: root, paint, dispose() { off.forEach(fn => fn()); } };
    }

    // inst: {t, bus, report, audioInfo, frame, playing, readonly, play(), pause(), seek(frame)}
    function createTransport(inst) {
        const { el, icon } = S;
        const t = inst.t;
        const playButton = el('button', { type: 'button', class: 'stm-btn stm-btn-primary stm-btn-icon', dataset: { stm: 'play' }, ariaLabel: t('storymaker.play'), title: t('storymaker.play'), onClick: () => (inst.playing ? inst.pause() : inst.play()) }, icon('play'));
        const clock = el('span', { class: 'stm-clock', dataset: { stm: 'clock' }, role: 'timer', text: '0:00.0' });
        const total = el('span', { class: 'stm-clock stm-clock-total', dataset: { stm: 'clock-total' }, text: '0:00.0' });
        const blocks = el('div', { class: 'stm-scrub-blocks' });
        const wave = el('canvas', { class: 'stm-scrub-wave', ariaHidden: 'true', dataset: { stm: 'scrub-wave', drawn: 'false' } });
        const head = el('div', { class: 'stm-scrub-head' });
        const scrub = el('div', {
            class: 'stm-scrub', role: 'slider', tabindex: '0', dataset: { stm: 'scrubber' },
            ariaLabel: t('storymaker.timeline'), ariaValuemin: '0', ariaValuemax: '0', ariaValuenow: '0'
        }, blocks, wave, head);
        const root = el('div', { class: 'stm-transport' }, playButton, clock, scrub, total);
        let dragging = false;

        function frameCount() {
            return inst.report && inst.report.frame_count > 0 ? inst.report.frame_count : 0;
        }

        function frameAt(event) {
            const rect = scrub.getBoundingClientRect();
            const ratio = rect.width > 0 ? S.clamp((event.clientX - rect.left) / rect.width, 0, 1) : 0;
            return Math.round(ratio * Math.max(0, frameCount() - 1));
        }

        function paintBlocks() {
            S.clear(blocks);
            const report = inst.report;
            const count = frameCount();
            if (!report || !count) return;
            report.scenes.forEach((scene, index) => {
                const left = scene.start_frame / count * 100;
                const width = Math.max(0.4, (scene.end_frame - scene.start_frame) / count * 100);
                blocks.appendChild(el('div', {
                    class: 'stm-scrub-block', title: t('storymaker.strip.scene_n', { n: index + 1 }) + ' · ' + scene.id,
                    dataset: { index, kind: scene.kind || 'storyboard', selected: String(index === inst.selected) },
                    style: { left: left + '%', width: width + '%' }
                }));
            });
        }

        function paintTime() {
            const count = frameCount();
            const fps = inst.report ? inst.report.fps : 30;
            const frame = S.clamp(inst.frame, 0, Math.max(0, count - 1));
            head.style.left = (count > 1 ? frame / (count - 1) * 100 : 0) + '%';
            clock.textContent = S.format.clock(frame, fps);
            total.textContent = S.format.clock(count, fps);
            scrub.setAttribute('aria-valuemax', String(Math.max(0, count - 1)));
            scrub.setAttribute('aria-valuenow', String(frame));
            scrub.setAttribute('aria-valuetext', S.format.clock(frame, fps));
            const enabled = count > 0;
            playButton.disabled = !enabled;
            scrub.setAttribute('aria-disabled', String(!enabled));
            if (playButton.dataset.playing !== String(!!inst.playing)) {
                const label = t(inst.playing ? 'storymaker.pause' : 'storymaker.play');
                playButton.setAttribute('aria-label', label);
                playButton.title = label;
                playButton.dataset.playing = String(!!inst.playing);
                S.clear(playButton).appendChild(icon(inst.playing ? 'pause' : 'play'));
            }
        }

        // One bar per column, as tall as the loudest frame under it. The colour comes from the stylesheet, so the wave follows the skin.
        function paintWave() {
            const report = inst.report;
            const count = frameCount();
            const narration = inst.audioInfo && inst.audioInfo.narration ? inst.audioInfo.narration : null;
            const width = Math.max(1, Math.round(wave.clientWidth || 300));
            const height = Math.max(1, Math.round(wave.clientHeight || 24));
            if (wave.width !== width) wave.width = width;
            if (wave.height !== height) wave.height = height;
            const ctx = wave.getContext('2d');
            if (!ctx) return;
            ctx.clearRect(0, 0, width, height);
            wave.dataset.drawn = 'false';
            if (!report || !count || !narration) return;
            const levels = S.audioModel.waveform(report, narration);
            ctx.fillStyle = window.getComputedStyle(wave).color;
            for (let x = 0; x < width; x++) {
                const from = Math.floor(x * count / width);
                const to = Math.max(from + 1, Math.floor((x + 1) * count / width));
                let peak = 0;
                for (let frame = from; frame < to && frame < count; frame++) peak = Math.max(peak, levels[frame]);
                if (!peak) continue;
                const bar = Math.max(1, Math.round(peak / 255 * height));
                ctx.fillRect(x, (height - bar) / 2, 1, bar);
                wave.dataset.drawn = 'true';
            }
        }

        function paintAll() {
            paintBlocks();
            paintWave();
            paintTime();
        }

        function pointer(event) {
            if (!frameCount()) return;
            inst.seek(frameAt(event));
        }
        scrub.addEventListener('pointerdown', event => {
            if (event.button !== 0 || !frameCount()) return;
            dragging = true;
            try { scrub.setPointerCapture(event.pointerId); } catch (_) { /* synthetic pointer */ }
            scrub.focus();
            pointer(event);
        });
        scrub.addEventListener('pointermove', event => { if (dragging) pointer(event); });
        const endDrag = () => { dragging = false; };
        scrub.addEventListener('pointerup', endDrag);
        scrub.addEventListener('pointercancel', endDrag);
        scrub.addEventListener('keydown', event => {
            const count = frameCount();
            if (!count) return;
            const fps = inst.report.fps || 30;
            const step = event.shiftKey ? fps : 1;
            let target = null;
            if (event.key === 'ArrowRight' || event.key === 'ArrowUp') target = inst.frame + step;
            else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') target = inst.frame - step;
            else if (event.key === 'Home') target = 0;
            else if (event.key === 'End') target = count - 1;
            else if (event.key === ' ' || event.key === 'Enter') { event.preventDefault(); if (inst.playing) inst.pause(); else inst.play(); return; }
            if (target === null) return;
            event.preventDefault();
            inst.seek(S.clamp(target, 0, count - 1));
        });

        const off = [
            inst.bus.on('report', paintAll),
            inst.bus.on('audio', paintWave),
            inst.bus.on('selection', paintBlocks),
            inst.bus.on('time', paintTime)
        ];
        paintAll();
        return { el: root, paint: paintAll, dispose() { off.forEach(fn => fn()); } };
    }

    S.createStrip = createStrip;
    S.createTransport = createTransport;
})();
