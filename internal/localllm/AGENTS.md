# Managed local models

## Purpose

Local model lifecycle, routing, attestation, and qualification.

## Ownership

`internal/localllm` owns this domain. The contracts below also bind related server, UI, config, asset, and test work through the root routing table.

## Local Contracts

### Managed Local Model Contract
- `spark` selects experimental AuraGo-Spark Q4_K_M with a fixed 64K context, Thinking on, one slot, and no speculative decoding or KVFlash. Config and Setup state at least 6 GB VRAM. Pin its Spark-capable hybrid engine independently; do not reuse Ling images or templates. Public CUDA/SYCL/Vulkan runtime digests are pinned and require explicit experimental selection. WSL CPU startup, tool rounds, streaming and cache reuse passed; filled-64K answer quality and native GPU acceptance remain unverified.
- `local_llm.model_family` defaults to `qwen`; `ling` selects AuraGo-Ling Q4_K_L with MTP off and the full 16K context. Ling 32K requires separate qualification. Keep the reserved provider ID `aurago-qwen-local`; resolve display name and API model alias from the selected family.
- Reuse the single local-model manager and its Vault credential, isolated container, resumable hash-verified downloads and routing gates. Family/engine changes invalidate attestation and prompt caches but preserve downloaded models.
- Qwen runtime pins remain independent. Ling pins hybrid engine `f37a34cd4e502284ca297e141a6c4013bd151b18`; CUDA uses 2048/64 phase batches and Q8 KV, while SYCL/Vulkan use conservative F16 profiles. Apply SM75 tuning only to compute capability 7.5. Disable KVFlash, Thinking, draft decoding and context fitting for Ling.
- Qwen's single-slot runtime uses and attests `--slot-prompt-similarity 0.999999`: only a complete input-prefix match at supported 16K/32K limits reuses the current slot directly. Partial matches compare saved RAM contexts; identical prompts must not trigger unnecessary full-state copies.
- Model publication requires an anonymously verified HF revision/hash/size and digest-pinned images. A backend without native Linux GPU acceptance stays experimental and unavailable to automatic selection; Windows/WSL results never grant that qualification.
- Local prompt reuse starts with the first attested request, including helper requests. Cache qualification runs only in idle time; ordinary misses and latency variance must not permanently disable reuse. Read actual reused tokens from `timings.cache_n` or OpenAI usage, never the native post-generation `tokens_cached` slot size. Ling keeps stable instructions in its first system message and moves `# TURN CONTEXT` into a second system message so its template renders stable tools before volatile context. Preserve all trusted instructions and ordered tool results.

## Verification

- Run `go test ./internal/localllm` and the named cross-component checks in the contracts above when those paths change.

## Child DOX Index

None.
