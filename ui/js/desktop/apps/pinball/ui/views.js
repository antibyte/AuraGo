// AuraPinball view: toolbar, HUD host and overlay panels. createView(host, tr, assetURL, tables) -> view (plan 3.14).
import { createHud } from './hud.js';
import { ASSET_ROOT } from '../config.js';

function el(tag, text, className) { const node = document.createElement(tag); if (text !== undefined) node.textContent = text; if (className) node.className = className; return node; }
function button(label, action, primary = false) { const node = el('button', label, primary ? 'pb-button pb-primary' : 'pb-button'); node.type = 'button'; node.dataset.action = action; return node; }
function label(text, control) { const node = el('label', undefined, 'pb-field'); node.append(el('span', text), control); return node; }

export function createView(host, tr, assetURL, tables) {
    const registry = tables || {};
    const entries = Array.isArray(registry) ? registry.map(t => [t.id || t.layout?.id, t]) : Object.entries(registry);
    const tableList = entries.map(([id, t]) => ({ id, title: t?.layout?.meta?.title || t?.title || id }));
    const titleOf = id => tableList.find(t => t.id === id)?.title || id;
    const root = el('section', undefined, 'pb-app'); root.tabIndex = 0; root.setAttribute('aria-label', 'AuraPinball');
    const stage = el('div', undefined, 'pb-stage'), chrome = el('header', undefined, 'pb-toolbar');
    const brand = el('strong', 'AURA / PINBALL', 'pb-brand');
    const camera = el('select'); camera.dataset.setting = 'camera'; camera.setAttribute('aria-label', tr('camera'));
    for (const key of ['follow', 'play', 'top', 'showcase']) { const option = el('option', tr(key)); option.value = key; camera.append(option); }
    const left = el('div', undefined, 'pb-toolbar-side pb-toolbar-left'), right = el('div', undefined, 'pb-toolbar-side pb-toolbar-right');
    left.append(brand); right.append(camera, button(tr('fullscreen'), 'fullscreen'), button(tr('pause_action'), 'pause'));
    chrome.append(left, right);
    const hud = createHud(tr);
    const status = el('div', undefined, 'pb-status'); status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    const overlay = el('div', undefined, 'pb-overlay');
    root.append(stage, hud.root, chrome, status, overlay); host.replaceChildren(root);
    let kind = 'loading', calloutsAvailable = true;
    const number = new Intl.NumberFormat(document.documentElement.lang || 'de');
    const heading = next => next === 'lobby' ? 'AuraPinball' : tr(next === 'binding' ? 'settings' : next === 'error' ? 'load_error' : next);

    function applyCallouts() {
        const box = overlay.querySelector('[data-setting=callouts]'); if (!box) return;
        box.disabled = !calloutsAvailable;
        const hintNode = overlay.querySelector('.pb-callouts-hint');
        if (hintNode) hintNode.hidden = calloutsAvailable;
    }
    function lobby(card, data) {
        card.classList.add('pb-lobby'); card.append(el('p', tr('tagline'), 'pb-tagline'));
        const choices = el('div', undefined, 'pb-tables');
        for (const table of tableList) {
            const choice = button('', 'table-' + table.id); choice.classList.add('pb-table'); choice.dataset.table = table.id;
            choice.setAttribute('aria-pressed', String(data.table === table.id));
            const image = el('img'); image.src = assetURL(ASSET_ROOT + table.id + '-preview.webp'); image.alt = ''; image.width = 640; image.height = 320;
            choice.append(image, el('strong', table.title), el('span', tr(table.id + '_description'))); choices.append(choice);
        }
        card.append(choices);
        const setup = el('div', undefined, 'pb-setup'), count = el('select'); count.dataset.field = 'players';
        for (let n = 1; n <= 4; n++) { const option = el('option', String(n)); option.value = n; count.append(option); }
        count.value = String(data.count || 1); setup.append(label(tr('players'), count));
        for (let i = 0; i < (data.count || 1); i++) {
            const name = tr('player').replace('{n}', String(i + 1)), input = el('input');
            input.maxLength = 48; input.dataset.player = i; input.value = data.names?.[i] || name; input.setAttribute('aria-label', name); setup.append(input);
        }
        card.append(setup);
        const actions = el('div', undefined, 'pb-actions');
        for (const key of ['start', 'training', 'scores', 'settings', 'help']) actions.append(button(tr(key), key, key === 'start'));
        card.append(actions);
    }
    function settingsPanel(card, data) {
        const s = data.settings;
        const select = (name, keys, value) => { const sel = el('select'); sel.dataset.setting = name; for (const key of keys) { const o = el('option', tr(key)); o.value = key; sel.append(o); } sel.value = value; return sel; };
        card.append(label(tr('quality'), select('quality', ['low', 'medium', 'high'], s.quality)));
        card.append(label(tr('camera'), select('camera', ['follow', 'play', 'top'], ['play', 'top'].includes(s.camera) ? s.camera : 'follow')));
        for (const key of ['effects', 'music', 'ambience']) {
            const input = el('input'); input.type = 'range'; input.min = 0; input.max = 1; input.step = .01; input.value = s[key]; input.dataset.setting = key; card.append(label(tr(key), input));
        }
        const callouts = el('input'); callouts.type = 'checkbox'; callouts.checked = s.callouts !== false; callouts.dataset.setting = 'callouts';
        card.append(label(tr('callouts'), callouts), el('p', tr('callouts_unavailable'), 'pb-callouts-hint'));
        const reduced = el('input'); reduced.type = 'checkbox'; reduced.checked = s.reduced; reduced.dataset.setting = 'reduced'; card.append(label(tr('reduced'), reduced));
        const bindings = el('div', undefined, 'pb-bindings');
        for (const device of ['keys', 'pad']) {
            bindings.append(el('h3', tr(device)));
            for (const [key, code] of Object.entries(s[device])) {
                const row = el('div', undefined, 'pb-binding-row');
                row.append(el('span', tr(key === 'pause' ? 'pause_action' : key === 'start' ? 'start_action' : key)), button(device === 'keys' ? code : tr('button').replace('{n}', String(code + 1)), 'bind-' + device + '-' + key));
                bindings.append(row);
            }
        }
        card.append(bindings, button(tr('back'), 'back', true));
        applyCallouts();
    }
    function helpPanel(card, data) {
        card.classList.add('pb-help');
        card.append(el('p', tr('help_controls'), 'pb-help-text'), el('p', tr('help_rules'), 'pb-help-text'));
        const id = data.table || tableList[0]?.id;
        if (id) card.append(el('h2', titleOf(id)), el('p', tr('help_' + id), 'pb-help-text'));
        const credits = el('p', undefined, 'pb-credits'); credits.append(el('span', 'Audio: Kenney · CC0 1.0. '));
        const source = el('a', 'Kenney Impact Sounds'); source.href = 'https://kenney.nl/assets/impact-sounds'; source.target = '_blank'; source.rel = 'noopener noreferrer';
        credits.append(source, el('br'), el('span', tr('credits_engine')));
        card.append(credits, button(tr('back'), 'back', true));
    }
    function scoresPanel(card, next, data) {
        const rows = next === 'game_over' ? [...(data.game?.players || [])] : [...(data.scores?.[data.table] || [])];
        card.append(el('h2', titleOf(data.table)));
        if (next === 'game_over') {
            const match = data.match ?? data.game?.match;
            if (match !== undefined && match !== null && match !== false) card.append(el('p', tr('match') + ' ' + match, 'pb-match'));
        }
        const olist = el('ol', undefined, 'pb-score-list');
        for (const row of rows) { const li = el('li'); li.append(el('span', row.name), el('strong', number.format(row.score))); olist.append(li); }
        if (!rows.length) card.append(el('p', tr('no_scores'))); else card.append(olist);
        card.append(button(tr(next === 'game_over' ? 'quit' : 'back'), next === 'game_over' ? 'lobby' : 'back', true));
    }
    function panel(next, data = {}) {
        kind = next; root.dataset.panel = next; overlay.replaceChildren(); overlay.hidden = !next;
        hud.show(!!data.game || next === '' || next === 'pause' || next === 'showcase');
        chrome.hidden = next === 'loading' || next === 'error';
        if (!next) { root.focus({ preventScroll: true }); return; }
        const card = el('div', undefined, 'pb-panel'); card.setAttribute('role', next === 'lobby' ? 'region' : 'dialog'); card.setAttribute('aria-label', heading(next)); overlay.append(card);
        card.append(el('h1', next === 'error' ? tr(data.error || 'load_error') : heading(next)));
        if (next === 'loading') { card.append(el('p', tr('loading'))); return; }
        if (next === 'error') { card.append(button(tr('retry'), 'retry', true)); return; }
        if (next === 'lobby') lobby(card, data);
        else if (next === 'pause') { card.append(el('p', tr(data.reason || 'paused_hint'))); for (const key of ['resume', 'settings', 'help', 'restart', 'quit']) card.append(button(tr(key), key, key === 'resume')); }
        else if (next === 'showcase') { card.classList.add('pb-showcase'); card.append(el('p', tr('showcase_hint')), button(tr('resume'), 'resume', true)); }
        else if (next === 'help') helpPanel(card, data);
        else if (next === 'settings') settingsPanel(card, data);
        else if (next === 'binding') card.append(el('p', tr(data.device === 'keys' ? 'press_key' : 'press_pad')), button(tr('cancel'), 'cancel_binding'));
        else if (next === 'scores' || next === 'game_over') scoresPanel(card, next, data);
        else if (next === 'confirm_exit') card.append(el('p', tr('exit_text')), button(tr('cancel'), 'back'), button(tr(data.restart ? 'restart' : 'quit'), data.restart ? 'confirm_restart' : 'lobby', true));
        requestAnimationFrame(() => { if (overlay.contains(card)) card.querySelector('button,input,select')?.focus({ preventScroll: true }); });
    }
    return {
        root, stage, dmdHost: hud.dmdHost, hud, panel,
        kind: () => kind,
        notice(key) { const text = tr(key); status.textContent = text; hud.announce(text); },
        names: () => [...root.querySelectorAll('[data-player]')].map(input => input.value),
        focus: () => root.focus({ preventScroll: true }),
        setCallouts(available) { calloutsAvailable = available !== false; applyCallouts(); },
        dispose() { root.remove(); }
    };
}
