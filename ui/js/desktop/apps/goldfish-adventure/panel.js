/* The SCUMM-style bar: sentence line, four verbs, inventory grid and the conversation menu mode. */
(function () {
    'use strict';

    const VERBS = ['look', 'take', 'use', 'talk'];
    const LONG_PRESS_MS = 500;

    // Shared pointer/keyboard wiring: click = default, right click / long press / Shift+Enter = look.
    function bind(node, handlers = {}) {
        let pressTimer = 0, swallowClick = false, lastLook = 0;
        const look = () => {
            const now = Date.now();
            if (now - lastLook < 600) return;
            lastLook = now;
            handlers.look?.();
        };
        const release = () => { if (pressTimer) clearTimeout(pressTimer); pressTimer = 0; };
        node.addEventListener('click', event => {
            if (swallowClick) { swallowClick = false; event.preventDefault(); event.stopImmediatePropagation(); return; }
            handlers.click?.(event);
        });
        node.addEventListener('contextmenu', event => { event.preventDefault(); release(); look(); });
        node.addEventListener('keydown', event => {
            if (event.key === 'Enter' && event.shiftKey) { event.preventDefault(); look(); }
        });
        node.addEventListener('pointerdown', event => {
            if (event.pointerType === 'mouse') return;
            release();
            swallowClick = false;
            pressTimer = setTimeout(() => {
                pressTimer = 0;
                if (!node.isConnected) return;
                swallowClick = true;
                look();
            }, LONG_PRESS_MS);
        });
        for (const name of ['pointerup', 'pointercancel', 'pointerleave']) node.addEventListener(name, release);
        node.addEventListener('pointerenter', event => { if (event.pointerType === 'mouse') handlers.hover?.(); });
        node.addEventListener('pointerleave', event => { if (event.pointerType === 'mouse') handlers.leave?.(); });
        node.addEventListener('focus', () => { if (node.matches(':focus-visible')) handlers.hover?.(); });
        node.addEventListener('blur', () => handlers.leave?.());
        return node;
    }

    function create(options) {
        const { root, t, button, element } = options;
        const commands = window.GoldfishCommands;
        const itemNodes = new Map(), emptySlots = [];
        let state = null, verb = null, item = null, hover = '', idle = '', menuState = null, disposed = false, arrowFrame = 0;
        let dimmed = false, locked = false;

        const el = element('section', 'agfish-panel');
        el.setAttribute('role', 'group');
        el.setAttribute('aria-label', t('goldfish.ui.panel_label'));
        const sentence = element('p', 'agfish-sentence');
        const body = element('div', 'agfish-bar-body');

        const verbBox = element('div', 'agfish-verbs');
        verbBox.setAttribute('role', 'group');
        verbBox.setAttribute('aria-label', t('goldfish.ui.verbs_label'));
        const verbButtons = new Map();
        for (const name of VERBS) {
            const pick = button(t('goldfish.ui.verb_' + name), () => toggleVerb(name), 'agfish-verb');
            pick.dataset.agfishVerb = name;
            pick.setAttribute('aria-pressed', 'false');
            verbButtons.set(name, pick);
            verbBox.append(pick);
        }

        const inventoryBox = element('div', 'agfish-inventory-box');
        inventoryBox.setAttribute('role', 'group');
        inventoryBox.setAttribute('aria-label', t('goldfish.ui.inventory'));
        const grid = element('div', 'agfish-inventory');
        const arrows = element('div', 'agfish-inv-arrows');
        const up = button('▲', () => scrollRows(-1), 'agfish-inv-up');
        const down = button('▼', () => scrollRows(1), 'agfish-inv-down');
        up.setAttribute('aria-label', t('goldfish.ui.inventory_up'));
        down.setAttribute('aria-label', t('goldfish.ui.inventory_down'));
        arrows.append(up, down);
        inventoryBox.append(grid, arrows);

        const choices = element('div', 'agfish-choice-menu');
        choices.hidden = true;
        body.append(verbBox, inventoryBox, choices);
        const menuButton = button('☰', () => {}, 'agfish-menu-button');
        menuButton.setAttribute('aria-label', t('goldfish.ui.menu'));
        menuButton.setAttribute('aria-haspopup', 'dialog');
        el.append(sentence, body, menuButton);

        const itemRecord = id => window.GoldfishContent.items.find(value => value.id === id);

        function armedText() {
            if (item) return commands.sentence(t, { item: itemRecord(item) });
            return verb ? commands.sentence(t, { verb }) : '';
        }

        function renderSentence() {
            const armed = armedText();
            const text = hover || armed || (menuState && menuState.sentence) || idle;
            sentence.textContent = text;
            sentence.dataset.mode = hover ? 'hover' : armed ? 'armed' : menuState ? 'menu' : 'idle';
        }

        function syncArmed() {
            for (const [name, node] of verbButtons) node.setAttribute('aria-pressed', String(verb === name));
            for (const [id, node] of itemNodes) node.setAttribute('aria-pressed', String(item === id));
            el.classList.toggle('agfish-armed', !!(verb || item));
            renderSentence();
            options.onChange?.();
        }

        function toggleVerb(name) {
            verb = verb === name ? null : name;
            item = null;
            syncArmed();
            if (verb) options.onArm?.();
        }

        function clickItem(id) {
            if (verb === 'look') { verb = null; syncArmed(); options.onItemLook?.(id); return; }
            verb = null;
            item = item === id ? null : id;
            syncArmed();
            if (item) options.onArm?.();
        }

        function itemSentence(id) {
            const record = itemRecord(id);
            return verb === 'look' ? commands.sentence(t, { verb: 'look', hotspot: { label: record.name } }) : t(record.name);
        }

        function createItem(id) {
            const record = itemRecord(id);
            const pick = element('button', 'agfish-item');
            pick.type = 'button';
            pick.dataset.agfishItem = id;
            pick.setAttribute('aria-pressed', 'false');
            pick.setAttribute('aria-label', t(record.name));
            pick.title = t(record.name);
            if (record.icon) {
                const image = document.createElement('img');
                image.src = record.icon;
                image.alt = '';
                image.loading = 'lazy';
                image.addEventListener('error', () => { image.remove(); pick.classList.add('agfish-no-icon'); }, { once: true });
                pick.append(image);
            } else pick.classList.add('agfish-no-icon');
            pick.append(element('span', 'agfish-item-name', t(record.name)));
            bind(pick, {
                click: () => clickItem(id),
                look: () => { verb = null; syncArmed(); options.onItemLook?.(id); },
                hover: () => { hover = itemSentence(id); renderSentence(); },
                leave: () => { hover = ''; renderSentence(); }
            });
            return pick;
        }

        function scrollRows(direction) {
            const first = grid.firstElementChild;
            if (!first) return;
            const gap = parseFloat(getComputedStyle(grid).rowGap) || 0;
            grid.scrollBy({ top: direction * (first.offsetHeight + gap), behavior: 'auto' });
            updateArrows();
        }

        function updateArrows() {
            arrowFrame = 0;
            if (disposed) return;
            up.disabled = grid.scrollTop <= 0;
            down.disabled = grid.scrollTop + grid.clientHeight >= grid.scrollHeight - 1;
        }

        function scheduleArrows() {
            if (!arrowFrame && !disposed) arrowFrame = requestAnimationFrame(updateArrows);
        }
        grid.addEventListener('scroll', scheduleArrows);

        function renderInventory() {
            const ids = state.inventory;
            for (const [id, node] of itemNodes) if (!ids.includes(id)) { node.remove(); itemNodes.delete(id); }
            const slots = Math.max(12, Math.ceil(ids.length / 6) * 6);
            while (emptySlots.length < slots - ids.length) {
                const slot = element('span', 'agfish-inv-empty');
                slot.setAttribute('aria-hidden', 'true');
                emptySlots.push(slot);
            }
            const desired = ids.map(id => itemNodes.get(id) || createItem(id));
            ids.forEach((id, index) => itemNodes.set(id, desired[index]));
            desired.push(...emptySlots.slice(0, slots - ids.length));
            const same = desired.length === grid.children.length && desired.every((node, index) => grid.children[index] === node);
            if (!same) grid.replaceChildren(...desired);
            if (item && !ids.includes(item)) item = null;
            syncArmed();
            scheduleArrows();
        }

        function closeMenu() {
            if (!menuState) return false;
            const hadFocus = choices.contains(document.activeElement);
            menuState = null;
            choices.hidden = true;
            choices.replaceChildren();
            verbBox.hidden = false;
            inventoryBox.hidden = false;
            el.classList.remove('agfish-menu-open');
            renderSentence();
            if (hadFocus) root.focus({ preventScroll: true });
            scheduleArrows();
            return true;
        }

        // Conversation menu: replaces verbs and inventory until closeMenu(). Calling it again swaps the content.
        function menu(spec) {
            const reopened = !!menuState;
            verb = item = null;
            syncArmed();
            menuState = spec;
            verbBox.hidden = true;
            inventoryBox.hidden = true;
            choices.hidden = false;
            el.classList.add('agfish-menu-open');
            choices.replaceChildren();
            choices.setAttribute('role', 'group');
            choices.setAttribute('aria-label', spec.title || '');
            if (spec.title) choices.append(element('p', 'agfish-choice-title', spec.title));
            if (spec.subtitle) choices.append(element('p', 'agfish-choice-subtitle', spec.subtitle));
            const list = element('div', 'agfish-choice-list');
            spec.lines.forEach((line, index) => {
                const pick = button(line.label, () => spec.onPick?.(line, index), 'agfish-choice-line');
                if (line.action) pick.dataset.agfishAction = line.action;
                if (line.option !== undefined) pick.dataset.agfishOption = line.option;
                pick.disabled = !!line.disabled;
                list.append(pick);
            });
            choices.append(list);
            const aux = element('div', 'agfish-choice-aux');
            if (spec.onBack) {
                const back = button('↶ ' + t('goldfish.ui.choice_back'), () => spec.onBack(), 'agfish-choice-line agfish-choice-back');
                back.dataset.agfishBack = '';
                aux.append(back);
            }
            const cancel = button(t('goldfish.ui.cancel'), () => cancelMenu(), 'agfish-choice-line agfish-choice-cancel');
            cancel.dataset.agfishCancel = '';
            aux.append(cancel);
            choices.append(aux);
            renderSentence();
            if (!reopened || choices.contains(document.activeElement) || document.activeElement === document.body) {
                list.querySelector('button:not(:disabled)')?.focus({ preventScroll: true });
            }
        }

        function cancelMenu() {
            const current = menuState;
            if (!current) return false;
            if (current.onCancel) current.onCancel(); else closeMenu();
            return true;
        }

        choices.addEventListener('keydown', event => {
            if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return;
            const nodes = [...choices.querySelectorAll('button:not(:disabled)')];
            const at = nodes.indexOf(document.activeElement);
            if (at < 0) return;
            event.preventDefault();
            nodes[(at + (event.key === 'ArrowDown' ? 1 : nodes.length - 1)) % nodes.length].focus();
        });

        renderSentence();

        return {
            el, menuButton, bind,
            render: next => { state = next; renderInventory(); },
            setIdle: text => { idle = text || ''; renderSentence(); },
            setSentence: text => { hover = text || ''; renderSentence(); },
            armedVerb: () => verb,
            armedItem: () => item,
            // Drops an armed verb/item; true if something was armed.
            disarm: () => {
                if (!verb && !item) return false;
                verb = item = null;
                syncArmed();
                return true;
            },
            // Esc: cancels an open menu, else an armed verb/item; true if anything changed.
            cancel: () => cancelMenu() || (verb || item ? (verb = item = null, syncArmed(), true) : false),
            menu, closeMenu,
            menuOpen: () => !!menuState,
            // Dimmed while a story queue plays; locked until the game has started. The menu button stays usable.
            setDim: on => {
                dimmed = !!on;
                el.classList.toggle('agfish-dim', dimmed);
                body.inert = dimmed || locked;
                // An inert subtree drops focus onto <body>; keep it inside the game so Space/Esc still reach it.
                if (body.inert && body.contains(document.activeElement)) el.closest('.agfish-app')?.focus({ preventScroll: true });
            },
            setLocked: on => {
                locked = !!on;
                el.classList.toggle('agfish-locked', locked);
                body.inert = dimmed || locked;
            },
            dispose: () => {
                disposed = true;
                if (arrowFrame) cancelAnimationFrame(arrowFrame);
                arrowFrame = 0;
                el.remove();
            }
        };
    }

    window.GoldfishPanel = Object.freeze({ create, bind });
}());
