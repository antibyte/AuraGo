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
- register: name, exact project_dir, reason and optional metadata. A project_dir
  that matches a registered site (case-insensitive on Windows and macOS) returns
  status `duplicate` with its id. An ancestor or descendant of another active
  local site is rejected, as with the other boundary checks.
- update: id or name, reason and changed metadata.
- delete: id or name and reason; archives the record and retains history.

Origin is agent_created, adopted or unknown. managed_site_id on focused tools
is internal; provider site_id/project_id remain separate. Workspace changes do
not silently rebind existing projects.

Site boundaries: a new project_dir that is an ancestor or a descendant of another
active local site is rejected, and the error names that site's managed_site_id.
Sibling folders are fine. adopt and relocate apply the same check. On Windows and
macOS a case variant of a registered project_dir resolves to that site and the
registry spelling is used; Linux matches exactly. Auto-derived site names that
collide get a -2, -3 ... suffix. Discovery after exec skips folders that break
these boundaries and reports a warning.

Identity errors: an unknown managed_site_id returns `managed site N not found;
list sites with homepage_registry overview`. A remote-only site has no local
folder, so combining its managed_site_id with a file path is an error ("remote-only
... attach one with homepage_registry relocate first, or omit managed_site_id").

Every agent Homepage mutation requires reason: a short purpose rather than an
operation name. Actual file changes and final outcomes are recorded
automatically. Do not repeat normal changes with log_edit and add_history.
Use get_site/list_events to answer what changed, why, where and whether it was
verified. Unknown reasons stay unknown; uncertain actions must not be replayed.

## Additional notes and compatibility

Use add_history for decisions, questions, feedback and explanatory notes.
History operations remain list_history, get_history, search_history,
update_history and delete_history. id identifies the project, history_id a note.
delete_history requires `reason` and records a `history_deleted` event (history id,
entry type, a 120-character excerpt, reason and actor). Deleting in the Studio
history panel records actor `user`. add_history and update_history are unchanged.
Entry types remain note, decision, question, feedback, milestone and observation;
optional source, tags, limit and offset retain their meaning.
Legacy log_edit, log_deploy, log_problem and resolve_problem accept reason.
Manual deploy notes do not prove that a provider is live.
