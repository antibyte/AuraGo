// HomeDash designer dialogs (M4 part 2, D8, D10; contract C20 E.ddialogs): run(ctx, action) for new / rename / import / export / look / add_page, the page tab
// menu and enhanceTabs (add button, icons, inline rename, Alt+Shift+Arrow and drag reorder on part 1's tab strip), export and import of dashboard files.
// Import is untrusted: the 512 KiB cap is checked before the file is read, the text is parsed and sanitised here and judged by POST dashboards/validate
// before anything is created or rendered. Names and issue paths reach the page only through textContent. Forms run on E.sheet (E.form.formSheet).
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.form || !E.look || !E.commands || !E.save || !E.doc) throw new Error('hd-designer-dialogs.js: load hd-form.js, hd-look.js, hd-commands.js and hd-save.js first');
    const { el, setAttr, setVars } = E.core, F = E.form, C = E.commands;
    const ACTIONS = Object.freeze(['new', 'rename', 'import', 'export', 'look', 'add_page']), IMPORT_MAX = 524288, NAME_MAX = 80, PAGE_NAME_MAX = 40, STRIP = Object.freeze(['id', 'revision', 'approved_revision', 'saved_by', 'updated_at']);
    const FOLD = new RegExp('[' + String.fromCodePoint(0x300) + '-' + String.fromCodePoint(0x36f) + ']', 'g'), isObj = v => v !== null && typeof v === 'object' && !Array.isArray(v);
    const slugify = name => String(name || '').normalize('NFD').replace(FOLD, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '').slice(0, 48).replace(/-+$/, '') || 'dashboard';
    // Export (D10): {schema_version, dashboard} of E.doc.forSave (drafts dropped, theme snapshot) without server fields; the kiosk block stays.
    function exportFile(doc, o) { const opts = o || {}, body = E.doc.forSave(doc, { catalog: opts.catalog, snapshot: opts.snapshot }), dashboard = {};
        for (const k of Object.keys(body)) if (!STRIP.includes(k)) dashboard[k] = E.core.clone(body[k]);
        const slug = typeof dashboard.slug === 'string' && /^[a-z0-9][a-z0-9-]{0,63}$/.test(dashboard.slug) ? dashboard.slug : slugify(dashboard.name);
        return Object.freeze({ filename: 'homedash-' + slug + '.json', text: JSON.stringify({ schema_version: Number(dashboard.schema_version) || 1, dashboard }, null, 2) + '\n', drafts: opts.catalog ? E.doc.drafts(doc, opts.catalog).size : 0 }); }
    // Import (D10): untrusted text; size cap again after reading; envelope {schema_version, dashboard} or a bare document with pages; id and slug
    // cleared, server fields removed; everything else is judged by POST dashboards/validate before anything is created or rendered.
    function overLimits(d) {
        const L = C.LIMITS, pages = d.pages; if (pages.length > L.pages) return true; let total = 0;
        for (const p of pages) {
            if (!isObj(p)) continue; if (Array.isArray(p.widgets)) { if (p.widgets.length > L.pageWidgets) return true; total += p.widgets.length; if (total > L.widgets) return true; }
            if (isObj(p.layouts)) for (const bp of ['lg', 'md', 'sm']) if (Array.isArray(p.layouts[bp]) && p.layouts[bp].length > L.pageWidgets) return true;
        }
        return false;
    }
    function importDocument(text) {
        if (typeof text !== 'string' || !text) return { ok: false, code: 'bad_json' }; if (E.save.utf8Bytes(text) > IMPORT_MAX) return { ok: false, code: 'too_large' };
        let data; try { data = JSON.parse(text.charCodeAt(0) === 0xfeff ? text.slice(1) : text); } catch (err) { return { ok: false, code: 'bad_json' }; }
        const d = isObj(data) && isObj(data.dashboard) && !Array.isArray(data.pages) ? data.dashboard : isObj(data) && Array.isArray(data.pages) ? data : null;
        if (!d || !Array.isArray(d.pages)) return { ok: false, code: 'bad_json' };
        if (overLimits(d)) return { ok: false, code: 'limit_exceeded' };   // S1: before normalise places anything; the server limits (pages, widgets per page, widgets, layout rows per page)
        const doc = E.core.clone(d); for (const k of STRIP) delete doc[k]; doc.id = ''; doc.slug = '';
        if (d !== data && Number.isInteger(data.schema_version) && doc.schema_version === undefined) doc.schema_version = data.schema_version;
        return { ok: true, document: doc };
    }
    const nameError = (raw, max) => { const n = Array.from(String(raw == null ? '' : raw).trim()).length; return n === 0 ? 'empty' : n > max ? 'long' : ''; };
    const newChoices = o => (o && o.readonly ? [] : [{ kind: 'starter', icon: 'gauge', primary: true }, { kind: 'empty', icon: 'dashboard' }].concat(o && Array.isArray(o.templates) && o.templates.includes('home_assistant') ? [{ kind: 'ha', icon: 'home' }] : []));
    // Drop index of a dragged tab among the others (C5 movePage target): the number of other tabs whose midpoint lies left of x.
    const targetIndex = (rects, x, from) => rects.reduce((n, r, i) => (i !== from && x > r.left + r.width / 2 ? n + 1 : n), 0);
    const copyName = (t, name) => { const sfx = t('homedash.conflict.copy_suffix'); return Array.from(String(name || '')).slice(0, Math.max(1, PAGE_NAME_MAX - Array.from(sfx).length)).join('') + sfx; };

    // ---- sheets: the designer's own, else one per visible host (the engine root is hidden in home mode, so the host element carries the sheet) ----
    const hostSheets = new WeakMap();
    function sheetOf(ctx) {
        if (ctx.designer) return ctx.designer.sheet();
        const st = ctx.state(), hostEl = st.mode !== 'dashboard' && typeof ctx.sheetHost === 'function' ? ctx.sheetHost() : null;
        if (hostEl) { if (!hostSheets.has(hostEl)) hostSheets.set(hostEl, E.sheet.create(hostEl)); return hostSheets.get(hostEl); }
        return ctx.sheet;
    }
    const closeAction = ctx => ({ id: 'close', label: ctx.t('homedash.dialog.close'), tone: 'primary' });
    const notice = (ctx, title, text) => sheetOf(ctx).open({ title, text: [text], actions: [Object.assign(closeAction(ctx), { autofocus: true })], kind: 'confirm' });
    function issuesSheet(ctx, issues, imported) {
        const t = ctx.t, list = el('ul', { class: 'hd-dlg-issues' }), shown = issues.slice(0, 8);
        for (const i of shown) list.appendChild(el('li', {}, [el('span', { text: E.save.errorText(t, { code: i && i.code, issues: [i] }) }), el('code', { text: Array.from(String((i && i.path) || '')).slice(0, 120).join('') })]));
        const parts = imported ? [list] : [el('p', { class: 'hd-sheet-text', text: t('homedash.dialog.import_issues_text', { count: issues.length }) }), list];
        if (issues.length > shown.length) parts.push(el('p', { class: 'hd-f-help', text: t('homedash.dialog.import_more', { count: issues.length - shown.length }) }));
        return F.formSheet(sheetOf(ctx), { title: imported ? t('homedash.dialog.import_unknown', { count: issues.length }) : t('homedash.dialog.import_issues_title'), content: el('div', { class: 'hd-dlg' }, parts), actions: [closeAction(ctx)] });
    }
    const tooLarge = ctx => notice(ctx, ctx.t('homedash.dialog.import_title'), ctx.t('homedash.dialog.import_too_large'));

    // ---- run ----
    async function run(ctx, action, arg) {
        if (!ACTIONS.includes(action)) return false; const st = ctx.state(), ro = !!st.desktopReadonly;
        if (action === 'new') return ro ? false : newSheet(ctx);
        if (action === 'import') return ro ? false : importFlow(ctx);          // Q12: with or without an open dashboard
        if (st.mode !== 'dashboard') return false;
        if (action === 'export') return exportFlow(ctx);
        if (ro) return false;
        if (!ctx.designer) { const ok = await ctx.mount.enterEdit(); return ok ? ctx.mount.run(action, arg) : false; }   // reading 8
        if (action === 'rename') return renameSheet(ctx);
        if (action === 'look') return E.look.open(ctx);
        return addPage(ctx.designer);
    }
    async function newSheet(ctx) {
        const t = ctx.t, st = ctx.state(); if (st.mode !== 'dashboard') { ctx.home(); return true; }                 // reading 9: the home screen is the chooser
        const TEXT = { starter: () => [t('homedash.home.create_starter'), t('homedash.home.create_starter_hint')], empty: () => [t('homedash.home.create_empty'), t('homedash.home.create_empty_hint')], ha: () => [t('homedash.home.create_ha'), t('homedash.home.create_ha_hint')] };
        const content = el('div', { class: 'hd-dlg hd-newcards' });
        for (const c of newChoices({ readonly: st.desktopReadonly, templates: st.templates })) { const [title, hint] = TEXT[c.kind]();
            content.appendChild(el('button', { type: 'button', class: 'hd-newcard', 'data-kind': c.kind }, [el('span', { class: 'hd-newcard-icon', 'aria-hidden': 'true' }, E.icons.icon(c.icon)), el('span', {}, [el('strong', { text: title }), el('small', { text: hint })])])); }
        const imp = F.button({ label: t('homedash.dialog.new_import'), icon: 'upload', tone: 'quiet' }); setAttr(imp.el, 'data-kind', 'import'); content.appendChild(imp.el);
        const sheet = sheetOf(ctx), first = content.querySelector('.hd-newcard');
        content.addEventListener('click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('[data-kind]') : null; if (b) sheet.close(b.getAttribute('data-kind') === 'import' ? 'import' : 'new:' + b.getAttribute('data-kind')); });
        const picked = await F.formSheet(sheet, { title: t('homedash.dialog.new_title'), content, actions: [{ id: 'cancel', label: t('homedash.dialog.cancel') }], first });
        if (picked === 'import') return importFlow(ctx);
        if (typeof picked !== 'string' || !picked.startsWith('new:')) return false;
        const kind = picked.slice(4);
        if (ctx.designer) { const left = await ctx.mount.exitEdit(); if (left !== 'left') return false; }
        const ok = await ctx.mount.run('new_' + kind); if (ok && kind === 'empty') await ctx.mount.enterEdit(); return ok;
    }
    // ---- export ----
    async function exportFlow(ctx) {
        const st = ctx.state(), dash = st.dashboard; if (!dash) return false; const catalog = ctx.catalog, t = ctx.t;
        let raw = ctx.designer ? ctx.designer.doc() : null;
        if (!raw) { const cached = ctx.store.docs.get(dash.id); raw = cached ? cached.doc : null; if (!raw) { try { const res = await ctx.api.dashboard(dash.id); raw = res && res.dashboard; } catch (err) { return false; } } }
        const doc = E.doc.normalise(raw, { sizeOf: w => catalog.defaultSize(w), constraints: w => catalog.constraints(w) });
        const file = exportFile(doc, { catalog, snapshot: E.widgets.paletteSnapshot(ctx.root, catalog.paletteTokens()) });
        const win = ctx.root.ownerDocument.defaultView, blob = new win.Blob([file.text], { type: 'application/json' }), href = win.URL.createObjectURL(blob);
        const a = el('a', { href, download: file.filename, hidden: true }); ctx.root.appendChild(a); a.click(); a.remove(); win.setTimeout(() => win.URL.revokeObjectURL(href), 1000);
        ctx.toast(t('homedash.dialog.export_done', { file: file.filename }) + (file.drafts ? ' ' + t('homedash.dialog.export_drafts', { count: file.drafts }) : '')); return true;
    }
    // ---- import ----
    function pickFile(ctx) {
        const st = ctx.state(), hostEl = st.mode !== 'dashboard' && typeof ctx.sheetHost === 'function' ? ctx.sheetHost() : ctx.root, input = el('input', { type: 'file', accept: '.json,application/json', hidden: true, 'aria-hidden': 'true', tabindex: '-1', 'data-hd-import': 'true' });
        hostEl.appendChild(input);
        return new Promise(resolve => { input.addEventListener('change', () => { const f = input.files && input.files[0] ? input.files[0] : null; input.remove(); resolve(f); }, { once: true }); input.addEventListener('cancel', () => { input.remove(); resolve(null); }, { once: true }); input.click(); });
    }
    async function importFlow(ctx) {
        const t = ctx.t, file = await pickFile(ctx); if (!file) return false;
        if (file.size > IMPORT_MAX) { await tooLarge(ctx); return false; }
        let text; try { text = await file.text(); } catch (err) { await notice(ctx, t('homedash.dialog.import_title'), t('homedash.dialog.import_failed', { reason: E.save.errorText(t, { code: 'bad_json' }) })); return false; }
        const parsed = importDocument(text); if (!parsed.ok) { if (parsed.code === 'too_large') await tooLarge(ctx); else await issuesSheet(ctx, [{ code: parsed.code }]); return false; }
        const failed = err => notice(ctx, t('homedash.dialog.import_title'), t('homedash.dialog.import_failed', { reason: E.save.errorText(t, { code: err && err.code }) }));
        // A bare document may lack the kiosk block the validator requires: normalise (server defaults) before validate and create.
        const document = E.doc.normalise(parsed.document, { sizeOf: w => ctx.catalog.defaultSize(w), constraints: w => ctx.catalog.constraints(w) });
        let size = 0; try { size = E.save.utf8Bytes(JSON.stringify(E.doc.forSave(document, { catalog: ctx.catalog, snapshot: E.widgets.paletteSnapshot(ctx.root, ctx.catalog.paletteTokens()) }))); } catch (err) { size = 0; }   // the body the lane would send; the server judges anything unreadable
        if (size > E.save.DOC_MAX) { await tooLarge(ctx); return false; }
        let res; try { res = await ctx.api.validateDocument(document); }
        catch (err) { const issues = err && err.data && Array.isArray(err.data.issues) ? err.data.issues : null; if (issues && issues.length) await issuesSheet(ctx, issues); else await failed(err); return false; }
        if (!res || res.valid !== true) { await issuesSheet(ctx, res && Array.isArray(res.issues) && res.issues.length ? res.issues : [{ code: 'invalid_document' }]); return false; }
        let made; try { made = await ctx.api.createDocument(document); } catch (err) { await failed(err); return false; }
        const d = made && made.dashboard; if (!d) { await failed(null); return false; }
        const unknownList = (Array.isArray(made.warnings) ? made.warnings : Array.isArray(res.warnings) ? res.warnings : []).filter(w => w && w.code === 'entity_unknown'), unknown = unknownList.length;
        ctx.refresh(); if (ctx.designer) { const left = await ctx.mount.exitEdit(); if (left !== 'left') { ctx.toast(t('homedash.dialog.import_done', { name: d.name || '' })); return true; } }
        await ctx.open(d.id, null); ctx.toast(t('homedash.dialog.import_done', { name: d.name || '' }) + (unknown ? ' ' + t('homedash.dialog.import_unknown', { count: unknown }) : ''));
        if (unknown) await issuesSheet(Object.assign({}, ctx, { designer: null }), unknownList, true);   // the designer of this ctx left with exitEdit: its sheet is gone, the app's sheet carries the list    // D10: the unknown devices are listed after the import (at most 8 rows plus import_more)
        return true;
    }
    // ---- rename ----
    async function renameSheet(ctx) {
        const D = ctx.designer, t = ctx.t; let field = null;
        const ctl = F.text({ key: 'dashboard-name', max: NAME_MAX, label: t('homedash.dialog.name'), value: D.doc().name || '', select: true, onEnter: () => submit() });
        field = F.field({ label: t('homedash.dialog.name'), control: ctl });
        const sheet = D.sheet();
        function check() { const v = ctl.get(), bad = nameError(v, NAME_MAX) || F.textIssue(v);
            if (!bad) { field.error(''); return true; } field.error(bad === 'empty' ? t('homedash.dialog.name_empty') : bad === 'long' ? t('homedash.dialog.name_long', { max: NAME_MAX }) : t('homedash.error.' + bad)); return false; }
        function submit() { if (check()) sheet.close('save'); }
        const picked = await F.formSheet(sheet, { title: t('homedash.dialog.rename_title'), content: el('div', { class: 'hd-dlg' }, field.el), actions: [{ id: 'cancel', label: t('homedash.dialog.cancel') }, { id: 'save', label: t('homedash.dialog.save'), tone: 'primary' }], onAction: id => (id === 'save' ? check() : true), first: ctl });
        if (picked !== 'save') return false; D.exec(C.setDoc(D.doc(), { path: ['name'], value: ctl.get().trim(), label: 'homedash.cmd.rename' })); D.seal(); return true;
    }
    const tabsOf = new WeakMap(), focusCurrentTab = (D, id) => { const h = tabsOf.get(D); if (h) h.focusTab(id || D.state().pageId); }, lastOf = n => { const k = n.children; return k.length ? k[k.length - 1] : null; };
    function addPage(D) {
        const doc = D.doc(), cmd = C.addPage(doc, { name: D.t('homedash.designer.page_n', { n: doc.pages.length + 1 }), icon: 'dashboard' });
        const ok = D.exec(cmd), h = tabsOf.get(D); if (ok && h && cmd && cmd.page) h.rename(cmd.page); return !!ok;   // a refusal (12 pages) is announced by exec
    }
    // ---- page tab menu ----
    function iconSheet(D, pageId) {
        const t = D.t, page = D.doc().pages.find(p => p.id === pageId); if (!page) return Promise.resolve(false);
        const grid = F.iconGrid({ t, label: t('homedash.inspector.page_icon'), value: page.icon || 'dashboard', allowAuto: false, onPick: n => D.exec(C.setPageIcon(D.doc(), { pageId, icon: n || undefined })) });
        return F.formSheet(D.sheet(), { title: t('homedash.inspector.page_icon'), content: el('div', { class: 'hd-dlg' }, grid.el), actions: [{ id: 'done', label: t('homedash.designer.done'), tone: 'primary' }], first: grid }).then(() => { D.seal(); focusCurrentTab(D, pageId); return true; });
    }
    async function deletePage(D, pageId) {
        const t = D.t, page = D.doc().pages.find(p => p.id === pageId); if (!page) return false;
        if (page.widgets.length) { const yes = await D.sheet().confirm({ title: t('homedash.dialog.page_delete_title', { name: page.name }), text: [t('homedash.dialog.page_delete_text', { count: page.widgets.length })], confirmLabel: t('homedash.cmd.page_delete'), cancelLabel: t('homedash.dialog.cancel'), tone: 'danger' }); if (!yes) return false; }
        return D.exec(C.deletePage(D.doc(), { pageId }));
    }
    function movePageBy(D, pageId, to) { const ok = D.exec(C.movePage(D.doc(), { pageId, to })); const h = tabsOf.get(D); if (ok && h) h.focusTab(pageId); return ok; }
    function pageMenu(D, pageId) {
        const t = D.t, doc = D.doc(), i = doc.pages.findIndex(p => p.id === pageId), n = doc.pages.length, page = doc.pages[i]; if (!page) return [];
        const h = () => tabsOf.get(D);
        return [{ id: 'page_rename', label: t('homedash.menu.page_rename'), icon: 'rename', shortcut: 'F2', run: () => { if (h()) h().rename(pageId); } },
            { id: 'page_icon', label: t('homedash.menu.page_icon'), icon: 'icon', run: () => iconSheet(D, pageId) },
            { id: 'page_duplicate', label: t('homedash.ctx.duplicate'), icon: 'duplicate', run: () => { const ok = D.exec(C.duplicatePage(D.doc(), { pageId, name: copyName(t, page.name) })); focusCurrentTab(D); return ok; } },
            { separator: true },
            { id: 'page_left', label: t('homedash.menu.page_left'), icon: 'left', shortcut: 'Alt+Shift+Left', disabled: i === 0, run: () => movePageBy(D, pageId, i - 1) },
            { id: 'page_right', label: t('homedash.menu.page_right'), icon: 'right', shortcut: 'Alt+Shift+Right', disabled: i === n - 1, run: () => movePageBy(D, pageId, i + 1) },
            { separator: true },
            { id: 'page_delete', label: t('homedash.menu.page_delete'), icon: 'delete', danger: true, disabled: n <= 1, run: () => deletePage(D, pageId).then(ok => { focusCurrentTab(D); return ok; }) }];
    }
    // ---- tab strip upgrade (part 1 builds the select-only tabs; this decorates the same nodes) ----
    function enhanceTabs(D, slot) {
        if (tabsOf.has(D)) return tabsOf.get(D);
        const t = D.t, life = E.core.bag(); let add = null, input = null, caret = null, drag = null;
        const pageOf = tab => (tab ? tab.getAttribute('data-page') : null), tabsEl = () => Array.from(slot.querySelectorAll('.hd-ptab')), tabFor = id => tabsEl().find(x => pageOf(x) === id) || null;
        function decorate() {
            const pages = D.doc().pages;
            if (!add) { add = el('button', { type: 'button', class: 'hd-ptab-add', 'aria-label': t('homedash.cmd.page_add'), title: t('homedash.cmd.page_add') }, E.icons.icon('plus')); slot.appendChild(add); }
            else if (lastOf(slot) !== add && !input) slot.appendChild(add);
            setAttr(add, 'aria-disabled', pages.length >= C.LIMITS.pages ? 'true' : null);
            for (const tab of tabsEl()) { const page = pages.find(p => p.id === pageOf(tab)); if (!page) continue; const want = E.icons.has(page.icon) ? page.icon : 'dashboard', have = tab.querySelector('.hd-ptab-icon');
                if (have && have.getAttribute('data-hd-icon') === want) continue; const ico = E.icons.icon(want); setAttr(ico, 'class', 'hd-ico hd-ptab-icon'); if (have) have.remove(); tab.insertBefore(ico, tab.children[0] || null); }
        }
        const off = D.on('state', decorate); decorate();   // synchronous: the bar rebuilds the tab buttons inside the same emit (its listener is registered first), so no frame shows them without icons
        const focusTab = id => { const tab = tabFor(id); if (tab) tab.focus(); };
        function rename(pageId, retry) {
            const tab = tabFor(pageId), page = D.doc().pages.find(p => p.id === pageId);
            if (!tab || !page) { if (!retry && page) E.core.frame(() => rename(pageId, true))(); return false; } if (input) cancelRename(false);
            const sr = slot.getBoundingClientRect(), tr = tab.getBoundingClientRect(); let closed = false;
            input = el('input', { class: 'hd-ptab-input', type: 'text', spellcheck: 'false', autocomplete: 'off', 'aria-label': t('homedash.inspector.page_name') }); input.value = page.name;
            setVars(input, { '--hd-e-ix': (tr.left - sr.left) + 'px', '--hd-e-iw': Math.max(96, tr.width) + 'px' }); setAttr(tab, 'data-hd-renaming', 'true'); slot.appendChild(input); input.focus(); if (typeof input.select === 'function') input.select();
            const finish = refocus => { if (closed) return; closed = true; const node = input; input = null; node.remove(); const tb = tabFor(pageId); if (tb) tb.removeAttribute('data-hd-renaming'); if (refocus) { focusTab(pageId); E.core.frame(() => focusTab(pageId))(); } if (add && lastOf(slot) !== add) slot.appendChild(add); };   // focus returns to the tab only for Enter and Escape; a blur (a click elsewhere) keeps the new focus
            const commit = byEnter => { if (closed) return; const v = F.runes(input.value, PAGE_NAME_MAX), bad = F.textIssue(v);
                if (!v.trim()) { finish(byEnter); return; }
                if (bad) { if (byEnter) { setAttr(input, 'aria-invalid', 'true'); D.announce(t('homedash.error.' + bad)); return; } finish(false); return; }
                D.exec(C.renamePage(D.doc(), { pageId, name: v })); D.seal(); finish(byEnter); };
            input.addEventListener('keydown', ev => { if (ev.isComposing) return; if (ev.key === 'Enter') { ev.preventDefault(); ev.stopPropagation(); commit(true); } else if (ev.key === 'Escape') { ev.preventDefault(); ev.stopPropagation(); finish(true); } });
            input.addEventListener('input', () => { setAttr(input, 'aria-invalid', null); }); input.addEventListener('blur', () => commit(false)); return true;
        }
        function cancelRename() { if (!input) return; const node = input; input = null; node.remove(); }
        life.on(slot, 'keydown', ev => {
            if (ev.target === input) return; const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null, pageId = pageOf(tab) || D.state().pageId, pages = D.doc().pages, i = pages.findIndex(p => p.id === pageId);
            if (ev.altKey && ev.shiftKey && (ev.key === 'ArrowLeft' || ev.key === 'ArrowRight') && i >= 0) { ev.preventDefault(); ev.stopPropagation(); const to = i + (ev.key === 'ArrowLeft' ? -1 : 1); if (to >= 0 && to < pages.length) movePageBy(D, pageId, to); return; }
            if (tab && (ev.key === 'F2' || (ev.key === 'Enter' && tab.getAttribute('aria-selected') === 'true'))) { ev.preventDefault(); ev.stopPropagation(); rename(pageId); }
        }, { capture: true });
        life.on(slot, 'click', ev => { const b = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab-add') : null; if (b && b.getAttribute('aria-disabled') !== 'true') addPage(D); });
        life.on(slot, 'dblclick', ev => { const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null; if (tab) rename(pageOf(tab)); });
        life.on(slot, 'pointerdown', ev => {
            const tab = ev.target && ev.target.closest ? ev.target.closest('.hd-ptab') : null; if (!tab || input || (ev.button !== undefined && ev.button !== 0)) return;
            const from = tabsEl().indexOf(tab), pageId = pageOf(tab), rects = () => tabsEl().map(x => x.getBoundingClientRect());
            E.input.track(ev, { threshold: 5, axis: 'x', capture: tab,
                onStart: () => { drag = { rects: rects(), from }; setAttr(tab, 'data-hd-dragging', 'true'); caret = el('span', { class: 'hd-ptab-caret', 'aria-hidden': 'true' }); slot.appendChild(caret); },
                onMove: (p, start) => { if (!drag) return; setVars(tab, { '--hd-e-tx': (p.x - start.x) + 'px' }); const idx = targetIndex(drag.rects, p.x, drag.from), others = drag.rects.filter((r, k) => k !== drag.from), sr = slot.getBoundingClientRect();
                    const at = idx < others.length ? others[idx].left : (others.length ? others[others.length - 1].left + others[others.length - 1].width : 0); setVars(caret, { '--hd-e-cx': (at - sr.left) + 'px' }); },
                onEnd: (p, start, how) => { const d = drag; drag = null; tab.removeAttribute('data-hd-dragging'); setVars(tab, { '--hd-e-tx': null }); if (caret) { caret.remove(); caret = null; }
                    if (d && how === 'up') { const to = targetIndex(d.rects, p.x, d.from); if (to !== d.from) movePageBy(D, pageId, to); } } });
        });
        const api = Object.freeze({ rename, focusTab, decorate, destroy() { off(); life.dispose(); if (add) add.remove(); if (input) input.remove(); tabsOf.delete(D); } });
        tabsOf.set(D, api); return api;
    }
    E.ddialogs = Object.freeze({ ACTIONS, run, pageMenu, enhanceTabs, exportFile, importDocument, newChoices, nameError, targetIndex, slugify, IMPORT_MAX });
})();
