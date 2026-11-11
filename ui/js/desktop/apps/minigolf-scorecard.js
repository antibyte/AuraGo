import { scoreTotal } from './minigolf-model.js';
import { colorValue, formatToPar, fillText } from './minigolf-shared.js';

// Players ordered by total strokes; equal totals share a place (1, 1, 3).
export function rankPlayers(players) {
    const ranked = players.map((player, index) => ({ index, player, total: scoreTotal(player) })).sort((a, b) => a.total - b.total || a.index - b.index);
    ranked.forEach((entry, position) => { entry.place = position && entry.total === ranked[position - 1].total ? ranked[position - 1].place : position + 1; });
    return ranked;
}

// Only finished holes count: completed scores plus the current hole once the player is done.
export function relativeToPar(player, round, courses) {
    const par = id => courses.find(course => course.id === id)?.par ?? 0;
    let diff = player.scores.reduce((sum, score, index) => sum + score - par(round.courseIds[index]), 0);
    if (player.done && !round.completed && round.holeIndex < round.courseIds.length) diff += player.strokes - par(round.courseIds[round.holeIndex]);
    return diff;
}

const grade = diff => diff <= -2 ? 'eagle' : diff === -1 ? 'birdie' : diff === 0 ? 'par' : diff === 1 ? 'bogey' : 'over';
const GRADES = ['ace', 'eagle', 'birdie', 'par', 'bogey', 'over'];

// One stroke is always an ace, whatever the par; otherwise the grade follows strokes against par.
export function holeGrade(strokes, par) { return strokes === 1 ? 'ace' : grade(strokes - par); }

// Banner and hero title of a grade; an over-par hole is just "sunk".
export function gradeTitle(name, tr) { return tr(name === 'ace' ? 'hole_in_one' : name === 'over' ? 'sunk' : name); }

// Holes that count towards the standing: completed scores plus the current hole once the player is done.
export function finishedHoles(player, round) {
    return player.scores.length + (player.done && !round.completed && round.holeIndex < round.courseIds.length ? 1 : 0);
}

// Scorecard table while a round runs; podium, single-player card or practice result once it ended.
export function createScorecard({ showPanel, tr, esc, icon = () => '' }) {
    const dot = player => `<i class="mg-dot" style="--mg-player:${esc(colorValue(player.color))}"></i>`;
    const button = (action, text, primary = false, symbol = '', after = false) => `<button ${primary ? 'class="mg-primary" ' : ''}data-action="${action}">${after || !symbol ? '' : icon(symbol)}<span>${esc(text)}</span>${after && symbol ? icon(symbol) : ''}</button>`;
    const toPar = (player, round, courses) => finishedHoles(player, round) ? esc(formatToPar(relativeToPar(player, round, courses), tr('par'))) : '–';
    const badge = on => on ? `<div class="mg-badge-row"><span class="mg-badge" data-new-best>${esc(tr('new_best'))}</span></div>` : '';
    const endButtons = (...extra) => `<div class="mg-panel-actions">${button('again', tr('play_again'), true, 'replay')}${extra.join('')}${button('menu', tr('menu'), false, 'home')}</div>`;
    const details = state => `<details class="mg-score-details"><summary>${esc(tr('scorecard'))}</summary>${table({ ...state, animate: false })}</details>`;
    const leaders = ({ round, courses }) => {
        const { players } = round;
        if (players.length < 2 || !players.every(player => finishedHoles(player, round) > 0)) return new Set();
        const diffs = players.map(player => relativeToPar(player, round, courses)), low = Math.min(...diffs);
        return diffs.every(diff => diff === low) ? new Set() : new Set(diffs.flatMap((diff, index) => diff === low ? [index] : []));
    };

    function table({ round, courses, animate }, legend = false) {
        const terms = { eagle: tr('eagle'), birdie: tr('birdie'), par: tr('par'), bogey: tr('bogey'), over: tr('over') };
        const columns = round.courseIds.map((id, index) => courses.find(course => course.id === id) || { id, number: index + 1, par: 0 });
        const justFinished = round.completed ? columns.length - 1 : round.players.every(p => p.done) ? round.holeIndex : -1;
        const head = columns.map((course, index) => `<th scope="col"${index === round.holeIndex ? ' class="mg-current"' : ''} title="${esc(tr('course_' + course.number))}">${esc(course.number)}</th>`).join('');
        const ahead = leaders({ round, courses });
        const parRow = `<tr class="mg-par-row"><th scope="row">${esc(tr('par'))}</th>${columns.map(course => `<td>${course.par}</td>`).join('')}<td>${columns.reduce((sum, course) => sum + course.par, 0)}</td><td></td></tr>`;
        const rows = round.players.map((player, row) => {
            const cells = columns.map((course, index) => {
                const played = index < player.scores.length, current = index === round.holeIndex && !round.completed;
                const value = played ? player.scores[index] : current && (player.done || player.strokes > 0) ? player.strokes : null;
                if (value === null) return '<td>–</td>';
                if (!played && !player.done) return `<td class="mg-running">${value}</td>`;
                const name = grade(value - course.par);
                return `<td class="mg-grade-${name}${index === justFinished ? ' mg-pop' : ''}" title="${esc(terms[name])}">${value}</td>`;
            }).join('');
            const leader = ahead.has(row);
            return `<tr${leader ? ' data-leader' : ''} style="--mg-row:${row}"><th scope="row">${dot(player)}<span>${esc(player.name)}</span>${leader ? `<span class="mg-leader">${icon('trophy')}</span>` : ''}</th>${cells}<td class="mg-total mg-pop"><strong>${scoreTotal(player)}</strong></td><td class="mg-topar mg-pop">${toPar(player, round, courses)}</td></tr>`;
        }).join('');
        const key = legend ? `<ul class="mg-legend" aria-hidden="true">${['eagle', 'birdie', 'bogey', 'over'].map(name => `<li class="mg-grade-${name}">${esc(terms[name])}</li>`).join('')}</ul>` : '';
        return `<div class="mg-score-scroll"><table class="mg-score-table" data-animate="${animate ? 'rows' : 'none'}"><thead><tr><th scope="col">${esc(tr('player_name'))}</th>${head}<th scope="col">${esc(tr('total'))}</th><th scope="col">${esc(tr('to_par'))}</th></tr>${parRow}</thead><tbody>${rows}</tbody></table></div>${key}`;
    }
    function progress(state, focus) {
        const { round, courses } = state, done = round.players.every(p => p.done);
        const current = courses.find(course => course.id === round.courseIds[round.holeIndex]);
        const title = done && current ? fillText(tr('hole_done'), { hole: current.number }) : tr('scorecard');
        const [only] = round.players, name = only && current ? holeGrade(only.strokes, current.par) : '';
        const hero = done && round.players.length === 1 && current ? `<p class="mg-grade-hero" data-grade="${name}">${esc(gradeTitle(name, tr))} · ${esc(tr('strokes'))}: ${only.strokes}</p>` : '';
        showPanel(`<h2>${esc(title)}</h2>${hero}${table(state, true)}<div class="mg-panel-actions">${done ? button('next-hole', tr('next_hole'), true, 'chevron', true) : button('close', tr('close'), true)}</div>`, 'scorecard', focus);
    }
    function podium(state, focus) {
        const { round, courses, newBest } = state, ranked = rankPlayers(round.players);
        const winners = ranked.filter(entry => entry.place === 1);
        const title = winners.length > 1 ? tr('tie') : fillText(tr('wins'), { name: winners[0].player.name });
        // Second place stands left, first in the middle, third right. Two players get only two
        // steps, centred as a pair (data-count), so no empty third column unbalances the podium.
        const steps = [[ranked[1], 'left'], [ranked[0], 'center'], [ranked[2], 'right']].filter(([entry]) => entry);
        const step = ([entry, slot]) => `<li class="mg-step" data-place="${entry.place}" data-slot="${slot}" style="--mg-player:${esc(colorValue(entry.player.color))}">${entry.place === 1 ? `<span class="mg-step-crown">${icon('trophy')}</span>` : ''}<span class="mg-step-ball"></span><span class="mg-step-name">${esc(entry.player.name)}</span><span class="mg-step-total">${entry.total}</span><span class="mg-step-par">${toPar(entry.player, round, courses)}</span><span class="mg-step-block"><b>${entry.place}</b></span></li>`;
        const rest = entry => `<p class="mg-podium-rest">${dot(entry.player)}<span>${esc(fillText(tr('place'), { n: entry.place }))}</span><strong>${esc(entry.player.name)}</strong><span>${entry.total} · ${toPar(entry.player, round, courses)}</span></p>`;
        showPanel(`<h2 class="mg-winner">${esc(title)}</h2>${badge(newBest)}<ol class="mg-podium" data-count="${steps.length}">${steps.map(step).join('')}</ol>${ranked.slice(3).map(rest).join('')}${details(state)}${endButtons()}`, 'finished', focus);
    }
    function single(state, focus) {
        const { round, courses, best, newBest } = state, [player] = round.players;
        const counts = new Map();
        player.scores.forEach((score, index) => { const name = holeGrade(score, courses.find(course => course.id === round.courseIds[index])?.par ?? 0); counts.set(name, (counts.get(name) || 0) + 1); });
        const summary = `<ul class="mg-grade-summary">${GRADES.filter(name => counts.has(name)).map(name => `<li class="mg-grade-${name}">${esc(counts.get(name) + '× ' + tr(name === 'ace' ? 'hole_in_one' : name))}</li>`).join('')}</ul>`;
        showPanel(`<h2>${esc(tr('finished'))}</h2><div class="mg-result-card" style="--mg-player:${esc(colorValue(player.color))}">${badge(newBest)}<span class="mg-result-total">${scoreTotal(player)}</span><span class="mg-result-label">${esc(tr('strokes'))}</span><span class="mg-result-par">${toPar(player, round, courses)}</span><span class="mg-result-best">${esc(tr('best'))}: ${esc(best?.rounds?.[round.mode] ?? '–')}</span></div>${summary}${details(state)}${endButtons()}`, 'finished', focus);
    }
    function practice(state, focus) {
        const { round, courses, best, newBest } = state;
        const rows = round.players.map(player => `<li>${dot(player)}<span class="mg-result-name">${esc(player.name)}</span><strong>${player.scores[0]}</strong><span>${toPar(player, round, courses)}</span></li>`).join('');
        showPanel(`<h2>${esc(tr('finished'))}</h2>${badge(newBest)}<ul class="mg-result-list">${rows}</ul><p class="mg-result-best">${esc(tr('hole_best'))}: ${esc(best?.holes?.[round.courseIds[0]] ?? '–')}</p>${endButtons(button('holes', tr('other_hole'), false, 'flag'))}`, 'finished', focus);
    }

    return {
        scorecard(state, focus) { (!state.round.completed ? progress : state.round.mode === 'practice' ? practice : state.round.players.length > 1 ? podium : single)(state, focus); }
    };
}
