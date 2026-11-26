// The open checks of the AI author panel: every code that can reach the list resolves to a real text key.
import assert from 'node:assert/strict';
import fs from 'node:fs';
import vm from 'node:vm';
import { read } from '../storymaker-test-support.mjs';
import { t } from '../storymaker-test-context.mjs';

t.check('agent panel: every open check code has its own text, advisory codes and unknown codes fall back', () => {
    const ctx = vm.createContext({ window: {}, document: { documentElement: { lang: 'en' } } });
    ctx.window = ctx;
    vm.runInContext(read('ui/js/desktop/apps/storymaker-core.js'), ctx);
    vm.runInContext(read('ui/js/desktop/apps/storymaker-agent.js'), ctx);
    const S = ctx.StorymakerParts, keyOf = S.agentCheckKey;
    assert.equal(typeof keyOf, 'function');
    const en = JSON.parse(fs.readFileSync('ui/lang/storymaker/en.json', 'utf8'));
    const codes = ['layout_unverified', 'compile_unverified', 'narration_unavailable', 'narration_failed', 'narration_budget', 'narration_pending',
        'corrections_exhausted', 'model_error', 'scene_unfilled', 'compile_failed', 'over_target', 'caption_overlap', 'scene_static', 'cast_unused',
        'scenes_look_alike', 'over_export_limit', 'text_offframe', 'flash', 'flash_unverified', 'code_error', 'deadline', 'glyph_missing',
        'invalid_value', 'missing_field', 'unknown_name', 'unknown_asset', 'unknown_spot', 'unknown_cast', 'unknown_cue', 'duplicate_id', 'limit_exceeded', 'schema_mismatch'];
    for (const code of codes) {
        const key = keyOf(code);
        assert.notEqual(key, 'storymaker.agent.check_unknown', code + ' must not fall back to the generic line');
        assert.ok(typeof en[key] === 'string' && en[key], code + ' -> ' + key + ' is missing in en.json');
    }
    assert.equal(keyOf('scenes_look_alike'), 'storymaker.agent.check_scenes_look_alike');
    assert.equal(keyOf('flash'), 'storymaker.issue.flash');
    for (const code of ['over_budget', 'nope', '', undefined, null, 'constructor', '__proto__']) assert.equal(keyOf(code), 'storymaker.agent.check_unknown', String(code));
});
