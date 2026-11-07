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
        let state, store = null, renderer = null, interactions = null, speech = null, panel = null, texts = {};
        let started = false, userPaused = false, authEnded = false, busy = false, dirty = false;
        let storageBlocked = false, memoryConsent = false, assetStatus = 'loading_art';
        let queueResult = false, flow = null, menuLive = null, saveStatus = { text: '', warning: false };
        let panelCount = 0, visibilityObserver = null, stageObserver = null, stageFrame = 0, currentSave = Promise.resolve(true), stateRevision = 0;
        const dialogs = new Set(), revisions = new Map(), hintLevels = new Map(), refusals = { take: 0, use: 0, talk: 0, item: 0 };
        const win = host.closest('.vd-window');
        const rawLocale = String(ctx.lang || window.SYSTEM_LANG || document.documentElement.lang || 'de').slice(0, 2).toLowerCase();
        const locale = localeIDs.has(rawLocale) ? rawLocale : 'de';
        const root = element('section', 'agfish-app');
        root.setAttribute('aria-label', ctx.t('desktop.app_goldfish_adventure'));
        root.setAttribute('lang', locale);
        const initial = element('div', 'agfish-boot');
        initial.append(element('span', 'agfish-orbit', '◌'), element('p', '', ctx.t('desktop.app_goldfish_adventure')));
        root.append(initial);
        host.replaceChildren(root);

        const t = (key, parameters = {}) => {
            const value = texts[key] || key;
            return value.replace(/\{([a-zA-Z_]+)\}/g, (match, name) => Object.hasOwn(parameters, name) ? String(parameters[name]) : match);
        };
        const commands = window.GoldfishCommands;
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
            speech?.setPaused(paused);
            panel?.setLocked(!started);
            if (refs.hotspots) refs.hotspots.inert = !started;
            root.classList.toggle('agfish-is-paused', userPaused || authEnded);
            root.classList.toggle('agfish-started', started);
            if (refs.pauseOverlay) refs.pauseOverlay.hidden = !(userPaused || authEnded);
            if (refs.pauseText) refs.pauseText.textContent = t(authEnded ? 'goldfish.ui.auth_ended' : 'goldfish.ui.paused');
        }

        // Keys reach the game while focus is inside it; give focus back after the start screen or a dialog goes away.
        function reclaimFocus() {
            const active = document.activeElement;
            if (disposed || !started || (active && active !== document.body && root.contains(active))) return;
            if (!active || active === document.body) root.focus({ preventScroll: true });
        }

        function dialog(title) {
            const node = element('dialog', 'agfish-dialog');
            const heading = element('header', 'agfish-dialog-heading');
            heading.append(element('h2', '', title));
            const body = element('div', 'agfish-dialog-body');
            let closed = false;
            // The DOM and the pause counter change together, whether closed by button, Esc or code.
            const tidy = () => {
                if (closed) return;
                closed = true;
                dialogs.delete(node);
                node.remove();
                panelCount--;
                updatePause();
                reclaimFocus();
            };
            const close = () => { if (node.open) node.close(); tidy(); };
            heading.append(button('×', close, 'agfish-close'));
            heading.lastChild.setAttribute('aria-label', t('goldfish.ui.close'));
            node.append(heading, body);
            root.append(node);
            panelCount++;
            dialogs.add(node);
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

        function syncMenuStatus() {
            if (!menuLive) return;
            menuLive.line.textContent = saveStatus.text;
            menuLive.line.classList.toggle('agfish-warning', saveStatus.warning);
            menuLive.export.hidden = !saveStatus.warning;
            menuLive.settings.disabled = busy;
        }

        function applyStatus(text, warning) {
            saveStatus = { text, warning };
            if (disposed || !refs.status) return;
            refs.status.textContent = text;
            refs.status.classList.toggle('agfish-warning', warning);
            panel?.menuButton.classList.toggle('agfish-warning', warning);
            syncMenuStatus();
        }

        function status(key, warning = false) { applyStatus(t('goldfish.ui.' + key), warning); }

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
        const itemRecord = id => window.GoldfishContent.items.find(value => value.id === id);
        const blocked = () => busy || !started || userPaused || authEnded || !!speech?.story();

        function refresh() {
            if (disposed || !state) return;
            root.dataset.room = state.room;
            refs.canvas.setAttribute('aria-label', t('goldfish.ui.scene_description', { room: t(room().title) }));
            root.classList.toggle('agfish-show-hotspots', state.settings.hotspots);
            root.classList.toggle('agfish-calm', state.settings.reducedMotion);
            root.classList.toggle('agfish-busy', busy);
            panel.setIdle(t(room().title));
            panel.render(state);
            syncMenuStatus();
            renderer?.update(state);
        }

        // ---- Speech -------------------------------------------------------------------------------------------

        // Lines play above the speaker's head. Story queues block the scene; ambient ones yield to any click.
        function say(lines, { story = false, result = false } = {}) {
            if (disposed || !speech) return;
            const list = lines || [];
            const allSeen = list.length > 0 && list.every(line => state.seenLines.includes(line.text));
            speech.play(list, { story, allSeen });
            queueResult = result;
        }

        const sayUi = (key, speaker = 'r0st') => say([{ speaker, text: 'goldfish.ui.' + key }]);

        function refuse(kind) {
            refusals[kind]++;
            say([{ speaker: 'r0st', text: commands.refusalKey(kind, refusals[kind]) }]);
        }

        // A line counts as read once shown long enough or skipped. Interface refusals are never recorded.
        function lineRead(line) {
            if (disposed || line.text.startsWith('goldfish.ui.') || state.seenLines.includes(line.text)) return;
            state.seenLines.push(line.text);
            if (queueResult) { changed(); persist(); }
        }

        function speechLine(line) {
            if (disposed || !line) return;
            renderer?.speak(line.speaker);
            panel?.setDim(speech.story());
        }

        function speechEnded() {
            if (disposed) return;
            renderer?.speak(null);
            panel?.setDim(false);
        }

        function resumeDialogue() {
            const pending = window.GoldfishState.pendingResults(state);
            const entry = window.GoldfishState.enter(state).filter(line => !state.seenLines.includes(line.text));
            say(pending.concat(entry), { story: true, result: pending.length > 0 });
        }

        // ---- Hotspots, verbs and walking ----------------------------------------------------------------------

        function sentenceFor(hotspot) {
            const item = panel.armedItem(), verb = panel.armedVerb();
            let shown = verb;
            if (!item && !verb) {
                const choice = commands.defaultAction(state, hotspot);
                shown = choice.kind === 'one' ? commands.ownVerb(choice.action) : choice.kind === 'look' ? 'look' : 'walk';
            }
            return commands.sentence(t, { verb: shown, hotspot, item: item && itemRecord(item) });
        }

        // Walks to a target; a second click on the same target while walking finishes the walk at once.
        async function approach(key, point) {
            if (flow && flow.key === key && flow.walking) {
                flow.instant = true;
                renderer.walk(point, { instant: true });
                return false;
            }
            const mine = flow = { key, walking: true, instant: false };
            const arrived = await renderer.walk(point);
            if (flow !== mine || disposed) return false;
            mine.walking = false;
            if (!arrived && !mine.instant) { flow = null; return false; }
            return true;
        }

        function lookAt(hotspot) {
            if (blocked()) return;
            panel.disarm();
            panel.closeMenu();
            say(window.GoldfishState.lines(state, hotspot.look));
        }

        function chooseAmong(hotspot, list, item) {
            if (list.length === 1) { chooseAction(list[0], item); return; }
            panel.menu({
                title: t(hotspot.label),
                lines: list.map(record => ({ id: record.id, action: record.id, label: t(record.label) })),
                onPick: line => { panel.closeMenu(); chooseAction(list.find(record => record.id === line.id), item); },
                onCancel: () => panel.closeMenu()
            });
        }

        function runAt(hotspot, verb, item) {
            if (blocked()) return;
            if (item) {
                const list = commands.actionsFor(state, hotspot, { item });
                if (list.length) chooseAmong(hotspot, list, item); else refuse('item');
            } else if (verb) {
                const list = commands.actionsFor(state, hotspot, { verb });
                if (list.length) chooseAmong(hotspot, list, null);
                else if (commands.needsItem(state, hotspot, verb).length) sayUi('item_needed');
                else refuse(verb);
            } else {
                const choice = commands.defaultAction(state, hotspot);
                if (choice.kind === 'one') chooseAction(choice.action, null);
                else if (choice.kind === 'many') chooseAmong(hotspot, choice.actions, null);
                else if (choice.kind === 'item') sayUi('item_needed');
                else say(window.GoldfishState.lines(state, hotspot.look));
            }
        }

        async function clickHotspot(hotspot) {
            if (blocked()) return;
            const verb = panel.armedVerb(), item = panel.armedItem();
            panel.disarm();
            panel.closeMenu();
            if (verb === 'look' && !item) { lookAt(hotspot); return; }
            speech.cancel();
            const point = hotspot.walk || [hotspot.rect[0] + hotspot.rect[2] / 2, 0.83];
            if (await approach('h:' + hotspot.id, point)) runAt(hotspot, verb, item);
        }

        function clickExit(exit) {
            if (blocked()) return;
            panel.disarm();
            panel.closeMenu();
            travel(exit.to);
        }

        function lookExit(exit) {
            if (blocked() || window.GoldfishState.conditions(state, exit)) return;
            panel.disarm();
            say(window.GoldfishState.lines(state, exit.blocked));
        }

        function renderHotspots() {
            // Rebuilding the buttons would drop keyboard focus onto <body>; remember it and put it back.
            const focused = refs.hotspots.contains(document.activeElement) ? document.activeElement : null;
            const focusKey = focused && { hotspot: focused.dataset.agfishHotspot, exit: focused.dataset.agfishExit };
            refs.hotspots.replaceChildren();
            panel?.setSentence('');
            for (const hotspot of room().hotspots) {
                if (!window.GoldfishState.conditions(state, hotspot)) continue;
                const pick = element('button', 'agfish-hotspot', t(hotspot.label));
                pick.type = 'button';
                pick.dataset.agfishHotspot = hotspot.id;
                const [x, y, width, height] = hotspot.rect;
                Object.assign(pick.style, { left: x * 100 + '%', top: y * 100 + '%', width: width * 100 + '%', height: height * 100 + '%' });
                panel.bind(pick, {
                    click: () => clickHotspot(hotspot), look: () => lookAt(hotspot),
                    hover: () => { if (started && !userPaused) panel.setSentence(sentenceFor(hotspot)); },
                    leave: () => panel.setSentence('')
                });
                refs.hotspots.append(pick);
            }
            for (const exit of room().exits) {
                const pick = element('button', 'agfish-exit', t(exit.label));
                pick.type = 'button';
                pick.dataset.agfishExit = exit.to;
                if (!window.GoldfishState.conditions(state, exit)) pick.classList.add('agfish-locked-exit');
                const [x, y, width, height] = exit.rect;
                Object.assign(pick.style, { left: x * 100 + '%', top: y * 100 + '%', width: width * 100 + '%', height: height * 100 + '%' });
                panel.bind(pick, {
                    click: () => clickExit(exit), look: () => lookExit(exit),
                    hover: () => { if (started && !userPaused) panel.setSentence(commands.sentence(t, { exit })); },
                    leave: () => panel.setSentence('')
                });
                refs.hotspots.append(pick);
            }
            if (focusKey) {
                const again = [...refs.hotspots.children].find(node => (focusKey.hotspot && node.dataset.agfishHotspot === focusKey.hotspot) ||
                    (focusKey.exit && node.dataset.agfishExit === focusKey.exit));
                (again || root).focus({ preventScroll: true });
            }
        }

        // ---- Actions ------------------------------------------------------------------------------------------

        function chooseAction(record, item) {
            if (blocked()) return;
            if (record.item && record.item !== item) { sayUi('item_needed'); return; }
            if (record.challenge) interactions.challenge(record, answer => perform(record, answer, item));
            else perform(record, undefined, item);
        }

        // A wrong answer speaks the record's wrongLines and offers the same menu again.
        async function perform(record, answer, item = null) {
            if (await run(record, answer, item) === 'wrong' && !disposed) chooseAction(record, item);
        }

        async function run(record, answer, item) {
            if (busy || disposed || authEnded || !visible()) return null;
            busy = true;
            flow = null;
            speech.cancel();
            panel.closeMenu();
            refresh();
            const ticket = generation;
            try {
                const result = window.GoldfishState.apply(state, record.id, { item, answer });
                if (result.error) {
                    if (result.error !== 'wrong') { sayUi(result.error); return null; }
                    say(result.lines);
                    return generation === ticket ? 'wrong' : null;
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
                speech.cancel();
                await renderer.act(record.id, record.animation || (record.effects?.puzzle ? 'celebrate' : record.verb === 'talk' ? 'talk' : 'use'));
                if (disposed || generation !== ticket) return;
                say(result.lines, { story: true, result: true });
            } finally {
                busy = false;
                if (!disposed) refresh();
            }
            return null;
        }

        async function travel(target) {
            if (blocked()) return;
            const previousAct = room().act;
            const result = window.GoldfishState.move(state, target);
            if (result.error) { say(result.lines); return; }
            speech.cancel();
            const previous = state, wasDirty = dirty;
            busy = true;
            flow = null;
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
                refresh();
                const firstActEntry = room().act !== previousAct && !previous.visited.some(id =>
                    window.GoldfishContent.rooms.find(value => value.id === id).act === room().act);
                if (firstActEntry) await checkpoint('chapter-' + room().act, durable);
                if (await loadRoom()) say(result.lines, { story: true });
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
            flow = null;
            panel.disarm();
            panel.closeMenu();
            speech.cancel();
            refs.start.hidden = true;
            renderer.cancelMotion();
            if (!await loadRoom()) return false;
            refresh();
            updatePause();
            resumeDialogue();
            return true;
        }

        // ---- Dialogs ------------------------------------------------------------------------------------------

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
            // Reading preferences live in this browser only, never in the save.
            const prefs = speech.prefs();
            const auto = element('label', 'agfish-setting');
            const toggle = document.createElement('input'); toggle.type = 'checkbox'; toggle.checked = prefs.autoAdvance;
            toggle.addEventListener('change', () => speech.setPrefs({ autoAdvance: toggle.checked }));
            auto.append(toggle, element('span', '', t('goldfish.ui.auto_advance')));
            const pace = element('label', 'agfish-setting');
            const speed = document.createElement('select');
            for (const name of ['slow', 'normal', 'fast']) {
                const option = element('option', '', t('goldfish.ui.read_speed_' + name)); option.value = name; speed.append(option);
            }
            speed.value = prefs.readSpeed;
            speed.addEventListener('change', () => speech.setPrefs({ readSpeed: speed.value }));
            pace.append(element('span', '', t('goldfish.ui.read_speed')), speed);
            pane.body.append(auto, pace);
            pane.node.addEventListener('close', () => { if (!disposed) persist(); }, { once: true });
        }

        function speechLog() {
            const pane = dialog(t('goldfish.ui.speech_log'));
            const entries = speech.log();
            if (!entries.length) { pane.body.append(element('p', '', t('goldfish.ui.speech_log_empty'))); return; }
            const list = element('ol', 'agfish-speech-log');
            for (const entry of entries) {
                const row = element('li', '');
                row.dataset.speaker = entry.speaker;
                row.append(element('strong', '', entry.name + ': '), document.createTextNode(entry.text));
                list.append(row);
            }
            pane.body.append(list);
        }

        function gameMenu() {
            if (disposed || !state) return;
            const pane = dialog(t('goldfish.ui.menu'));
            pane.body.append(element('p', 'agfish-small', [t('goldfish.ui.chapter', { act: room().act }), t(room().title),
                t('goldfish.ui.playtime', { minutes: Math.floor(state.elapsedMs / 60000) })].join(' · ')));
            const list = element('nav', 'agfish-menu');
            list.setAttribute('aria-label', t('goldfish.ui.title'));
            const entry = (label, callback) => {
                const pick = button(label, () => { pane.close(); callback(); });
                list.append(pick);
                return pick;
            };
            entry(t('goldfish.ui.journal'), journal);
            entry(t('goldfish.ui.hint'), hint);
            entry(t('goldfish.ui.saves'), () => interactions.saves());
            entry(t('goldfish.ui.speech_log'), speechLog);
            const preferences = entry(t('goldfish.ui.settings'), settings);
            if (started) entry(t(userPaused ? 'goldfish.ui.resume' : 'goldfish.ui.pause'), () => { userPaused = !userPaused; updatePause(); });
            const line = element('p', 'agfish-menu-status');
            line.setAttribute('role', 'status');
            const exporter = button(t('goldfish.ui.export'), () => interactions.exportState());
            pane.body.append(list, line, exporter);
            const mine = menuLive = { line, export: exporter, settings: preferences };
            pane.node.addEventListener('close', () => { if (menuLive === mine) menuLive = null; }, { once: true });
            syncMenuStatus();
        }

        // ---- Layout and lifecycle -----------------------------------------------------------------------------

        function build() {
            root.replaceChildren();
            const play = element('div', 'agfish-play');
            const stage = element('div', 'agfish-stage');
            const scene = element('div', 'agfish-scene');
            // The bar switches to its compact layout in narrow windows and when a short window would shrink the scene
            // (and with it the wide bar) too far. The decision uses a fixed wide-bar height, never the measured one, so
            // toggling the layout cannot feed back into the decision.
            const WIDE_BAR = 172, MIN_SCENE = 280;
            const fitScene = () => {
                if (disposed) return;
                const availableW = play.clientWidth, availableH = play.clientHeight;
                const narrow = root.clientWidth <= 760 || Math.min(availableW, (availableH - WIDE_BAR) * 16 / 9) < 560;
                const flipped = root.classList.contains('agfish-narrow') !== narrow;
                if (flipped) root.classList.toggle('agfish-narrow', narrow);
                const fitted = Math.min(availableW, (availableH - (panel?.el.offsetHeight || 0)) * 16 / 9);
                const width = Math.max(0, Math.max(Math.min(availableW, MIN_SCENE), fitted)) + 'px';
                if (root.style.getPropertyValue('--agfish-scene-w') !== width) root.style.setProperty('--agfish-scene-w', width);
                if (flipped) scheduleFit();
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
                root.focus({ preventScroll: true });
                resumeDialogue();
            }, 'agfish-primary');
            refs.continue.hidden = true;
            const startNew = button(t('goldfish.ui.start'), async () => {
                if (busy || disposed) return;
                if (dirty && !await confirm(t('goldfish.ui.new_title'), t('goldfish.ui.new_body'))) return;
                const previous = { state, started, dirty, userPaused, memoryConsent, startHidden: refs.start.hidden, saveStatus };
                busy = true;
                startNew.disabled = true;
                try {
                    state = window.GoldfishState.create();
                    changed(); started = true; userPaused = false; memoryConsent = false;
                    flow = null; panel.disarm(); panel.closeMenu(); speech.cancel();
                    refs.start.hidden = true;
                    renderer.unlock();
                    root.focus({ preventScroll: true });
                    if (!await loadRoom()) return;
                    const ticket = generation;
                    const accepted = await checkpoint('chapter-1');
                    if (disposed || generation !== ticket) return;
                    if (!accepted) {
                        state = previous.state; started = previous.started; dirty = previous.dirty;
                        userPaused = previous.userPaused; memoryConsent = previous.memoryConsent;
                        refs.start.hidden = previous.startHidden;
                        applyStatus(previous.saveStatus.text, previous.saveStatus.warning);
                        stateRevision++;
                        await loadRoom();
                        return;
                    }
                    say(window.GoldfishState.enter(state), { story: true });
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
            // While a story queue runs the scene only skips lines: hotspots and the floor wait.
            scene.addEventListener('click', event => {
                renderer.unlock();
                if (!started || userPaused || authEnded || !event.target.closest('.agfish-hotspots, .agfish-canvas')) return;
                if (speech.story()) { event.stopPropagation(); event.preventDefault(); speech.skipLine(); return; }
                if (event.target !== refs.hotspots) return;
                if (blocked()) return;
                panel.disarm();
                panel.closeMenu();
                speech.cancel();
                const bounds = scene.getBoundingClientRect();
                const x = (event.clientX - bounds.left) / bounds.width;
                approach('floor:' + Math.round(x * 25), [x, 0.83]);
            }, true);
            scene.addEventListener('contextmenu', event => {
                if (!event.target.closest('.agfish-hotspots, .agfish-canvas')) return;
                event.preventDefault();
                if (started && !userPaused) panel.disarm();
            });
            stage.append(scene);
            panel = window.GoldfishPanel.create({ root, t, button, element,
                onArm: () => { if (!speech.story()) speech.cancel(); },
                onItemLook: id => {
                    if (blocked()) return;
                    panel.closeMenu();
                    say([{ speaker: 'r0st', text: itemRecord(id).description }]);
                }
            });
            panel.menuButton.addEventListener('click', gameMenu);
            play.append(stage, panel.el);
            refs.status = element('p', 'agfish-save-status'); refs.status.setAttribute('role', 'status');
            refs.live = element('div', 'agfish-speech-live');
            refs.live.setAttribute('aria-live', 'polite'); refs.live.setAttribute('aria-atomic', 'true');
            root.append(play, refs.status, refs.live);
            stageObserver = new ResizeObserver(scheduleFit);
            stageObserver.observe(play);
            stageObserver.observe(panel.el);
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
            speech = window.GoldfishSpeech.create({ scene, t, live: refs.live, onRead: lineRead, onLine: speechLine, onEnd: speechEnded,
                anchor: speaker => renderer?.anchor?.(speaker) ?? null });
            interactions = window.GoldfishInteractions.create({ root, t, element, button, dialog, confirm, locale, panel,
                getState: () => state, getStore: () => store, persist, adopt, status,
                flushSaves,
                adoptRevisions: slots => { for (const slot of slots) if (!revisions.has(slot.id)) revisions.set(slot.id, slot.revision); }
            });
            refresh();
            updatePause();
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

        // Space/Esc: speech first (skip a line / skip the queue), then armed state, then the game menu.
        let swallowSpace = false;
        // Keys count when focus is inside the game, or when nothing is focused and this window is the active one.
        const mine = event => root.contains(event.target) ||
            ((event.target === document.body || event.target === document.documentElement) && (typeof ctx.isActive !== 'function' || ctx.isActive()));
        const keydown = event => {
            if (!mine(event)) return;
            if (panelCount || /^(INPUT|TEXTAREA|SELECT)$/.test(event.target.tagName) || !started || authEnded) return;
            if (event.code === 'Space') {
                if (!userPaused && speech.active()) { event.preventDefault(); swallowSpace = true; speech.skipLine(); }
                else if (event.target.tagName !== 'BUTTON') { event.preventDefault(); userPaused = !userPaused; updatePause(); }
            } else if (event.key === 'Escape' && !userPaused) {
                event.preventDefault();
                if (renderer?.skipMoment()) return;
                if (speech.active()) speech.skipAll();
                else if (!panel.cancel()) gameMenu();
            }
        };
        const keyup = event => {
            if (event.code === 'Space' && swallowSpace && mine(event)) { swallowSpace = false; event.preventDefault(); }
        };
        root.tabIndex = -1;
        document.addEventListener('keydown', keydown);
        document.addEventListener('keyup', keyup);
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
            document.removeEventListener('keydown', keydown);
            document.removeEventListener('keyup', keyup);
            window.removeEventListener('aurago:auth-ended', auth);
            document.removeEventListener('visibilitychange', updatePause);
            visibilityObserver?.disconnect();
            stageObserver?.disconnect();
            if (stageFrame) cancelAnimationFrame(stageFrame);
            stageFrame = 0;
            for (const node of dialogs) node.close();
            dialogs.clear();
            speech?.dispose();
            panel?.dispose();
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
