# Homepage registry

Persistent managed website identity, provenance, local/remote locations, changes,
reasons, revisions and observations. Query before editing; never invent a folder
or deployment URL.

## Read operations

- overview: query, limit (default 20, maximum 100), offset. Returns sites, total,
  optional next_offset and ledger_version.
- get_site: internal id or exact project_dir; includes targets and recent events.
- list_events: project id, limit, offset; what changed, why and the outcome.
- discover_workspace: unregistered directory candidates; read-only.
- audit_revisions: legacy credential filenames without secret contents.
- Legacy list, search and get retain their metadata response envelopes.

## Identity and changes

- adopt: existing project_dir and reason.
- relocate: existing id, new existing project_dir and reason. Preserves history;
  it does not move files.
- register: name, exact project_dir, reason and optional metadata.
- update: id or name, reason and changed metadata.
- delete: id or name and reason; archives the record and retains history.

Origin is agent_created, adopted or unknown. managed_site_id on focused tools
is internal; provider site_id/project_id remain separate. Workspace changes do
not silently rebind existing projects.

Every agent Homepage mutation requires reason: a short purpose rather than an
operation name. Actual file changes and final outcomes are recorded
automatically. Do not repeat normal changes with log_edit and add_history.
Use get_site/list_events to answer what changed, why, where and whether it was
verified. Unknown reasons stay unknown; uncertain actions must not be replayed.

## Additional notes and compatibility

Use add_history for decisions, questions, feedback and explanatory notes.
History operations remain list_history, get_history, search_history,
update_history and delete_history. id identifies the project, history_id a note.
Entry types remain note, decision, question, feedback, milestone and observation;
optional source, tags, limit and offset retain their meaning.
Legacy log_edit, log_deploy, log_problem and resolve_problem accept reason.
Manual deploy notes do not prove that a provider is live.
