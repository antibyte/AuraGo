// AuraGo – Mobile viewport / virtual keyboard fix
// Publishes a CSS custom property `--vvh` mirroring visualViewport.height,
// so layout can prefer modern viewport units (`100dvh`) and only fall back
// to JS-provided pixel heights on browsers without dvh support. We avoid
// writing `body.style.height` directly because that conflicted with CSS
// dvh/svh/lvh on iOS Safari and Android Chrome (visible layout jumps when
// the address bar or keyboard animated).
(function () {
    var body = document.body;
    var root = document.documentElement;
    var footer = document.querySelector('.app-footer');
    var header = document.querySelector('.app-header');

    // Ensure critical flex properties are always applied, even if
    // the Tailwind CDN fails to load or generates classes in wrong order.
    if (body) {
        body.style.display = 'flex';
        body.style.flexDirection = 'column';
    }
    if (footer) {
        footer.style.flexShrink = '0';
    }
    if (header) {
        header.style.flexShrink = '0';
    }

    // Detect whether dvh/svh are supported. If yes, the JS fallback is
    // strictly opt-in (consumers can still read --vvh) but we no longer
    // need to override body height.
    var supportsDvh = false;
    try {
        supportsDvh = window.CSS && CSS.supports && CSS.supports('height', '100dvh');
    } catch (_) { /* noop */ }

    function applyVVH() {
        var h = (window.visualViewport ? window.visualViewport.height : window.innerHeight);
        if (root) {
            root.style.setProperty('--vvh', h + 'px');
        }
        // Legacy fallback: only if the browser cannot natively size
        // to the visual viewport via dvh, write the pixel height
        // straight onto <body>. Modern browsers stay untouched.
        if (!supportsDvh && body) {
            body.style.height = h + 'px';
        }
    }

    // Initial apply
    applyVVH();

    // Listen for viewport changes (keyboard open/close, address bar show/hide)
    if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', applyVVH);
        window.visualViewport.addEventListener('scroll', applyVVH);
    }
    window.addEventListener('resize', applyVVH);

    // Re-apply after a short delay to catch late layout shifts
    setTimeout(applyVVH, 100);
    setTimeout(applyVVH, 500);
})();
