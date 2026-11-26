import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const load = (dir, loc) => JSON.parse(fs.readFileSync(path.join(ROOT, 'ui', 'lang', dir, loc + '.json'), 'utf8'));
const hd = {}, desk = {};
for (const loc of LOCALES) { hd[loc] = load('homedash', loc); desk[loc] = load('desktop', loc); }

// Values that stay identical in every language (brand names, technical abbreviations and units).
const SAME_VALUES = new Set(['HomeDash', 'CPU', 'Home Assistant']);

// Genuine loanwords: the native UI uses the English word. locale -> keys, with the reason on each line.
const SAME_WORD_KEYS = {
  cs: ['homedash.climate.mode_auto', 'homedash.conn.offline', 'homedash.system.disk', 'homedash.menu.dashboard', 'homedash.icon.dashboard', 'homedash.icon.disk', 'homedash.inspector.tab_data'], // Czech UIs use "Auto" and "Offline" as they are; "Disk" is the Czech word for a disk; the menu title uses the Czech loanword "Dashboard"; part 2: "Dashboard" is the Czech loanword used in the menu; "Disk" and "Data" are Czech words
  da: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back', 'homedash.home.starter_name', 'homedash.source.system', 'homedash.kind.scene', 'homedash.bp.label', 'homedash.menu.dashboard', 'homedash.menu.layout', 'homedash.family.instrument', 'homedash.category.layout', 'homedash.category.status', 'homedash.icon.cpu', 'homedash.icon.disk', 'homedash.icon.download', 'homedash.icon.scene', 'homedash.icon.timer', 'homedash.icon.upload', 'homedash.inspector.enum_start', 'homedash.inspector.multi_title', 'homedash.inspector.tab_data', 'homedash.kind.printer', 'homedash.kind.sensor', 'homedash.look.accent', 'homedash.palette.tab_widgets'], // Danish: Auto, Live, Offline, Dashboards, System; Scene is the native word and the Home Assistant term; "Layout", "Dashboard" and "Instrument" are the Danish words; part 2: "Layout", "Status", "Processor", "Disk", "Scene", "Start", "Data", "Printer", "Sensor" and "Accent" are Danish words; "Download", "Upload" and "Timer" are used as they are in Danish UIs; "widgets" is the Danish UI term
  de: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back', 'homedash.home.starter_name', 'homedash.source.system', 'homedash.family.minimal', 'homedash.family.instrument', 'homedash.dialog.name', 'homedash.icon.dashboard', 'homedash.icon.download', 'homedash.icon.thermometer', 'homedash.icon.timer', 'homedash.icon.upload', 'homedash.kind.sensor', 'homedash.palette.tab_widgets'], // German: Auto, Live, Offline, Dashboards, System; the German UI uses the same word; part 2: "Name", "Thermometer" and "Sensor" are the German words; "Widgets" is the German UI term; "Dashboard", "Download", "Upload" and "Timer" are used as they are in German UIs
  es: ['homedash.climate.mode_auto', 'homedash.kind.script', 'homedash.icon.script', 'homedash.inspector.multi_title', 'homedash.kind.sensor', 'homedash.palette.tab_widgets'], // "Auto" is the usual Spanish thermostat label; "Script" is the Home Assistant term; part 2: "Script" is the Home Assistant term; "Sensor" is the Spanish word; "widgets" is the Spanish UI term
  fr: ['homedash.climate.mode_auto', 'homedash.view.pages', 'homedash.kind.script', 'homedash.designer.canvas', 'homedash.designer.page_n', 'homedash.designer.announce.page', 'homedash.widget.action.name', 'homedash.variant.gauge.arc', 'homedash.variant.select.segmented', 'homedash.variant.media.compact', 'homedash.variant.weather.compact', 'homedash.family.tactile', 'homedash.family.instrument', 'homedash.icon.script', 'homedash.inspector.enum_center', 'homedash.inspector.multi_title', 'homedash.inspector.page_title', 'homedash.inspector.tab_actions', 'homedash.inspector.tab_options', 'homedash.inspector.target_page', 'homedash.inspector.variant', 'homedash.look.accent', 'homedash.look.density_compact', 'homedash.look.surface', 'homedash.palette.filter_source', 'homedash.palette.sample_volume', 'homedash.palette.tab_widgets'], // "Auto" is the usual French thermostat label; "Pages" is the French word too; "Script" is the Home Assistant term; "Page", "Action", "Arc", "Segments", "Compact", "Tactile" and "Instrument" are French words too ("widgets" in the canvas label is the loanword the other strings use); part 2: "Script" is the Home Assistant term; "Centre", "Page", "Actions", "Options", "Style", "Accent", "Compact", "Surface", "Source" and "Volume" are French words too; "widgets" is the French UI term
  it: ['homedash.climate.mode_auto', 'homedash.conn.offline', 'homedash.control.dimmer', 'homedash.kind.script', 'homedash.bp.label', 'homedash.menu.dashboard', 'homedash.menu.layout', 'homedash.variant.temperature.comfort', 'homedash.category.layout', 'homedash.category.media', 'homedash.icon.dashboard', 'homedash.icon.download', 'homedash.icon.script', 'homedash.icon.timer', 'homedash.icon.upload', 'homedash.palette.filter_area', 'homedash.palette.sample_volume'], // Italian apps use "Auto" and "Offline"; "dimmer" is the everyday word; "Script" is the Home Assistant term; "Layout", "Dashboard" and "Comfort" are used as they are in Italian; part 2: "Layout" and "Dashboard" are used as they are in Italian; "Media", "Area" and "Volume" are Italian words; "Download", "Upload" and "Timer" are used as they are in Italian UIs; "Script" is the Home Assistant term
  nl: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back', 'homedash.control.effect', 'homedash.control.dimmer', 'homedash.kind.script', 'homedash.menu.dashboard', 'homedash.widget.camera.name', 'homedash.variant.temperature.comfort', 'homedash.variant.binary.lamp', 'homedash.variant.media.compact', 'homedash.variant.weather.compact', 'homedash.family.instrument', 'homedash.category.media', 'homedash.category.status', 'homedash.icon.cpu', 'homedash.icon.dashboard', 'homedash.icon.download', 'homedash.icon.effect', 'homedash.icon.script', 'homedash.icon.thermometer', 'homedash.icon.timer', 'homedash.icon.upload', 'homedash.inspector.multi_title', 'homedash.kind.camera', 'homedash.kind.printer', 'homedash.kind.sensor', 'homedash.kind.update', 'homedash.look.accent', 'homedash.look.density_compact', 'homedash.palette.sample_volume', 'homedash.palette.tab_widgets'], // Dutch: Auto, Live, Offline, Dashboards; "Effect", "dimmer" and "Script" are used as they are; "Dashboard", "Camera", "Comfort", "Lamp", "Compact" and "Instrument" are Dutch words; part 2: "Media", "Status", "Processor", "Effect", "Thermometer", "Camera", "Printer", "Sensor", "Accent", "Compact" and "Volume" are Dutch words; "Dashboard", "Download", "Upload", "Timer" and "Update" are used as they are in Dutch UIs; "Script" is the Home Assistant term; "widgets" is the Dutch UI term
  no: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.home.starter_name', 'homedash.source.system', 'homedash.control.dimmer', 'homedash.kind.scene', 'homedash.family.glass', 'homedash.family.instrument', 'homedash.category.status', 'homedash.icon.disk', 'homedash.icon.scene', 'homedash.inspector.enum_start', 'homedash.inspector.tab_data', 'homedash.kind.sensor'], // Norwegian: Auto, Live, System; "dimmer" is the everyday word; "Scene" is the native word and the Home Assistant term; "Glass" and "Instrument" are the Norwegian words; part 2: "Status", "Disk", "Start", "Data" and "Sensor" are Norwegian words; "Scene" is the native word and the Home Assistant term
  pl: ['homedash.climate.mode_auto', 'homedash.conn.offline', 'homedash.home.starter_name', 'homedash.source.system', 'homedash.look.backdrop_gradient'], // Polish: Auto, Offline, System; part 2: "Gradient" is the Polish word
  pt: ['homedash.climate.mode_auto', 'homedash.conn.offline', 'homedash.control.dimmer', 'homedash.kind.script', 'homedash.bp.label', 'homedash.menu.layout', 'homedash.category.layout', 'homedash.category.status', 'homedash.icon.download', 'homedash.icon.script', 'homedash.icon.upload', 'homedash.inspector.multi_title', 'homedash.kind.sensor', 'homedash.palette.sample_volume', 'homedash.palette.tab_widgets'], // Portuguese apps use "Auto" and "Offline"; "dimmer" is the everyday word; "Script" is the Home Assistant term; "Layout" is the usual Portuguese UI word; part 2: "Layout" and "Status" are the usual Portuguese UI words; "Download" and "Upload" are used as they are; "Script" is the Home Assistant term; "Sensor" and "Volume" are Portuguese words; "widgets" is the Portuguese UI term
  sv: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.starter_name', 'homedash.source.system', 'homedash.control.dimmer', 'homedash.bp.label', 'homedash.menu.layout', 'homedash.family.instrument', 'homedash.category.layout', 'homedash.category.media', 'homedash.category.status', 'homedash.icon.cpu', 'homedash.icon.disk', 'homedash.icon.timer', 'homedash.inspector.tab_data', 'homedash.kind.sensor', 'homedash.look.accent'] // Swedish: Auto, Live, Offline, System; "dimmer" is the everyday word; "Layout" and "Instrument" are the Swedish words; part 2: "Layout", "Media", "Status", "Processor", "Disk", "Timer", "Data", "Sensor" and "Accent" are Swedish words
};

const letters = /\p{L}/u;
const stripped = v => v.replace(/\{\{[^}]*\}\}/g, '');
const exempt = (loc, key, v) => !letters.test(stripped(v)) || SAME_VALUES.has(v) || (SAME_WORD_KEYS[loc] || []).includes(key);

function sameAsEnglish(files, keys) {
  const bad = [];
  for (const loc of LOCALES) {
    if (loc === 'en') continue;
    for (const key of keys) {
      const v = files[loc][key], en = files.en[key];
      if (v === en && !exempt(loc, key, v)) bad.push(loc + ' ' + key + ' = ' + JSON.stringify(v));
    }
  }
  return bad;
}

export const tests = [
  ['i18n: every locale has exactly the English homedash key set', () => {
    const want = Object.keys(hd.en).sort();
    for (const loc of LOCALES) assert.deepEqual(Object.keys(hd[loc]).sort(), want, loc);
  }],
  ['i18n: no non-English homedash value equals the English value (explicit allowlist only)', () => {
    const bad = sameAsEnglish(hd, Object.keys(hd.en));
    assert.deepEqual(bad, [], 'untranslated values:\n' + bad.join('\n'));
  }],
  ['i18n: desktop.app_homedash exists in all locales and is the brand name', () => {
    for (const loc of LOCALES) assert.equal(desk[loc]['desktop.app_homedash'], 'HomeDash', loc);
    assert.deepEqual(sameAsEnglish(desk, ['desktop.app_homedash']), []);
  }],
  ['i18n: allowlisted keys exist and really equal the English value', () => {
    for (const [loc, keys] of Object.entries(SAME_WORD_KEYS)) {
      for (const key of keys) {
        assert.ok(key in hd.en, 'unknown allowlist key ' + key);
        assert.equal(hd[loc][key], hd.en[key], 'stale allowlist entry ' + loc + ' ' + key);
      }
    }
  }],
  ['i18n: German uses informal address and real umlauts', () => {
    for (const [key, v] of Object.entries(hd.de)) {
      assert.ok(!/(^|[\s„“"(])Sie[\s.,;:!?]/.test(v) && !/\bIhr(e|en|er|em|es)?\b/.test(v), 'formal address in ' + key + ': ' + v);
      assert.ok(!/(fuer|ueber|koennen|moechte|waehl|aender|loesch|pruef|zurueck|Geraet)/i.test(v), 'umlaut substitute in ' + key + ': ' + v);
    }
  }]
];
