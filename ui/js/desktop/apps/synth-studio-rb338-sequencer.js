        // ---- T8: 303 editor, drum steps, taps ---------------------------------------------------
        const now = () => (typeof performance !== 'undefined' ? performance.now() : Date.now());
        const stepMsg = (r, p, step, field, value) => M.encodeDevice({ type: 'step', dev: M.ROW_LAYOUT[r].dev, p, step, field, value });
        const canEdit = () => ready() && !lock();
        // A pattern held back by the deferred Undo goes at once when it is selected or edited (link.rs).
        function sendDeferred() {
            if (!deferred) return;
            const d = deferred;
            deferred = null;
            emit(M.encodeDevice({ type: 'pattern', dev: M.ROW_LAYOUT[d.row].dev, p: d.pattern, bytes: d.bytes }));
        }
        function touchDeferred(r, p) { if (deferred && deferred.row === r && deferred.pattern === p) sendDeferred(); }
        function followDeferred() {
            if (!deferred || !snap) return;
            if (snap.appliedSeq >= deferred.after && (!snap.playing || snap.playingPattern[deferred.row] !== deferred.pattern)) sendDeferred();
        }
        // Writes `bytes` into pattern p of row r: mirror and wire. False when it held them already.
        function writePatternBytes(r, p, bytes) {
            const same = M.patternBytes(mirror, r, p);
            let equal = same.length === bytes.length;
            for (let i = 0; equal && i < same.length; i++) if (same[i] !== bytes[i]) equal = false;
            if (equal) return false;
            mirror.set(bytes, M.patternOffset(r, p));
            emit(M.encodeDevice({ type: 'pattern', dev: M.ROW_LAYOUT[r].dev, p, bytes }));
            return true;
        }
        function moveEdit(r, d) { rows[r].editStep = (((rows[r].editStep + d) % 16) + 16) % 16; }
        // One press on a 303 editor control (model.rs press_303): key0..key12, slide, accent, up, down,
        // note_pause, step, back, pitch, clear.
        function press303(r, name) {
            if (!canEdit() || r < 0 || r > 1) return;
            const R = rows[r], p = curPattern(r), edit = R.editStep;
            touchDeferred(r, p);
            let sent = null;
            const flip = field => { sent = [field, M.stepField(mirror, r, p, edit, field) ? 0 : 1]; };
            if (/^key\d+$/.test(name)) {
                const note = clamp(Number(name.slice(3)), 0, 12);
                sent = [0, note];
                R.flashUntil = now() + 400;
                if (R.pitch) moveEdit(r, 1);
            } else if (name === 'slide') flip(1);
            else if (name === 'accent') flip(2);
            else if (name === 'up') flip(3);
            else if (name === 'down') flip(4);
            else if (name === 'note_pause') flip(5);
            else if (name === 'step') moveEdit(r, 1);
            else if (name === 'back') moveEdit(r, -1);
            else if (name === 'pitch') { R.pitch = !R.pitch; if (R.pitch) R.editStep = 0; }
            else if (name === 'clear') {
                const bytes = M.emptiedPattern(r, M.patternBytes(mirror, r, p));
                mirror.set(bytes, M.patternOffset(r, p));
                emit(M.encodeDevice({ type: 'pattern', dev: M.ROW_LAYOUT[r].dev, p, bytes }));
                markModified();
                return;
            }
            if (!sent) return;
            M.setStepField(mirror, r, p, edit, sent[0], sent[1]);
            emit(stepMsg(r, p, edit, sent[0], sent[1]));
            markModified();
        }
        function putStep808(inst, step, on) {
            const r = 2, p = curPattern(r), v = on ? 1 : 0;
            if (M.stepField(mirror, r, p, step, inst) === v) return false;
            touchDeferred(r, p);
            M.setStepField(mirror, r, p, step, inst, v);
            emit(stepMsg(r, p, step, inst, v));
            markModified();
            return true;
        }
        function putStep909(inst, step, value) {
            const r = 3, p = curPattern(r);
            if (M.stepField(mirror, r, p, step, inst) === value) return false;
            touchDeferred(r, p);
            const msgs = [];
            M.setStepField(mirror, r, p, step, inst, value);
            msgs.push(stepMsg(r, p, step, inst, value));
            const other = M.otherHat909(inst, value);
            if (other >= 0 && M.stepField(mirror, r, p, step, other) !== 0) {
                M.setStepField(mirror, r, p, step, other, 0);
                msgs.push(stepMsg(r, p, step, other, 0));
            }
            emitMany(msgs);
            markModified();
            return true;
        }
        // Mouse down on a drum step: the new state, then a drag paints the crossed steps with it.
        function stepDown(r, step, mods) {
            const inst = rows[r].inst, p = curPattern(r);
            const cur = M.stepField(mirror, r, p, step, inst);
            if (r === 2) {
                const on = cur === 0;
                putStep808(inst, step, on);
                return { row: r, inst, value: on ? 1 : 0, last: step };
            }
            const value = M.tr909StepValue(cur, inst === 0, !!vals['tr909.flam'], mods);
            putStep909(inst, step, value);
            return { row: r, inst, value, last: step };
        }
        function paintTo(d, step) {
            if (step === d.last) return;
            const dir = step > d.last ? 1 : -1;
            for (let s = d.last + dir; ; s += dir) {
                if (d.row === 2) putStep808(d.inst, s, d.value !== 0); else putStep909(d.inst, s, d.value);
                if (s === step) break;
            }
            d.last = step;
        }
        const modsOf = e => ({ ctrl: !!(e && (e.ctrlKey || e.metaKey)), alt: !!(e && e.altKey) });
        // model.rs tap_303 / tap_drum: set what plays right now.
        function playingStep(r) {
            if (!snap || !snap.playing) return -1;
            const s = snap.playStep[r];
            return s >= 0 && s < 16 ? s : -1;
        }
        function tap303(r, rest) {
            if (!canEdit()) return;
            const step = playingStep(r);
            if (step < 0) return;
            const p = curPattern(r), v = rest ? 1 : 0;
            if (M.stepField(mirror, r, p, step, 5) === v) return;
            touchDeferred(r, p);
            M.setStepField(mirror, r, p, step, 5, v);
            emit(stepMsg(r, p, step, 5, v));
            markModified();
        }
        function tapDrum(is808, inst) {
            if (!canEdit()) return;
            const r = is808 ? 2 : 3, step = playingStep(r);
            if (step < 0) return;
            if (is808) putStep808(inst, step, true); else putStep909(inst, step, M.tapDrumValue(false, inst));
        }
        function selectPattern(r, p) { focusRow = r; setControl(M.ROW_LAYOUT[r].section + '.pattern', p); }

        behaviours.note = b => press303(Number(b.dataset.row), 'key' + b.dataset.note);
        behaviours.flag = b => press303(Number(b.dataset.row), b.dataset.flag);
        behaviours.note_pause = b => press303(Number(b.dataset.row), 'note_pause');
        behaviours.back = b => press303(Number(b.dataset.row), 'back');
        behaviours.step_next = b => press303(Number(b.dataset.row), 'step');
        behaviours.pitch = b => press303(Number(b.dataset.row), 'pitch');
        behaviours.clear = b => press303(Number(b.dataset.row), 'clear');
        // Drum steps act on pointerdown (so a drag can paint); a keyboard click (detail 0) toggles here.
        behaviours.step = (b, e) => {
            const r = Number(b.dataset.row);
            if (r < 2 || (e && e.detail !== 0) || !canEdit()) return;
            stepDown(r, Number(b.dataset.step), modsOf(e));
        };

        // drag state: paint (drum steps) or legend (instrument selection while held)
        on(section, 'pointerdown', e => {
            if (e.button !== 0 || !e.target.closest) return;
            const st = e.target.closest('.ss-rb-step');
            if (st && !st.disabled && Number(st.dataset.row) >= 2) {
                if (canEdit()) paint = Object.assign({ kind: 'paint' }, stepDown(Number(st.dataset.row), Number(st.dataset.step), modsOf(e)));
                return;
            }
            const lg = e.target.closest('[data-rb-act=inst]');
            if (lg) { rows[Number(lg.dataset.row)].inst = Number(lg.dataset.inst); paint = { kind: 'legend', row: Number(lg.dataset.row) }; }
        });
        on(window, 'pointermove', e => {
            if (!paint) return;
            const el = typeof document.elementFromPoint === 'function' ? document.elementFromPoint(e.clientX, e.clientY) : null;
            if (!el || !el.closest) return;
            if (paint.kind === 'legend') {
                const lg = el.closest('[data-rb-act=inst]');
                if (lg && Number(lg.dataset.row) === paint.row) rows[paint.row].inst = Number(lg.dataset.inst);
                return;
            }
            const st = el.closest('.ss-rb-step');
            if (st && Number(st.dataset.row) === paint.row) paintTo(paint, Number(st.dataset.step));
        });
        const endPaint = () => { paint = null; };
        on(window, 'pointerup', endPaint);
        on(window, 'pointercancel', endPaint);

        // ---- T8: keyboard (ui_spec section 10) --------------------------------------------------
        function typing(t) {
            if (!t || !t.tagName) return false;
            if (t.isContentEditable) return true;
            const n = t.tagName;
            if (n === 'TEXTAREA' || n === 'SELECT') return true;
            if (n === 'INPUT') return ['range', 'checkbox', 'radio', 'button', 'submit'].indexOf((t.type || 'text').toLowerCase()) < 0;
            return false;
        }
        function runKey(a) {
            switch (a.type) {
            case 'transport': press(a.name); break;
            case 'focus': {
                focusRow = (focusRow + a.delta + 4) % 4;
                const plate = $('.ss-rb-row[data-row="' + focusRow + '"] > .ss-rb-device');
                if (plate && plate.scrollIntoView) plate.scrollIntoView({ block: 'nearest' });
                break;
            }
            case 'scroll': section.scrollTop += a.delta * Math.max(80, section.clientHeight * 0.85); break;
            case 'pattern': selectPattern(a.row, M.patternIndex(rows[a.row].bank, a.button)); break;
            case 'note303': press303(focusRow, 'key' + a.note); break;
            case 'step303': press303(focusRow, a.name); break;
            case 'tap303': tap303(focusRow, a.rest); break;
            case 'tapDrum': tapDrum(focusRow === 2, a.inst); break;
            case 'edit': patternCommand(a.cmd); break;
            case 'save': if (actions['rb-save']) actions['rb-save'](); break;
            default: break;
            }
        }
        // The window this rack lives in (the Synth Studio root). Only the window that holds the focus reacts.
        const appRoot = () => o.root || (section.closest ? section.closest('.ss-app') : null) || section;
        const FOCUSABLE = 'button, a, summary, input, select, textarea, [role=button], [role=tab], [role=checkbox], [role=switch], [role=menuitem], [role=link], [role=slider]';
        // Anything that keeps a key of its own: Space/Enter activate it natively, Tab moves on from it.
        function focusable(t) {
            if (!t || !t.closest || t === document.body || t === document.documentElement || t === section || t === appRoot()) return false;
            return !!t.closest(FOCUSABLE);
        }
        // A range input has no native Space/Enter action, so those keys still reach the transport from a knob.
        const nativeActivation = t => focusable(t) && !(t.type === 'range');
        on(window, 'keydown', e => {
            if (!active || disposed || e.defaultPrevented || e.isComposing || e.altKey || !ready()) return;
            const t = e.target, ae = document.activeElement, win = appRoot();
            if (!win || !ae || !win.contains(ae)) return;
            const combo = e.ctrlKey || e.metaKey;
            if (combo && !section.contains(ae)) return;
            if (typing(t)) return;
            if (t && t.closest && t.closest('dialog, [role=dialog], [aria-modal=true]')) return;
            if (t && t.type === 'range' && /^(Arrow|Page|Home|End)/.test(e.code)) return;
            if ((e.code === 'Space' || e.code === 'Enter') && nativeActivation(t)) return;
            const a = M.keyAction(e.code, { shift: e.shiftKey, ctrl: combo, mode: keyMode, focusRow, playing: transport.playing, bare: !focusable(t) });
            if (!a) return;
            e.preventDefault();
            handledKeys.add(e.code);
            if (!e.repeat) runKey(a);
        });
        on(window, 'blur', () => { handledKeys.clear(); });
        // a handled Space/Enter must not click the focused button on release either
        on(window, 'keyup', e => { if (handledKeys.delete(e.code)) e.preventDefault(); });
        function setKeyMode(mode) {
            keyMode = mode === 'patterns' ? 'patterns' : 'synth';
            const b = $('[data-action="rb-keys"]');
            if (b) {
                const label = tr(keyMode === 'patterns' ? 'rb_keys_patterns' : 'rb_keys_synth');
                b.setAttribute('aria-pressed', keyMode === 'patterns' ? 'true' : 'false');
                b.title = label;
                b.setAttribute('aria-label', tr('rb_keys') + ': ' + label);
                const sp = b.querySelector('span');
                if (sp) sp.textContent = label;
            }
        }

        // ---- T9: pattern menu (menu.rs copy / paste / duplicate / undo) -------------------------
        const patternName = p => String.fromCharCode(65 + (p >> 3)) + ((p & 7) + 1);
        const rowKind = r => (r < 2 ? 0 : r);
        function patternCommand(cmd) {
            if (!canEdit()) return;
            const r = focusRow, p = curPattern(r), R = M.ROW_LAYOUT[r];
            const status = text => say(R.name + ' ' + patternName(p) + ' \u00b7 ' + text);
            if (cmd === 'copy' || cmd === 'cut') {
                const bytes = M.patternBytes(mirror, r, p);
                clip = { row: r, bytes };
                if (cmd === 'cut' && writePatternBytes(r, p, M.emptiedPattern(r, bytes))) {
                    markModified();
                    undo = { what: 'cut', row: r, pattern: p, bytes, select: null };
                }
                status(tr(cmd === 'cut' ? 'rb_cut' : 'rb_copy'));
            } else if (cmd === 'paste') {
                if (!clip || rowKind(clip.row) !== rowKind(r)) return;
                const before = M.patternBytes(mirror, r, p);
                if (writePatternBytes(r, p, clip.bytes)) { markModified(); undo = { what: 'paste', row: r, pattern: p, bytes: before, select: null }; }
                status(tr('rb_paste_done'));
            } else if (cmd === 'duplicate') {
                const pl = M.ROW_PATTERN[r];
                const to = M.nextEmptyPattern(r, mirror.subarray(pl.base, pl.base + 32 * pl.plen), p);
                if (to === null) { say(R.name + ' \u00b7 ' + tr('rb_no_empty')); return; }
                const bytes = M.patternBytes(mirror, r, p), before = M.patternBytes(mirror, r, to);
                writePatternBytes(r, to, bytes);
                markModified();
                selectPattern(r, to);
                undo = { what: 'duplicate', row: r, pattern: to, bytes: before, select: p };
                say(R.name + ' ' + patternName(p) + ' \u2192 ' + patternName(to) + ' \u00b7 ' + tr('rb_dup_done'));
            } else if (cmd === 'undo') {
                if (!undo) return;
                const u = undo;
                undo = null;
                if (u.select !== null) selectPattern(u.row, u.select);
                if (curPattern(u.row) === u.pattern) {
                    if (writePatternBytes(u.row, u.pattern, u.bytes)) markModified();
                } else {
                    // the engine may still play it: it goes once the engine stops playing it (link.rs follow)
                    sendDeferred();
                    flush();
                    mirror.set(u.bytes, M.patternOffset(u.row, u.pattern));
                    deferred = { row: u.row, pattern: u.pattern, bytes: u.bytes, after: lastSent };
                    markModified();
                    followDeferred();
                }
                say(M.ROW_LAYOUT[u.row].name + ' ' + patternName(u.pattern) + ' \u00b7 ' + tr('rb_undo'));
            }
        }

        // ---- T9: song document ------------------------------------------------------------------
        const NAMESPACE = 'synth-studio';
        const origin = () => (typeof location !== 'undefined' ? location.origin : '');
        // One draft per Synth Studio window; the old shared key is read once as a migration fallback.
        const draftKey = () => origin() + ':synth-studio:rb338:' + (o.windowId ? 'window:' + o.windowId : 'current');
        const legacyDraftKey = () => origin() + ':synth-studio:rb338:current';
        const draftApi = () => (root.OfficeSession && typeof root.OfficeSession.draft === 'function' ? root.OfficeSession : null);
        const fileBase = () => {
            const raw = (info && info.title ? String(info.title) : '') || docName || 'rb338-song';
            const clean = raw.replace(/[\\/:*?"<>|\x00-\x1f]+/g, '-').replace(/^[\s.-]+|[\s.]+$/g, '').slice(0, 60);
            return clean || 'rb338-song';
        };
        function download(blob, name) {
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = name;
            a.rel = 'noopener';
            document.body.appendChild(a);
            a.click();
            a.remove();
            setTimeout(() => URL.revokeObjectURL(url), 4000);
        }
        async function saveBytes() {
            if (!ready() || typeof host.save !== 'function') throw new Error('not ready');
            return bytesOf(await host.save());
        }
        async function confirmReplace(titleKey) {
            if (!modified) return true;
            const text = tr('rb_replace_confirm');
            const r = typeof o.confirm === 'function' ? o.confirm(tr(titleKey), text) : undefined;
            if (r === undefined) return typeof window !== 'undefined' && typeof window.confirm === 'function' ? window.confirm(text) : true;
            return !!(await r);
        }
        function dropPending() { outbox.length = 0; flushToken++; held.clear(); }
        const LOAD_ERR = { 1: 'rb_err_not_song', 2: 'rb_err_convert', 3: 'rb_err_memory', 21: 'rb_err_convert' };
        async function openBytes(bytes, name) {
            if (!ready()) return false;
            // The worker pre-validates the song, so a refused or crashing file never touches the live engine.
            const Host = root.SynthStudioRB338Host;
            if (Host && typeof Host.checkSong === 'function') {
                let chk;
                try { chk = await Host.checkSong(bytes); } catch (_) { chk = { ok: false, code: 21 }; }
                if (disposed || !ready()) return false;
                if (!chk || chk.ok === false) { say(tr(LOAD_ERR[chk && chk.code] || 'rb_err_not_song')); return false; }
            }
            dropPending();
            let res;
            try { res = await host.load(bytes); } catch (_) { say(tr('rb_err_memory')); return false; }
            if (!res || res.ok === false) { say(tr(LOAD_ERR[res && res.code] || 'rb_err_not_song')); return false; }
            docName = String(name || '').replace(/\.rbs$/i, '');
            titleLock = '';
            say('');
            return true;
        }
        async function openFile(file) {
            if (!file || !ready()) return;
            if (file.size > 32 * 1024 * 1024) { say(tr('rb_err_not_song')); return; }
            let bytes;
            try { bytes = new Uint8Array(await file.arrayBuffer()); } catch (_) { say(tr('rb_err_not_song')); return; }
            if (!(await confirmReplace('rb_open'))) return;
            await openBytes(bytes, file.name);
        }
        let fileInput = null;
        function pickFile() {
            if (!fileInput) {
                fileInput = document.createElement('input');
                fileInput.type = 'file';
                fileInput.accept = '.rbs,.RBS';
                fileInput.hidden = true;
                fileInput.addEventListener('change', () => { const f = fileInput.files && fileInput.files[0]; fileInput.value = ''; if (f) openFile(f); }, { signal: sig });
                section.appendChild(fileInput);
            }
            fileInput.click();
        }
        async function newSong() {
            if (!ready()) return;
            if (!(await confirmReplace('rb_new'))) return;
            dropPending();
            try { await host.newSong(); docName = ''; titleLock = ''; say(''); } catch (_) { say(tr('rb_err_memory')); }
        }
        async function saveSong() {
            if (!ready()) return;
            const ticket = editSeq;
            let bytes;
            try { bytes = await saveBytes(); } catch (_) { say(tr('rb_err_export')); return; }
            download(new Blob([bytes], { type: 'application/octet-stream' }), fileBase() + '.rbs');
            if (editSeq === ticket) modified = false; // an edit made while the engine answered stays unsaved
            draftDirty = true;
            scheduleAutosave();
        }
        // drag and drop a .rbs onto the rack
        const hasFiles = e => !!(e.dataTransfer && Array.from(e.dataTransfer.types || []).indexOf('Files') >= 0);
        on(section, 'dragover', e => { if (!hasFiles(e)) return; e.preventDefault(); section.classList.add('is-drop'); });
        on(section, 'dragleave', e => { if (!e.relatedTarget || !section.contains(e.relatedTarget)) section.classList.remove('is-drop'); });
        on(section, 'drop', e => {
            if (!hasFiles(e)) return;
            e.preventDefault();
            section.classList.remove('is-drop');
            const f = e.dataTransfer.files && e.dataTransfer.files[0];
            if (f) openFile(f);
        });

        // WAV export: the live engine's song as saved, rendered in the Worker (no pre-roll).
        async function exportWav() {
            const Host = root.SynthStudioRB338Host;
            if (!ready() || exporting || !Host || typeof Host.renderer !== 'function') return;
            exporting = true;
            section.dataset.rbBusy = 'export';
            const btn = $('[data-action="rb-export"]');
            if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
            say(tr('rb_export_wav') + '\u2026');
            let rend = null;
            try {
                const song = await saveBytes();
                rend = Host.renderer();
                const out = await rend.render({ song: song.slice().buffer, mode: transport.songMode ? 'song' : 'pattern', bars: exportBars, preroll: false });
                download(new Blob([out.wav], { type: 'audio/wav' }), fileBase() + '.wav');
                say('');
            } catch (_) {
                say(tr('rb_err_export'));
            } finally {
                if (rend) rend.close();
                exporting = false;
                delete section.dataset.rbBusy;
                if (btn) { btn.disabled = false; btn.removeAttribute('aria-busy'); }
            }
        }

        // ---- T11: Send to Arrange ---------------------------------------------------------------
        // Renders the live song in the Worker (pattern mode with the seamless pre-roll cycle), then hands
        // the PCM plus its recipe to Synth Studio, which owns the project mutation.
        let sending = false;
        async function sendToArrange() {
            const Host = root.SynthStudioRB338Host;
            if (!ready() || sending || exporting || typeof o.sendToArrange !== 'function') return;
            if (!Host || typeof Host.renderer !== 'function' || typeof Host.wavToBuffer !== 'function') { say(tr('rb_clip_failed')); return; }
            sending = true;
            section.dataset.rbBusy = 'arrange';
            const btn = $('[data-action="rb-send-arrange"]');
            if (btn) { btn.disabled = true; btn.setAttribute('aria-busy', 'true'); }
            say(tr('rb_clip_rendering'));
            let rend = null;
            try {
                const song = await saveBytes();
                const mode = transport.songMode ? 'song' : 'pattern';
                const bars = exportBars;
                rend = Host.renderer();
                const out = await rend.render({ song: song.slice().buffer, mode, bars, preroll: mode === 'pattern' });
                const buffer = Host.wavToBuffer(out.wav);
                let peak = 0;
                for (let c = 0; c < buffer.numberOfChannels; c++) {
                    const d = buffer.getChannelData(c);
                    for (let i = 0; i < d.length; i++) { const v = d[i] < 0 ? -d[i] : d[i]; if (v > peak) peak = v; }
                }
                if (peak === 0) { say(tr('rb_clip_empty')); return; }
                say('');
                await o.sendToArrange({ song, mode, start: 0, end: bars * 768, frames: out.frames || buffer.length, tempo: transport.tempo / 1000, buffer, name: fileBase() });
            } catch (_) {
                say(tr('rb_clip_failed'));
            } finally {
                if (rend) rend.close();
                sending = false;
                delete section.dataset.rbBusy;
                if (btn) { btn.disabled = false; btn.removeAttribute('aria-busy'); }
            }
        }
        // Loads a clip's song into the rack (Edit in RB-338 on an audio clip). The engine may still be
        // starting when the tab was just opened, so wait for it; the replace confirm applies as usual.
        async function openSong(bytes, name) {
            for (let i = 0; i < 200 && !ready() && hostState !== 'error' && !disposed; i++) await new Promise(r => setTimeout(r, 100));
            if (!ready() || disposed) return false;
            if (!(await confirmReplace('rb_open'))) return false;
            return openBytes(bytes instanceof Uint8Array ? bytes : new Uint8Array(bytes), name);
        }

        // ---- T9: autosave, restore, crash recovery ----------------------------------------------
        function scheduleAutosave() {
            if (autoTimer || disposed) return;
            autoTimer = setTimeout(() => { autoTimer = 0; autosave(); }, 1500);
        }
        // Writes the current song to the draft store; `h` is the host to ask (it may be closing).
        // Returns the promise of the write in flight (or a settled one when there is nothing to write).
        function autosave(h) {
            if (autoTimer) { clearTimeout(autoTimer); autoTimer = 0; }
            const target = h || host;
            const api = draftApi();
            if (!draftDirty || !api || !target || (!h && !ready())) return autoInflight || Promise.resolve();
            if (autoBusy) { autoAgain = true; return autoInflight; }
            autoBusy = true;
            // an edit made while the write runs sets draftDirty again and so schedules the next one
            draftDirty = false;
            autoInflight = (async () => {
                try {
                    const bytes = bytesOf(await target.save());
                    const rec = { id: draftId, bytes, name: fileBase() === 'rb338-song' ? docName : fileBase(), modified, updated: Date.now() };
                    await api.draft('put', draftKey(), rec, NAMESPACE);
                    lastSaved = rec;
                    autoSavedAt = Date.now();
                } catch (_) {
                    draftDirty = true;
                } finally {
                    autoBusy = false;
                    autoInflight = null;
                    if (autoAgain) { autoAgain = false; if (draftDirty && !disposed) scheduleAutosave(); }
                }
            })();
            return autoInflight;
        }
        // Everything still owed to the draft store: the write in flight, then the edits made meanwhile.
        async function flushDraft(h) {
            if (autoInflight) { try { await autoInflight; } catch (_) { /* reported by autosave */ } }
            if (draftDirty) await autosave(h);
        }
        function readDraft() {
            const api = draftApi();
            if (!api || typeof indexedDB === 'undefined') { draftState = 'done'; return; }
            draftState = 'reading';
            let p;
            const fetchDraft = key => { try { return Promise.resolve(api.draft('get', key, undefined, NAMESPACE)).catch(() => null); } catch (_) { return Promise.resolve(null); } };
            const usable = rec => (rec && rec.bytes && rec.bytes.byteLength ? rec : null);
            p = fetchDraft(draftKey()).then(rec => {
                if (usable(rec) || !o.windowId) return rec;
                // the shared key is migrated by the first window to ask; later windows start empty
                return fetchDraft(legacyDraftKey()).then(old => {
                    if (usable(old)) { try { Promise.resolve(api.draft('delete', legacyDraftKey(), undefined, NAMESPACE)).catch(() => {}); } catch (_) { /* kept */ } }
                    return old;
                });
            });
            draftPromise = p.then(rec => { draftRec = rec && rec.bytes && rec.bytes.byteLength ? rec : null; }, () => {}).then(() => { draftState = 'done'; if (host) lateRestore(); });
        }
        // The draft arrived after the host was created: load it once the engine is ready and nothing was edited.
        function lateRestore() {
            if (restoreUsed || draftState !== 'done' || !draftRec || !ready()) return;
            if (modified || draftDirty) return;
            restoreUsed = true;
            const rec = draftRec;
            const wasModified = !!rec.modified;
            openBytes(bytesOf(rec.bytes), rec.name || '').then(ok => {
                if (!ok) return;
                modified = wasModified;
                draftDirty = false;
                if (autoTimer) { clearTimeout(autoTimer); autoTimer = 0; }
                lastSaved = rec;
            });
        }
        // Crash recovery: a new live host (its AudioContext is made in this click) from the last autosave.
        function restart() {
            if (disposed) return;
            const old = host;
            host = null;
            if (old) { try { old.close(); } catch (_) { /* closed */ } }
            hostState = 'idle';
            outbox.length = 0;
            flushToken++;
            seq = 0;
            lastSent = 0;
            snap = null;
            lastMerges = null;
            undo = null;
            deferred = null;
            transport = M.createTransport();
            setLoadText(tr('rb_loading'), false);
            const rec = lastSaved;
            restoreModified = !!(rec && rec.modified);
            if (rec) docName = String(rec.name || docName || '');
            ensureHost(rec ? bytesOf(rec.bytes) : undefined);
        }

        // ---- T9: UI prefs -----------------------------------------------------------------------
        const PREFS_KEY = 'aurago:synth-studio:rb338:1';
        function loadPrefs() {
            try {
                const raw = typeof localStorage !== 'undefined' ? localStorage.getItem(PREFS_KEY) : null;
                const p = raw ? JSON.parse(raw) : null;
                if (!p || typeof p !== 'object') return;
                if (p.keys === 'patterns' || p.keys === 'synth') keyMode = p.keys;
                if (Number.isInteger(p.focus)) focusRow = clamp(p.focus, 0, 3);
                if (Array.isArray(p.banks)) p.banks.slice(0, 4).forEach((v, i) => { if (Number.isInteger(v)) rows[i].bank = clamp(v, 0, 3); });
                if (Array.isArray(p.insts)) p.insts.slice(0, 4).forEach((v, i) => { if (Number.isInteger(v)) rows[i].inst = clamp(v, 0, 11); });
                if ([1, 2, 4, 8, 16].indexOf(p.bars) >= 0) exportBars = p.bars;
            } catch (_) { /* unreadable prefs: defaults */ }
        }
        function savePrefs() {
            prefsTimer = 0;
            try {
                if (typeof localStorage === 'undefined') return;
                localStorage.setItem(PREFS_KEY, JSON.stringify({ keys: keyMode, focus: focusRow, banks: rows.map(r => r.bank), insts: rows.map(r => r.inst), bars: exportBars }));
            } catch (_) { /* storage blocked or full */ }
        }
        function watchPrefs() {
            const sigNow = keyMode + focusRow + rows.map(r => r.bank + ':' + r.inst).join(',') + exportBars;
            if (sigNow === prefsSig) return;
            prefsSig = sigNow;
            if (!prefsTimer) prefsTimer = setTimeout(savePrefs, 400);
        }
        // Toolbar state: pattern menu availability, document status line.
        function renderToolbar() {
            const paste = $('[data-action="rb-paste"]'), und = $('[data-action="rb-undo"]');
            if (paste) { const d = !clip || rowKind(clip.row) !== rowKind(focusRow); if (paste._d !== d) { paste._d = d; paste.disabled = d; } }
            const dm = $('[data-action="rb-demos"]');
            if (dm) { const d = !ready() || demoBusy; if (dm._d !== d) { dm._d = d; dm.disabled = d; if (d) closeDemos(false); } }
            if (und) { const d = !undo; if (und._d !== d) { und._d = d; und.disabled = d; } }
            const st = $('[data-rb-docstate]');
            if (st) {
                const name = titleLock || (info && info.title ? String(info.title) : '') || docName;
                setText(st, (name ? name + ' \u00b7 ' : '') + (modified ? tr('rb_unsaved') : autoSavedAt ? tr('rb_autosaved') : ''));
            }
            const bars = $('[data-rb-bars]');
            if (bars && bars._v !== exportBars) { bars._v = exportBars; bars.value = String(exportBars); }
        }
        on(section, 'change', e => {
            const t = e.target;
            if (t && t.matches && t.matches('[data-rb-bars]')) { exportBars = Number(t.value) || 4; }
        });

        // ---- demo songs menu --------------------------------------------------------------------
        let titleLock = '', demoBusy = false, demoList = null, demoOpen = false, demoLoading = null;
        const demoBtn = () => $('[data-action="rb-demos"]');
        const demoMenu = () => $('.ss-rb-demomenu');
        const demoVersioned = url => {
            const lazy = root.AuraLazyAssets;
            if (lazy && typeof lazy.versionedURL === 'function') return lazy.versionedURL(url);
            return url + '?v=' + encodeURIComponent(root.BUILD_VERSION || 'dev');
        };
        const DEMO_BASE = '/js/desktop/apps/rb338-demos/';
        function loadDemoList() {
            if (demoList) return Promise.resolve(demoList);
            if (!demoLoading) {
                demoLoading = fetch(demoVersioned(DEMO_BASE + 'demos.json'), { signal: sig }).then(r => {
                    if (!r.ok) throw new Error('http ' + r.status);
                    return r.json();
                }).then(list => {
                    if (!Array.isArray(list) || !list.length) throw new Error('empty');
                    demoList = list;
                    return list;
                }).finally(() => { demoLoading = null; });
            }
            return demoLoading;
        }
        function demoItems() { const m = demoMenu(); return m ? Array.prototype.slice.call(m.querySelectorAll('[role=menuitem]')) : []; }
        function closeDemos(refocus) {
            const m = demoMenu(), b = demoBtn();
            demoOpen = false;
            if (m) m.hidden = true;
            if (b) b.setAttribute('aria-expanded', 'false');
            if (refocus && b && !b.disabled) b.focus();
        }
        async function toggleDemos() {
            if (demoOpen) { closeDemos(true); return; }
            if (!ready() || demoBusy) return;
            const m = demoMenu(), b = demoBtn();
            if (!m || !b) return;
            try { await loadDemoList(); } catch (_) { if (!disposed) say(tr('rb_demos_failed')); return; }
            if (disposed || !ready() || demoOpen) return;
            m.textContent = '';
            demoList.forEach((d, i) => {
                const it = document.createElement('button');
                it.type = 'button';
                it.setAttribute('role', 'menuitem');
                it.tabIndex = -1;
                it.dataset.demo = String(i);
                const t = document.createElement('b');
                t.textContent = String(d.title || '');
                const sub = document.createElement('small');
                sub.textContent = [d.style, d.bpm ? d.bpm + ' BPM' : ''].filter(Boolean).join(' \u00b7 ');
                it.appendChild(t);
                it.appendChild(sub);
                m.appendChild(it);
            });
            m.hidden = false;
            demoOpen = true;
            b.setAttribute('aria-expanded', 'true');
            const first = demoItems()[0];
            if (first) first.focus();
        }
        async function playDemo(d) {
            if (!d || !d.file || demoBusy || !ready()) return;
            closeDemos(true);
            demoBusy = true;
            section.dataset.rbBusy = 'demo';
            renderToolbar();
            try {
                let bytes;
                try {
                    const r = await fetch(demoVersioned(DEMO_BASE + String(d.file).replace(/^\/+/, '')), { signal: sig });
                    if (!r.ok) throw new Error('http ' + r.status);
                    bytes = new Uint8Array(await r.arrayBuffer());
                } catch (_) { if (!disposed) say(tr('rb_demos_failed')); return; }
                if (disposed || !(await confirmReplace('rb_demos'))) return;
                if (await openBytes(bytes, String(d.title || ''))) { titleLock = String(d.title || ''); modified = false; renderToolbar(); }
            } finally {
                demoBusy = false;
                section.removeAttribute('data-rb-busy');
                if (!disposed) renderToolbar();
            }
        }
        on(section, 'keydown', e => {
            if (!demoOpen) return;
            const items = demoItems();
            const i = items.indexOf(document.activeElement);
            let n = -1;
            if (e.key === 'ArrowDown') n = (i + 1) % items.length;
            else if (e.key === 'ArrowUp') n = (i <= 0 ? items.length : i) - 1;
            else if (e.key === 'Home') n = 0;
            else if (e.key === 'End') n = items.length - 1;
            else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); closeDemos(true); return; }
            else if (e.key === 'Tab') { closeDemos(false); return; }
            else return;
            e.preventDefault();
            e.stopPropagation();
            if (items[n]) items[n].focus();
        });
        on(section, 'click', e => {
            const it = e.target.closest ? e.target.closest('.ss-rb-demomenu [role=menuitem]') : null;
            if (it && demoList) playDemo(demoList[Number(it.dataset.demo)]);
        });
        on(document, 'pointerdown', e => {
            if (demoOpen && !(e.target.closest && e.target.closest('[data-rb-demos]'))) closeDemos(false);
        });

        // toolbar actions
        setAction('rb-new', newSong);
        setAction('rb-open', pickFile);
        setAction('rb-demos', toggleDemos);
        setAction('rb-save', saveSong);
        setAction('rb-export', exportWav);
        if (typeof o.sendToArrange === 'function') setAction('rb-send-arrange', sendToArrange);
        setAction('rb-keys', () => setKeyMode(keyMode === 'patterns' ? 'synth' : 'patterns'));
        ['cut', 'copy', 'paste', 'duplicate', 'undo'].forEach(cmd => setAction('rb-' + cmd, () => patternCommand(cmd)));
        loadPrefs();
        setKeyMode(keyMode);
        on(window, 'pagehide', () => { autosave(); });
        readDraft();

        // ---- api -------------------------------------------------------------------------------
        function setAction(name, fn) {
            actions[name] = fn;
            const b = $('[data-action="' + name + '"]');
            if (b) b.disabled = !fn;
        }
        function activate(next, opts) {
            if (disposed) return;
            const was = active;
            active = !!next;
            section.dataset.rbActive = active ? 'true' : 'false';
            if (active) {
                if (!host) {
                    // restored view: no AudioContext outside a gesture, the first click or key in the rack starts the engine
                    if (opts && opts.restored) awaitGesture(); else ensureHost();
                } else wake();
                renderAll();
                startLoop();
            } else {
                stopLoop();
                if (was) { autosave(); if (hooks.leave) hooks.leave(); }
                if (host && !transport.playing && typeof host.suspend === 'function') { try { host.suspend(); } catch (_) { /* closing */ } }
            }
            if (hooks.activate) hooks.activate(active);
        }
        function stop() { if (ready() && transport.playing) press('stop'); }
        function dispose() {
            if (disposed) return;
            const h = host;
            const last = h && ready() && draftApi() && (draftDirty || autoInflight) ? flushDraft(h) : null;
            disposed = true;
            if (hooks.leave) hooks.leave();
            stopLoop();
            stopRepeat();
            if (autoTimer) { clearTimeout(autoTimer); autoTimer = 0; }
            if (prefsTimer) { clearTimeout(prefsTimer); savePrefs(); }
            ac.abort();
            host = null;
            const closeHost = () => { if (h && typeof h.close === 'function') { try { h.close(); } catch (_) { /* already closed */ } } };
            // the engine must stay up until its last autosave answered
            if (last) { let done = false; const once = () => { if (!done) { done = true; closeHost(); } }; last.then(once, once); setTimeout(once, 1500); } else closeHost();
        }
        const api = { activate, stop, openSong, isModified: () => modified, isPlaying: () => !!(transport.playing || (snap && snap.playing)), dispose };
        // Internals for the later tasks (T8 behaviours/keys, T9 documents, T11 arrange): same file, no public contract.
        api.internals = { M, tr, E, section, rows, vals, mirror: () => mirror, setMirror, getVal, setControl, press, bump, toggle, emit, emitMany, flush, curPattern,
            transport: () => transport, snapshot: () => snap, host: () => host, state: () => hostState, info: () => info, setInfo, behaviours, actions, setAction, hooks,
            focusRow: () => focusRow, setFocusRow: r => { focusRow = r; }, markModified, isModified: () => modified, clearModified: () => { modified = false; }, renderAll, say, fmt, lock,
            press303, putStep808, putStep909, stepDown, paintTo, tap303, tapDrum, patternCommand, selectPattern, keyMode: () => keyMode, setKeyMode, undo: () => undo, clip: () => clip,
            openBytes, newSong, saveSong, saveBytes, exportWav, autosave, restart, docName: () => docName, fileBase };
        return api;
    }

    return { create, rackMarkup };
});
