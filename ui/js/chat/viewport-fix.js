// AuraGo – Mobile viewport / virtual keyboard fix
// Tracks visualViewport and keeps body height in sync so the
// input toolbar is always visible when the on-screen keyboard opens.
(function () {
    function applyVVH() {
        var h = (window.visualViewport ? window.visualViewport.height : window.innerHeight);
        document.body.style.height = h + 'px';
        // Also ensure the footer is never shrunk away on small screens
        var footer = document.querySelector('.app-footer');
        if (footer) {
            footer.style.flexShrink = '0';
        }
    }
    if (window.visualViewport) {
        window.visualViewport.addEventListener('resize', applyVVH);
        window.visualViewport.addEventListener('scroll', applyVVH);
    }
    window.addEventListener('resize', applyVVH);
    applyVVH();
})();
