# Dry run of the plan, 2026-11-06

The plan's Create, Edit and Append blocks were applied by script (`dry.js` next to this file), in the order of the plan index, to a scratch copy of main at e214cf38e. `dry.js <partsDir> <tree> <stopTask>` applies up to a task; `dry.js <partsDir> - - --check` matches every Old text against the repo's HEAD in memory.

Result on the final text: 340 changes to 160 files, every Old text matches exactly once.

| Stop | check:ui | storymaker + config | server `-run Storymaker` | ui guards | agent `-run storymaker...` |
| --- | --- | --- | --- | --- | --- |
| A10 | ok | ok | ok | ok | ok |
| B5 | ok | ok | ok | ok | ok |
| C10 | ok | ok | ok | ok | ok |
| D7 | ok | ok | ok | ok | ok |
| F4 (whole plan) | ok | ok | ok | ok | ok |

`go build ./...` and `go vet` were clean at each stop. E4 was not tested as a stop of its own: F4 adds only the config page, docs and landing steps to it.

On the whole plan also:

- `GOOS=linux go build ./...` clean.
- `./internal/agent` complete: only `TestFollowUpAndWaitForEventNeedARuleWhenUnattended` and `TestUnattendedFetchCopyingIsolatedTextNeedsARule` fail. Both fail the same way on main without the plan (MQTT relay policy).
- `./internal/prompts` and `./internal/security` ok. `./internal/audit`: the three tests known to be red on main.
- Browser, sandbox off: `TestStorymakerAppRealServerBrowser`, `TestCompileCheckBrowser` and `TestStorymakerNarratedExportEndToEnd` pass.

What the dry run found: two compile errors and four wrong tests. The reviews of the applied code found nine more points (design section 11), which no test could show.

**Limits.**

- It proves batch ends, not each single task.
- It applied text to files with LF. A real worktree has CRLF in some files, which the executor must keep.
- The doc edits of F2 were matched against HEAD but not read as prose.
- The restore branch of `silent:true` after a refused enqueue is reached only by a direct helper call in its test, because the fixture cannot make the enqueue refuse after the silent save.
- No real LLM ran. Whether a real weak model gets through is what the acceptance in task F4 measures.
