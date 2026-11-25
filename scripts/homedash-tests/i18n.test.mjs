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
const SAME_VALUES = new Set(['HomeDash', 'CPU']);

// Genuine loanwords: the native UI uses the English word. locale -> keys, with the reason on each line.
const SAME_WORD_KEYS = {
  cs: ['homedash.climate.mode_auto', 'homedash.conn.offline'], // Czech UIs use "Auto" and "Offline" as they are
  da: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back', 'homedash.home.starter_name', 'homedash.source.system'], // Danish: Auto, Live, Offline, Dashboards, System
  de: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back', 'homedash.home.starter_name', 'homedash.source.system'], // German: Auto, Live, Offline, Dashboards, System
  es: ['homedash.climate.mode_auto'], // "Auto" is the usual Spanish thermostat label
  fr: ['homedash.climate.mode_auto', 'homedash.view.pages'], // "Auto" is the usual French thermostat label; "Pages" is the French word too
  it: ['homedash.climate.mode_auto', 'homedash.conn.offline'], // Italian apps use "Auto" and "Offline"
  nl: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.title', 'homedash.view.back'], // Dutch: Auto, Live, Offline, Dashboards
  no: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.home.starter_name', 'homedash.source.system'], // Norwegian: Auto, Live, System
  pl: ['homedash.climate.mode_auto', 'homedash.conn.offline', 'homedash.home.starter_name', 'homedash.source.system'], // Polish: Auto, Offline, System
  pt: ['homedash.climate.mode_auto', 'homedash.conn.offline'], // Portuguese apps use "Auto" and "Offline"
  sv: ['homedash.climate.mode_auto', 'homedash.conn.live', 'homedash.conn.offline', 'homedash.home.starter_name', 'homedash.source.system'] // Swedish: Auto, Live, Offline, System
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
