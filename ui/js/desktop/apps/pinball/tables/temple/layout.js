// Lost Temple of the Sun: layout stub (shell + lower kit + meta); the table task adds playfield features.
import { composeLayout } from '../kit/compose.js';
import { shell } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';

export const layout = composeLayout(
    { id: 'temple', version: 1, meta: {
        title: 'Lost Temple of the Sun', accent: '#ffcb65', secondary: '#8ad6a3', cx: CX, troughSize: 4,
        giColor: { lower: '#ffd59a', upper: '#ffb347' },
        attract: [['sweepUp', 1600], ['ring', 1600], ['spiral', 2000], ['sparkle', 2400], ['radial', 1500]]
    } },
    shell(),
    lowerKit({ cx: CX, kickback: 'left' }),
    { gi: [
        { id: 'lower', color: '#ffd59a', yMin: 0, yMax: 0.55, bulbs: [] },
        { id: 'upper', color: '#ffb347', yMin: 0.55, yMax: 1.07, bulbs: [] }
    ] }
);

export default layout;
