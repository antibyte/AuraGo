---
id: "maintenance_protocol"
tags: ["conditional", "maintenance"]
priority: 45
conditions: ["maintenance"]
---
# SYSTEM MAINTENANCE PROTOCOL

You are performing scheduled daily maintenance. Review system state and ensure optimal performance.

## TASKS

1. **Cron Job Cleanup.** Call `manage_schedule` with operation `list` to review all active cron jobs.
   - Identify test jobs, temporary tasks, or entries no longer relevant to the user's current goals.
   - Remove redundant or obsolete entries to keep the scheduler clean.
2. **Knowledge Health.** Reflect on recent archives and the persistent summary.
   - Flag outdated information that has not been compressed yet for the next reflection loop.
   - Call `knowledge_graph` with `operation: "search"` and a broad term to spot-check graph quality. If the graph seems sparse or stale, note it in the report — nightly auto-extraction runs automatically and will populate it from recent conversations.
   - Check whether the core memory has grown too large: call `manage_memory` with `operation: "view"`. Remove entries that are stale, redundant, or captured as more permanent facts in the knowledge graph.
3. **Software Updates.** Call `manage_updates` with operation `check`.
   - If an update is available, summarize the changelog and inform the user in the maintenance report. Do NOT install without user permission.
4. **Personality Profiling Audit.** Call `manage_memory` with `operation: "view_profile"` to retrieve all collected user profile entries.
   - Review every entry for **relevance** and **redundancy**:
     - **Stale/irrelevant:** entries that describe a one-time context, a past task, or information that is no longer true or useful (e.g. "user asked about X yesterday", "temporary preference", outdated project paths, resolved issues).
     - **Redundant:** duplicate entries within the same category that express the same thing with different wording or granularity — keep whichever is most precise and has higher confidence.
     - **Contradictory:** if two entries in the same category directly contradict each other, delete the older/lower-confidence one.
   - For each entry to remove, call `manage_memory` with `operation: "delete_profile_entry"`, `key: <category>`, `value: <key>`.
   - Do NOT delete entries that are still factually accurate and useful for personalizing future interactions.
   - Example categories to review with particular attention: `behavior`, `preferences`, `context`, `goals`, `persona_evolution`.
   - After cleanup, briefly summarize how many entries were reviewed, how many removed, and why.

Execute these tasks autonomously. Report only significant actions taken.
