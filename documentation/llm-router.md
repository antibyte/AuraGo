# Task LLM router

The optional router chooses a configured provider and model for each new chat
task. It is off by default. Enable it under **Configuration → LLM router**.
Create providers in **Providers** first; credentials stay in the existing Vault.

Assign any of these nine areas:

| Area | Typical request |
| --- | --- |
| General | Conversation, greetings, questions about the assistant |
| Easy tasks | Simple arithmetic, conversions and short factual requests |
| Normal tasks | Organizing appointments, files or a list of options |
| Complex tasks | Planning architecture, dependencies, migrations or risks |
| Coding | Implementing, debugging, refactoring or testing software |
| Research | Finding sources, checking evidence, researching alternatives |
| Creativity | Inventing ideas, stories, names or poems |
| Security | Assessing vulnerabilities, permissions or hardening |
| Writing | Drafting, translating, editing or summarizing text |

Specialized areas take precedence over difficulty. A complex programming task
uses **Coding**. If Coding is empty, it uses the ordinary active model, even if
Complex tasks or General has an assignment. General is its own area, not a
catch-all model override. A blank model uses the selected provider's default.
Clearing a provider also clears its model override.
Agnes AI chat models are available, including an already configured model that is
newer than the bundled catalog. Agnes image and video generation models cannot
be used as chat targets, including through a model override.

The decision belongs to one agent run, including its tool rounds. A later human
message can select a different model. A brief continuation such as “weiter” can
reuse the previous decision in that session for up to ten minutes. A different
intent or configuration invalidates that reuse. Routing never changes the global
model setting or learns from tool output. Local rules support German and English;
weak evidence and unsupported phrasing remain uncertainty.

Ordinary Web Chat, Desktop/AgoDesk, Telegram, Discord, Rocket.Chat, SMS and
authorized MeshCore chat use the shared router. Explicit Desktop and Speech Lab
provider choices remain binding. Prepared workflows, Game Maker, Detective, SIP,
missions, maintenance, co-agents and minimal loops keep their existing models.
The reserved managed-local provider remains available through its established
primary/fallback roles; it is not a category target. The router never downloads
models, starts a model family or adds providers on its own.

## Helper use

Local rules and cached decisions run first. The helper is only considered for
uncertain input when at least one usable assignment differs from the ordinary
route. No assignments means no classification work. The helper must already be
explicitly enabled and configured in LLM settings; the main model is never used
as a substitute classifier.

Defaults are a 1,500 ms deadline, one physical completion attempt, at most 768
counted input tokens and 256 generated tokens including reasoning. A reasoning
model requiring a larger output reserve is skipped. The helper shares the
existing concurrency and budget limits. Busy, timed-out, invalid or unavailable
helpers immediately leave the ordinary model in use.
The configured AI Gateway also receives a one-attempt limit for classification;
ordinary model requests keep their own gateway retry setting.

The installation quota refills at 20 calls per hour with a maximum burst of two;
each session also has a 60-second cooldown. Set the hourly limit to zero or turn
off helper fallback to use local decisions only. The deadline can be set from
250 to 5,000 ms and the refill rate from 0 to 120 calls per hour. Classification
caches hold at most 256 entries for ten minutes and store no raw prompts.

## Preview and feedback

Save or discard configuration changes before testing the preview. **Test locally**
classifies an example using saved assignments without a helper call, provider
metadata probe or task execution. **Ask helper once** becomes available for an
uncertain result when a configured helper could change the outcome. This explicit
action may incur provider cost and consumes the same quota as ordinary routing.
Preview never inserts a chat message or changes a session's previous decision.

Web Chat and Desktop show the area, decision source and actual provider/model in
a transient turn badge. The Dashboard's system tab shows in-memory counters since
restart, including helper calls, reported tokens, failures and fallbacks. Unknown
usage is shown as `?` or a lower bound rather than zero cost. No dollar savings are
inferred. Routing metadata is not included in assistant text or persistent memory.

## Failures and compatibility

Each selected route uses a matching cloned config and client. The main provider's
shared failover manager is not reconfigured. Required images, native tools and
context/output limits are checked before selection. Model overrides do not inherit
the original model's capability or context overrides.

An unusable selection returns to the ordinary route. For a selected task, the
captured fallback chain is selected provider → ordinary provider → existing
eligible fallback, without duplicates. Provider rate limits and transient server
or network errors may advance that chain. Cancellation and authentication or
authorization errors are terminal. A partially received stream is not replayed
through another provider. Permissions, live authorization checks, tool scope and
normal request budgeting still apply.

Assignments and toggles hot-reload for new tasks; existing runs retain their
accepted snapshot. Old configurations remain valid and disabled. Provider deletion
is blocked while a saved router assignment references it, including when the
router is disabled. Remove the assignment first. No database migration is needed.
A stored zero deadline from a programmatic configuration snapshot is normalized
to 1,500 ms without changing explicit disabled switches or a zero helper quota.

## Development checks

Use the repository's Windows cache wrapper when needed. Focused checks:

```text
go test ./internal/config ./internal/llm ./internal/agent ./internal/server ./cmd/config-merger -run 'TestTaskRouter|TestLLMRouter'
go test ./internal/llm -run TestTaskRouterEvaluation -v
npm run build:ui -- --check
```

`AURAGO_RUN_BROWSER_SMOKE=1` enables `go test ./ui -run TestLLMRouterConfigBrowser`.
The browser fixture exercises real settings controls, save/clear behavior, preview
actions and text-safe badges at narrow/wide sizes in both themes. The versioned
240-case synthetic evaluation set is separate from development fixtures. It is a
regression gate, not evidence of model quality or measured production savings.
Real-provider billing, network latency and deployed behavior require an enabled
installation and its own observations.
