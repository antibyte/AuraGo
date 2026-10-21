// Pixel pure image library, cutout / background removal part (WP8a). DOM-free; loads in a window, in the Pixel worker
// and in a node vm. Register ops with AuraPixelLib.registerOp('cutout.classic', (args, ctx) => ...). Empty until WP8a fills it.
(function (root) {
    'use strict';

    const Lib = root.AuraPixelLib = root.AuraPixelLib || {};
    void Lib;
})(typeof self !== 'undefined' ? self : (typeof globalThis !== 'undefined' ? globalThis : this));
