// AuraGo – Mobile viewport / virtual keyboard fix
// Tracks visualViewport and keeps body height in sync so the
// input toolbar is always visible when the on-screen keyboard opens.
(function () {
    var body = document.body;
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

    function applyVVH() {
        var h = (window.visualViewport ? window.visualViewport.height : window.innerHeight);
        if (body) {
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
