#!/usr/bin/env node
// Summarise Game Maker benchmark reports instead of running one-off forensics.
//
// Usage:
//   node scripts/game-maker-forensics.mjs
//   node scripts/game-maker-forensics.mjs --dir reports/game-maker-benchmark --out reports/game-maker-forensics.md
//
// Reads the JSON files written by scripts/game-maker-benchmark.mjs. "playable"
// is a human judgement; this script never invents a quality verdict and warns
// when no case has been judged.

import { mkdir, readdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const args = process.argv.slice(2);
const value = (flag, fallback) => {
  const index = args.indexOf(flag);
  return index >= 0 && args[index + 1] ? args[index + 1] : fallback;
};
const dir = value("--dir", "reports/game-maker-benchmark");
const out = value("--out", "");

let files = [];
try {
  files = (await readdir(dir)).filter(name => name.endsWith(".json")).sort();
} catch (error) {
  console.error(`No benchmark reports in ${dir}: ${error.message}`);
  console.error("Run scripts/game-maker-benchmark.mjs first. A missing report is not a result.");
  process.exit(2);
}
if (files.length === 0) {
  console.error(`No benchmark JSON in ${dir}. A missing report is not a result.`);
  process.exit(2);
}

const rows = [];
for (const file of files) {
  try {
    const report = JSON.parse(await readFile(path.join(dir, file), "utf8"));
    for (const entry of report.cases || []) {
      rows.push({
        file,
        provider: report.provider || "",
        model: report.model || "",
        commit: report.commit || "unknown",
        case_id: entry.case_id || "?",
        status: entry.status || "?",
        error_code: entry.error_code || "",
        repairs: entry.repair_passes ?? null,
        calls: entry.usage_calls ?? null,
        seconds: entry.duration_seconds ?? null,
        playable: entry.playable,
      });
    }
  } catch (error) {
    console.error(`Skipping ${file}: ${error.message}`);
  }
}

function median(values) {
  const sorted = values.filter(value => Number.isFinite(value)).sort((a, b) => a - b);
  if (sorted.length === 0) return "?";
  const middle = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[middle] : Math.round((sorted[middle - 1] + sorted[middle]) / 2);
}

function summarise(name, entries) {
  const count = entries.length;
  const ready = entries.filter(entry => entry.status === "ready").length;
  const preview = entries.filter(entry => entry.error_code === "preview").length;
  const model = entries.filter(entry => entry.error_code === "model").length;
  const judged = entries.filter(entry => typeof entry.playable === "boolean").length;
  const falseSuccess = entries.filter(entry => entry.playable === false).length;
  return {
    name, count, ready, preview, model, judged, falseSuccess,
    firstTry: entries.filter(entry => entry.status === "ready" && entry.repairs === 0).length,
    medianSeconds: median(entries.map(entry => entry.seconds)),
    medianCalls: median(entries.map(entry => entry.calls)),
  };
}

const groups = new Map();
for (const row of rows) {
  const key = `${row.model || "?"} @ ${row.commit}`;
  if (!groups.has(key)) groups.set(key, []);
  groups.get(key).push(row);
}
const summaries = [summarise("all", rows)];
for (const [key, entries] of groups) summaries.push(summarise(key, entries));

const table = [
  "| Scope | Cases | Ready | First try | Preview errors | Model errors | Judged | False successes | Median s | Median calls |",
  "| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |",
  ...summaries.map(item => `| ${item.name} | ${item.count} | ${item.ready} | ${item.firstTry} | ${item.preview} | ${item.model} | ${item.judged} | ${item.falseSuccess} | ${item.medianSeconds} | ${item.medianCalls} |`),
].join("\n");

const errors = new Map();
for (const row of rows) {
  if (!row.error_code) continue;
  const key = `${row.error_code}`;
  errors.set(key, (errors.get(key) || 0) + 1);
}
const errorLines = [...errors.entries()].sort((a, b) => b[1] - a[1])
  .map(([code, count]) => `- ${code}: ${count}`).join("\n") || "- none recorded";

const judged = rows.filter(row => typeof row.playable === "boolean").length;
const warning = judged === 0
  ? "\n> No case has a human `playable` judgement. Status `ready` is not evidence that the game plays correctly; fill `playable` in the benchmark JSON before drawing quality conclusions.\n"
  : `\n> ${judged}/${rows.length} cases have a human playability judgement.\n`;

const markdown = `# Game Maker forensics (${new Date().toISOString().slice(0, 10)})\n\nSource: \`${dir}\` · ${files.length} report(s) · ${rows.length} case(s)\n\n${table}\n\n## Terminal error codes\n\n${errorLines}\n${warning}\n`;
console.log(markdown);
if (out) {
  await mkdir(path.dirname(out), { recursive: true });
  await writeFile(out, markdown, "utf8");
  console.log(`wrote ${out}`);
}
