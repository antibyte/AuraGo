// Deep Sea Leviathan: layout stub (shell + lower kit + meta); the table task adds playfield features.
import { composeLayout } from '../kit/compose.js';
import { shell } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';

export const layout = composeLayout(
    { id: 'leviathan', version: 1, meta: {
        title: 'Deep Sea Leviathan', accent: '#43e3d5', secondary: '#dcad63', cx: CX, troughSize: 4,
        giColor: { lower: '#bff6ff', upper: '#7fd3ff' },
        attract: [['breath', 2400], ['sweepUp', 1800], ['radial', 1600], ['sparkle', 2400], ['ring', 1600]]
    } },
    shell(),
    lowerKit({ cx: CX, kickback: 'right' }),
    { gi: [
        { id: 'lower', color: '#bff6ff', yMin: 0, yMax: 0.55, bulbs: [] },
        { id: 'upper', color: '#7fd3ff', yMin: 0.55, yMax: 1.07, bulbs: [] }
    ] }
);

export default layout;
