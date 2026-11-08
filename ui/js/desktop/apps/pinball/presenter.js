// AuraPinball presenter: turns presentation commands from the rules into lamp shows, DMD scenes, sound and camera shake,
// and runs the attract-mode lamp playlist. No DOM; every collaborator is optional so it also runs headless.
export function createPresenter({ layout, lamps, dmd, audio, scene, callouts = () => true, reduced = () => false }) {
    let flip = 1, playlist = 0, playlistT = 0, playlistOn = false;

    const handlers = {
        show(c) {
            const opts = { duration: (c.ms > 0 ? c.ms : 1000) / 1000, priority: c.priority || 0 };
            for (const key of ['replace', 'color', 'x', 'y']) if (c[key] !== undefined) opts[key] = c[key];
            if (c.repeat > 1) opts.loop = c.repeat;
            if (c.name === 'rainbow') lamps.stopShows('rainbow');   // the wizard re-emits it every 3 s
            lamps.show(c.name, opts);
        },
        flash(c) { lamps.flash(c.id ?? 'all', c.ms); },
        gi(c) { lamps.gi(c.id ?? 'all', c.mode, c.ms); },
        lampColor(c) { lamps.setColor(c.id, c.color); },
        dmd(c) { if (c.scene === null || c.scene === undefined) dmd.clear(); else dmd.play(c.scene, c.params, { ms: c.ms, priority: c.priority, transition: c.transition }); },
        dmdBase(c) { dmd.setBase(c.scene, c.params); },
        cue(c) { if (audio) audio.cue(c.name); },
        music(c) { if (audio) audio.music(c.section); },
        say(c) { if (audio && callouts()) audio.say(c.key); },
        shake(c) { shake(c.strength || 0.004, flip, 0.6); flip = -flip; }
    };

    function shake(strength, dx, dy) { if (scene && !reduced()) scene.shake(dx, dy, strength); }

    function startShow() {
        const list = (layout.meta && layout.meta.attract) || [];
        if (!list.length) return;
        const [name, ms] = list[playlist % list.length];
        lamps.stopShows();
        lamps.show(name, { duration: ms / 1000 });
    }

    return {
        handle(command) { const fn = handlers[command.type]; if (fn) fn(command); },
        // Physics events that need a visual or audible reaction beyond the rules (sound, camera).
        event(ev) {
            if (audio) audio.event(ev);
            if (scene) scene.hit(ev);
        },
        // A nudge pushes the ball (dx, dy); the camera jolts the other way.
        nudge(dx, dy) { shake(0.004, -Math.sign(dx) || 0, -Math.sign(dy) || 0); },
        // Attract lamp playlist: advances through layout.meta.attract while `on`.
        attract(dt, on) {
            const list = (layout.meta && layout.meta.attract) || [];
            if (!on) { playlistOn = false; return; }
            if (!playlistOn) { playlistOn = true; playlist = 0; playlistT = 0; startShow(); return; }
            playlistT += dt;
            if (list.length && playlistT * 1000 >= list[playlist % list.length][1]) { playlistT = 0; playlist = (playlist + 1) % list.length; startShow(); }
        },
        // Back to a neutral stage (game start, return to the lobby).
        reset() { lamps.stopShows(); lamps.gi('all', 'on', 0); dmd.clear(); playlistOn = false; }
    };
}

export default createPresenter;
