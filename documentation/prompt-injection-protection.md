# Web and email prompt-injection protection

Guardian treats retrieved content as data. Deterministic checks, escaped boundaries,
tool permissions and private-data URL checks remain active without a local model.

## Optional Horizon model

Open **Config → PromptSec Guardian → Local protection for web and email** (German:
**Lokaler Schutz für Web und E-Mail**). Click **Download**, wait for verification and
the local self-test, then enable the checkbox and save. Downloading alone does not
activate inspection. Status requests and application startup never initiate downloads.
Closing the browser leaves the download running; **Cancel** stops it. Retry reuses
verified files. Disabling stops the classifier worker and retains the downloaded files.

```yaml
guardian:
  injection_guard:
    enabled: false
    action: quarantine # quarantine | datamark
```

The model is off by default and upgrades preserve the operator's explicit choice.
The operator decides whether CPU capacity and available memory are sufficient;
AuraGo never enables the model automatically based on a hardware probe.
CPU runtimes support Windows and Linux
amd64/arm64, macOS arm64. Unsupported platforms retain ordinary Guardian operation.
No Python, CGO or GPU is required on the end-user machine.

The pinned [Horizon Labs model](https://huggingface.co/Horizon-Labs/prompt-injection-guard-small/tree/0e3774536debbcd71dbb8534ef71451b583853b8)
and tokenizer total **302,796,789 bytes** (about 303 MB), plus the platform's CPU ONNX
Runtime if not cached. Runtime assets are shared with existing ONNX features; model
files live in `data/injectionguard/models`. The official quantized export quantizes
embedding/Gather weights; it is not a fully int8 transformer. Allow roughly 1–2 GiB
of available RAM and qualify the machine with real workloads. Exact artifact sizes
and SHA-256 pins are in `internal/injectionguard/model.go`.

Supported installations with the model disabled receive one informational Chat notice
with an **Open settings** link to `/config#guardian`. It only opens settings; download
and activation remain separate explicit actions. The existing typed notification store
deduplicates `security:horizon_optional` across restarts and updates. Chat acknowledges
only successfully displayed IDs. Failed delivery or acknowledgment leaves the notice
available for another attempt; a read notice stays read across browsers and chats.
An outstanding notice is hidden while the classifier is enabled or unavailable.
The notice is display-only and never enters conversation history or model context.

## Local scans and archived output

`guardian.max_scan_bytes` bounds each local scan window, not total input coverage.
Larger inputs use overlapping windows; `scan_edge_bytes` is their overlap and zero
selects the default. Raw text, normalized scan views and decoded shadows are checked
separately. Diagnostic normalization does not rewrite delivered source content.
`use_sanitized_output` optionally replaces only the latest user message, and only
when a complete sanitized version is available. Tool outputs retain their own checks.

All normal dispatches select one effective Guardian, including the shared local
fallback, and pass it through a request-local context to handlers. Both archive-read
APIs then pass through the outer dispatch's scanning, isolation and warnings. An
inner handler response is not the final model-facing result. Archived source identity
comes from the stored tool name and operation; `web_search` remains a recognized
legacy web identity. Unknown tools retain external isolation without gaining web/mail
provenance. Project-source isolation keeps ordinary code copyable; escaping hostile
markup alone is not a guarantee against instruction following.

Taint attaches provenance metadata; it is not a semantic detector or a prerequisite
for the optional LLM judge. Judge mode `uncertain` requires a non-blocking local
signal and does not inspect text with no local signal. Model-backed checks stay
opt-in and add latency and, for remote providers, API cost. Retired Structure,
Spotlight and Canary options remain inert for legacy configuration compatibility.

## Decisions and failure behavior

| Condition | Result |
| --- | --- |
| Hard local rule | Fixed quarantine notice, regardless of model action |
| Model disabled | Ordinary Guardian rules and isolation |
| Model score below 0.5 | Content continues through ordinary isolation |
| Model hit, quarantine | Original content withheld |
| Model hit, datamark | Natural text marked with `^`; JSON structure, code and machine fields preserved |
| Enabled model unavailable, cancelled, timed out or incomplete | Original withheld with a fixed reason |

Datamarking still sends suspicious text to the language model. It is weaker than
quarantine and cannot guarantee safe instruction following. Suspicious watched emails
do not start automatic missions. Completed tool mutations retain their execution outcome
even if returned content is withheld; do not repeat them.

Classification happens before isolation on web/search/browser and email adapters and
before summary helpers. Archived web/mail results are checked again when read.
Source identity comes from server/tool metadata, never tags printed in content.
Archives retain the tool operation as well as its name. Older Google Workspace
archives without an operation are conservatively checked as mail. The additive
archive migration backs up the existing database before changing its schema.
Datamarking preserves paths, URLs, identifiers and operation/status fields, including
within archived JSON and repeated isolation; these fields still undergo all scans.
The classifier sees unnormalized text and decoded JSON strings. Every forwarded window
is evaluated: 2048 tokens including special tokens, 512-token overlap, maximum score.
Inputs over 256 KiB are withheld; callers may select a bounded page first. One job runs
with eight queued jobs. The 20-second budget includes queue and worker startup; an
earlier caller deadline wins. Configuration changes revoke pending decisions. Fixed
self-tests contact no external language model.

Shared HTML cleanup removes zero opacity/font size, transparent text, clipping and
distant positioning. Short screen-reader labels, accessible collapsed panels and
`noscript` survive. This is bounded HTML cleanup, not a full CSS/layout engine.
ZWJ/ZWNJ remain in delivered text; normalized scan views are separate. Clear DE/EN/FR
directives and anchored ROT13 attacks quarantine whole content rather than deleting
paragraphs. Versioned control-token handling preserves ordinary copyable project code.

## URL disclosure boundary

After web/email contact, outgoing URLs are compared with authentic human input and
private file/mail contents in the session. Four consecutive words in query/fragment,
six in a path, or a private email address cause `possible_exfiltration`. An exact URL
provided by the authenticated human may pass the copy check; links printed by a web
page cannot grant that exception. Registered secrets remain blocked. Generic allow
rules do not bypass the check. Ordinary link following is retained.
Private email addresses remain protected when they occur inside source URLs,
including encoded query values. Checks inspect destination arguments and literal
URLs in executable input. A URL merely saved in a local file is not a network request.

The session index is bounded to 256 KiB per session, 16 MiB total and 256 sessions;
external-contact records expire after one hour. Evicted data cannot be compared.
This is not a network sandbox and cannot detect arbitrary URLs computed inside
shell/Python code.

## Administration and verification

Only administrators can access `/api/guardian/injection-guard/status` (GET) and
`download`, `cancel`, `test` (POST). Enablement/action use the ordinary Config API.
The agent has no model lifecycle tool. API errors contain fixed categories, not
private text or native process output. Downloads end with the server lifecycle.

Run package and opt-in CPU/browser tests described in `internal/injectionguard/AGENTS.md`.
The corpus includes the 24 documented attacks, three disguised requirements and benign
DE/EN/FR examples. Token IDs and CPU scores have independent reference fixtures.
Sample detection rates are not guarantees; false positives and false negatives remain.
