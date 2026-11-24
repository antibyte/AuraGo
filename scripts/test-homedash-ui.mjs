// HomeDash engine Node tests and guards. Run: npm run test:homedash. Every scripts/homedash-tests/*.test.mjs exports
// `tests`, a list of [name, fn] pairs (fn may be async).
import fs from 'node:fs'; import path from 'node:path'; import { pathToFileURL } from 'node:url';
const dir = path.join(import.meta.dirname, 'homedash-tests');
let failures = 0, count = 0;
for (const file of fs.readdirSync(dir).filter(f => f.endsWith('.test.mjs')).sort()) {
  const { tests } = await import(pathToFileURL(path.join(dir, file)).href);
  for (const [name, fn] of tests) {
    count++;
    try { await fn(); console.log(`PASS ${file}: ${name}`); } catch (e) { failures++; console.error(`FAIL ${file}: ${name}: ${e && e.stack || e}`); }
  }
}
console.log(`homedash: ${count - failures}/${count} passed`);
if (failures || !count) process.exitCode = 1;
