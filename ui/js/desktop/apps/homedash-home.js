/* HomeDash desktop home view (F2): the dashboard list and the first-run empty state, painted from the desktop theme (desktop-app-homedash.css).
   window.HomeDashHome.create({t, lang, onOpen(id), onStarter(), onEmpty(), onHa(), sourceName(src), onDelete(summary)}) returns {el, render(state), destroy()}.
   render(state) reads state.dashboards (null while loading), desktopReadonly, busy and notice (Task 12 state). Names and dates come from the
   server and reach the page only through textContent; the same list renders no new nodes and keeps the focus. */
(function () {
    'use strict';
    const DOT = ' ' + String.fromCodePoint(0xb7) + ' ';
    const NOTICE_TONES = { limit: 'warn', create_failed: 'warn', list_failed: 'warn', open_failed: 'warn', delete_failed: 'warn', delete_conflict: 'warn', template_empty: 'warn', source_unavailable: 'warn', readonly: 'info', dashboard_deleted: 'info', dashboard_missing: 'info' };
    function create(options) {
        const o = options || {}, E = window.HomeDashEngine, F = E.format, el = E.el, svg = E.svg, setAttr = E.core.setAttr, t = o.t, lang = o.lang || 'en', life = E.core.bag();
        let current = [], lastSig = '', pending = '';
        function noticeText(code, source) {
            switch (code) {
                case 'dashboard_deleted': return t('homedash.home.notice_deleted');
                case 'dashboard_missing': return t('homedash.home.notice_missing');
                case 'open_failed': return t('homedash.home.notice_open_failed');
                case 'list_failed': return t('homedash.home.notice_list_failed');
                case 'limit': return t('homedash.home.notice_limit');
                case 'create_failed': return t('homedash.home.notice_create_failed');
                case 'readonly': return t('homedash.home.notice_readonly');
                case 'delete_conflict': return t('homedash.home.notice_delete_conflict');
                case 'delete_failed': return t('homedash.home.notice_delete_failed');
                case 'template_empty': return t('homedash.home.notice_template_empty');
                case 'source_unavailable': return t('homedash.home.notice_source_unavailable', { source: typeof o.sourceName === 'function' ? o.sourceName(source || 'ha') : String(source || 'ha') });
                default: return '';
            }
        }
        function art() {
            const s = (tag, attrs) => svg(tag, attrs);
            return svg('svg', { class: 'hdx-art', viewBox: '0 0 160 112', 'aria-hidden': 'true', focusable: 'false' }, [
                s('rect', { class: 'hdx-art-card', x: 4, y: 4, width: 72, height: 60, rx: 10 }), s('rect', { class: 'hdx-art-card', x: 84, y: 4, width: 72, height: 28, rx: 10 }),
                s('rect', { class: 'hdx-art-card', x: 84, y: 36, width: 72, height: 28, rx: 10 }), s('rect', { class: 'hdx-art-card', x: 4, y: 72, width: 152, height: 36, rx: 10 }),
                s('path', { class: 'hdx-art-track', d: 'M22 50a18 18 0 1 1 36 0' }), s('path', { class: 'hdx-art-value', d: 'M22 50a18 18 0 0 1 30.7-12.7' }),
                s('rect', { class: 'hdx-art-pill', x: 96, y: 12, width: 26, height: 12, rx: 6 }), s('circle', { class: 'hdx-art-knob', cx: 116, cy: 18, r: 4 }),
                s('rect', { class: 'hdx-art-bar', x: 96, y: 46, width: 48, height: 6, rx: 3 }),
                s('polyline', { class: 'hdx-art-line', points: '16,98 40,88 62,94 86,80 108,86 132,78 146,82' }), s('circle', { class: 'hdx-art-dot', cx: 146, cy: 82, r: 3.5 })]);
        }
        const button = (action, icon, label, primary) => el('button', { type: 'button', class: 'hdx-btn' + (primary ? ' is-primary' : ''), 'data-action': action }, [E.icons.icon(icon), el('span', { text: label })]);
        const choice = (action, icon, label, hint, primary) => el('button', { type: 'button', class: 'hdx-choice' + (primary ? ' is-primary' : ''), 'data-action': action }, [
            el('span', { class: 'hdx-choice-icon', 'aria-hidden': 'true' }, E.icons.icon(icon)), el('span', { class: 'hdx-choice-text' }, [el('strong', { text: label }), el('small', { text: hint })])]);
        const head = el('div', { class: 'hdx-home-head', hidden: 'true' }, [el('h2', { class: 'hdx-home-title', text: t('homedash.home.title') }), el('div', { class: 'hdx-home-actions' },
            [button('create-starter', 'gauge', t('homedash.home.create_starter'), true), button('create-empty', 'plus', t('homedash.home.create_empty'), false),
                button('create-ha', 'home', t('homedash.home.create_ha'), false)])]);
        const notice = el('p', { class: 'hdx-note hdx-notice', role: 'status', 'data-tone': 'info' }); /* always rendered, empty when there is nothing to say: a live region must exist before its text does */
        const roNote = el('p', { class: 'hdx-note hdx-readonly-note', 'data-tone': 'info', hidden: 'true' }, [E.icons.icon('lock'), el('span', { text: t('homedash.home.readonly_note') })]);
        const skeleton = el('div', { class: 'hdx-skeleton', 'aria-hidden': 'true' }, [el('div', { class: 'hdx-skel-card' }), el('div', { class: 'hdx-skel-card' }), el('div', { class: 'hdx-skel-card' })]);
        const list = el('ul', { class: 'hdx-list', 'aria-label': t('homedash.home.title'), hidden: 'true' });
        const empty = el('div', { class: 'hdx-empty', hidden: 'true' }, [art(), el('h2', { class: 'hdx-empty-title', text: t('homedash.home.empty_title') }),
            el('p', { class: 'hdx-empty-text', text: t('homedash.home.empty_text') }), el('div', { class: 'hdx-choices' }, [
                choice('create-starter', 'gauge', t('homedash.home.create_starter'), t('homedash.home.create_starter_hint'), true),
                choice('create-empty', 'dashboard', t('homedash.home.create_empty'), t('homedash.home.create_empty_hint'), false),
                choice('create-ha', 'home', t('homedash.home.create_ha'), t('homedash.home.create_ha_hint'), false)])]);
        const root = el('section', { class: 'hdx-home', 'data-hdx-mode': 'loading', 'aria-label': t('homedash.home.title') }, el('div', { class: 'hdx-home-inner' }, [head, notice, roNote, skeleton, list, empty]));
        function item(d, ro) {
            const name = typeof d.name === 'string' && d.name ? d.name : d.id, when = Date.parse(d.updated_at);
            const meta = [Number.isFinite(when) ? t('homedash.home.updated', { when: F.relTime(when, Date.now(), lang) }) : '', d.saved_by === 'agent' ? t('homedash.home.by_agent') : ''].filter(Boolean).join(DOT);
            return el('li', { class: 'hdx-item' }, el('div', { class: 'hdx-card' }, [
                el('button', { type: 'button', class: 'hdx-card-open', 'data-action': 'open', 'data-id': d.id, 'aria-label': t('homedash.home.open', { name }) }, [
                    el('span', { class: 'hdx-card-icon', 'aria-hidden': 'true' }, E.icons.icon('dashboard')),
                    el('span', { class: 'hdx-card-text' }, [el('span', { class: 'hdx-card-name', text: name }), el('span', { class: 'hdx-card-meta', text: meta })])]),
                ro ? null : el('button', { type: 'button', class: 'hdx-card-delete', 'data-action': 'delete', 'data-id': d.id, 'aria-label': t('homedash.home.delete', { name }), title: t('homedash.home.delete', { name }) }, E.icons.icon('trash'))]));
        }
        function paintList(dashboards, ro) {
            const active = root.ownerDocument && root.ownerDocument.activeElement;
            const keep = active && active.closest && active.closest('.hdx-list') === list ? [active.getAttribute('data-action'), active.getAttribute('data-id')] : null;
            list.replaceChildren(...dashboards.map(d => item(d, ro)));
            if (keep) { const again = Array.from(list.querySelectorAll('button')).find(b => b.getAttribute('data-action') === keep[0] && b.getAttribute('data-id') === keep[1]); if (again) again.focus(); }
        }
        function render(s) {
            const st = s || {}, dashboards = Array.isArray(st.dashboards) ? st.dashboards : null, ro = !!st.desktopReadonly, busy = !!st.busy;
            const mode = !dashboards ? 'loading' : dashboards.length ? 'list' : 'empty';
            if (!busy) pending = '';
            setAttr(root, 'data-hdx-mode', mode);
            setAttr(head, 'hidden', mode !== 'list'); setAttr(list, 'hidden', mode !== 'list'); setAttr(empty, 'hidden', mode !== 'empty'); setAttr(skeleton, 'hidden', mode !== 'loading');
            const ha = Array.isArray(st.templates) && st.templates.includes('home_assistant');
            for (const b of root.querySelectorAll('[data-action="create-starter"], [data-action="create-empty"], [data-action="create-ha"]')) {
                setAttr(b, 'hidden', ro || (b.getAttribute('data-action') === 'create-ha' && !ha)); setAttr(b, 'aria-disabled', busy ? 'true' : null); setAttr(b, 'aria-busy', busy && b.getAttribute('data-action') === pending ? 'true' : null);
            }
            setAttr(roNote, 'hidden', !ro);
            const text = noticeText(st.notice, st.noticeSource); notice.textContent = text; setAttr(notice, 'data-tone', NOTICE_TONES[st.notice] || 'info');
            if (dashboards) {
                current = dashboards;
                const sig = JSON.stringify([ro, dashboards.map(d => [d.id, d.name, d.revision, d.updated_at, d.saved_by])]);
                if (sig !== lastSig) { lastSig = sig; paintList(dashboards, ro); }
            }
        }
        life.on(root, 'click', ev => {
            const b = ev.target && ev.target.closest ? ev.target.closest('button') : null;
            if (!b || b.hasAttribute('disabled') || b.getAttribute('aria-disabled') === 'true') return;
            const action = b.getAttribute('data-action'), id = b.getAttribute('data-id');
            if (action === 'create-starter') { pending = action; o.onStarter(); }
            else if (action === 'create-empty') { pending = action; o.onEmpty(); }
            else if (action === 'create-ha') { pending = action; o.onHa(); }
            else if (action === 'open' && id) o.onOpen(id);
            else if (action === 'delete' && id) { const s = current.find(d => d.id === id); if (s) o.onDelete(s); }
        });
        return Object.freeze({ el: root, render, destroy() { life.dispose(); root.remove(); } });
    }
    window.HomeDashHome = Object.freeze({ create });
})();
