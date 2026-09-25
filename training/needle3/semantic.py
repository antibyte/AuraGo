"""Surface possible scenario duplicates; a clustering result is not an audit."""
from __future__ import annotations

import argparse
from collections import defaultdict

from .assets import MODEL_DIR
from .common import canonical, digest, read_jsonl, write_json, write_jsonl


def find_pairs(rows, threshold=.97):
    import numpy as np
    from sentence_transformers import SentenceTransformer
    groups = {}
    for row in rows:
        groups.setdefault(row["group_id"], {"group_id": row["group_id"], "split": row["split"], "scenario": row["scenario"]})
    groups = [groups[k] for k in sorted(groups)]
    model = SentenceTransformer(str(MODEL_DIR / "retriever"), device="cpu", local_files_only=True, trust_remote_code=False)
    vectors = model.encode(["query: " + g["scenario"] for g in groups], normalize_embeddings=True, batch_size=32)
    pairs = []
    for start in range(0, len(groups), 256):
        similarities = vectors[start:start + 256] @ vectors.T
        for a, b in zip(*np.where(similarities >= threshold)):
            a += start
            if a < b:
                pairs.append({"left": groups[a], "right": groups[b], "similarity": float(similarities[a - start, b]),
                              "cross_split": groups[a]["split"] != groups[b]["split"], "decision": "unreviewed"})
    return pairs


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("input")
    parser.add_argument("--out", required=True)
    parser.add_argument("--threshold", type=float, default=.97)
    args = parser.parse_args()
    rows = list(read_jsonl(args.input))
    pairs = find_pairs(rows, args.threshold)
    write_jsonl(args.out, pairs)
    write_json(args.out + ".receipt.json", {"corpus_sha256": digest(canonical(rows)), "threshold": args.threshold,
                                           "candidate_groups": len({r["group_id"] for r in rows}), "distinct_groups": 0,
                                           "unresolved_pairs": len(pairs), "reviewer": "", "approved": False})
