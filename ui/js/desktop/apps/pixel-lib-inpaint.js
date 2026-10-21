// Pixel pure image library, inpainting / content-aware fill part (WP7). DOM-free; loads in a window, in the Pixel worker
// and in a node vm. Register ops with AuraPixelLib.registerOp('inpaint.fill', (args, ctx) => ...). Empty until WP7 fills it.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    void Lib;
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
