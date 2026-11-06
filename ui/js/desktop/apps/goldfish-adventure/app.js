(function () {
    'use strict';

    const instances = new Map();
    const localeIDs = new Set(['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']);
    const element = (tag, className = '', text) => {
        const value = document.createElement(tag);
        if (className) value.className = className;
        if (text !== undefined) value.textContent = text;
        return value;
    };
    const button = (text, callback, className = '') => {
        const value = element('button', 'agfish-button ' + className, text);
        value.type = 'button';
        value.addEventListener('click', callback);
        return value;
    };

    function render(host, windowId, ctx) {
        dispose(windowId);
        let disposed = false, generation = 0, controller = new AbortController();
        let state, store = null, renderer = null, interactions = null, texts = {};
        let started = false, userPaused = false, authEnded = false, busy = false, dirty = false;
        let storageBlocked = false, memoryConsent = false, assetStatus = 'loading_art';
        let selected = null, selectedItem = null, lineQueue = [], lineIndex = -1, allSeen = false, resultDialogue = false;
        let panelCount = 0, visibilityObserver = null, stageObserver = null, stageFrame = 0, currentSave = Promise.resolve(true), stateRevision = 0;
        const dialogs = new Set(), revisions = new Map(), hintLevels = new Map();
        const win = host.closest('.vd-window');
        const rawLocale = String(ctx.lang || window.SYSTEM_LANG || document.documentElement.lang || 'de').slice(0, 2).toLowerCase();
        const locale = localeIDs.has(rawLocale) ? rawLocale : 'de';
        const root = element('section', 'agfish-app');
        root.setAttribute('aria-label', ctx.t('desktop.app_goldfish_adventure'));
        const initial = element('div', 'agfish-boot');
        initial.append(element('span', 'agfish-orbit', '◌'), element('p', '', ctx.t('desktop.app_goldfish_adventure')));
        root.append(initial);
        host.replaceChildren(root);

        const t = (key, parameters = {}) => {
            const value = texts[key] || key;
            return value.replace(/\{([a-zA-Z_]+)\}/g, (match, name) => Object.hasOwn(parameters, name) ? String(parameters[name]) : match);
        };
        let refs = {};

        function changed() { stateRevision++; dirty = true; }

        async function flushSaves() {
            let pending;
            do { pending = currentSave; await pending; } while (pending !== currentSave);
        }

        function visible() {
            const active = typeof ctx.isActive !== 'function' || ctx.isActive();
            return !disposed && !authEnded && active && !document.hidden && (!win ||
                (!win.hidden && !win.classList.contains('vd-space-hidden') && win.getClientRects().length > 0 && getComputedStyle(win).display !== 'none'));
        }

        function updatePause() {
            const paused = !started || userPaused || panelCount > 0 || !visible();
            renderer?.setPaused(paused);
            root.classList.toggle('agfish-is-paused', userPaused || authEnded);
            if (refs.pause) refs.pause.textContent = t(userPaused ? 'goldfish.ui.resume' : 'goldfish.ui.pause');
            if (refs.pauseOverlay) refs.pauseOverlay.hidden = !(userPaused || authEnded);
            if (refs.pauseText) refs.pauseText.textContent = t(authEnded ? 'goldfish.ui.auth_ended' : 'goldfish.ui.paused');
        }

        function dialog(title) {
            const node = element('dialog', 'agfish-dialog');
            const heading = element('header', 'agfish-dialog-heading');
            heading.append(element('h2', '', title));
            const body = element('div', 'agfish-dialog-body');
            const close = () => { if (node.open) node.close(); else node.remove(); };
            heading.append(button('×', close, 'agfish-close'));
            heading.lastChild.setAttribute('aria-label', t('goldfish.ui.close'));
            node.append(heading, body);
            root.append(node);
            panelCount++;
            dialogs.add(node);
            let closed = false;
            const tidy = () => {
                if (closed) return;
                closed = true;
                dialogs.delete(node);
                node.remove();
                panelCount--;
                updatePause();
            };
            node.addEventListener('close', tidy, { once: true });
            node.showModal();
            updatePause();
            return { node, body, close };
        }

        function confirm(title, message) {
            return new Promise(resolve => {
                const pane = dialog(title);
                pane.body.append(element('p', '', message));
                const actions = element('div', 'agfish-dialog-actions');
                let accepted = false;
                actions.append(button(t('goldfish.ui.cancel'), pane.close), button(t('goldfish.ui.continue'), () => {
                    accepted = true; pane.close();
                }, 'agfish-primary'));
                pane.body.append(actions);
                pane.node.addEventListener('close', () => resolve(accepted), { once: true });
            });
        }

        function status(key, warning = false) {
            if (disposed || !refs.status) return;
            refs.status.textContent = t('goldfish.ui.' + key);
            refs.status.classList.toggle('agfish-warning', warning);
            refs.export.hidden = !warning;
        }

        async function persist(slot = 'auto', options = {}) {
            if (disposed) return false;
            if (!store || (slot === 'auto' && storageBlocked)) { status('memory_only', true); return false; }
            const snapshot = window.GoldfishState.validate(state);
            const snapshotRevision = stateRevision;
            status('saving');
            currentSave = currentSave.then(async () => {
                if (slot === 'auto' && storageBlocked) return false;
                try {
                    const revision = options.expectedRevision ?? revisions.get(slot) ?? 0;
                    const saved = await store.write(slot, snapshot, { expectedRevision: revision });
                    revisions.set(slot, saved.revision);
                    if (!disposed && snapshotRevision === stateRevision) { dirty = false; status('saved'); }
                    else if (!disposed && dirty) status('unsaved', true);
                    return true;
                } catch (reason) {
                    if (slot === 'auto') storageBlocked = true;
                    if (!disposed) status(reason.code === 'conflict' ? 'save_conflict' : reason.code === 'quota' ? 'save_quota' : 'save_failed', true);
                    return false;
                }
            });
            return currentSave;
        }

        async function allowMemory() {
            if (memoryConsent || disposed) return !disposed;
            return new Promise(resolve => {
                const pane = dialog(t('goldfish.ui.unsaved'));
                pane.body.append(element('p', 'agfish-warning', t('goldfish.ui.checkpoint_failed')));
                const actions = element('div', 'agfish-dialog-actions');
                actions.append(button(t('goldfish.ui.export'), () => interactions.exportState()),
                    button(t('goldfish.ui.continue_unsaved'), () => { memoryConsent = true; pane.close(); }, 'agfish-primary'));
                pane.body.append(actions);
                pane.node.addEventListener('close', () => resolve(memoryConsent), { once: true });
            });
        }

        async function checkpoint(slot, durable = false) {
            return await persist(slot) || (durable ? false : await allowMemory());
        }

        function room() { return window.GoldfishContent.rooms.find(value => value.id === state.room); }

        function refresh() {
            if (disposed || !state) return;
            root.dataset.room = state.room;
            refs.act.textContent = t('goldfish.ui.chapter', { act: room().act });
            refs.room.textContent = t(room().title);
            refs.canvas.setAttribute('aria-label', t('goldfish.ui.scene_description', { room: t(room().title) }));
            root.classList.toggle('agfish-show-hotspots', state.settings.hotspots);
            root.classList.toggle('agfish-busy', busy);
            if (refs.settings) refs.settings.disabled = busy;
            refs.inventory.replaceChildren();
            if (!state.inventory.length) refs.inventory.append(element('p', 'agfish-small', t('goldfish.ui.inventory_empty')));
            for (const id of state.inventory) {
                const item = window.GoldfishContent.items.find(value => value.id === id);
                const pick = button('', () => {
                    if (busy || !started) return;
                    selectedItem = selectedItem === id ? null : id;
                    say([{ speaker: 'r0st', text: item.description }]);
                    refresh();
                }, 'agfish-item');
                pick.dataset.agfishItem = id;
                pick.setAttribute('aria-pressed', String(selectedItem === id));
                pick.setAttribute('title', t(item.description));
                if (item.icon) {
                    const image = document.createElement('img'); image.src = item.icon; image.alt = ''; image.loading = 'lazy';
                    image.addEventListener('error', () => image.remove(), { once: true });
                    pick.append(image);
                }
                pick.append(element('span', '', t(item.name)));
                refs.inventory.append(pick);
            }
            if (selectedItem && !state.inventory.includes(selectedItem)) selectedItem = null;
            refs.selectedItem.hidden = !selectedItem;
            refs.selectedItem.textContent = selectedItem ? t('goldfish.ui.selected_item', {
                item: t(window.GoldfishContent.items.find(value => value.id === selectedItem).name)
            }) : '';
            renderContext();
            renderer?.update(state);
        }

        function renderContext() {
            refs.context.replaceChildren();
            if (!selected) {
                refs.context.append(element('p', 'agfish-small', t('goldfish.ui.select_object')));
                return;
            }
            const hotspot = room().hotspots.find(value => value.id === selected);
            if (!hotspot) { selected = null; return; }
            refs.context.append(element('h3', '', t(hotspot.label)));
            refs.context.append(button(t('goldfish.ui.inspect'), () => say(window.GoldfishState.lines(state, hotspot.look))));
            const actions = hotspot.actions.map(id => window.GoldfishContent.actions.find(value => value.id === id))
                .filter(value => window.GoldfishState.available(state, value));
            for (const record of actions) {
                const actionButton = button(t(record.label), () => chooseAction(record));
                actionButton.dataset.agfishAction = record.id;
                actionButton.disabled = busy;
                if (record.item && selectedItem !== record.item) actionButton.title = t('goldfish.ui.item_needed');
                refs.context.append(actionButton);
            }
            if (!actions.length) refs.context.append(element('p', 'agfish-small', t('goldfish.ui.no_action')));
        }

        function renderHotspots() {
            refs.hotspots.replaceChildren();
            for (const hotspot of room().hotspots) {
                if (!window.GoldfishState.conditions(state, hotspot)) continue;
                const pick = button(t(hotspot.label), async () => {
                    if (busy || !started || userPaused || authEnded) return;
                    selected = hotspot.id;
                    renderContext();
                    say(window.GoldfishState.lines(state, hotspot.look));
                    await renderer.walk(hotspot.walk || [hotspot.rect[0] + hotspot.rect[2] / 2, 0.83]);
                }, 'agfish-hotspot');
                pick.dataset.agfishHotspot = hotspot.id;
                const [x, y, width, height] = hotspot.rect;
                Object.assign(pick.style, { left: x * 100 + '%', top: y * 100 + '%', width: width * 100 + '%', height: height * 100 + '%' });
                refs.hotspots.append(pick);
            }
            for (const exit of room().exits) {
                const pick = button('↝ ' + t(exit.label), () => travel(exit.to), 'agfish-exit');
                pick.dataset.agfishExit = exit.to;
                if (!window.GoldfishState.conditions(state, exit)) pick.classList.add('agfish-locked-exit');
                const [x, y, width, height] = exit.rect;
                Object.assign(pick.style, { left: x * 100 + '%', top: y * 100 + '%', width: width * 100 + '%', minHeight: height * 100 + '%' });
                refs.hotspots.append(pick);
            }
        }

        function say(lines, result = false) {
            if (disposed) return;
            lineQueue = lines || [];
            lineIndex = -1;
            resultDialogue = result;
            allSeen = lineQueue.length > 0 && lineQueue.every(line => state.seenLines.includes(line.text));
            nextLine();
        }

        function nextLine() {
            if (disposed) return;
            const acknowledged = lineQueue[lineIndex];
            if (acknowledged && !state.seenLines.includes(acknowledged.text)) {
                state.seenLines.push(acknowledged.text);
                if (resultDialogue) { changed(); persist(); }
            }
            lineIndex++;
            const line = lineQueue[lineIndex];
            if (!line) {
                refs.dialogue.hidden = true;
                lineQueue = [];
                renderer?.speak(null);
                return;
            }
            refs.dialogue.hidden = false;
            refs.speaker.textContent = t('goldfish.ui.speaker_' + line.speaker);
            refs.dialogue.dataset.speaker = line.speaker;
            refs.line.textContent = t(line.text);
            refs.skip.hidden = !allSeen;
            refs.next.textContent = t(lineIndex === lineQueue.length - 1 ? 'goldfish.ui.close' : 'goldfish.ui.dialogue_next');
            renderer?.speak(line.speaker);
        }

        function resumeDialogue() {
            const pending = window.GoldfishState.pendingResults(state);
            const entry = window.GoldfishState.enter(state).filter(line => !state.seenLines.includes(line.text));
            say(pending.concat(entry), pending.length > 0);
        }

        async function chooseAction(record) {
            if (busy || !started || userPaused || authEnded) return;
            if (record.item && selectedItem !== record.item) { status('item_needed', true); return; }
            if (record.challenge) interactions.challenge(record, answer => perform(record, answer));
            else perform(record);
        }

        async function perform(record, answer) {
            if (busy || disposed || authEnded || !visible()) return;
            busy = true;
            refresh();
            const ticket = generation;
            try {
                const result = window.GoldfishState.apply(state, record.id, { item: selectedItem, answer });
                if (result.error) {
                    status(result.error, true);
                    if (result.lines.length) say(result.lines);
                    return;
                }
                const checkpointID = { 'pre-customs': 'branch-customs', 'pre-garden': 'branch-garden', 'pre-hall': 'branch-hall' }[record.checkpoint];
                if (checkpointID && !await checkpoint(checkpointID)) return;
                if (record.effects?.ending && !await checkpoint('prefinal')) return;
                if (disposed || generation !== ticket) return;
                const previous = state, wasDirty = dirty;
                state = result.state;
                if (result.changed) {
                    changed();
                    if (!await persist() && !await allowMemory()) {
                        state = previous; stateRevision++; dirty = wasDirty;
                        renderHotspots();
                        return;
                    }
                }
                if (disposed || generation !== ticket) return;
                refresh();
                renderHotspots();
                refs.dialogue.hidden = true;
                lineQueue = [];
                renderer.speak(null);
                await renderer.act(record.id, record.animation || (record.effects?.puzzle ? 'celebrate' : record.verb === 'talk' ? 'talk' : 'use'));
                if (disposed || generation !== ticket) return;
                say(result.lines, true);
            } finally {
                busy = false;
                if (!disposed) refresh();
            }
        }

        async function travel(target) {
            if (busy || !started || userPaused || authEnded) return;
            const previousAct = room().act;
            const result = window.GoldfishState.move(state, target);
            if (result.error) { say(result.lines); return; }
            const previous = state, wasDirty = dirty;
            busy = true;
            refresh();
            renderer.cancelMotion();
            generation++;
            state = result.state;
            changed();
            try {
                const durable = await persist();
                if (!durable && !await allowMemory()) {
                    state = previous; stateRevision++; dirty = wasDirty;
                    renderHotspots();
                    return;
                }
                if (disposed) return;
                selected = null;
                lineQueue = [];
                refresh();
                const firstActEntry = room().act !== previousAct && !previous.visited.some(id =>
                    window.GoldfishContent.rooms.find(value => value.id === id).act === room().act);
                if (firstActEntry) await checkpoint('chapter-' + room().act, durable);
                if (await loadRoom()) say(result.lines);
            } finally { busy = false; if (!disposed) refresh(); }
        }

        async function loadRoom() {
            const ticket = ++generation;
            renderHotspots();
            try { await renderer.load(state.room, state); }
            catch (_) { if (!disposed && ticket === generation) artStatus('art_failed'); }
            if (!disposed && ticket === generation) { refresh(); updatePause(); }
            return !disposed && ticket === generation;
        }

        function artStatus(key) {
            assetStatus = key;
            if (!refs.artStatus || disposed) return;
            refs.artStatus.hidden = key === 'ready';
            refs.artText.textContent = key === 'ready' ? '' : t('goldfish.ui.' + key);
            refs.artRetry.hidden = key !== 'art_failed';
            root.classList.toggle('agfish-art-loading', key === 'loading_art');
        }

        async function adopt(next, imported = false, source = null) {
            const clean = window.GoldfishState.validate(next);
            await flushSaves();
            if (disposed) return false;
            generation++;
            state = clean;
            started = true;
            busy = false;
            dirty = imported;
            stateRevision++;
            memoryConsent = false;
            if (source && window.GoldfishSave.SLOTS.includes(source.id) && Number.isSafeInteger(source.revision) && source.revision > 0) {
                if (source.id !== 'auto' || source.prior === false) revisions.set(source.id, source.revision);
                if (source.id === 'auto' && source.prior === false) { storageBlocked = false; dirty = false; }
            }
            selected = selectedItem = null;
            lineQueue = [];
            refs.start.hidden = true;
            refs.dialogue.hidden = true;
            renderer.cancelMotion();
            if (!await loadRoom()) return false;
            refresh();
            updatePause();
            resumeDialogue();
            return true;
        }

        function journal() {
            const pane = dialog(t('goldfish.ui.journal'));
            pane.body.append(element('p', 'agfish-small', t('goldfish.ui.playtime', { minutes: Math.floor(state.elapsedMs / 60000) })));
            if (!state.journal.length) pane.body.append(element('p', '', t('goldfish.ui.journal_empty')));
            else {
                const entries = element('ol', 'agfish-journal');
                for (const key of state.journal) entries.append(element('li', '', t(key)));
                pane.body.append(entries);
            }
            if (state.ending) pane.body.append(element('p', '', t('goldfish.ui.ending_body')));
        }

        function hint() {
            const puzzles = window.GoldfishContent.puzzles.filter(value => value.room === state.room && window.GoldfishState.conditions(state, value));
            const pane = dialog(t(puzzles.length === 1 ? puzzles[0].title : 'goldfish.ui.hint'));
            if (!puzzles.length) { pane.body.append(element('p', '', t('goldfish.ui.hint_none'))); return; }
            const hints = element('div', 'agfish-hint-content');
            const show = puzzle => {
                hints.replaceChildren();
                if (!puzzle) return;
                let level = hintLevels.get(puzzle.id) || 0;
                const text = element('p', 'agfish-hint', t(puzzle.hints[level]));
                const more = button(t('goldfish.ui.hint_next'), () => {
                    level = Math.min(puzzle.hints.length - 1, level + 1);
                    hintLevels.set(puzzle.id, level);
                    text.textContent = t(puzzle.hints[level]);
                    more.disabled = level >= puzzle.hints.length - 1;
                });
                more.disabled = level >= puzzle.hints.length - 1;
                hints.append(text, more);
            };
            if (puzzles.length > 1) {
                const label = element('label', 'agfish-hint-picker', t('goldfish.ui.choose'));
                const select = element('select');
                const empty = element('option', '', t('goldfish.ui.choose')); empty.value = ''; select.append(empty);
                for (const puzzle of puzzles) {
                    const option = element('option', '', t(puzzle.title)); option.value = puzzle.id; select.append(option);
                }
                select.addEventListener('change', () => show(puzzles.find(value => value.id === select.value)));
                label.append(select); pane.body.append(label);
            } else show(puzzles[0]);
            pane.body.append(hints);
        }

        function settings() {
            const pane = dialog(t('goldfish.ui.settings'));
            pane.body.append(element('p', 'agfish-small', t('goldfish.ui.subtitles')));
            for (const [key, label] of [['sound', 'sound'], ['reducedMotion', 'reduced_motion'], ['hotspots', 'hotspots']]) {
                const wrapper = element('label', 'agfish-setting');
                const input = document.createElement('input'); input.type = 'checkbox'; input.checked = state.settings[key];
                input.addEventListener('change', () => {
                    state.settings[key] = input.checked;
                    changed();
                    renderer.unlock(); refresh();
                });
                wrapper.append(input, element('span', '', t('goldfish.ui.' + label)));
                pane.body.append(wrapper);
            }
            pane.node.addEventListener('close', () => { if (!disposed) persist(); }, { once: true });
        }

        function build() {
            root.replaceChildren();
            const toolbar = element('header', 'agfish-toolbar');
            const identity = element('div', 'agfish-identity');
            identity.append(element('span', 'agfish-maker', 'R0ST / 07'));
            refs.act = element('span', 'agfish-act'); refs.room = element('h1', 'agfish-room');
            const place = element('div', 'agfish-place'); place.append(refs.act, refs.room); identity.append(place);
            const menu = element('nav', 'agfish-menu');
            menu.setAttribute('aria-label', t('goldfish.ui.title'));
            refs.pause = button(t('goldfish.ui.pause'), () => { if (started) { userPaused = !userPaused; updatePause(); } });
            refs.settings = button(t('goldfish.ui.settings'), settings);
            menu.append(button(t('goldfish.ui.journal'), journal), button(t('goldfish.ui.hint'), hint),
                button(t('goldfish.ui.saves'), () => interactions.saves()), refs.settings, refs.pause);
            toolbar.append(identity, menu);
            const workspace = element('div', 'agfish-workspace');
            const stageColumn = element('div', 'agfish-stage-column');
            const scene = element('div', 'agfish-scene');
            const fitScene = () => {
                if (disposed) return;
                const availableHeight = Math.max(1, stageColumn.clientHeight - (refs.dialogue.hidden ? 0 : refs.dialogue.offsetHeight));
                const width = root.clientWidth <= 760 ? '100%' : Math.min(stageColumn.clientWidth, availableHeight * 16 / 9) + 'px';
                if (scene.style.width !== width) scene.style.width = width;
            };
            const scheduleFit = () => {
                if (disposed || stageFrame) return;
                stageFrame = requestAnimationFrame(() => { stageFrame = 0; fitScene(); });
            };
            refs.canvas = element('canvas', 'agfish-canvas'); refs.canvas.setAttribute('role', 'img');
            refs.canvas.width = 1920; refs.canvas.height = 1080;
            refs.hotspots = element('div', 'agfish-hotspots');
            refs.skipScene = button(t('goldfish.ui.skip_scene'), () => renderer.skipMoment(), 'agfish-skip-scene');
            refs.skipScene.hidden = true;
            refs.artStatus = element('div', 'agfish-art-status'); refs.artText = element('p');
            refs.artRetry = button(t('goldfish.ui.retry'), loadRoom); refs.artRetry.hidden = true;
            refs.artStatus.append(refs.artText, refs.artRetry);
            refs.pauseOverlay = element('div', 'agfish-pause-overlay'); refs.pauseOverlay.hidden = true;
            refs.pauseText = element('p');
            refs.pauseOverlay.append(refs.pauseText, button(t('goldfish.ui.resume'), () => { userPaused = false; updatePause(); }));
            refs.start = element('div', 'agfish-start');
            const startContent = element('div', 'agfish-start-content');
            startContent.append(element('span', 'agfish-eyebrow', 'AURAGO · R0ST · BRUNO · KARLA · KIESEL · KUNO'),
                element('h2', '', t('goldfish.ui.title')), element('p', '', t('goldfish.ui.subtitle')));
            refs.continue = button(t('goldfish.ui.continue'), async () => {
                started = true; refs.start.hidden = true; renderer.unlock(); updatePause();
                resumeDialogue();
            }, 'agfish-primary');
            refs.continue.hidden = true;
            const startNew = button(t('goldfish.ui.start'), async () => {
                if (busy || disposed) return;
                if (dirty && !await confirm(t('goldfish.ui.new_title'), t('goldfish.ui.new_body'))) return;
                const previous = { state, started, dirty, userPaused, memoryConsent, selected, selectedItem,
                    lineQueue, lineIndex, allSeen, resultDialogue, startHidden: refs.start.hidden,
                    dialogueHidden: refs.dialogue.hidden, status: refs.status.textContent,
                    warning: refs.status.classList.contains('agfish-warning'), exportHidden: refs.export.hidden };
                busy = true;
                startNew.disabled = true;
                try {
                    state = window.GoldfishState.create();
                    changed(); started = true; userPaused = false; memoryConsent = false;
                    selected = selectedItem = null; lineQueue = []; resultDialogue = false;
                    refs.start.hidden = true; refs.dialogue.hidden = true;
                    renderer.unlock();
                    if (!await loadRoom()) return;
                    const ticket = generation;
                    const accepted = await checkpoint('chapter-1');
                    if (disposed || generation !== ticket) return;
                    if (!accepted) {
                        state = previous.state; started = previous.started; dirty = previous.dirty;
                        userPaused = previous.userPaused; memoryConsent = previous.memoryConsent;
                        selected = previous.selected; selectedItem = previous.selectedItem;
                        lineQueue = previous.lineQueue; lineIndex = previous.lineIndex;
                        allSeen = previous.allSeen; resultDialogue = previous.resultDialogue;
                        refs.start.hidden = previous.startHidden; refs.dialogue.hidden = previous.dialogueHidden;
                        refs.status.textContent = previous.status;
                        refs.status.classList.toggle('agfish-warning', previous.warning);
                        refs.export.hidden = previous.exportHidden;
                        stateRevision++;
                        await loadRoom();
                        return;
                    }
                    say(window.GoldfishState.enter(state));
                } finally {
                    busy = false;
                    startNew.disabled = false;
                    if (!disposed) { refresh(); updatePause(); }
                }
            }, 'agfish-primary');
            const startActions = element('div', 'agfish-start-actions');
            startActions.append(refs.continue, startNew, button(t('goldfish.ui.saves'), () => interactions.saves()));
            startContent.append(startActions, element('small', '', t('goldfish.ui.pause_hint')));
            refs.start.append(startContent);
            scene.append(refs.canvas, refs.hotspots, refs.artStatus, refs.start, refs.pauseOverlay, refs.skipScene);
            scene.addEventListener('click', event => {
                renderer.unlock();
                if (event.target !== refs.hotspots || busy || !started || userPaused || authEnded) return;
                const bounds = scene.getBoundingClientRect();
                renderer.walk([(event.clientX - bounds.left) / bounds.width, 0.83]);
            });
            refs.dialogue = element('section', 'agfish-dialogue'); refs.dialogue.hidden = true;
            refs.dialogue.setAttribute('aria-live', 'polite'); refs.dialogue.setAttribute('aria-atomic', 'true');
            refs.speaker = element('strong', 'agfish-speaker'); refs.line = element('p', 'agfish-line');
            const lineContent = element('div', 'agfish-dialogue-content'); lineContent.append(refs.speaker, refs.line);
            const lineControls = element('div', 'agfish-dialogue-controls');
            refs.next = button(t('goldfish.ui.dialogue_next'), nextLine, 'agfish-primary');
            refs.skip = button(t('goldfish.ui.dialogue_skip'), () => { lineQueue = []; nextLine(); }); refs.skip.hidden = true;
            lineControls.append(refs.next, refs.skip);
            refs.dialogue.append(lineContent, lineControls);
            stageColumn.append(scene, refs.dialogue);
            const sidebar = element('aside', 'agfish-sidebar');
            refs.context = element('section', 'agfish-context');
            const inventorySection = element('section', 'agfish-inventory-section');
            inventorySection.append(element('h2', '', t('goldfish.ui.inventory')));
            refs.selectedItem = button('', () => { selectedItem = null; refresh(); }, 'agfish-selected-item');
            refs.selectedItem.setAttribute('aria-label', t('goldfish.ui.deselect'));
            refs.inventory = element('div', 'agfish-inventory');
            inventorySection.append(refs.selectedItem, refs.inventory);
            sidebar.append(refs.context, inventorySection);
            workspace.append(stageColumn, sidebar);
            const footer = element('footer', 'agfish-footer');
            refs.status = element('p', 'agfish-save-status'); refs.status.setAttribute('role', 'status');
            refs.export = button(t('goldfish.ui.export'), () => interactions.exportState()); refs.export.hidden = true;
            footer.append(refs.status, refs.export);
            root.append(toolbar, workspace, footer);
            stageObserver = new ResizeObserver(scheduleFit);
            stageObserver.observe(stageColumn);
            stageObserver.observe(refs.dialogue);
            scheduleFit();
            renderer = window.GoldfishRenderer.create(refs.canvas, {
                onStatus: artStatus,
                onMoment: active => {
                    refs.skipScene.hidden = !active;
                    root.classList.toggle('agfish-in-moment', active);
                },
                onPosition: position => { if (!disposed) state.position = position; },
                onTick: delta => { if (state && !disposed) state.elapsedMs += Math.round(delta); }
            });
            interactions = window.GoldfishInteractions.create({ root, t, element, button, dialog, confirm, locale,
                getState: () => state, getStore: () => store, persist, adopt, status,
                flushSaves,
                adoptRevisions: slots => { for (const slot of slots) if (!revisions.has(slot.id)) revisions.set(slot.id, slot.revision); }
            });
            refresh();
            status(store ? 'unsaved' : 'storage_failed', !store);
        }

        async function dictionary(section) {
            const response = await fetch('/lang/' + section + '/' + locale + '.json?v=' + encodeURIComponent(window.BUILD_VERSION || 'dev'),
                { signal: controller.signal, credentials: 'same-origin' });
            if (!response.ok) throw new Error('locale_failed');
            const source = await response.text();
            if (source.length > 2 * 1024 * 1024) throw new Error('locale_failed');
            const parsed = JSON.parse(source);
            if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('locale_failed');
            for (const [key, value] of Object.entries(parsed)) {
                if (!key.startsWith('goldfish.') || typeof value !== 'string' || value.length > 4096) throw new Error('locale_failed');
            }
            return parsed;
        }

        async function initialize() {
            const ticket = ++generation;
            try {
                const results = await Promise.allSettled([dictionary('goldfish'), dictionary('goldfish-interface'), window.GoldfishSave.open()]);
                if (disposed || ticket !== generation) { if (results[2].status === 'fulfilled') results[2].value.close(); return; }
                if (results[0].status !== 'fulfilled' || results[1].status !== 'fulfilled') {
                    if (results[2].status === 'fulfilled') results[2].value.close();
                    throw new Error('locale_failed');
                }
                texts = { ...results[0].value, ...results[1].value };
                window.GoldfishContent.texts = results[0].value;
                state = window.GoldfishState.create();
                if (results[2].status === 'fulfilled') {
                    store = results[2].value;
                    try { for (const slot of await store.list()) revisions.set(slot.id, slot.revision); }
                    catch (_) { storageBlocked = true; }
                }
                if (disposed) { store?.close(); return; }
                build();
                if (store) {
                    try {
                        const saved = await store.read('auto');
                        if (disposed) return;
                        if (saved) {
                            state = saved.state;
                            revisions.set('auto', saved.revision);
                            refs.continue.hidden = false;
                            refresh();
                        }
                    } catch (_) { status('unknown_save', true); storageBlocked = true; }
                }
                await loadRoom();
            } catch (_) {
                if (disposed) return;
                root.replaceChildren();
                const failure = element('div', 'agfish-boot');
                failure.append(element('p', '', ctx.t('desktop.load_failed')), button(ctx.t('desktop.retry'), () => {
                    controller.abort(); controller = new AbortController(); initialize();
                }));
                root.append(failure);
            }
        }

        const keydown = event => {
            if (!root.contains(document.activeElement) && !win?.classList.contains('vd-window-active')) return;
            if (panelCount || /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName)) return;
            if (event.code === 'Space' && started && event.target.tagName !== 'BUTTON') {
                event.preventDefault(); userPaused = !userPaused; updatePause();
            } else if (event.key === 'Escape') {
                if (renderer?.skipMoment()) { event.preventDefault(); return; }
                selectedItem = selected = null; refs.dialogue.hidden = true; lineQueue = []; if (state) refresh();
            }
        };
        root.tabIndex = -1;
        root.addEventListener('keydown', keydown);
        const auth = () => { authEnded = true; updatePause(); };
        window.addEventListener('aurago:auth-ended', auth);
        document.addEventListener('visibilitychange', updatePause);
        if (win) {
            visibilityObserver = new MutationObserver(updatePause);
            visibilityObserver.observe(win, { attributes: true, attributeFilter: ['style', 'class', 'hidden'] });
        }
        ctx.setWindowBeforeClose?.(windowId, async () => {
            await flushSaves();
            if (disposed || !started || !dirty) return true;
            const pane = dialog(t('goldfish.ui.unsaved'));
            pane.body.append(element('p', '', t('goldfish.ui.storage_failed')));
            const actions = element('div', 'agfish-dialog-actions');
            let accepted = false;
            actions.append(button(t('goldfish.ui.export'), () => interactions.exportState()), button(t('goldfish.ui.close'), () => { accepted = true; pane.close(); }));
            pane.body.append(actions);
            return new Promise(resolve => pane.node.addEventListener('close', () => resolve(accepted), { once: true }));
        });
        const stop = () => {
            disposed = true;
            generation++;
            controller.abort();
            root.removeEventListener('keydown', keydown);
            window.removeEventListener('aurago:auth-ended', auth);
            document.removeEventListener('visibilitychange', updatePause);
            visibilityObserver?.disconnect();
            stageObserver?.disconnect();
            if (stageFrame) cancelAnimationFrame(stageFrame);
            stageFrame = 0;
            for (const node of dialogs) node.close();
            dialogs.clear();
            interactions?.dispose();
            renderer?.dispose();
            currentSave.finally(() => store?.close());
            ctx.setWindowBeforeClose?.(windowId, null);
            root.remove();
        };
        instances.set(windowId, stop);
        initialize();
    }

    function dispose(windowId) {
        instances.get(windowId)?.();
        instances.delete(windowId);
    }

    window.GoldfishAdventureApp = Object.freeze({ render, dispose });
}());
