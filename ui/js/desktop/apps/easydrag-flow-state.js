// EasyDrag flow facts from the server (W1c): when the flow runs next (ed.flow.next_run), whether its
// exact draft revision passed a test or a simulation (ed.flow.tested) and how much the draft differs
// from the live version (ed.flow.changes). The editor takes them from GET /flows/{id} and from every
// save answer; the header's pill and primary button read them through the helpers below.
(function () {
    'use strict';

    const ED = window.EasyDrag = window.EasyDrag || {};

    const count = value => (Number.isFinite(Number(value)) && Number(value) > 0 ? Math.floor(Number(value)) : 0);

    // take copies the facts of a server answer onto ed.flow. An answer is GET /flows/{id} ({flow, enabled, next_run?, tested?,
    // changes?}) or a save answer ({draft_revision, tested?, changes?}). opts.revision is the draft revision the answer describes
    // (the flag "tested" is true for that revision only); opts.get says that the answer is a complete read, so a missing next_run
    // means "none". A save answer says nothing about the schedule: next_run stays as it was.
    function take(ed, answer, opts) {
        const o = opts || {};
        const flow = ed.flow;
        if (!flow || !answer || typeof answer !== 'object') return;
        // An answer for another revision than the one on screen says nothing about this draft: only the schedule is taken.
        const other = Number.isInteger(o.revision) && o.revision !== currentRevision(ed);
        if (!other) {
            flow.tested = answer.tested === true;
            const changes = answer.changes;
            if (changes && typeof changes === 'object') flow.changes = { steps: count(changes.steps), connections: count(changes.connections) };
            else delete flow.changes;
        }
        if (o.get || Object.prototype.hasOwnProperty.call(answer, 'next_run')) {
            const next = answer.next_run;
            if (next && typeof next.at === 'string' && Number.isFinite(Date.parse(next.at))) flow.next_run = { at: next.at, node: typeof next.node === 'string' ? next.node : '' };
            else delete flow.next_run;
        }
        if (!other) ed.flowFactsRevision = Number.isInteger(o.revision) ? o.revision : count(flow.draft_revision);
    }

    // currentRevision is the draft revision the editor shows: the saver's, else the record's.
    function currentRevision(ed) {
        return ed.saver && Number.isInteger(ed.saver.revision) ? ed.saver.revision : count(ed.flow && ed.flow.draft_revision);
    }

    // dirty: the draft has an edit the server has not seen, so nothing it says about "tested" applies.
    function dirty(ed) { return !!(ed.saver && typeof ed.saver.isDirty === 'function' && ed.saver.isDirty()); }

    // isTested: the exact draft revision on screen passed a test or a simulation. The server's flag counts for the revision it
    // was read for; the editor's own evidence (ed.testEvidence, set when a test or simulation of this session ends) counts at
    // once, before the next save or read brings the flag. A failed run is no evidence.
    function isTested(ed) {
        if (!ed.flow || dirty(ed)) return false;
        const revision = currentRevision(ed);
        if (ed.flow.tested === true && ed.flowFactsRevision === revision) return true;
        const evidence = ed.testEvidence;
        return !!evidence && Number(evidence.revision) === revision && ED.status.of(evidence.status, 'run') === 'ok';
    }

    // changeCount is the number of changes since the live version (steps and connections together), or null when the server
    // sent none (a flow that was never published, an older server) or counted it for another revision than the one on screen.
    function changeCount(ed) {
        if (!ed.flow || ed.flowFactsRevision !== currentRevision(ed)) return null;
        const changes = ed.flow && ed.flow.changes;
        return changes ? count(changes.steps) + count(changes.connections) : null;
    }

    // refresh reads the flow again and takes its facts: the popover asks for the next run when it opens. A read that answers for
    // another revision than the one on screen, or after the editor closed, is dropped. It resolves true when the facts changed hands.
    async function refresh(ed) {
        if (!ed.flow || !ed.api || typeof ed.api.get !== 'function') return false;
        let res;
        try { res = await ed.api.get(ed.flow.id); } catch (err) { return false; }
        if (!res || !res.flow || ed.root.isConnected === false || ed.runView) return false;
        const revision = count(res.flow.draft_revision);
        if (revision !== currentRevision(ed)) return false;
        take(ed, res, { get: true, revision });
        ed.bus.emit('flow-facts', ed.flow);
        return true;
    }

    // bind keeps the facts honest between answers: a restored version replaces the draft, so the old flag and count no longer apply.
    // loaded is the first GET answer.
    function bind(ed, bag, loaded) {
        take(ed, loaded, { get: true, revision: count(loaded && loaded.flow && loaded.flow.draft_revision) });
        bag.add(ed.bus.on('restored', res => {
            if (!res || !res.flow) return;
            ed.flow.tested = false;
            delete ed.flow.changes;
            ed.flowFactsRevision = count(res.flow.draft_revision);
        }));
    }

    ED.flowState = { take, bind, refresh, isTested, changeCount, dirty };
})();
