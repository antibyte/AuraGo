#!/usr/bin/env node
// Weak-model benchmark runner for Game Maker Studio.
//
// Reads documentation/game-maker-comparison.json, starts the fixed cases against
// a running AuraGo instance with one explicitly selected provider/model and
// records the terminal job outcome. "playable" stays a manual judgement.
//
// Usage:
//   node scripts/game-maker-benchmark.mjs --dry-run
//   AURAGO_BASE_URL=https://127.0.0.1:8443 AURAGO_TOKEN=... \
//   GM_PROVIDER=main GM_MODEL=<model> node scripts/game-maker-benchmark.mjs
//
// Optional: --cases shooter,platformer  --out reports/game-maker-benchmark
// Environment: GM_TIMEOUT (seconds, default 2700), GM_COMMIT (recorded in the report)

import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";

const SPEC_PATH = "documentation/game-maker-comparison.json";
const TERMINAL = new Set(["ready", "failed", "cancelled"]);
const args = process.argv.slice(2);
const dryRun = args.includes("--dry-run");
const baseUrl = (process.env.AURAGO_BASE_URL || "").replace(/\/+$/, "");
const token = process.env.AURAGO_TOKEN || "";
const provider = process.env.GM_PROVIDER || "";
const model = process.env.GM_MODEL || "";
const timeoutMs = Number(process.env.GM_TIMEOUT || 2700) * 1000;

function flagValue(flag) {
  const index = args.indexOf(flag);
  return index >= 0 && index + 1 < args.length ? args[index + 1] : "";
}

const outDir = flagValue("--out") || "reports/game-maker-benchmark";
const wanted = flagValue("--cases").split(",").map((value) => value.trim()).filter(Boolean);

const spec = JSON.parse(await readFile(SPEC_PATH, "utf8"));
const cases = spec.cases.filter((entry) => wanted.length === 0 || wanted.includes(entry.id));

if (cases.length === 0) {
  console.error(`no benchmark case matched ${wanted.join(",")}`);
  process.exit(2);
}
if (dryRun) {
  for (const entry of cases) console.log(`${entry.id}\t${entry.prompt}`);
  console.log(`\n${cases.length} case(s) from ${SPEC_PATH} (${spec.policy})`);
  process.exit(0);
}
if (!baseUrl || !token || !provider || !model) {
  console.error("AURAGO_BASE_URL, AURAGO_TOKEN, GM_PROVIDER and GM_MODEL are required");
  process.exit(2);
}

async function api(method, route, body) {
  const response = await fetch(baseUrl + route, {
    method,
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const text = await response.text();
  let data = null;
  try {
    data = text ? JSON.parse(text) : null;
  } catch {
    data = null;
  }
  if (!response.ok) {
    const detail = data?.error || data?.message || text.slice(0, 200);
    throw new Error(`${method} ${route} -> ${response.status}: ${detail}`);
  }
  return data;
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function waitForJob(projectID, jobID) {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const record = await api("GET", `/api/game-maker/projects/${projectID}`);
    const latest = record?.latest_job;
    if (latest?.id === jobID && TERMINAL.has(latest.status)) return { job: latest, project: record.project };
    await sleep(5000);
  }
  throw new Error(`timed out after ${timeoutMs / 1000}s waiting for job ${jobID}`);
}

function summarise(entry, job, startedAt) {
  return {
    case_id: entry.id,
    job_id: job.id,
    status: job.status,
    error_code: job.error_code || "",
    checks_passed: job.checks_passed ?? null,
    checks_failed: job.checks_failed ?? null,
    repair_passes: job.repair_passes ?? null,
    gameplay_status: job.gameplay_status || "",
    usage_calls: job.usage?.calls ?? null,
    usage_tokens: job.usage?.tokens ?? null,
    usage_cost_usd: job.usage?.cost_usd ?? null,
    duration_seconds: job.started_at
      ? Math.round((Date.parse(job.finished_at || Date.now()) - Date.parse(job.started_at)) / 1000)
      : Math.round((Date.now() - startedAt) / 1000),
    playable: null,
    notes: "",
  };
}

const stamp = new Date().toISOString().slice(0, 10);
const results = [];
for (const entry of cases) {
  const startedAt = Date.now();
  const project = await api("POST", "/api/game-maker/projects", {
    name: `bench-${entry.id}-${Date.now()}`,
    dimension: "2d",
    description: entry.prompt,
    provider_id: provider,
    model,
  });
  const job = await api("POST", `/api/game-maker/projects/${project.id}/jobs`, {
    prompt: entry.prompt,
    provider_id: provider,
    model,
  });
  let terminal = null;
  try {
    terminal = await waitForJob(project.id, job.id);
  } catch (error) {
    results.push({ case_id: entry.id, job_id: job.id, status: "timeout", error_code: "benchmark_timeout", notes: error.message, playable: null });
    continue;
  }
  const record = summarise(entry, terminal.job, startedAt);
  results.push(record);
  console.log(`${entry.id}: ${record.status} (${record.duration_seconds}s, repairs=${record.repair_passes ?? "?"})`);
}

const report = {
  generated_at: new Date().toISOString(),
  base_url: baseUrl,
  provider,
  model,
  commit: process.env.GM_COMMIT || "",
  policy: spec.policy,
  cases: results,
};
await mkdir(outDir, { recursive: true });
const jsonPath = path.join(outDir, `${stamp}-${model.replace(/[^A-Za-z0-9._-]+/g, "_")}.json`);
await writeFile(jsonPath, `${JSON.stringify(report, null, 2)}\n`, "utf8");
const rows = results.map((entry) => `| ${entry.case_id} | ${entry.status} | ${entry.repair_passes ?? "?"} | ${entry.usage_calls ?? "?"} | ${entry.duration_seconds ?? "?"} |`).join("\n");
await writeFile(jsonPath.replace(/\.json$/, ".md"), `# Game Maker weak-model benchmark (${stamp})\n\nProvider: ${provider} · Model: ${model} · Commit: ${report.commit || "unknown"}\n\n| Case | Status | Repairs | Calls | Seconds |\n| --- | --- | --- | --- | --- |\n${rows}\n\nFill \`playable\` in the JSON manually after watching each game.\n`, "utf8");
console.log(`\nwrote ${jsonPath}`);
