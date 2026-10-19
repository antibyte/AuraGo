# Netlify integration

AuraGo can manage Netlify sites, deploys, environment variables, notification
hooks, forms and SSL certificates, and it can publish a Homepage project as a
Netlify deploy. The integration is off by default: enable it under
**Configuration → Netlify** and store a personal access token first.

## Setup

1. Create a token at **Netlify → Applications → Personal access tokens**. Grant
   access to the team when the team uses SAML SSO, otherwise the token is denied
   for that team.
2. Store it in the Vault under the exact key `netlify_token` (Config → Netlify →
   **Save token**).
3. Enable the integration and only the grants you need.

```yaml
netlify:
  enabled: true
  readonly: false
  allow_deploy: false             # rollback and cancel_deploy
  allow_site_management: false    # create/update/delete site, hooks, SSL
  allow_env_management: false     # set_env and delete_env
  default_site_id: ""             # optional; a site UUID, name or custom domain
  team_slug: ""                   # optional; required for multi-account tokens
```

`readonly: true` blocks every mutation regardless of the individual grants. The
token itself never appears in `config.yaml`.

## Permissions

| Operation | Read-only blocks | Needed grant |
| --- | --- | --- |
| `list_sites`, `get_site`, `list_deploys`, `get_deploy`, `list_env`, `get_env`, `list_files`, `list_forms`, `get_submissions`, `list_hooks`, `check_connection` | no | – |
| `create_site`, `update_site`, `delete_site`, `create_hook`, `delete_hook`, `provision_ssl` | yes | `allow_site_management` |
| `rollback`, `cancel_deploy` | yes | `allow_deploy` |
| `set_env`, `delete_env` | yes | `allow_env_management` |
| `homepage` → `deploy_netlify` | yes | `allow_deploy` (plus `enabled`) |

`delete_site` is permanent and also removes the site's forms and deploy history.
Keep `allow_site_management` off when an agent should never delete a site.

Environment variable reads return metadata only (key, scopes, timestamp, value
count). Secret values are never returned to the model.

## Homepage deploys

`homepage` with `operation: deploy_netlify` builds the project, validates the
static output, uploads one ZIP, polls the deploy and verifies the result. The
agent never transports ZIP bytes through tool arguments.

- **Only regular files are packed.** Symlinks, sockets, devices and FIFOs in the
  build output are skipped, so a link can never publish a file from outside the
  deploy tree.
- **Credential material is never packed.** `.env*`, `*.env`, `private keys`,
  `*.pem`, `*.p12`, `*.pfx`, `*.crt`, `*.jks`, `*.keystore`, `*.kdbx`,
  `.netrc`, `.git-credentials`, `.htpasswd`, SQLite files, logs, `vault.bin` and
  the AuraGo config files are filtered by name.
- **The deploy is bounded.** At most 25 000 files and 100 MiB of source bytes;
  beyond that the deploy fails with a clear message instead of an opaque
  provider error.
- **MIME types come from one `_headers` file.** AuraGo does not generate a
  `netlify.toml`; a project that ships its own configuration keeps it.
- **The single-page rewrite is opt-in by structure.** A catch-all
  `_redirects` to `index.html` is added only when the output contains a single
  root `index.html` (optionally with `404.html`). A multi-page site keeps
  Netlify's real 404 instead of a soft 200. A project that ships several HTML
  files but still needs the rewrite (per-locale entry documents, prerendered
  routes) provides its own `_redirects`; AuraGo then leaves it untouched.
- **`deploy_url` is the deploy permalink, `site_url` is the published site.**
  They are never mixed up. Verification always uses the permalink.
- **Draft deploys never claim a publication.** With `draft: true` the result
  carries `published: false`. The permalink may require a Netlify login, so a
  missing verification is reported as `verified: false` with a
  `verification_note` instead of a failure, and the Homepage ledger and history
  do not record the deploy.
- **Failed builds are not deployed blindly.** An unreadable build answer aborts
  the deploy; a failed build only falls back to a detected static sibling
  project.

The Homepage ledger records a deployment only when the deploy was verified and
published.

## Site names and IDs

Every Netlify path accepts the site UUID, its name or its custom domain. AuraGo
resolves a name or domain to the UUID before deploying (following Netlify's
pagination, so a site beyond the first 100 entries is still found) and
auto-creates the site only when `allow_site_management` is on and no
`default_site_id` is configured.

## Environment variables and accounts

Netlify's environment variable API is account-scoped, so operations need the
team. When `team_slug` is empty AuraGo asks Netlify for the visible accounts: a
single account is adopted automatically, several accounts require an explicit
`team_slug`, and none is an error. The legacy `/sites/{id}/env` fallback is not
used.

## Limits

Netlify rate limits the API at 500 requests per minute overall, **3 deploys per
minute and 100 per day**. AuraGo repeats an idempotent request (GET/HEAD) at most
twice on `429`, `500`, `502`, `503` or `504` and honours `Retry-After` up to 30
seconds. Mutations are sent exactly once: a failed `set_env` retries the
single-key update only when Netlify answered that the variable already exists.
Netlify terminates API requests after 30 seconds; AuraGo allows an upload up to
10 minutes so a slow uplink can still deliver the body.

## Diagnostics

`check_connection` reports `dns_ok`, `tcp_ok`, `api_ok`, `resolved_ips` and the
account e-mail/slug, and `check_connection` runs its DNS and TCP probes through
the same transport as the real requests. A failure therefore points at the same
network path the API call uses.

- DNS failure: check the resolver or a blocked hostname.
- TCP failure: outbound HTTPS to `api.netlify.com:443` is blocked. AuraGo does
  not use an `HTTP_PROXY` for this integration.
- HTTP 401: the token was invalidated, for example by a password reset.

## Security notes

- The bearer token only travels to the configured API origin. A redirect to any
  other origin is refused before the header is sent, and `Link` pagination
  targets are checked against the same origin.
- The token is registered for log and output scrubbing as soon as it is read
  from the Vault.
- Provider error bodies are parsed and bounded before they reach the model;
  they can never exceed 300 characters or break the JSON result envelope.

## Troubleshooting

| Symptom | Cause | Fix |
| --- | --- | --- |
| `Netlify integration is not enabled` | `netlify.enabled` is false | enable the integration |
| `Netlify token not found in vault` | no `netlify_token` | save the token in Config |
| `Netlify deploy is not allowed` | `allow_deploy` is false | grant it, or use a read-only operation |
| `set netlify.team_slug` | token sees several accounts | set the team slug |
| `too many files for a Netlify ZIP deploy` | output above 25 000 files | deploy a smaller build |
| `draft deploy: the deploy permalink could not be verified` | draft previews are login-gated | expected; nothing to fix |
| `HTTP 429` | rate limit | wait; deploys are limited to 3 per minute |

## Tests

```powershell
go test ./internal/tools -run 'Netlify|HomepageDeployNetlify' -count=1
go test ./internal/agent -run 'Netlify' -count=1
go test ./internal/server -run 'Netlify' -count=1
go test ./internal/audit -run 'CredentialedHTTPClients' -count=1
```

Live checks that no unit test can cover: a draft deploy, a published deploy of a
multi-page site, and a listing of an account with more than 100 sites.
