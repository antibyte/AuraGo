"""Actual AuraGo catalog search fused with frozen multilingual E5 passages."""
from __future__ import annotations

from collections import defaultdict
import json
from pathlib import Path
import subprocess

from .assets import MODEL_DIR, PINS
from .common import ROOT, HOME, canonical, digest, inference_query, read_json, write_json


def reciprocal_rank(lists, allowed):
    scores = defaultdict(float)
    for ranking in lists:
        for i, mid in enumerate(dict.fromkeys(ranking)):
            if mid in allowed:
                scores[mid] += 1 / (60 + i + 1)
    return sorted(scores, key=lambda mid: (-scores[mid], mid))


class CatalogSearch:
    def __init__(self, executable, catalog_sha):
        self.process = subprocess.Popen([str(Path(executable).resolve()), "--manual-router-search"], cwd=ROOT,
                                        stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL,
                                        text=True, encoding="utf-8", bufsize=1)
        self.catalog_sha = catalog_sha

    def search(self, query, allowed):
        self.process.stdin.write(canonical({"query": query, "allowed_manuals": list(allowed)}) + "\n")
        self.process.stdin.flush()
        line = self.process.stdout.readline()
        if not line:
            raise RuntimeError("AuraGo catalog search exited unexpectedly")
        result = json.loads(line)
        if result["catalog_sha256"] != self.catalog_sha:
            raise ValueError("search binary and dataset catalog differ")
        return result["manual_ids"]

    def close(self):
        self.process.stdin.close()
        try:
            self.process.wait(timeout=5)
        except subprocess.TimeoutExpired:
            self.process.kill()
            self.process.wait()
        self.process.stdout.close()


class Retriever:
    def __init__(self, cat, search_executable):
        import numpy as np
        from sentence_transformers import SentenceTransformer
        self.cat = cat
        self.manual_ids = [m["id"] for m in cat["manuals"]]
        self.model = SentenceTransformer(str(MODEL_DIR / "retriever"), device="cpu", local_files_only=True, trust_remote_code=False)
        self.model.max_seq_length = 512
        self.searcher = CatalogSearch(search_executable, cat["catalog_sha256"])
        key = digest(canonical({"catalog": cat["catalog_sha256"], "retriever": PINS["retriever"], "chunk_tokens": 400}))
        path = HOME / ".cache" / (key + ".npz")
        if path.exists():
            with np.load(path, allow_pickle=False) as archive:
                self.embeddings, self.owners = archive["embeddings"], archive["owners"].tolist()
        else:
            passages, self.owners = [], []
            for manual in cat["manuals"]:
                tokens = self.model.tokenizer.encode(manual["body"], add_special_tokens=False)
                for start in range(0, len(tokens), 350):
                    text = self.model.tokenizer.decode(tokens[start:start + 400], skip_special_tokens=True)
                    passage = "passage: " + manual["id"] + "\n" + text
                    if len(self.model.tokenizer.encode(passage)) > 512:
                        raise ValueError("E5 passage exceeded explicit chunk bound")
                    passages.append(passage)
                    self.owners.append(manual["id"])
            self.embeddings = self.model.encode(passages, normalize_embeddings=True, show_progress_bar=True, batch_size=32)
            path.parent.mkdir(parents=True, exist_ok=True)
            np.savez_compressed(path, embeddings=self.embeddings, owners=np.asarray(self.owners))

    def retrieve(self, row, allowed=None, k=12):
        return self.retrieve_details(row, allowed, k)["manual_ids"]

    def retrieve_details(self, row, allowed=None, k=12):
        import numpy as np
        allowed = set(self.manual_ids if allowed is None else allowed)
        if allowed - set(self.manual_ids):
            raise ValueError("unknown available manual")
        if not allowed:
            return {"manual_ids": [], "embedding_scores": {}}
        query = inference_query(row)
        # E5 length handling is explicit; Needle has its own complete 2048-token gate.
        tokens = self.model.tokenizer.encode(query, add_special_tokens=False)
        chunks = ["query: " + self.model.tokenizer.decode(tokens[i:i + 450], skip_special_tokens=True) for i in range(0, max(1, len(tokens)), 450)]
        vectors = self.model.encode(chunks, normalize_embeddings=True)
        similarity = np.max(self.embeddings @ vectors.T, axis=1)
        scores = defaultdict(lambda: -2.0)
        for owner, score in zip(self.owners, similarity):
            if owner in allowed:
                scores[owner] = max(scores[owner], float(score))
        embedding = sorted(scores, key=lambda mid: (-scores[mid], mid))
        lexical = self.searcher.search(query, allowed)
        ranked = reciprocal_rank([lexical, embedding], allowed)[:k]
        return {"manual_ids": ranked, "embedding_scores": {mid: scores[mid] for mid in ranked}}

    def close(self):
        self.searcher.close()


def choose_k(rows, rankings, cfg):
    if any(row["split"] != "validation" for row in rows):
        raise ValueError("candidate count may only be selected on validation")
    reports = {}
    for k in cfg["candidate_counts"]:
        by_language = defaultdict(lambda: [0, 0])
        for row in rows:
            gold = set(row["all_manual_ids"])
            by_language[row["language"]][0] += len(gold & set(rankings[row["id"]][:k]))
            by_language[row["language"]][1] += len(gold)
        recalls = {lang: hit / total if total else None for lang, (hit, total) in by_language.items()}
        reports[k] = recalls
        if all(recalls.get(lang) is not None and recalls[lang] >= (0.98 if lang in {"de", "en"} else 0.95) for lang in cfg["languages"]):
            return k, reports
    raise ValueError("no candidate count met validation recall targets: " + canonical(reports))
