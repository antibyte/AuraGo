/* Runenpfad dialogue window: portrait, name plate, typewriter text, choices and the trade panel.
 * Builds DOM through the `el` helper (createElement/textContent only); reads RunenpfadDialogue.view and
 * calls Dialogue.choose / Trade.buy. app.js owns the shell, input routing and the audio sink (`emit`). */
(function (root) {
    'use strict';

    const GUARD_MS = 350;      // same constant as the round cards in app.js
    const TYPE_CPS = 45;
    const BLIP_MS = 120;
    const NPC = 'runenpfad.npc.';
    const UI = NPC + 'ui.';
    const NPC_ICON_CLIPS = { map_charm: 'ui-npc-icons.quest', lucky_amulet: 'ui-npc-icons.heart', discount_token: 'ui-npc-icons.coins', rune_attune: 'ui-npc-icons.ask' };
    const RUNE_OFFER = /^rune_(fire|frost|storm|magnet)$/;

    function create(o) {
        const el = o.el;
        const t = o.t;
        const fill = o.fill;
        const hud = o.hud;
        const isBlocked = typeof o.isBlocked === 'function' ? o.isBlocked : function () { return false; };
        const emit = typeof o.emit === 'function' ? o.emit : function () {};
        // Persist profile changes right after a choice or purchase (the paused/dialogue tick may not).
        function flush(sim) { if (sim && sim.storyDirty && typeof o.flush === 'function') o.flush(); }
        const perf = root.performance;

        let open = false;
        let disposed = false;
        let openSim = null;
        let lastSim = null;
        let lastProfile = null;
        let lastView = null;
        let seqKey = '';
        let changedAt = 0;
        let focusIdx = 0;
        let typedAt = 0;
        let chars = [];
        let shownCount = -1;
        let lastBlip = 0;
        let portraitKey = '';
        let portraitAsked = '';
        let tradeSig = '';
        let tradeMsg = null; // { text, ok }

        // ---- DOM ----
        const rootEl = el('div', 'rp-talk');
        rootEl.hidden = true;
        rootEl.setAttribute('role', 'dialog');
        rootEl.setAttribute('aria-modal', 'false');
        const panel = el('div', 'rp-talk-panel');

        const portrait = el('div', 'rp-talk-portrait');
        const portraitCanvas = el('canvas', 'rp-talk-canvas');
        portraitCanvas.width = 384;
        portraitCanvas.height = 384;
        portraitCanvas.setAttribute('aria-hidden', 'true');
        const mono = el('span', 'rp-talk-mono');
        mono.hidden = true;
        mono.setAttribute('aria-hidden', 'true');
        portrait.append(portraitCanvas, mono);

        const main = el('div', 'rp-talk-main');
        const plate = el('div', 'rp-talk-name');
        const who = el('strong', 'rp-talk-who');
        const title = el('span', 'rp-talk-title');
        const badge = el('span', 'rp-talk-faction');
        const badgeText = el('span', 'rp-talk-faction-name');
        const rep = el('span', 'rp-talk-rep');
        const repPips = el('span', 'rp-pips rp-talk-pips');
        for (let i = 0; i < 11; i++) repPips.append(el('i', i === 5 ? 'is-mid' : ''));
        repPips.setAttribute('role', 'img');
        const repNum = el('span', 'rp-talk-rep-num rp-num');
        rep.append(repPips, repNum);
        badge.append(badgeText);
        plate.append(who, title, badge, rep);

        const text = el('p', 'rp-talk-text');
        const shownEl = el('span', 'rp-talk-shown');
        const restEl = el('span', 'rp-talk-rest');
        restEl.setAttribute('aria-hidden', 'true');
        text.append(shownEl, restEl);
        const tipEl = el('p', 'rp-talk-tip');
        tipEl.hidden = true;
        const choices = el('ol', 'rp-talk-choices');

        const trade = el('div', 'rp-trade');
        trade.hidden = true;
        main.append(plate, text, tipEl, choices, trade);
        panel.append(portrait, main);
        rootEl.append(panel);

        // ---- helpers ----
        function npcs() { return root.RunenpfadNpcs; }
        function dlg() { return root.RunenpfadDialogue; }
        function trd() { return root.RunenpfadTrade; }
        function art() { return root.RunenpfadArt; }

        function reduced() {
            return !!((root.document.body && root.document.body.dataset.animations === 'false') ||
                (root.matchMedia && root.matchMedia('(prefers-reduced-motion: reduce)').matches));
        }

        function npcOf(sim, enc) {
            const list = (sim.stage && sim.stage.npcs) || [];
            for (let i = 0; i < list.length; i++) if (list[i].ref === enc.npcRef) return list[i];
            return null;
        }

        function say(events) { emit(events); }

        // Events pushed by core calls made outside a simulation tick would be dropped; forward the new ones.
        function forward(sim, before) {
            if (sim.events && sim.events.length > before) say(sim.events.slice(before));
        }

        function evCount(sim) { return sim.events ? sim.events.length : 0; }

        function returnFocus() {
            const shellEl = rootEl.parentNode;
            const active = root.document.activeElement;
            if (shellEl && active && rootEl.contains(active) && typeof shellEl.focus === 'function') shellEl.focus({ preventScroll: true });
        }

        // ---- portrait ----
        function paintPortrait(npcId, mood) {
            const A = art();
            const atlas = A ? A.current() : null;
            const portraitDef = npcs() && npcs().NPCS[npcId];
            const sheetId = 'portrait-' + ((portraitDef && portraitDef.portrait) || npcId);
            const ready = !!(atlas && atlas.sheets[sheetId]);
            const npcClip = 'npc-' + npcId + '.talk';
            const mode = ready ? 'p' : (atlas && A.has(atlas, npcClip) ? 'n' : 'm');
            const key = npcId + ':' + mood + ':' + mode;
            if (atlas && !ready && portraitAsked !== sheetId) {
                portraitAsked = sheetId;
                A.loadSheet(atlas, sheetId).then(function (ok) { if (ok && !disposed) portraitKey = ''; });
            }
            if (key === portraitKey) return;
            portraitKey = key;
            const cx = portraitCanvas.getContext('2d');
            if (cx) cx.clearRect(0, 0, 384, 384);
            const def = npcs() && npcs().NPCS[npcId];
            mono.hidden = mode !== 'm';
            portraitCanvas.hidden = mode === 'm';
            rootEl.dataset.rpPortrait = mode === 'p' ? 'sheet' : (mode === 'n' ? 'sprite' : 'monogram');
            if (mode === 'm') {
                mono.textContent = (def ? t(def.nameKey) : npcId).charAt(0).toUpperCase();
                return;
            }
            if (!cx) return;
            if (mode === 'p') {
                const clip = A.has(atlas, sheetId + '.' + mood) ? sheetId + '.' + mood : sheetId + '.neutral';
                A.draw(cx, atlas, clip, 0, 192, 384, { height: 384, pixel: true });
            } else {
                A.draw(cx, atlas, npcClip, 0, 192, 372, { height: 340, pixel: true });
            }
        }

        // ---- name plate ----
        function paintPlate(npcId) {
            const N = npcs();
            const def = N && N.NPCS[npcId];
            who.textContent = def ? t(def.nameKey) : npcId;
            title.textContent = def ? t(def.titleKey) : '';
            title.hidden = !def;
            if (!def || !def.faction) { // villagers have no faction: no badge, no reputation pips
                badge.hidden = true;
                rep.hidden = true;
                return;
            }
            badge.hidden = false;
            rep.hidden = false;
            badge.dataset.rpFaction = def.faction;
            badgeText.textContent = t(NPC + 'faction.' + def.faction);
            const value = lastSim ? N.getRep(lastSim, def.faction) : 0;
            const pipsList = repPips.children;
            for (let i = 0; i < 11; i++) {
                const v = i - 5;
                const on = value >= 0 ? (v >= 0 && v <= value) : (v <= 0 && v >= value);
                pipsList[i].className = (i === 5 ? 'is-mid ' : '') + (on ? 'is-on ' + (value < 0 ? 'is-neg' : 'is-pos') : '');
            }
            repNum.textContent = (value > 0 ? '+' : '') + value;
            const label = t(UI + 'rep') + ': ' + t(NPC + 'faction.' + def.faction) + ' ' + repNum.textContent;
            repPips.setAttribute('aria-label', label);
            rep.title = label;
        }

        // ---- typewriter ----
        function setTyped(n) {
            if (n === shownCount) return;
            shownCount = n;
            shownEl.textContent = chars.slice(0, n).join('');
            restEl.textContent = chars.slice(n).join('');
            rootEl.classList.toggle('is-typing', n < chars.length);
        }

        function startText(full) {
            chars = Array.from(full);
            shownCount = -1;
            typedAt = perf.now();
            lastBlip = 0;
            text.setAttribute('aria-label', full);
            setTyped(reduced() ? chars.length : 0);
        }

        function typing() { return shownCount < chars.length; }

        function finishTyping() {
            typedAt = -1e9;
            setTyped(chars.length);
        }

        function stepTyping(now) {
            if (!typing() || !open) return;
            const n = Math.min(chars.length, Math.floor((now - typedAt) * TYPE_CPS / 1000));
            if (n === shownCount) return;
            const fresh = chars.slice(Math.max(0, shownCount), n).join('');
            setTyped(n);
            if (lastView && /\S/.test(fresh) && now - lastBlip >= BLIP_MS) {
                lastBlip = now;
                say([{ type: 'npc_text', detail: { npcId: lastView.npcId } }]);
            }
        }

        // ---- focus ----
        function focusables() {
            return lastView && lastView.panel === 'trade'
                ? Array.prototype.slice.call(trade.querySelectorAll('[data-rp-focus]'))
                : Array.prototype.slice.call(choices.querySelectorAll('button'));
        }

        function setFocus(i) {
            const list = focusables();
            if (!list.length) return;
            focusIdx = (i + list.length) % list.length;
            list.forEach(function (node, k) { node.classList.toggle('is-focus', k === focusIdx); });
            const target = list[focusIdx];
            if (lastView && lastView.panel === 'trade' && target && typeof target.scrollIntoView === 'function') {
                try { target.scrollIntoView({ block: 'nearest' }); } catch (err) { /* ignore */ }
            }
        }

        // ---- talk panel ----
        function renderTalk(v) {
            const template = t(v.textKey);
            const params = Object.assign({}, v.params);
            const tipText = v.tip ? fill(v.tip.key, v.tip.params || {}) : '';
            if (v.tip) params.tip = tipText;
            startText(fill(v.textKey, params));
            const showTip = !!(v.tip && template.indexOf('{tip}') < 0);
            tipEl.hidden = !showTip;
            tipEl.textContent = showTip ? tipText : '';
            choices.replaceChildren();
            v.choices.forEach(function (c, i) {
                const li = el('li', 'rp-talk-li');
                const btn = el('button', 'rp-btn rp-talk-choice');
                btn.type = 'button';
                btn.dataset.rpChoice = String(i);
                hud.setButton(btn, fill(c.textKey, c.params || {}), String(i + 1));
                btn.addEventListener('click', function () { activate(i); });
                btn.addEventListener('pointerenter', function () { if (lastView && lastView.panel === 'talk') setFocus(i); });
                li.append(btn);
                choices.append(li);
            });
            focusIdx = 0;
            setFocus(0);
        }

        function activate(i) {
            if (!open || !lastSim || isBlocked()) return;
            if (perf.now() - changedAt < GUARD_MS) return;
            if (lastView.panel === 'trade') {
                activateTrade(i);
                return;
            }
            if (typing()) {
                finishTyping();
                return;
            }
            if (i < 0 || i >= lastView.choices.length) return;
            const sim = lastSim;
            const before = evCount(sim);
            dlg().choose(sim, i);
            forward(sim, before);
            flush(sim);
            sync(sim, lastProfile);
        }

        // The last unconditional choice that only closes the dialogue (Esc = leave), as a visible index.
        function leaveIndex(sim, enc) {
            const S = root.RunenpfadNpcStory;
            const g = S && S.GRAPHS && S.GRAPHS[enc.npcId];
            const node = g && g.nodes ? g.nodes[enc.nodeId.slice(enc.npcId.length + 1)] : null;
            if (!node) return -1;
            let visible = 0;
            let found = -1;
            node.choices.forEach(function (c) {
                if (!dlg().evalCond(c.if, sim)) return;
                if (c.to === null && !c.if && !c.roll && !(c.effects && c.effects.length)) found = visible;
                visible += 1;
            });
            return found;
        }

        // ---- trade panel ----
        function offerIcon(id) {
            const box = el('span', 'rp-trade-icon');
            box.setAttribute('aria-hidden', 'true');
            const A = art();
            const atlas = A ? A.current() : null;
            const clip = RUNE_OFFER.test(id) ? 'powerups.' + id.slice(5) : NPC_ICON_CLIPS[id];
            if (id === 'potion') {
                box.append(hud.icon('potion', 26));
            } else if (atlas && clip && A.has(atlas, clip)) {
                const c = el('canvas', 'rp-trade-canvas');
                c.width = 56;
                c.height = 56;
                const cx = c.getContext('2d');
                if (cx) {
                    A.draw(cx, atlas, clip, 0, 28, 28, { height: RUNE_OFFER.test(id) ? 58 : 50 });
                    box.append(c);
                }
            }
            if (!box.firstChild) box.textContent = '✦';
            return box;
        }

        function signature(offers, sim) {
            const A = art();
            return (A && A.current() ? '1' : '0') + '|' + sim.profile.coins + '|' + offers.map(function (x) {
                return x.id + ':' + x.price + ':' + x.stock + ':' + x.reason;
            }).join(',');
        }

        function reasonText(reason) {
            if (reason === 'coins') return t(UI + 'too_poor');
            if (reason === 'stock') return t(UI + 'sold_out');
            if (reason === 'cap') return t(UI + 'owned');
            return '';
        }

        function renderTrade(v, sim) {
            const T = trd();
            trade.replaceChildren();
            const offers = v.offers || [];
            tradeSig = signature(offers, sim);
            const head = el('div', 'rp-trade-head');
            const heading = el('h3', 'rp-trade-title', t(UI + 'trade_title'));
            const coinBox = el('span', 'rp-chip rp-trade-coins');
            coinBox.title = t(UI + 'coins');
            coinBox.setAttribute('aria-label', t(UI + 'coins') + ' ' + sim.profile.coins);
            coinBox.append(hud.icon('coin', 16), el('span', 'rp-num', String(sim.profile.coins)));
            head.append(heading, coinBox);
            const list = el('ul', 'rp-trade-list');
            offers.forEach(function (off, i) {
                const def = T && T.OFFERS[off.id];
                const card = el('li', 'rp-trade-card' + (off.canBuy ? '' : ' is-off') + (off.reason ? ' is-' + off.reason : ''));
                card.dataset.rpFocus = '1';
                card.dataset.rpOffer = off.id;
                const body = el('div', 'rp-trade-body');
                body.append(el('strong', 'rp-trade-name', def ? t(def.nameKey) : off.id), el('span', 'rp-trade-desc', def ? t(def.descKey) : ''));
                const meta = el('div', 'rp-trade-meta');
                const price = el('span', 'rp-trade-price');
                price.append(hud.icon('coin', 14), el('span', 'rp-num', String(off.price)));
                if (off.price !== off.base) {
                    const up = off.price > off.base;
                    const mark = el('span', 'rp-trade-delta ' + (up ? 'is-up' : 'is-down'), up ? '↑' : '↓');
                    mark.title = t(UI + (up ? 'price_up' : 'price_down'));
                    price.append(mark);
                }
                meta.append(price, el('span', 'rp-trade-stock rp-num', '×' + off.stock));
                const note = off.reason ? el('span', 'rp-trade-note', reasonText(off.reason)) : null;
                const buy = el('button', 'rp-btn-gold rp-trade-buy');
                buy.type = 'button';
                buy.disabled = !off.canBuy;
                buy.dataset.rpBuy = off.id;
                hud.setButton(buy, t(UI + 'buy'), String(i + 1));
                buy.addEventListener('click', function () { activateTrade(i); });
                body.append(meta);
                if (note) body.append(note);
                card.append(offerIcon(off.id), body, buy);
                card.addEventListener('pointerenter', function () { setFocus(i); });
                list.append(card);
            });
            const foot = el('div', 'rp-trade-foot');
            const msg = el('p', 'rp-trade-msg' + (tradeMsg ? (tradeMsg.ok ? ' is-ok' : ' is-bad') : ''), tradeMsg ? tradeMsg.text : '');
            msg.setAttribute('role', 'status');
            const back = el('button', 'rp-btn rp-trade-back');
            back.type = 'button';
            back.dataset.rpFocus = '1';
            hud.setButton(back, t(UI + 'back'), 'Esc');
            back.addEventListener('click', function () { leaveTrade(); });
            back.addEventListener('pointerenter', function () { setFocus(offers.length); });
            foot.append(msg, back);
            trade.append(head, list, foot);
            if (focusIdx > offers.length) focusIdx = 0;
            setFocus(focusIdx);
        }

        function activateTrade(i) {
            const sim = lastSim;
            const offers = lastView.offers || [];
            if (i === offers.length) {
                leaveTrade();
                return;
            }
            const off = offers[i];
            const enc = sim.encounter;
            const npc = enc ? npcOf(sim, enc) : null;
            if (!off || !npc || !trd()) return;
            setFocus(i);
            if (!off.canBuy) {
                tradeMsg = { text: reasonText(off.reason), ok: false };
            } else {
                const before = evCount(sim);
                const res = trd().buy(sim, npc, off.id);
                forward(sim, before);
                flush(sim);
                // A fake sale looks exactly like a real one; the core reveals it when the dialogue closes.
                if (res.ok) tradeMsg = { text: fill(UI + 'bought', { name: t(trd().OFFERS[off.id].nameKey) }), ok: true };
                else tradeMsg = { text: reasonText(res.reason), ok: false };
            }
            lastView = dlg().view(sim) || lastView;
            renderTrade(lastView, sim);
        }

        function leaveTrade() {
            const sim = lastSim;
            if (!sim) return;
            dlg().closeTrade(sim);
            flush(sim);
            sync(sim, lastProfile);
        }

        // ---- lifecycle ----
        function show(sim) {
            open = true;
            openSim = sim;
            rootEl.hidden = false;
            void rootEl.offsetWidth; // let the entry transition start from the hidden state
            rootEl.classList.add('is-in');
        }

        function hide(sim) {
            const wasSim = openSim;
            open = false;
            openSim = null;
            lastView = null;
            seqKey = '';
            tradeMsg = null;
            rootEl.classList.remove('is-in', 'is-typing');
            rootEl.hidden = true;
            returnFocus();
            // A duel (or any other encounter) taking over is not a farewell.
            if (wasSim && sim === wasSim && !sim.encounter) say([{ type: 'npc_close', detail: { npcId: rootEl.dataset.rpNpc || '' } }]);
        }

        function render(v, sim, enc) {
            lastView = v;
            tradeMsg = null;
            rootEl.dataset.rpNode = enc.nodeId;
            rootEl.dataset.rpNpc = v.npcId;
            rootEl.dataset.rpPanel = v.panel;
            rootEl.dataset.rpMood = v.mood;
            const N = npcs();
            rootEl.setAttribute('aria-label', (N && N.NPCS[v.npcId] ? t(N.NPCS[v.npcId].nameKey) : v.npcId) + ' – ' + t(UI + 'talk'));
            paintPlate(v.npcId);
            paintPortrait(v.npcId, v.mood);
            const isTrade = v.panel === 'trade';
            text.hidden = isTrade;
            choices.hidden = isTrade;
            trade.hidden = !isTrade;
            if (isTrade) {
                tipEl.hidden = true;
                startText('');
                focusIdx = 0;
                renderTrade(v, sim);
            } else {
                renderTalk(v);
            }
        }

        function sync(sim, profile) {
            if (disposed) return;
            lastSim = sim;
            lastProfile = profile;
            const D = dlg();
            const enc = sim && sim.encounter;
            const v = D && enc && enc.kind === 'dialogue' ? D.view(sim) : null;
            if (!v) {
                if (open) hide(sim);
                return;
            }
            const now = perf.now();
            if (!open) show(sim);
            const key = enc.npcRef + ':' + enc.seq + ':' + enc.panel;
            if (key !== seqKey) {
                seqKey = key;
                changedAt = now;
                render(v, sim, enc);
            } else {
                lastView = v;
                paintPortrait(v.npcId, v.mood);
                if (v.panel === 'trade' && signature(v.offers || [], sim) !== tradeSig) renderTrade(v, sim);
            }
            stepTyping(now);
        }

        // Returns true when the key was consumed. Esc falls through (pause) when there is nothing to leave with.
        function handleKey(ev, sim) {
            if (!open || !sim || sim !== lastSim || !lastView || isBlocked()) return false;
            const code = ev.code;
            if (code === 'KeyP') return false;
            const digit = /^(?:Digit|Numpad)([1-9])$/.exec(code);
            const nav = code === 'ArrowUp' || code === 'ArrowDown' || code === 'KeyW' || code === 'KeyS';
            const act = code === 'Enter' || code === 'NumpadEnter' || code === 'Space';
            const esc = code === 'Escape';
            if (!(digit || nav || act || esc)) return true;
            // Nothing to leave with: Esc falls through so app.js opens the pause dialog.
            if (esc && !ev.repeat && lastView.panel !== 'trade' && leaveIndex(sim, sim.encounter) < 0) return false;
            ev.preventDefault();
            if (ev.repeat) return true;
            if (nav) {
                setFocus(focusIdx + (code === 'ArrowUp' || code === 'KeyW' ? -1 : 1));
                return true;
            }
            if (perf.now() - changedAt < GUARD_MS) return true;
            if (esc) {
                if (lastView.panel === 'trade') leaveTrade();
                else activate(leaveIndex(sim, sim.encounter));
                return true;
            }
            if (lastView.panel === 'trade') {
                if (!digit) activateTrade(focusIdx);
                else if (Number(digit[1]) <= (lastView.offers || []).length) activateTrade(Number(digit[1]) - 1);
                return true;
            }
            if (typing()) {
                finishTyping();
                return true;
            }
            activate(digit ? Number(digit[1]) - 1 : focusIdx);
            return true;
        }

        // Click on the text skips the typewriter.
        text.addEventListener('click', function () { if (open && typing()) finishTyping(); });
        portrait.addEventListener('click', function () { if (open && typing()) finishTyping(); });

        function close() {
            const sim = lastSim;
            if (sim && sim.encounter && sim.encounter.kind === 'dialogue' && dlg()) dlg().close(sim);
            if (open) hide(sim);
        }

        function dispose() {
            disposed = true;
            open = false;
            rootEl.hidden = true;
        }

        return { root: rootEl, sync: sync, handleKey: handleKey, close: close, dispose: dispose };
    }

    root.RunenpfadDialogueUi = { create: create };
})(typeof window !== 'undefined' ? window : globalThis);
