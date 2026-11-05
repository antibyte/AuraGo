// Visual groups share the real graph's commands and preserve omitted legacy metadata.
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const A = 'n_aaaaaaaa', B = 'n_bbbbbbbb', T = 'n_tttttttt';
    function fixture() { const h = sandbox(() => undefined), make = h.ED.canvas.create; let canvas; h.ED.canvas.create = ed => (canvas = make(ed)); const editor = openEditor(h); return { h, editor, model: editor.ed.model, canvas }; }
    await guardAsync('groups legacy documents stay group-free until an explicit command', async () => {
        const { editor, model } = fixture(); await settle(); const before = model.toJSON();
        eq('groups omitted metadata is not normalized into an empty array', Object.hasOwn(model.toJSON(), 'groups'), false);
        const id = model.createGroup([A, B], 'Research', 'blue', 'Two related steps');
        eq('groups creation uses a real stable ID and node membership', [/^g_[a-z2-7]{8}$/.test(id), model.group(id).members], [true, [A, B]]);
        model.undo(); eq('groups intentional removal by undo sends an explicit clear', model.toJSON(), Object.assign({}, before, { groups: [] })); model.redo();
        model.ungroup(id); eq('groups ungroup explicitly clears metadata without deleting nodes', [model.doc.groups, model.doc.nodes.length], [[], 3]); editor.dispose();
    });
    await guardAsync('groups acknowledged create undo clear save and reload cannot resurrect omitted metadata', async () => {
        let stored, revision = 3;
        const h = sandbox(req => {
            if (req.method === 'PUT' && req.url === '/api/desktop/flows/f1') {
                const next = JSON.parse(JSON.stringify(req.body.doc));
                if (next.groups === undefined || next.groups === null) { if (stored && Object.hasOwn(stored, 'groups')) next.groups = stored.groups; }
                stored = next; return { draft_revision: ++revision, issues: [] };
            }
        });
        const editor = openEditor(h), model = editor.ed.model; const id = model.createGroup([A, B], 'Persisted'); await editor.ed.saver.flush();
        eq('groups server acknowledges an actual grouped draft', stored.groups[0].id, id);
        model.undo(); await editor.ed.saver.flush();
        const payload = h.requests.filter(req => req.method === 'PUT' && req.url === '/api/desktop/flows/f1').at(-1).body.doc;
        eq('groups undo serializes [] across the omission-preserving saver boundary', [payload.groups, stored.groups, h.ED.model.create(stored, h.catalog).doc.groups], [[], [], []]); editor.dispose();
    });
    await guardAsync('groups membership moves and deletion cleanup share one undo', async () => {
        const { editor, model } = fixture(); const first = model.createGroup([A], 'First'), second = model.createGroup([B], 'Second');
        const before = model.toJSON(); model.setGroup(second, { members: [A, B] });
        eq('groups moving a member keeps membership unique and removes the empty old group', [model.group(first), model.group(second).members], [null, [A, B]]);
        model.undo(); eq('groups membership transfer is one undo', model.toJSON(), before);
        model.removeNodes([A]); eq('groups deleting the last member removes only its empty group', [model.group(first), model.group(second).members], [null, [B]]);
        model.undo(); eq('groups deleting a member and its metadata is one undo', model.toJSON(), before);
        model.setGroup(first, { members: [] }); eq('groups removing the last member explicitly ungroups it', [model.group(first), !!model.node(A)], [null, true]); editor.dispose();
    });
    await guardAsync('groups mixed movement and clipboard remapping never duplicate member work', async () => {
        const { editor, model } = fixture(); const id = model.createGroup([A, B], 'Source', 'green', 'Keep note'); const start = model.node(A).position.x;
        model.moveNodes([A, B, A], 16, 0); eq('groups a member included twice moves exactly once', model.node(A).position.x, start + 16);
        const partial = model.fragment([A]); eq('groups partial copies carry ordinary nodes only', Object.hasOwn(partial, 'groups'), false);
        const fragment = model.fragment([A, B]); const pasted = model.paste(fragment, { x: 1000, y: 400 }); const copy = model.doc.groups.find(group => group.id !== id);
        eq('groups complete copies retain text and remap both group and member identities', [copy.title, copy.note, copy.id !== id, copy.members], ['Source', 'Keep note', true, pasted]);
        model.undo(); eq('groups paste including metadata is one undo', model.doc.groups.length, 1); editor.dispose();
    });
    await guardAsync('groups invalid data and excessive membership are refused without normalization', async () => {
        const { h, editor, model } = fixture(); const valid = { id: 'g_abcdefgh', title: 'Valid', color: 'neutral', members: [A] }, ids = new Set([A, B, T]);
        const cases = [null, 3, Object.assign({}, valid, { id: { toString: 'not callable' } }), Object.assign({}, valid, { title: '' }), Object.assign({}, valid, { title: '😀'.repeat(81) }), Object.assign({}, valid, { color: 'url(javascript:x)' }), Object.assign({}, valid, { members: [A, A] }), Object.assign({}, valid, { members: ['missing'] })];
        eq('groups malformed imported entries fail closed without coercion crashes', cases.map(group => h.ED.model.validGroups([group], ids)), cases.map(() => false));
        const before = model.toJSON(); eq('groups invalid commands never mutate the document', [model.createGroup(['missing'], 'Invalid'), model.setGroups([valid, Object.assign({}, valid)])], [null, false]); eq('groups refused commands preserve exact data', model.toJSON(), before); editor.dispose();
    });
    await guardAsync('groups regrouping at the final cap and invalid legacy values remain honest', async () => {
        const { h, editor, model } = fixture(), node = model.toJSON().nodes[0];
        const suffix = i => 'c' + i.toString(2).padStart(7, '0').replaceAll('0', 'a').replaceAll('1', 'b');
        const nodes = Array.from({ length: 100 }, (_, i) => Object.assign({}, node, { id: 'n_' + suffix(i), key: 'step_' + i }));
        const groups = nodes.map((n, i) => ({ id: 'g_' + suffix(i), title: 'Group ' + i, color: 'neutral', members: [n.id] }));
        model.replaceDoc(Object.assign({}, model.toJSON(), { nodes, edges: [], groups }));
        const id = model.createGroup([nodes[0].id], 'Regroup');
        eq('groups replacing one complete group at the cap validates the final list', [!!id, model.doc.groups.length, model.doc.groups[0].id !== groups[0].id], [true, 100, true]);
        for (const raw of [0, false, '', { invalid: true }, [null]]) {
            model.replaceDoc(Object.assign({}, model.toJSON(), { groups: raw })); const before = model.toJSON();
            eq('groups invalid raw data stays unchanged when creation is refused ' + JSON.stringify(raw), [model.createGroup([nodes[0].id], 'Do not coerce'), model.toJSON()], [null, before]);
            editor.ed.groups.openList(); const dialog = h.dialogs.at(-1);
            eq('groups invalid display offers explicit repair without changing source ' + JSON.stringify(raw), [!!dialog.body.querySelector('[data-ed-group-clear]'), model.toJSON()], [true, before]); dialog.close(null);
        }
        editor.dispose();
    });
    await guardAsync('groups direct creation inspector validation IME and one-command movement', async () => {
        const { h, editor, model } = fixture(), ed = editor.ed; ed.selection.add(A); ed.selection.add(B);
        ed.groups.createFromSelection(); const id = model.doc.groups[0].id, title = editor.el.querySelector('[data-ed-group-title]');
        eq('groups creation opens the real inspector and focuses the default title without a prompt', [ed.detail.groupId(), h.dom.document.activeElement === title, h.dialogs.length], [id, true, 0]);
        title.value = '😀'.repeat(81); title.fire('change');
        eq('groups invalid title remains visible and blocks switching without losing text', [ed.flushPendingEdits(), title.value, model.group(id).title], [false, '😀'.repeat(81), 'group_default']);
        title.value = 'Composing'; title.fire('compositionstart'); title.fire('change');
        eq('groups pending composition cannot commit or leave', [ed.flushPendingEdits(), model.group(id).title], [false, 'group_default']);
        title.fire('compositionend'); title.fire('change');
        eq('groups valid rename is a model command', model.group(id).title, 'Composing');
        model.undo(); eq('groups rename undo keeps the inspector on the restored value', title.value, 'group_default');
        const start = model.node(A).position.x;
        editor.el.querySelector('[data-ed-group-dx]').value = '32'; editor.el.querySelector('[data-ed-group-move]').fire('click');
        eq('groups single-pointer move applies once to every member', [model.node(A).position.x, model.node(B).position.x], [start + 32, 632]);
        model.undo(); eq('groups move is one independent undo command', [model.node(A).position.x, !!model.group(id)], [start, true]);
        ed.groups.toggle(id, 'lock');
        eq('groups locked layout disables single-pointer movement and refuses direct controller movement', [editor.el.querySelector('[data-ed-group-move]').disabled, ed.groups.move(id, 16, 0), model.node(A).position.x], [true, false, start]);
        editor.dispose(); eq('groups disposal releases inspector resize listeners', (h.winListeners.resize || []).length, 0);
    });
    await guardAsync('groups collapse preserves graph and minimap boundary navigation', async () => {
        const { h, editor, model, canvas } = fixture(), ed = editor.ed; const id = model.createGroup([A, B], 'Grouped'), snapshot = model.toJSON(), version = model.version;
        ed.groups.toggle(id, 'collapse'); h.flushFrames(); h.runTimers(400);
        eq('groups collapse is view state and hides only member cards and internal edges', [model.toJSON(), model.version, canvas.nodeEl(A).hidden, canvas.nodeEl(B).hidden, canvas.nodeEl(T).hidden, canvas.wiresSvg.querySelectorAll('.ed-edge').length], [snapshot, version, true, true, false, 1]);
        eq('groups minimap shows one summary and the visible outside node', [editor.el.querySelectorAll('.ed-minimap-nodes rect').length, editor.el.querySelectorAll('.ed-minimap-wires line').length], [2, 1]);
        const rect = ed.groups.bounds(model.group(id)); eq('groups boundary endpoint lands on the collapsed summary', ed.groups.endpoint(A, 'in', { x: -1, y: -1 }), { x: rect.x, y: rect.y + rect.h / 2 });
        canvas.setOutline(true); h.runTimers(400); const member = editor.el.querySelector('[data-ed-focus-node="' + A + '"]');
        eq('groups outline retains every hidden member as an accessible button', [!!member, member.tabIndex], [true, 0]); member.fire('click');
        eq('groups navigation expands before focusing the real step', [ed.groups.hidden(A), canvas.nodeEl(A).hidden, Array.from(ed.selection)], [false, false, [A]]);
        editor.dispose();
    });
    await guardAsync('groups run summaries patch only affected counters and reset safely', async () => {
        const { h, editor, model } = fixture(), ed = editor.ed, a = model.createGroup([A], 'A'), b = model.createGroup([B], 'B');
        const head = id => editor.el.querySelector('.ed-group-head[data-ed-group="' + id + '"]');
        const first = head(a), other = head(b), controls = first.querySelector('.ed-group-controls').firstChild;
        ed.run = { id: 'r1', steps: new Map(), changes: [] }; ed.bus.emit('run', ed.run);
        const untouched = other.querySelector('.ed-group-counts').textContent;
        let reads = 0; const query = other.querySelector.bind(other); other.querySelector = selector => { reads++; return query(selector); };
        for (let i = 0; i < 50; i++) { ed.run.steps.set(A, { status: i % 2 ? 'running' : 'error' }); ed.run.changes.push(A); ed.bus.emit('run', ed.run); }
        eq('groups journal updates never rebuild controls or touch unrelated summaries', [first.querySelector('.ed-group-controls').firstChild === controls, reads, query('.ed-group-counts').textContent === untouched], [true, 0, true]);
        ed.run = null; ed.bus.emit('run', null); eq('groups clearing a run resets stale counters', first.querySelector('.ed-group-counts').textContent.includes('group_running'), false);
        editor.dispose();
    });
    await guardAsync('groups readonly inspection never announces or writes a group command', async () => {
        const h = sandbox(() => undefined), doc = { schema: 1, name: 'Read', nodes: [{ id: A, key: 'alpha', type: 'logic.set', position: { x: 0, y: 0 }, params: {}, settings: {} }], edges: [], groups: [{ id: 'g_abcdefgh', title: 'Read group', color: 'rose', members: [A] }] };
        const editor = openEditor(h, { readonly: true, flow: { draft: doc } }), ed = editor.ed, before = ed.model.toJSON(); ed.selection.add(A); ed.groups.createFromSelection(); ed.groups.open('g_abcdefgh');
        const title = editor.el.querySelector('[data-ed-group-title]'); title.value = 'Refused'; title.fire('change'); ed.groups.move('g_abcdefgh', 16, 0);
        eq('groups readonly controls preserve documents and expose the same inspection path', [title.readOnly, editor.el.querySelector('[data-ed-group-ungroup]').disabled, ed.model.toJSON(), h.notes.length, h.puts().length], [true, true, before, 0, 0]);
        ed.groups.toggle('g_abcdefgh', 'collapse'); eq('groups readonly collapse is allowed as view state only', [ed.model.toJSON(), ed.groups.hidden(A)], [before, true]); editor.dispose();
    });
    await guardAsync('groups shared mouse touch keyboard movement preserves mixed selection exactly once', async () => {
        const { h, editor, model, canvas } = fixture(), ed = editor.ed;
        model.moveNodes({ [A]: { x: 304, y: 0 }, [B]: { x: 608, y: 0 } }); const id = model.createGroup([A, B], 'Mixed');
        canvas.setView({ x: 0, y: 0, zoom: 1 });
        ed.groups.select(id); ed.selection.add(T); const before = model.toJSON();
        const handle = editor.el.querySelector('[data-ed-group-drag="' + id + '"]'), start = h.pe(1, 100, 100);
        handle.fire('pointerdown', start); canvas.el.fire('pointermove', h.pe(1, 132, 100)); canvas.el.fire('pointerup', Object.assign(h.pe(1, 132, 100), { buttons: 0 }));
        eq('groups header drag preserves selected outside nodes and moves every member once', [model.node(A).position.x, model.node(B).position.x, model.node(T).position.x, ed.selection.size], [336, 640, 32, 3]);
        model.undo(); eq('groups pointer gesture has one complete undo', model.toJSON(), before);
        const touch = Object.assign(h.pe(2, 100, 100), { pointerType: 'touch' });
        handle.fire('pointerdown', touch); canvas.el.fire('pointermove', Object.assign(h.pe(2, 132, 100), { pointerType: 'touch' })); canvas.el.fire('pointercancel', Object.assign(h.pe(2, 132, 100), { pointerType: 'touch' }));
        eq('groups cancelled touch gesture restores every selected position', [model.node(A).position.x, model.node(B).position.x, model.node(T).position.x], [304, 608, 0]);
        ed.groups.toggle(id, 'lock'); ed.selection.add(T); h.fireDoc('keydown', h.key('ArrowRight', canvas.el, { shiftKey: true }));
        eq('groups keyboard movement honors locked members and moves an outside selection', [model.node(A).position.x, model.node(B).position.x, model.node(T).position.x], [304, 608, 8]);
        canvas.autoLayout(); eq('groups auto-layout retains locked real positions', [model.node(A).position.x, model.node(B).position.x], [304, 608]);
        ed.groups.toggle(id, 'lock'); ed.groups.select(id); h.fireDoc('keydown', h.key('Delete', canvas.el));
        eq('groups Delete on a group ungroups and keeps real steps', [model.doc.groups, model.doc.nodes.length, ed.selection.size], [[], 3, 0]);
        model.undo(); eq('groups ungroup keyboard command has one undo', !!model.group(id), true); editor.dispose();
    });
    await guardAsync('groups initial issues and touch context use the actual group target', async () => {
        const h = sandbox(() => undefined), make = h.ED.groups.create;
        h.ED.groups.create = (ed, canvas) => { ed.issues = [{ severity: 'error', node_id: A }]; return make(ed, canvas); };
        const editor = openEditor(h), ed = editor.ed, id = ed.model.createGroup([A, B], 'Initial');
        eq('groups initial backend issues appear without waiting for another validation event', ed.groups.countText(ed.model.group(id)).includes('group_errors'), true);
        const menus = []; h.ctx.showContextMenu = (...args) => menus.push(args);
        const before = ed.model.toJSON(), handle = editor.el.querySelector('[data-ed-group-drag="' + id + '"]');
        handle.fire('pointerdown', Object.assign(h.pe(1, 100, 100), { pointerType: 'touch' })); h.runTimers(550);
        eq('groups touch hold opens group properties rather than a hidden member context menu', [ed.detail.groupId(), menus.length, ed.model.toJSON()], [id, 0, before]);
        editor.dispose();
    });
    await guardAsync('groups undo of an explicitly null legacy boundary must clear acknowledged metadata', async () => {
        let stored, revision = 3;
        const h = sandbox(req => { if (req.method === 'PUT' && req.url === '/api/desktop/flows/f1') { const next = JSON.parse(JSON.stringify(req.body.doc)); if (next.groups == null && stored) next.groups = stored.groups; stored = next; return { draft_revision: ++revision, issues: [] }; } });
        const editor = openEditor(h), model = editor.ed.model; model.replaceDoc(Object.assign({}, model.toJSON(), { groups: null }));
        eq('groups loading null metadata leaves the legacy document untouched', model.doc.groups, null);
        model.createGroup([A, B], 'New'); await editor.ed.saver.flush(); model.undo();
        eq('groups an intentional null-state undo sends [] rather than the preserve sentinel', model.doc.groups, []);
        await editor.ed.saver.flush(); eq('groups acknowledged null-origin undo remains clear after save and reload', [stored.groups, h.ED.model.create(stored, h.catalog).doc.groups], [[], []]); editor.dispose();
    });
}
