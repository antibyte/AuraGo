// Pixel module "brush" (stub). WP4 brush engine: stamp-based brush, pencil, eraser and airbrush tools.
// Empty until its work package fills it. It is already listed in the module loader, in pixelAppScriptPaths and in the
// camera handoff test, so the package only edits this file: register panels, tools, menu items, commands and overlays
// with Pixel.registerPanel / registerTool / registerMenuItems / registerCommand, and add runtime methods in install(r).
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.registerModule('brush', function install(r) {
        void r;
    });
})();
