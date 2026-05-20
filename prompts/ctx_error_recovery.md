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
4. **No fake workarounds.** A different approach means a genuinely different method to achieve the original goal. DO NOT mock data, bypass verification, or hallucinate success.
5. **Escalate.** If the same tool call or same error fails twice, stop retrying that approach. Inspect the exact error, verify inputs and manuals, then try one materially different method. If that also fails, STOP and explain the bottleneck clearly.
