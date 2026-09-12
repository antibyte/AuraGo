#!/usr/bin/env node
// Runs the pure Mission Control schedule module in Node and checks parse/build/describe/validate.
import fs from 'node:fs';
import path from 'node:path';
import vm from 'node:vm';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const file = path.join(here, '..', 'ui', 'js', 'desktop', 'apps', 'mission-control-schedule.js');
const sandbox = { window: {}, Intl, Number, String, Array, Object, Math, Set, RegExp, Date };
vm.runInNewContext(fs.readFileSync(file, 'utf8'), sandbox, { filename: file });
const S = sandbox.window.MissionControlSchedule;
if (!S) throw new Error('window.MissionControlSchedule not exported');

let failures = 0;
function check(name, cond, detail) {
    if (cond) { console.log('ok   ' + name); return; }
    failures++;
    console.log('FAIL ' + name + (detail ? ' — ' + detail : ''));
}
function eq(name, got, want) { check(name, JSON.stringify(got) === JSON.stringify(want), `got ${JSON.stringify(got)} want ${JSON.stringify(want)}`); }

// Fake translator that interpolates {{x}}.
const t = (key, params) => key.replace(/^desktop\.mc_schedule_/, '') + (params ? ' ' + Object.entries(params).map(([k, v]) => `${k}=${v}`).join(' ') : '');

// parse → mode
eq('parse every 5 min', S.parse('*/5 * * * *').mode, 'minutes');
eq('parse every 5 min value', S.parse('*/5 * * * *').every, 5);
eq('parse every 6 hours', S.parse('0 */6 * * *').mode, 'hours');
eq('parse hourly', S.parse('0 * * * *').mode, 'hourly');
eq('parse hourly at 30', S.parse('30 * * * *').minute, 30);
eq('parse daily', S.parse('0 9 * * *').mode, 'daily');
eq('parse daily hour', S.parse('15 7 * * *').hour, 7);
eq('parse weekly', S.parse('0 9 * * 1').mode, 'weekly');
eq('parse weekly days', S.parse('0 9 * * 1,3,5').weekdays, [1, 3, 5]);
eq('parse weekly sunday 7→0', S.parse('0 9 * * 7,1').weekdays, [0, 1]);
eq('parse monthly', S.parse('0 0 1 * *').mode, 'monthly');
eq('parse monthly day', S.parse('30 8 15 * *').dayOfMonth, 15);
eq('parse custom (month)', S.parse('0 9 1 6 *').mode, 'custom');
eq('parse custom (6 fields)', S.parse('0 0 9 * * 1').mode, 'custom');
eq('parse custom (descriptor)', S.parse('@daily').mode, 'custom');
eq('parse custom keeps raw', S.parse('@daily').cron, '@daily');
eq('parse empty → default daily 09:00', S.build(S.parse('')), '0 9 * * *');

// all nine presets are representable
for (const p of S.PRESETS) check('preset supported ' + p.value, S.isSupported(p.value));
eq('quick presets', S.QUICK_PRESETS, ['0 * * * *', '0 9 * * *', '0 9 * * 1', '0 0 1 * *']);

// build
eq('build minutes', S.build({ mode: 'minutes', every: 15 }), '*/15 * * * *');
eq('build minutes invalid step → 15', S.build({ mode: 'minutes', every: 7 }), '*/15 * * * *');
eq('build hours', S.build({ mode: 'hours', every: 6 }), '0 */6 * * *');
eq('build hourly', S.build({ mode: 'hourly', minute: 45 }), '45 * * * *');
eq('build daily', S.build({ mode: 'daily', hour: 18, minute: 5 }), '5 18 * * *');
eq('build weekly', S.build({ mode: 'weekly', hour: 9, minute: 0, weekdays: [5, 1] }), '0 9 * * 1,5');
eq('build weekly no days → monday', S.build({ mode: 'weekly', hour: 9, minute: 0, weekdays: [] }), '0 9 * * 1');
eq('build monthly', S.build({ mode: 'monthly', hour: 0, minute: 0, dayOfMonth: 31 }), '0 0 31 * *');
eq('build monthly clamps day', S.build({ mode: 'monthly', hour: 0, minute: 0, dayOfMonth: 45 }), '0 0 31 * *');
eq('build custom passthrough', S.build({ mode: 'custom', cron: ' 0 0 9 * * 1 ' }), '0 0 9 * * 1');
eq('build clamps hour', S.build({ mode: 'daily', hour: 99, minute: -3 }), '0 23 * * *');

// round trips
for (const expr of ['*/30 * * * *', '0 */12 * * *', '10 * * * *', '0 9 * * *', '0 9 * * 1,5', '0 0 1 * *']) eq('roundtrip ' + expr, S.build(S.parse(expr)), expr);

// validate
for (const ok of ['0 9 * * *', '*/5 * * * *', '0 0 9 * * 1', '@daily', '@hourly', '@every 1h30m', '0 9 * * MON-FRI']) check('valid ' + ok, S.validate(ok));
for (const bad of ['', '0 9 * *', '0 9 * * * * *', '@sometimes', 'foo bar baz qux quux;', '0 9 * * *;drop']) check('invalid ' + JSON.stringify(bad), !S.validate(bad));

// describe
eq('describe minutes', S.describe('*/15 * * * *', t, 'en'), 'every_minutes count=15');
eq('describe hours', S.describe('0 */6 * * *', t, 'en'), 'every_hours count=6');
eq('describe hourly', S.describe('5 * * * *', t, 'en'), 'hourly_at minute=05');
check('describe daily has time', /^daily_at time=.*9.*00/.test(S.describe('0 9 * * *', t, 'en')), S.describe('0 9 * * *', t, 'en'));
check('describe weekly names days', /^weekly_at days=Mon, Fri time=/.test(S.describe('0 9 * * 1,5', t, 'en')), S.describe('0 9 * * 1,5', t, 'en'));
check('describe all seven days → daily', S.describe('0 9 * * 0,1,2,3,4,5,6', t, 'en').startsWith('daily_at'));
check('describe monthly', S.describe('0 0 1 * *', t, 'en').startsWith('monthly_at day=1 time='));
eq('describe @daily', S.describe('@daily', t, 'en'), 'descriptor_daily');
eq('describe @midnight', S.describe('@midnight', t, 'en'), 'descriptor_daily');
eq('describe @every', S.describe('@every 2h', t, 'en'), 'every_duration duration=2h');
eq('describe custom', S.describe('0 9 1 6 *', t, 'en'), 'custom_desc');
eq('describe empty', S.describe('', t, 'en'), '');
eq('describe german time', S.describe('30 14 * * *', t, 'de'), 'daily_at time=14:30');

if (failures) { console.error(`${failures} check(s) failed`); process.exit(1); }
console.log('all schedule checks passed');
