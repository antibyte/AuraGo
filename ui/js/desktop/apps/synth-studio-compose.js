(function () {
    'use strict';
    function create(host, options) {
        const { tr, esc, signal } = options, M = window.SynthStudioModel, G = window.SynthStudioMusic;
        const previewAudio = window.SynthStudioAudio.create();
        let preview = null, degree = 0, epoch = 0, timer = null, nextTime = 0, index = 0, lastTrack = '', lastHarmony = '', held = new Map(), physical = new Set(), scheduled = [], padReleases = [];

        const label = (key, html) => '<label>' + esc(tr(key)) + html + '</label>';
        const select = (key, items) => '<select data-music="' + key + '">' + items.map(([v, t]) => '<option value="' + v + '">' + esc(t) + '</option>').join('') + '</select>';
        const range = (key, min, max, step, value) => '<input data-music="' + key + '" data-ss-kind="knob" type="range" min="' + min + '" max="' + max + '" step="' + step + '" value="' + value + '"><output>' + value + '</output>';
        const check = key => '<label><input data-music="' + key + '" type="checkbox"> ' + esc(tr(key)) + '</label>';
        const button = key => '<button type="button" data-compose="' + key + '">' + esc(tr(key)) + '</button>';

        const harmonySlot = host.querySelector('[data-perform-harmony]');
        const stageSlot = host.querySelector('[data-perform-panel="stage"]');
        const composeSlot = host.querySelector('[data-perform-panel="compose"]') || host;

        const harmonyMarkup = '<div class="ss-harmony">' +
            label('key', select('root', ['C', 'C♯', 'D', 'D♯', 'E', 'F', 'F♯', 'G', 'G♯', 'A', 'A♯', 'B'].map((n, i) => [i, n]))) +
            label('scale', select('scale', ['major', 'minor', 'dorian', 'phrygian', 'lydian', 'mixolydian', 'harmonic_minor', 'pentatonic_minor', 'hirajoshi'].map(s => [s, tr(s)]))) +
            check('scale_snap') +
            label('chord', select('seventh', [[0, tr('triad')], [1, tr('seventh')]])) +
            '</div>';

        const stageChordsMarkup = '<section class="ss-stage-chords">' +
            '<div class="ss-stage-heading">' +
            '<strong>' + esc(tr('chord')) + '</strong>' +
            check('arp_live') +
            check('arp_latch') +
            '<button type="button" class="ss-arp-jump" data-jump-arp title="' + esc(tr('arpeggiator')) + '"></button>' +
            '</div>' +
            '<div class="ss-chord-pads"></div>' +
            '</section>';

        const createItems = [['chord', 'chord'], ['progression', 'progression'], ['bassline', 'bassline'], ['melody', 'melody_generator'], ['rhythm', 'rhythm_generator']];
        const transformItems = [['arp', 'arpeggiator'], ['variation', 'variation_lab'], ['harmonize', 'harmonizer'], ['groove', 'groove_lab'], ['scale', 'scale_apply'], ['reverse', 'reverse_notes']];
        const allItems = [...createItems, ...transformItems];
        const navBtn = ([id, key]) => '<button type="button" class="ss-gen-pick" data-gen-pick="' + id + '" aria-pressed="' + (id === 'chord') + '"><span>' + esc(tr(key)) + '</span><small data-gen-badge="' + id + '"></small></button>';

        const composeMarkup = '<div class="ss-compose" data-active-generator="chord">' +
            '<aside class="ss-gen-nav" aria-label="' + esc(tr('generator')) + '">' +
            '<div class="ss-gen-group"><span class="ss-gen-group-title">' + esc(tr('gen_create')) + '</span>' + createItems.map(navBtn).join('') + '</div>' +
            '<div class="ss-gen-group"><span class="ss-gen-group-title">' + esc(tr('gen_transform')) + '</span>' + transformItems.map(navBtn).join('') + '</div>' +
            '</aside>' +
            '<div class="ss-gen-main">' +
            '<div class="ss-gen-panel-host">' +
            '<div class="ss-gen-info-card" data-gen-panel="chord"><strong>' + esc(tr('chord')) + '</strong><p>' + esc(tr('gen_chord_hint')) + '</p><div class="ss-gen-degrees" data-gen-degrees></div></div>' +
            '<div class="ss-gen-info-card" data-gen-panel="scale"><strong>' + esc(tr('scale_apply')) + '</strong><p>' + esc(tr('gen_scale_hint')) + '</p></div>' +
            '<div class="ss-gen-info-card" data-gen-panel="reverse"><strong>' + esc(tr('reverse_notes')) + '</strong><p>' + esc(tr('gen_reverse_hint')) + '</p></div>' +
            '<details open data-gen-panel="arp"><summary>' + esc(tr('arpeggiator')) + '</summary><div class="ss-generator-controls">' +
            '<div class="ss-arp-mirrors"><button type="button" data-arp-mirror="arp_live" aria-pressed="false">' + esc(tr('arp_live')) + '</button><button type="button" data-arp-mirror="arp_latch" aria-pressed="false">' + esc(tr('arp_latch')) + '</button></div>' +
            label('order', select('order', ['up', 'down', 'pendulum', 'converge', 'diverge', 'pinky', 'random', 'chord', 'strum'].map(s => [s, tr(s)]))) +
            label('grid', select('division', [8, 16, 32].map(n => [n, '1/' + n]))) +
            label('octaves', select('octaves', [1, 2, 3].map(n => [n, String(n)]))) +
            label('gate', range('gate', 0.1, 1, 0.05, 0.7)) +
            label('seed', '<input data-music="seed" type="text" maxlength="64" value="1">') +
            '</div></details>' +
            '<details open data-gen-panel="variation"><summary>' + esc(tr('variation_lab')) + '</summary><div class="ss-generator-controls">' + label('density', range('density', 0, 2, 0.1, 1)) + label('swing', range('swing', 0, 0.6, 0.05, 0)) + label('humanize', range('humanize', 0, 1, 0.05, 0.2)) + check('lock_pitch') + check('lock_rhythm') + '</div></details>' +
            '<details open data-gen-panel="rhythm"><summary>' + esc(tr('rhythm_generator')) + '</summary><div class="ss-generator-controls">' + label('steps', select('steps', [[16, '16'], [32, '32']])) + label('hits', '<input data-music="hits" type="number" min="0" max="32" value="5">') + label('rotation', '<input data-music="rotation" type="number" min="0" max="31" value="0">') + label('accent', select('accent', [1, 2, 3, 4, 8].map(n => [n, String(n)]))) + label('pitch', select('pitch', window.SynthStudioPresets.drums.map(d => [d.drumNote, d.name]))) + '</div></details>' +
            '<details open data-gen-panel="groove"><summary>' + esc(tr('groove_lab')) + '</summary><div class="ss-generator-controls">' +
            label('grid', select('groove_grid', [8, 16, 32].map(n => [n, '1/' + n]))) +
            label('quantize_strength', range('quantize', 0, 1, 0.05, 0)) +
            label('swing', range('groove_swing', 0, 0.66, 0.02, 0)) +
            label('humanize', range('groove_humanize', 0, 1, 0.05, 0)) +
            label('transpose', '<input data-music="groove_transpose" type="number" inputmode="numeric" enterkeyhint="done" min="-24" max="24" step="1" value="0">') +
            label('seed', '<input data-music="groove_seed" type="text" maxlength="64" value="1">') +
            '</div></details>' +
            '<details open data-gen-panel="progression"><summary>' + esc(tr('progression')) + '</summary><div class="ss-generator-controls">' +
            label('progression', select('progression', ['pop', 'ballad', 'jazz', 'minor', 'epic', 'canon', 'dreamy', 'darkwave', 'trance'].map(p => [p, tr('progression_' + p)]))) +
            label('progression_rate', select('progression_rate', [[1, '1'], [2, '2'], [4, '4']])) +
            label('gate', range('song_gate', 0.1, 1, 0.05, 0.9)) +
            '</div></details>' +
            '<details open data-gen-panel="bassline"><summary>' + esc(tr('bassline')) + '</summary><div class="ss-generator-controls">' +
            label('bass_style', select('bass_style', ['root', 'octave', 'walk', 'pulse', 'acid'].map(b => [b, tr('bass_' + b)]))) +
            label('gate', range('bass_gate', 0.1, 1, 0.05, 0.8)) +
            '</div></details>' +
            '<details open data-gen-panel="melody"><summary>' + esc(tr('melody_generator')) + '</summary><div class="ss-generator-controls">' +
            label('density', range('melody_density', 0.2, 1, 0.05, 0.65)) +
            label('melody_range', select('melody_range', [[1, '1'], [2, '2']])) +
            label('seed', '<input data-music="melody_seed" type="text" maxlength="64" value="1">') +
            '</div></details>' +
            '<details open data-gen-panel="harmonize"><summary>' + esc(tr('harmonizer')) + '</summary><div class="ss-generator-controls">' +
            label('harmony_interval', select('harmony_interval', ['third', 'fifth', 'sixth', 'octave', 'power', 'below'].map(i => [i, tr('interval_' + i)]))) +
            '</div></details>' +
            '</div>' +
            '<div class="ss-gen-dock">' +
            '<div class="ss-generator-actions">' +
            '<label class="ss-compose-clip-wrap">' + esc(tr('clip')) + '<select data-compose-clip aria-label="' + esc(tr('select_clip')) + '"></select></label>' +
            '<span data-compose-target hidden></span>' +
            label('generator', select('generator', allItems.map(([id, key]) => [id, tr(key)]))) +
            button('generate') +
            '</div>' +
            '<div class="ss-preview" hidden role="group" aria-label="' + esc(tr('preview')) + '">' + button('original') + button('variant') + '<span data-preview-state role="status"></span>' + button('apply') + button('cancel') + '</div>' +
            '</div></div></div>';

        if (harmonySlot && stageSlot) {
            harmonySlot.innerHTML = harmonyMarkup;
            stageSlot.insertAdjacentHTML('beforeend', stageChordsMarkup);
            composeSlot.innerHTML = composeMarkup;
        } else {
            composeSlot.innerHTML = harmonyMarkup + stageChordsMarkup + composeMarkup;
        }

        const q = key => host.querySelector('[data-music="' + key + '"]');
        const settings = () => Object.fromEntries([...host.querySelectorAll('[data-music]')].map(e => [e.dataset.music, e.type === 'checkbox' ? e.checked : e.type === 'number' || e.type === 'range' || ['root', 'seventh', 'division', 'octaves', 'steps', 'accent', 'pitch', 'groove_grid', 'progression_rate', 'melody_range'].includes(e.dataset.music) ? Number(e.value) : e.value]));
        function selection() { const p = options.project(), s = options.selection(), track = p.tracks.find(t => t.id === s.trackId); const clip = track?.clips.find(c => c.id === s.clipId); return { p, track, clip: clip && !clip.audio ? clip : undefined }; }
        function cancel() { epoch++; previewAudio.stop(); preview = null; host.querySelector('.ss-preview').hidden = true; }
        function stopLive() { epoch++; clearInterval(timer); timer = null; held.clear(); physical.clear(); for (const v of scheduled) v.cancel(); scheduled = []; padReleases.forEach(fn => fn()); padReleases = []; }
        function stop() { cancel(); stopLive(); }

        function pump() {
            if (!held.size) return;
            const { p, track } = selection(); if (!track) { stopLive(); return; }
            const now = options.audio.clock(), s = settings(), step = 60 / p.tempo * 4 / s.division;
            scheduled = scheduled.filter(v => v.end > now);
            if (nextTime < now - 0.1) { const missed = Math.ceil((now - nextTime) / step); index += missed; nextTime += missed * step; }
            try {
                while (nextTime < now + 0.075) {
                    const notes = [...held.values()], pitches = (s.order === 'chord' || s.order === 'strum') ? G.arpPitches(notes.map(n => n.pitch), s) : [G.arpPitch(notes.map(n => n.pitch), s, index)];
                    const duration = step * s.gate, spread = s.order === 'strum' ? step * 0.08 : 0;
                    index++;
                    pitches.forEach((pitch, voice) => {
                        const at = nextTime + voice * spread, velocity = notes[Math.min(voice, notes.length - 1)].velocity;
                        const cancelNote = options.audio.scheduleNote(track, pitch, velocity, at, duration);
                        scheduled.push({ cancel: cancelNote, end: at + duration + track.params.release });
                        options.onArpNote?.(track, pitch, velocity, at, duration);
                    });
                    nextTime += step;
                }
            } catch (error) { stopLive(); options.fail(error); }
        }

        async function hold(key, pitch, velocity = 0.8) {
            if (!q('arp_live').checked) return false;
            const latch = q('arp_latch').checked;
            if (latch && physical.size === 0 && held.size > 0) held.clear();
            physical.add(key);
            const token = epoch; held.set(key, { pitch, velocity });
            try {
                await options.audio.unlock();
                if (token !== epoch || !held.has(key)) return true;
                if (!timer) { nextTime = options.audio.clock() + 0.015; index = 0; pump(); timer = setInterval(pump, 25); }
            } catch (error) { stopLive(); options.fail(error); }
            return true;
        }
        function release(key) { physical.delete(key); if (!q('arp_latch').checked) held.delete(key); if (!held.size) stopLive(); }

        function generate() {
            if (options.blocked()) return;
            const { p, track, clip } = selection(); if (!clip) return;
            cancel();
            const s = settings(); let notes;
            if (s.generator === 'chord') { const start = Math.max(0, Math.min(clip.length - 1, Math.round((options.cursor() - clip.start) / 120) * 120)); notes = [...clip.notes, ...G.chord(p.harmony, degree, !!s.seventh).map((pitch, i) => ({ id: 'chord-' + i, pitch, start, duration: Math.min(1920, clip.length - start), velocity: 0.75 }))]; }
            else if (s.generator === 'arp') { const pitches = clip.notes.length ? [...new Set(clip.notes.map(n => n.pitch))] : G.chord(p.harmony, degree, !!s.seventh); notes = G.arpeggio(pitches, clip.length, s); }
            else if (s.generator === 'variation') notes = G.variation(clip.notes, clip.length, p.harmony, { ...s, lockPitch: s.lock_pitch, lockRhythm: s.lock_rhythm });
            else if (s.generator === 'rhythm') { if (!isDrum(track)) throw Error('invalid_project'); notes = G.rhythm(clip.length, s); }
            else if (s.generator === 'progression') { if (isDrum(track)) throw Error('invalid_project'); notes = G.progression(p.harmony, clip.length, s); }
            else if (s.generator === 'bassline') { if (isDrum(track)) throw Error('invalid_project'); notes = G.bassline(p.harmony, clip.length, s); }
            else if (s.generator === 'melody') { if (isDrum(track)) throw Error('invalid_project'); notes = G.melody(p.harmony, clip.length, s); }
            else if (s.generator === 'harmonize') { if (isDrum(track)) throw Error('invalid_project'); notes = G.harmonize(clip.notes, clip.length, p.harmony, s); }
            else if (s.generator === 'groove') notes = G.groove(clip.notes, clip.length, s);
            else if (s.generator === 'reverse') notes = G.reverseNotes(clip.notes, clip.length);
            else notes = clip.notes.map(n => ({ ...n, pitch: G.fit(n.pitch, p.harmony) }));
            const next = M.clone(p), target = next.tracks.find(t => t.id === track.id).clips.find(c => c.id === clip.id);
            target.notes = notes.map(n => ({ ...n, id: M.newID('note') }));
            M.validate(next);
            preview = { base: JSON.stringify(p), p: next, track: track.id, clip: clip.id };
            host.querySelector('.ss-preview').hidden = false;
            host.querySelector('[data-preview-state]').textContent = tr('preview') + ' · ' + target.notes.length + ' ' + tr('notes');
        }

        function isDrum(t) { return t && (t.instrument === 'gm-drums' || t.instrument === 'drums-kit' || t.instrument.startsWith('drum-')); }

        function compare(variant) {
            if (!preview) return;
            options.stopTransport(); previewAudio.stop();
            const p = M.clone(variant ? preview.p : JSON.parse(preview.base)), t = p.tracks.find(t => t.id === preview.track), c = t.clips.find(c => c.id === preview.clip);
            p.tracks = [t]; t.clips = [c]; const start = c.start;
            for (const parameter of ['tone', 'reverb']) for (const point of t.automation) if (point.parameter === parameter && point.tick <= start) t.params[parameter] = point.value;
            t.automation = t.automation.filter(point => point.tick >= start && point.tick < start + c.length).map(point => ({ ...point, tick: point.tick - start }));
            c.start = 0; p.loop = { enabled: true, start: 0, end: c.length };
            const token = ++epoch;
            previewAudio.unlock().then(() => { if (token !== epoch) return; return previewAudio.play(p, 0); }).catch(options.fail);
            host.querySelectorAll('[data-compose="original"],[data-compose="variant"]').forEach(b => b.setAttribute('aria-pressed', String(b.dataset.compose === (variant ? 'variant' : 'original'))));
        }

        function pads() {
            const { p } = selection(), s = settings(), box = host.querySelector('.ss-chord-pads');
            const names = ['C', 'C♯', 'D', 'D♯', 'E', 'F', 'F♯', 'G', 'G♯', 'A', 'A♯', 'B'];
            const degrees = ['I', 'II', 'III', 'IV', 'V', 'VI', 'VII', 'I↑'];
            box.innerHTML = Array.from({ length: 8 }, (_, i) => '<button type="button" data-chord="' + i + '" style="--ss-chord-color:var(--ss-color-' + (i % 6) + ')" aria-pressed="' + (i === degree) + '"><strong>' + degrees[i] + '</strong><span>' + G.chord(p.harmony, i, !!s.seventh).map(n => names[n % 12]).join(' · ') + '</span></button>').join('');
            const degBox = host.querySelector('[data-gen-degrees]');
            if (degBox) {
                degBox.innerHTML = Array.from({ length: 8 }, (_, i) => '<button type="button" data-gen-degree="' + i + '" aria-pressed="' + (i === degree) + '">' + degrees[i] + ' · ' + G.chord(p.harmony, i, !!s.seventh).map(n => names[n % 12]).join('') + '</button>').join('');
            }
        }

        async function padStart(buttonEl) {
            stopLive();
            degree = Number(buttonEl.dataset.chord);
            host.querySelectorAll('[data-chord]').forEach(b => b.setAttribute('aria-pressed', String(b === buttonEl)));
            host.querySelectorAll('[data-gen-degree]').forEach(b => b.setAttribute('aria-pressed', String(Number(b.dataset.genDegree) === degree)));
            const { p, track } = selection(); if (!track) return;
            const pitches = G.chord(p.harmony, degree, !!settings().seventh), token = epoch;
            if (q('arp_live').checked) { pitches.forEach(p => hold('pad:' + p, p)); return; }
            try { await options.audio.unlock(); if (token !== epoch) return; padReleases = pitches.map(p => options.audio.noteOn(track, p, 0.75)); } catch (error) { options.fail(error); }
        }

        host.addEventListener('pointerdown', e => {
            const b = e.target.closest('[data-chord]');
            if (b && e.button === 0) { e.preventDefault(); b.focus(); b.setPointerCapture(e.pointerId); padStart(b); }
        }, { signal });
        function releasePads() {
            if (!q('arp_latch').checked) { epoch++; stopLive(); return; }
            for (const key of [...held.keys()]) if (key.startsWith('pad:')) physical.delete(key);
        }
        host.addEventListener('pointerup', e => { if (e.target.closest('[data-chord]')) releasePads(); }, { signal });
        host.addEventListener('pointercancel', () => { epoch++; stopLive(); }, { signal });
        host.addEventListener('keydown', e => { const b = e.target.closest('[data-chord]'); if (b && ['Enter', ' '].includes(e.key)) { e.preventDefault(); if (!e.repeat) padStart(b); } }, { signal });
        host.addEventListener('keyup', e => { if (e.target.closest('[data-chord]') && ['Enter', ' '].includes(e.key)) { e.preventDefault(); releasePads(); } }, { signal });

        host.addEventListener('click', e => {
            const pick = e.target.closest('[data-gen-pick]');
            if (pick) {
                const genSelect = q('generator');
                if (genSelect) {
                    genSelect.value = pick.dataset.genPick;
                    genSelect.dispatchEvent(new Event('change', { bubbles: true }));
                }
                return;
            }
            const deg = e.target.closest('[data-gen-degree]');
            if (deg) {
                degree = Number(deg.dataset.genDegree);
                host.querySelectorAll('[data-chord]').forEach(b => b.setAttribute('aria-pressed', String(Number(b.dataset.chord) === degree)));
                host.querySelectorAll('[data-gen-degree]').forEach(b => b.setAttribute('aria-pressed', String(Number(b.dataset.genDegree) === degree)));
                return;
            }
            const mirror = e.target.closest('[data-arp-mirror]');
            if (mirror) {
                const target = q(mirror.dataset.arpMirror);
                if (target && !target.disabled) {
                    target.checked = !target.checked;
                    target.dispatchEvent(new Event('change', { bubbles: true }));
                }
                return;
            }
            if (e.target.closest('[data-jump-arp]')) {
                options.showCompose?.();
                const genSelect = q('generator');
                if (genSelect) {
                    genSelect.value = 'arp';
                    genSelect.dispatchEvent(new Event('change', { bubbles: true }));
                }
                return;
            }
            const action = e.target.closest('[data-compose]')?.dataset.compose;
            try {
                if (action === 'generate') generate();
                if (action === 'cancel') cancel();
                if (action === 'original' || action === 'variant') compare(action === 'variant');
                if (action === 'apply' && preview && !options.blocked()) {
                    const current = preview;
                    if (current.base !== JSON.stringify(options.project())) { cancel(); return; }
                    const notes = current.p.tracks.find(t => t.id === current.track).clips.find(c => c.id === current.clip).notes;
                    cancel();
                    if (options.mutate(p => { p.tracks.find(t => t.id === current.track).clips.find(c => c.id === current.clip).notes = notes; })) options.showNotes();
                }
            } catch (error) { options.fail(error); }
        }, { signal });

        host.addEventListener('input', e => { const out = e.target.parentElement.querySelector('output'); if (out) out.textContent = e.target.value; }, { signal });
        host.addEventListener('change', e => {
            if (e.target.hasAttribute('data-compose-clip')) {
                const { track } = selection();
                if (track && e.target.value) options.onSelectClip?.(track.id, e.target.value);
                return;
            }
            if (!e.target.dataset.music) return;
            cancel(); stopLive();
            const s = settings();
            if (['root', 'scale', 'scale_snap'].includes(e.target.dataset.music)) options.mutate(p => { p.harmony = { root: s.root, scale: s.scale, snap: s.scale_snap }; }, 'metadata');
            pads(); refresh();
        }, { signal });

        function syncGeneratorNav(generator, track, clip) {
            const composeEl = host.querySelector('.ss-compose');
            if (composeEl) composeEl.dataset.activeGenerator = generator;
            const detail = composeEl?.querySelector('details[data-gen-panel="' + generator + '"]');
            if (detail) detail.open = true;
            const drum = isDrum(track), hasNotes = !!clip?.notes.length;
            host.querySelectorAll('[data-gen-pick]').forEach(btn => {
                const id = btn.dataset.genPick;
                btn.setAttribute('aria-pressed', String(id === generator));
                const badge = btn.querySelector('[data-gen-badge]');
                if (!badge) return;
                let note = '';
                if (!clip) note = '—';
                else if (id === 'rhythm' && !drum) note = tr('drums');
                else if (['progression', 'bassline', 'melody', 'harmonize'].includes(id) && drum) note = tr('piano_roll');
                else if (['groove', 'reverse', 'harmonize'].includes(id) && !hasNotes) note = tr('notes');
                badge.textContent = note;
                btn.classList.toggle('is-incompat', !!note);
            });
        }

        function refresh() {
            const { p, track, clip } = selection();
            if (preview && preview.base !== JSON.stringify(p)) cancel();
            if (lastTrack !== track?.id) { epoch++; stopLive(); lastTrack = track?.id; }
            q('root').value = p.harmony.root;
            q('scale').value = p.harmony.scale;
            q('scale_snap').checked = p.harmony.snap;
            host.querySelector('[data-compose-target]').textContent = clip?.name || tr('select_clip');

            const clipSel = host.querySelector('[data-compose-clip]');
            if (clipSel) {
                const editableClips = (track?.clips || []).filter(c => !c.audio);
                clipSel.innerHTML = editableClips.length
                    ? editableClips.map(c => '<option value="' + esc(c.id) + '"' + (c.id === clip?.id ? ' selected' : '') + '>' + esc(c.name) + '</option>').join('')
                    : '<option value="">' + esc(tr('select_clip')) + '</option>';
                clipSel.disabled = options.blocked() || !editableClips.length;
            }

            const generator = q('generator').value;
            const noSource = (generator === 'groove' || generator === 'reverse' || generator === 'harmonize') && !clip?.notes.length;
            const tonalOnly = (generator === 'progression' || generator === 'bassline' || generator === 'melody' || generator === 'harmonize') && isDrum(track);
            host.querySelector('[data-compose=generate]').disabled = options.blocked() || !clip || noSource || (generator === 'rhythm' && !isDrum(track)) || tonalOnly;
            host.querySelector('[data-compose=apply]').disabled = options.blocked();
            host.querySelectorAll('input,select').forEach(e => e.disabled = options.blocked());
            if (clipSel) clipSel.disabled = options.blocked() || !((track?.clips || []).some(c => !c.audio));

            syncGeneratorNav(generator, track, clip);
            for (const key of ['arp_live', 'arp_latch']) {
                const mir = host.querySelector('[data-arp-mirror="' + key + '"]');
                if (mir) { mir.setAttribute('aria-pressed', String(!!q(key)?.checked)); mir.disabled = options.blocked(); }
            }
            const jump = host.querySelector('[data-jump-arp]');
            if (jump) jump.textContent = '1/' + q('division').value + ' · ' + tr(q('order').value);

            const h = JSON.stringify([p.harmony, settings().seventh]);
            if (h !== lastHarmony) { lastHarmony = h; pads(); }
            host.querySelectorAll('[data-chord],[data-gen-degree]').forEach(e => e.disabled = options.blocked());
        }

        signal.addEventListener('abort', () => { stop(); previewAudio.dispose(); }, { once: true });
        refresh();
        return { refresh, stop, hold, release, enabled: () => q('arp_live').checked };
    }
    window.SynthStudioCompose = { create };
})();
