# homepage_quality

Run homepage quality checks: Lighthouse, screenshots, JavaScript console checks, and lint.

Use this after file or project changes before deployment.

## Lighthouse score attribution

`lighthouse` stores its score on a site only when `managed_site_id` is given, or when the audited URL exactly matches the URL of exactly one site or deploy target (trailing slash and scheme/host case ignored). Otherwise the result carries the note `lighthouse_score not stored: pass managed_site_id to attribute the score`.


## Managed website identity and change reasons

Query homepage_registry overview/get_site before changing an existing page. Every mutation requires reason, a short statement of purpose. managed_site_id is the internal identity, distinct from provider IDs. Exec requires project_dir and starts there. Changes and outcomes are recorded automatically; avoid duplicate routine notes. Use list_events for what changed and why, and discover_workspace/adopt for unregistered projects. Historical unknown reasons remain unknown.
