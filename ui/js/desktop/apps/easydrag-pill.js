// EasyDrag header pill and primary button (W1c). One pill says where the flow stands: a status disc and one short text,
// picked in a fixed order (a run, a past run, a problem, a pause, then the version state). Its popover holds what left the
// header: the Active switch, the next run, Run now, Runs and the action that finishes a partial publish. The primary button
// names the next step: Test, or Publish with its change count, or nothing for a clean live flow. Every control goes through
// the action registry (easydrag-actions.js); the words come from the status map and the easydrag.ui.pill_* keys.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};
    const MISSION_MISSING = 'FLOW_MISSION_MISSING';
    // ENTRIES maps a popover entry to its registry action.
    const ENTRIES = { active: 'flow.active', 'run-now': 'flow.run_now', runs: 'view.runs', 'publish-finish': 'flow.publish', repair: 'flow.repair_mission' };

    const missionMissing = ed => ed.publishIncomplete === MISSION_MISSING || (ed.liveIssues || []).some(issue => issue.code === MISSION_MISSING);
    // outdated: the published version no longer passes the server's checks (and the issue is not the missing Mission Control entry).
    const outdated = ed => !ed.runView && !!ed.flow.live && (ed.liveIssues || []).some(issue => issue.code !== MISSION_MISSING) && Number(ed.flow.live_revision) === ed.liveIssuesRevision;
    // changed: a live flow whose draft differs from the live version, or has an edit the server has not seen yet.
    const changed = ed => !!ed.flow.live && (ed.flow.draft_revision !== ed.flow.published_draft_revision || ED.flowState.dirty(ed));

    // describe picks the one state of the pill. f: {running (a run of this flow is going, its own or the one viewed),
    // partialText(code) (the words of a partial publish)}. It returns {kind, status (an ED.status value, '' for no disc), key
    // (the text key; none for a running flow, whose word is the status word), numbered (the text names the live version, {n}), hint}.
    function describe(ed, f) {
        const block = !!ed.isBlock;
        if (f.running) return { kind: 'running', status: 'running', key: '', hint: '' };
        if (ed.runView) return { kind: 'past', status: '', key: 'easydrag.ui.state_run_view', hint: '' };
        if (ed.publishIncomplete && ed.publishIncomplete !== MISSION_MISSING) return { kind: 'partial', status: 'blocked', key: 'easydrag.ui.state_publish_incomplete', hint: f.partialText(ed.publishIncomplete) };
        if (missionMissing(ed)) return { kind: 'link', status: 'blocked', key: 'easydrag.ui.pill_link_missing', hint: f.partialText(MISSION_MISSING) };
        if (outdated(ed)) return { kind: 'outdated', status: 'blocked', key: 'easydrag.ui.state_live_outdated', hint: ed.t('easydrag.ui.live_outdated_hint') };
        if (!block && ed.timersRunning === false) return { kind: 'timers', status: 'uncertain', key: 'easydrag.ui.timers_stopped_title', hint: ed.t('easydrag.ui.timers_stopped_text') };
        if (!block && ed.flow.live && !ed.flowEnabled) return { kind: 'paused', status: 'paused', key: 'easydrag.ui.pill_paused', numbered: true, hint: '' };
        if (changed(ed)) return { kind: 'changed', status: 'ok', key: 'easydrag.ui.pill_live_changed', numbered: true, hint: '' };
        if (ed.flow.live) return { kind: 'live', status: 'ok', key: 'easydrag.ui.pill_live', numbered: true, hint: '' };
        return { kind: 'draft', status: 'not_run', key: 'easydrag.ui.pill_draft', hint: '' };
    }

    function wordOf(ed, d) {
        if (!d.key) return ED.status.word(ed.t, d.status);
        return d.numbered ? ed.t(d.key, { n: Number(ed.flow.live_revision) || 0 }) : ed.t(d.key);
    }

    // primaryOf names the next step. {kind: 'test'} (the draft is not tested yet), {kind: 'publish', count} (tested, with changes;
    // count is null when the server sent none or the flow was never published) or {kind: 'none'}. A partial publish offers Publish
    // again, as it always did; a read-only desktop and a past run offer nothing.
    function primaryOf(ed) {
        if (ed.readonly || ed.runView) return { kind: 'none', count: null };
        if (ed.publishIncomplete) return { kind: 'publish', count: null };
        if (ed.flow.live && !changed(ed)) return { kind: 'none', count: null };
        if (!ED.flowState.isTested(ed)) return { kind: 'test', count: null };
        const count = ed.flow.live ? ED.flowState.changeCount(ed) : null;
        return { kind: 'publish', count: count > 0 ? count : null };
    }

    function markup() {
        return '<button type="button" class="ed-head-pill" data-ed-pill data-ed-revisions aria-haspopup="dialog" aria-expanded="false">' +
            '<span class="ed-head-pill-disc" data-ed-pill-disc></span><span class="ed-head-pill-text" data-ed-pill-text></span></button>';
    }

    // create builds the pill renderer of one editor. host: {ed, el, runs, publish}.
    function create(host) {
        const { ed, el, runs, publish } = host;
        const { t, esc } = ed;
        const anchor = el.querySelector('[data-ed-pill]');
        const disc = anchor.querySelector('[data-ed-pill-disc]');
        const label = anchor.querySelector('[data-ed-pill-text]');
        const c = { host: ed };
        let pop = null;
        let shown = '';

        const facts = () => ({
            running: runs.isRunning() || !!(typeof runs.hasParked === 'function' && runs.hasParked()) || !!(ed.runView && ed.runView.run && ED.runs.isActive(ed.runView.run.status)),
            partialText: code => publish.partialText(code)
        });

        // The popover is built as {html, texts}: the markup holds the structure and the attributes, the words are set as text after
        // the nodes exist (no escaping to forget), and the pair is compared with the last one so an unchanged popover is left alone.
        function entry(parts, texts, id, name, glyph, text) {
            const attrs = ED.actions.button(id, c);
            if (!attrs) return;
            delete attrs['aria-label'];
            const reason = attrs['aria-disabled'] ? attrs['aria-description'] || '' : '';
            parts.push('<button type="button" class="ed-btn ed-btn--ghost ed-pop-item" data-ed-pop="' + name + '"' + ED.actions.attrString(attrs) + '>' + glyph +
                '<span data-ed-fill="' + name + '"></span>' + (reason ? '<small class="ed-pop-reason" data-ed-fill="' + name + '-reason"></small>' : '') + '</button>');
            texts[name] = text || ED.actions.label(id, c);
            if (reason) texts[name + '-reason'] = reason;
        }

        function switchEntry(parts, texts) {
            const attrs = ED.actions.button('flow.active', c);
            if (!attrs) return;
            delete attrs['aria-label'];
            const reason = attrs['aria-disabled'] ? attrs['aria-description'] || '' : '';
            parts.push('<button type="button" class="ed-switch-btn ed-pop-item" role="switch" aria-checked="' + (ed.flowEnabled ? 'true' : 'false') + '" data-ed-pop="active"' + ED.actions.attrString(attrs) +
                '><span class="ed-switch" aria-hidden="true"><span></span></span><span data-ed-fill="active"></span>' + (reason ? '<small class="ed-pop-reason" data-ed-fill="active-reason"></small>' : '') + '</button>');
            texts.active = ED.actions.label('flow.active', c);
            if (reason) texts['active-reason'] = reason;
        }

        function nextRunText() {
            if (!ed.flow.live) return t('easydrag.ui.pill_next_none');
            if (!ed.flowEnabled) return t('easydrag.ui.state_inactive');
            return ed.flow.next_run ? ED.format.nextRun(ed.flow.next_run.at) : t('easydrag.ui.pill_next_none');
        }

        // content is the popover's markup and words. Blocks have no Active switch, next run or Run now.
        function content() {
            const d = describe(ed, facts());
            const texts = { state: wordOf(ed, d) };
            const parts = ['<div class="ed-pop-body"><p class="ed-pop-state">' + (d.status ? ED.status.disc(d.status, 14, { t, hidden: true }) : '') + '<strong data-ed-fill="state"></strong></p>'];
            if (d.hint) { parts.push('<p class="ed-hint" data-ed-fill="hint"></p>'); texts.hint = d.hint; }
            if (!ed.isBlock) {
                switchEntry(parts, texts);
                const title = ed.flow.live && ed.flowEnabled && ed.flow.next_run ? ' title="' + esc(ED.format.date(ed.flow.next_run.at)) + '"' : '';
                parts.push('<p class="ed-pop-row"><span class="ed-pop-label" data-ed-fill="next-label"></span><span class="ed-pop-value" data-ed-next-run data-ed-fill="next"' + title + '></span></p>');
                texts['next-label'] = t('easydrag.ui.pill_next_run');
                texts.next = nextRunText();
                if (ed.flow.live) entry(parts, texts, 'flow.run_now', 'run-now', ED.icons.of('run'));
                entry(parts, texts, 'view.runs', 'runs', ED.icons.of('runs'));
            }
            if (!ed.runView) {
                if (missionMissing(ed)) entry(parts, texts, 'flow.repair_mission', 'repair', ED.core.icon('link'));
                else if (ed.publishIncomplete || outdated(ed)) entry(parts, texts, 'flow.publish', 'publish-finish', ED.icons.of('publish'), t('easydrag.ui.pill_finish_publish'));
            }
            parts.push('</div>');
            return { html: parts.join(''), texts };
        }

        function render(made) {
            const node = ED.core.el(made.html);
            Object.keys(made.texts).forEach(name => {
                const target = node.querySelector('[data-ed-fill="' + name + '"]');
                if (target) target.textContent = made.texts[name];
            });
            return node;
        }

        // paint draws the pill; an open popover follows when its content changed.
        function paint() {
            const d = describe(ed, facts());
            const word = wordOf(ed, d);
            if (label.textContent !== word) label.textContent = word;
            const mark = d.status ? ED.status.disc(d.status, 14, { t, hidden: true }) : '';
            if (disc.dataset.mark !== mark) { disc.dataset.mark = mark; disc.innerHTML = mark; }
            anchor.setAttribute('data-ed-pill-kind', d.kind);
            anchor.setAttribute('data-status', d.status || 'none');
            anchor.setAttribute('title', d.hint ? word + ' ' + String.fromCodePoint(0x2013) + ' ' + d.hint : word);
            // The revision facts the tests and the stylesheet read: draft (never published), newer (draft ahead of live) or live.
            anchor.dataset.kind = !ed.flow.live ? 'draft' : ed.flow.draft_revision !== ed.flow.published_draft_revision ? 'newer' : 'live';
            anchor.dataset.draft = String(Number(ed.flow.draft_revision) || 0);
            anchor.dataset.live = String(Number(ed.flow.live_revision) || 0);
            anchor.dataset.paused = !ed.isBlock && ed.flow.live && !ed.flowEnabled ? 'true' : 'false';
            if (pop) {
                const made = content();
                const key = JSON.stringify(made);
                if (key !== shown) { shown = key; pop.update(render(made)); }
            }
        }

        function close(restore) { if (pop) pop.close(restore); }

        // onClick: one handler for every entry. The Active switch stays open; any other entry closes the popover and gives the
        // focus back to the pill first, so a dialog it opens returns the focus there.
        function onClick(event) {
            const b = event.target.closest && event.target.closest('[data-ed-pop]');
            if (!b || b.getAttribute('aria-disabled') === 'true') return;
            const id = ENTRIES[b.getAttribute('data-ed-pop')];
            if (!id) return;
            if (id !== 'flow.active') close(true);
            ED.actions.run(id, c);
        }

        function open() {
            if (pop) { pop.close(true); return; }
            const made = content();
            shown = JSON.stringify(made);
            anchor.setAttribute('aria-expanded', 'true');
            pop = ED.kit.popover(anchor, {
                content: render(made), label: t('easydrag.ui.pill_label'),
                onClose: () => { pop = null; anchor.setAttribute('aria-expanded', 'false'); }
            });
            pop.el.setAttribute('data-ed-pill-popover', '');
            pop.el.addEventListener('click', onClick);
            // The next run is read when the popover opens: the schedule may have changed since the editor loaded.
            ED.flowState.refresh(ed);
        }

        anchor.addEventListener('click', open);

        return { paint, open, close, isOpen: () => !!pop, dispose() { close(false); } };
    }

    ED.pill = { markup, create, describe, primaryOf, missionMissing, outdated, changed };
})();
