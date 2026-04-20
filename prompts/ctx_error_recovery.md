---
id: "error_recovery"
tags: ["conditional"]
priority: 50
conditions: ["is_error"]
---
# ERROR RECOVERY CODEX

1. **Check Memory.** Before analyzing the error, search your memory for past solutions. Run `query_memory` across `error_patterns`, `journal`, and `cheatsheets` with a query describing the error or failing component. Treat every memory hit as a clue to verify, not a fact: if something failed before, re-test it under current conditions before ruling it out. If you have resolved this before, reuse or adapt the documented solution — do not start from scratch.
2. **Analyze.** Inspect `stderr` output and exit codes to identify the exact line and exception type.
3. **Fix the root cause.** Target imports, paths, or type mismatches directly. NO blind guessing.
4. **No workarounds.** DO NOT mock data or hallucinate success. The original goal must be achieved.
5. **Escalate.** After 3 failed attempts on the same issue, STOP. Explain the bottleneck clearly and ask the user for help.
