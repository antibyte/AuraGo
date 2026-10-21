// WP5 filters (pixel-lib-fx.js): node test (stub). Replace the body with real checks on tiny images; this loads every pixel lib into a vm.
import { loadPixelLibs } from './pixel-test-util.mjs';

const { AuraPixelLib: Lib } = loadPixelLibs();
if (!Lib || typeof Lib.handleMessage !== 'function') throw new Error('AuraPixelLib did not load');
console.log('test-pixel-fx: libs load (no checks yet)');
