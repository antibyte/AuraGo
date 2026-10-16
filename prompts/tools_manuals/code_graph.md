# code_graph

Native, read-only structural analysis of the current server-authorized project.
Requires `code_graph.enabled`. First use builds a resident index; active project
indexes are refreshed from content fingerprints. No root/path argument can grant
access to another project, container or host directory.

Operations:

- `status`: freshness, coverage, skipped-file reasons, unresolved-edge count,
  retained index estimate and last indexing duration.
- `search`: symbol-name fragment in `query`; returns IDs, locations and snippets.
- `outline`: definitions in a project-relative `file`.
- `callers`, `callees`: exact `symbol` ID from search/outline; optionally `file`
  disambiguates a name. A missing target ID means an unresolved call.
- `impact`: incoming relationships, bounded by `depth` 1..4 (default 1).
- `rescan`: refresh now. It changes only the in-memory index.

`limit` is bounded to 100 (default 30). Go, JS/JSX, TS/TSX and Python are supported.
Calls resolve only through unambiguous lexical/project imports. Dynamic dispatch,
reflection, external modules and ambiguous names remain unresolved. No rename,
code generation, source execution or write operation is provided.

Always report coverage and freshness with conclusions. Missing edges never prove
that a change has no impact. Syntax errors, cancellations and resource limits do
not produce a complete index. Generated/dependency directories and linked files
are excluded. Limits: one parser worker, 1 MiB per file, 64 MiB parser allocation
budget, 250 ms parse timeout, 256 MiB estimated retained index, 16 active projects.
Source and index stay in RAM and are rebuilt on demand after restart.

Project agents use their own job workspace. A Game Maker or CYD job cannot use
the graph to cross into another job or the ordinary agent workspace. C/C++ is
outside the supported language set, including CYD-generated firmware.
