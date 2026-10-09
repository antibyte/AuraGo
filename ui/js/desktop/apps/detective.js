(function () {
    'use strict';
    const instances = new Map();
    const base = '/api/desktop/detective';
    const active = c => ['queued', 'running'].includes(c?.run?.status);
    const key = () => window.crypto.randomUUID();
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, v = window.DetectiveViews, e = v.esc;
        const tr = k => { const text = ctx.t?.('desktop.detective_' + k); return !text || text === 'desktop.detective_' + k ? k : text; };
        const controller = new AbortController();
        const state = {host, id: windowId, ctx, controller, timer: null, cases: [], current: null, events: [], tab: 'report', revision: 0, busy: false, disposed: false, fresh: false, caps: null, latestSourceID: '', notice: '', forcePaint: false};
        instances.set(windowId, state);
        const readOnly = () => { const caps = state.caps || {}; return !!(ctx.readonly || caps.read_only || !caps.enabled || !caps.ready); };
        const request = async (path, options = {}) => {
            const opts = {...options, signal: controller.signal};
            if (opts.body) { opts.headers = {'Content-Type': 'application/json'}; opts.body = JSON.stringify(opts.body); }
            if (!ctx.api) throw new Error(tr('unavailable'));
            return ctx.api(base + path, opts);
        };
        const error = err => {
            if (state.disposed || !err || err.name === 'AbortError') return;
            state.notice = err.message || tr('error');
            const node = host.querySelector('.dt-error');
            if (node) { node.hidden = false; node.textContent = state.notice; }
        };
        const chosenReport = () => {
            const revisions = state.current?.reports || [];
            return state.revision ? revisions.find(x => x.revision === state.revision) : revisions.at(-1);
        };
        function listHTML() {
            const c = state.current;
            return state.cases.map(item => `<button class="dt-case ${item.id === c?.id ? 'is-selected' : ''}" data-case="${e(item.id)}" aria-pressed="${item.id === c?.id}"><b>${e(item.request.topic)}</b><small>${e(tr(item.run.status))} · ${e(tr(item.request.effort))}</small></button>`).join('') || `<p>${e(tr('no_cases'))}</p>`;
        }
        function actionsHTML(c, ro) {
            const r = c.run || {};
            const caps = state.caps || {};
            const canStop = !!caps.enabled && !!caps.ready;
            if (active(c)) return `<button data-do="finish" ${ro ? 'disabled' : ''}>${e(tr('finish'))}</button><button data-do="stop" ${canStop ? '' : 'disabled'}>${e(tr('stop'))}</button>`;
            const lead = r.status === 'draft' ? `<button data-do="start" ${ro ? 'disabled' : ''}>${e(tr('start'))}</button>` : r.status !== 'completed' ? `<button data-do="continue" ${ro ? 'disabled' : ''}>${e(tr('continue'))}</button>` : '';
            const effort = `<select class="dt-effort" aria-label="${e(tr('effort'))}">${['quick', 'normal', 'maximum'].map(x => `<option value="${x}" ${c.request.effort === x ? 'selected' : ''}>${e(tr(x))}</option>`).join('')}</select>`;
            const deepen = r.status === 'draft' ? '' : `<button data-do="deepen" ${ro ? 'disabled' : ''}>${e(tr('deepen'))}</button>`;
            return `${lead}${effort}${deepen}<button data-do="delete" ${ro ? 'disabled' : ''}>${e(tr('delete'))}</button>`;
        }
        function reasonText(reason) {
            if (!reason) return '';
            const translated = tr(reason);
            return translated === 'desktop.detective_' + reason ? reason : translated;
        }
        function answerFieldHTML() {
            return `<label>${e(tr('answer'))}<textarea class="dt-answer" maxlength="8000"></textarea></label>`;
        }
        function statusInner(c, ro) {
            const r = c.run || {}, usage = r.usage || {}, p = r.profile || {};
            const reason = reasonText(r.reason);
            return `<h2>${e(c.request.topic)}</h2><div><strong>${e(tr(r.status))}</strong><span>${e(tr(r.phase || 'research'))}</span><span>${Math.floor((usage.active_ms || 0) / 60000)} / ${Math.ceil((p.seconds || 0) / 60)} min · ${usage.tools || 0}/${p.tools || 0} ${e(tr('tools'))} · ${usage.pages || 0} ${e(tr('sources'))}</span></div>${reason ? `<p>${e(reason)}</p>` : ''}<div class="dt-actions">${actionsHTML(c, ro)}</div>${r.status === 'waiting_for_user' ? answerFieldHTML() : ''}`;
        }
        function exportHTML(c, ro) {
            const revisions = c.reports || [], chosen = chosenReport();
            if (!revisions.length) return '';
            return `<label>${e(tr('revision'))}<select class="dt-revision">${revisions.map(x => `<option value="${x.revision}" ${x === chosen ? 'selected' : ''}>${x.revision}${x.partial ? ' · ' + e(tr('partial')) : ''}</option>`).join('')}</select></label>${['md', 'pdf', 'docx'].map(format => `<a class="dt-download" href="${base}/cases/${encodeURIComponent(c.id)}/export?revision=${chosen.revision}&format=${format}">${format === 'md' ? 'Markdown' : format === 'docx' ? 'Word' : 'PDF'}</a>`).join('')}<button data-do="autor" ${ro ? 'disabled' : ''}>${e(tr('autor'))}</button>`;
        }
        function linkedExportRevision() {
            const link = host.querySelector('.dt-export a.dt-download');
            if (!link) return 0;
            const match = /[?&]revision=(\d+)/.exec(link.getAttribute('href') || '');
            return match ? Number(match[1]) : 0;
        }
        function exportRevisionSet() {
            const select = host.querySelector('.dt-export .dt-revision');
            if (!select) return '';
            return [...select.options].map(option => option.value).join(',');
        }
        function reportRevisionSet(reports) {
            return (reports || []).map(report => String(report.revision)).join(',');
        }
        function syncExport() {
            const bar = host.querySelector('.dt-export');
            if (!bar || !state.current) return;
            const chosen = chosenReport()?.revision || 0;
            if (linkedExportRevision() === chosen && exportRevisionSet() === reportRevisionSet(state.current.reports)) return;
            bar.innerHTML = exportHTML(state.current, readOnly());
        }
        function contentHTML() {
            if (state.tab === 'sources') return v.sources(state.current.sources, tr);
            if (state.tab === 'activity') return v.activity(state.events, tr);
            return v.report(chosenReport(), tr);
        }
        function caseMainHTML(c, ro) {
            const tabs = ['report', 'sources', 'activity'].map(tab => `<button data-tab="${tab}" aria-pressed="${state.tab === tab}">${e(tr(tab))}${tab === 'sources' ? ` (${(c.sources || []).length})` : ''}</button>`).join('');
            const activity = state.tab === 'activity' ? '' : `<ol class="dt-activity" hidden>${v.activityItems(state.events, tr)}</ol>`;
            return `<section class="dt-status" aria-live="polite">${statusInner(c, ro)}</section><nav class="dt-tabs" aria-label="${e(tr('views'))}">${tabs}</nav><div class="dt-export">${exportHTML(c, ro)}</div><div class="dt-content">${contentHTML()}</div>${activity}`;
        }
        function appHTML(main) {
            const ro = readOnly();
            return `<div class="dt-app"><header class="dt-header"><div><b>Detective</b><span>${e(tr('subtitle'))}</span></div><button data-do="new" ${ro ? 'disabled' : ''}>+ ${e(tr('new'))}</button></header><div class="dt-error" role="alert"${state.notice ? '' : ' hidden'}>${e(state.notice || '')}</div><div class="dt-layout"><aside aria-label="${e(tr('cases'))}">${listHTML()}</aside><main class="dt-main">${main}</main></div></div>`;
        }
        function draw() {
            if (state.disposed) return;
            const c = state.current, caps = state.caps || {}, ro = readOnly();
            host.innerHTML = appHTML(c ? caseMainHTML(c, ro) : '');
            if (!c) drawForm(host.querySelector('.dt-main'), caps, ro);
        }
        function drawForm(main, caps, ro) {
            main.innerHTML = `<form class="dt-form"><h1>${e(tr('new'))}</h1><p>${e(tr('intro'))}</p><label>${e(tr('topic'))}<textarea name="topic" required maxlength="8000" rows="3"></textarea></label><fieldset><legend>${e(tr('effort'))}</legend><div class="dt-profiles">${['quick', 'normal', 'maximum'].map(level => { const p = caps.profiles?.[level] || {}; return `<label><input type="radio" name="effort" value="${level}" ${level === 'normal' ? 'checked' : ''}><b>${e(tr(level))}</b><small>${Math.ceil((p.seconds || 0) / 60)} min · ${p.tools || 0} ${e(tr('tools'))}</small></label>`; }).join('')}</div></fieldset><details><summary>${e(tr('advanced'))}</summary><label>${e(tr('scope'))}<textarea name="scope" maxlength="8000"></textarea></label><label>${e(tr('language'))}<input name="language" value="${e(document.documentElement.lang || 'de')}" maxlength="30"></label><label>${e(tr('source_urls'))}<textarea name="source_urls" rows="2"></textarea></label><label>${e(tr('provider'))}<select name="provider_id">${(caps.providers || []).map(p => `<option value="${e(p.id)}" ${p.id === caps.provider_id ? 'selected' : ''}>${e(p.name || p.id)} · ${e(p.model)}</option>`).join('')}</select></label><label>${e(tr('model'))}<input name="model" placeholder="${e(caps.model || '')}" maxlength="200"></label>${(caps.private_sources || []).length ? `<fieldset><legend>${e(tr('private_sources'))}</legend>${caps.private_sources.map(name => `<label><input type="checkbox" name="private_sources" value="${e(name)}">${e(name)}</label>`).join('')}</fieldset>` : ''}</details><p class="dt-capabilities">${e(tr('available'))}: ${e((caps.tools || []).filter(x => !['detective_report', 'invoke_tool', 'discover_tools', 'execute_skill', 'list_skills'].includes(x)).join(', '))}</p>${ro ? `<p role="status">${e(tr('unavailable'))}</p>` : ''}<button type="submit" ${ro || state.busy ? 'disabled' : ''}>${e(tr('start'))}</button></form>`;
        }
        function rewriteList() {
            const aside = host.querySelector('.dt-layout aside');
            if (!aside) return;
            const buttons = [...aside.querySelectorAll('.dt-case')];
            const same = buttons.length === state.cases.length && buttons.every((btn, i) => btn.dataset.case === state.cases[i].id);
            if (!same) { aside.innerHTML = listHTML(); return; }
            buttons.forEach((btn, i) => {
                const item = state.cases[i];
                const shown = item.id === state.current?.id ? state.current : item;
                const title = btn.querySelector('b');
                if (title) title.textContent = shown.request?.topic || '';
                const small = btn.querySelector('small');
                if (small) small.textContent = `${tr(shown.run?.status)} · ${tr(shown.request?.effort)}`;
                const selected = item.id === state.current?.id;
                btn.classList.toggle('is-selected', selected);
                btn.setAttribute('aria-pressed', String(selected));
            });
        }
        function restoreAnswer(value, focused) {
            const box = host.querySelector('.dt-answer');
            if (!box) return;
            box.value = value || '';
            if (focused) box.focus();
        }
        function paint(sourcesChanged, reportChanged, freshEvents) {
            if (state.disposed) return;
            if (!state.current) {
                if (!host.querySelector('.dt-form')) draw();
                return;
            }
            const focused = host.ownerDocument.activeElement;
            const answerFocused = !!(focused && host.contains(focused) && focused.matches?.('.dt-answer'));
            const answerValue = host.querySelector('.dt-answer')?.value || '';
            if (state.forcePaint || !host.querySelector('.dt-status') || host.querySelector('.dt-case.is-selected')?.dataset.case !== state.current.id) {
                state.forcePaint = false;
                draw();
                restoreAnswer(answerValue, answerFocused);
                return;
            }
            const content = host.querySelector('.dt-content');
            const scroll = content ? content.scrollTop : 0;
            const openIDs = [...host.querySelectorAll('.dt-source[open]')].map(node => node.dataset.id);
            rewriteList();
            rewriteStatus(answerValue, answerFocused);
            const sourcesTabChanged = state.tab === 'sources' && sourcesChanged;
            const reportTabChanged = state.tab === 'report' && reportChanged;
            if (sourcesTabChanged && content) {
                content.innerHTML = v.sources(state.current.sources, tr);
                content.querySelectorAll('.dt-source').forEach(node => { if (openIDs.includes(node.dataset.id)) node.open = true; });
            } else if (reportTabChanged && content) {
                content.innerHTML = v.report(chosenReport(), tr);
            }
            if (content) content.scrollTop = scroll;
            syncExport();
            const sourceBtn = host.querySelector('[data-tab=sources]');
            if (sourceBtn) {
                const label = `${tr('sources')} (${(state.current.sources || []).length})`;
                if (sourceBtn.textContent !== label) sourceBtn.textContent = label;
            }
            if (freshEvents && freshEvents.length) {
                const list = host.querySelector('.dt-activity');
                if (list) {
                    const present = new Set([...list.querySelectorAll('[data-event]')].map(node => node.dataset.event));
                    const missing = freshEvents.filter(ev => !present.has(String(ev.id)));
                    if (missing.length) list.insertAdjacentHTML('beforeend', v.activityItems(missing, tr));
                }
            }
        }
        function rewriteStatus(answerValue, answerFocused) {
            const section = host.querySelector('.dt-status');
            if (!section || !state.current) return;
            const c = state.current, r = c.run || {}, usage = r.usage || {}, p = r.profile || {};
            const title = section.querySelector('h2');
            if (title && title.textContent !== (c.request?.topic || '')) title.textContent = c.request?.topic || '';
            const strong = section.querySelector('strong');
            const statusText = tr(r.status);
            if (strong && strong.textContent !== statusText) strong.textContent = statusText;
            const row = section.querySelector('div');
            const spans = row ? [...row.children].filter(node => node.tagName === 'SPAN') : [];
            const phaseText = tr(r.phase || 'research');
            const usageText = `${Math.floor((usage.active_ms || 0) / 60000)} / ${Math.ceil((p.seconds || 0) / 60)} min · ${usage.tools || 0}/${p.tools || 0} ${tr('tools')} · ${usage.pages || 0} ${tr('sources')}`;
            if (spans[0] && spans[0].textContent !== phaseText) spans[0].textContent = phaseText;
            if (spans[1] && spans[1].textContent !== usageText) spans[1].textContent = usageText;
            const actions = section.querySelector('.dt-actions');
            if (actions) rewriteActions(actions, c, readOnly());
            syncReason(section, r.reason);
            syncAnswer(section, r.status, answerValue, answerFocused);
        }
        function syncReason(section, reason) {
            const text = reasonText(reason);
            let para = section.querySelector(':scope > p');
            if (!text) {
                if (para) para.remove();
                return;
            }
            if (!para) {
                para = document.createElement('p');
                section.insertBefore(para, section.querySelector('.dt-actions'));
            }
            if (para.textContent !== text) para.textContent = text;
        }
        function syncAnswer(section, status, answerValue, answerFocused) {
            let box = section.querySelector('.dt-answer');
            if (status === 'waiting_for_user') {
                if (!box) {
                    section.insertAdjacentHTML('beforeend', answerFieldHTML());
                    box = section.querySelector('.dt-answer');
                    if (box && answerValue) box.value = answerValue;
                    if (box && answerFocused) box.focus();
                } else if (answerFocused && host.ownerDocument.activeElement !== box) {
                    box.focus();
                }
                return;
            }
            if (box) (box.closest('label') || box).remove();
        }
        function rewriteActions(actions, c, ro) {
            const desired = actionsHTML(c, ro);
            const probe = document.createElement('div');
            probe.innerHTML = desired;
            const have = [...actions.querySelectorAll('button')].map(btn => btn.dataset.do).join(',');
            const want = [...probe.querySelectorAll('button')].map(btn => btn.dataset.do).join(',');
            if (have !== want || !!actions.querySelector('select') !== !!probe.querySelector('select')) {
                actions.innerHTML = desired;
                return;
            }
            probe.querySelectorAll('button').forEach(src => {
                const btn = actions.querySelector(`[data-do="${src.dataset.do}"]`);
                if (!btn) return;
                btn.disabled = src.disabled;
                if (btn.textContent !== src.textContent) btn.textContent = src.textContent;
            });
            const sel = actions.querySelector('.dt-effort');
            if (sel && host.ownerDocument.activeElement !== sel && c.request?.effort) sel.value = c.request.effort;
        }
        function applyLive(live) {
            const c = state.current;
            if (!c || !live) return;
            c.run = c.run || {};
            c.request = c.request || {};
            c.run.status = live.status;
            c.run.phase = live.phase;
            c.run.reason = live.reason || '';
            c.run.usage = live.usage || {};
            c.run.profile = live.profile || {};
            c.request.topic = live.topic;
            c.request.effort = live.effort;
            if (live.updated_at) { c.run.updated_at = live.updated_at; c.updated_at = live.updated_at; }
        }
        let refreshChain = Promise.resolve();
        function refresh(redraw = true) {
            const run = refreshChain.then(() => load(redraw));
            refreshChain = run.catch(() => {});
            return run;
        }
        async function load(redraw) {
            const list = await request('/cases');
            if (state.disposed) return;
            state.cases = list.cases || [];
            if (!state.current) { if (redraw && !host.querySelector('.dt-form')) draw(); return; }
            const id = state.current.id;
            const live = await request('/cases/' + id + '/live');
            const after = state.events.at(-1)?.id || 0;
            const data = await request('/cases/' + id + '/events?after=' + after);
            if (state.disposed || state.current?.id !== id) return;
            const fresh = (data.events || []).filter(ev => ev.id > after);
            state.events = [...state.events, ...fresh].slice(-500);
            const liveSourceID = live.latest_source_id || '';
            const sourcesChanged = (state.current.sources || []).length !== live.sources || state.latestSourceID !== liveSourceID;
            const reportChanged = ((state.current.reports || []).at(-1)?.revision || 0) !== (live.latest_revision || 0);
            applyLive(live);
            if ((state.tab === 'sources' && sourcesChanged) || (state.tab === 'report' && reportChanged) || (state.current.sources == null && state.tab !== 'activity')) {
                const full = await request('/cases/' + id);
                if (state.disposed || state.current?.id !== id) return;
                state.current = full;
                if (!Array.isArray(state.current.sources)) state.current.sources = [];
                const loaded = state.current.sources;
                const loadedSourceID = loaded.length ? (loaded[loaded.length - 1].id || '') : '';
                if (loadedSourceID === liveSourceID) state.latestSourceID = loadedSourceID;
            }
            if (state.disposed || state.current?.id !== id) return;
            if (redraw) paint(sourcesChanged, reportChanged, fresh);
        }
        async function poll() {
            if (state.disposed) return;
            try { await refresh(true); }
            catch (err) { error(err); }
            if (!state.disposed) state.timer = setTimeout(poll, 3000);
        }
        host.addEventListener('submit', async ev => {
            if (!ev.target.matches('.dt-form')) return;
            ev.preventDefault();
            if (state.busy) return;
            state.busy = true;
            const data = new FormData(ev.target);
            const submit = ev.target.querySelector('[type=submit]');
            if (submit) submit.disabled = true;
            state.notice = '';
            try {
                const body = Object.fromEntries(data);
                body.source_urls = String(body.source_urls || '').split(/\s+/).filter(Boolean);
                body.private_sources = data.getAll('private_sources');
                const c = await request('/cases', {method: 'POST', body});
                state.current = c;
                state.events = [];
                state.fresh = false;
                state.latestSourceID = '';
                try {
                    await request('/cases/' + c.id + '/run', {method: 'POST', body: {action: 'start', idempotency_key: key()}});
                } catch (err) {
                    error(err);
                }
                await refresh();
            } catch (err) {
                error(err);
                if (submit) submit.disabled = false;
            } finally { state.busy = false; }
        }, {signal: controller.signal});
        host.addEventListener('change', ev => { if (ev.target.matches('.dt-revision')) { state.revision = Number(ev.target.value); draw(); } }, {signal: controller.signal});
        host.addEventListener('click', async ev => {
            const el = ev.target.closest('button,a[data-ref],a.dt-download');
            if (!el || el.disabled || state.busy) return;
            if (el.matches('.dt-download')) {
                ev.preventDefault();
                state.busy = true;
                try {
                    const response = await fetch(el.href, {signal: controller.signal, credentials: 'same-origin'});
                    if (!response.ok) { const problem = await response.json(); throw new Error(problem.error || tr('error')); }
                    const blob = await response.blob();
                    if (state.disposed) return;
                    const url = URL.createObjectURL(blob), link = document.createElement('a');
                    link.href = url;
                    link.download = 'detective-report.' + (new URL(el.href).searchParams.get('format') || 'md');
                    link.click();
                    setTimeout(() => URL.revokeObjectURL(url), 1000);
                } catch (err) { error(err); }
                finally { state.busy = false; }
                return;
            }
            if (el.dataset.tab) { state.notice = ''; state.tab = el.dataset.tab; draw(); return; }
            if (el.dataset.ref) {
                ev.preventDefault();
                state.tab = 'sources';
                draw();
                const index = (state.current.sources || []).findIndex(s => s.id === el.dataset.ref);
                const target = host.querySelectorAll('.dt-source')[index];
                if (target) { target.open = true; target.scrollIntoView({block: 'nearest'}); }
                return;
            }
            try {
                if (el.dataset.case) {
                    state.notice = '';
                    state.current = await request('/cases/' + el.dataset.case);
                    state.events = [];
                    state.revision = 0;
                    state.latestSourceID = '';
                    state.forcePaint = true;
                    await refresh();
                    return;
                }
                const action = el.dataset.do;
                if (action === 'new') { state.notice = ''; state.current = null; state.fresh = true; draw(); host.querySelector('[name=topic]')?.focus(); return; }
                if (!action || !state.current) return;
                state.busy = true;
                if (action === 'delete') {
                    const accepted = ctx.confirmDialog ? await ctx.confirmDialog(tr('delete'), tr('delete_confirm')) : window.confirm(tr('delete_confirm'));
                    if (!accepted) return;
                    await request('/cases/' + state.current.id, {method: 'DELETE'});
                    state.current = null;
                } else if (action === 'autor') {
                    const revision = state.revision || state.current.reports.at(-1).revision;
                    const result = await request('/cases/' + state.current.id + '/autor?revision=' + revision, {method: 'POST'});
                    ctx.openApp?.('writer', {path: result.path});
                } else {
                    const body = {action, idempotency_key: key()};
                    if (action === 'start' || action === 'deepen') body.effort = host.querySelector('.dt-effort')?.value || state.current.request.effort;
                    if (state.current.run.status === 'waiting_for_user') body.answer = host.querySelector('.dt-answer')?.value || '';
                    await request('/cases/' + state.current.id + '/run', {method: 'POST', body});
                }
                await refresh();
            } catch (err) { error(err); }
            finally { state.busy = false; }
        }, {signal: controller.signal});
        (async () => { try { state.caps = await request('/capabilities'); await refresh(false); if (!state.fresh && state.cases.length) { state.current = state.cases[0]; await refresh(false); } draw(); poll(); } catch (err) { draw(); error(err); } })();
    }
    function dispose(id) { const s = instances.get(id); if (!s) return; s.disposed = true; clearTimeout(s.timer); s.controller.abort(); instances.delete(id); }
    window.DetectiveApp = {render, dispose};
})();
