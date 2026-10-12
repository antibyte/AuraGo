/* Gallery descriptions, rule-based categories and explicitly requested vision analysis. */
(function () {
    'use strict';
    const BASE = '/api/desktop/gallery/';
    const splitTags = value => String(value || '').split(',').map(s => s.trim()).filter(Boolean);

    function categoryMatches(item, category) {
        if (!category) return true;
        const m = item.metadata || {};
        const text = `${m.description || ''} ${m.prompt || ''} ${m.source || ''} ${(m.tags || []).join(' ')}`.toLocaleLowerCase();
        return category.keywords.some(word => text.includes(word.toLocaleLowerCase()));
    }

    function editorHTML(v, item) {
        const { t, esc, readonly } = v;
        const m = item.metadataDraft || item.metadata || {};
        return `<section class="vd-gallery-metadata-editor" data-metadata-path="${esc(item.path)}" data-metadata-version="${esc(m.version || '')}">
            <label>${esc(t('desktop.gallery_description'))}<textarea rows="4" maxlength="16000" data-gallery-description${readonly ? ' readonly' : ''}>${esc(m.description || '')}</textarea></label>
            <label>${esc(t('desktop.gallery_tags'))}<input type="text" data-gallery-tags maxlength="10000" value="${esc((m.tags || []).join(', '))}"${readonly ? ' readonly' : ''}></label>
            <p class="vd-gallery-info-hint">${esc(t('desktop.gallery_metadata_hint'))}</p>
            ${m.source ? `<p class="vd-gallery-info-hint">${esc(t('desktop.gallery_source'))}: ${esc(m.source)}</p>` : ''}
            ${m.prompt ? `<details><summary>${esc(t('desktop.gallery_prompt'))}</summary><p>${esc(m.prompt)}</p></details>` : ''}
            ${readonly ? '' : `<div class="vd-gallery-info-actions"><button type="button" class="vd-button" data-gallery-metadata-save${m.version ? '' : ' disabled'}>${esc(t('desktop.save'))}</button><button type="button" class="vd-button" data-gallery-analyze${m.version ? '' : ' disabled'}>${esc(t('desktop.gallery_analyze'))}</button></div>`}
        </section>`;
    }

    function create({ root, g, ctx, t, esc, api, readonly, refresh }) {
        let categories = [], categoryVersion = '', entries = {}, empty = {}, ready = false;
        let busy = false, controller = null, dialog = null, dirtyPath = '', blocked = readonly;
        const drafts = new Map();
        const select = root.querySelector('[data-gallery-category]');
        const status = root.querySelector('[data-gallery-metadata-status]');
        const stop = root.querySelector('[data-gallery-analysis-stop]');
        const text = key => t('desktop.' + key);
        function message(key, vars) { status.textContent = t('desktop.' + key, vars); }
        function controls() {
            root.querySelectorAll('[data-gallery-description], [data-gallery-tags]').forEach(input => { input.disabled = busy || blocked; });
            root.querySelectorAll('[data-gallery-analyze], [data-gallery-analyze-all], [data-gallery-metadata-save], [data-gallery-categories]').forEach(button => {
                button.disabled = busy || blocked || !ready || (button.hasAttribute('data-gallery-analyze-all') && (g.loading || !g.complete || g.tab !== 'Photos'));
            });
            select.disabled = g.tab !== 'Photos';
            stop.hidden = !busy;
        }
        function updateCategories() {
            const previous = select.value;
            select.innerHTML = `<option value="">${esc(text('gallery_category_all'))}</option>` + categories.map(c => `<option value="${esc(c.name)}">${esc(c.name)}</option>`).join('');
            select.value = previous;
            if (select.selectedIndex < 0) select.value = '';
        }
        async function load(signal) {
            try {
                const [metadata, rules] = await Promise.all([api(BASE + 'metadata', { signal }), api(BASE + 'categories', { signal })]);
                if (g.disposed || (signal && signal.aborted)) return;
                entries = metadata.items || {}; empty = metadata.empty || {};
                categories = rules.categories || []; categoryVersion = rules.version; ready = true;
                updateCategories();
                if (!busy && !dirtyPath) status.textContent = '';
            } catch (err) {
                if (g.disposed || (signal && signal.aborted)) return;
                ready = false;
                message('gallery_metadata_unavailable');
            }
            controls();
        }
        function enrich(item) {
            if (item.path.startsWith('Photos/')) item.metadata = entries[item.path] || empty;
            item.metadataDraft = drafts.get(item.path);
            return item;
        }
        function keepDraft(path) {
            controls();
            const form = root.querySelector('[data-metadata-path]');
            return !!dirtyPath && dirtyPath === path && form && form.dataset.metadataPath === path;
        }
        function errorMessage(err) {
            const code = err && (err.status || (err.body && err.body.status));
            message(code === 409 ? 'gallery_metadata_conflict' : 'gallery_metadata_failed');
        }
        async function write(item, method, values, signal) {
            const result = await api(BASE + 'metadata', { method, signal, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(Object.assign({ path: item.path, version: item.metadata && item.metadata.version }, values)) });
            if (g.disposed) return;
            entries[item.path] = result;
            drafts.delete(item.path);
            const current = g.byPath.get(item.path);
            if (current) { current.metadata = result; current.metadataDraft = null; }
            if (dirtyPath === item.path) dirtyPath = '';
            refresh();
        }
        async function saveMetadata() {
            if (busy || blocked || !ready) return;
            const form = root.querySelector('[data-metadata-path]');
            if (!form) return;
            const item = { path: form.dataset.metadataPath, metadata: { version: form.dataset.metadataVersion } };
            busy = true; controller = new AbortController(); controls();
            try {
                await write(item, 'PUT', { description: form.querySelector('[data-gallery-description]').value, tags: splitTags(form.querySelector('[data-gallery-tags]').value) }, controller.signal);
                if (!g.disposed) message('gallery_metadata_saved');
            } catch (err) { if (!g.disposed) errorMessage(err); }
            finally { busy = false; controller = null; if (!g.disposed) controls(); }
        }
        async function analyze(items, all) {
            if (busy || blocked || !ready || !items.length) return;
            busy = true; controls();
            try {
                const confirmed = await ctx.confirmDialog(text(all && items.length > 20 ? 'gallery_analysis_cost_title' : 'gallery_analyze'), t('desktop.' + (all && items.length > 20 ? 'gallery_analysis_cost_warning' : 'gallery_analysis_confirm'), { count: items.length }));
                if (!confirmed || g.disposed || blocked) return;
                controller = new AbortController();
                let done = 0, failed = 0;
                for (const item of items) {
                    if (g.disposed || controller.signal.aborted || blocked) break;
                    message('gallery_analysis_progress', { done, total: items.length, failed });
                    try { await write(item, 'POST', {}, controller.signal); done += 1; }
                    catch (err) { if (controller.signal.aborted) break; failed += 1; }
                }
                if (!g.disposed) message(controller.signal.aborted ? 'gallery_analysis_cancelled' : 'gallery_analysis_finished', { done, failed });
            } finally { busy = false; controller = null; if (!g.disposed) controls(); }
        }
        function closeDialog() {
            if (!dialog) return;
            dialog.remove(); dialog = null;
            root.querySelector('[data-gallery-categories]').focus();
        }
        function manageCategories() {
            if (busy || blocked || !ready || dialog) return;
            const originalVersion = categoryVersion;
            const draft = categories.map(c => ({ name: c.name, keywords: c.keywords.slice() }));
            dialog = document.createElement('div'); dialog.className = 'vd-gallery-dialog-backdrop';
            dialog.innerHTML = `<section class="vd-gallery-category-dialog" role="dialog" aria-modal="true" aria-label="${esc(text('gallery_categories_manage'))}">
                <h3>${esc(text('gallery_categories_manage'))}</h3><p>${esc(text('gallery_categories_hint'))}</p>
                <select size="5" data-category-list aria-label="${esc(text('gallery_category'))}"></select>
                <label>${esc(text('gallery_category_name'))}<input maxlength="100" data-category-name></label>
                <label>${esc(text('gallery_category_keywords'))}<input maxlength="3000" data-category-keywords></label>
                <div class="vd-gallery-info-actions"><button class="vd-button" data-category-apply>${esc(text('gallery_category_apply'))}</button><button class="vd-button" data-category-new>${esc(text('gallery_category_new'))}</button><button class="vd-button" data-category-delete>${esc(text('gallery_delete'))}</button></div>
                <p data-category-error role="status"></p><div class="vd-gallery-info-actions"><button class="vd-button" data-category-save>${esc(text('save'))}</button><button class="vd-button" data-category-close>${esc(text('cancel'))}</button></div>
            </section>`;
            root.appendChild(dialog);
            const panel = dialog, list = panel.querySelector('[data-category-list]'), name = panel.querySelector('[data-category-name]'), words = panel.querySelector('[data-category-keywords]'), error = panel.querySelector('[data-category-error]');
            let index = -1, saving = false;
            function repaint() { list.innerHTML = draft.map((c, i) => `<option value="${i}">${esc(c.name)}</option>`).join(''); list.value = String(index); }
            function choose(i) { index = i; name.value = draft[i] ? draft[i].name : ''; words.value = draft[i] ? draft[i].keywords.join(', ') : ''; repaint(); }
            function apply() {
                const c = { name: name.value.trim(), keywords: splitTags(words.value) };
                if (!c.name || !c.keywords.length || c.keywords.length > 30 || c.keywords.some(word => word.length > 100) || draft.some((other, i) => i !== index && other.name.toLowerCase() === c.name.toLowerCase()) || (index < 0 && draft.length >= 50)) { error.textContent = text('gallery_category_invalid'); return false; }
                if (index < 0) { index = draft.length; draft.push(c); } else draft[index] = c;
                error.textContent = ''; repaint(); return true;
            }
            list.addEventListener('change', () => choose(Number(list.value)));
            panel.addEventListener('click', async event => {
                if (saving) return;
                if (event.target.closest('[data-category-close]')) closeDialog();
                if (event.target.closest('[data-category-apply]')) apply();
                if (event.target.closest('[data-category-new]')) choose(-1);
                if (event.target.closest('[data-category-delete]') && index >= 0) { draft.splice(index, 1); choose(-1); }
                if (!event.target.closest('[data-category-save]')) return;
                if ((name.value.trim() || words.value.trim()) && !apply()) return;
                saving = true;
                panel.querySelectorAll('button, input, select').forEach(e => { e.disabled = true; });
                try {
                    const result = await api(BASE + 'categories', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ categories: draft, version: originalVersion }) });
                    if (g.disposed) return;
                    categories = result.categories; categoryVersion = result.version; select.value = ''; updateCategories(); closeDialog(); refresh();
                } catch (_) { if (!g.disposed) error.textContent = text('gallery_categories_failed'); }
                finally { saving = false; panel.querySelectorAll('button, input, select').forEach(e => { e.disabled = false; }); }
            });
            panel.addEventListener('keydown', event => {
                if (event.key === 'Escape' && !saving) { event.preventDefault(); closeDialog(); }
                if (event.key === 'Tab') {
                    const focusable = Array.from(panel.querySelectorAll('button, input, select')).filter(e => !e.disabled);
                    const first = focusable[0], last = focusable[focusable.length - 1];
                    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
                    if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
                }
                event.stopPropagation();
            });
            repaint(); name.focus();
        }
        select.addEventListener('change', refresh);
        root.addEventListener('input', event => {
            if (!event.target.matches('[data-gallery-description], [data-gallery-tags]')) return;
            const form = event.target.closest('[data-metadata-path]');
            dirtyPath = form.dataset.metadataPath;
            const item = g.byPath.get(dirtyPath);
            const draft = Object.assign({}, item && item.metadata, { version: form.dataset.metadataVersion, description: form.querySelector('[data-gallery-description]').value, tags: splitTags(form.querySelector('[data-gallery-tags]').value) });
            drafts.set(dirtyPath, draft);
            if (item) item.metadataDraft = draft;
        });
        root.addEventListener('click', event => {
            if (event.target.closest('[data-gallery-search-reset]')) { select.value = ''; refresh(); }
            if (event.target.closest('[data-gallery-analysis-stop]')) { if (controller) controller.abort(); return; }
            if (event.target.closest('[data-gallery-categories]')) manageCategories();
            if (event.target.closest('[data-gallery-metadata-save]')) saveMetadata();
            if (event.target.closest('[data-gallery-analyze-all]') && g.complete && g.tab === 'Photos') analyze(g.items.slice(), true);
            if (event.target.closest('[data-gallery-analyze]')) { const item = g.byPath.get(g.focusPath); if (item) analyze([item], false); }
        });
        function onPolicy(event) {
            const detail = event.detail || {};
            blocked = readonly || detail.enabled === false || !!detail.readonly;
            if (blocked && controller) controller.abort();
            if (blocked) closeDialog();
            controls();
        }
        document.addEventListener('aurago:desktop-policy', onPolicy);
        return { load, enrich, sync: controls, category: () => g.tab === 'Photos' ? select.value : '', matches: item => g.tab !== 'Photos' || categoryMatches(item, categories.find(c => c.name === select.value)), keepDraft,
            dispose() { if (controller) controller.abort(); if (dialog) dialog.remove(); document.removeEventListener('aurago:desktop-policy', onPolicy); }
        };
    }
    window.GalleryMetadata = { create, editorHTML, categoryMatches };
})();
