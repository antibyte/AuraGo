# Cloudflare AI Gateway

AuraGo offers two explicit choices: route existing providers through native gateway endpoints, or select a separate Cloudflare REST provider. Neither choice automatically creates Cloudflare resources or changes other providers' billing.

## Existing providers

Enable **Config → Integrations → AI Gateway**, enter its account and gateway IDs, and save. Native routes support OpenAI, Anthropic, OpenRouter, DeepSeek, Groq, Mistral, xAI, Cerebras and Perplexity. Local providers are excluded; explicit custom endpoints remain direct. Workers AI requires its own provider account ID and API token; the gateway account is never substituted.

Native requests keep their provider API key and use `cf-aig-authorization` for the optional authenticated-gateway token. That token is account-scoped, not isolated to one gateway. Store credentials through the provider/configuration UI, which keeps them in the Vault.

## Cloudflare REST, including Google models

Add a provider of type **Cloudflare REST** in the normal provider manager. Supply its own account ID, gateway ID, Cloudflare API token and canonical model ID, for example `google/gemini-2.5-flash` or `openai/gpt-4.1`. Workers AI models retain `@cf/author/model`. Choose that provider for the main LLM, helper, fallback or task router as usual.

The API URL is generated as `https://api.cloudflare.com/client/v4/accounts/{account}/ai/v1`. Requests use OpenAI Chat Completions, `Authorization: Bearer {provider-token}` and `cf-aig-gateway-id`. The native gateway token is not reused. The token needs **Account → Workers AI → Read**, including for third-party models; AI Gateway permissions alone are insufficient. Third-party models use Cloudflare Unified Billing and require Cloudflare credits.

The bundled Cloudflare catalog is a curated starter list from the official catalog; additional canonical IDs can be entered normally. Model IDs are sent unchanged. Provider capabilities and request format remain distinct from native Anthropic translation. Cloudflare costs are not presented as measured upstream-provider prices; configure model cost overrides where needed and distinguish estimated accounting from provider-reported measurements.

The rewrite toggle governs existing providers. An explicitly selected Cloudflare REST provider continues to use Cloudflare when rewriting is off. Shared logging and response-start controls still apply.

## Hugging Face: modern chat router

1. In Cloudflare's **AI Gateway → Custom Providers**, create and enable a provider with the upstream root `https://router.huggingface.co` and a slug such as `hf-chat`.
2. Enter `hf-chat` in AuraGo's **Hugging Face custom provider slug**, enable native gateway rewriting and save.
3. Keep the Hugging Face provider URL at `https://router.huggingface.co/v1` and store its inference token through the provider UI.

AuraGo routes to `https://gateway.ai.cloudflare.com/v1/{account}/{gateway}/custom-hf-chat/v1/chat/completions`. Cloudflare appends `/v1/chat/completions` to the configured upstream root. Do not configure `/v1` twice. Model IDs, including inference-provider suffixes, remain unchanged. Streaming and tool support depend on the selected model. The legacy `/huggingface/{model}` inference API is not a substitute for this chat protocol.

AuraGo does not provision the custom provider or request Cloudflare administrative permissions. Its upstream mapping is managed by the Cloudflare operator; local configuration cannot attest the remote mapping. A live model test verifies the configured route.

## Privacy, retries and timeouts

| Logging mode | Collect log | Collect payload |
| --- | --- | --- |
| `metadata_only` (default) | Respect Cloudflare's gateway setting | `false` |
| `off` | `false` | `false` |
| `full` | `true` | `true` |

Full logging explicitly stores prompts and responses at Cloudflare. Metadata is limited to five complete, unique pairs and a 4 KiB serialized header. Never put credentials or personal content in metadata. Invalid saves are rejected before Vault writes; entries are not silently truncated.

AuraGo owns retries and always sends `cf-aig-max-attempts: 1`. Legacy `mode`, `max_attempts`, `retry_delay_ms` and `backoff` values remain readable. Modes retain native routing; gateway retry values are ignored with diagnostics. They never enable REST or change billing.

`request_timeout_ms` accepts 0–300000. Zero keeps existing client limits. A positive value limits response start; the shorter response-header, caller or per-attempt limit wins. Status exposes the requested value and `request_timeout_effective_ms`; individual requests can have a shorter remaining caller deadline. Active streams retain AuraGo's progress/idle/cancellation rules, rather than being cut off by a whole-stream gateway timer.

## Diagnosis

Select a saved provider. Unsaved settings must be saved before testing.

- **Status** is local and performs no DNS, metadata or generation requests.
- **Test Gateway** makes a non-generating metadata request. A successful Workers AI/REST account check does not validate the selected model or available credits. Unsupported metadata probes are explicitly skipped.
- **Test model (billable)** makes one generation request, at most 32 output tokens, with a 20-second deadline and normal budget accounting. Incomplete output is not success. Some reasoning models cannot finish within this diagnostic allowance; this is not proof of a broken route.

Failures expose stable codes and, when available, the upstream HTTP status; provider response bodies are never returned. Request cancellation and server shutdown cancel live tests. Health recovery uses the actual runtime route and still requires a bounded generation confirmation.

Upstream references: [REST API](https://developers.cloudflare.com/ai-gateway/usage/rest-api/), [custom providers](https://developers.cloudflare.com/ai-gateway/configuration/custom-providers/), [logging](https://developers.cloudflare.com/ai-gateway/observability/logging/), [request handling](https://developers.cloudflare.com/ai-gateway/configuration/request-handling/), [model catalog](https://developers.cloudflare.com/ai/models/).
