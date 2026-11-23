export default function scene(api) {
    api.duration(8);
    // Cue {1} marks the narrated word where the shield appears; without that marker use a fixed time.
    function cueOr(n, fallback) {
        try {
            return api.cue(n);
        } catch (e) {
            return fallback;
        }
    }
    const shown = cueOr(1, 1.5);
    const title = api.text('Jedes Paket wird geprüft', 'label', { at: { x: 50, y: 3 } });
    const rule = api.shape('rect', { at: { x: 50, y: 11 }, w: 40, h: 6, fill: '#d49b36' });
    api.tween(title, { opacity: 1 }, { start: 0, dur: 0.6, ease: 'ease-out', from: { opacity: 0 } });
    api.tween(rule, { width: 420 }, { start: 0.3, dur: 0.8, ease: 'ease-out' });
    api.tween(rule, { opacity: 0.35 }, { start: shown + 1, dur: 1, ease: 'ease-in-out' });
    const shield = 'M0-120L96-84V-6Q96 70 0 122Q-96 70-96-6V-84Z';
    const tick = 'M-38 4L-10 32L42-28';
    const lanes = [-56, 0, 56];
    api.onFrame(function (t, g) {
        const appear = Math.min(1, Math.max(0, (t - shown) / 0.5));
        g.save();
        g.translate(960, 460);
        g.save();
        g.translate(0, 8);
        g.alpha(appear * 0.16);
        g.fill('#183f39');
        g.path(shield);
        g.restore();
        g.alpha(appear);
        g.fill('#eee5cf');
        g.stroke('#4b7067', 6);
        g.path(shield, 'both');
        g.stroke('#28594e', 10);
        g.path(tick, 'stroke');
        g.restore();
        let checked = 0;
        g.save();
        for (let i = 0; i < 3; i++) {
            const u = Math.min(1, Math.max(0, (t - shown - 0.5 - i * 0.7) / 2.4));
            if (u <= 0) continue;
            const x = 740 + u * 440;
            if (x > 960) checked++;
            g.fill(x > 960 ? '#2f8f6b' : '#d49b36');
            g.circle(x, 460 + lanes[i], 12);
        }
        g.restore();
        g.save();
        g.alpha(appear);
        g.text(checked + ' / 3', 960, 700, { size: 30, color: '#28594e' });
        g.restore();
    });
}
