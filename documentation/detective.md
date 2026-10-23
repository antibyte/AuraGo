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
  It stays unavailable while Detective or Virtual Desktop is read-only.
- **Stop** cancels the current run and retains its sources, findings and private
  continuation, and disables automatic updates. During initial research,
  available findings produce a clearly marked partial revision. An automatic
  update keeps the last published report and archives its intermediate findings.
  Stop stays available when Detective or Virtual Desktop is read-only, as long
  as Detective is enabled — it is a state write and does not need a ready
  research runner. Start, Continue, Deepen, Delete and New do not. When actions
  are disabled, the app names the reason (disabled, read-only, starting up)
  from the capabilities answer.
- **Continue** resumes with the remaining recorded budget, including after a
  restart. It does not apply a new effort.
- **Deepen** explicitly grants a new effort budget to an existing case that has
  already left draft. A draft offers Start and the effort control, not Deepen.
  The effort control next to Deepen applies only to Deepen and to the first
  Start of a draft.
- Closing the window leaves the server job running. Reopening restores the case.
  An open window polls live status and new activity, and reloads source text or
  the report only when that list changes.

One case runs at a time; additional cases queue. The app shows actual tool,
source and time counters, concise plans, retrieved sources and recorded findings.
Private model continuation never appears in the activity feed or exports.

## Automatic Update

Enable **Automatic Update** when creating a case, or open that section in an
existing case and save its settings. Existing cases start with this mode off.
Choose a whole number of hours or days, from one hour to 365 days (default:
24 hours). Update effort is independent of the initial research and defaults to
Quick. The configured effort limits and global spending limits apply to every
run. Recurring research incurs model costs.

- **Dated additions** (default) preserves the published report and adds new,
  evidenced sections with a server-generated date.
- **Rewrite the full report** creates a new complete edition. Older editions
  stay available unchanged under **History**.

Scheduling begins after the first completed report. The next interval starts
when a check finishes, including a failed check; failures do not retry early.
After downtime, at most one overdue check is caught up. Interrupted automatic
runs resume their saved conversation and remaining budget after restart. Each
new check starts with a fresh conversation. Closing the Desktop window has no
effect on scheduling.

The app shows activation, next check, last successful check, last report change,
and errors or questions. A question pauses checks until answered. **Pause** or
**Stop** cancels an active automatic update and disables future checks, retaining
recorded evidence. Permission restrictions prevent scheduling and revoke active
work. A queued case waits until permissions return instead of starting and failing;
a run stopped this way is reported as "research permission revoked" and can be
continued. Pause remains available in readonly mode.

No new findings means no report version is created. Extra URLs for the same
finding do not count as news. Failed or stopped updates keep the last published
report; a later run can revisit and publish pending findings after checking their
sources again. Corrections need newly retrieved evidence as well.

## Research and evidence

The bundled `aurago-detective` skill is embedded, hash-verified and supplied at
run start, together with the actual tool schemas. It covers search, reading,
primary sources, contradictions, stopping criteria, evidence and report format.
Search snippets are marked as search hits. Findings require a quote of at least
eight characters that is an exact contiguous substring of a source the server
recorded as retrieved. Reports reference these findings; model-invented source
IDs are rejected. The continuation prompt receives the bounded working findings, with
each finding text limited to 500 characters and each quote to 240. The case
keeps the full quotes in the archive. Automatic checks receive the question,
latest report preview, last successful check and recent findings; bounded,
case-scoped archive reads provide older sources and report segments.

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
**History** and **Sources** load older records in pages. For an appended revision,
load previous sections to read its base; all export formats include the complete
selected edition and its additions. Export assembly is limited to 4096 segments
and 16 MiB of report data, output to 32 MiB, and requests to 60 seconds. Oversized
editions return an explicit error; choose an earlier edition or a new full rewrite.

The built-in PDF renderer embeds Go fonts and requires no Docker service. It
supports their Latin, Greek and Cyrillic coverage. Unsupported glyphs and complex
scripts return an explicit error rather than a damaged PDF. For those reports,
use DOCX/Markdown, or configure the existing Document Creator Gotenberg backend
with suitable fonts. Detective uses that configured renderer without starting a
container. Report HTML has no scripts, remote images or external stylesheets.

Cases live in `detective.db` beside the configured Game Maker SQLite database
(default `data/detective.db`). The store includes private continuation, so protect
it like other application data. Deleting a case removes its evidence,
continuation, revisions, events and cached artifacts. Up to 100 cases are allowed.
The working set remains bounded to 400 sources and 200 findings; those limits and
the former 30-revision cap no longer bound a case's lifetime archive. Source,
finding and report tables grow with use; nothing is automatically removed.

Opening an older database first creates a consistent, synced
`detective.db.pre-v3-<timestamp>.bak` snapshot. The version-3 migration then moves
existing evidence and reports into archive tables transactionally, retaining IDs,
revision numbers and cached exports. A backup or migration error aborts opening.
Protect backups like the database; they contain the same private data.

The case API includes `auto_update`, archive counts and the latest revision.
`PUT /api/desktop/detective/cases/{id}/auto-update` accepts `enabled`,
`interval_hours`, `report_style` (`append`/`rewrite`) and `effort`
(`quick`/`normal`/`maximum`). GET `reports`, `sources` and `findings` below that case
accept `before`, `limit` (20 by default, at most 50) and optional `query`, returning
`next` as the next cursor. GET `report?revision=N` returns one immutable segment;
`source?id=...` retrieves one source. Existing revision-bound export URLs keep
their meaning. All routes retain the Desktop administrator boundary.

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
