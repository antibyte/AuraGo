# ziro_export

Exports saved KiCad 10 projects through the Store's isolated export worker.

## Requirements

Connect the installed Ziro Designer Store app and grant execution. Docker must
be enabled and writable, Desktop agent control and file publication enabled,
and the versioned export image installed. A connection test checks the worker
version and CLI capabilities without exporting a project.

## Operations

- `netlist`: supply `path` to a saved `.kicad_sch` or matching `.kicad_pro`
  below Shared. Returns a new `.net` file in KiCad S-expression format.
- `manufacturing`: supply `path` to a saved `.kicad_pcb` or matching
  `.kicad_pro`. Returns a ZIP containing Gerber-X2, Excellon drill files and
  `export-manifest.json` with source/output hashes and KiCad version.

Keep each project in its own Shared directory, including its relative libraries
and hierarchical sheets. The snapshot is limited to 2048 entries, 32 levels and
the configured file-size limit (at most 64 MiB). Links, escaping references and
missing libraries are refused. Unsaved editor drafts are never exported.

One job runs at a time, with no network, a read-only container root and only its
private job directory mounted. The five-minute deadline includes queueing and
image preparation. Cancellation or binding revocation kills the worker.

Outputs use a new `Shared/Store/ziro/<job>/` directory; sources remain unchanged.
Export success is not an electrical/design-rule or fabrication-quality approval.
