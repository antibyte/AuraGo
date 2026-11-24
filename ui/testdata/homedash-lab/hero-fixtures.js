/* HomeDash M1 style lab: hero fixtures and scenarios (hdLab.fixtures.define), look application, focus marking for widget
 * cells and the M1 extra probes m1-tabular and m1-svgcount (hdLab.report). Test-only; loaded through a sheet's "scripts". */
(function () {
  'use strict';
  const lab = window.hdLab;
  if (!lab || !lab.fixtures || typeof lab.fixtures.define !== 'function' || typeof lab.report !== 'function') {
    throw new Error('hero-fixtures.js needs window.hdLab with fixtures.define and report (M0 C4)');
  }
  const NOW = Date.UTC(2026, 10, 23, 18, 0, 0), MIN = 60000;
  const iso = (ms) => new Date(ms).toISOString();
  function merge(base, over) { const o = over || {}; return Object.assign({}, base, o, { attributes: Object.assign({}, base.attributes, o.attributes) }); }
  function entity(id, kind, name, extra) {
    return merge({ id, kind, name, area: 'Living room', unit: '', device_class: '', value: null, attributes: { updated_at: iso(NOW - 2 * MIN) },
      available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, extra);
  }
  const STALE = { stale: true, attributes: { updated_at: iso(NOW - 14 * MIN) } };
  // Every lab state resolves for every hero, so sheets that mix heroes never fall back to the built-in labView().
  function states(on, off, extra) {
    return Object.assign({ off, on, focus: on, min: off, max: on, pending: merge(on, { pending: true }), disabled: merge(on, { capabilities: [] }),
      unavailable: merge(off, { available: false, status: 'unavailable', value: null }), stale: merge(on, STALE), readonly: merge(on, { readonly: true }),
      error: merge(off, { status: 'error', error: { code: 'timeout' } }), alert: merge(on, { tone: 'crit' }) }, extra || {});
  }
  function displayStates(normal, extra) {
    return states(normal, normal, Object.assign({ pending: normal, disabled: normal, error: merge(normal, { status: 'unknown', value: null }) }, extra));
  }
  function series(base, amp, ripple, gapAt, last3) {
    const points = [];
    for (let i = 0; i < 96; i++) {
      const day = Math.sin((i / 96) * Math.PI * 2 - 1.9), r = Math.sin(i * 0.9) * 0.35 + Math.sin(i * 0.37 + 1) * 0.65;
      const gap = gapAt >= 0 && i >= gapAt && i < gapAt + 3;
      points.push([NOW - (95 - i) * 15 * MIN, gap ? null : Math.round((base + amp * day + ripple * r) * 10) / 10]);
    }
    last3.forEach((v, k) => { points[93 + k][1] = v; });
    return { points, source: 'ring' };
  }

  const lamp = entity('ha:switch.desk_lamp', 'switch', 'Desk lamp', { value: 'on', capabilities: ['toggle'] }), lampOff = merge(lamp, { value: 'off' });
  lab.fixtures.define('switch', 'pill', { widget: { title: 'Desk lamp' }, states: states(lamp, lampOff),
    scenarios: { toggle: { from: lampOff, to: lamp }, pending: { from: lamp, to: merge(lamp, { pending: true }) } } });
  const heater = entity('ha:switch.fan_heater', 'switch', 'Fan heater', { value: 'on', capabilities: ['toggle'] }), heaterOff = merge(heater, { value: 'off' });
  lab.fixtures.define('switch', 'rocker', { widget: { title: 'Fan heater' }, states: states(heater, heaterOff),
    scenarios: { toggle: { from: heaterOff, to: heater }, pending: { from: heater, to: merge(heater, { pending: true }) } } });

  const light = entity('ha:light.reading', 'light', 'Reading light', { value: 'on', attributes: { brightness: 163 }, capabilities: ['toggle', 'brightness'] });
  const lightOff = merge(light, { value: 'off', attributes: { brightness: null } });
  lab.fixtures.define('light', 'dial', { widget: { title: 'Reading light' },
    states: states(light, lightOff, { min: merge(light, { attributes: { brightness: 3 } }), max: merge(light, { attributes: { brightness: 255 } }) }),
    scenarios: { remote: { from: merge(light, { attributes: { brightness: 51 } }), to: merge(light, { attributes: { brightness: 204 } }) },
      pending: { from: light, to: merge(light, { pending: true }) } } });

  const clim = entity('ha:climate.living', 'climate', 'Living room', { value: 'heat', capabilities: ['set_temperature', 'set_hvac_mode'],
    attributes: { current_temperature: 19.5, temperature: 21.5, min_temp: 5, max_temp: 30, target_temp_step: 0.5, hvac_action: 'heating', hvac_modes: ['off', 'heat', 'cool', 'auto'] } });
  lab.fixtures.define('climate', 'dial', { widget: { title: 'Living room' },
    states: states(clim, merge(clim, { value: 'off', attributes: { hvac_action: 'off' } }), {
      min: merge(clim, { attributes: { temperature: 5, hvac_action: 'idle' } }), max: merge(clim, { attributes: { temperature: 30 } }),
      alert: merge(clim, { attributes: { current_temperature: 12 }, tone: 'crit' }),
      // readonly doubles as the cooling sample (cool mode, target 22, current 24.5)
      readonly: merge(clim, { readonly: true, value: 'cool', attributes: { temperature: 22, current_temperature: 24.5, hvac_action: 'cooling' } }) }),
    scenarios: { target: { from: merge(clim, { attributes: { temperature: 19 } }), to: merge(clim, { attributes: { temperature: 23 } }) },
      pending: { from: clim, to: merge(clim, { pending: true }) } } });

  const cpu = entity('system:cpu_load', 'sensor', 'CPU load', { value: 42, unit: '%' });
  lab.fixtures.define('gauge', 'needle', { widget: { title: 'CPU load', icon: 'cpu', options: { min: 0, max: 100, zones: [{ from: 70, to: 85, tone: 'warn' }, { from: 85, to: 100, tone: 'crit' }] } },
    states: displayStates(cpu, { min: merge(cpu, { value: 0 }), max: merge(cpu, { value: 100 }), alert: merge(cpu, { value: 91 }) }),
    scenarios: { value: { from: merge(cpu, { value: 20 }), to: merge(cpu, { value: 80 }) } } });
  const disk = entity('system:disk_data', 'sensor', 'Disk /data', { value: 64, unit: '%' });
  lab.fixtures.define('gauge', 'arc', { widget: { title: 'Disk /data', icon: 'disk', options: { min: 0, max: 100, zones: [{ from: 80, to: 92, tone: 'warn' }, { from: 92, to: 100, tone: 'crit' }] } },
    states: displayStates(disk, { min: merge(disk, { value: 0 }), max: merge(disk, { value: 100 }), alert: merge(disk, { value: 95 }) }),
    scenarios: { value: { from: merge(disk, { value: 20 }), to: merge(disk, { value: 85 }) } } });

  const temp = entity('ha:sensor.living_temperature', 'sensor', 'Living room', { value: 21.4, unit: String.fromCodePoint(0xb0) + 'C', device_class: 'temperature' });
  lab.fixtures.define('temperature', 'numeral', { widget: { title: 'Living room', icon: 'thermometer' },
    states: displayStates(temp, { min: merge(temp, { value: -12.5 }), max: merge(temp, { value: 38.9 }), alert: merge(temp, { value: 31.6, tone: 'crit' }) }),
    history: { default: series(21.2, 1.4, 0.3, -1, [21.9, 21.65, 21.4]), min: series(-10.5, 2.5, 0.4, -1, [-12.0, -12.3, -12.5]), max: series(35.5, 2.4, 0.4, -1, [38.1, 38.6, 38.9]) },
    scenarios: { value: { from: temp, to: merge(temp, { value: 21.9 }) } } });
  const power = entity('ha:sensor.kitchen_power', 'sensor', 'Kitchen power', { value: 412, unit: 'W', device_class: 'power' });
  lab.fixtures.define('sensor', '*', { widget: { title: 'Kitchen power', icon: 'plug' },
    states: displayStates(power, { min: merge(power, { value: 0 }), max: merge(power, { value: 2860 }), alert: merge(power, { value: 2480, tone: 'crit' }) }),
    history: { default: series(380, 220, 120, 40, [520, 455, 412]), min: series(60, 50, 30, 40, [12, 4, 0]), max: series(1400, 700, 300, 40, [2410, 2700, 2860]),
      alert: series(1300, 650, 280, 40, [2290, 2400, 2480]) },
    scenarios: { value: { from: power, to: merge(power, { value: 980 }) } } });

  // Look and focus: applyLook once per panel; widget cells in the focus state get the M0 lab focus hook on their role element.
  let looked = false;
  function decorate() {
    const root = document.querySelector('.hd-lab-panel');
    if (!root) return;
    const E = window.HomeDashEngine;
    if (!looked && E && E.widgets && typeof E.widgets.applyLook === 'function') { E.widgets.applyLook(root); looked = true; }
    for (const cell of root.querySelectorAll('.hd-lab-cell[data-hd-lab-state="focus"]')) {
      const target = cell.querySelector('.hd-widget [role="switch"], .hd-widget [role="slider"]');
      if (target && !target.hasAttribute('data-hd-focus')) target.setAttribute('data-hd-focus', 'ring');
    }
  }
  new MutationObserver(decorate).observe(document.documentElement, { childList: true, subtree: true });

  // Extra probes, computed when the Go test asks for probes (after render), then stored with hdLab.report.
  function widthOf(host, stack, features, text) {
    const s = document.createElement('span'); s.textContent = text;
    s.style.setProperty('font-family', stack); s.style.setProperty('font-size', '40px'); s.style.setProperty('font-feature-settings', features || 'normal'); s.style.setProperty('white-space', 'pre');
    host.appendChild(s); const w = s.getBoundingClientRect().width; s.remove(); return w;
  }
  function tabular(root) {
    const out = [];
    for (const family of ['glass', 'tactile', 'minimal', 'instrument']) {
      const host = document.createElement('div'); host.className = 'hd-widget'; host.setAttribute('data-hd-family', family);
      host.style.setProperty('position', 'absolute'); host.style.setProperty('visibility', 'hidden'); host.style.setProperty('inset-inline-start', '-10000px');
      root.appendChild(host);
      const cs = getComputedStyle(host), features = cs.getPropertyValue('--hd-num-features').trim();
      for (const name of ['--hd-font-num', '--hd-font-label', '--hd-font-vfd']) {
        const stack = cs.getPropertyValue(name).trim(); if (!stack) continue;
        const widths = ['1111', '8888', '0000', '4747'].map((t) => widthOf(host, stack, features, t));
        const spread = Math.max.apply(null, widths) - Math.min.apply(null, widths);
        out.push({ family, token: name, stack, loaded: document.fonts.check('40px ' + stack.split(',')[0].trim()), spread, pass: spread <= 0.5 });
      }
      host.remove();
    }
    return out;
  }
  function svgCount(root) {
    return Array.from(root.querySelectorAll('.hd-widget')).map((w) => {
      const n = w.querySelectorAll('svg *').length;
      return { type: w.getAttribute('data-hd-type'), variant: w.getAttribute('data-hd-variant'), family: w.getAttribute('data-hd-family'), nodes: n, pass: n <= 150 };
    });
  }
  function runM1() {
    const root = document.querySelector('.hd-lab-panel'); if (!root) return;
    decorate();
    const tab = tabular(root), svg = svgCount(root);
    lab.report('m1-tabular', { ok: tab.every((r) => r.pass), message: tab.filter((r) => !r.pass).map((r) => r.family + ' ' + r.token + ' spread ' + r.spread.toFixed(2) + 'px').join('; '), results: tab });
    lab.report('m1-svgcount', { ok: svg.every((r) => r.pass), message: svg.filter((r) => !r.pass).map((r) => r.type + '/' + r.variant + ' ' + r.family + ': ' + r.nodes + ' nodes').join('; '), results: svg });
  }
  let inner = typeof lab.probes === 'function' ? lab.probes : null;
  const wrapped = async function () { runM1(); return inner ? inner.apply(this, arguments) : {}; };
  Object.defineProperty(lab, 'probes', { configurable: true, enumerable: true, get() { return wrapped; }, set(fn) { inner = fn; } });
})();
