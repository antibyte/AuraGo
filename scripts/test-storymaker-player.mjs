// Storymaker player runtime (ui/js/storymaker): runs the real player and host sources in a Node vm against a recording
// fake 2D context, in-process workers and a fake parent window / fake fetch. The checks live in scripts/storymaker-tests/
// (one module per area, run in file name order):
//   01 easing and tracks        02 stage mapping, parts, text    03 rig                 04 compile and timing
//   05 renderFrame              06 code scenes and determinism   07 worker protocol     08 app host (section 8 protocol)
//   09 render driver (section 7)   10 static guards
// Run: node scripts/test-storymaker-player.mjs   (npm run test:storymaker)
import fs from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';
import { results, pending } from './storymaker-test-context.mjs';

const dir = path.join(import.meta.dirname, 'storymaker-tests');
for (const file of fs.readdirSync(dir).filter(f => f.endsWith('.mjs')).sort()) {
    await import(pathToFileURL(path.join(dir, file)).href);
}
await Promise.all(pending);
console.log(results.join('\n'));
const failed = results.filter(r => r.startsWith('FAIL')).length;
console.log('\n' + (results.length - failed) + ' passed, ' + failed + ' failed');
process.exit(failed || !results.length ? 1 : 0);
