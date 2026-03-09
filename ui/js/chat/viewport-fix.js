// AuraGo – Mobile viewport fix
// Extracted from index.html

        // ── Mobile viewport / virtual keyboard fix ───────────────────────────────
        // When the on-screen keyboard opens on Android/iOS the visual viewport
        // shrinks but the layout viewport may not.  We track visualViewport and
        // keep the body height in sync so the input toolbar is always visible.
        (function () {
            function applyVVH() {
                var h = (window.visualViewport ? window.visualViewport.height : window.innerHeight);
                document.body.style.height = h + 'px';
            }
            if (window.visualViewport) {
                window.visualViewport.addEventListener('resize', applyVVH);
                window.visualViewport.addEventListener('scroll', applyVVH);
            }
            window.addEventListener('resize', applyVVH);
            applyVVH();
        })();
