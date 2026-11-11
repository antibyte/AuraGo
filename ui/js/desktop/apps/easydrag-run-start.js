// EasyDrag run start: the test dialog (trigger, sample data, real-effects warning, Simulation) and
// starting a test of the saved draft, one at a time. One of the four run parts that create() in
// easydrag-runs.js installs on its run core; it loads before easydrag-runs.js.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    // installStart installs the start part on the run core rc and returns its functions, which
    // create() puts on rc. Shared state is read and written as rc.<name>; functions of the other
    // parts are called through rc when they run, after every part is installed.
    function installStart(rc) {
        const { ed, canvas, core, t, esc, triggers, realEffects, effects, unknownEffects, EFFECTS_KEY, REDACTED, EFFECT_ICONS } = rc;
        const isRunning = (...args) => rc.isRunning(...args);
        const rootsFrom = (...args) => rc.rootsFrom(...args);
        const setRun = (...args) => rc.setRun(...args);
        const run = (...args) => rc.run(...args);
        const baseRevision = (...args) => rc.baseRevision(...args);
        const stepsFrom = (...args) => rc.stepsFrom(...args);

        // Real effects are confirmed one by one (sends_message, deletes, …): confirming the effects
        // of one step does not cover other effects of the flow, and only a test that started
        // confirms them. ed.effectsConfirmed holds this session's set; "remember" stores the list
        // under EFFECTS_KEY. An older stored `true` (one switch for the whole flow) counts as
        // nothing confirmed, so the warning comes once more.
        function storedEffects() {
            const stored = core.storage.get(EFFECTS_KEY + ed.flow.id, []);
            return Array.isArray(stored) ? stored.filter(x => typeof x === 'string') : [];
        }

        function confirmedEffects() {
            const set = new Set(storedEffects());
            if (ed.effectsConfirmed instanceof Set) ed.effectsConfirmed.forEach(x => set.add(x));
            return set;
        }

        function confirmEffects(list, remember) {
            if (!(ed.effectsConfirmed instanceof Set)) ed.effectsConfirmed = new Set();
            list.forEach(x => ed.effectsConfirmed.add(x));
            if (remember) core.storage.set(EFFECTS_KEY + ed.flow.id, Array.from(new Set(storedEffects().concat(list))));
        }

        function uncheckedMarkup() {
            return '<p class="ed-callout ed-callout--warn" data-ed-effects-unchecked>' + core.icon('alert') + '<span>' + esc(t('easydrag.ui.effects_unchecked')) + '</span></p>';
        }

        function effectsMarkup(list) {
            if (!list.size) return '';
            return '<div class="ed-callout ed-callout--warn" data-ed-test-effects>' + core.icon('alert') + '<div><strong>' + esc(t('easydrag.ui.effects_title')) + '</strong><p data-ed-effects-real>' + esc(t('easydrag.ui.effects_real_stop')) + '</p><ul class="ed-effects">' +
                Array.from(list.entries()).map(([effect, nodes]) => '<li data-ed-effect="' + esc(effect) + '">' + core.icon(EFFECT_ICONS[effect] || 'bolt') + '<span>' + esc(core.tr(t, 'easydrag.ui.effect_' + effect, effect)) + '</span><span class="ed-muted">' + esc(nodes.join(', ')) + '</span></li>').join('') +
                '</ul><label class="ed-check"><input type="checkbox" data-ed-effects-skip> ' + esc(t('easydrag.ui.effects_remember')) + '</label></div></div>';
        }

        function trackStart(work) {
            if (rc.startPost) return rc.startPost;
            const pending = work().finally(() => { if (rc.startPost === pending) rc.startPost = null; });
            rc.startPost = pending;
            return pending;
        }

        function editsReady() {
            return !(ed.flushPendingEdits && ed.flushPendingEdits() === false);
        }

        async function startTest(opts) {
            if (rc.disposed || ed.readonly || ed.runView || rc.starting || rc.startPost || isRunning()) return undefined;
            rc.starting = true;
            let dialog;
            try {
                dialog = await openTest(opts || {});
            } finally {
                if (!dialog) rc.starting = false;
            }
            if (dialog) dialog.done.then(() => { rc.starting = false; });
            return dialog;
        }

        async function openTest(o) {
            // Field and note edits land before the draft is saved for the test.
            if (!editsReady()) return undefined;
            // A test runs the saved draft: an invalid, offline or conflicting draft is not tested
            // silently in its last saved version.
            if (ed.saver && !(await ed.saver.flush())) {
                if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                return undefined;
            }
            if (rc.disposed) return undefined;
            // version is the draft the effects were checked for (the one just saved).
            let version = ed.model.version;
            const list = triggers(ed);
            if (!list.length) { ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_no_trigger'), type: 'error' }); return undefined; }
            const remembered = core.storage.get('aurago.easydrag.test-trigger.' + ed.flow.id, '');
            let trigger = list.find(n => n.id === (o.triggerNode || remembered)) || list.find(n => n.type === 'trigger.manual') || list[0];
            // The catalog describes each step with its default settings; the publish preview (a
            // read-only check of the saved draft) names the effects of the real ones. Without it
            // the dialog says that the effects could not be fully checked, and a test never starts
            // without the dialog.
            let real = null;
            const loadReal = async () => {
                try { real = realEffects(await ed.api.preview(ed.flow.id)); } catch (err) { real = null; }
            };
            await loadReal();
            if (rc.disposed) return undefined;
            const confirmed = confirmedEffects();
            // A step test runs what its trigger reaches: the effects follow the chosen trigger.
            const pending = id => new Map(Array.from(effects(ed, o.onlyNode, id, real)).filter(([effect]) => !confirmed.has(effect)));
            // unchecked: the effects could not be named in full (no preview, or a step whose effects
            // hook failed); the dialog then says so, and a test never starts without it.
            const unchecked = id => !real || unknownEffects(ed, o.onlyNode, id);
            let fx = pending(trigger.id);
            // recheck follows an edit that landed after the flush: it saves again and asks the
            // preview again. False when the draft cannot be saved.
            const recheck = async () => {
                if (!editsReady()) return false;
                if (ed.saver && !(await ed.saver.flush())) return false;
                version = ed.model.version;
                await loadReal();
                return true;
            };
            if (o.quick && ed.model.version !== version) {
                if (!(await recheck())) {
                    if (!rc.disposed) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_unsaved'), type: 'error' });
                    return undefined;
                }
                if (rc.disposed) return undefined;
                fx = pending(trigger.id);
            }
            if (o.quick && !fx.size && !unchecked(trigger.id) && ed.model.version === version) { await run(trigger.id, null, o.onlyNode, false); return undefined; }
            // A rerun from the run view (o.data) starts from that run's trigger data instead of the
            // stored sample. It runs as given even unchanged, and is not remembered unless ticked.
            let prefilled = !!(o.data && typeof o.data === 'object' && !Array.isArray(o.data));
            let sample = {};
            if (prefilled) sample = o.data;
            else {
                try { sample = (await ed.api.testData(ed.flow.id, trigger.id)).data || {}; } catch (err) { sample = {}; }
            }
            if (rc.disposed) return undefined;
            // The sample comes scrubbed (secret values read "[redacted]") and must never be saved
            // back: unchanged text runs without trigger_data, so the server uses the stored sample.
            let shown = JSON.stringify(sample, null, 2);
            let simulation = null;
            const dialog = core.modal(ed.root, {
                title: o.onlyNode ? t('easydrag.ui.test_node_title') : t('easydrag.ui.test_title'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--test',
                body: (list.length > 1 ? '<label class="ed-label">' + esc(t('easydrag.ui.test_trigger')) + '<select class="ed-input" data-ed-test-trigger>' +
                    list.map(n => '<option value="' + esc(n.id) + '"' + (n.id === trigger.id ? ' selected' : '') + '>' + esc(n.label || n.type) + '</option>').join('') + '</select></label>' : '') +
                    '<label class="ed-label">' + esc(t('easydrag.ui.test_data')) + '<textarea class="ed-input ed-code" rows="9" spellcheck="false" data-ed-test-data>' + esc(shown) + '</textarea></label>' +
                    '<p class="ed-hint">' + esc(t('easydrag.ui.test_data_hint')) + '</p><p class="ed-error" role="alert" hidden></p>' +
                    '<label class="ed-check"><input type="checkbox" data-ed-test-remember' + (prefilled ? '' : ' checked') + '> ' + esc(t('easydrag.ui.test_remember')) + '</label>' +
                    (prefilled && shown.includes(REDACTED) ? '<p class="ed-callout ed-callout--warn" data-ed-rerun-redacted>' + core.icon('alert') + '<span>' + esc(t('easydrag.ui.rerun_redacted')) + '</span></p>' : '') +
                    (unchecked(trigger.id) ? uncheckedMarkup() : '') + effectsMarkup(fx),
                actions: [{ id: 'cancel', label: t('easydrag.ui.cancel') }, { id: 'run', label: t('easydrag.ui.test_run'), primary: true, icon: 'play' }],
                onAction: async (action, d) => {
                    if (action !== 'run') return true;
                    const err = d.body.querySelector('.ed-error');
                    // An edit landed after the effects were checked: check them again and let the
                    // user confirm the new list.
                    if (ed.model.version !== version) {
                        const saved = await recheck();
                        if (rc.disposed) return false;
                        const pick = d.body.querySelector('[data-ed-test-trigger]');
                        showChecks(d.body, pick ? pick.value : trigger.id);
                        if (!saved) { err.hidden = false; err.textContent = t('easydrag.ui.test_unsaved'); }
                        return false;
                    }
                    if (simulation && !simulation.flush()) return false;
                    if (simulation && simulation.isSelected()) return trackStart(() => simulation.run());
                    const text = d.body.querySelector('[data-ed-test-data]').value;
                    const edited = text !== shown || prefilled;
                    let data = null;
                    if (edited) {
                        try { data = JSON.parse(text || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.json_invalid'); return false; }
                        if (!data || typeof data !== 'object' || Array.isArray(data)) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_object'); return false; }
                        if (ED.testing) {
                            try { ED.testing.parseSample(text || '{}'); } catch (e) { err.hidden = false; err.textContent = t('easydrag.ui.test_data_incomplete'); return false; }
                        }
                    }
                    const sel = d.body.querySelector('[data-ed-test-trigger]');
                    if (sel) trigger = ed.model.node(sel.value) || trigger;
                    // The effects shown count as confirmed once the test started: a refused or failed
                    // POST keeps the dialog open, and the next attempt asks again.
                    const skip = d.body.querySelector('[data-ed-effects-skip]');
                    const shownEffects = Array.from(fx.keys());
                    const rememberEffects = !!(skip && skip.checked);
                    core.storage.set('aurago.easydrag.test-trigger.' + ed.flow.id, trigger.id);
                    // Edited data that still holds a placeholder runs, but is not remembered: it
                    // would replace the stored secret values with "[redacted]".
                    const remember = edited && d.body.querySelector('[data-ed-test-remember]').checked;
                    const redacted = remember && text.includes(REDACTED);
                    return run(trigger.id, data, o.onlyNode, remember && !redacted).then(ok => {
                        if (ok && shownEffects.length) confirmEffects(shownEffects, rememberEffects);
                        if (ok && redacted) ed.ctx.notify({ title: t('easydrag.ui.test_title'), message: t('easydrag.ui.test_data_redacted') });
                        return ok ? true : false;
                    });
                }
            });
            dialog.body.querySelector('[data-ed-test-data]').value = shown;
            if (ED.testing && ed.api.simulate) simulation = ED.testing.decorate(ed, dialog, {
                trigger: () => { const select = dialog.body.querySelector('[data-ed-test-trigger]'); return select ? select.value : trigger.id; },
                onlyNode: o.onlyNode, revision: baseRevision,
                isCurrent: () => !rc.disposed && !ed.runView && ed.model.version === version,
                apply: (result, data) => {
                    const record = { id: 'simulation', mode: 'simulation', revision: result.revision, status: result.status, error_code: result.error_code, error_message: result.error_message, assertions: result.assertions, started_at: result.started_at || new Date().toISOString(), trigger_node: dialog.body.querySelector('[data-ed-test-trigger]') ? dialog.body.querySelector('[data-ed-test-trigger]').value : trigger.id, trigger_data: data };
                    const run = stepsFrom(result.steps || [], record);
                    if (ED.childHistory) ED.childHistory.rememberSimulation(run, result);
                    ed.lastRunData = { label: t('easydrag.ui.simulation_result'), roots: rootsFrom(result.steps || [], record) };
                    ed.testEvidence = { revision: result.revision, status: (result.assertions || []).some(assertion => !assertion.passed) ? 'error' : result.status, mode: 'simulation' };
                    setRun(run);
                    ed.bus.emit('last-run', record);
                    ed.bus.emit('last-run-data', ed.lastRunData);
                    canvas.announce(t('easydrag.ui.simulation_result'));
                }
            });
            // showChecks shows the hint and the effects for a trigger again (another trigger, a newer
            // check); the "don't ask again" tick stays as it was.
            function showChecks(body, id) {
                const skip = body.querySelector('[data-ed-effects-skip]');
                const remember = !!(skip && skip.checked);
                body.querySelectorAll('[data-ed-effects-unchecked], [data-ed-test-effects]').forEach(n => n.remove());
                fx = pending(id);
                if (unchecked(id)) body.appendChild(core.el(uncheckedMarkup()));
                if (fx.size) body.appendChild(core.el(effectsMarkup(fx))).querySelector('[data-ed-effects-skip]').checked = remember;
                if (simulation) simulation.syncMode();
            }
            // Another trigger shows its own sample, unless the text was edited. For a step test it
            // also shows the effects of what that trigger reaches.
            const select = dialog.body.querySelector('[data-ed-test-trigger]');
            if (select) {
                select.addEventListener('change', async () => {
                    if (o.onlyNode) showChecks(dialog.body, select.value);
                    const area = dialog.body.querySelector('[data-ed-test-data]');
                    const id = select.value;
                    if (area.value !== shown) return;
                    let next = {};
                    try { next = (await ed.api.testData(ed.flow.id, id)).data || {}; } catch (err) { next = {}; }
                    if (select.value !== id || area.value !== shown) return;
                    shown = JSON.stringify(next, null, 2);
                    area.value = shown;
                    if (simulation) simulation.refreshData();
                    prefilled = false;
                });
            }
            return dialog;
        }

        return { storedEffects, confirmedEffects, confirmEffects, uncheckedMarkup, effectsMarkup, trackStart, editsReady, startTest, openTest };
    }

    (ED.runParts = ED.runParts || {}).start = installStart;
})();
