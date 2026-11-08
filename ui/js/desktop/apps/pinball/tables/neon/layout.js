// Cyber Neon Metropolis: layout stub (shell + lower kit + meta); the table task adds playfield features.
import { composeLayout } from '../kit/compose.js';
import { shell } from '../kit/shell.js';
import { lowerKit, CX } from '../kit/lower.js';

export const layout = composeLayout(
    { id: 'neon', version: 1, meta: {
        title: 'Cyber Neon Metropolis', accent: '#37d9ff', secondary: '#ff48c4', cx: CX, troughSize: 4,
        giColor: { lower: '#d8f0ff', upper: '#c9b6ff' },
        attract: [['sweepUp', 1600], ['chase', 2400], ['radial', 1500], ['sparkle', 2500], ['spiral', 2000], ['strobe', 600], ['rainbow', 3000]]
    } },
    shell(),
    lowerKit({ cx: CX, kickback: 'left' }),
    { gi: [
        { id: 'lower', color: '#d8f0ff', yMin: 0, yMax: 0.55, bulbs: [] },
        { id: 'upper', color: '#c9b6ff', yMin: 0.55, yMax: 1.07, bulbs: [] }
    ] }
);

export default layout;
