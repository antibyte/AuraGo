# treg integration

AuraGo exposes the dynamic treg catalog through three native tools: `treg_catalog`,
`treg_call` and `treg_status`. The integration is optional, off by default and
read-only by default. No additional runtime, library or container is required.

## Setup
1. Create an organization-scoped token in treg. Identity tokens are unsupported.
2. Open **Settings → treg**, enter the token, and save it in the encrypted Vault.
3. Enable network requests and treg, choose the USD limit, then use the shared Save.
4. Test the connection. This reads the saved account and balance without a catalog call.
5. Search the catalog, inspect inputs, price basis and provider requirements. Choose
   an explicit permission and add the endpoint to the draft, then Save.

Permission classes are read, create/publish, update and delete. No permission is
preselected. The provider's `kind` or HTTP verb does not establish a permission.
Read-only blocks all classes except read. Specialist agents are limited to read.
Revoke an entry in the draft and Save to stop subsequent calls. Changes to endpoint
ID/method/path need renewed approval; price or description corrections alone do not.
New grants take effect in a new agent run. Disabling, revocation, read-only and a
lower cost cap apply before the next request of an existing run.

## Cost and outcomes
The default limit is **1 USD per call**, encoded as integer micro-USD. Zero permits
only calls that treg can serve within a zero budget. The backend sets the cost
header; the model cannot replace it. The ceiling covers **treg fees only**. Charges
to your own connected provider accounts and costs created by actions (such as
advertising campaigns) are outside this ceiling.

The result separates reserved amounts, settled charges and raw cost-header metadata.
Missing amounts remain unknown. A receipt ID links to treg's accounting. An accepted
media task is pending, not completed. A timeout or lost connection is ambiguous:
AuraGo does not retry automatically. Keep the receipt/idempotency key and inspect
treg history before deciding whether any new call is appropriate.

Status polling uses a server-owned, token-and-session-bound reference. Each poll is
one provider status check, followed by bounded media retrieval when complete, with
no background scheduler. References expire after 24 hours or a
restart. Existing tool history retains the treg call ID for later accounting.
Binary media, declared download URLs and inline image results are stored in the
normal data/files tree and media registry. Uploads obey the
agent workspace jail and protected-path checks. CDN downloads get no treg token.

## Configuration
```yaml
treg:
  enabled: false
  readonly: true
  max_call_cost_micro: 1000000
  allowed_endpoints: []
```
The Vault key is `treg_token`; its value is never serialized into YAML/config
responses or exported to Python tools. Permissions contain `endpoint_id`, `method`,
`path` and `operation`. Use the Config UI to capture the current catalog contract.
Administrative `/api/treg/` routes expose local status, catalog/detail, balance and
connection test only. They do not execute catalog endpoints.

Provider payloads and catalog text remain untrusted external data. No account
administration, CLI execution, automatic top-up or arbitrary team API URL is exposed.
See [the tool manual](../prompts/tools_manuals/treg.md) for request shapes and limits.
Protocol references: [treg documentation](https://treg.to/llms.txt) and
[integration notes](https://treg.to/integrate.md).

## Verification
Focused checks: `go test ./internal/config ./internal/tools ./internal/agent
./internal/server -run Treg`, plus `AURAGO_RUN_BROWSER_SMOKE=1 go test ./ui -run Treg`.
The fixtures do not spend credits or authenticate with a real account. Live
acceptance requires an operator-provided organization token and an explicitly
approved endpoint/budget; mock success is not evidence of live provider success.
