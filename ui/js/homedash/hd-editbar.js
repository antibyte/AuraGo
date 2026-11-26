// HomeDash edit bar (M4 contract C15; frontend 5.1). create({t, onCommand(name, arg, ev), now}) builds the 48 px strip of design mode: Done, Undo and Redo,
// the page tabs (a roving tablist), the breakpoint switch, the save state, Look, More and the two drawer toggles. update(s) only writes what changed
// (the tab buttons are rebuilt when the [id, name] list changes); the bar never decides anything: every click and key becomes onCommand(name, arg, ev).
// Disabled buttons use aria-disabled so that focus stays where it is. Texts come through t(); the node classes and data attributes are the C15 contract.
(function () {
    'use strict';
    const E = window.HomeDashEngine;
    if (!E || !E.core || !E.icons) throw new Error('hd-editbar.js: load hd-core.js and hd-icons.js first');
    const { el, setAttr } = E.core, BPS = Object.freeze(['lg', 'md', 'sm']), BP_ICON = Object.freeze({ lg: 'device-desktop', md: 'device-tablet', sm: 'device-phone' });
    const PHASES = Object.freeze(['saved', 'saving', 'retrying', 'invalid', 'conflict', 'readonly', 'gone']), PHASE_ICON = Object.freeze({ saved: 'check', saving: 'sync', retrying: 'cloud-off', invalid: 'alert', conflict: 'alert', readonly: 'lock', gone: 'cloud-off' });
    const isTab = n => n.classList.contains('hd-ptab');
    function roving(key, i, n) { return key === 'ArrowRight' ? (i + 1) % n : key === 'ArrowLeft' ? (i - 1 + n) % n : key === 'Home' ? 0 : key === 'End' ? n - 1 : null; }
    function create(o) {
        const t = o.t, onCommand = typeof o.onCommand === 'function' ? o.onCommand : () => {}, now = typeof o.now === 'function' ? o.now : () => Date.now(), life = E.core.bag();
        const btn = (cmd, icon, label, cls) => el('button', { type: 'button', class: 'hd-eb-btn' + (cls ? ' ' + cls : ''), 'data-hd-cmd': cmd, 'aria-label': label, title: label }, [E.icons.icon(icon)]);
        const done = el('button', { type: 'button', class: 'hd-eb-btn hd-eb-done', 'data-hd-cmd': 'exit_edit' }, [E.icons.icon('check'), el('span', { class: 'hd-eb-label', text: t('homedash.designer.done') })]);
        const undo = btn('undo', 'undo', t('homedash.designer.undo')), redo = btn('redo', 'redo', t('homedash.designer.redo'));
        const tablist = el('div', { class: 'hd-ptabs', role: 'tablist', 'aria-label': t('homedash.designer.pages') });
        const opts = BPS.map(b => el('button', { type: 'button', class: 'hd-bp-opt', role: 'radio', 'aria-checked': 'false', tabindex: '-1', 'data-bp': b, 'aria-label': t('homedash.bp.' + b), title: t('homedash.bp.' + b) }, [E.icons.icon(BP_ICON[b])]));
        const chip = el('span', { class: 'hd-bp-chip', 'data-state': 'auto', hidden: 'true' }), reset = el('button', { type: 'button', class: 'hd-bp-reset', 'data-hd-cmd': 'reset_layout', hidden: 'true', text: t('homedash.bp.reset') });
        const bp = el('div', { class: 'hd-bp', role: 'radiogroup', 'aria-label': t('homedash.bp.label') }, opts.concat([chip, reset]));
        const saveIcon = el('span', { class: 'hd-save-icon' }), saveText = el('span', { class: 'hd-save-text' }), saveAct = el('button', { type: 'button', class: 'hd-save-action', hidden: 'true' });
        const save = el('div', { class: 'hd-save', 'data-phase': 'saved' }, [saveIcon, saveText, saveAct]);
        const look = btn('look', 'look', t('homedash.designer.look'), 'hd-eb-look'), more = btn('more', 'more', t('homedash.designer.more'), 'hd-eb-more'); setAttr(more, 'aria-haspopup', 'menu'); setAttr(look, 'hidden', true);
        const tp = btn('toggle_palette', 'panel-left', t('homedash.designer.toggle_palette'), 'hd-eb-drawer'), ti = btn('toggle_inspector', 'panel-right', t('homedash.designer.toggle_inspector'), 'hd-eb-drawer');
        for (const b of [tp, ti]) { setAttr(b, 'aria-expanded', 'false'); setAttr(b, 'hidden', true); }
        const slots = { pages: el('div', { class: 'hd-eb-group', 'data-hd-group': 'pages', 'data-hd-slot': 'pages' }, [tablist]), bp: el('div', { class: 'hd-eb-slot', 'data-hd-slot': 'bp' }, [bp]) };
        const bar = el('div', { class: 'hd-editbar', role: 'group', 'aria-label': t('homedash.designer.editbar') }, [
            el('div', { class: 'hd-eb-group', 'data-hd-group': 'start' }, [done, el('span', { class: 'hd-eb-sep', 'aria-hidden': 'true' }), undo, redo]), slots.pages,
            el('div', { class: 'hd-eb-group', 'data-hd-group': 'end' }, [slots.bp, save, look, more, tp, ti])]);
        const focusable = { done, undo, redo, look, more, toggle_palette: tp, toggle_inspector: ti, save: saveAct };
        let sig = '';
        const tabEls = () => Array.from(tablist.children).filter(isTab);
        function rebuildTabs(pages) {
            const doc = bar.ownerDocument, act = doc ? doc.activeElement : null, old = tabEls(), keep = act && old.includes(act) ? act.getAttribute('data-page') : null;
            for (const n of old) n.remove();
            const ref = tablist.children[0] || null;
            for (const p of pages) tablist.insertBefore(el('button', { type: 'button', class: 'hd-ptab', role: 'tab', 'aria-selected': 'false', tabindex: '-1', 'data-page': p.id }, [el('span', { class: 'hd-ptab-label', text: p.name })]), ref);
            if (keep) { const n = tabEls().find(x => x.getAttribute('data-page') === keep); if (n) n.focus(); }
        }
        function setText(node, text) { if (node.textContent !== text) node.textContent = text; }
        function updateSave(s) {
            const sv = s.save || {}, phase = sv.phase === 'pending' ? 'saving' : PHASES.includes(sv.phase) ? sv.phase : 'saved', n = Number(s.drafts) || 0; let text = '', act = '', label = '';
            if (phase === 'saved') { text = n === 0 ? t('homedash.save.saved') : n === 1 ? t('homedash.save.drafts_one') : t('homedash.save.drafts_many', { count: n }); if (n) { act = 'show_draft'; label = t('homedash.save.show_draft'); } }
            else if (phase === 'saving') text = t('homedash.save.saving');
            else if (phase === 'retrying') { text = t('homedash.save.retrying', { seconds: Math.max(1, Math.ceil(((Number(sv.nextRetryAt) || 0) - now()) / 1000)) }); act = 'retry_save'; label = t('homedash.save.retry_now'); }
            else if (phase === 'invalid') { text = t('homedash.save.invalid', { reason: s.saveReason || '' }); if (s.canUndo) { act = 'undo'; label = t('homedash.save.undo_last'); } }
            else if (phase === 'conflict') { text = t('homedash.save.conflict'); act = 'resolve_conflict'; label = t('homedash.conflict.resolve'); }
            else if (phase === 'readonly') text = t('homedash.save.readonly');
            else { text = t('homedash.save.gone'); act = 'resolve_conflict'; label = t('homedash.save.copy'); }
            setAttr(save, 'data-phase', phase); setText(saveText, text);
            if (saveIcon.getAttribute('data-icon') !== PHASE_ICON[phase]) { setAttr(saveIcon, 'data-icon', PHASE_ICON[phase]); saveIcon.replaceChildren(E.icons.icon(PHASE_ICON[phase])); }
            setAttr(saveAct, 'hidden', !act); setAttr(saveAct, 'data-hd-cmd', act || null); setText(saveAct, label);
        }
        function update(s) {
            const st = s || {}, pages = Array.isArray(st.pages) ? st.pages : [];
            const ul = st.undoLabel ? t('homedash.designer.undo_label', { label: st.undoLabel }) : t('homedash.designer.undo'), rl = st.redoLabel ? t('homedash.designer.redo_label', { label: st.redoLabel }) : t('homedash.designer.redo');
            for (const [b, label, can] of [[undo, ul, st.canUndo], [redo, rl, st.canRedo]]) { setAttr(b, 'aria-label', label); setAttr(b, 'title', label); setAttr(b, 'aria-disabled', !can); }
            const next = JSON.stringify(pages.map(p => [p.id, p.name]));
            if (next !== sig) { sig = next; rebuildTabs(pages); }
            for (const n of tabEls()) { const on = n.getAttribute('data-page') === st.pageId; setAttr(n, 'aria-selected', on ? 'true' : 'false'); setAttr(n, 'tabindex', on ? '0' : '-1'); setAttr(n, 'aria-controls', st.panelId || null); }
            const cur = BPS.includes(st.bp) ? st.bp : 'lg', ov = st.override || {};
            for (const n of opts) { const on = n.getAttribute('data-bp') === cur; setAttr(n, 'aria-checked', on ? 'true' : 'false'); setAttr(n, 'tabindex', on ? '0' : '-1'); }
            const custom = cur !== 'lg' && !!ov[cur]; setAttr(chip, 'hidden', cur === 'lg'); setAttr(chip, 'data-state', custom ? 'custom' : 'auto');
            setText(chip, cur === 'lg' ? '' : custom ? t('homedash.bp.custom') : t('homedash.bp.auto')); setAttr(reset, 'hidden', !custom);
            updateSave(st);
            setAttr(look, 'hidden', !st.hasLook);
            for (const [b, name, has] of [[tp, 'palette', st.hasPalette], [ti, 'inspector', st.hasInspector]]) { setAttr(b, 'hidden', !st.drawers || has === false); setAttr(b, 'aria-expanded', st.panel === name ? 'true' : 'false'); setAttr(b, 'aria-controls', (st.ids && st.ids[name]) || null); }
        }
        life.on(bar, 'click', ev => {
            const n = ev.target && typeof ev.target.closest === 'function' ? ev.target : null; if (!n) return;
            const tab = n.closest('.hd-ptab'); if (tab) { onCommand('page', tab.getAttribute('data-page'), ev); return; }
            const opt = n.closest('.hd-bp-opt'); if (opt) { onCommand('layout_' + opt.getAttribute('data-bp'), null, ev); return; }
            const c = n.closest('[data-hd-cmd]'); if (!c || c.getAttribute('aria-disabled') === 'true' || c.hasAttribute('hidden')) return;
            const name = c.getAttribute('data-hd-cmd'); if (name === 'more' && typeof ev.stopPropagation === 'function') ev.stopPropagation();
            onCommand(name, null, ev);
        });
        function rove(ev, list, pick) {
            if (ev.altKey || ev.ctrlKey || ev.metaKey || !ev.target || typeof ev.target.closest !== 'function') return;
            const i = list().indexOf(pick(ev.target)); if (i < 0) return; const j = roving(ev.key, i, list().length); if (j === null) return;
            ev.preventDefault(); const n = list()[j]; n.focus(); onCommand(n.getAttribute('data-page') ? 'page' : 'layout_' + n.getAttribute('data-bp'), n.getAttribute('data-page') || null, ev);
        }
        life.on(tablist, 'keydown', ev => rove(ev, tabEls, n => n.closest('.hd-ptab')));
        life.on(bp, 'keydown', ev => rove(ev, () => opts, n => n.closest('.hd-bp-opt')));
        return Object.freeze({
            el: bar, slot: name => slots[name] || null,
            focus(name) { const n = name === 'tabs' ? (tabEls().find(x => x.getAttribute('tabindex') === '0') || tabEls()[0]) : name === 'bp' ? (opts.find(x => x.getAttribute('aria-checked') === 'true') || opts[0]) : focusable[name]; if (n && typeof n.focus === 'function') n.focus({ preventScroll: true }); return !!n; },
            update, destroy() { life.dispose(); bar.remove(); }
        });
    }
    E.editbar = Object.freeze({ create });
})();
