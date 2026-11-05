// EasyDrag local-pack review, import uncertainty, and bounded attachment export.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const FORMAT = 'aurago.easydrag.local-pack';
    const MAX_FILE_BYTES = 32 * 1024 * 1024;
    const LEGACY_MAX_BYTES = 4 * 1024 * 1024;
    const MAX_BLOCKS = 32;
    const MAX_PREVIEW_ITEMS = 1500;
    const MAX_PREVIEW_STRING = 8000;
    const MAX_PREVIEW_CHARS = 120000;
    const REQUEST_TIMEOUT = 45000;
    const HASH = /^[a-f0-9]{64}$/;
    const pendingByWindow = new WeakMap();

    function isPackName(name) { return typeof name === 'string' && name.toLowerCase().endsWith('.easydrag-pack.json'); }
    function isPackEnvelope(value) { return !!value && typeof value === 'object' && value.format === FORMAT; }
    function nonempty(value) { return typeof value === 'string' && value.length > 0; }
    function validHash(value) { return typeof value === 'string' && HASH.test(value); }
    function blockKey(id, revision) { return String(id) + '\u0000' + String(revision); }

    function packShape(pack) {
        return isPackEnvelope(pack) && pack.version === 1 && pack.parent && typeof pack.parent === 'object' &&
            Number.isInteger(pack.parent.source_revision) && pack.parent.source_revision > 0 && pack.parent.flow &&
            validHash(pack.parent.hash) && typeof pack.parent.flow === 'object' && pack.parent.flow.kind === 'flow' &&
            nonempty(pack.parent.flow.id) && Array.isArray(pack.blocks) && pack.blocks.length <= MAX_BLOCKS &&
            pack.blocks.every(block => block && nonempty(block.source_id) && Number.isInteger(block.source_revision) &&
                block.source_revision > 0 && validHash(block.hash) && block.flow && typeof block.flow === 'object' &&
                block.flow.kind === 'block' && block.flow.id === block.source_id);
    }

    function readImportFile(file) {
        const name = String(file && file.name || '');
        const packSuffix = isPackName(name);
        const size = Number(file && file.size) || 0;
        if (size > MAX_FILE_BYTES) return Promise.resolve({ kind: packSuffix ? 'pack-too-large' : 'legacy-too-large' });
        return Promise.resolve().then(() => file.text()).then(raw => {
            let parsed;
            try { parsed = JSON.parse(raw); } catch (err) {
                if (packSuffix) return { kind: 'pack-invalid' };
                return { kind: size > LEGACY_MAX_BYTES ? 'legacy-too-large' : 'legacy-invalid' };
            }
            if (packSuffix || isPackEnvelope(parsed)) {
                if (parsed && isPackEnvelope(parsed) && parsed.version !== 1) return { kind: 'pack-unsupported' };
                if (!packShape(parsed)) return { kind: 'pack-invalid' };
                return { kind: 'pack', pack: parsed, name };
            }
            if (size > LEGACY_MAX_BYTES) return { kind: 'legacy-too-large' };
            return { kind: 'legacy', doc: parsed };
        }, () => ({ kind: packSuffix ? 'pack-invalid' : (size > LEGACY_MAX_BYTES ? 'legacy-too-large' : 'legacy-invalid') }));
    }

    function validProposal(proposal, pack, review) {
        if (!proposal || !validHash(proposal.pack_digest) || !validHash(proposal.proposal_digest) ||
            !nonempty(proposal.parent_id) || !validHash(proposal.parent_hash) || !Array.isArray(proposal.blocks) ||
            proposal.blocks.length !== pack.blocks.length || !review || !review.parent ||
            typeof review.parent.name !== 'string' || !nonempty(review.parent.source_id) || review.parent.source_id !== pack.parent.flow.id ||
            review.parent.source_revision !== pack.parent.source_revision || review.sample_data_cleared !== true ||
            review.local_only !== true || !Array.isArray(review.blocks) || review.blocks.length !== pack.blocks.length) return false;
        const sources = new Map(), reviewed = new Map(), localIDs = new Set(), targetSources = new Set();
        for (const source of pack.blocks) {
            const key = blockKey(source.source_id, source.source_revision);
            if (sources.has(key)) return false;
            sources.set(key, source);
        }
        for (const item of review.blocks) {
            const key = blockKey(item.source_id, item.source_revision);
            if (reviewed.has(key)) return false;
            reviewed.set(key, item);
        }
        return proposal.blocks.every(target => {
            const key = blockKey(target && target.source_id, target && target.source_revision);
            const source = sources.get(key), item = reviewed.get(key);
            if (!target || localIDs.has(target.local_id) || targetSources.has(key)) return false;
            localIDs.add(target.local_id);
            targetSources.add(key);
            return target && source && item && nonempty(target.source_id) && nonempty(target.local_id) && validHash(target.hash) &&
                target.source_id === source.source_id && target.source_revision === source.source_revision &&
                typeof item.name === 'string' && nonempty(item.source_id) && item.source_id === source.source_id &&
                item.source_revision === source.source_revision;
        });
    }

    function validRecords(flow, blocks, proposal, expectedPins) {
        if (!flow || flow.kind !== 'flow' || flow.id !== proposal.parent_id || flow.draft_revision !== 1 ||
            flow.live_revision !== 0 || flow.published_draft_revision !== 0 || !nonempty(flow.mission_id) || flow.hash !== proposal.parent_hash ||
            !Array.isArray(flow.pins) || flow.pins.length !== 0 || !Array.isArray(blocks) || blocks.length !== proposal.blocks.length) return false;
        const expected = new Map(proposal.blocks.map(target => [target.local_id, target.hash]));
        const seen = new Set();
        for (const block of blocks) {
            if (!block || block.kind !== 'block' || block.draft_revision !== 1 || block.live_revision !== 1 || block.published_draft_revision !== 1 ||
                (block.mission_id != null && block.mission_id !== '') || expected.get(block.id) !== block.hash || seen.has(block.id) || !Array.isArray(block.pins)) return false;
            seen.add(block.id);
            const pinKeys = new Set();
            for (const pin of block.pins) {
                const key = pin && blockKey(pin.block_id, pin.revision);
                if (!pin || expected.get(pin.block_id) !== pin.hash || pin.revision !== 1 || pinKeys.has(key)) return false;
                if (expectedPins && (!expectedPins.has(block.id) || !expectedPins.get(block.id).has(key))) return false;
                pinKeys.add(key);
            }
            if (expectedPins && expectedPins.has(block.id) && expectedPins.get(block.id).size !== pinKeys.size) return false;
        }
        return proposal.blocks.every(target => seen.has(target.local_id));
    }

    function expectedBlockPins(pack, proposal) {
        const targets = new Map(proposal.blocks.map(target => [blockKey(target.source_id, target.source_revision), target]));
        const sources = new Map(pack.blocks.map(source => [blockKey(source.source_id, source.source_revision), source]));
        const pins = new Map();
        for (const source of pack.blocks) {
            const target = targets.get(blockKey(source.source_id, source.source_revision));
            if (!target) return null;
            const expected = new Set();
            const visited = new Set();
            const visit = item => {
                const itemKey = blockKey(item.source_id, item.source_revision);
                if (visited.has(itemKey)) return true;
                visited.add(itemKey);
                const nodes = Array.isArray(item.flow.nodes) ? item.flow.nodes : [];
                for (const node of nodes) {
                    if (!node || node.type !== 'component.call') continue;
                    const params = node.params || {}, dependencyKey = blockKey(params.flow_id, params.revision);
                    const dependency = sources.get(dependencyKey), mapped = targets.get(dependencyKey);
                    if (!dependency || !mapped) return false;
                    expected.add(blockKey(mapped.local_id, 1));
                    if (!visit(dependency)) return false;
                }
                return true;
            };
            if (!visit(source)) return null;
            pins.set(target.local_id, expected);
        }
        return pins;
    }

    function exportSnapshotMatches(pack, flowId, revision) {
        return packShape(pack) && pack.parent.source_revision === revision &&
            pack.parent.flow && pack.parent.flow.id === flowId;
    }

    function capped(value, state, depth) {
        const d = depth || 0;
        if (!Number.isInteger(state.characters)) state.characters = MAX_PREVIEW_CHARS;
        if (state.left <= 0) { state.truncated = true; return '…'; }
        state.left--;
        if (typeof value === 'string') {
            const limit = Math.min(MAX_PREVIEW_STRING, state.characters);
            if (value.length <= limit) { state.characters -= value.length; return value; }
            const keep = Math.max(0, limit - 1);
            state.characters -= keep + (limit > keep ? 1 : 0);
            state.truncated = true;
            return value.slice(0, keep) + (limit > keep ? '…' : '');
        }
        if (value === null || typeof value !== 'object') return value;
        if (d >= 24) { state.truncated = true; return '…'; }
        if (Array.isArray(value)) {
            const out = [];
            let i = 0;
            for (; i < Math.min(value.length, 200) && state.left > 0 && state.characters > 0; i++) out.push(capped(value[i], state, d + 1));
            if (i < value.length) { state.truncated = true; out.push('…'); }
            return out;
        }
        const keys = Object.keys(value);
        const out = Object.create(null);
        let i = 0;
        for (; i < Math.min(keys.length, 200) && state.left > 0 && state.characters > 0; i++) {
            const key = keys[i];
            const shownKey = capped(key, state, d + 1);
            let uniqueKey = shownKey;
            for (let suffix = 2; Object.prototype.hasOwnProperty.call(out, uniqueKey); suffix++) uniqueKey = String(shownKey).slice(0, Math.max(0, MAX_PREVIEW_STRING - 12)) + '…' + suffix;
            out[uniqueKey] = capped(value[key], state, d + 1);
        }
        if (i < keys.length) state.truncated = true;
        return out;
    }

    function formatMarkup(template, values, esc) {
        const text = String(template || '');
        const pattern = /\{(name|revision|count)\}/g;
        let result = '', cursor = 0, match;
        while ((match = pattern.exec(text))) {
            result += esc(text.slice(cursor, match.index)) + esc(values[match[1]] == null ? '' : values[match[1]]);
            cursor = match.index + match[0].length;
        }
        return result + esc(text.slice(cursor));
    }

    function recordMatches(flow, blocks, proposal, pins) {
        return validRecords(flow, blocks, proposal, pins);
    }

    function createHome(app, host, options) {
        const core = ED.core, opts = options || {}, t = app.t, esc = app.esc || core.esc;
        const ctx = app.ctx;
        let disposed = false, dialog = null, current = null;
        const exports = new Set();
        const banner = host.querySelector('[data-ed-pack-status]');
        const onChanged = typeof opts.onChanged === 'function' ? opts.onChanged : () => {};

        function pending() { return pendingByWindow.get(app) || null; }
        function savePending(proposal, expectedPins) {
            const previous = pending();
            const value = { proposal, expectedPins: expectedPins || (previous && previous.proposal.proposal_digest === proposal.proposal_digest ? previous.expectedPins : null) };
            pendingByWindow.set(app, value); renderBanner(); return value;
        }
        function clearPending(value) {
            if (value && pending() === value) pendingByWindow.delete(app);
            renderBanner();
        }
        function renderBanner() {
            if (!banner || disposed) return;
            const value = pending();
            banner.hidden = !value;
            banner.innerHTML = value ? '<p>' + esc(t('easydrag.ui.pack_unknown')) + '</p><button type="button" class="ed-btn" data-ed-pack-reconcile>' +
                core.icon('refresh') + '<span>' + esc(t('easydrag.ui.pack_reconcile')) + '</span></button>' : '';
        }

        function notifyKey(key) {
            if (!disposed && ctx && typeof ctx.notify === 'function') ctx.notify({ title: t('easydrag.ui.pack_import'), message: t('easydrag.ui.' + key), type: 'error' });
        }

        async function readFile(file) {
            const result = await readImportFile(file);
            if (disposed) return { kind: 'cancelled' };
            return result;
        }

        function bodyMarkup(op) {
            if (op.phase === 'success') return '<div class="ed-pack-result"><h3>' + esc(t('easydrag.ui.pack_success')) + '</h3>' +
                reviewMarkup(op.result.flow, op.result.blocks, op.proposal, op.review, esc, t) + '<p class="ed-hint">' +
                esc(t('easydrag.ui.pack_success_detail')) + '</p></div>';
            if (op.phase === 'uncertain' || op.phase === 'reconciling' || op.phase === 'finish-confirm') return '<p class="ed-error" role="status">' + esc(t('easydrag.ui.pack_unknown')) + '</p>' +
                (op.unconfirmed ? '<p class="ed-hint">' + esc(t('easydrag.ui.pack_unconfirmed')) + '</p>' : '') +
                (op.phase === 'finish-confirm' ? '<p class="ed-hint ed-pack-warning" role="alert">' + esc(t('easydrag.ui.pack_finish_status_text')) + '</p>' : '') +
                (op.phase === 'reconciling' ? '<p class="ed-hint" aria-live="polite">' + esc(t('easydrag.ui.loading')) + '</p>' : '');
            let html = '<p class="ed-hint">' + esc(t('easydrag.ui.pack_intro')) + '</p><p class="ed-hint ed-pack-warning">' +
                esc(t('easydrag.ui.pack_warning')) + '</p>';
            if (op.phase === 'error') return html + '<p class="ed-error" role="alert">' + esc(op.error || t('easydrag.ui.pack_invalid')) + '</p>';
            if (op.phase === 'previewing' || op.phase === 'importing') return html + '<p class="ed-hint" aria-live="polite">' + esc(t('easydrag.ui.loading')) + '</p>';
            if (op.review) html += reviewMarkup(op.pack, null, op.proposal, op.review, esc, t);
            return html;
        }

        function reviewMarkup(parent, importedBlocks, proposal, review, escape, translate) {
            const isResult = !!importedBlocks;
            const parentName = isResult ? parent.name : review.parent.name;
            const reviewed = new Map((review && Array.isArray(review.blocks) ? review.blocks : []).map(item => [blockKey(item.source_id, item.source_revision), item]));
            const blocks = proposal.blocks.map(target => {
                const item = reviewed.get(blockKey(target.source_id, target.source_revision));
                const record = isResult ? importedBlocks.find(block => block.id === target.local_id) : null;
                return { name: record && record.name || item && item.name || '', source_revision: target.source_revision };
            });
            const list = blocks.map(block => '<li>' + formatMarkup(translate('easydrag.ui.pack_block_version'), { name: block.name, revision: block.source_revision }, escape) + '</li>').join('');
            let html = '<div class="ed-pack-doc-list"><p class="ed-pack-parent">' + formatMarkup(translate('easydrag.ui.pack_parent'), { name: parentName }, escape) + '</p>';
            html += '<h3>' + formatMarkup(translate('easydrag.ui.pack_blocks'), { count: blocks.length }, escape) + '</h3><ul>' + list + '</ul></div>';
            if (!isResult && parent && parent.parent && parent.parent.flow) {
                const packBlocks = new Map(parent.blocks.map(block => [blockKey(block.source_id, block.source_revision), block]));
                const docs = [{ label: formatMarkup(translate('easydrag.ui.pack_parent'), { name: review.parent.name }, escape), value: parent.parent.flow }];
                proposal.blocks.forEach(target => {
                    const item = reviewed.get(blockKey(target.source_id, target.source_revision));
                    const block = packBlocks.get(blockKey(target.source_id, target.source_revision));
                    if (block && item) docs.push({
                        label: formatMarkup(translate('easydrag.ui.pack_block_version'), { name: item.name, revision: target.source_revision }, escape),
                        value: block.flow
                    });
                });
                html += '<section class="ed-pack-content"><label class="ed-label">' + escape(translate('easydrag.ui.pack_content')) +
                    '</label><select class="ed-input" data-ed-pack-doc aria-label="' + escape(translate('easydrag.ui.pack_content')) + '">' +
                    docs.map((doc, i) => '<option value="' + i + '">' + doc.label + '</option>').join('') + '</select>' +
                    '<div class="ed-seg ed-seg--small" role="group" aria-label="' + escape(translate('easydrag.ui.pack_content')) + '"><button type="button" data-ed-pack-view="tree" aria-pressed="true">' + escape(translate('easydrag.ui.view_tree')) +
                    '</button><button type="button" data-ed-pack-view="json" aria-pressed="false">' + escape(translate('easydrag.ui.view_json')) + '</button></div>' +
                    '<div class="ed-pack-document" data-ed-pack-document></div></section>';
            }
            return html;
        }

        function renderDocument(op) {
            const select = op.dialog.body.querySelector('[data-ed-pack-doc]');
            const target = op.dialog.body.querySelector('[data-ed-pack-document]');
            if (!select || !target) return;
            const index = Math.max(0, Math.min(op.docs.length - 1, Number(select.value) || 0));
            const state = { left: MAX_PREVIEW_ITEMS, characters: MAX_PREVIEW_CHARS, truncated: false };
            const value = capped(op.docs[index].value, state, 0);
            const tree = op.view !== 'json';
            const rendered = tree && ED.detail ? ED.detail.jsonTree(value, esc, 0, { left: MAX_PREVIEW_ITEMS }) : '<pre class="ed-code ed-output-json">' + esc(JSON.stringify(value, null, 2)) + '</pre>';
            target.innerHTML = (state.truncated ? '<p class="ed-hint">' + esc(t('easydrag.ui.output_truncated')) + '</p>' : '') + rendered;
        }

        function render(op) {
            if (!alive(op)) return;
            const active = document.activeElement;
            op.dialog.body.innerHTML = bodyMarkup(op);
            if (op.phase === 'success' && op.renderedPhase !== 'success') op.dialog.body.scrollTop = 0;
            op.renderedPhase = op.phase;
            const actions = {
                preview: op.phase === 'ready',
                import: op.phase === 'review' && !(pending() && pending().proposal.pack_digest === op.proposal.pack_digest),
                reconcile: op.phase === 'uncertain',
                finish: op.phase === 'finish-confirm' || (op.phase === 'uncertain' && op.unconfirmed),
                open: op.phase === 'success'
            };
            Object.keys(actions).forEach(id => {
                const button = op.dialog.el.querySelector('[data-ed-action="' + id + '"]');
                if (!button) return;
                button.hidden = !actions[id];
                button.disabled = !!op.busy || (id === 'import' && !canWrite());
            });
            const close = op.dialog.el.querySelector('[data-ed-action="cancel"]');
            if (close) close.disabled = false;
            if (op.phase === 'review') {
                const packBlocks = new Map(op.pack.blocks.map(block => [blockKey(block.source_id, block.source_revision), block]));
                op.docs = [{ value: op.pack.parent.flow }].concat(op.proposal.blocks.map(target => ({ value: packBlocks.get(blockKey(target.source_id, target.source_revision))?.flow })));
                renderDocument(op);
            }
            if (active && op.dialog.el.contains(active) && (active.hidden || active.disabled)) {
                const target = op.dialog.el.querySelector('[data-ed-action="open"]:not([hidden]), [data-ed-action="reconcile"]:not([hidden]), [data-ed-action="import"]:not([hidden]), [data-ed-action="preview"]:not([hidden]), [data-ed-action="cancel"]');
                if (target) target.focus({ preventScroll: true });
            }
        }

        function canWrite() { return !disposed && typeof opts.canWrite === 'function' && opts.canWrite(); }
        function alive(op) { return !disposed && current === op && dialog === op.dialog && !op.closed && op.dialog && op.dialog.el.isConnected && !op.dialog.el.closest('.is-closing'); }
        function schedule(op, fn) {
            op.startTimer = setTimeout(() => { op.startTimer = 0; if (alive(op)) fn(); }, 0);
        }

        async function request(op, fn, mutation) {
            const control = op.control = new AbortController();
            op.timedOut = false;
            const timer = setTimeout(() => { op.timedOut = true; control.abort(); }, REQUEST_TIMEOUT);
            try { return await fn(control.signal); }
            catch (err) {
                if (mutation && !definiteRefusal(err)) savePending(op.proposal, op.expectedPins);
                throw err;
            } finally {
                clearTimeout(timer);
                if (op.control === control) op.control = null;
            }
        }

        function definiteRefusal(err) {
            return !!(err && [400, 413, 422].includes(Number(err.status)) && core.errorCode(err) !== 'FLOW_PACK_CONFLICT');
        }

        function openDialog(op) {
            if (disposed || dialog) return;
            current = op;
            const owned = dialog = core.modal(host, {
                title: t('easydrag.ui.pack_import'), closeLabel: t('easydrag.ui.close'), className: 'ed-modal--pack',
                body: bodyMarkup(op),
                actions: [
                    { id: 'cancel', label: t('easydrag.ui.close') },
                    { id: 'preview', label: t('easydrag.ui.pack_preview'), primary: true },
                    { id: 'import', label: t('easydrag.ui.pack_import_action'), primary: true },
                    { id: 'reconcile', label: t('easydrag.ui.pack_reconcile'), primary: true },
                    { id: 'finish', label: t('easydrag.ui.pack_finish_status') },
                    { id: 'open', label: t('easydrag.ui.pack_open'), primary: true }
                ],
                onAction(id, instance) {
                    if (id === 'cancel') { instance.close('cancel'); return false; }
                    if (id === 'preview' && op.phase === 'ready') schedule(op, () => { void preview(op); });
                    else if (id === 'import' && op.phase === 'review') schedule(op, () => { void importReviewed(op); });
                    else if (id === 'reconcile' && op.phase === 'uncertain') schedule(op, () => { void reconcile(op); });
                    else if (id === 'finish' && op.phase === 'uncertain' && op.unconfirmed) { op.phase = 'finish-confirm'; render(op); }
                    else if (id === 'finish' && op.phase === 'finish-confirm') {
                        const value = pending();
                        if (value && value.proposal.proposal_digest === op.proposal.proposal_digest) clearPending(value);
                        op.abandonAccepted = true;
                        instance.close('finish');
                        return false;
                    }
                    else if (id === 'open' && op.phase === 'success' && op.result.flow) {
                        const id = op.result.flow.id;
                        instance.close('open');
                        if (typeof app.openFlow === 'function') app.openFlow(id);
                        return false;
                    }
                    return false;
                }
            });
            op.dialog = owned;
            op.closed = false;
            owned.body.addEventListener('change', event => {
                if (event.target.matches('[data-ed-pack-doc]')) renderDocument(op);
            });
            owned.body.addEventListener('click', event => {
                const button = event.target.closest('[data-ed-pack-view]');
                if (!button) return;
                op.view = button.dataset.edPackView === 'json' ? 'json' : 'tree';
                owned.body.querySelectorAll('[data-ed-pack-view]').forEach(item => item.setAttribute('aria-pressed', String(item === button)));
                renderDocument(op);
            });
            owned.done.then(() => {
                op.closed = true;
                if (op.startTimer) clearTimeout(op.startTimer);
                if (op.sent && !op.confirmed && !op.definiteFailure && !op.abandonAccepted) savePending(op.proposal, op.expectedPins);
                if (op.control) op.control.abort();
                if (current === op) current = null;
                if (dialog === owned) dialog = null;
                renderBanner();
            });
            render(op);
        }

        function openPending() {
            const value = pending();
            if (!value || dialog || disposed) return;
            openDialog({ phase: 'uncertain', proposal: value.proposal, expectedPins: value.expectedPins, pendingValue: value, unconfirmed: false, view: 'tree' });
        }

        function startReview(pack) {
            if (pending()) { openPending(); return; }
            openDialog({ phase: 'ready', pack, proposal: null, review: null, result: null, view: 'tree', docs: [] });
        }

        async function preview(op) {
            if (!alive(op) || op.busy) return;
            op.phase = 'previewing'; op.busy = true; render(op);
            try {
                const result = await request(op, signal => app.api.previewPack(op.pack, signal), false);
                if (!alive(op)) return;
                if (!result || !validProposal(result.proposal, op.pack, result.review)) {
                    op.error = t('easydrag.ui.pack_invalid'); op.phase = 'error';
                    return;
                }
                op.proposal = result.proposal; op.review = result.review; op.phase = 'review';
            } catch (err) {
                if (!alive(op)) return;
                op.error = core.errorText(t, err); op.phase = 'error';
            } finally { op.busy = false; if (alive(op)) render(op); }
        }

        async function importReviewed(op) {
            if (!alive(op) || op.busy || !canWrite() || !op.pack || !op.proposal) return;
            op.expectedPins = expectedBlockPins(op.pack, op.proposal);
            if (!op.expectedPins) {
                op.error = t('easydrag.ui.pack_invalid'); op.phase = 'error'; render(op);
                return;
            }
            op.sent = true; op.busy = true; op.phase = 'importing';
            const value = savePending(op.proposal, op.expectedPins);
            op.pendingValue = value;
            render(op);
            const pack = op.pack;
            let pendingRequest;
            try {
                pendingRequest = request(op, signal => app.api.importPack(pack, op.proposal, signal), true);
                op.pack = null;
                op.docs = [];
                const result = await pendingRequest;
                if (!alive(op)) return;
                if (!result || result.status !== 'applied' || !recordMatches(result.flow, result.blocks, op.proposal, op.expectedPins)) throw new Error('Unconfirmed local pack import');
                op.result = result; op.confirmed = true; op.phase = 'success'; clearPending(value); onChanged();
            } catch (err) {
                if (definiteRefusal(err)) {
                    op.definiteFailure = true; clearPending(value);
                    if (alive(op)) { op.error = core.errorText(t, err); op.phase = 'error'; }
                } else {
                    savePending(op.proposal, op.expectedPins);
                    if (alive(op)) { op.phase = 'uncertain'; op.pendingValue = pending(); op.unconfirmed = false; }
                }
            } finally { op.busy = false; if (alive(op)) render(op); }
        }

        async function reconcile(op) {
            if (!alive(op) || op.busy || !op.proposal) return;
            op.phase = 'reconciling'; op.busy = true; render(op);
            try {
                const result = await request(op, signal => app.api.reconcilePack(op.proposal, signal), false);
                if (!alive(op)) return;
                if (result && result.status === 'applied_state_confirmed' && recordMatches(result.flow, result.blocks, op.proposal, op.expectedPins)) {
                    op.result = result; op.confirmed = true; op.phase = 'success'; clearPending(op.pendingValue || pending()); onChanged();
                } else {
                    op.phase = 'uncertain'; op.unconfirmed = true;
                }
            } catch (err) {
                if (!alive(op)) return;
                op.phase = 'uncertain'; op.unconfirmed = true;
            } finally { op.busy = false; if (alive(op)) render(op); }
        }

        const onBannerClick = event => { if (event.target?.closest?.('[data-ed-pack-reconcile]')) openPending(); };
        banner?.addEventListener('click', onBannerClick);
        renderBanner();
        return {
            readImportFile: readFile,
            review: startReview,
            async exportStored(id, name) {
                if (disposed) return false;
                const control = new AbortController(); exports.add(control);
                try { return await downloadPack(app.api.localPackUrl(id), name, control.signal, () => !disposed); }
                catch (err) {
                    if (!disposed && ctx && typeof ctx.notify === 'function') ctx.notify({ title: t('easydrag.ui.pack_export'), message: core.errorText(t, err), type: 'error' });
                    return false;
                } finally { exports.delete(control); }
            },
            hasPending() { return !!pending(); },
            openPending,
            dispose() {
                disposed = true;
                if (current && current.dialog) current.dialog.close('dispose');
                if (banner) banner.removeEventListener('click', onBannerClick);
                exports.forEach(control => control.abort());
            }
        };
    }

    function packFilename(header, fallback) {
        let name = '';
        const disposition = String(header || '');
        const extended = /filename\*=UTF-8''([^;]+)/i.exec(disposition);
        const plain = /filename="?([^";]+)"?/i.exec(disposition);
        try { name = extended ? decodeURIComponent(extended[1]) : (plain ? plain[1] : ''); } catch (err) { name = ''; }
        if (!name) name = fallback || 'flow';
        name = name.split(/[\\/]/).pop().replace(/[\u0000-\u001f:*?"<>|]/g, '_').trim().slice(0, 80) || 'flow';
        return name.toLowerCase().endsWith('.easydrag-pack.json') ? name : name.replace(/\.json$/i, '') + '.easydrag-pack.json';
    }

    async function downloadPack(url, fallback, signal, guard) {
        const response = await fetch(url, { credentials: 'same-origin', cache: 'no-store', signal });
        if (!response.ok) {
            let body = {};
            try { body = await response.json(); } catch (err) { /* retain a generic HTTP error */ }
            const error = new Error(body.error || ('HTTP ' + response.status)); error.body = body; error.status = response.status; throw error;
        }
        const length = Number(response.headers.get('content-length')) || 0;
        if (length > MAX_FILE_BYTES) throw Object.assign(new Error('Local pack too large'), { body: { code: 'FLOW_PACK_TOO_LARGE' }, status: 413 });
        const blob = await response.blob();
        if (!guard() || blob.size > MAX_FILE_BYTES) return false;
        const objectURL = URL.createObjectURL(blob), anchor = document.createElement('a');
        anchor.href = objectURL;
        anchor.download = packFilename(response.headers.get('content-disposition'), fallback);
        document.body.appendChild(anchor); anchor.click(); anchor.remove();
        setTimeout(() => URL.revokeObjectURL(objectURL), 10000);
        return true;
    }

    async function exportEditor(ed) {
        if (!ed || !ed.api || !ed.draftModel || !ed.saver || ed.packExportControl) return false;
        if (!ed.readonly && (!ed.flushPendingEdits() || !await ed.saver.flush())) return false;
        if (ed.saver.isDirty()) return false;
        const version = ed.draftModel.version, revision = ed.saver.revision, control = new AbortController();
        ed.packExportControl = control;
        const current = () => !control.signal.aborted && ed.root.isConnected && ed.draftModel.version === version &&
            ed.saver.revision === revision && !ed.saver.isDirty();
        try {
            if (!current()) return false;
            const url = ed.api.localPackUrl(ed.flow.id);
            const response = await fetch(url, { credentials: 'same-origin', cache: 'no-store', signal: control.signal });
            if (!response.ok) {
                let body = {};
                try { body = await response.json(); } catch (err) { /* retain a generic HTTP error */ }
                const error = new Error(body.error || ('HTTP ' + response.status)); error.body = body; error.status = response.status; throw error;
            }
            const length = Number(response.headers.get('content-length')) || 0;
            if (length > MAX_FILE_BYTES) throw Object.assign(new Error('Local pack too large'), { body: { code: 'FLOW_PACK_TOO_LARGE' }, status: 413 });
            const blob = await response.blob();
            if (!current() || blob.size > MAX_FILE_BYTES) return false;
            let exported;
            try { exported = JSON.parse(await blob.text()); }
            catch (err) { throw Object.assign(new Error('Invalid local pack response'), { body: { code: 'FLOW_PACK_INVALID' }, status: 400 }); }
            if (!exportSnapshotMatches(exported, ed.flow.id, revision)) {
                if (ed.ctx && typeof ed.ctx.notify === 'function') ed.ctx.notify({ title: ed.t('easydrag.ui.pack_export'), message: ED.core.tr(ed.t, 'easydrag.ui.error_flow_revision_conflict', ed.t('easydrag.ui.pack_invalid')), type: 'warning' });
                return false;
            }
            if (!current()) return false;
            const objectURL = URL.createObjectURL(blob), anchor = document.createElement('a');
            anchor.href = objectURL;
            anchor.download = packFilename(response.headers.get('content-disposition'), ed.draftModel.doc.name);
            document.body.appendChild(anchor); anchor.click(); anchor.remove();
            setTimeout(() => URL.revokeObjectURL(objectURL), 10000);
            return true;
        } catch (err) {
            if (!control.signal.aborted && ed.ctx && typeof ed.ctx.notify === 'function') {
                ed.ctx.notify({ title: ed.t('easydrag.ui.pack_export'), message: ED.core.errorText(ed.t, err), type: 'error' });
            }
            return false;
        } finally { if (ed.packExportControl === control) ed.packExportControl = null; }
    }

    ED.pack = {
        FORMAT, MAX_FILE_BYTES, readImportFile, isPackName, isPackEnvelope, packShape, validProposal,
        validRecords, expectedBlockPins, exportSnapshotMatches, capped, formatMarkup, createHome, exportEditor,
        _test: { readImportFile, isPackName, isPackEnvelope, packShape, validProposal, validRecords, expectedBlockPins, exportSnapshotMatches, capped, formatMarkup, packFilename }
    };
})();
