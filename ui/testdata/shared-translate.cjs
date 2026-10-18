// Runs the shared translation helpers t() and tOr() from ui/js/shared/shared-core.js in a fresh Node vm context per
// scenario (only the helper source is extracted, the rest of the file has DOM side effects). Checks that parameter
// values are inserted literally even when they contain replacement patterns ($&, $1, $$, $` and $'), that ordinary
// substitution is unchanged ({{x}} and {x} syntax, repeated and unknown placeholders, numbers) and that the
// missing-key and missing-dictionary behaviour is unchanged.
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');

const file = path.join(__dirname, '..', 'js', 'shared', 'shared-core.js');
const source = fs.readFileSync(file, 'utf8');

function sourceBetween(text, startMarker, endMarker) {
    const start = text.indexOf(startMarker);
    const end = text.indexOf(endMarker, start + startMarker.length);
    assert.notEqual(start, -1, 'missing source marker ' + startMarker);
    assert.notEqual(end, -1, 'missing source marker ' + endMarker);
    return text.slice(start, end);
}

const helperSource = sourceBetween(source, 'function t(k, p)', 'let _sharedModalOverlay = null;');

// Builds t/tOr in a fresh context. Pass undefined to leave I18N undefined altogether.
function load(dict) {
    const ctx = vm.createContext(dict === undefined ? {} : { I18N: dict });
    vm.runInContext(helperSource, ctx);
    assert.equal(typeof ctx.t, 'function', 't must be defined by the helper source');
    assert.equal(typeof ctx.tOr, 'function', 'tOr must be defined by the helper source');
    return { t: ctx.t, tOr: ctx.tOr };
}

// Values that String.prototype.replaceAll expands when they are passed as a replacement string.
const tricky = {
    amp: 'a $& b',
    group: 'x $1 y',
    dollars: 'p $$ q',
    before: 'm $` n',
    after: "r $' s",
};
const trickyTemplate = '{{amp}}|{group}|{{dollars}}|{before}|{{after}}';
const trickyExpected = 'a $& b|x $1 y|p $$ q|m $` n|r $\' s';

// t(): every replacement pattern value is inserted literally.
{
    const { t } = load({
        'issue.title': 'Issue: {{title}} ({kind})',
        'tricky.all': trickyTemplate,
    });
    for (const [name, value] of Object.entries(tricky)) {
        assert.equal(
            t('issue.title', { title: value, kind: value }),
            'Issue: ' + value + ' (' + value + ')',
            't must insert the ' + name + ' value literally'
        );
    }
    assert.equal(t('tricky.all', tricky), trickyExpected, 't must insert all tricky values literally');
}

// tOr(): the same for a key that is present.
{
    const { tOr } = load({
        'issue.title': 'Issue: {{title}} ({kind})',
        'tricky.all': trickyTemplate,
    });
    for (const [name, value] of Object.entries(tricky)) {
        assert.equal(
            tOr('issue.title', 'fallback', { title: value, kind: value }),
            'Issue: ' + value + ' (' + value + ')',
            'tOr must insert the ' + name + ' value literally'
        );
    }
    assert.equal(tOr('tricky.all', 'fallback', tricky), trickyExpected, 'tOr must insert all tricky values literally');
}

// Ordinary substitution is unchanged.
{
    const { t, tOr } = load({
        count: 'n={n}',
        double: 'Hello {{name}}!',
        single: 'Hello {name}!',
        repeated: '{{x}} and {{x}} and {x} and {x}',
        unknown: 'a {{y}} b {z} c {{x}}',
        nothing: 'null={a} undefined={b}',
    });
    for (const fn of [t, (k, p) => tOr(k, 'fallback', p)]) {
        assert.equal(fn('count', { n: 3 }), 'n=3', 'a number renders as its decimal text');
        assert.equal(fn('double', { name: 'Ada' }), 'Hello Ada!', '{{x}} syntax');
        assert.equal(fn('single', { name: 'Ada' }), 'Hello Ada!', '{x} syntax');
        assert.equal(fn('repeated', { x: '$&' }), '$& and $& and $& and $&', 'every occurrence is replaced');
        assert.equal(fn('repeated', { x: 'v' }), 'v and v and v and v', 'every occurrence is replaced');
        assert.equal(fn('unknown', { x: '1' }), 'a {{y}} b {z} c 1', 'unknown placeholders stay as they are');
        assert.equal(fn('nothing', { a: null, b: undefined }), 'null=null undefined=undefined', 'null and undefined render as before');
    }
}

// Missing key: t() returns the key itself (placeholders in it are still substituted), tOr() returns the fallback.
{
    const { t, tOr } = load({ present: 'value' });
    assert.equal(t('missing.key'), 'missing.key');
    assert.equal(t('raw {{x}}', { x: '$&' }), 'raw $&', 'the key is substituted literally');
    assert.equal(t('raw {x} {x}', { x: '$$' }), 'raw $$ $$', 'the key is substituted literally');
    assert.equal(tOr('missing.key', 'fb {{x}}', { x: '1' }), 'fb {{x}}', 'a string fallback is returned unchanged');
    assert.equal(tOr('missing.key', 'fb'), 'fb');
    assert.equal(tOr('missing.key', 42), '42', 'a non-string fallback is converted with String()');
    assert.equal(tOr('missing.key', null), 'missing.key', 'a null fallback returns the key');
    assert.equal(tOr('missing.key', undefined), 'missing.key', 'an undefined fallback returns the key');
}

// No dictionary at all.
{
    const { t, tOr } = load(undefined);
    assert.equal(t('k'), 'k');
    assert.equal(tOr('k', 'fb'), 'fb');
}

console.log('shared-translate checks passed');
