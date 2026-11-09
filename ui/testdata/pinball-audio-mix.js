// Scripted 20 s gameplay sequence for the offline audio renderer: plunger, flippers, bumpers, sling, ramp roll,
// jackpot (SFX cue + music stinger, like the game's audio.cue) and a drain, over the main music and the ambience bed.
// sched(t, fn) runs fn at virtual time t seconds.
export function mixScript({ sched, sfx, music }) {
    const cue = name => { sfx.cue(name); music.stinger(name); };
    const ev = e => sfx.event(e);
    // plunger pull and launch
    for (let i = 1; i <= 12; i++) sched(0.6 + i * 0.07, () => sfx.plunger(i / 12));
    sched(1.6, () => sfx.plunger(0));
    sched(1.65, () => ev({ type: 'launch', speed: 9 }));
    sched(1.75, () => ev({ type: 'shooter' }));
    // rolling ball: field, then a wire ramp, then field again
    const roll = (from, to, ball) => { for (let t = from; t < to; t += 0.05) sched(t, () => sfx.roll([ball])); };
    roll(1.8, 3.6, { mode: 'free', speed: 2.2, x: 0.02 });
    roll(8.0, 10.6, { mode: 'path', pathKind: 'wire', speed: 2.0, x: -0.05 });
    roll(10.6, 12.4, { mode: 'free', speed: 1.4, x: 0 });
    roll(14.2, 17.4, { mode: 'free', speed: 1.8, x: 0.05 });
    sched(3.7, () => sfx.roll([])); sched(12.5, () => sfx.roll([])); sched(17.5, () => sfx.roll([]));
    // flipper pairs
    for (const [t, id] of [[3.0, 'flipperL'], [4.4, 'flipperR'], [5.2, 'flipperL'], [13.6, 'flipperR'], [15.0, 'flipperL'], [16.2, 'flipperR']]) {
        sched(t, () => ev({ type: 'flipper', up: true, id }));
        sched(t + 0.22, () => ev({ type: 'flipper', up: false, id }));
    }
    // bumper burst, sling
    ['pop1', 'pop2', 'pop3', 'pop1', 'pop2', 'pop3', 'pop1', 'pop3'].forEach((id, i) => sched(6.0 + i * 0.13, () => ev({ type: 'bumper', id })));
    sched(7.5, () => ev({ type: 'sling', id: 'slingL' }));
    sched(7.75, () => ev({ type: 'sling', id: 'slingR' }));
    // spinner and standup/drop around the ramp
    for (let i = 0; i < 12; i++) sched(10.7 + i * (0.05 + i * 0.008), () => ev({ type: 'spinner', id: 'spin1', rate: 40 - i * 3 }));
    sched(12.0, () => ev({ type: 'standup', id: 'st1' }));
    sched(12.25, () => ev({ type: 'drop', id: 'drop1' }));
    // jackpot
    sched(12.6, () => cue('jackpot'));
    // late play, then the drain
    sched(15.5, () => ev({ type: 'rollover', id: 'laneL' }));
    sched(17.6, () => { ev({ type: 'drain' }); music.stinger('drain'); });
}
