/* HomeDash M3 lab fixtures (test-only; loaded through a sheet's "scripts" after hero-fixtures.js): the W1 control variants, the light more-info sheet and the
 * four P1 scenarios (press, confirm, rollback, arm), which are also added to the H1-H3 heroes. Entities follow the contract demo set in
 * ui/testdata/homedash-m3/ha-entities.json (names, units, options, ranges); capabilities are objects (contract M3 section 2).
 * mid = brightness 50 / a value in the middle of the range / option 2; armed = armed {command, seconds}; error keeps the previous value. */
(function () {
  'use strict';
  const lab = window.hdLab;
  if (!lab || !lab.fixtures || typeof lab.fixtures.define !== 'function') throw new Error('w1-fixtures.js needs window.hdLab with fixtures.define (M0 C4)');
  const NOW = Math.floor(Date.now() / 60000) * 60000, MIN = 60000, iso = (ms) => new Date(ms).toISOString();
  const DEG_C = String.fromCodePoint(0xb0) + 'C', RO = 'home_assistant.allowed_services', ERR = { code: 'timeout', text: 'Not confirmed by the device' };
  const STALE = { stale: true, attributes: { updated_at: iso(NOW - 14 * MIN) } };
  function merge(base, over) { const o = over || {}; return Object.assign({}, base, o, { attributes: Object.assign({}, base.attributes, o.attributes) }); }
  const cap = (name, extra) => Object.assign({ name, allowed: true }, extra || {});
  function entity(id, kind, name, extra) {
    return merge({ id, source: 'ha', kind, name, area: '', unit: '', device_class: '', icon_hint: '', value: null, attributes: { updated_at: iso(NOW - 2 * MIN) },
      available: true, status: 'ok', readonly: false, capabilities: [], pending: false, stale: false, error: null, tone: null }, extra);
  }
  // Every lab state resolves for every W1 fixture (sheets that mix states never fall back to the built-in labView()).
  function states(o) {
    const on = o.on, off = o.off, mid = o.mid || on, prev = o.prev || off;
    return { off, on, mid, pending: merge(on, { pending: true }), armed: merge(off, { armed: { command: o.arm, seconds: 3 } }),
      error: merge(prev, { status: 'error', error: ERR }), unavailable: merge(off, { available: false, status: 'unavailable', value: null }),
      readonly: merge(on, { readonly: true, readonlyReason: RO }), stale: merge(on, STALE), focus: on, drag: mid, open: on, min: off, max: on,
      disabled: merge(on, { capabilities: [] }), alert: merge(on, { tone: 'crit' }) };
  }
  function scenarios(s, o) {
    const x = o || {};
    return { press: { from: s.off, to: x.press || s.on }, confirm: { from: s.pending, to: merge(s.on, { confirmed: true }) },
      rollback: { from: s.pending, to: s.error }, arm: { from: s.off, to: s.armed } };
  }
  function define(type, variant, on, off, extra) {
    const x = extra || {}, s = states({ on, off, mid: x.mid, prev: x.prev, arm: x.arm });
    lab.fixtures.define(type, variant, { widget: {}, states: Object.assign(s, x.states || {}), scenarios: scenarios(s, x) });
  }

  // switch (push chip, tile): contract entities ha:switch.coffee and ha:input_boolean.vacation_mode.
  const power = [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('history')];
  const coffee = entity('ha:switch.coffee', 'switch', 'Coffee machine', { area: 'Kitchen', device_class: 'outlet', icon_hint: 'outlet', value: 'on', capabilities: power });
  define('switch', 'push', coffee, merge(coffee, { value: 'off' }), { arm: 'turn_on' });
  const vacation = entity('ha:input_boolean.vacation_mode', 'switch', 'Vacation mode', { icon_hint: 'power', value: 'on', capabilities: power });
  define('switch', 'tile', vacation, merge(vacation, { value: 'off' }), { arm: 'turn_on' });

  // light (slider, fader, tile): ha:light.kitchen; on 80 %, mid 50 %, off without a brightness.
  const kitchenCaps = [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 }), cap('color_temp', { min: 2202, max: 6535, step: 50 }), cap('color'),
    cap('effect', { options: ['none', 'colorloop'] })];
  const kitchenAttrs = { brightness_pct: 80, color_mode: 'color_temp', color_temp_kelvin: 3003, effect: 'none', effect_list: ['none', 'colorloop'], hs_color: [27.2, 56.1],
    min_color_temp_kelvin: 2202, max_color_temp_kelvin: 6535, supported_color_modes: ['color_temp', 'hs'] };
  const kitchen = entity('ha:light.kitchen', 'light', 'Kitchen', { area: 'Kitchen', icon_hint: 'bulb', value: 'on', capabilities: kitchenCaps, attributes: kitchenAttrs });
  const kitchenOff = merge(kitchen, { value: 'off', attributes: { brightness_pct: null, color_mode: null, color_temp_kelvin: null } });
  const kitchenMid = merge(kitchen, { attributes: { brightness_pct: 50 } });
  for (const variant of ['slider', 'fader', 'tile']) define('light', variant, kitchen, kitchenOff, { arm: 'turn_on', mid: kitchenMid, prev: kitchenMid });

  // action: ha:scene.movie. A scene has no on/off: off and on are the idle button, press goes to pending, confirm to the check.
  const movie = entity('ha:scene.movie', 'scene', 'Movie night', { area: 'Living room', icon_hint: 'scene', value: '2026-11-24T20:15:00.000000+00:00', capabilities: [cap('activate')] });
  define('action', 'button', movie, movie, { arm: 'activate', press: merge(movie, { pending: true }) });

  // select: ha:select.heater_mode (3 options, segmented) and ha:input_select.radio (6 options, list).
  const heaterOptions = ['eco', 'comfort', 'boost'], radioOptions = ['Jazz', 'Rock', 'News', 'Classic', 'Pop', 'Lounge'];
  const selectEntity = (id, name, area, options, value) => entity(id, 'select', name, { area, icon_hint: 'list', value, capabilities: [cap('select_option', { options })], attributes: { options } });
  const heater = selectEntity('ha:select.heater_mode', 'Heater mode', 'Kitchen', heaterOptions, 'boost');
  define('select', 'segmented', heater, merge(heater, { value: 'eco' }), { arm: 'select_option', mid: merge(heater, { value: 'comfort' }), prev: merge(heater, { value: 'comfort' }) });
  const radio = selectEntity('ha:input_select.radio', 'Radio', '', radioOptions, 'News');
  define('select', 'list', radio, merge(radio, { value: 'Jazz' }), { arm: 'select_option', mid: merge(radio, { value: 'Rock' }), prev: merge(radio, { value: 'Rock' }) });

  // number: ha:number.boiler_target (stepper, 30-70 step 1, degrees C) and ha:input_number.volume (slider, 0-100 step 5).
  const numberEntity = (id, name, area, c, attrs, unit, value) => entity(id, 'number', name, { area, unit, icon_hint: 'number', value, capabilities: [cap('set_value', c), cap('history')], attributes: attrs });
  const boiler = numberEntity('ha:number.boiler_target', 'Boiler target', '', { min: 30, max: 70, step: 1 }, { min: 30, max: 70, step: 1, mode: 'box', unit_of_measurement: DEG_C }, DEG_C, 65);
  define('number', 'stepper', boiler, merge(boiler, { value: 30 }), { arm: 'set_value', mid: merge(boiler, { value: 50 }), prev: merge(boiler, { value: 50 }) });
  const volume = numberEntity('ha:input_number.volume', 'Volume', 'Living room', { min: 0, max: 100, step: 5 }, { min: 0, max: 100, step: 5, mode: 'slider' }, '', 85);
  define('number', 'slider', volume, merge(volume, { value: 0 }), { arm: 'set_value', mid: merge(volume, { value: 50 }), prev: merge(volume, { value: 50 }) });

  // The light more-info sheet. on = the contract kitchen light (40 %, colour temperature 3003 K); colour = hue/saturation mode; dimonly = brightness only, so the
  // colour temperature, colour and effect rows are hidden.
  const sheetOn = merge(kitchen, { attributes: { brightness_pct: 40 } }), sheetOff = merge(kitchenOff, {});
  const sheetColour = merge(sheetOn, { attributes: { brightness_pct: 60, color_mode: 'hs', color_temp_kelvin: null, hs_color: [210, 70], effect: 'colorloop' } });
  const desk = entity('ha:light.desk', 'light', 'Desk lamp', { area: 'Office', icon_hint: 'bulb', value: 'on', attributes: { brightness_pct: 70, color_mode: 'brightness', supported_color_modes: ['brightness'] },
    capabilities: [cap('toggle'), cap('turn_on'), cap('turn_off'), cap('brightness', { min: 0, max: 100, step: 1 })] });
  const sheetStates = states({ on: sheetOn, off: sheetOff, arm: 'turn_on' });
  lab.fixtures.define('moreinfo', 'light', { widget: {}, states: Object.assign(sheetStates, { colour: sheetColour, dimonly: desk }) });

  // The P1 scenarios of the three M1 switch and light heroes (hero-fixtures.js defines their states; a sheet that loads this file after it gets them).
  for (const [type, variant, command] of [['switch', 'pill', 'turn_on'], ['switch', 'rocker', 'turn_on'], ['light', 'dial', 'turn_on']]) {
    const entry = lab.fixtures.get(type, variant);
    if (!entry || !entry.states || !entry.states.on || !entry.states.off) continue;
    const s = entry.states, set = {
      off: s.off, on: s.on, pending: merge(s.on, { pending: true }), armed: merge(s.off, { armed: { command, seconds: 3 } }), error: merge(s.off, { status: 'error', error: ERR }) };
    entry.scenarios = Object.assign({}, entry.scenarios, scenarios(set));
  }
})();
