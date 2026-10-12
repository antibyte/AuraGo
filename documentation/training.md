# External training repository

The versioned synthetic datasets, curated operation fixtures, Python validators,
Unsloth environment and Needle3 research live in
[antibyte/agotrain](https://github.com/antibyte/agotrain), normally checked out at
`../agotrain`. AuraGo does not need that repository to build or run.

AuraGo retains `disposable/export_tools` and `disposable/import_training_traces`:
they import the real internal schemas, manual catalog and credential scrubber.
Moving or duplicating these Go packages into an unrelated module would break Go's
`internal` boundary and let training drift away from the runtime contract.

## Verify and refresh

From the AuraGo root:

```sh
go run ./disposable/export_tools --out ../agotrain/training --check
go run ./disposable/export_tools --source ../agotrain/training --manual-router-out ../agotrain/training/needle3/catalog.json --check
```

The exporter defaults to `../agotrain/training`; explicit paths support other
checkout layouts. Remove `--check` to regenerate after reviewing the curated
manifests. Import traces only from an explicitly approved file, with `--staging`
pointing into agotrain's ignored `training/trace_staging/` area.

Set `AURAGO_TRAINING_DIR` to an **absolute** path to `agotrain/training` to run the
three exporter integration tests and validate Layerling against the real training
contracts. Without that variable, only those three cross-repository tests skip;
Layerling still validates its local operation fixture, and exporter unit tests
and importer privacy tests still run. Invalid explicit paths fail, never skip.

The Training Dataset Gates workflow always sets this variable, checks out the
exact agotrain commit in `.github/agotrain-revision`, and verifies regeneration
against the candidate AuraGo code. agotrain's own CI checks the exact AuraGo
commit in its `aurago-revision`. Both use ordinary read-only public checkouts.

For a tool change, reconcile agotrain's curated contracts and regenerate using
the new AuraGo source. Commit the compatible pack, then update
`.github/agotrain-revision` here. After the AuraGo change is published, advance
agotrain's `aurago-revision`. These sequential pins avoid circular commit IDs;
there is no submodule or automatic cross-repository push.

## Migration and retained material

The extraction preserves the `training/` layout in agotrain and records the
original AuraGo commit in `origin.json`. The new repo starts with a reviewed
snapshot. AuraGo history is retained without rewriting or force-pushing; old
clones still contain historical dataset blobs.

Only tracked source and synthetic data moved. Existing ignored local models,
environments, trace staging, reports and private cost ledgers were left in their
original locations. They must never be swept into the new public repository.
Migration grants no permission for paid generation or GPU/RunPod execution.

Known Python advisories belong to agotrain's `SECURITY.md`. Relocation does not
patch vulnerable packages; do not dismiss their alerts as fixed. Neither local
CPU checks nor CI success establishes GPU compatibility.
