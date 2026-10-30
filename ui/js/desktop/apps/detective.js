(function () {
    'use strict';
    const instances = new Map();
    const base = '/api/desktop/detective';
    const active = c => ['queued', 'running'].includes(c?.run?.status);
    // crypto.randomUUID only exists in a secure context, which a plain-HTTP LAN install is not.
    // The server caps the idempotency key at 128 characters (internal/detective/jobs.go).
    const key = () => {
        const c = window.crypto;
        if (c && typeof c.randomUUID === 'function') return c.randomUUID();
        const bytes = new Uint8Array(16);
        if (c && typeof c.getRandomValues === 'function') c.getRandomValues(bytes);
        else for (let i = 0; i < bytes.length; i++) bytes[i] = Math.floor(Math.random() * 256);
        return Array.from(bytes, byte => byte.toString(16).padStart(2, '0')).join('');
    };
    function render(host, windowId, context) {
        dispose(windowId);
        const ctx = context || {}, v = window.DetectiveViews, e = v.esc;
        const tr = k => { const text = ctx.t?.('desktop.detective_' + k); return !text || text === 'desktop.detective_' + k ? k : text; };
        const controller = new AbortController();
        const state = {host, id: windowId, ctx, controller, timer: null, cases: [], current: null, events: [], tab: 'report', revision: 0, busy: false, disposed: false, fresh: false, caps: null, latestSourceID: '', notice: '', forcePaint: false};
        instances.set(windowId, state);
        let selectedReport = null, reportParts = [], history = [], historyNext = 0, historyLoaded = false;
        let archiveSources = [], sourceNext = 0, sourcesLoaded = false, viewGeneration = 0;
        // Source text loads on first expand; the case response carries metadata only.
        const excerpts = new Map();
        const autoDrafts = new Map();
        const draftKey = () => state.current?.id || 'new';
        function resetArchive() { selectedReport = null; reportParts = []; history = []; historyNext = 0; historyLoaded = false; archiveSources = []; sourceNext = 0; sourcesLoaded = false; excerpts.clear(); state.revision = 0; viewGeneration++; }
        const readOnly = () => { const caps = state.caps || {}; return !!(ctx.readonly || caps.read_only || !caps.enabled || !caps.ready || state.current?.auto_update?.outcome === 'permission_blocked'); };
        const blockText = () => {
            const caps = state.caps || {};
            if (caps.initialization_error) return `${tr('init_failed')} (${caps.initialization_error})`;
            const reason = caps.block_reason;
            return reason ? tr('block_' + reason) : '';
        };
        const request = async (path, options = {}) => {
            const opts = {...options, signal: controller.signal};
            if (opts.body) { opts.headers = {'Content-Type': 'application/json'}; opts.body = JSON.stringify(opts.body); }
            if (!ctx.api) throw new Error(tr('unavailable'));
            return ctx.api(base + path, opts);
        };
        const error = err => {
            if (state.disposed || !err || err.name === 'AbortError') return;
            state.notice = v.errorText(tr, err);
            const node = host.querySelector('.dt-error');
            if (node) { node.hidden = false; node.textContent = state.notice; }
        };
        const chosenReport = () => v.chosenReport(state.current, state.revision, history, selectedReport);
        window.AuraDesktopEmail?.attach(windowId, '.dt-export', () => {
            const report=chosenReport(),id=state.current?.id;
            return {key:id+':'+report?.revision,signal:controller.signal,disabled:state.disposed||!id||!report,
                subject:state.current?.title,formats:v.formats(state.caps||{}).map(f=>({id:f.id,label:f.label})),
                createFiles:(format,signal)=>window.AuraDesktopEmail.files([{url:base+'/cases/'+encodeURIComponent(id)+'/export?revision='+report.revision+'&format='+encodeURIComponent(format),name:'report-r'+report.revision+'.'+format}],signal)};
        });
        function autoDraft() {
            const box = host.querySelector('.dt-auto-settings');
            if (!box) return null;
            return Object.fromEntries([...box.querySelectorAll('[name]')].map(node => [node.name, node.type === 'checkbox' ? node.checked : node.value]));
        }
        function autoSettings() {
            const draft = autoDraft();
            if (!draft) return {enabled: false, interval_hours: 24, report_style: 'append', effort: 'quick'};
            const hours = Number(draft.auto_interval) * (draft.auto_unit === 'days' ? 24 : 1);
            if (!Number.isInteger(Number(draft.auto_interval)) || hours < 1 || hours > 8760) throw new Error(tr('auto_invalid'));
            return {enabled: draft.auto_enabled, interval_hours: hours, report_style: draft.auto_style, effort: draft.auto_effort};
        }
        function syncAuto() {
            const box = host.querySelector('.dt-auto-settings');
            if (!box) return;
            box.disabled = readOnly();
            const draft = autoDrafts.get(draftKey());
            if (draft) {
                box.querySelectorAll('[name]').forEach(node => { if (node.type === 'checkbox') node.checked = draft[node.name]; else node.value = draft[node.name]; });
            } else if (state.current) {
                const a = state.current.auto_update || {}, hours = a.interval_hours || 24, days = hours % 24 === 0;
                box.querySelector('[name=auto_enabled]').checked = !!a.enabled;
                box.querySelector('[name=auto_interval]').value = days ? hours / 24 : hours;
                box.querySelector('[name=auto_unit]').value = days ? 'days' : 'hours';
                box.querySelector('[name=auto_style]').value = a.report_style || 'append';
                box.querySelector('[name=auto_effort]').value = a.effort || 'quick';
            }
            box.querySelector('[name=auto_interval]').max = box.querySelector('[name=auto_unit]').value === 'days' ? 365 : 8760;
            const output = host.querySelector('.dt-auto-state');
            const html = v.autoStatus(state.current?.auto_update, tr);
            if (output && output.innerHTML !== html) output.innerHTML = html;
            const save = host.querySelector('[data-do=auto-save]');
            if (save) save.disabled = readOnly() || state.busy;
            const pause = host.querySelector('[data-do=auto-pause]');
            if (pause) pause.disabled = !state.current?.auto_update?.enabled || state.busy || !state.caps?.ready;
        }
        function autoPanel(c, ro) {
            return `<details class="dt-auto-panel"><summary>${e(tr('auto_title'))}</summary><div class="dt-auto-state" aria-live="polite">${v.autoStatus(c.auto_update, tr, c.run?.usage)}</div>${v.autoSettings(c.auto_update, tr, ro)}<div class="dt-auto-actions"><button data-do="auto-save"${ro ? ' disabled' : ''}>${e(tr('auto_save'))}</button><button data-do="auto-pause"${!c.auto_update?.enabled ? ' disabled' : ''}>${e(tr('auto_pause'))}</button></div></details>`;
        }
        const knownReports = () => v.knownReports(state.current, history, selectedReport);
        function caseTokens(item) {
            const usage = item.run?.usage || {};
            const total = (usage.prompt_tokens || 0) + (usage.completion_tokens || 0);
            return total ? ` · ${total.toLocaleString()} ${e(tr('tokens'))}` : '';
        }
        function listHTML() {
            const c = state.current;
            return state.cases.map(item => `<button class="dt-case ${item.id === c?.id ? 'is-selected' : ''}" data-case="${e(item.id)}" aria-pressed="${item.id === c?.id}"><b>${e(item.request.topic)}</b><small>${e(tr(item.run.status))} · ${e(tr(item.request.effort))}${caseTokens(item)}</small></button>`).join('') || `<p>${e(tr('no_cases'))}</p>`;
        }
        function actionsHTML(c, ro) {
            const r = c.run || {};
            const caps = state.caps || {};
            const canStop = v.canStop(caps);
            if (active(c)) return `<button data-do="finish" ${ro ? 'disabled' : ''}>${e(tr('finish'))}</button><button data-do="stop" ${canStop ? '' : 'disabled'}>${e(tr('stop'))}</button>`;
            const lead = r.status === 'draft' ? `<button data-do="start" ${ro ? 'disabled' : ''}>${e(tr('start'))}</button>` : r.status !== 'completed' ? `<button data-do="continue" ${ro ? 'disabled' : ''}>${e(tr('continue'))}</button>` : '';
            const effort = `<select class="dt-effort" aria-label="${e(tr('effort'))}">${['quick', 'normal', 'maximum'].map(x => `<option value="${x}" ${c.request.effort === x ? 'selected' : ''}>${e(tr(x))}</option>`).join('')}</select>`;
            const deepen = r.status === 'draft' ? '' : `<button data-do="deepen" ${ro ? 'disabled' : ''}>${e(tr('deepen'))}</button>`;
            return `${lead}${effort}${deepen}<button data-do="delete" ${ro ? 'disabled' : ''}>${e(tr('delete'))}</button>`;
        }
        function reasonText(reason) {
            if (!reason) return '';
            if (reason === 'no_changes') return tr('auto_no_changes');
            const translated = tr(reason);
            return translated === 'desktop.detective_' + reason ? reason : translated;
        }
        function answerFieldHTML() {
            return `<label>${e(tr('answer'))}<textarea class="dt-answer" maxlength="8000"></textarea></label>`;
        }
        function statusInner(c, ro) {
            const r = c.run || {}, usage = r.usage || {}, p = r.profile || {};
            const reason = reasonText(r.reason);
            return `<h1>${e(c.request.topic)}</h1><div aria-live="polite"><strong>${e(tr(r.status))}</strong><span>${e(tr(r.phase || 'research'))}</span><span>${Math.floor((usage.active_ms || 0) / 60000)} / ${Math.ceil((p.seconds || 0) / 60)} min · ${usage.tools || 0}/${p.tools || 0} ${e(tr('tools'))} · ${usage.pages || 0} ${e(tr('sources'))}${v.tokenText(usage, tr) ? ' ' + v.tokenText(usage, tr) : ''}</span></div>${reason ? `<p>${e(reason)}</p>` : ''}<div class="dt-actions"${blockText() ? ` title="${e(blockText())}"` : ''}>${actionsHTML(c, ro)}</div>${r.status === 'waiting_for_user' ? answerFieldHTML() : ''}`;
        }
        function provisionalNotice() {
            const report = chosenReport();
            if (!report?.provisional) return '';
            const complete = state.current?.latest_complete_revision || 0;
            return `<p class="dt-hint dt-provisional">${e(tr('provisional_notice'))}${complete ? ` <button data-revision="${complete}">${e(tr('revision'))} ${complete}</button>` : ''}</p>`;
        }
        // selectCase is the entry point for a launch context (notification click).
        async function selectCase(id) {
            if (!id || state.disposed || state.current?.id === id) return;
            resetArchive();
            state.events = [];
            state.tab = 'report';
            state.notice = '';
            state.current = state.cases.find(c => c.id === id) || null;
            if (!state.current) {
                try {
                    state.current = await request('/cases/' + encodeURIComponent(id));
                } catch (err) {
                    error(err);
                    return;
                }
                if (!Array.isArray(state.current.sources)) state.current.sources = [];
            }
            try { await refresh(true); } catch (err) { error(err); }
            if (!state.disposed) draw();
        }
        function exportHTML(c, ro) {
            const revisions = knownReports(), chosen = chosenReport() || revisions.at(-1);
            if (!revisions.length || !chosen) return '';
            const formats = v.formats(state.caps), hintID = `dt-pdf-hint-${state.id}`;
            const links = formats.map(f => `<a class="dt-download" href="${base}/cases/${encodeURIComponent(c.id)}/export?revision=${chosen.revision}&format=${f.id}"${f.hint ? ` aria-describedby="${hintID}"` : ''}>${f.label}</a>`).join('');
            const hint = formats.some(f => f.hint) ? `<span class="dt-hint" id="${hintID}">${e(tr('pdf_hint'))}</span>` : '';
            return `<label>${e(tr('revision'))}<select class="dt-revision">${revisions.map(x => `<option value="${x.revision}" ${x === chosen ? 'selected' : ''}>${x.revision}${x.partial ? ' · ' + e(tr('partial')) : ''}${x.provisional ? ' · ' + e(tr('provisional')) : ''}</option>`).join('')}</select></label>${links}${hint}<button data-do="autor" ${ro ? 'disabled' : ''}>${e(tr('autor'))}</button>`;
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
            if (linkedExportRevision() === chosen && exportRevisionSet() === reportRevisionSet(knownReports())) return;
            bar.innerHTML = exportHTML(state.current, readOnly());
        }
        function contentHTML() {
            if (state.tab === 'sources') {
                const items = [...new Map([...(state.current.sources || []), ...archiveSources].map(s => [s.id, s])).values()]
                    .map(s => (s.excerpt || !excerpts.has(s.id) ? s : {...s, excerpt: excerpts.get(s.id)}));
                return v.sources(items, tr) + (!sourcesLoaded || sourceNext ? `<button data-do="sources-more">${e(tr('archive_more'))}</button>` : '');
            }
            if (state.tab === 'activity') return v.activity(state.events, tr);
            if (state.tab === 'history') return `<div class="dt-history">${history.map(r => `<button data-revision="${r.revision}"><b>${e(tr('revision'))} ${r.revision}</b><span>${e(new Date(r.created_at).toLocaleString())}</span><span>${e(r.title)}</span></button>`).join('')}${!historyLoaded || historyNext ? `<button data-do="history-more">${e(tr('archive_more'))}</button>` : ''}</div>`;
            const parts = reportParts.length ? reportParts : [chosenReport()];
            const baseRevision = parts[0]?.base_revision || 0;
            return `${provisionalNotice()}${baseRevision ? `<button data-base-revision="${baseRevision}">${e(tr('archive_previous'))}</button>` : ''}${parts.map(r => v.report(r, tr)).join('')}`;
        }
        function caseMainHTML(c, ro) {
            const tabs = ['report', 'sources', 'activity', 'history'].map(tab => `<button data-tab="${tab}" role="tab" id="dt-tab-${state.id}-${tab}" aria-controls="dt-panel-${state.id}" aria-selected="${state.tab === tab}">${e(tr(tab))}${tab === 'sources' ? ` (${c.source_count || (c.sources || []).length})` : ''}</button>`).join('');
            const activity = state.tab === 'activity' ? '' : `<ol class="dt-activity" hidden>${v.activityItems(state.events, tr)}</ol>`;
            return `<section class="dt-status">${statusInner(c, ro)}</section>${autoPanel(c, ro)}<nav class="dt-tabs" role="tablist" aria-label="${e(tr('views'))}">${tabs}</nav><div class="dt-export">${exportHTML(c, ro)}</div><div class="dt-content" role="tabpanel" id="dt-panel-${state.id}" aria-labelledby="dt-tab-${state.id}-${state.tab}">${contentHTML()}</div>${activity}`;
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
            syncAuto();
        }
        function drawForm(main, caps, ro) {
            const eligibleProviders = (caps.providers || []).filter(provider => provider && provider.id && provider.usage !== 'decisions');
            const selectedProvider = String(caps.provider_id || '');
            const missingProvider = selectedProvider && !eligibleProviders.some(provider => String(provider.id) === selectedProvider)
                ? `<option value="${e(selectedProvider)}" selected disabled>${e(selectedProvider)} — ${e(tr('provider_unavailable'))}</option>` : '';
            const providerOptions = missingProvider + eligibleProviders.map(p => `<option value="${e(p.id)}" ${p.id === caps.provider_id ? 'selected' : ''}>${e(p.name || p.id)} · ${e(p.model)}</option>`).join('');
            main.innerHTML = `<form class="dt-form"><h1>${e(tr('new'))}</h1><p>${e(tr('intro'))}</p><label>${e(tr('topic'))}<textarea name="topic" required maxlength="8000" rows="3"></textarea></label><fieldset><legend>${e(tr('effort'))}</legend><div class="dt-profiles">${['quick', 'normal', 'maximum'].map(level => { const p = caps.profiles?.[level] || {}; return `<label><input type="radio" name="effort" value="${level}" ${level === 'normal' ? 'checked' : ''}><b>${e(tr(level))}</b><small>${Math.ceil((p.seconds || 0) / 60)} min · ${p.tools || 0} ${e(tr('tools'))}</small></label>`; }).join('')}</div></fieldset><details><summary>${e(tr('advanced'))}</summary><label>${e(tr('scope'))}<textarea name="scope" maxlength="8000"></textarea></label><label>${e(tr('language'))}<input name="language" value="${e(document.documentElement.lang || 'de')}" maxlength="30"></label><label>${e(tr('source_urls'))}<textarea name="source_urls" rows="2"></textarea></label><label>${e(tr('provider'))}<select name="provider_id">${providerOptions}</select></label><label>${e(tr('model'))}<input name="model" placeholder="${e(caps.model || '')}" maxlength="200"></label>${(caps.private_sources || []).length ? `<fieldset><legend>${e(tr('private_sources'))}</legend>${caps.private_sources.map(name => `<label><input type="checkbox" name="private_sources" value="${e(name)}">${e(name)}</label>`).join('')}</fieldset>` : ''}</details><p class="dt-capabilities">${e(tr('available'))}: ${e((caps.tools || []).filter(x => !['detective_report', 'invoke_tool', 'discover_tools', 'execute_skill', 'list_skills'].includes(x)).join(', '))}</p>${ro ? `<p role="status">${e(tr('unavailable'))}${blockText() ? ' ' + e(blockText()) : ''}</p>` : ''}<button type="submit" ${ro || state.busy ? 'disabled' : ''}>${e(tr('start'))}</button></form>`;
            main.querySelector('.dt-profiles')?.closest('fieldset')?.insertAdjacentHTML('afterend', v.autoSettings(null, tr, ro));
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
            syncAuto();
            const sourcesTabChanged = state.tab === 'sources' && sourcesChanged;
            const reportTabChanged = state.tab === 'report' && reportChanged && !state.revision;
            if (sourcesTabChanged && content) {
                content.innerHTML = contentHTML();
                content.querySelectorAll('.dt-source').forEach(node => { if (openIDs.includes(node.dataset.id)) node.open = true; });
            } else if (reportTabChanged && content) {
                reportParts = [];
                content.innerHTML = contentHTML();
            }
            if (content) content.scrollTop = scroll;
            syncExport();
            const sourceBtn = host.querySelector('[data-tab=sources]');
            if (sourceBtn) {
                const label = `${tr('sources')} (${state.current.source_count || (state.current.sources || []).length})`;
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
            if (live.auto_update) c.auto_update = live.auto_update;
            c.source_count = live.sources;
        }
        let refreshChain = Promise.resolve();
        function refresh(redraw = true) {
            const run = refreshChain.then(() => load(redraw));
            refreshChain = run.catch(() => {});
            return run;
        }
        async function load(redraw) {
            if (!state.current) {
                // Research form open: no case to poll. The list arrives once so the
                // sidebar is filled, then stays until a case is selected again.
                if (!state.cases.length) {
                    const list = await request('/cases');
                    if (state.disposed) return;
                    state.cases = list.cases || [];
                }
                if (redraw && !host.querySelector('.dt-form')) draw();
                return;
            }
            const id = state.current.id;
            const after = state.events.at(-1)?.id || 0;
            // One round trip for status and new events; the list stays for the sidebar.
            const snapshot = await request('/cases/' + id + '/state?after=' + after);
            if (state.disposed || state.current?.id !== id) return;
            const list = await request('/cases');
            if (state.disposed || state.current?.id !== id) return;
            state.cases = list.cases || [];
            const live = snapshot.live || {};
            const fresh = (snapshot.events || []).filter(ev => ev.id > after);
            state.events = [...state.events, ...fresh].slice(-500);
            const liveSourceID = live.latest_source_id || '';
            const sourcesChanged = state.latestSourceID !== liveSourceID;
            const reportChanged = ((state.current.reports || []).at(-1)?.revision || 0) !== (live.latest_revision || 0);
            applyLive(live);
            if ((state.tab === 'sources' && sourcesChanged) || (state.tab === 'report' && reportChanged) || (state.current.sources == null && state.tab !== 'activity')) {
                const full = await request('/cases/' + id);
                if (state.disposed || state.current?.id !== id) return;
                state.current = full;
                if (!Array.isArray(state.current.sources)) state.current.sources = [];
                const loaded = state.current.sources;
                const loadedSourceID = loaded.length ? (loaded[loaded.length - 1].id || '') : '';
                if (full.latest_source_id === liveSourceID || loadedSourceID === liveSourceID) state.latestSourceID = liveSourceID;
            }
            if (state.disposed || state.current?.id !== id) return;
            if (redraw) paint(sourcesChanged, reportChanged, fresh);
        }
        async function poll() {
            if (state.disposed || document.hidden) return;
            try { await refresh(true); }
            catch (err) { error(err); }
            if (!state.disposed) state.timer = setTimeout(poll, v.pollDelay(state.current?.run?.status));
        }
        // A hidden tab or minimized Desktop window does no work at all; becoming visible
        // polls immediately instead of waiting for the next interval.
        const onVisibility = () => {
            if (state.disposed) return;
            clearTimeout(state.timer);
            if (!document.hidden) poll();
        };
        document.addEventListener('visibilitychange', onVisibility, {signal: controller.signal});
        state.selectCase = selectCase;
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
                body.auto_update = autoSettings();
                for (const name of ['auto_enabled', 'auto_interval', 'auto_unit', 'auto_style', 'auto_effort']) delete body[name];
                const c = await request('/cases', {method: 'POST', body});
                autoDrafts.delete('new');
                resetArchive();
                state.current = c;
                state.events = [];
                state.fresh = false;
                state.latestSourceID = '';
                state.tab = 'report';
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
        const rememberAuto = ev => {
            if (!ev.target.closest('.dt-auto-settings')) return;
            autoDrafts.set(draftKey(), autoDraft());
            const number = host.querySelector('[name=auto_interval]');
            number.max = host.querySelector('[name=auto_unit]').value === 'days' ? 365 : 8760;
        };
        host.addEventListener('input', rememberAuto, {signal: controller.signal});
        host.addEventListener('change', rememberAuto, {signal: controller.signal});
        // A source disclosure fetches its text the first time it opens. The excerpt is
        // not part of the case payload, so a metadata poll never downloads evidence.
        host.addEventListener('toggle', async ev => {
            const node = ev.target;
            if (!node?.matches?.('.dt-source[open]') || state.busy) return;
            const id = node.dataset.id;
            const block = node.querySelector('blockquote');
            if (!id || !block || !block.hasAttribute('aria-busy')) return;
            const known = [...(state.current?.sources || []), ...archiveSources].find(s => s.id === id);
            if (known?.excerpt) { block.textContent = known.excerpt; block.removeAttribute('aria-busy'); return; }
            if (excerpts.has(id)) { block.textContent = excerpts.get(id); block.removeAttribute('aria-busy'); return; }
            const caseID = state.current?.id;
            if (!caseID) return;
            try {
                const source = await request('/cases/' + caseID + '/source?id=' + encodeURIComponent(id));
                if (state.disposed || state.current?.id !== caseID) return;
                excerpts.set(id, source.excerpt || '');
                block.textContent = source.excerpt || '';
                block.removeAttribute('aria-busy');
            } catch (err) {
                block.removeAttribute('aria-busy');
                error(err);
            }
        }, {signal: controller.signal});
        async function selectRevision(revision) {
            const id = state.current.id, generation = viewGeneration;
            const report = (state.current.reports || []).find(r => r.revision === revision) || await request('/cases/' + id + '/report?revision=' + revision);
            if (state.disposed || state.current?.id !== id || generation !== viewGeneration) return;
            selectedReport = report;
            state.revision = revision;
            state.tab = 'report';
            reportParts = [];
            draw();
        }
        host.addEventListener('change', async ev => {
            if (!ev.target.matches('.dt-revision') || state.busy) return;
            state.busy = true;
            try { await selectRevision(Number(ev.target.value)); } catch (err) { error(err); } finally { state.busy = false; syncAuto(); }
        }, {signal: controller.signal});
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
                state.busy = true;
                try {
                    const id = state.current.id;
                    if (![...(state.current.sources || []), ...archiveSources].some(s => s.id === el.dataset.ref)) {
                        const source = await request('/cases/' + id + '/source?id=' + encodeURIComponent(el.dataset.ref));
                        if (state.disposed || state.current?.id !== id) return;
                        archiveSources.push(source);
                    }
                    state.tab = 'sources';
                    draw();
                    const target = [...host.querySelectorAll('.dt-source')].find(node => node.dataset.id === el.dataset.ref);
                    if (target) { target.open = true; target.scrollIntoView({block: 'nearest'}); }
                } catch (err) { error(err); } finally { state.busy = false; syncAuto(); }
                return;
            }
            try {
                if (el.dataset.revision) { state.busy = true; await selectRevision(Number(el.dataset.revision)); return; }
                if (el.dataset.baseRevision) {
                    state.busy = true;
                    const id = state.current.id, generation = viewGeneration;
                    const report = await request('/cases/' + id + '/report?revision=' + Number(el.dataset.baseRevision));
                    if (state.disposed || state.current?.id !== id || generation !== viewGeneration) return;
                    if (!reportParts.length) reportParts = [chosenReport()];
                    reportParts.unshift(report);
                    const content = host.querySelector('.dt-content'), top = content.scrollTop, height = content.scrollHeight;
                    content.innerHTML = contentHTML();
                    content.scrollTop = top + content.scrollHeight - height;
                    return;
                }
                if (el.dataset.case) {
                    state.busy = true;
                    state.notice = '';
                    state.current = await request('/cases/' + el.dataset.case);
                    resetArchive();
                    state.events = [];
                    state.revision = 0;
                    state.latestSourceID = '';
                    state.forcePaint = true;
                    await refresh();
                    return;
                }
                const action = el.dataset.do;
                if (action === 'new') { state.notice = ''; state.current = null; resetArchive(); state.fresh = true; draw(); host.querySelector('[name=topic]')?.focus(); return; }
                if (!action || !state.current) return;
                state.busy = true;
                if (action === 'history-more' || action === 'sources-more') {
                    const id = state.current.id, generation = viewGeneration, reports = action === 'history-more';
                    const page = await request('/cases/' + id + (reports ? '/reports?before=' + historyNext : '/sources?before=' + sourceNext));
                    if (state.disposed || state.current?.id !== id || generation !== viewGeneration) return;
                    if (reports) { history = [...new Map([...history, ...(page.reports || [])].map(r => [r.revision, r])).values()]; historyNext = page.next || 0; historyLoaded = true; }
                    else { archiveSources = [...new Map([...archiveSources, ...(page.sources || [])].map(s => [s.id, s])).values()]; sourceNext = page.next || 0; sourcesLoaded = true; }
                    const content = host.querySelector('.dt-content'), top = content.scrollTop;
                    const open = [...content.querySelectorAll('.dt-source[open]')].map(n => n.dataset.id);
                    content.innerHTML = contentHTML();
                    content.querySelectorAll('.dt-source').forEach(n => { if (open.includes(n.dataset.id)) n.open = true; });
                    content.scrollTop = top;
                    syncExport();
                    return;
                }
                if (action === 'auto-save' || action === 'auto-pause') {
                    const id = state.current.id, draft = JSON.stringify(autoDrafts.get(id));
                    const settings = action === 'auto-pause' ? {...state.current.auto_update, enabled: false} : autoSettings();
                    const body = {enabled: settings.enabled, interval_hours: settings.interval_hours, report_style: settings.report_style, effort: settings.effort};
                    const updated = await request('/cases/' + id + '/auto-update', {method: 'PUT', body});
                    if (state.disposed || state.current?.id !== id) return;
                    state.current.auto_update = updated.auto_update;
                    if (JSON.stringify(autoDrafts.get(id)) === draft) autoDrafts.delete(id);
                    syncAuto();
                    await refresh();
                    return;
                }
                if (action === 'delete') {
                    if (typeof ctx.confirmDialog !== 'function') { error(new Error(tr('unavailable'))); return; }
            const accepted = await ctx.confirmDialog(tr('delete'), tr('delete_confirm'));
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
            finally { state.busy = false; syncAuto(); }
        }, {signal: controller.signal});
        (async () => { try { state.caps = await request('/capabilities'); await refresh(false); if (!state.fresh && state.cases.length) { state.current = state.cases[0]; await refresh(false); } const launch = String(ctx.context?.case_id || ctx.launch?.case_id || ''); draw(); poll(); if (launch) state.selectCase(launch); } catch (err) { draw(); error(err); } })();
    }
    // A Desktop notification names a case; the shell calls open for a running app.
    async function open(windowId, context) {
        const st = instances.get(windowId);
        const id = context && context.case_id ? String(context.case_id) : '';
        if (!st || st.disposed || !id || typeof st.selectCase !== 'function') return;
        await st.selectCase(id);
    }
    function dispose(id) { const s = instances.get(id); if (!s) return; s.disposed = true; clearTimeout(s.timer); s.controller.abort(); instances.delete(id); }
    window.DetectiveApp = {render, dispose, open};
})();
