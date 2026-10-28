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

Existing installations keep the model off. CPU runtimes support Windows and Linux
amd64/arm64, macOS arm64. Unsupported platforms retain ordinary Guardian operation.
No Python, CGO or GPU is required on the end-user machine.

The pinned [Horizon Labs model](https://huggingface.co/Horizon-Labs/prompt-injection-guard-small/tree/0e3774536debbcd71dbb8534ef71451b583853b8)
and tokenizer total **302,796,789 bytes** (about 303 MB), plus the platform's CPU ONNX
Runtime if not cached. Runtime assets are shared with existing ONNX features; model
files live in `data/injectionguard/models`. The official quantized export quantizes
embedding/Gather weights; it is not a fully int8 transformer. Allow roughly 1–2 GiB
of available RAM and qualify the machine with real workloads. Exact artifact sizes
and SHA-256 pins are in `internal/injectionguard/model.go`.

## Decisions and failure behavior

| Condition | Result |
| --- | --- |
| Hard local rule | Fixed quarantine notice, regardless of model action |
| Model disabled | Ordinary Guardian rules and isolation |
| Model score below 0.5 | Content continues through ordinary isolation |
| Model hit, quarantine | Original content withheld |
| Model hit, datamark | Natural text marked with `^`; JSON structure and code preserved |
| Enabled model unavailable, cancelled, timed out or incomplete | Original withheld with a fixed reason |

Datamarking still sends suspicious text to the language model. It is weaker than
quarantine and cannot guarantee safe instruction following. Suspicious watched emails
do not start automatic missions. Completed tool mutations retain their execution outcome
even if returned content is withheld; do not repeat them.

Classification happens before isolation on web/search/browser and email adapters and
before summary helpers. Archived web/mail results are checked again when read.
Source identity comes from server/tool metadata, never tags printed in content.
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
