import fs from 'node:fs/promises';
import validator from 'gltf-validator';

const root = 'ui/3d/screensaver/abyss/v1/';
const manifest = JSON.parse(await fs.readFile(root + 'manifest.json', 'utf8'));
let errors = 0;
let warnings = 0;
for (const asset of manifest.assets) {
  const bytes = await fs.readFile(root + asset.file);
  const result = await validator.validateBytes(bytes, {
    uri: asset.file,
    maxIssues: 100,
    externalResourceFunction: () => Promise.reject(Error('external resource forbidden'))
  });
  errors += result.issues.numErrors;
  warnings += result.issues.numWarnings;
  for (const message of result.issues.messages.filter(m => m.severity === 0)) {
    console.error(asset.file + ': ' + message.code + ' ' + message.message);
  }
}
console.log(manifest.assets.length + ' Tiefsee GLBs: ' + errors + ' errors, ' + warnings + ' warnings (' + validator.version() + ')');
if (errors) process.exitCode = 1;
