export default function scene(api) {
    api.duration(8);
    const panel = api.add('prop.solar-panel', { at: { x: 50, y: 29 }, scale: 1.15 });
    const battery = api.add('prop.battery', { at: { x: 50, y: 72 }, scale: 1.15 });
    api.text('Sonnenstrom speichern', 'label', { at: { x: 50, y: 3 } });
    panel.fadeIn({ start: 0, dur: 0.5, ease: 'ease-out' });
    battery.fadeIn({ start: 2.8, dur: 0.5, ease: 'ease-out' });
    const shell = 'M-104-78H104Q120-78 120-62V62Q120 78 104 78H-104Q-120 78-120 62V-62Q-120-78-104-78Z';
    const screen = 'M-78-49H78Q86-49 86-41V23Q86 31 78 31H-78Q-86 31-86 23V-41Q-86-49-78-49Z';
    const bolt = 'M5-40L-18-13H-3L-10 8L18-21H4Z';
    api.onFrame(function (t, g) {
        const reveal = Math.min(1, Math.max(0, (t - 1) / 0.5));
        const flow = Math.min(1, Math.max(0, (t - 3.3) / 0.4));
        g.save();
        g.alpha(reveal);
        g.stroke('#799b91', 6);
        g.path('M960 345V388 M960 562V601', 'stroke');
        g.translate(960, 475);
        g.save();
        g.translate(0, 7);
        g.alpha(reveal * 0.16);
        g.fill('#183f39');
        g.path(shell);
        g.restore();
        g.fill('#eee5cf');
        g.stroke('#4b7067', 3);
        g.path(shell, 'both');
        g.fill('#183f39');
        g.path(screen);
        g.fill('#e4b55e');
        g.path(bolt);
        g.text('Laderegler', 0, 21, { size: 18, color: '#f2ead7' });
        g.stroke('#849c8e', 3);
        for (let i = 0; i < 5; i++) {
            const x = -40 + i * 20;
            g.line(x, 48, x + 10, 48);
        }
        g.fill('#b8bba8');
        g.circle(-104, -59, 4);
        g.circle(104, -59, 4);
        g.circle(-104, 59, 4);
        g.circle(104, 59, 4);
        g.restore();
        g.save();
        g.alpha(flow);
        g.fill('#d49b36');
        for (let i = 0; i < 3; i++) {
            const u = ((Math.max(0, t - 3.3) + i * 0.45) % 1.35) / 1.35;
            g.circle(960, 345 + u * 40, 4);
            g.circle(960, 562 + u * 35, 4);
        }
        g.text('Speicher', 960, 800, { size: 27, color: '#28594e' });
        g.restore();
    });
}
