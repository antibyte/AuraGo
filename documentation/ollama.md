# Ollama integration

The optional `ollama` tool manages models independently of the selected chat LLM.
Enable it in Settings > Ollama. External mode uses the saved HTTP(S) API URL.
Managed mode derives that URL automatically and makes it read-only in Settings.
Save changes before using Test connection or Recreate.

## Managed containers

Managed inference uses `aurago_ollama_managed`; local embeddings use
`aurago_ollama_embeddings`. On native installations with a local Docker Engine,
their host ports default to 11434 and 11435 and are published only on loopback.
An empty managed model path uses `aurago_ollama_managed_models`; embeddings use
`aurago_ollama_embeddings_models`. Recreate retains those volumes. Changing the
model path selects different storage and does not migrate the old files.

In the supplied Docker Compose deployment, AuraGo proves its container identity
on the configured Engine and attaches Ollama to an existing application bridge
with outbound access. The API uses container DNS on internal port 11434; no host
port is published. The Docker control network is excluded. Both containers can
therefore use internal port 11434 without a collision. Configured host ports
apply to native deployments only. A reachable Docker socket proxy does not by
itself establish network reachability of the daemon's containers.

Managed operation refuses unsupported remote Engines, host/shared network
namespaces, unverifiable custom container hostnames, and setups without a shared
bridge that permits model downloads. Configure an external Ollama endpoint in
these cases. Do not expose an unauthenticated Ollama port on all host interfaces
just to work around loopback reachability.

Managed operation requires Docker enabled and `docker.readonly: false`.
`ollama.readonly` separately restricts agent model-management writes, including
load/unload; inference can still affect model residency. Disable a managed
instance and wait for its stop to complete before revoking Docker writes.
Changing Docker Engines cancels obsolete work but retains containers on the old
Engine for explicit administrator review.

Memory limits accept positive bytes or binary `k`, `m`, `g` suffixes; an empty
managed limit remains unlimited. Invalid values, overflow, protected bind paths,
and native port conflicts are rejected before replacing a container. Managed
containers use no-new-privileges and a PID limit. A container at a reserved name
with unverifiable ownership is left untouched. Known legacy Ollama images with
the reserved model volume and loopback bindings can be migrated to owner labels.
Resolve other legacy or foreign occupants explicitly in the administrator
container page. Generic agent Docker and Compose access to these containers and
model volumes is blocked by name, ID, labels and resource references.

## Readiness and downloads

Container-running and API-ready are separate states. Readiness requires an
Ollama `/api/version` response. Test connection reports the saved endpoint's
version and installed-model count; it performs no inference or download.

Recreate returns a receipt and Settings follows it through image download,
creation, API startup and model downloads. Existing containers are preserved
when preflight or replacement-image acquisition fails. Missing GPU devices in
AuraGo's filesystem are not evidence that the daemon lost its hardware. A GPU
spec is never silently replaced with CPU after detection changes. If device
availability cannot be verified before replacement, the existing container is
retained with an actionable error. Real GPU offload still requires hardware
acceptance; a created container alone is not proof of acceleration.

The agent's `pull` operation accepts a background download immediately; use
`running` to inspect its receipt. Only `ready` confirms successful completion of
the full response. `unknown` means the response was lost or interrupted: inspect
the installed models before explicitly trying again. Such writes are never
automatically replayed. Automatic and agent pulls share concurrent work for the
same endpoint and model. At most four top-level operations are admitted; up to
128 process-local receipts are retained. After restart an absent receipt is
unknown, not success. Disabling or changing configuration cancels obsolete work;
server shutdown cancels and drains its requests.

The embeddings settings display managed Ollama download and failure states.
Embedding requests may fail until their model is available; the process need
not wait for a thirty-minute download to become usable. The early managed
embedding startup remains before VectorDB initialization and already knows the
native/container environment.
