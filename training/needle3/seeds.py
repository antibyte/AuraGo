"""Import only repository-owned synthetic scenarios as traceable seed material."""
from __future__ import annotations

from .common import ROOT, HOME, canonical, catalog, file_digest, read_jsonl, write_jsonl, write_json


def import_seeds():
    source = ROOT / "training" / "dataset_native_fc.jsonl"
    cat = catalog()
    bindings = {t["name"]: t.get("manual_id") for t in cat["tools"]}
    rows = []
    for row in read_jsonl(source):
        if row.get("source") not in {"operation_contract", "curated_multicall", "curated_error", "curated_discovery", "curated_no_call"}:
            continue
        messages = row["messages"]
        calls = [call for m in messages for call in m.get("tool_calls", [])]
        manuals = list(dict.fromkeys(bindings[c["function"]["name"]] for c in calls if bindings.get(c["function"]["name"])))
        rows.append({"seed_id": row["id"], "source_family": row["family"], "source_split": row["split"],
                     "language": row["language"], "queries": [m["content"] for m in messages if m["role"] == "user"],
                     "suggested_manuals": manuals, "status": "seed_only_not_accepted"})
    write_jsonl(HOME / "generated" / "seeds.jsonl", rows)
    write_json(HOME / "seed_manifest.json", {"source": source.relative_to(ROOT).as_posix(), "source_sha256": file_digest(source),
                                          "rows": len(rows), "accepted_rows": 0, "contains_user_traces": False})
    print(canonical({"imported_synthetic_seeds": len(rows), "accepted_rows": 0}))


if __name__ == "__main__":
    import_seeds()
