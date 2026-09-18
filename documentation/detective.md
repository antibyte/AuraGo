# Detective

Detective is a built-in Virtual Desktop app for bounded, agent-driven research.
Each case has its own model conversation, evidence, report revisions and activity
feed. It does not post the research into the ordinary Desktop chat.

## Using the app

Open **Detective** from the Desktop app menu, enter a question, and choose an
effort. Advanced options include scope, report language, starting URLs and an
existing provider/model. The app lists the research tools enabled on this server.

| Effort | Active time | Research tool calls | Agent iterations |
| --- | ---: | ---: | ---: |
| Quick | 5 minutes | 40 | 60 |
| Normal | 15 minutes | 100 | 140 |
| Maximum effort | 60 minutes | 250 | 320 |

These are ceilings, not target durations. Queue time and time waiting for an
answer do not consume active time. At 80% of the time allowance, or at the
research-call limit, the agent must write from the evidence it has. Recording
findings and publishing the report do not consume research tool calls; they
remain subject to the iteration, time and optional token limits. Existing global
provider/spending limits still apply.

- **Finish now** ends new retrieval and asks for a report from existing findings.
- **Stop** cancels the current run and retains its sources, findings and private
  continuation. Available findings produce a clearly marked partial revision.
- **Continue** resumes with the remaining budget, including after a restart.
- **Deepen** explicitly grants a new effort budget to the existing case.
- Closing the window leaves the server job running. Reopening restores the case.

One case runs at a time; additional cases queue. The app shows actual tool,
source and time counters, concise plans, retrieved sources and recorded findings.
Private model continuation never appears in the activity feed or exports.

## Research and evidence

The bundled `aurago-detective` skill is embedded, hash-verified and supplied at
run start, together with the actual tool schemas. It covers search, reading,
primary sources, contradictions, stopping criteria, evidence and report format.
Search snippets are marked as search hits. Findings require an exact supporting
quote in a source the server recorded as retrieved. Reports reference these
findings; model-invented source IDs are rejected.

These checks establish retrieval and reference integrity. They do not prove that
an interpretation is correct, that two publishers are independent, or that every
relevant fact has been found. The report must describe uncertainty and coverage.

Configured search, scraping, crawler, API and browser tools are available when
their normal permissions permit them. Browser sessions are case-owned and
cleaned up after the run. A visible workspace awaiting human interaction is
retained under its existing lease. Public research does not gain shell execution,
configuration writes or access to private integrations automatically.

Additional integrations require an administrator's exact read-operation list and
selection in the individual case. Configure this in YAML, for example:

```yaml
detective:
  enabled: true
  readonly: false
  profiles:
    quick: {seconds: 300, tools: 40, iterations: 60, tokens: 0}
    normal: {seconds: 900, tools: 100, iterations: 140, tokens: 0}
    maximum: {seconds: 3600, tools: 250, iterations: 320, tokens: 0}
  extra_read_operations:
    mcp_call: ["research-archive/read_document"]
```

The example MCP server/tool must actually exist and be enabled. For native
integration tools, values are exact operation names. For MCP, use
`server/tool_name`; for Composio, use exact tool slugs. Operation names alone do
not establish read-only behavior: only approve operations you have checked.
Every wrapped dispatch is checked again, including permission changes mid-run.

The **Detective** configuration section exposes enablement, read-only mode and
effort profiles. Restart after changing profiles; already accepted case budgets
stay unchanged. The Virtual Desktop and its agent control must also be enabled.

## Reports and exports

Select a report revision and download **Markdown**, **PDF**, or **Word**. Exports
use that immutable revision and are cached by format and checksum. Re-exporting
does not call the model. Source links and editable tables are preserved.
**Open in Autor** creates a separate DOCX copy without replacing an edited file.

The built-in PDF renderer embeds Go fonts and requires no Docker service. It
supports their Latin, Greek and Cyrillic coverage. Unsupported glyphs and complex
scripts return an explicit error rather than a damaged PDF. For those reports,
use DOCX/Markdown, or configure the existing Document Creator Gotenberg backend
with suitable fonts. Detective uses that configured renderer without starting a
container. Report HTML has no scripts, remote images or external stylesheets.

Cases live in `detective.db` beside the configured Game Maker SQLite database
(default `data/detective.db`). The store includes private continuation, so protect
it like other application data. Deleting a case removes its evidence,
continuation, revisions, events and cached artifacts. Fixed initial bounds are
100 cases, 400 source records, 200 findings and 30 revisions per case, with bounded
source and continuation sizes. Automatic retention and configurable research
concurrency are not part of this initial release.

## Verification

Automated coverage includes cumulative budgets for all three efforts, queue and
clarification timing, cancellation, persistent restart, idempotent actions,
wrapped-tool policy, source capture, stable prompts, report references, immutable
exports, long tables and Desktop lifecycle. Chrome exercises the production app
modules in Standard/Fruity, light/dark and narrow/wide layouts. All 16 Desktop
languages include the new labels.

Live comparative provider runs, DOCX roundtrips in Word/LibreOffice/Autor and
multilingual Gotenberg rendering remain release acceptance work. Automated
reference checks are not a substitute for those quality checks.
