// Shared lower playfield kit: flippers, dividers, inlanes, slings, rollovers, kickback, lamps (plan section 5). DOM-free.
import { post, wall, bandSegments } from './geom.js';

export const CX = -0.017;
const POST_R = 0.0055;
const COLORS = Object.freeze({ lane: '#ffd34d', kickback: '#ff5a3c', saver: '#ff3b3b', bonus: '#ffb347' });

// opts: { cx, kickback: 'left' | 'right' | null }
export function lowerKit({ cx = CX, kickback = null } = {}) {
    const part = { flippers: [], walls: [], posts: [], devices: [], lamps: [] };
    for (const [side, sgn] of [['L', -1], ['R', 1]]) {
        const X = u => cx - sgn * u; // u is the signed left-side offset from the centre line (negative); right mirrors it
        const s = side;
        part.flippers.push({ id: side === 'L' ? 'left' : 'right', key: side === 'L' ? 'left' : 'right', x: X(-0.080), y: 0.160, side: -sgn });
        // Divider between inlane and outlane, and the inlane guide down to the flipper.
        part.walls.push(wall('divider' + s, 'metal', [[X(-0.2045), 0.400], [X(-0.2045), 0.2376]]));
        part.walls.push(wall('inlaneGuide' + s, 'metal', [[X(-0.2045), 0.2376], [X(-0.0955), 0.1714]]));
        part.posts.push(post('dividerPost' + s, X(-0.2045), 0.4045, 0.0045, 'rubber'));
        // Sling: three rubber posts joined by a rubber band offset outward by the post radius.
        const S = [[X(-0.1665), 0.360], [X(-0.1665), 0.2586], [X(-0.1150), 0.2273]];
        S.forEach((p, i) => part.posts.push(post('sling' + s + '_p' + (i + 1), p[0], p[1], POST_R, 'rubber')));
        const band = bandSegments(S, POST_R);
        part.walls.push(wall('sling' + s + '_face', 'rubber', [band[2][1], band[2][0]]));
        part.walls.push(wall('sling' + s + '_side', 'rubber', band[0]));
        part.walls.push(wall('sling' + s + '_base', 'rubber', band[1]));
        part.devices.push({ id: 'sling' + s, type: 'sling', face: 'sling' + s + '_face' });
        // Lane switches and arrows.
        part.devices.push({ id: 'inlane' + s, type: 'rollover', x: X(-0.1855), y: 0.300 });
        part.devices.push({ id: 'outlane' + s, type: 'rollover', x: X(-0.2225), y: 0.300 });
        for (const lane of ['inlane', 'outlane']) {
            part.lamps.push({ id: lane + s, x: X(lane === 'inlane' ? -0.1855 : -0.2225), y: 0.335, shape: 'arrow', w: 0.014, h: 0.022, angle: Math.PI, color: COLORS.lane, mount: 'insert', device: lane + s, group: 'lanes' });
        }
        if (kickback === (side === 'L' ? 'left' : 'right')) {
            part.devices.push({ id: 'kickback' + s, type: 'kickback', x: X(-0.2225), y: 0.150, r: 0.012, speed: 3.6, armed: false });
            part.lamps.push({ id: 'kickback' + s, x: X(-0.2225), y: 0.205, shape: 'roundRect', w: 0.022, h: 0.016, angle: 0, color: COLORS.kickback, label: 'lamp_kickback', mount: 'insert', device: 'kickback' + s });
        }
    }
    part.lamps.push({ id: 'shootAgain', x: cx, y: 0.215, shape: 'shield', w: 0.030, h: 0.024, angle: 0, color: COLORS.saver, label: 'lamp_shoot_again', mount: 'insert' });
    [['bonus2x', -0.045, 0.285], ['bonus3x', -0.015, 0.295], ['bonus4x', 0.015, 0.295], ['bonus5x', 0.045, 0.285]].forEach(([id, u, y], i) => {
        part.lamps.push({ id, x: cx + u, y, shape: 'circle', w: 0.016, h: 0.016, angle: 0, color: COLORS.bonus, text: (i + 2) + '×', mount: 'insert', group: 'bonus' });
    });
    return part;
}
