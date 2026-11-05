// Pure local-pack wire-contract checks. DOM journeys live in the native EasyDrag fixtures.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

function loadPackHarness() {
    const here = path.dirname(fileURLToPath(import.meta.url));
    const source = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'easydrag-pack.js');
    const state = { requests: [], downloads: [], events: [], objectURLs: 0, timers: [], fetchHandler: null };
    const anchor = { href: '', download: '', click() { state.downloads.push(this.download); }, remove() {} };
    const sandbox = vm.createContext({
        window: { EasyDrag: {} }, AbortController,
        fetch(url, options) {
            state.requests.push({ url, options });
            state.events.push('GET');
            if (!state.fetchHandler) return Promise.reject(new Error('unexpected fetch'));
            return state.fetchHandler(url, options);
        },
        URL: { createObjectURL() { state.objectURLs++; return 'blob:pack-test'; }, revokeObjectURL() {} },
        document: { body: { appendChild() {} }, createElement: () => anchor },
        setTimeout(fn, delay) { state.timers.push({ fn, delay }); return state.timers.length; }
    });
    vm.runInContext(fs.readFileSync(source, 'utf8'), sandbox, { filename: source });
    return { pack: sandbox.window.EasyDrag.pack, state };
}

function responseFor(pack) {
    const raw = JSON.stringify(pack);
    return {
        ok: true,
        headers: { get(name) { return name.toLowerCase() === 'content-length' ? String(raw.length) : 'attachment; filename="Bundle.easydrag-pack.json"'; } },
        async blob() { return { size: raw.length, async text() { return raw; } }; }
    };
}

function exportEditorFixture(state, { readonly = false, dirty = true, flushResult = true } = {}) {
    const saveState = { dirty };
    const ed = {
        api: { localPackUrl: id => '/api/desktop/flows/' + id + '/pack' },
        flow: { id: 'flow-source' },
        draftModel: { version: 11, doc: { name: 'Bundle' } },
        saver: {
            revision: 7,
            isDirty: () => saveState.dirty,
            async flush() { state.events.push('flush'); if (flushResult) saveState.dirty = false; return flushResult; }
        },
        readonly,
        root: { isConnected: true },
        flushPendingEdits() { state.events.push('flushPendingEdits'); return true; }
    };
    return ed;
}

async function waitForRequest(state) {
    for (let i = 0; i < 20 && state.requests.length === 0; i++) await Promise.resolve();
    assert.ok(state.requests.length === 1, 'export should start exactly one held GET');
}

export async function run({ sandbox, pack, eq } = {}) {
    pack = pack || (sandbox ? sandbox().ED.pack : loadPackHarness().pack);
    eq = eq || ((name, got, want) => assert.equal(JSON.stringify(got), JSON.stringify(want), name));
    const helpers = pack._test;
    const hash = char => char.repeat(64);
    const sourcePack = {
        format: 'aurago.easydrag.local-pack', version: 1,
        parent: { source_revision: 7, hash: hash('1'), flow: { kind: 'flow', id: 'flow-source', name: 'Parent' } },
        blocks: [
            { source_id: 'source-z', source_revision: 3, hash: hash('2'), flow: { kind: 'block', id: 'source-z', name: 'Z', nodes: [
                { type: 'component.call', params: { flow_id: 'source-a', revision: 2 } }
            ] } },
            { source_id: 'source-a', source_revision: 2, hash: hash('3'), flow: { kind: 'block', id: 'source-a', name: 'A' } }
        ]
    };
    const proposal = {
        pack_digest: hash('4'), proposal_digest: hash('5'), parent_id: 'fresh-parent', parent_hash: hash('6'),
        blocks: [
            { source_id: 'source-a', source_revision: 2, local_id: 'fresh-a', hash: hash('7') },
            { source_id: 'source-z', source_revision: 3, local_id: 'fresh-z', hash: hash('8') }
        ]
    };
    const review = {
        parent: { name: 'Parent', source_id: 'flow-source', source_revision: 7 },
        blocks: [
            { source_id: 'source-z', source_revision: 3, name: 'Z' },
            { source_id: 'source-a', source_revision: 2, name: 'A' }
        ],
        sample_data_cleared: true, local_only: true
    };

    eq('pack envelope recognizes fixed format', helpers.isPackEnvelope(sourcePack), true);
    eq('pack shape accepts canonical envelope structure', helpers.packShape(sourcePack), true);
    eq('pack filename is case insensitive', helpers.isPackName('backup.EASYDRAG-PACK.JSON'), true);
    eq('export snapshot requires source ID and exact draft revision', [
        helpers.exportSnapshotMatches(sourcePack, 'flow-source', 7),
        helpers.exportSnapshotMatches(sourcePack, 'other-flow', 7),
        helpers.exportSnapshotMatches(sourcePack, 'flow-source', 8),
        helpers.exportSnapshotMatches({ ...sourcePack, blocks: [{ ...sourcePack.blocks[0], hash: 'bad' }] }, 'flow-source', 7)
    ], [true, false, false, false]);
    eq('preview targets may have fresh hashes', helpers.validProposal(proposal, sourcePack, review), true);
    eq('review binds its parent identity to the pack source', helpers.validProposal(proposal, sourcePack, { ...review, parent: { ...review.parent, source_id: 'other-parent' } }), false);

    const orderedDifferently = {
        ...proposal,
        blocks: [...proposal.blocks].reverse()
    };
    eq('proposal and review are matched by source identity, not order', helpers.validProposal(orderedDifferently, sourcePack, review), true);
    eq('proposal rejects duplicate local IDs', helpers.validProposal({ ...proposal, blocks: proposal.blocks.map(target => ({ ...target, local_id: 'duplicate' })) }, sourcePack, review), false);

    const flowRecord = {
        id: proposal.parent_id, kind: 'flow', draft_revision: 1, live_revision: 0, published_draft_revision: 0,
        mission_id: 'mission-fresh-parent', hash: proposal.parent_hash, pins: []
    };
    const blockRecords = [
        { id: 'fresh-z', kind: 'block', draft_revision: 1, live_revision: 1, published_draft_revision: 1, hash: hash('8'), pins: [{ block_id: 'fresh-a', revision: 1, hash: hash('7') }] },
        { id: 'fresh-a', kind: 'block', draft_revision: 1, live_revision: 1, published_draft_revision: 1, hash: hash('7'), mission_id: '', pins: [] }
    ];
    eq('confirmed records may arrive in any order and omit block mission_id', helpers.validRecords(flowRecord, blockRecords, proposal), true);
    const expectedPins = helpers.expectedBlockPins(sourcePack, proposal);
    const derivedPin = Array.from(expectedPins.get('fresh-z') || [])[0] || '';
    eq('expected pin closure remaps source references by identity', [Array.from(expectedPins.keys()).sort(), expectedPins.get('fresh-a').size,
        derivedPin.startsWith('fresh-a'), derivedPin.endsWith('1'), derivedPin.includes('\u0000')], [['fresh-a', 'fresh-z'], 0, true, true, true]);
    eq('confirmed records match the exact derived pin closure', helpers.validRecords(flowRecord, blockRecords, proposal, expectedPins), true);
    eq('confirmed records reject a missing nested pin', helpers.validRecords(flowRecord, blockRecords.map(block => block.id === 'fresh-z' ? { ...block, pins: [] } : block), proposal, expectedPins), false);
    eq('changed target record hash is rejected', helpers.validRecords(flowRecord, blockRecords.map(block => block.id === 'fresh-a' ? { ...block, hash: hash('9') } : block), proposal), false);
    eq('parent published revision must remain disabled', helpers.validRecords({ ...flowRecord, published_draft_revision: 1 }, blockRecords, proposal), false);
    eq('block published revision must match revision one', helpers.validRecords(flowRecord, blockRecords.map(block => block.id === 'fresh-z' ? { ...block, published_draft_revision: 0 } : block), proposal), false);
    eq('parent mission owner is required in confirmed records', helpers.validRecords({ ...flowRecord, mission_id: undefined }, blockRecords, proposal), false);
    eq('confirmed empty closure requires and accepts an empty blocks array', helpers.validRecords({ ...flowRecord, hash: proposal.parent_hash, pins: [] }, [], { ...proposal, blocks: [] }), true);
    eq('confirmed empty closure rejects omitted blocks', helpers.validRecords({ ...flowRecord, hash: proposal.parent_hash, pins: [] }, undefined, { ...proposal, blocks: [] }), false);

    const fileResult = async (name, raw, size = raw.length) => helpers.readImportFile({ name, size, text: async () => raw });
    eq('pack filename selects pack import', (await fileResult('flow.easydrag-pack.json', JSON.stringify(sourcePack))).kind, 'pack');
    eq('pack envelope with unsupported version is distinguished', (await fileResult('flow.easydrag-pack.json', JSON.stringify({ ...sourcePack, version: 2 }))).kind, 'pack-unsupported');
    eq('invalid named pack remains a pack error', (await fileResult('flow.easydrag-pack.json', '{')).kind, 'pack-invalid');
    const legacyV2 = { schema: 2, name: 'Grouped legacy', nodes: [{ id: 'n_aaaaaaaa', type: 'logic.set', params: { value: 'kept' } }],
        edges: [], groups: [{ id: 'g_abcdefgh', title: 'Kept group', color: 'neutral', members: ['n_aaaaaaaa'] }] };
    const legacyV2Result = await fileResult('legacy.json', JSON.stringify(legacyV2));
    eq('legacy schema-2 document keeps its nodes, edges and groups unchanged', [legacyV2Result.kind, legacyV2Result.doc], ['legacy', legacyV2]);
    eq('generic JSON marker imports a pack above the legacy 4 MiB cap',
        (await fileResult('portable.json', JSON.stringify(sourcePack), 4 * 1024 * 1024 + 1)).kind, 'pack');
    let readOversized = false;
    const oversizedFile = name => ({ name, size: pack.MAX_FILE_BYTES + 1, text: async () => { readOversized = true; return '{}'; } });
    const genericTooLarge = await helpers.readImportFile(oversizedFile('huge.json'));
    const namedTooLarge = await helpers.readImportFile(oversizedFile('huge.easydrag-pack.json'));
    eq('over-32-MiB JSON is rejected before file read', [genericTooLarge.kind, namedTooLarge.kind, readOversized], ['legacy-too-large', 'pack-too-large', false]);

    const budget = { left: 3, truncated: false };
    const bounded = helpers.capped({ a: { b: { c: 'unvisited' } }, d: 'also unvisited' }, budget, 0);
    eq('preview tree consumes a node budget and reports truncation', [budget.left, budget.truncated, JSON.stringify(bounded).includes('unvisited')], [0, true, false]);
    const longState = { left: 5, truncated: false };
    const longPreview = helpers.capped('x'.repeat(8005), longState, 0);
    eq('preview strings have a fixed content cap', [longPreview.length, longPreview.endsWith('…'), longState.truncated], [8000, true, true]);
    const charsState = { left: 10, characters: 5, truncated: false };
    const charsPreview = helpers.capped(['abc', 'def'], charsState, 0);
    eq('preview tree shares an aggregate character budget', [charsPreview, charsState.characters, charsState.truncated], [['abc', 'd…'], 0, true]);

    const escape = value => String(value).replace(/[&<>"']/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[char]);
    const hostile = '</span><script>alert(1)</script>&$&';
    eq('review placeholders escape hostile text and preserve replacement literals',
        helpers.formatMarkup('Name: {name}', { name: hostile }, escape),
        'Name: &lt;/span&gt;&lt;script&gt;alert(1)&lt;/script&gt;&amp;$&amp;');

    const exportDoc = {
        format: 'aurago.easydrag.local-pack', version: 1,
        parent: { source_revision: 7, hash: hash('a'), flow: { kind: 'flow', id: 'flow-source', name: 'Bundle' } },
        blocks: []
    };

    {
        const { pack: exportPack, state } = loadPackHarness();
        const ed = exportEditorFixture(state, { dirty: true, flushResult: false });
        const result = await exportPack.exportEditor(ed);
        eq('export failed flush sends no GET or download', [result, state.events, state.requests.length, state.downloads.length],
            [false, ['flushPendingEdits', 'flush'], 0, 0]);
    }
    {
        const { pack: exportPack, state } = loadPackHarness();
        const ed = exportEditorFixture(state, { readonly: true, dirty: true });
        const result = await exportPack.exportEditor(ed);
        eq('export readonly dirty draft sends no GET or download', [result, state.events, state.requests.length, state.downloads.length], [false, [], 0, 0]);
    }

    for (const [label, change] of [
        ['draft version', ed => { ed.draftModel.version++; }],
        ['saver revision', ed => { ed.saver.revision++; }]
    ]) {
        const { pack: exportPack, state } = loadPackHarness();
        let resolveFetch;
        state.fetchHandler = () => new Promise(resolve => { resolveFetch = resolve; });
        const ed = exportEditorFixture(state);
        const pending = exportPack.exportEditor(ed);
        await waitForRequest(state);
        change(ed);
        resolveFetch(responseFor(exportDoc));
        const result = await pending;
        eq('export does not download after held GET with changed ' + label, [result, state.requests.length, state.downloads.length], [false, 1, 0]);
    }

    for (const [label, dispose] of [
        ['detached editor', ed => { ed.root.isConnected = false; }],
        ['aborted export', ed => { ed.packExportControl.abort(); }]
    ]) {
        const { pack: exportPack, state } = loadPackHarness();
        let resolveFetch;
        state.fetchHandler = () => new Promise(resolve => { resolveFetch = resolve; });
        const ed = exportEditorFixture(state);
        const pending = exportPack.exportEditor(ed);
        await waitForRequest(state);
        dispose(ed);
        resolveFetch(responseFor(exportDoc));
        const result = await pending;
        eq('export does not download after ' + label, [result, state.requests.length, state.downloads.length], [false, 1, 0]);
    }

    {
        const { pack: exportPack, state } = loadPackHarness();
        state.fetchHandler = async () => responseFor(exportDoc);
        const ed = exportEditorFixture(state);
        const result = await exportPack.exportEditor(ed);
        eq('export flushes before one GET and downloads only the matching source revision', [result, state.events, state.requests.length,
            state.requests[0] && state.requests[0].url, state.downloads, ed.packExportControl],
            [true, ['flushPendingEdits', 'flush', 'GET'], 1, '/api/desktop/flows/flow-source/pack', ['Bundle.easydrag-pack.json'], null]);
    }
    {
        const { pack: exportPack, state } = loadPackHarness();
        state.fetchHandler = async () => responseFor({ ...exportDoc, parent: { ...exportDoc.parent, source_revision: 8 } });
        const ed = exportEditorFixture(state);
        const result = await exportPack.exportEditor(ed);
        eq('export rejects a pack from a different source revision', [result, state.events, state.requests.length, state.downloads.length],
            [false, ['flushPendingEdits', 'flush', 'GET'], 1, 0]);
    }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) await run();
