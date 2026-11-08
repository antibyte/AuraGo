export default function scene(api) {
    api.duration(8);
    const actor = api.add('char.office-woman-1', { at: 'left', pose: 'present' });
    const lock = api.add('prop.lock', { at: { x: 75, y: 38 }, scale: 1.2 });
    api.text('Geschützt verbunden', 'label', { at: { x: 50, y: 18 } });
    lock.fadeIn({ start: 0.6, dur: 0.5, ease: 'ease-out' });
    actor.gesture('point', { start: 1.1, dur: 1.2 });
    actor.emote('happy', { start: 2.4 });
    actor.look('camera', { start: 3 });
    api.tween(lock, { scale: 1.35 }, { start: 2, dur: 0.6, ease: 'back-out' });
    api.camera.zoom(1.05, { start: 4, dur: 1.2, ease: 'ease-in-out' });
    const random = api.random('protected-packets');
    const phase = random() * Math.PI * 2;
    api.onFrame(function (t, g) {
        g.save();
        g.fill('#287b71');
        g.alpha(Math.min(1, Math.max(0, (t - 0.5) * 2)));
        for (let i = 0; i < 4; i++) {
            const u = ((t + i * 0.5) % 2) / 2;
            g.circle(800 + u * 320, 600 + Math.sin(t * 3 + phase + i) * 12, 9);
        }
        g.restore();
    });
}
