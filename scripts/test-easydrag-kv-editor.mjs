// a1025 G1/G4 checks (audit 2026-10-25): the key-value editor (HTTP headers, for example) saves
// nothing while two rows share a key, marks those rows aria-invalid with one message linked by
// aria-describedby, and keeps a number or a flag it did not edit instead of turning it into text.
// They run on the c1d07 sandbox (test-easydrag-extra3.mjs returns sandbox); test-easydrag.mjs
// calls run(env) with its helpers and counts the failures.
import fs from 'node:fs';
import path from 'node:path';

const LOCALES = ['cs', 'da', 'de', 'el', 'en', 'es', 'fr', 'hi', 'it', 'ja', 'nl', 'no', 'pl', 'pt', 'sv', 'zh'];

function missingKeys(apps, keys) {
    return LOCALES.flatMap(locale => {
        const words = JSON.parse(fs.readFileSync(path.join(apps, '..', '..', '..', 'lang', 'easydrag', locale + '.json'), 'utf8'));
        return keys.filter(key => typeof words[key] !== 'string' || !words[key].trim()).map(key => locale + ':' + key);
    });
}

export async function run(env) {
    const { apps, t, eq, guardAsync, sandbox } = env;

    await guardAsync('a1025 G1 G4 key-value editor', async () => {
        const h = sandbox(() => undefined);
        const F = h.ED.fields;
        const fieldEnv = { t, esc: h.ED.core.esc, readonly: false, roots: {}, upstream: [], change() {} };
        const changes = [];
        const kv = F.keyValue(fieldEnv, { 'X-A': 'one', Retries: 2, Debug: true }, value => changes.push(value));
        const keys = () => kv.querySelectorAll('[data-ed-kv-key]');
        const error = kv.querySelector('[data-ed-kv-error]');
        const errorId = error.getAttribute('id');
        const marks = () => keys().map(k => [k.getAttribute('aria-invalid'), k.getAttribute('aria-describedby')]);
        const type = (i, text) => { const k = keys()[i]; k.value = text; k.fire('input'); };

        type(1, 'X-A');
        eq('a1025 G1 a duplicate key emits nothing and marks both rows with the linked message',
            [changes.length, marks(), error.hidden, error.textContent],
            [0, [['true', errorId], ['true', errorId], [null, null]], false, 'kv_duplicate_key']);

        type(1, 'Retries');
        eq('a1025 G1 G4 fixing the key emits once, clears the marks and keeps the untouched number and flag',
            [changes.length, changes[0], marks(), error.hidden],
            [1, { 'X-A': 'one', Retries: 2, Debug: true }, [[null, null], [null, null], [null, null]], true]);

        kv.querySelector('[data-ed-kv-add]').fire('click');
        type(3, ' Debug ');
        const blocked = changes.length;
        kv.querySelector('[data-ed-kv-remove="3"]').fire('click');
        eq('a1025 G1 a duplicate typed into a new row blocks saving until the row is removed',
            [blocked, changes.length, changes[changes.length - 1], error.hidden],
            [1, 2, { 'X-A': 'one', Retries: 2, Debug: true }, true]);

        eq('a1025 G1 the message exists in all 16 locales', missingKeys(apps, ['easydrag.ui.kv_duplicate_key']), []);
        eq('a1025 G1 the editor logs no errors', h.logged, []);
    });
}
