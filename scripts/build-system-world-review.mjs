import {build} from 'esbuild';
import fs from 'node:fs/promises';
await fs.mkdir('reports/aurora',{recursive:true});
await build({entryPoints:['scripts/system-world-visual-review.js'],bundle:true,format:'esm',minify:true,outfile:'reports/aurora/system-world-review.mjs'});
