// Pixel module "selection" (stub). WP3 selection: selection tools, the "select" menu and selection commands.
// Empty until its work package fills it. It is already listed in the module loader, in pixelAppScriptPaths and in the
// camera handoff test, so the package only edits this file: register panels, tools, menu items, commands and overlays
// with Pixel.registerPanel / registerTool / registerMenuItems / registerCommand, and add runtime methods in install(r).
(function () {
    const Pixel = window.AuraPixelApp = window.AuraPixelApp || {};

    Pixel.registerModule('selection', function install(r) {
        void r;
    });
})();
