"""Record golden JSON for internal/zim/xapian from real Xapian (dev tooling only).

Runs inside debian:trixie-slim with the Debian packages xapian-tools,
python3-xapian (Xapian 1.4.29) and python3-icu. Invoked by generate.sh:

    python3 scripts/localwiki/fixtures/make_goldens.py \
        --work disposable/_localwiki_fixtures --out internal/zim/xapian/testdata

Inputs (written by make_zims.py): <name>_fulltext.glass, <name>_title.glass and
libzim_results.json in --work. Outputs in --out:

  db_<name>_<kind>.json   statistics, metadata, terms + postings, document
                          lengths, document data and value slots 0/1
  queries_<name>.json     libzim's own result order plus a python3-xapian
                          replica of libzim's query construction with weights
  analysis.json           normalisation, tokenisation, query-parse and stemmer
                          goldens

The replica is checked against libzim's order; any mismatch aborts so that
golden data never encodes a guess.
"""
import argparse
import json
import pathlib
import subprocess
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import corpus  # noqa: E402
import icu  # noqa: E402
import xapian  # noqa: E402

ANCHOR = "0posanchor "
LIMIT = 50
TRANSLIT = icu.Transliterator.createInstance("Lower; NFD; [:M:] remove; NFC", icu.UTransDirection.FORWARD)
STEM_LANGS = ["da", "de", "en", "es", "fr", "it", "nb", "nl", "no", "pt", "sv"]


def remove_accents(text):
    return TRANSLIT.transliterate(text)


def stemmer_for(language):
    two = icu.Locale(language).getLanguage()
    try:
        return xapian.Stem(two)
    except xapian.InvalidArgumentError:
        return xapian.Stem("none")


def dec(b):
    return b.decode("utf-8")


def dump_db(path, name, kind, golden_terms):
    """golden_terms=None dumps everything; a set limits terms and sampled documents (bulk)."""
    db = xapian.Database(str(path))
    delve = subprocess.run(["xapian-delve", str(path)], capture_output=True, text=True, check=True).stdout
    out = {
        "fixture": name,
        "kind": kind,
        "delve": delve,
        "doccount": db.get_doccount(),
        "lastdocid": db.get_lastdocid(),
        "total_length": db.get_total_length(),
        "avlength": db.get_avlength(),
        "doclength_lower_bound": db.get_doclength_lower_bound(),
        "doclength_upper_bound": db.get_doclength_upper_bound(),
        "has_positions": db.has_positions(),
        "metadata": {dec(k): dec(db.get_metadata(k)) for k in db.metadata_keys()},
        "term_count": 0,
        "terms_complete": golden_terms is None,
        "terms": [],
        "doclens": [],
        "data": [],
        "values": {"0": [], "1": []},
    }
    for item in db.allterms():
        out["term_count"] += 1
        term = dec(item.term)
        if golden_terms is not None and term not in golden_terms:
            continue
        out["terms"].append({
            "term": term,
            "tf": item.termfreq,
            "cf": db.get_collection_freq(item.term),
            "postings": [[p.docid, p.wdf] for p in db.postlist(item.term)],
        })
    last = db.get_lastdocid()
    for did in range(1, last + 1):
        out["doclens"].append([did, db.get_doclength(did)])
        doc = db.get_document(did)
        if golden_terms is not None and did not in (1, 2, 3, last - 1, last) and not doc.get_data().startswith(b"C/Long_"):
            continue
        out["data"].append([did, dec(doc.get_data())])
        for slot in (0, 1):
            value = doc.get_value(slot)
            if value:
                out["values"][str(slot)].append([did, dec(value)])
    return db, out


def fulltext_query(db, language, query, op):
    # libzim hands its QueryParser an empty Database handle (set_database() runs
    # before the real database is attached), so no set_database() here either:
    # that matters for the C++/C# suffix rule, which consults term_exists().
    qp = xapian.QueryParser()
    qp.set_default_op(xapian.Query.OP_AND)
    qp.set_stemmer(stemmer_for(language))
    qp.set_stemming_strategy(xapian.QueryParser.STEM_ALL)
    parsed = qp.parse_query(remove_accents(query), xapian.QueryParser.FLAG_CJK_NGRAM)
    terms = [dec(t) for t in parsed]
    if op == "or":
        parsed = xapian.Query(xapian.Query.OP_OR, [xapian.Query(t) for t in parsed])
    enq = xapian.Enquire(db)
    enq.set_query(parsed)
    mset = enq.get_mset(0, LIMIT, LIMIT)
    return terms, mset.get_matches_estimated(), [[m.docid, m.weight, dec(m.document.get_data())] for m in mset]


def suggest_query(db, language, query):
    qp = xapian.QueryParser()  # no set_database(): see fulltext_query
    qp.set_default_op(xapian.Query.OP_AND)
    qp.set_stemmer(stemmer_for(language))
    uq = remove_accents(query)
    qp.set_stemming_strategy(xapian.QueryParser.STEM_SOME)
    flags = xapian.QueryParser.FLAG_DEFAULT | xapian.QueryParser.FLAG_PARTIAL | xapian.QueryParser.FLAG_CJK_NGRAM
    xq = qp.parse_query(uq, flags)
    if uq and xq.empty():
        xq = xapian.Query(xapian.Query.OP_WILDCARD, uq)
    elif uq:
        qp.set_stemming_strategy(xapian.QueryParser.STEM_NONE)
        ph = qp.parse_query(uq, xapian.QueryParser.FLAG_CJK_NGRAM)
        ph = xapian.Query(xapian.Query.OP_PHRASE, [xapian.Query(t) for t in ph], ph.get_length())
        an = qp.parse_query(ANCHOR + uq, xapian.QueryParser.FLAG_CJK_NGRAM)
        an = xapian.Query(xapian.Query.OP_PHRASE, [xapian.Query(t) for t in an], an.get_length())
        xq = xapian.Query(xapian.Query.OP_OR, xapian.Query(xapian.Query.OP_OR, xq, ph), an)
    enq = xapian.Enquire(db)
    enq.set_query(xq)
    enq.set_weighting_scheme(xapian.BM25Weight(0.001, 0, 1, 1, 0.5))
    enq.set_sort_by_relevance_then_value(0, False)
    enq.set_collapse_key(1)
    mset = enq.get_mset(0, LIMIT, LIMIT)
    return str(xq), [[m.docid, m.weight, dec(m.document.get_data()), dec(m.document.get_value(0))] for m in mset]


def strip_ns(data):
    return data[2:] if len(data) > 2 and data[1] == "/" else data


def queries_for(name, language, ft_db, title_db, libzim_results, fulltext, suggest):
    out = {"fixture": name, "language": language, "fulltext": [], "suggest": []}
    failures = []
    for q in fulltext:
        terms, est, and_hits = fulltext_query(ft_db, language, q, "and")
        _, _, or_hits = fulltext_query(ft_db, language, q, "or")
        lz = libzim_results["fulltext"][q]
        replica_paths = [strip_ns(h[2]) for h in and_hits][:len(lz["paths"])]
        if replica_paths != lz["paths"]:
            failures.append(f"{name} fulltext {q!r}: libzim {lz['paths']} != replica {replica_paths}")
        out["fulltext"].append({"query": q, "terms": terms, "libzim": lz, "estimated": est, "and": and_hits, "or": or_hits})
    for q in suggest:
        desc, hits = suggest_query(title_db, language, q)
        lz = libzim_results["suggest"][q]
        replica_paths = [strip_ns(h[2]) for h in hits][:len(lz["paths"])]
        if replica_paths != lz["paths"]:
            failures.append(f"{name} suggest {q!r}: libzim {lz['paths']} != replica {replica_paths} ({desc})")
        out["suggest"].append({"query": q, "xapian_query": desc, "libzim": lz, "replica": hits})
    return out, failures


def tokenize(text):
    tg = xapian.TermGenerator()
    tg.set_flags(xapian.TermGenerator.FLAG_CJK_NGRAM)
    tg.set_stemming_strategy(xapian.TermGenerator.STEM_NONE)
    doc = xapian.Document()
    tg.set_document(doc)
    tg.index_text(text)
    return {dec(t.term): {"wdf": t.wdf, "positions": list(t.positer)} for t in doc.termlist()}


def analysis():
    out = {"normalize": [], "tokenize": [], "queryparse": [], "stems": {}, "index_words": {}}
    for s in corpus.NORMALIZE_SAMPLES:
        out["normalize"].append([s, remove_accents(s)])
    for s in corpus.TOKENIZE_SAMPLES:
        out["tokenize"].append({"text": s, "terms": tokenize(s)})
    for name, fx in corpus.FIXTURES.items():
        qp = xapian.QueryParser()
        qp.set_default_op(xapian.Query.OP_AND)
        qp.set_stemmer(stemmer_for(fx["language"]))
        qp.set_stemming_strategy(xapian.QueryParser.STEM_ALL)
        for q in fx["fulltext"] + fx["suggest"]:
            parsed = qp.parse_query(remove_accents(q), xapian.QueryParser.FLAG_CJK_NGRAM)
            out["queryparse"].append({"language": fx["language"], "query": q, "terms": sorted(dec(t) for t in parsed)})
    for name, fx in corpus.FIXTURES.items():
        seen = set()
        for _, title, paragraphs in fx["articles"]:
            for text in [title] + paragraphs:
                seen.update(tokenize(remove_accents(text)).keys())
        out["index_words"][name] = sorted(w for w in seen if len(w.encode("utf-8")) <= 64)
    words = {lang: set() for lang in STEM_LANGS}
    for lang, extra in corpus.EXTRA_STEM_WORDS.items():
        words[lang].update(remove_accents(w) for w in extra)
    for name in ("de", "en"):
        fx = corpus.FIXTURES[name]
        for _, title, paragraphs in fx["articles"]:
            for text in [title] + paragraphs:
                words[name].update(tokenize(remove_accents(text)).keys())
    for lang in STEM_LANGS:
        st = xapian.Stem(lang)
        out["stems"][lang] = sorted([w, dec(st(w))] for w in words[lang] if w and not w[0].isdigit())
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--work", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    work = pathlib.Path(args.work)
    dest = pathlib.Path(args.out)
    dest.mkdir(parents=True, exist_ok=True)
    results = json.loads((work / "libzim_results.json").read_text(encoding="utf-8"))

    def write(name, obj):
        (dest / name).write_text(json.dumps(obj, ensure_ascii=False, indent=1, sort_keys=True) + "\n", encoding="utf-8")

    failures = []
    jobs = [(n, fx["language"], fx["fulltext"], fx["suggest"], None) for n, fx in corpus.FIXTURES.items()]
    jobs.append(("bulk", corpus.BULK["language"], corpus.BULK["fulltext"], corpus.BULK["suggest"], set(corpus.BULK["golden_terms"])))
    for name, language, fulltext, suggest, golden_terms in jobs:
        ft_db, ft = dump_db(work / f"{name}_fulltext.glass", name, "fulltext", golden_terms)
        title_db, ti = dump_db(work / f"{name}_title.glass", name, "title", golden_terms)
        write(f"db_{name}_fulltext.json", ft)
        write(f"db_{name}_title.json", ti)
        queries, fails = queries_for(name, language, ft_db, title_db, results["fixtures"][name], fulltext, suggest)
        failures.extend(fails)
        write(f"queries_{name}.json", queries)
    write("analysis.json", analysis())
    write("versions.json", {"libzim": results["libzim"], "debian_xapian": xapian.version_string()})
    if failures:
        print("\n".join(failures), file=sys.stderr)
        raise SystemExit(1)


if __name__ == "__main__":
    main()
