// G13 round 4: the state mark of a stale, read-only or alert widget, the seven variants that used to be placeholders (climate stepper and scale, gauge bar and tank,
// temperature thermometer and comfort, sensor icon) and the select picker that stands in for a segmented bar whose labels do not fit.
import assert from 'node:assert/strict'; import { loadEngine, ENGINE_ORDER, read, eq } from './harness.mjs';
const EN = JSON.parse(read('ui/lang/homedash/en.json'));
const DEG = String.fromCodePoint(0xb0);
const view = p => Object.assign({ id: 'x:1', kind: 'sensor', name: 'Thing', value: 42, unit: '%', attributes: { updated_at: new Date().toISOString() }, available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, p);
function mount(type, variant, views, o) {
    const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), t = E.core.makeT(EN, { warn() {} }), state = Object.assign({}, views), calls = [], opts = o || {};
    const api = () => ({ entity: b => state[b] || null, can: (b, cap) => opts.can ? opts.can(cap) : true, command: (...a) => { calls.push(a); return Promise.resolve({}); }, moreInfo() {}, history: () => Promise.resolve({ points: opts.points || [], source: 'ring' }), announce() {} });
    const h = E.widgets.host({ widget: Object.assign({ id: 'w', type, variant, title: 'Title', bindings: { primary: 'x:1' }, options: {} }, opts.widget), size: opts.size || { w: 2, h: 2 }, t, api });
    document.body.appendChild(h.el);
    return { E, h, state, calls, q: s => h.el.querySelector(s), all: s => Array.from(h.el.querySelectorAll(s)), set(b, v) { state[b] = v; h.update(); } };
}

export const tests = [
    ['mark: a healthy widget has a hidden mark; stale, read-only and alert show it with a name and a tooltip, alert before read-only before stale', () => {
        const w = mount('sensor', 'numeral', { primary: view({}) }), mark = () => w.q('.hd-head > .hd-mark');
        assert.ok(mark(), 'the mark sits in the head'); assert.equal(mark().hasAttribute('hidden'), true); assert.equal(mark().getAttribute('title'), null);
        w.set('primary', view({ stale: true, attributes: { updated_at: new Date(Date.now() - 14 * 60000).toISOString() } }));
        assert.equal(mark().hasAttribute('hidden'), false); assert.equal(mark().getAttribute('data-kind'), 'stale'); assert.match(mark().getAttribute('aria-label'), /^Updated 14\S* ago$/); assert.equal(mark().getAttribute('title'), mark().getAttribute('aria-label'));
        w.set('primary', view({ stale: true, attributes: {} })); assert.equal(mark().getAttribute('aria-label'), EN['homedash.state.stale'], 'no parsable update time: the plain word');
        w.set('primary', view({ stale: true, readonly: true })); assert.equal(mark().getAttribute('data-kind'), 'readonly'); assert.equal(mark().getAttribute('title'), EN['homedash.state.readonly']);
        w.set('primary', view({ stale: true, readonly: true, tone: 'crit' })); assert.equal(mark().getAttribute('data-kind'), 'alert'); assert.equal(mark().getAttribute('aria-label'), EN['homedash.state.alert']);
        w.set('primary', view({})); assert.equal(mark().hasAttribute('hidden'), true); assert.equal(mark().getAttribute('aria-label'), null); }],
    ['mark: the alert tone a widget derives itself (a gauge zone) counts as well as the entity tone', () => {
        const w = mount('gauge', 'needle', { primary: view({ value: 95 }) }, { widget: { options: { zones: [{ from: 90, to: 100, tone: 'crit' }] } } });
        assert.equal(w.q('.hd-mark').getAttribute('data-kind'), 'alert'); w.set('primary', view({ value: 10 })); assert.equal(w.q('.hd-mark').hasAttribute('hidden'), true); }],
    ['every language carries the two new state words', () => {
        for (const l of ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh']) { const d = JSON.parse(read('ui/lang/homedash/' + l + '.json')); for (const k of ['homedash.state.alert', 'homedash.state.stale']) assert.ok(typeof d[k] === 'string' && d[k].length > 1, l + ' ' + k); } }],
    ['gauge bar and tank: the readout, the level, the zones from the warn and alert options and the tone of the zone the value is in', () => {
        const w = mount('gauge', 'bar', { primary: view({ value: 80 }) }, { widget: { options: { warn: 70, alert: 90 } }, size: { w: 3, h: 1 } }), bar = w.q('.hd-bar');
        assert.equal(bar.getAttribute('data-hd-variant'), 'h'); assert.equal(bar.getAttribute('role'), 'meter'); assert.equal(bar.style.getPropertyValue('--hd-value'), '0.8000');
        eq(w.all('.hd-bar-zone').map(z => z.getAttribute('data-tone')), ['warn', 'crit']); assert.equal(w.q('.hd-bar-readout').textContent.replace(/\s/g, ''), '80%'); assert.equal(w.q('.hd-widget, .hd-frame') !== null, true);
        assert.equal(w.h.el.getAttribute('data-hd-tone'), 'warn'); w.set('primary', view({ value: 95 })); assert.equal(w.h.el.getAttribute('data-hd-tone'), 'crit', 'the alert zone wins at its edge and above'); w.set('primary', view({ value: 20 })); assert.equal(w.h.el.getAttribute('data-hd-tone'), null);
        const t = mount('gauge', 'tank', { primary: view({ value: 42 }) }, { size: { w: 1, h: 2 } }); assert.equal(t.q('.hd-bar').getAttribute('data-hd-variant'), 'v'); assert.equal(t.q('.hd-bar').getAttribute('data-hd-shape'), 'tank'); assert.equal(t.q('.hd-bar').style.getPropertyValue('--hd-value'), '0.4200'); }],
    ['gauge bar: options.min and max set the span; no value and a dead entity read the dash and an empty bar', () => {
        const w = mount('gauge', 'bar', { primary: view({ value: 15, unit: 'W' }) }, { widget: { options: { min: 10, max: 20 } } }); assert.equal(w.q('.hd-bar').style.getPropertyValue('--hd-value'), '0.5000'); assert.equal(w.q('.hd-bar').getAttribute('aria-valuemin'), '10');
        w.set('primary', view({ value: null })); assert.equal(w.q('.hd-bar').getAttribute('data-hd-empty'), 'true'); assert.equal(w.q('.hd-bar').style.getPropertyValue('--hd-value'), '0.0000');
        w.set('primary', view({ available: false, status: 'unavailable', value: 15 })); assert.equal(w.q('.hd-bar').getAttribute('data-hd-empty'), 'true', 'a dead entity shows no old value'); }],
    ['temperature thermometer: -10..40 C (14..104 F), the degree sign, and the min and max marks follow the history unless show_minmax is off', async () => {
        const pts = [[1, 18], [2, 24], [3, 21]], w = mount('temperature', 'thermometer', { primary: view({ kind: 'sensor', value: 20, unit: DEG + 'C', device_class: 'temperature' }) }, { points: pts, size: { w: 1, h: 2 } });
        await Promise.resolve(); await Promise.resolve(); const bar = w.q('.hd-bar'); assert.equal(bar.getAttribute('data-hd-shape'), 'thermo'); assert.ok(w.q('.hd-bar-bulb'));
        assert.equal(Number(bar.style.getPropertyValue('--hd-value')), 0.6); const marks = w.all('.hd-bar-mark'); assert.equal(marks.length, 2); eq(marks.map(m => m.hasAttribute('hidden')), [false, false]);
        eq(marks.map(m => Number(m.style.getPropertyValue('--hd-at'))), [0.56, 0.68]); assert.match(w.q('.hd-bar-readout').textContent, /20/); assert.equal(w.q('.hd-num-deg').textContent, DEG);
        w.set('primary', view({ value: 68, unit: DEG + 'F', device_class: 'temperature' })); assert.equal(Number(bar.style.getPropertyValue('--hd-value')), 0.6, '68 F is 20 C on the Fahrenheit scale');
        const off = mount('temperature', 'thermometer', { primary: view({ value: 20, unit: DEG + 'C' }) }, { points: pts, widget: { options: { show_minmax: false } } }); assert.equal(off.all('.hd-bar-mark').length, 0); }],
    ['temperature comfort: the dot is in the zone for 21.4 C and 48 %, outside for a hot room; without a humidity sensor the plot is the temperature band', () => {
        const w = mount('temperature', 'comfort', { primary: view({ value: 21.4, unit: DEG + 'C' }), humidity: view({ id: 'x:2', value: 48, unit: '%' }) }), c = w.q('.hd-comfort'), plot = w.q('.hd-cf-plot');
        assert.equal(c.getAttribute('data-hd-zone'), 'in'); assert.equal(c.getAttribute('data-hd-nohum'), null); assert.equal(plot.style.getPropertyValue('--hd-cy'), '0.4667'); assert.equal(plot.style.getPropertyValue('--hd-cx'), '0.4700');
        assert.equal(plot.style.getPropertyValue('--hd-zx0'), '0.4000'); assert.equal(plot.style.getPropertyValue('--hd-zx1'), '0.7000'); assert.equal(plot.style.getPropertyValue('--hd-zy0'), '0.3333'); assert.equal(plot.style.getPropertyValue('--hd-zy1'), '0.6667');
        assert.match(w.q('.hd-comfort .hd-sr-only').textContent, /21\.4/); assert.match(w.q('.hd-comfort .hd-sr-only').textContent, /48/); assert.ok(w.q('.hd-comfort .hd-sr-only').textContent.endsWith(EN['homedash.comfort.inside']), 'the reading says inside the zone');
        w.set('primary', view({ value: 29, unit: DEG + 'C' })); assert.equal(c.getAttribute('data-hd-zone'), 'out'); assert.ok(w.q('.hd-comfort .hd-sr-only').textContent.endsWith(EN['homedash.comfort.outside']), 'and outside it'); w.set('primary', view({ value: 22, unit: DEG + 'C' })); w.set('humidity', view({ id: 'x:2', value: 80, unit: '%' })); assert.equal(c.getAttribute('data-hd-zone'), 'out', 'too humid');
        const bare = mount('temperature', 'comfort', { primary: view({ value: 22, unit: DEG + 'C' }) }); assert.equal(bare.q('.hd-comfort').getAttribute('data-hd-nohum'), 'true'); assert.equal(bare.q('.hd-comfort').getAttribute('data-hd-zone'), 'in');
        bare.set('primary', view({ value: null })); assert.equal(bare.q('.hd-comfort').getAttribute('data-hd-empty'), 'true'); assert.equal(bare.q('.hd-comfort').getAttribute('data-hd-zone'), null); }],
    ['the temperature widget binds the humidity sensor as well as the primary one (the catalog lists both)', () => {
        const { E } = loadEngine(ENGINE_ORDER, { dom: true }); eq(E.widgets.get('temperature').bindings, ['primary', 'humidity']); }],
    ['sensor icon: the entity icon hint and the value, the unit and a screen-reader reading', () => {
        const w = mount('sensor', 'icon', { primary: view({ value: 412, unit: 'W', icon_hint: 'plug' }) }, { size: { w: 1, h: 1 } });
        assert.equal(w.q('.hd-sicon > .hd-iconbadge svg').getAttribute('data-hd-icon'), 'plug'); assert.equal(w.q('.hd-sicon .hd-num-int').textContent, '412'); assert.equal(w.q('.hd-sicon .hd-num-unit').textContent, 'W'); assert.match(w.q('.hd-sicon .hd-sr-only').textContent, /412/);
        w.set('primary', view({ value: null, available: false, status: 'unavailable' })); assert.equal(w.q('.hd-sicon .hd-numeral').getAttribute('data-hd-empty'), 'true'); }],
    ['climate stepper: keys send set_temperature with phase key inside the entity range and step, the current temperature shows, a limit and a read-only entity send nothing', () => {
        const c = p => view({ kind: 'climate', value: 'heat', unit: DEG + 'C', attributes: Object.assign({ temperature: 21.5, current_temperature: 19.5, min_temp: 21, max_temp: 22, target_temp_step: 0.5, hvac_action: 'heating', updated_at: new Date().toISOString() }, p) });
        const w = mount('climate', 'stepper', { primary: c() }, { size: { w: 2, h: 1 } }), spin = w.q('.hd-step-value'), click = { type: 'click', preventDefault() {} };
        assert.equal(spin.getAttribute('aria-valuemin'), '21'); assert.equal(spin.getAttribute('aria-valuemax'), '22'); assert.equal(w.q('.hd-stepper .hd-num-int').textContent, '21'); assert.equal(w.q('.hd-cnow .hd-num-int').textContent, '19');
        assert.equal(w.q('.hd-stepper').getAttribute('aria-label'), 'Title'); assert.match(w.q('.hd-state-text').textContent, /Heat/); assert.equal(w.q('.hd-state-text').textContent.includes('19.5'), true);
        w.q('.hd-step[data-dir="1"]').dispatchEvent(click); eq(w.calls, [['primary', 'set_temperature', { value: 22 }, { phase: 'key' }]]);
        w.set('primary', c({ temperature: 22 })); assert.equal(w.q('.hd-step[data-dir="1"]').getAttribute('aria-disabled'), 'true', 'at the top of the range'); w.q('.hd-step[data-dir="1"]').dispatchEvent(click); assert.equal(w.calls.length, 1, 'a limit sends nothing');
        w.set('primary', Object.assign(c(), { readonly: true })); w.q('.hd-step[data-dir="-1"]').dispatchEvent(click); assert.equal(w.calls.length, 1, 'a read-only entity sends nothing'); }],
    ['climate scale: a vertical slider over the entity range with the target on top, a marker for the current temperature and the dial-free layout', () => {
        const w = mount('climate', 'scale', { primary: view({ kind: 'climate', value: 'cool', unit: DEG + 'C', attributes: { temperature: 20, current_temperature: 25, min_temp: 10, max_temp: 30 } }) }, { size: { w: 1, h: 3 } }), slider = w.q('.hd-cscale .hd-slider');
        assert.equal(slider.getAttribute('data-hd-variant'), 'v'); assert.equal(slider.getAttribute('aria-valuemin'), '10'); assert.equal(slider.getAttribute('aria-valuemax'), '30'); assert.equal(slider.getAttribute('aria-valuenow'), '20');
        assert.equal(slider.getAttribute('aria-label'), 'Title: ' + EN['homedash.climate.target']); assert.equal(w.q('.hd-cscale > .hd-numeral .hd-num-int').textContent, '20');
        const mark = w.q('.hd-cscale-now'); assert.equal(mark.hasAttribute('hidden'), false); assert.equal(mark.style.getPropertyValue('--hd-cur'), '0.7500'); assert.equal(w.q('.hd-cscale').getAttribute('class').includes('hd-lcol'), true);
        w.set('primary', view({ kind: 'climate', value: 'heat', attributes: { temperature: 20 } })); assert.equal(w.q('.hd-cscale-now').hasAttribute('hidden'), true, 'no current temperature: no marker');
        assert.equal(w.q('.hd-cscale .hd-slider').getAttribute('aria-valuemin'), '7', 'no reported range: the platform defaults'); assert.equal(w.q('.hd-body').getAttribute('data-hd-action'), 'idle'); }],
    ['climate stepper and scale: the capability gates the controls (blocked word), the dial rules are kept for the dial only', () => {
        const w = mount('climate', 'stepper', { primary: view({ kind: 'climate', value: 'heat', attributes: { temperature: 20 } }) }, { can: () => false });
        assert.equal(w.q('.hd-step').getAttribute('aria-disabled'), 'true'); assert.equal(w.q('.hd-state-text').textContent, EN['homedash.state.blocked']);
        const css = read('ui/css/homedash-widgets.css'); assert.equal(/\.hd-widget\[data-hd-type="climate"\] \.hd-body \{/.test(css), false, 'the dial body rule names the dial variant');
        assert.ok(css.includes('.hd-widget[data-hd-type="climate"]:where([data-hd-variant="dial"]) .hd-body {'), 'zero added specificity, so the family rules keep winning'); }],
    ['select: a segmented bar whose label is cut (ellipsis) is replaced by the same list picker with the same value and name; a roomy card keeps the bar; a resize re-judges', () => {
        const { E, document } = loadEngine(ENGINE_ORDER, { dom: true }), t = E.core.makeT(EN, { warn() {} });
        const opts = ['Low', 'Medium', 'High'], e = view({ kind: 'select', name: 'Mode', value: 'Medium', unit: '', attributes: { options: opts }, capabilities: [{ name: 'select_option', allowed: true, options: opts }] });
        const api = () => ({ entity: () => e, can: () => true, command: () => Promise.resolve({}), moreInfo() {}, history: () => Promise.resolve({ points: [], source: 'ring' }), announce() {} });
        const h = E.widgets.host({ widget: { id: 'w', type: 'select', variant: 'segmented', title: 'Heater', bindings: { primary: 'x:1' } }, size: { w: 2, h: 1 }, t, api }); document.body.appendChild(h.el);
        assert.ok(h.el.querySelector('.hd-seg'), 'the fake DOM has no layout: the bar stays');
        // a layout: every label reports a width and a narrower client box while the card is 146 px wide
        let narrow = true; const labels = () => Array.from(h.el.querySelectorAll('.hd-seg-label'));
        for (const proto of [Object.getPrototypeOf(h.el)]) { Object.defineProperty(proto, 'clientWidth', { configurable: true, get() { return this.classList && this.classList.contains('hd-seg-label') ? 30 : 0; } });
            Object.defineProperty(proto, 'scrollWidth', { configurable: true, get() { return this.classList && this.classList.contains('hd-seg-label') ? (narrow ? 42 : 30) : 0; } }); }
        h.resize({ w: 2, h: 1, px: { width: 146, height: 67 } });
        assert.equal(h.el.querySelector('.hd-seg'), null, 'cut labels: no bar'); const pick = h.el.querySelector('button.hd-pick'); assert.ok(pick); assert.equal(pick.querySelector('.hd-pick-label').textContent, 'Medium'); assert.equal(pick.getAttribute('aria-label'), 'Heater');
        narrow = false; h.resize({ w: 3, h: 1, px: { width: 225, height: 67 } }); assert.ok(h.el.querySelector('.hd-seg'), 'a wider card: the bar is back'); assert.equal(h.el.querySelector('button.hd-pick'), null);
        assert.equal(h.el.querySelector('.hd-seg').getAttribute('aria-label'), 'Heater'); assert.equal(h.el.querySelector('.hd-seg-opt[aria-checked="true"]').getAttribute('data-hd-option'), 'Medium'); }],
    ['css: the picker economy, the bar and the mark rules exist and the one-column mark is an 8px dot', () => {
        const css = read('ui/css/homedash-widgets.css'), prim = read('ui/css/homedash-primitives.css');
        assert.ok(/\.hd-mark \{[^}]*inline-size: 8px; block-size: 8px;/.test(css.replace(/\s+/g, ' ')), 'the dot is 8px'); assert.ok(css.includes('.hd-widget[data-hd-type="select"] .hd-pick > .hd-ico { display: none; }'));
        assert.ok(prim.includes('.hd-bar-fill {') && prim.includes('.hd-bar-track {')); }],
];
