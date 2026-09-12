#!/usr/bin/env node

import fs from 'node:fs';
import path from 'node:path';

const PROVIDERS = ['agnesai', 'stepfun'];
const TASKS = ['breakout', 'platformer', 'forest-fps', 'hybrid'];
const VARIANTS = ['baseline', 'candidate'];

// These are the fixed acceptance briefs used by game_maker_builder_eval_test.go.
// Existing report metadata is preferred when present; this fallback keeps a
// missing-run row auditable without copying the full user brief into output.
const FALLBACK_REQUIREMENTS = {
    breakout: [
        auto('identity', 'accepted plan retains the Neon Orchard identity', ['neon orchard']),
        auto('rules', 'accepted plan states lives, scoring and reachable brick play', ['three lives', 'ten points', 'reachable']),
        auto('custom_mechanic', 'accepted plan preserves the every-fifth collectible paddle mechanic', ['every fifth', 'paddle', 'eight seconds']),
        auto('terminal_states', 'accepted plan distinguishes victory after clearing and defeat after misses', ['victory', 'defeat']),
        manual('visual_identity', 'inspect the running game for a coherent Neon Orchard identity, readable HUD and clear terminal states'),
        manual('gameplay_quality', 'play the exported game to verify reachability, collectible timing and natural win/loss behavior'),
    ],
    platformer: [
        auto('identity', 'accepted plan retains the Moss Post woodland identity', ['moss post']),
        auto('level_contract', 'accepted plan states six varied platforms and a reachable exit', ['six', 'platform', 'reachable exit']),
        auto('collectibles', 'accepted plan requires four letters before the exit opens', ['four', 'letters', 'exit']),
        auto('safety', 'accepted plan preserves crossable gaps and stable start and exit areas', ['crossable', 'stable', 'start', 'exit']),
        manual('visual_identity', 'inspect the running game for friendly woodland presentation and readable letters/lives HUD'),
        manual('gameplay_quality', 'play the exported game to verify every gap, collectible gate and falling-life behavior'),
    ],
    'forest-fps': [
        auto('identity', 'accepted plan retains the Lantern Patrol identity', ['lantern patrol']),
        auto('world_contract', 'accepted plan names local low-poly forest assets and five targets', ['low-poly', 'five', 'target']),
        auto('combat_rules', 'accepted plan states safe routes, health and five-target win/loss rules', ['safe', 'health', 'five']),
        auto('hit_feedback', 'accepted plan binds effects and sounds to actual hits', ['muzzle-flash', 'actual hits']),
        manual('visual_identity', 'inspect the running game for Lantern Patrol identity, legible objective/health and readable cover'),
        manual('gameplay_quality', 'play the exported game to verify navigable routes, target damage and terminal outcomes'),
    ],
    hybrid: [
        auto('identity', 'accepted plan retains the Quiet Orbit Courier identity', ['quiet orbit courier']),
        auto('delivery_loop', 'accepted plan states three delivery rings and a destination marker', ['three', 'delivery', 'destination']),
        auto('custom_lantern_rule', 'accepted plan records the custom lantern charge, brake and drain rule', ['lantern', 'brake', 'drain']),
        auto('peaceful_outcome', 'accepted plan explicitly avoids combat and countdown loss', ['no combat', 'no countdown', 'win']),
        manual('visual_identity', 'inspect the running game for calm blue/gold space identity and readable charge/delivery state'),
        manual('gameplay_quality', 'play the exported game to verify ring timing, charge/drain behavior and peaceful completion'),
    ],
};

function auto(id, summary, allOf) {
    return {id, mode: 'automatic', summary, all_of: allOf};
}

function manual(id, summary) {
    return {id, mode: 'manual', summary};
}

function usage(message = '') {
    if (message) console.error(`error: ${message}`);
    console.error('usage: node scripts/test/eval/game-maker-builder-eval.mjs [--baseline-dir DIR] [--candidate-dir DIR] [--out FILE] [--format json|markdown]');
    process.exitCode = message ? 2 : 0;
}

function parseArgs(argv) {
    const args = {baselineDir: 'reports/game-maker-builder-baseline', candidateDir: 'reports/game-maker-builder-eval', format: 'json'};
    for (let i = 0; i < argv.length; i += 1) {
        const arg = argv[i];
        if (arg === '--help' || arg === '-h') return {...args, help: true};
        const [key, inline] = arg.split('=', 2);
        const names = {'--baseline-dir': 'baselineDir', '--candidate-dir': 'candidateDir', '--out': 'out', '--format': 'format'};
        const name = names[key];
        if (!name) throw new Error(`unknown option ${arg}`);
        const value = inline ?? argv[++i];
        if (!value) throw new Error(`${key} requires a value`);
        args[name] = value;
    }
    if (!['json', 'markdown'].includes(args.format)) throw new Error('--format must be json or markdown');
    return args;
}

function readReport(dir, provider, task) {
    const file = path.join(dir, `${provider}-${task}.json`);
    if (!fs.existsSync(file)) return {file, status: 'missing'};
    try {
        return {file, status: 'present', report: JSON.parse(fs.readFileSync(file, 'utf8'))};
    } catch (error) {
        return {file, status: 'invalid', error: String(error.message || error)};
    }
}

function planText(plan) {
    if (!plan || typeof plan !== 'object') return '';
    // The report generator only checks bounded accepted-plan metadata. It does
    // not emit or index the prompt, source, event payloads, or generated code.
    return JSON.stringify(plan).toLowerCase();
}

function requirementsFor(task, report) {
    const supplied = report?.brief_requirements;
    if (Array.isArray(supplied) && supplied.length > 0) return supplied;
    return FALLBACK_REQUIREMENTS[task] || [manual('visual_identity', 'inspect the running game for a coherent requested identity'), manual('gameplay_quality', 'play the exported game to verify the requested loop')];
}

function checkRequirements(task, report, rowStatus) {
    const requirements = requirementsFor(task, report);
    if (rowStatus === 'missing' || rowStatus === 'invalid') {
        return requirements.map(req => ({id: req.id, mode: req.mode, summary: req.summary, status: rowStatus}));
    }
    const text = planText(report?.plan);
    return requirements.map(req => {
        if (req.mode === 'manual') return {id: req.id, mode: req.mode, summary: req.summary, status: 'manual_review'};
        const allOf = Array.isArray(req.all_of) ? req.all_of : [];
        const anyOf = Array.isArray(req.any_of) ? req.any_of : [];
        const missing = allOf.filter(term => !text.includes(String(term).toLowerCase()));
        const anyMatched = anyOf.length === 0 || anyOf.some(term => text.includes(String(term).toLowerCase()));
        // These checks describe what the accepted plan declared. They are never a
        // gameplay or technical verdict; runtime evidence and manual play remain
        // separate acceptance inputs.
        return {
            id: req.id,
            mode: req.mode,
            summary: req.summary,
            status: missing.length === 0 && anyMatched ? 'plan_declared' : 'plan_not_declared',
            evidence_source: 'accepted_plan_metadata',
            ...(missing.length > 0 ? {missing_terms: missing} : {}),
            ...(anyOf.length > 0 && !anyMatched ? {missing_any_of: anyOf} : {}),
            ...(!text ? {reason: 'accepted plan metadata is absent from this report'} : {}),
        };
    });
}

function deriveMetrics(report) {
    if (report && report.event_metrics && typeof report.event_metrics === 'object') return report.event_metrics;
    const events = Array.isArray(report?.events) ? report.events : [];
    const typeCounts = {};
    let building = 0;
    let validationResults = 0;
    let validationFailures = 0;
    let skillActivations = 0;
    let toolCallEvents = 0;
    for (const event of events) {
        const type = typeof event?.type === 'string' ? event.type : 'unknown';
        typeCounts[type] = (typeCounts[type] || 0) + 1;
        if (type === 'phase' && event?.payload?.phase === 'building') building += 1;
        if (type === 'skill_activation') skillActivations += 1;
        if (type === 'tool_call') toolCallEvents += 1;
        if (type === 'validation_result') {
            validationResults += 1;
            const result = event?.payload?.result;
            if (result?.gameplay_status === 'failed' || result?.ok === false) validationFailures += 1;
        }
    }
    const complete = events.length < 500;
    return {
        event_count: events.length,
        complete,
        type_counts: typeCounts,
        tool_calls: complete && toolCallEvents > 0 ? toolCallEvents : null,
        tool_calls_reason: complete && toolCallEvents > 0 ? 'counted persisted tool_call events' : 'persisted Game Maker events contain no tool_call records; skill_activations is the only tool-related event counter',
        skill_activations: skillActivations,
        repair_rounds: complete && building > 0 ? Math.max(0, building - 1) : null,
        repair_rounds_reason: complete && building > 0 ? 'counted phase=building events after the initial build' : 'repair count is unavailable because the bounded phase event stream is incomplete or has no building boundary',
        validation_results: validationResults,
        validation_failures: validationFailures,
        token_usage: null,
        token_usage_reason: 'the evaluation broker does not retain provider usage and persisted Game Maker events have no numeric usage fields',
    };
}

function makeRow(variant, dir, provider, task) {
    const input = readReport(dir, provider, task);
    const base = {run_key: `${variant}/${provider}/${task}`, variant, provider, task, expected: true};
    if (input.status === 'missing') {
        return {...base, status: 'missing', report_file: input.file, plan_declaration_status: 'missing', manual_status: 'missing', requirements: checkRequirements(task, null, 'missing')};
    }
    if (input.status === 'invalid') {
        return {...base, status: 'invalid', report_file: input.file, error: input.error, plan_declaration_status: 'invalid', manual_status: 'invalid', requirements: checkRequirements(task, null, 'invalid')};
    }
    const report = input.report;
    const jobStatus = typeof report?.job?.status === 'string' ? report.job.status : 'unknown';
    const requirements = checkRequirements(task, report, 'present');
    const declarations = requirements.filter(req => req.mode === 'automatic');
    const planDeclarationStatus = declarations.length === 0 ? 'none' :
        declarations.every(req => req.status === 'plan_declared') ? 'declared' : 'not_declared';
    return {
        ...base,
        status: jobStatus,
        report_file: input.file,
        model: typeof report?.model === 'string' ? report.model : null,
        seconds: Number.isFinite(report?.seconds) ? report.seconds : null,
        job_status: jobStatus,
        job_phase: typeof report?.job?.phase === 'string' ? report.job.phase : null,
        plan_declaration_status: planDeclarationStatus,
        manual_status: requirements.some(req => req.mode === 'manual') ? 'required' : 'none',
        requirements,
        event_metrics: deriveMetrics(report),
    };
}

function makeEvaluation(args) {
    const rows = [];
    for (const variant of VARIANTS) {
        const dir = variant === 'baseline' ? args.baselineDir : args.candidateDir;
        for (const provider of PROVIDERS) for (const task of TASKS) rows.push(makeRow(variant, dir, provider, task));
    }
    return {
        schema_version: 2,
        expected_runs: VARIANTS.length * PROVIDERS.length * TASKS.length,
        generated_at: new Date().toISOString(),
        source: {baseline_dir: args.baselineDir, candidate_dir: args.candidateDir, providers: PROVIDERS, tasks: TASKS},
        summary: {
            present_runs: rows.filter(row => row.status !== 'missing' && row.status !== 'invalid').length,
            missing_runs: rows.filter(row => row.status === 'missing').length,
            invalid_runs: rows.filter(row => row.status === 'invalid').length,
            ready_runs: rows.filter(row => row.status === 'ready').length,
            plan_declared_runs: rows.filter(row => row.plan_declaration_status === 'declared').length,
            plan_not_declared_runs: rows.filter(row => row.plan_declaration_status === 'not_declared').length,
            manual_review_runs: rows.filter(row => row.manual_status === 'required').length,
        },
        rows,
    };
}

function markdown(evaluation) {
    const lines = [
        '# Game Maker builder evaluation',
        '',
        `Expected ${evaluation.expected_runs} fixed runs; present ${evaluation.summary.present_runs}, missing ${evaluation.summary.missing_runs}, invalid ${evaluation.summary.invalid_runs}.`,
        '',
        '| Variant | Provider | Task | Run status | Plan declarations | Manual review | Tool calls | Repairs | Tokens |',
        '|---|---|---|---|---|---|---:|---:|---|',
    ];
    for (const row of evaluation.rows) {
        const metrics = row.event_metrics || {};
        const token = metrics.token_usage == null ? 'null' : String(metrics.token_usage.total_tokens ?? 'n/a');
        lines.push(`| ${row.variant} | ${row.provider} | ${row.task} | ${row.status} | ${row.plan_declaration_status} | ${row.manual_status} | ${metrics.tool_calls ?? 'null'} | ${metrics.repair_rounds ?? 'null'} | ${token} |`);
    }
    lines.push('', 'Plan declarations inspect accepted-plan metadata only and never certify runtime behavior. Missing terms are not functional failures; manual running-game identity and gameplay checks remain required. Technical readiness does not certify those qualities.', '');
    return lines.join('\n');
}

let args;
try {
    args = parseArgs(process.argv.slice(2));
} catch (error) {
    usage(error.message || String(error));
}
if (args?.help) {
    usage();
} else if (args) {
    const evaluation = makeEvaluation(args);
    const output = args.format === 'markdown' ? markdown(evaluation) : `${JSON.stringify(evaluation, null, 2)}\n`;
    if (args.out) fs.writeFileSync(path.resolve(args.out), output, {encoding: 'utf8', mode: 0o600});
    else process.stdout.write(output);
}
