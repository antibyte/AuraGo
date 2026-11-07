(function () {
    'use strict';

    function create(options) {
        const { t, element, button, dialog } = options;
        let disposed = false, fileInput = null, fileGeneration = 0;
        const urls = new Set();

        function exportState() {
            const clean = window.GoldfishState.validate(options.getState());
            const blob = new Blob([JSON.stringify(clean)], { type: 'application/json' });
            const url = URL.createObjectURL(blob);
            urls.add(url);
            const link = document.createElement('a');
            link.href = url;
            link.download = 'goldfish-adventure-' + new Date().toISOString().slice(0, 10) + '.json';
            document.body.append(link);
            link.click();
            link.remove();
        }

        async function importState() {
            const ticket = ++fileGeneration;
            fileInput?.remove();
            fileInput = document.createElement('input');
            fileInput.type = 'file';
            fileInput.accept = '.json,application/json';
            fileInput.hidden = true;
            options.root.append(fileInput);
            fileInput.addEventListener('change', async () => {
                const file = fileInput.files?.[0];
                if (!file) return;
                try {
                    if (file.size > window.GoldfishState.MAX_BYTES) throw new Error('oversized');
                    const bytes = await file.arrayBuffer();
                    if (disposed || ticket !== fileGeneration) return;
                    const text = new TextDecoder('utf-8', { fatal: true }).decode(bytes);
                    const clean = window.GoldfishState.validate(JSON.parse(text));
                    if (!await options.confirm(t('goldfish.ui.load_title'), t('goldfish.ui.load_body'))) return;
                    if (disposed || ticket !== fileGeneration) return;
                    if (await options.adopt(clean, true) === false) return;
                    options.status('import_ready', true);
                } catch (_) {
                    if (!disposed && ticket === fileGeneration) options.status('import_failed', true);
                }
            }, { once: true });
            fileInput.click();
        }

        function slotName(id) {
            if (id.startsWith('manual-')) return t('goldfish.ui.manual_slot', { number: id.slice(-1) });
            if (id.startsWith('chapter-')) return t('goldfish.ui.chapter_slot', { act: id.slice(-1) });
            const keys = { auto: 'auto_slot', 'branch-customs': 'branch_customs', 'branch-garden': 'branch_garden',
                'branch-hall': 'branch_hall', prefinal: 'prefinal_slot' };
            return t('goldfish.ui.' + keys[id]);
        }

        async function saves() {
            const pane = dialog(t('goldfish.ui.saves'));
            pane.body.append(element('p', 'agfish-small', t('goldfish.ui.storage_notice')));
            const transfers = element('div', 'agfish-dialog-actions');
            transfers.append(button(t('goldfish.ui.export'), exportState), button(t('goldfish.ui.import'), () => { pane.close(); importState(); }));
            pane.body.append(transfers);
            const list = element('div', 'agfish-slot-list');
            pane.body.append(list);
            try {
                const store = options.getStore();
                if (!store) throw new Error('storage');
                const slots = await store.list();
                if (disposed || !pane.node.isConnected) return;
                options.adoptRevisions(slots);
                for (const slot of slots) {
                    const row = element('article', 'agfish-slot');
                    const label = element('div', 'agfish-slot-label');
                    label.append(element('strong', '', slotName(slot.id)));
                    const room = window.GoldfishContent.rooms.find(value => value.id === slot.room);
                    label.append(element('span', 'agfish-small', slot.empty ? t('goldfish.ui.empty_slot') :
                        room ? t(room.title) + ' · ' + new Intl.DateTimeFormat(options.locale, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(slot.updatedAt)) : t('goldfish.ui.unknown_save')));
                    row.append(label);
                    const actions = element('div', 'agfish-dialog-actions');
                    if (slot.id.startsWith('manual-')) {
                        const save = button(t('goldfish.ui.save'), async () => {
                            if (!slot.empty && !await options.confirm(t('goldfish.ui.replace_title'), t('goldfish.ui.replace_body'))) return;
                            save.disabled = true;
                            const ok = await options.persist(slot.id, { expectedRevision: slot.revision });
                            if (disposed || !pane.node.isConnected) return;
                            save.disabled = false;
                            if (ok) { pane.close(); saves(); }
                        });
                        save.disabled = !slot.empty && !slot.valid;
                        actions.append(save);
                    }
                    if (!slot.empty) {
                        const load = async prior => {
                            if (!await options.confirm(t('goldfish.ui.load_title'), t('goldfish.ui.load_body'))) return;
                            try {
                                await options.flushSaves();
                                const saved = await store.read(slot.id, { prior });
                                if (disposed || !pane.node.isConnected || !saved) return;
                                pane.close();
                                if (await options.adopt(saved.state, true, saved) === false) return;
                                const currentAuto = saved.id === 'auto' && saved.prior === false;
                                options.status(currentAuto ? 'saved' : 'replay_ready', !currentAuto);
                            } catch (_) { options.status('unknown_save', true); }
                        };
                        const loadButton = button(t(slot.id.startsWith('branch-') || slot.id.startsWith('chapter-') || slot.id === 'prefinal' ? 'goldfish.ui.replay' : 'goldfish.ui.load'), () => load(false));
                        loadButton.disabled = !slot.valid;
                        actions.append(loadButton);
                        if (slot.hasPrior) actions.append(button(t('goldfish.ui.recover'), () => load(true)));
                    }
                    row.append(actions);
                    list.append(row);
                }
            } catch (_) {
                if (!disposed && pane.node.isConnected) list.append(element('p', 'agfish-warning', t('goldfish.ui.storage_failed')));
            }
        }

        // Puzzle prompts are conversation menus in the bar. The menu only collects answers; the state decides.
        function challenge(record, complete) {
            const spec = record.challenge, panel = options.panel;
            const heading = t(spec.prompt);
            const cancel = () => panel.closeMenu();
            const finish = answer => { panel.closeMenu(); complete(answer); };
            if (spec.type === 'choice') {
                panel.menu({
                    title: heading, onCancel: cancel,
                    lines: spec.options.map(option => ({ id: option.id, option: option.id, label: t(option.label) })),
                    onPick: line => finish(line.id)
                });
                return;
            }
            const answer = [];
            if (spec.type === 'sequence') {
                const label = id => t(spec.options.find(option => option.id === id).label);
                const show = () => panel.menu({
                    title: heading, onCancel: cancel,
                    sentence: answer.length ? answer.map(label).join(' → ') + ' → …' : '',
                    // Authored order lists the solution first; offer the steps reversed.
                    lines: [...spec.options].reverse().filter(option => !answer.includes(option.id))
                        .map(option => ({ id: option.id, option: option.id, label: t(option.label) })),
                    onBack: answer.length ? () => { answer.pop(); show(); } : null,
                    onPick: line => {
                        answer.push(line.id);
                        if (answer.length >= spec.answer.length) finish(answer.slice()); else show();
                    }
                });
                show();
                return;
            }
            const pairs = spec.pairs || [];
            const choices = spec.options?.length ? spec.options : pairs.map(value => ({ id: value.id, label: value.right }));
            const label = id => t(choices.find(option => option.id === id).label);
            const show = () => panel.menu({
                title: heading, onCancel: cancel,
                subtitle: t('goldfish.ui.match_pick', { left: t(pairs[answer.length].left) }),
                sentence: answer.map((id, index) => t(pairs[index].left) + ' → ' + label(id)).join(' · '),
                lines: [...choices].reverse().map(option => ({ id: option.id, option: option.id, label: t(option.label) })),
                onBack: answer.length ? () => { answer.pop(); show(); } : null,
                onPick: line => {
                    answer.push(line.id);
                    if (answer.length >= pairs.length) finish(answer.slice()); else show();
                }
            });
            show();
        }

        return { saves, challenge, exportState, importState,
            dispose: () => { disposed = true; fileGeneration++; fileInput?.remove(); for (const url of urls) URL.revokeObjectURL(url); urls.clear(); } };
    }

    window.GoldfishInteractions = Object.freeze({ create });
}());
