/* Docker Manager operations: the progress drawer at the bottom of the app and the commands that start an operation.
   A classic script (window.DockerManagerOps) with no DOM work at load.
   - html(snap) builds the drawer: a status line that is all there is while nothing runs, and when opened one entry per
     operation (started here or found running on the engine) with its state, the elapsed time, a progress bar per image
     that reported a total, the failure with its line, the health-wait warning, the published ports and the output with the
     terminal colour codes removed. Leaving a stack never cancels anything; the entries stay.
   - run/prepare start an operation through the store and open the drawer; a stack that is busy attaches to the running
     operation instead of failing; done() tells the user how an operation ended.
   - tick() keeps the elapsed times moving without repainting the drawer. */
(function () {
    'use strict';

    function create(deps) {
        const { t, esc, model, stackModel: sm, views, store, toast, announce, uid } = deps;
        const { glyph, dot, chip } = views;
        const toggled = new Set();
        let open = false, ticker = 0;

        const titleOf = (kind, stack) => (stack ? t('op_title', { kind: t('op_kind_' + kind), stack }) : t('op_kind_' + kind));
        const entryTitle = op => titleOf(op.kind, op.stack);
        const stateText = op => t('op_state_' + (sm.OP_STATES.includes(op.state) ? op.state : 'failed'));
        const defaultOutput = entry => sm.isActiveOp(entry.op) || entry.op.state === 'failed' || entry.op.state === 'interrupted';
        const outputOpen = entry => (toggled.has(entry.id) ? !defaultOutput(entry) : defaultOutput(entry));

        function line(snap) {
            const entries = snap.state.ops, active = entries.filter(entry => sm.isActiveOp(entry.op));
            if (active.length) {
                const first = active[0];
                const more = active.length > 1 ? ' ' + t('ops_more', { count: active.length - 1 }) : '';
                return { tone: 'info', busy: true, text: entryTitle(first.op) + ' · ' + stateText(first.op) + more, entry: first };
            }
            if (entries.length) {
                const last = entries[0];
                return { tone: sm.OP_TONE[last.op.state] || 'idle', busy: false, text: entryTitle(last.op) + ' · ' + stateText(last.op), entry: last };
            }
            return { tone: 'idle', busy: false, text: t('ops_idle'), entry: null };
        }

        function bars(entry) {
            const items = sm.progressBars(entry);
            if (!items.length) return '';
            return '<ul class="dm-bars" aria-label="' + esc(t('ops_progress')) + '">' + items.map(item => '<li class="dm-bar"><span class="dm-bar-label" title="' + esc(item.id) + '">' + esc(item.id) + '</span>'
                + '<progress class="dm-progress" max="100" value="' + Math.round(item.percent) + '" aria-label="' + esc(item.id) + '"></progress><span class="dm-bar-pct">' + Math.round(item.percent) + '%</span></li>').join('') + '</ul>';
        }

        function entryHtml(snap, entry) {
            const op = entry.op, active = sm.isActiveOp(op), tone = sm.OP_TONE[op.state] || 'idle';
            const host = snap.hosts.find(item => item.id === op.host_id);
            const out = views.stripAnsi(entry.output), shown = outputOpen(entry);
            const parts = ['<div class="dm-op-head">' + dot(tone) + '<strong class="dm-op-title">' + esc(entryTitle(op)) + '</strong>' + (host && snap.multi ? chip(host.name, model.colorOf(host.color)) : '')
                + '<span class="dm-op-state">' + esc(stateText(op)) + '</span><span class="dm-op-time" data-elapsed="' + esc(entry.id) + '">' + esc(sm.elapsed(op, snap.now)) + '</span><span class="dm-spacer"></span>'
                + '<button type="button" class="dm-btn" data-action="op-output" data-op-id="' + esc(entry.id) + '" data-focus-id="opout:' + esc(entry.id) + '" aria-expanded="' + shown + '"><span>' + esc(shown ? t('ops_hide_output') : t('ops_show_output')) + '</span></button></div>'];
            const barsHtml = bars(entry);
            if (barsHtml) parts.push(barsHtml);
            if (op.error) {
                const jump = op.error_line > 0 && op.stack
                    ? ' <button type="button" class="dm-btn" data-action="op-goto-line" data-op-id="' + esc(entry.id) + '" data-focus-id="opline:' + esc(entry.id) + '"><span>' + esc(t('ops_show_line', { line: op.error_line })) + '</span></button>' : '';
                parts.push('<p class="dm-op-error" role="alert">' + esc(op.error) + jump + '</p>');
            }
            if (op.warning) parts.push('<p class="dm-op-warning">' + glyph('warning') + '<span>' + esc(op.warning) + '</span></p>');
            const links = op.state === 'succeeded' && Array.isArray(op.ports) ? sm.portLinks(op.ports, sm.linkAddress(host, snap.pageHost)).filter(link => link.url) : [];
            if (links.length) parts.push('<p class="dm-op-ports">' + esc(t('ops_ports')) + ' ' + links.map(link => '<a class="dm-portlink" href="' + esc(link.url) + '" target="_blank" rel="noopener noreferrer">' + esc(link.label) + '</a>').join('') + '</p>');
            if (shown) parts.push('<pre class="dm-log dm-op-output" tabindex="0" data-op-output="' + esc(entry.id) + '" aria-label="' + esc(t('ops_output_label', { title: entryTitle(op) })) + '">' + esc(out || t('ops_no_output')) + '</pre>');
            return '<li class="dm-op" data-state="' + esc(op.state) + '" data-op-id="' + esc(entry.id) + '"' + (active ? ' aria-busy="true"' : '') + '>' + parts.join('') + '</li>';
        }

        // The whole drawer. `open` only decides whether the body shows; the bar is always there.
        function html(snap) {
            const status = line(snap), entries = snap.state.ops;
            const body = entries.length ? '<ul class="dm-oplist">' + entries.map(entry => entryHtml(snap, entry)).join('') + '</ul>' : '<p class="dm-muted dm-ops-empty">' + esc(t('ops_empty')) + '</p>';
            return '<button type="button" class="dm-ops-bar" data-action="ops-toggle" data-focus-id="ops:toggle" aria-expanded="' + open + '" aria-controls="' + uid + '-ops-body">'
                + (status.busy ? '<span class="dm-spinner is-small" aria-hidden="true"></span>' : dot(status.tone)) + '<span class="dm-ops-line" data-tone="' + status.tone + '">' + esc(status.text) + '</span>'
                + (status.entry && sm.isActiveOp(status.entry.op) ? '<span class="dm-op-time" data-elapsed="' + esc(status.entry.id) + '">' + esc(sm.elapsed(status.entry.op, snap.now)) + '</span>' : '')
                + '<span class="dm-spacer"></span><span class="dm-sr-only">' + esc(open ? t('ops_collapse') : t('ops_expand')) + '</span>' + glyph('chevron', 'dm-ops-chevron') + '</button>'
                + '<div class="dm-ops-body" id="' + uid + '-ops-body"' + (open ? '' : ' hidden') + '>' + body + '</div>';
        }

        const isOpen = () => open;
        function setOpen(next) { open = !!next; }
        function toggleOutput(id) { if (toggled.has(id)) toggled.delete(id); else toggled.add(id); }

        // Updates the elapsed times in place (once a second while something runs); returns whether anything is still running.
        function tick(container) {
            const now = Date.now();
            for (const el of container.querySelectorAll('[data-elapsed]')) {
                const entry = store.state.ops.find(item => item.id === el.dataset.elapsed);
                if (entry && sm.isActiveOp(entry.op)) el.textContent = sm.elapsed(entry.op, now);
            }
            return store.state.ops.some(entry => sm.isActiveOp(entry.op));
        }
        function startTicker(container) {
            if (ticker) return;
            ticker = setInterval(() => { if (!tick(container)) stopTicker(); }, 1000);
        }
        function stopTicker() { clearInterval(ticker); ticker = 0; }

        // ---- starting operations ---------------------------------------------------------------------------------------

        // Starts an operation and opens the drawer. A busy stack is not an error: the running operation is attached.
        async function report(result, label) {
            if (result.ok) { open = true; announce(t('op_started', { title: label })); return result; }
            const error = result.error || {};
            if (error.code === 'aborted') return result;
            if (result.busy) { open = true; toast(t('op_busy', { title: label }), 'info'); return result; }
            toast(t('op_start_failed', { title: label, message: views.errorText(error) }), 'error');
            return result;
        }
        async function run(hostId, name, kind, options) {
            return report(await store.startOp(hostId, name, kind, options), titleOf(kind, name));
        }
        async function prepare(hostId) {
            return report(await store.prepareHost(hostId), titleOf('prepare', ''));
        }
        async function remove(hostId, name, volumes) {
            return report(await store.deleteStack(hostId, name, volumes), titleOf('delete', name));
        }

        // How an operation ended, for the toast and the live region. The drawer already shows the details.
        function done(entry) {
            const op = entry.op, title = entryTitle(op);
            if (op.state === 'succeeded') {
                const text = op.warning ? t('op_done_warning', { title, warning: op.warning }) : t('op_done', { title });
                toast(text, op.warning ? 'info' : 'success');
                announce(text);
            } else {
                const text = t('op_failed', { title, message: op.error || stateText(op) });
                open = true;
                toast(text, 'error');
                announce(text);
            }
        }

        return { html, isOpen, setOpen, toggleOutput, tick, startTicker, stopTicker, run, prepare, remove, done, entryTitle };
    }

    window.DockerManagerOps = { create };
})();
