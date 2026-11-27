export default function scene(api) {
    api.duration(8);
    const box = api.safe();
    // Cues {1} and {2} mark the narrated words where the check starts and ends; without markers use fixed times.
    function cueOr(n, fallback) {
        try {
            return api.cue(n);
        } catch (e) {
            return fallback;
        }
    }
    const check = cueOr(1, 2.8);
    const done = cueOr(2, 5);
    api.headline('Jedes Paket wird *geprüft*', { at: 'top', start: 0.2 });
    const flow = api.diagram({
        layout: 'flow',
        flow: true,
        nodes: [
            { id: 'packet', label: 'Paket', icon: 'icon.mail' },
            { id: 'firewall', label: 'Firewall', icon: 'icon.lock' },
            { id: 'network', label: 'Netzwerk', icon: 'icon.server' }
        ]
    }, { at: { x: 50, y: 62 }, reveals: [1, check, done] });
    flow.highlight('firewall', { start: check + 0.4, dur: 1.6 });
    // One small accent: a hairline at the foot of the safe area that fills over the whole scene.
    api.onFrame(function (t, g) {
        const u = Math.min(1, Math.max(0, t / 8));
        g.save();
        g.alpha(0.35);
        g.fill('$text');
        g.rect(box.x, box.y + box.h - 6, box.w, 2);
        g.alpha(1);
        g.fill('$accent');
        g.rect(box.x, box.y + box.h - 8, box.w * u, 6);
        g.restore();
    });
}
