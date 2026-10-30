# virtual_desktop_apps

Open, inspect, and diagnose virtual desktop apps. Install generated apps with `virtual_desktop_app_install`.

Use this tool for app and document launch requests from Desktop Chat and Live Speech:

- Open an app: `{"operation":"open_app","app_id":"calculator"}`.
- Open an existing document: `{"operation":"open_in_app","app_id":"writer","path":"Documents/report.docx"}`.
- Use `list_apps` to discover exact IDs; use `virtual_desktop_files` with
  `list_files` to locate documents. Reuse the returned workspace-relative paths.
- `writer` opens DOCX and imports text/HTML; `sheets` opens spreadsheets/CSV;
  `editor` opens plain text; `viewer` displays PDFs/images. Code Studio paths
  stay inside the Desktop workspace mounted at `/workspace`.
- Opening a document never requires rewriting it. Host paths and files on the
  browser device must first be imported through the existing Desktop file flow.
- Agent control and tool access must be enabled. Desktop readonly blocks agent
  runs. A successful result emits a browser launch event; it does not attest
  that the app finished rendering or loading the document.

Generated apps should bundle local assets and use the Aura Desktop SDK rules documented in `virtual_desktop.md`.
