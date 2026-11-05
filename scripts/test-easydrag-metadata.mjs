// C6 user-level metadata/CAS/privacy checks on the existing EasyDrag DOM harness.
import { webcrypto } from 'node:crypto';
export async function run({ sandbox, openEditor, eq, guardAsync, settle }) {
    const copy = value => JSON.parse(JSON.stringify(value));
    const failure = (status, code) => Object.assign(new Error(code), { status, body: { code } });
    const pending = () => { let resolve, reject; const promise = new Promise((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; };
    function fixture(extra) {
        const state = { folders: [{ id: 'folder_a', name: 'A', revision: 1, flow_count: 0, editable: true, redacted: false }, { id: 'folder_b', name: 'B', revision: 1, flow_count: 0, editable: true, redacted: false }], library: { folder_id: null, tags: [], revision: 2, tags_redacted: false, tags_editable: true }, note: { note: 'Stored note', revision: 4, redacted: false, editable: true }, writeError: null, readError: null, flows: [{ id: 'f1', name: 'Flow', published: false, enabled: false, triggers: [] }] };
        const h = sandbox(req => {
            const result = extra && extra(req, state); if (result !== undefined) return result;
            const metadata = req.url === '/api/desktop/flow-folders' || req.url.endsWith('/library') || req.url.endsWith('/note');
            if (metadata && req.method === 'GET' && state.readError) throw state.readError;
            if (metadata && req.method !== 'GET' && state.writeError) throw state.writeError;
            if (req.url === '/api/desktop/flows') return { flows: state.flows.map(flow => Object.assign({}, flow, { library: copy(flow.library || state.library) })) };
            if (req.url.startsWith('/api/desktop/flows/templates')) return { templates: [] };
            if (req.url === '/api/desktop/flow-folders') return { folders: copy(state.folders) };
            if (req.url.endsWith('/publish')) return { flow: { id: 'f1', draft_revision: 3, live_revision: 1 } };
            if (req.url.endsWith('/library')) {
                if (req.method === 'PUT') { if (req.body.base_revision !== state.library.revision) throw failure(409, 'FLOW_LIBRARY_CONFLICT'); for (const field of ['folder_id', 'tags']) if (Object.hasOwn(req.body, field)) state.library[field] = copy(req.body[field]); state.library.revision++; }
                return { library: copy(state.library) };
            }
            if (req.url.endsWith('/note')) {
                if (req.method === 'PUT') { if (req.body.base_revision !== state.note.revision) throw failure(409, 'FLOW_VERSION_NOTE_CONFLICT'); state.note.note = req.body.note; state.note.revision++; state.note.redacted = false; state.note.editable = true; }
                return copy(state.note);
            }
        });
        const editor = openEditor(h); h.confirmAnswer = false; h.win.crypto = webcrypto;
        const model = () => [editor.ed.model.toJSON(), editor.ed.model.version, editor.ed.saver.revision, editor.ed.model.canUndo(), editor.ed.testEvidence];
        const writes = suffix => h.requests.filter(req => req.method === 'PUT' && req.url.endsWith(suffix));
        const home = () => { const view = h.ED.home.create({ ctx: h.ctx, t: h.ctx.t, esc: h.ED.core.esc, api: h.api, catalog: h.catalog, readonly: false, openFlow() {} }); h.body.appendChild(view.el); return view; };
        return { h, editor, state, model, writes, home };
    }
    await guardAsync('metadata bounded literal text accepts valid strings and rejects markers controls and broken Unicode', async () => {
        const { h, editor } = fixture(), m = h.ED.metadata;
        eq('metadata string validation is independent of object-only sample JSON parsing', [m.clean('ordinary redacted word'), m.clean(''), m.clean('before [redacted] after')], [true, true, false]);
        eq('metadata UTF8 and codepoint bounds preserve valid emoji but reject lone surrogates', [m.textFits('😀', 1, 4), m.textFits('😀', 1, 3), m.textFits('\uD800', 1, 3), m.textFits('\uDC00', 1, 3)], [true, false, false, false]);
        eq('metadata notes allow only the documented multiline controls', [m.textFits('a\nb\tc\r', 20, 30, true), m.textFits('a\nb', 20, 30), m.textFits('a\u0085b', 20, 30, true), m.trim('\u0085 name \u0085')], [true, false, false, 'name']);
        editor.dispose();
    });
    await guardAsync('metadata folder changes preserve hidden tags and never alter the document saver undo or evidence', async () => {
        const { h, editor, state, model, writes } = fixture(); await settle();
        state.library.tags = ['[redacted]']; state.library.tags_redacted = true; state.library.tags_editable = false;
        editor.ed.testEvidence = { revision: 3, mode: 'simulation', status: 'success' };
        const before = copy(model()), dialog = h.ED.metadata.organization(editor.ed, 'f1'); await settle();
        const select = dialog.body.querySelector('[data-ed-organization-folder]'); select.value = 'folder_b'; select.fire('change');
        dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        eq('metadata selective folder PUT omits hidden tags', writes('/library')[0].body, { base_revision: 2, folder_id: 'folder_b' });
        eq('metadata organization is not a graph edit or test invalidation', [model(), state.library.tags, h.requests.some(req => req.method === 'PUT' && req.url === '/api/desktop/flows/f1')], [before, ['[redacted]'], false]);
        editor.dispose();
    });
    await guardAsync('metadata tags retain exact case and pending text and respect IME on Save', async () => {
        const { h, editor, state, writes } = fixture(); await settle();
        const dialog = h.ED.metadata.organization(editor.ed, 'f1'); await settle();
        const input = dialog.body.querySelector('[data-ed-tag-input]');
        input.value = 'Home'; input.fire('keydown', { key: 'Enter' }); input.value = 'home'; dialog.body.querySelector('[data-ed-tag-add]').fire('click');
        input.value = 'Home'; dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        eq('metadata duplicate tags preserve the raw edit and send nothing', [input.value, writes('/library').length], ['Home', 0]);
        input.value = 'Fresh'; input.fire('compositionstart'); dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        eq('metadata Save never cuts off an uncommitted tag composition', [input.value, writes('/library').length], ['Fresh', 0]);
        input.fire('compositionend'); dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        eq('metadata Save commits a pending literal tag and preserves Home versus home', writes('/library')[0].body, { base_revision: 2, tags: ['Home', 'home', 'Fresh'] });
        eq('metadata explicit tags replacement did not silently add a folder write', Object.hasOwn(writes('/library')[0].body, 'folder_id'), false);
        editor.dispose();
    });
    await guardAsync('metadata notes keep their CAS revision after clearing and stay outside the document', async () => {
        for (const block of [false, true]) {
            const { h, editor, state, model, writes } = fixture(); await settle();
            const env = block ? Object.assign({}, editor.ed, { api: h.ED.blocks.api(h.transport), isBlock: true }) : editor.ed;
            const before = copy(model()); let dialog = h.ED.metadata.note(env, 1, true); await settle();
            dialog.body.querySelector('[data-ed-note-text]').value = ''; dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
            dialog = h.ED.metadata.note(env, 1, true); await settle(); dialog.body.querySelector('[data-ed-note-text]').value = 'Again'; dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
            eq('metadata empty notes retain the next CAS base for ' + (block ? 'block' : 'flow'), writes('/note').map(req => req.body), [{ note: '', base_revision: 4 }, { note: 'Again', base_revision: 5 }]);
            eq('metadata note changes never mutate graph state', model(), before);
            editor.dispose();
        }
    });
    await guardAsync('metadata conflict reload requires explicit permission before replacing pending note text', async () => {
        const { h, editor, state, writes } = fixture(); await settle();
        const dialog = h.ED.metadata.note(editor.ed, 1, true); await settle();
        let area = dialog.body.querySelector('[data-ed-note-text]'); area.value = 'Mine'; state.note.note = 'Other window'; state.note.revision++;
        dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        eq('metadata conflict keeps unsaved text and exposes explicit reload', [area.value, dialog.body.querySelector('[data-ed-metadata-reload]').hidden], ['Mine', false]);
        dialog.body.querySelector('[data-ed-metadata-reload]').fire('click'); await settle();
        eq('metadata declining conflict reload preserves text and never retries PUT', [area.value, writes('/note').length], ['Mine', 1]);
        h.confirmAnswer = true; dialog.body.querySelector('[data-ed-metadata-reload]').fire('click'); await settle(); area = dialog.body.querySelector('[data-ed-note-text]');
        eq('metadata confirmed reload uses the latest text without replaying the write', [area.value, writes('/note').length], ['Other window', 1]);
        editor.dispose();
    });
    await guardAsync('metadata write denial probes read access and keeps a locked pending note without retry', async () => {
        for (const probe of ['allowed', 'network', 'forbidden', 'desktop_readonly']) {
            const { h, editor, state, writes } = fixture(); await settle();
            let denied = 0; const dialog = h.ED.metadata.note(editor.ed, 1, true, undefined, () => { denied++; }); await settle();
            const area = dialog.body.querySelector('[data-ed-note-text]'); area.value = 'My pending note';
            state.writeError = failure(403, probe === 'desktop_readonly' ? 'desktop_readonly' : 'FLOW_PERMISSION_DENIED');
            if (probe === 'network') state.readError = new Error('Offline');
            if (probe === 'forbidden') state.readError = failure(403, 'desktop_scope_required');
            dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
            const current = dialog.body.querySelector('[data-ed-note-text]');
            eq('metadata write/read rights are separated for ' + probe, [current ? current.value : null, dialog.el.querySelector('[data-ed-action="save"]').disabled, writes('/note').length, denied, h.requests.filter(req => req.method === 'GET' && req.url.endsWith('/note')).length], [probe === 'forbidden' ? null : 'My pending note', true, 1, probe === 'forbidden' ? 1 : 0, probe === 'desktop_readonly' ? 1 : 2]);
            editor.dispose();
        }
    });
    await guardAsync('metadata cancelled requests cannot repaint a reopened note or another editor', async () => {
        const { h, editor } = fixture(); await settle();
        const old = pending(), signals = []; let count = 0, updates = 0;
        h.api.versionNote = (id, revision, signal) => { signals.push(signal); return ++count === 1 ? Promise.resolve({ note: 'Old', revision: 1, editable: true }) : count === 2 ? old.promise : Promise.resolve({ note: 'New', revision: 9, editable: true }); };
        h.api.saveVersionNote = async () => { throw failure(403, 'FLOW_PERMISSION_DENIED'); };
        let dialog = h.ED.metadata.note(editor.ed, 1, true, () => { updates++; }); await settle(); dialog.body.querySelector('[data-ed-note-text]').value = 'My edit'; dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle();
        dialog.close(null); dialog = h.ED.metadata.note(editor.ed, 1, true, () => { updates++; }); await settle();
        old.resolve({ note: '[redacted]', revision: 2, redacted: true, editable: false }); await settle();
        eq('metadata late read probes are aborted and cannot overwrite a new dialog', [signals[1].aborted, dialog.body.querySelector('[data-ed-note-text]').value, updates], [true, 'New', 2]);
        const saving = pending(); let savedSignal;
        h.api.saveVersionNote = (id, revision, text, base, signal) => { savedSignal = signal; return saving.promise; };
        dialog.body.querySelector('[data-ed-note-text]').value = 'Unfinished save'; dialog.el.querySelector('[data-ed-action="save"]').fire('click'); await settle(); editor.dispose(); saving.resolve({ note: 'Late', revision: 10, editable: true }); await settle();
        eq('metadata editor disposal aborts pending PUT and suppresses its result', [savedSignal.aborted, updates], [true, 2]);
    });
    await guardAsync('metadata reading a newly masked note replaces flow and block list previews before closing', async () => {
        for (const block of [false, true]) {
            const { h, editor, state } = fixture(); await settle();
            const env = block ? Object.assign({}, editor.ed, { api: h.ED.blocks.api(h.transport), isBlock: true }) : editor.ed;
            const rows = [{ revision: 1, note_preview: 'Formerly public', note_revision: 3, note_editable: true }];
            const parent = h.ED.core.modal(editor.el, { title: 'Versions', body: '' });
            const paint = () => { parent.body.innerHTML = h.ED.metadata.noteMarkup(env, rows[0]); };
            paint(); h.ED.metadata.bindNotes(env, parent, rows, paint);
            state.note = { note: '[redacted]', revision: 4, redacted: true, editable: false };
            parent.body.querySelector('[data-ed-note-read]').fire('click'); await settle();
            eq('metadata GET immediately scrubs cached ' + (block ? 'block' : 'flow') + ' previews', [rows[0].note_preview, rows[0].note_redacted, rows[0].note_editable, parent.body.html.includes('Formerly public')], ['[redacted]', true, false, false]);
            h.ED.metadata.dispose(editor.el); state.note = { note: '😀'.repeat(121) + '\r\nsecond line', revision: 5, editable: true };
            parent.body.querySelector('[data-ed-note-read]').fire('click'); await settle();
            eq('metadata preview cuts at 120 whole codepoints and the first line', rows[0].note_preview, '😀'.repeat(120) + '…');
            h.ED.metadata.dispose(editor.el); state.readError = failure(403, 'desktop_scope_required');
            parent.body.querySelector('[data-ed-note-edit]').fire('click'); await settle();
            eq('metadata revoked read access clears the linked versions cache', [rows[0].note_preview, rows[0].note_editable, !!rows[0].note_error, !!parent.body.querySelector('[data-ed-note-read]')], ['', false, true, false]);
            parent.close(null); editor.dispose();
        }
    });
    await guardAsync('metadata library options are cached for 1000 flows with 16 distinct tags each', async () => {
        const { h, editor, state, home } = fixture();
        state.flows = Array.from({ length: 1000 }, (_, index) => ({ id: 'f' + index, name: 'Flow ' + index, triggers: [], library: { folder_id: null, revision: 1, tags: Array.from({ length: 16 }, (_, tag) => index + ':' + tag) } }));
        const view = home(); await settle();
        const picker = view.el.querySelector('[data-ed-tag-filter]'), option = picker.querySelectorAll('option')[1];
        const search = view.el.querySelector('[data-ed-home-search]'); search.value = 'no matching flow'; search.fire('input'); h.runTimers(120);
        view.el.querySelector('[data-ed-library-view="list"]').fire('click'); view.el.querySelector('[data-ed-library-favorites]').fire('click');
        eq('metadata all 16000 tags remain available without select DOM churn on local view and search events', [picker.querySelectorAll('option').length, picker.querySelectorAll('option')[1] === option], [16001, true]);
        picker.focus(); state.flows[0].library.tags.push('New tag'); await view.reload();
        eq('metadata an open unchanged picker survives background options updates', picker.querySelectorAll('option')[1] === option, true);
        h.body.focus(); picker.fire('blur'); h.flushFrames();
        eq('metadata deferred options appear after leaving the picker', picker.querySelectorAll('option').length, 16002);
        view.dispose(); editor.dispose();
    });
    await guardAsync('metadata secret tags disappear even from a focused picker and saved filters use neutral missing options', async () => {
        const { h, editor, state, home } = fixture();
        state.library.tags = [':missing', 'Formerly public'];
        const view = home(); await settle(); const picker = view.el.querySelector('[data-ed-tag-filter]');
        picker.value = 'tag:Formerly public'; picker.fire('change');
        h.ctx.promptDialog = async () => 'Keep organization view'; view.el.querySelector('[data-ed-library-save]').fire('click'); await settle();
        const key = 'aurago.easydrag.library', saved = JSON.parse(h.store.get(key));
        picker.focus(); state.library.tags = [':missing', '[redacted]']; state.library.tags_redacted = true; state.library.tags_editable = false; await view.reload();
        eq('metadata masking purges focused options and uses an opaque unavailable selection', [picker.value, picker.html.includes('Formerly public')], [':unavailable', false]);
        state.flows = [{ id: 'f2', name: 'Other', triggers: [], library: { tags: [':missing'], revision: 1 } }]; h.body.focus(); await view.reload();
        picker.value = 'tag::missing'; picker.fire('change');
        eq('metadata a real :missing tag remains distinct from the unavailable placeholder', [picker.value, view.el.querySelectorAll('[data-ed-flow]').map(row => row.dataset.edFlow)], ['tag::missing', ['f2']]);
        const preset = view.el.querySelector('[data-ed-library-filter]'); preset.value = saved.filters[0].id; preset.fire('change');
        eq('metadata restoring old local filters never puts hidden raw tags back into the DOM or deletes the preset', [picker.value, picker.html.includes('Formerly public'), JSON.parse(h.store.get(key)).filters[0].tag], [':unavailable', false, 'Formerly public']);
        view.dispose(); editor.dispose();
    });
    await guardAsync('metadata new publication is atomic while same-draft repair always omits its committed note', async () => {
        const { h, editor, state } = fixture(); await settle(); const m = h.ED.metadata, env = editor.ed;
        let input = m.publicationNote(env, { draft_revision: 3, live_revision: 0, published_draft_revision: 0 }); editor.el.appendChild(input.el);
        const area = input.el.querySelector('textarea'); area.value = '  version one\nExact whitespace  ';
        area.fire('compositionstart'); eq('metadata publication refuses an unfinished composition', input.flush(), false); area.fire('compositionend');
        eq('metadata valid publication note preserves its exact whitespace', [input.flush(), input.value()], [true, '  version one\nExact whitespace  ']);
        await h.api.publish('f1', 3, input.value()); input.dispose();
        state.note.note = '  version one\nExact whitespace  ';
        input = m.publicationNote(env, { draft_revision: 3, live_revision: 1, published_draft_revision: 3 }); editor.el.appendChild(input.el); await settle();
        await h.api.publish('f1', 3, input.value());
        eq('metadata repair only reads the existing note and omits any note field on publish', [!!input.el.querySelector('textarea'), input.value(), h.requests.filter(req => req.url.endsWith('/publish')).map(req => req.body)], [false, undefined, [{ base_revision: 3, note: '  version one\nExact whitespace  ' }, { base_revision: 3 }]]);
        input.dispose(); editor.dispose();
    });
}
