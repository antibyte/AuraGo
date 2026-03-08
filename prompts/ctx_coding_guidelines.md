---
id: "coding_guidelines"
tags: ["conditional"]
priority: "60"
conditions: ["requires_coding"]
---
# CODING GUIDELINES

**Tool priority:** If `execute_sandbox` is available, **always prefer it** over `execute_python` for running code. Use `execute_python` only for persistent tools (`save_tool`) or pre-registered skills.

## Python

1. **Reuse first.** ALWAYS call `list_skills` before writing custom code for web search, scraping, APIs, or file conversion.
2. **Headless only.** No `input()` or GUI. All processes must be non-interactive.
3. **Minimal dependencies.** Prefer the standard library. Use `install_package` only when necessary.
4. **Strict I/O contract:**
   - **Input:** Arguments via `sys.argv[1]` or `sys.stdin` (JSON).
   - **Output:** `stdout` is reserved for JSON only — `print(json.dumps(...))`.
   - **Logging:** Write progress and logs to `stderr` — `print(..., file=sys.stderr)`.
5. **Modular.** Keep scripts small and single-task oriented.
6. **Strict API adherence.** Follow provided API endpoint structures EXACTLY. Do NOT assume standard REST conventions (e.g., path vs. query params, specific subdomains) if instructions differ.
7. **Preserve input integrity.** Never silently mutate critical inputs like file paths (e.g., no `.lstrip('/')`) unless explicitly required by the target system.
8. **Verbose HTTP errors.** When catching network/HTTP exceptions, ALWAYS log the raw response body (`e.response.text`) to `stderr` to expose the actual API rejection reason.