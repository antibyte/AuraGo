---
id: "error_recovery"
tags: ["conditional"]
priority: 50
conditions: ["is_error"]
---
# ERROR RECOVERY CODEX

1. **Analyze.** Inspect `stderr` output and exit codes to identify the exact line and exception type.
2. **Fix the root cause.** Target imports, paths, or type mismatches directly. NO blind guessing.
3. **No workarounds.** DO NOT mock data or hallucinate success. The original goal must be achieved.
4. **Escalate.** After 3 failed attempts on the same issue, STOP. Explain the bottleneck clearly and ask the user for help.