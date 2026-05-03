import { mkdir } from 'node:fs/promises';
import { rollup } from 'rollup';
import { nodeResolve } from '@rollup/plugin-node-resolve';

const entryFile = 'scripts/codemirror-entry.js';
const outputFile = 'ui/js/vendor/codemirror-bundle.esm.js';

const expectedExports = [
  '@codemirror/view',
  '@codemirror/state',
  '@codemirror/commands',
  '@codemirror/search',
  '@codemirror/lang-javascript',
  '@codemirror/lang-python',
  '@codemirror/lang-go',
  '@codemirror/lang-rust',
  '@codemirror/lang-json',
  '@codemirror/lang-html',
  '@codemirror/lang-css',
  '@codemirror/lang-markdown',
  '@codemirror/theme-one-dark',
  '@codemirror/language',
  '@codemirror/autocomplete',
  '@codemirror/lint',
];

console.log(`Building CodeMirror bundle from ${entryFile}`);
console.log(`Including ${expectedExports.length} CodeMirror packages`);

await mkdir('ui/js/vendor', { recursive: true });

const bundle = await rollup({
  input: entryFile,
  plugins: [nodeResolve()],
  onwarn(warning, warn) {
    if (warning.code === 'MODULE_LEVEL_DIRECTIVE') {
      return;
    }
    warn(warning);
  },
});

try {
  await bundle.write({
    file: outputFile,
    format: 'esm',
    sourcemap: false,
    generatedCode: 'es2015',
  });
  console.log(`CodeMirror bundle written to ${outputFile}`);
} finally {
  await bundle.close();
}
