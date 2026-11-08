// Cyber Neon Metropolis: rule pack stub (framework defaults only; no shots or modes yet).
export const rules = {
    points: { default: 1010 },
    shotPoints: {},
    bonusFrom: {},
    bonusCounters: [],
    topLanes: null,
    lowerLanes: { switches: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], lamps: ['inlaneL', 'outlaneL', 'inlaneR', 'outlaneR'], award: 'kickback' },
    bonusX: { lamps: ['bonus2x', 'bonus3x', 'bonus4x', 'bonus5x'], max: 5 },
    skill: { value: 250000 },
    ballSave: { seconds: 8, lamp: 'shootAgain' },
    kickback: { device: 'kickbackL', lamp: 'kickbackL', litAtStart: true },
    modes: [],
    combos: { window: 3.0, base: 100000 },
    replay: 40000000,
    hooks: {}
};

export default rules;
