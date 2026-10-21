// Pixel pure image library, generators part (WP6). DOM-free; loads in a window, in the Pixel worker and in a node vm.
// Register ops with AuraPixelLib.registerOp('gen.render', (args, ctx) => ...). Empty until WP6 fills it.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    void Lib;
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
