# Anthropic provider behavior

AuraGo's `anthropic` provider translates the existing OpenAI chat client to the
Messages API. Native Anthropic and compatible gateways share this translator.
API keys remain in the Vault; configure the provider through Setup or Config.
The API version remains `2023-06-01`.

## Thinking

The existing `llm.anthropic_thinking` configuration controls which supported
models may use thinking. It defaults to disabled. `model_allowlist` optionally
restricts enabled models by exact/prefix match, with an optional trailing `*`.
An empty list selects the known supported families automatically.

| Models | Enabled | Disabled or excluded by allowlist |
| --- | --- | --- |
| Sonnet 3.7; Opus 4, 4.1, 4.5; Sonnet 4, 4.5; Haiku 4.5 | Manual budget, with sufficient output room | Omit thinking; these models default to off |
| Opus/Sonnet 4.6; Opus 4.7, 4.8 | Adaptive, without `budget_tokens` | Omit thinking; these models default to off |
| Opus 5; Sonnet 5; Haiku 5.5 | Adaptive | Explicit native `thinking: {type: disabled}` |
| Opus 5.5; Sonnet 5.5; Fable 5/5.1; Mythos 5/5.1/Preview | Adaptive | Local configuration error: this setting cannot provide fully disabled thinking on these models |

Dot and hyphen version aliases and dated IDs use the same known policy. Unknown
versions are not assumed to inherit the restrictions or capabilities of a known
version. An explicit allowlist can opt a compatible unknown model into the legacy
manual-budget path; verify its native requirements before doing so.

Manual thinking uses at least 1024 tokens, at most 32000, and always less than the
request's output cap. Requests capped at 1024 or fewer tokens omit manual thinking;
AuraGo never raises that cap. Adaptive requests retain small supplied caps and
have no application-imposed 1024-token cutoff. Thinking and final text share the
output allowance. `budget_tokens` does not select an adaptive thinking budget.

Forced tool choices disable manual thinking. Adaptive thinking permits forcing a
tool where the model supports it. Opus 5.5, Sonnet 5.5, Fable 5.1 and Mythos 5.1
reject forced choices locally; use `auto` or `none`. A model's fixed-sampling
restrictions still apply when its thinking is disabled. AuraGo omits incompatible
sampling fields and otherwise sends temperature or top-p, never both.

For an always-on model, enable `llm.anthropic_thinking.enabled` and ensure the
allowlist includes it, or select a model that supports thinking off. AuraGo does
not silently change the route or override an off setting. Some upstream models
offer additional modes such as Sonnet 5.5's `between_tools`; AuraGo's off setting
means fully disabled thinking and does not substitute those modes.

Signed and redacted reasoning blocks retain their native continuation data.
The configured model controls the native thinking display default; the off/on
setting does not guarantee that readable thinking text is returned.

## Request compatibility

- `max_completion_tokens` and `max_tokens` are honored. Conflicting positive caps
  and negative values fail locally. Zero/unset uses the existing 8192-token
  default when no positive cap is present.
- `tool_choice: none` sends native `none` while retaining the cacheable tool catalog.
- Parameterless functions get an object input schema. Unsupported tool kinds,
  invalid choices, malformed schemas, roles and content parts fail before sending.
- Developer instructions join system instructions in their original instruction
  order. User images support data URIs and native URL sources. Remote providers
  must be able to reach a URL themselves; private/loopback images should use data.
- This bridge supports text response format. Effective structured-output capability
  is disabled, including OpenCode Go's MiniMax Messages route. Existing JSON helpers
  continue to prompt for JSON, parse and validate it. Explicit `json_object` or
  `json_schema` format requests fail locally instead of being silently weakened.
  Raw catalog/model capabilities remain unchanged.

## Streaming and completion

Native `message_stop` must arrive before a successful finish marker is published.
API errors, malformed events and truncated streams remain failures; they never
become assistant text or fabricated tool arguments. Available usage is retained
and booked once, and accepted streams are not automatically replayed. Closing a
translated stream closes its native response body.

Context exhaustion and token limits remain incomplete output. Refusal, paused
server-tool turns and unknown/missing stop reasons do not authorize normal
completion or tool execution. This bridge does not implement native server tools.

## Upstream contracts

The supported matrix follows the documented contracts checked on 2026-11-02:
[thinking configuration](https://platform.claude.com/docs/en/build-with-claude/thinking),
[tool choice](https://platform.claude.com/docs/en/agents-and-tools/tool-use/define-tools),
[streaming](https://platform.claude.com/docs/en/build-with-claude/streaming),
[stop reasons](https://platform.claude.com/docs/en/build-with-claude/handling-stop-reasons)
and [structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs).
Compatible proxies can implement a narrower contract. No live native/gateway
acceptance is implied by local protocol fixtures.
