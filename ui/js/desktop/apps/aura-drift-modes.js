/* Aura Drift - rule sets, elimination, hot potato, drift attack and results.
   T1.1 state: the mode table (section 2.8), cup points and a result builder for the
   classic "player finishes" modes. T1.5 adds elimination, hot potato, drift attack,
   the sweeper and medals. */
(function () {
    'use strict';
    const AD = window.AuraDriftCore;

    function mode(id, opponents, weapons, crates, ramps, pads, destructibles, animals, laps, extra) {
        return Object.assign({
            id: id, opponents: opponents,
            weapons: weapons,                    /* 'setting' follows the user option, false = off, array = pool */
            crates: crates, ramps: ramps, pads: pads, destructibles: destructibles, animals: animals,
            laps: laps                           /* 'config' (menu), 'track' (manifest) or a fixed number */
        }, extra || {});
    }
    AD.MODES = [
        mode('quick', 5, 'setting', true, true, true, true, true, 'config'),
        mode('time_trial', 0, false, false, true, true, true, false, 'config'),
        mode('cup', 5, 'setting', true, true, true, true, true, 'track'),
        mode('daily', 5, 'setting', true, true, true, true, true, 3),
        mode('drift_attack', 0, false, false, true, true, true, false, 99, { clock: 90, noSmashPenalty: true }),
        mode('elimination', 5, 'setting', true, true, true, true, true, 99),
        mode('hot_potato', 5, ['air_horn', 'shield', 'overdrive'], true, true, true, true, false, 99)
    ];
    AD.modeRules = function (id) {
        return AD.MODES.find(m => m.id === id) || AD.MODES[0];
    };

    AD.cupPoints = function (position) {
        return [25, 18, 15, 12, 10, 8][position - 1] || 0;
    };

    AD.initMode = function (race) {};
    AD.updateMode = function (race, dt) {};
    AD.modeAiOverride = function (race, car, input) {};

    /* Classic rule: the race ends when the player crosses the line. */
    AD.modeFinished = function (race) {
        return !!race.cars[0].finished;
    };

    AD.buildResult = function (race) {
        const player = race.cars[0];
        const style = typeof AD.styleSummary === 'function' ? AD.styleSummary(player) : null;
        const tricks = player.tricks || {};
        return {
            time: player.finishTime, bestLap: player.bestLap,
            position: race.standings.indexOf(player) + 1,
            tape: race.tapeOverflow ? [] : player.tape.slice(),
            style: style,
            sectors: player.bestSectors.slice(),
            mode: race.mode,
            score: style ? style.banked + style.pending : 0,
            medal: null,
            title: null,                         /* set by the shell through banter */
            tricks: { counts: Object.assign({}, tricks.counts), best: tricks.best || 0 },
            tally: Object.assign({}, player.tally),
            tapeComplete: !race.tapeOverflow,
            eliminatedAt: player.eliminatedAt,
            field: race.cars.length,
            carId: player.v.car.id
        };
    };
})();
