// Shared context of the Storymaker player tests: the check runner and lazily built fixtures. Every module under
// scripts/storymaker-tests/ imports `t` from here and registers its checks at the top level.
import assert from 'node:assert/strict';
import * as esbuild from 'esbuild';
import { read, miniCatalog, miniFiles, story, loadPlayer } from './storymaker-test-support.mjs';

export const results = [];
export const pending = [];

export function check(name, fn) {
    const fail = err => results.push('FAIL ' + name + ': ' + (err && err.stack || err));
    try {
        const out = fn();
        if (out && typeof out.then === 'function') {
            const done = out.then(() => results.push('ok   ' + name), fail);
            pending.push(done);
            return done;
        }
        results.push('ok   ' + name);
    } catch (err) { fail(err); }
    return undefined;
}

export const near = (actual, expected, eps, label) => assert.ok(Math.abs(actual - expected) <= eps, (label || 'value') + ': ' + actual + ' vs ' + expected);

let player = null, compiledOnce = null;
const catalog = miniCatalog();

// Bundle a code scene exactly like the Go side does (contract section 4: esbuild Transform, IIFE, globalName
// __smSceneModule, target es2020, no legal comments, then the registration line).
function bundleScene(source, name) {
    const out = esbuild.transformSync(source, { format: 'iife', globalName: '__smSceneModule', target: 'es2020', legalComments: 'none', loader: 'js', sourcefile: 'scenes/' + name + '.js' });
    return out.code + '\nself.__smScene=__smSceneModule.default;\n';
}
const s3Source = read('internal/storymaker/testdata/scenes/s3.js');

export const t = {
    results, pending, check, near, catalog, files: miniFiles(), firewall: story('firewall'), minimal: story('minimal'), bundleScene, s3Source,
    s3Bundle: bundleScene(s3Source, 's3'),
    // core(): StorymakerCore of the real player sources in one shared vm context
    core() { if (!player) player = loadPlayer(); return player.StorymakerCore; },
    byId(id) { return catalog.entries.find(e => e.id === id); },
    // compiled(): the firewall story compiled once; the code scene s3 lasts api.duration(5)
    compiled() {
        if (!compiledOnce) compiledOnce = t.core().compile(t.firewall, {}, catalog, { codeScenes: { s3: { duration: 5 } } });
        return compiledOnce;
    },
    sceneById(id) { return t.compiled().timeline.scenes.find(s => s.id === id); }
};
