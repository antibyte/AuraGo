# virtual_desktop_files

Manage files in AuraGo's virtual desktop workspace.

Use this for normal workspace file reads, writes, patches, searches, deletes, and exports. Route Writer documents to `office_document` and spreadsheets to `office_workbook`.

Workspace paths are relative; never supply a host path. Desktop readonly blocks
mutations and agent runs. Existing Notes and their protected Trash/Notes copies
remain unavailable to native agent mutation. File-manager trash/restore preserves
Notes subfolders. HTTP/SDK clients must retain each read ETag, use If-Match when
saving, and use If-None-Match: * for a new destination. On a conflict, ask the user
to replace the observed version, keep a distinct copy, or cancel; never silently
overwrite. These HTTP preconditions do not add arguments to this native tool.
