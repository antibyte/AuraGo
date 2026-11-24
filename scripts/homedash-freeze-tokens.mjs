#!/usr/bin/env node
// HomeDash token freeze (M1 Task 22): extracts every rule whose declarations are all --hd-* custom properties.
// `node scripts/homedash-freeze-tokens.mjs` writes ui/testdata/homedash-lab/token-freeze.json (only with the lead's approval).
import fs from 'node:fs'; import path from 'node:path'; import { pathToFileURL } from 'node:url';
export const ROOT = path.resolve(import.meta.dirname, '..');
export const FREEZE_PATH = 'ui/testdata/homedash-lab/token-freeze.json';
export const FREEZE_FILES = ['ui/css/homedash-tokens.css', 'ui/css/homedash-bridge.css', 'ui/css/homedash-family-glass.css',
  'ui/css/homedash-family-tactile.css', 'ui/css/homedash-family-minimal.css', 'ui/css/homedash-family-instrument.css'];
export function tokenRules(text) {
  const css = text.replace(/\/\*[\s\S]*?\*\//g, ''), rules = {};
  for (const m of css.matchAll(/([^{}]+)\{([^{}]*)\}/g)) {
    const selector = m[1].trim().replace(/\s+/g, ' ');
    if (selector.startsWith('@')) continue;
    const decls = m[2].split(';').map((d) => d.trim()).filter(Boolean);
    if (!decls.length || !decls.every((d) => d.startsWith('--hd-'))) continue;
    const body = rules[selector] || {};
    for (const d of decls) { const i = d.indexOf(':'); body[d.slice(0, i).trim()] = d.slice(i + 1).trim().replace(/\s+/g, ' '); }
    rules[selector] = body;
  }
  return rules;
}
export const current = () => Object.fromEntries(FREEZE_FILES.map((f) => [f, tokenRules(fs.readFileSync(path.join(ROOT, f), 'utf8'))]));
if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  fs.writeFileSync(path.join(ROOT, FREEZE_PATH), JSON.stringify(current(), null, 2) + '\n');
  console.log('token freeze written to ' + FREEZE_PATH + '; commit it only with the lead\'s approval');
}
