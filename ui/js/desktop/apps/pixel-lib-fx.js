// Pixel pure image library, filters part (WP5). DOM-free; loads in a window, in the Pixel worker and in a node vm.
// Register ops with AuraPixelLib.registerOp('fx.apply', (args, ctx) => ...). Empty until WP5 fills it.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    void Lib;
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
