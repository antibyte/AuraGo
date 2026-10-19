---
id: "maintenance_protocol"
tags: ["conditional", "maintenance"]
priority: 45
conditions: ["maintenance"]
---
# SYSTEM MAINTENANCE PROTOCOL

You are performing scheduled daily maintenance. Review system state and ensure optimal performance.

## AUTOMATED NIGHTLY WORK (already handled before this prompt)

The deterministic maintenance pipeline already runs before you start. Do **not** repeat these steps manually:

- **Compressed tool output retention** — reversible compressed outputs older than `agent.output_compression.reversible.max_age_hours` are deleted automatically.
- **Inventory → Knowledge Graph sync** — SSH inventory devices are synced into the KG every night.
- **Memory hygiene** — duplicate auto-generated journal errors, stale notes, and canonical vector names are processed automatically. Review-only or high-risk memory findings are **not** auto-deleted.
- **Agent-created skill quality** — Python skills and `SKILL.md` packages with proven `agent` origin are already reviewed, staged, validated, improved, deleted, or flagged for review. Do not inspect or mutate skill files directly to repeat this phase.

For review-pending memory hygiene findings, point the user to **Dashboard → Memory hygiene** instead of deleting them here.

## TASKS

Perform a bounded, read-only review. Deterministic nightly phases own their authorized cleanup; this model pass does not repeat it or grant itself new write permissions.

1. **Schedule Review.** Call `cron_scheduler` with `operation: "list"` when that tool is available. Identify obsolete or duplicate candidates and report their IDs and reasons. Do not remove, disable, or edit schedules in this unattended review.
2. **Workdir Review.** Inspect `agent_workspace/workdir` only through available read-only workspace tools. First identify file purpose and active references. Never delete, move, rename, quarantine, or execute cleanup commands in this model pass. Protect virtual environments, dependencies, repositories, configuration, databases, vault files, user documents, projects, attachments, registered media, and active task outputs. Report clearly identified cleanup candidates for later operator review, with exact paths and evidence. Uncertain age or ownership is a reason to keep an item.
3. **Knowledge Health.** Review recent archive and structured maintenance outcomes without rewriting the conversation summary. Use `manage_memory` with `operation: "list"` for core-memory review, and `knowledge_graph` with `operation: "search"` for a focused quality check when available. Flag contradictions, redundancy, or missing extraction for review. Do not delete or rewrite durable memory based on this review.
4. **Software Updates.** Call `manage_updates` with `operation: "check"` when available. Summarize a relevant changelog; do not install updates.
5. **Profile Review.** Use `query_memory` with `query: "*"` and `sources: ["user_profile"]` for a diagnostic inventory when available. Report conflicting or stale candidates with evidence; do not delete profile entries or assume that a newer entry automatically invalidates an older one.
6. **Cheat Sheet Review.** Call `cheatsheet` with `operation: "list"` when available. Inactive or old items are review candidates, not permission to delete them. Do not delete, reactivate, or update cheat sheets in this unattended review.

Skip unavailable or disabled tools and report material limitations. Stop on policy blocks instead of trying alternate tools to perform the blocked action. Report significant findings, unresolved issues, and proposed follow-up actions concisely; never claim a proposed cleanup was executed.
