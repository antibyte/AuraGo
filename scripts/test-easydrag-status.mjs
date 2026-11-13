#!/usr/bin/env node
// W1a A (FD-08): the status map. Every value has a token, a glyph, a word key and an aria key (in all 16 locales);
// of() maps today's raw run, step, effect and flow values and sends an unknown one to "uncertain", never to "ok";
// disc() carries role="img" and the aria text; label() is the disc plus the word.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';

const apps = new URL('../ui/js/desktop/apps/', import.meta.url);
const box = vm.createContext({ window: {}, console });
vm.runInContext(fs.readFileSync(new URL('easydrag-status.js', apps), 'utf8'), box, { filename: 'easydrag-status.js' });
const S = box.window.EasyDrag.status;

const VALUES = ['ok', 'stopped', 'failed', 'running', 'waiting', 'uncertain', 'blocked', 'skipped', 'paused', 'simulated', 'not_run', 'cancelled'];
assert.deepEqual(Array.from(S.VALUES), VALUES, 'the twelve values in contract order');

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];
const words = lang => JSON.parse(fs.readFileSync(new URL('../../../lang/easydrag/' + lang + '.json', apps), 'utf8'));
const TOKENS = new Set(['--ed-ok', '--ed-danger', '--ed-info', '--ed-warn', '--ed-status-neutral']);
for (const value of VALUES) {
    const row = S.table[value];
    assert.ok(row, value + ' has a table row');
    assert.ok(TOKENS.has(row.token), value + ' has a known token: ' + row.token);
    assert.ok(row.glyph.startsWith('<'), value + ' has glyph markup');
    assert.equal(row.word, 'easydrag.status.' + value);
    assert.equal(row.aria, 'easydrag.status.' + value + '_aria');
}
assert.deepEqual(['ok', 'stopped'].map(v => S.table[v].token), ['--ed-ok', '--ed-ok'], 'done is green');
assert.equal(S.table.failed.token, '--ed-danger');
assert.deepEqual(['running', 'waiting'].map(v => S.table[v].token), ['--ed-info', '--ed-info'], 'under way is blue');
assert.deepEqual(['uncertain', 'blocked'].map(v => S.table[v].token), ['--ed-warn', '--ed-warn'], 'look here is amber');
assert.deepEqual(['skipped', 'paused', 'simulated', 'not_run', 'cancelled'].map(v => S.table[v].token), Array(5).fill('--ed-status-neutral'), 'did not happen is grey');

for (const lang of LOCALES) {
    const w = words(lang);
    for (const value of VALUES) {
        for (const key of [S.table[value].word, S.table[value].aria]) assert.ok(typeof w[key] === 'string' && w[key].trim(), lang + ' has ' + key);
    }
    assert.equal(w['easydrag.ui.port_uncertain'], w['easydrag.status.uncertain'], lang + ': one word for unclear');
    assert.equal(new Set(VALUES.map(v => w['easydrag.status.' + v])).size, VALUES.length, lang + ': twelve different words');
}

// of(): today's raw values
const cases = [
    ['success', 'run', 'ok'], ['error', 'run', 'failed'], ['running', 'run', 'running'], ['queued', 'run', 'waiting'], ['waiting', 'run', 'waiting'], ['cancelled', 'run', 'cancelled'],
    ['success', 'step', 'ok'], ['error', 'step', 'failed'], ['running', 'step', 'running'], ['pending', 'step', 'not_run'], ['skipped', 'step', 'skipped'], ['waiting', 'step', 'waiting'],
    ['blocked', 'run', 'blocked'], ['approval_pending', 'run', 'blocked'], ['needs_setup', 'step', 'blocked'], ['simulated', 'step', 'simulated'],
    ['effect_uncertain', 'effect', 'uncertain'], ['uncertain', 'effect', 'uncertain'], [true, 'effect', 'uncertain'], [false, 'effect', 'ok'],
    ['paused', 'flow', 'paused'], ['disabled', 'flow', 'paused'], ['off', 'flow', 'paused']
];
for (const [raw, kind, want] of cases) assert.equal(S.of(raw, kind), want, kind + ' ' + String(raw));
for (const [raw, kind] of [['bogus', 'run'], ['', 'step'], [undefined, 'run'], [null, 'effect'], ['pending', 'run'], ['constructor', 'step'], ['__proto__', 'flow'], [7, 'step'], ['success', 'flow']]) {
    assert.equal(S.of(raw, kind), 'uncertain', 'an unknown raw value is uncertain, never ok: ' + String(raw) + ' as ' + kind);
}
assert.equal(S.ofStep({ status: 'success' }, true), 'simulated', 'a step that ran in a simulation');
assert.equal(S.ofStep({ status: 'error' }, true), 'failed', 'a failed simulated step stays failed');
assert.equal(S.ofStep({ status: 'success' }, false), 'ok');
assert.equal(S.ofStep(null, false), 'uncertain');

// disc(), word(), aria(), label() with the real en texts
const en = words('en');
const t = key => en[key] || key;
const disc = S.disc('ok', 20, { t });
assert.ok(disc.startsWith('<span class="ed-status-disc" data-status="ok" role="img" aria-label="Status: succeeded"'), disc);
assert.ok(disc.includes('--ed-disc:20px') && disc.includes('<svg') && disc.includes('<path'), 'size and glyph');
assert.ok(S.disc('ok', undefined, { t }).includes('--ed-disc:18px'), 'the default size is 18');
assert.ok(S.disc('nonsense', 18, { t }).includes('data-status="uncertain"'), 'an unknown value draws uncertain');
assert.ok(S.disc('ok', 18, { t, title: 'a "b"' }).includes('title="a &quot;b&quot;"'), 'the title is escaped');
assert.ok(S.disc('ok', 18, { t, hidden: true }).includes('aria-hidden="true"') && S.disc('ok', 18, { t, hidden: true }).includes('role="img"'));
assert.equal(S.word(t, 'stopped'), 'Ended by stop');
assert.equal(S.aria(t, 'cancelled'), 'Status: cancelled on request, not a failure');
assert.equal(S.word(key => key, 'failed'), 'failed', 'a missing text falls back to the value name');
const label = S.label(t, 'failed');
assert.ok(label.startsWith('<span class="ed-status-label" data-status="failed">') && label.includes('<span class="ed-status-word">Failed</span>') && label.includes('aria-hidden="true"'), label);
assert.ok(S.label(t, 'ok', '<b>x</b>').includes('&lt;b&gt;x&lt;/b&gt;'), 'a word override is escaped');
S.bind(t);
assert.ok(S.disc('waiting', 14).includes('aria-label="Status: waiting"'), 'a bound translator serves disc() without t');
console.log('PASS easydrag status map');
