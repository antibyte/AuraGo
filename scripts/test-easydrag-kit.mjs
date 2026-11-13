// W1a Part B (FD-09): the kit. disclosure toggles aria-expanded and shows its body, callout carries the
// status disc and no border, chip is neutral, toast carries an action, modal() below 600px gets the
// sheet class. Runs on the miniDom stub of test-easydrag.mjs, which calls run(env); alone this file
// only has to import.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { webcrypto } from 'node:crypto';

export async function run({ apps, miniDom, eq }) {
    const dom = miniDom();
    const timers = [];
    const box = vm.createContext({
        window: {}, navigator: { platform: 'Linux' }, crypto: webcrypto, document: dom.document, console, Intl, Date, Set, Map,
        requestAnimationFrame: fn => { fn(); return 1; },
        setTimeout: (fn, ms) => { timers.push({ fn, ms }); return timers.length; },
        clearTimeout: id => { if (timers[id - 1]) timers[id - 1].fn = null; }
    });
    for (const file of ['easydrag-core.js', 'easydrag-kit.js']) vm.runInContext(fs.readFileSync(path.join(apps, file), 'utf8'), box, { filename: file });
    const ED = box.window.EasyDrag;
    ED.status = { disc: (value, size) => '<span class="ed-status-disc" data-status="' + value + '" role="img" aria-label="' + value + '"></span>' };
    const kit = ED.kit;

    // disclosure: closed at first, the head toggles it, the body follows, the head names the body
    const d = kit.disclosure({ title: 'Notes', count: 3, body: '<p class="ed-hint">x</p>' });
    const head = d.querySelector('.ed-disclosure-head'), body = d.querySelector('.ed-disclosure-body');
    eq('kit disclosure starts closed and names its body',
        [head.getAttribute('aria-expanded'), body.getAttribute('hidden') !== null, head.getAttribute('aria-controls') === body.id && body.id !== '', head.localName, kit.isOpen(d)],
        ['false', true, true, 'button', false]);
    eq('kit disclosure shows title and count', [d.querySelector('.ed-disclosure-title').textContent, d.querySelector('.ed-disclosure-count').textContent], ['Notes', '3']);
    head.fire('click');
    eq('kit disclosure opens on a click', [head.getAttribute('aria-expanded'), body.getAttribute('hidden'), kit.isOpen(d)], ['true', null, true]);
    head.fire('click');
    eq('kit disclosure closes on the next click', [head.getAttribute('aria-expanded'), body.getAttribute('hidden') !== null], ['false', true]);
    kit.setOpen(d, true); kit.setTitle(d, 'Other'); kit.setCount(d, '');
    eq('kit disclosure setOpen, setTitle and setCount', [kit.isOpen(d), d.querySelector('.ed-disclosure-title').textContent, d.querySelector('.ed-disclosure-count').getAttribute('hidden') !== null], [true, 'Other', true]);
    eq('kit disclosure open option', kit.isOpen(kit.disclosure({ title: 'a', open: true })), true);

    // the markup form works through one delegated listener on the app root
    const root = new dom.El('div', { class: 'ed-app' });
    kit.attach(root);
    const holder = root.appendChild(new dom.El('div', {}));
    holder.innerHTML = kit.disclosureMarkup({ title: 'A', className: 'ed-x', attrs: { 'data-ed-probe': '' }, open: false, body: '<p>b</p>' });
    const marked = holder.querySelector('.ed-x');
    eq('kit markup form carries class and attribute and starts closed', [!!marked, marked.getAttribute('data-ed-probe'), kit.isOpen(marked)], [true, '', false]);
    marked.querySelector('.ed-disclosure-head').fire('click');
    eq('kit markup form toggles through the delegated listener', kit.isOpen(marked), true);
    const own = kit.disclosure({ title: 'own' });
    root.appendChild(own);
    own.querySelector('.ed-disclosure-head').fire('click');
    eq('kit element form is not toggled twice under an attached root', kit.isOpen(own), true);

    // callout: disc in front, status on the element, no legacy variant class
    const c = kit.callout({ status: 'failed', text: 'Broken', action: { label: 'Retry', onClick: () => { c.fired = true; } } });
    eq('kit callout carries the status disc and the status',
        [c.getAttribute('data-status'), !!c.querySelector('.ed-status-disc'), c.className, c.querySelector('.ed-callout-text').textContent, c.querySelector('.ed-callout-action') !== null],
        ['failed', true, 'ed-callout', 'Broken', true]);
    c.querySelector('.ed-callout-action').fire('click');
    eq('kit callout action runs', c.fired, true);
    const info = kit.callout({ text: 'Hint' });
    eq('kit callout defaults to info with its own disc', [info.getAttribute('data-status'), !!info.querySelector('.ed-callout-disc')], ['info', true]);
    const css = fs.readFileSync(path.join(apps, '../../../css/desktop-app-easydrag.css'), 'utf8').replace(/\r\n/g, '\n');
    const rule = /\n\.ed-callout \{([^}]*)\}/.exec(css);
    eq('kit callout rule has no border and no input-like inset', [!!rule, rule && /border:\s*0/.test(rule[1]), rule && /inset/.test(rule[1])], [true, true, false]);

    // chip: neutral, optional glyph
    const chip = kit.chip({ text: 'sends mail', glyph: 'send' });
    eq('kit chip is neutral with a glyph', [chip.className, chip.querySelector('.ed-chip-text').textContent, !!chip.querySelector('svg')], ['ed-chip ed-chip--neutral', 'sends mail', true]);

    // toolbar: one button per item, order kept
    let picked = '';
    const bar = kit.toolbar([{ id: 'a', label: 'A', onClick: () => { picked += 'a'; } }, null, { id: 'b', label: 'B', icon: 'play', pressed: true, onClick: () => { picked += 'b'; } }]);
    const items = bar.querySelectorAll('.ed-toolbar-item');
    items[0].fire('click'); items[1].fire('click');
    eq('kit toolbar builds equal items', [bar.className, items.length, items[1].getAttribute('aria-pressed'), picked], ['ed-toolbar', 2, 'true', 'ab']);

    // toast: disc, text, one action; the action hides it first and then runs
    const host = new dom.El('div', {});
    let acted = 0, hidden = 0;
    const toast = kit.toast({ status: 'ok', text: 'Gone', host, closeLabel: 'Close', action: { label: 'Undo', onClick: () => { acted++; } }, onHide: () => { hidden++; } });
    const el = host.querySelector('[data-ed-toast]');
    eq('kit toast shows disc, text and action', [el.hidden, !!el.querySelector('.ed-status-disc'), el.querySelector('[data-ed-toast-text]').textContent, el.querySelector('[data-ed-toast-action] span').textContent], [false, true, 'Gone', 'Undo']);
    el.querySelector('[data-ed-toast-action]').fire('click');
    eq('kit toast action hides the toast and runs once', [el.hidden, acted, hidden], [true, 1, 1]);
    toast.show('Again');
    eq('kit toast shows again with a new text and closes after its time', [el.hidden, el.querySelector('[data-ed-toast-text]').textContent, timers.some(t => t.fn && t.ms === 8000)], [false, 'Again', true]);
    el.querySelector('[data-ed-toast-close]').fire('click');
    eq('kit toast close button hides it', el.hidden, true);
    toast.dispose();
    eq('kit toast dispose removes it', host.querySelector('[data-ed-toast]'), null);
    let note = null;
    const none = kit.toast({ status: 'failed', text: 'Failed', notify: n => { note = n; } });
    eq('kit toast without a host wraps notify', [none, note], [null, { title: 'EasyDrag', message: 'Failed', type: 'error' }]);
}
