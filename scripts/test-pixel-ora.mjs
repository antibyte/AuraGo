// WP2 OpenRaster (pixel-ora.js): node test (stub). Replace the body with write/read round-trip checks.
import { loadPixelLibs } from './pixel-test-util.mjs';

const { AuraPixelLib: Lib } = loadPixelLibs();
if (!Lib) throw new Error('AuraPixelLib did not load');
console.log('test-pixel-ora: nothing to check yet');
