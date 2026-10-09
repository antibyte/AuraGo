"""Build the Local Wikipedia test ZIMs with python-libzim (dev tooling only).

Runs inside python:3.12-slim-trixie with the hash-pinned libzim wheel from
requirements.txt (libzim 9.8.2, Xapian 1.4.23, ICU 73.2). Never used at runtime
or by `go test`. Invoked by generate.sh:

    python scripts/localwiki/fixtures/make_zims.py \
        --zim-dir internal/zim/testdata --work disposable/_localwiki_fixtures

Writes fixture_<name>.zim into --zim-dir and, into --work, the extracted Xapian
blobs (<name>_fulltext.glass, <name>_title.glass) plus libzim_results.json with
python-libzim's own Searcher / SuggestionSearcher results for the fixed queries.
libzim prints progress lines on stdout, so all results go to files.
"""
import argparse
import hashlib
import html
import json
import pathlib
import sys

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import corpus  # noqa: E402
import libzim.version  # noqa: E402
from libzim.reader import Archive  # noqa: E402
from libzim.search import Query, Searcher  # noqa: E402
from libzim.suggestion import SuggestionSearcher  # noqa: E402
from libzim.writer import Creator, Hint, Item, StringProvider  # noqa: E402

RESULT_LIMIT = 50


class Page(Item):
    def __init__(self, path, title, body_html):
        super().__init__()
        self._path, self._title, self._html = path, title, body_html

    def get_path(self):
        return self._path

    def get_title(self):
        return self._title

    def get_mimetype(self):
        return "text/html"

    def get_contentprovider(self):
        return StringProvider(self._html)

    def get_hints(self):
        return {Hint.FRONT_ARTICLE: True, Hint.COMPRESS: True}


def page_html(title, paragraphs):
    body = "".join(f"<p>{html.escape(p)}</p>" for p in paragraphs)
    t = html.escape(title)
    return (
        "<!DOCTYPE html><html><head><meta charset=\"utf-8\">"
        f"<title>{t}</title></head><body><h1>{t}</h1>{body}</body></html>"
    )


def hexchain(n):
    out, h = [], b"aurago-localwiki-bulk"
    while sum(len(x) for x in out) < n:
        h = hashlib.sha256(h).digest()
        out.append(h.hex())
    return "".join(out)[:n]


def bulk_articles(count):
    for i in range(1, count + 1):
        title = f"Bulk item {i:04d}"
        text = f"Bulk item number {i:04d} shares the word zebra with every other item. Group g{i % 7}."
        yield f"Bulk_{i:04d}", title, [text]
    # One entry with a 4000-byte path (compressed, multi-component docdata tag)
    # and a ~3 KB title (multi-component value chunk and value statistics).
    words = hexchain(75 * 27)
    long_title = " ".join("longtitleword" + words[k * 27:(k + 1) * 27] for k in range(75))
    yield "Long_" + hexchain(4000), long_title, ["zebra long entry"]


def build(zim_path, language, title, articles, redirects):
    if zim_path.exists():
        zim_path.unlink()
    creator = Creator(zim_path).config_indexing(True, language).config_nbworkers(1)
    creator.config_compression("zstd")
    with creator as c:
        c.add_metadata("Name", zim_path.stem)
        c.add_metadata("Title", title)
        c.add_metadata("Language", language)
        c.add_metadata("Creator", "AuraGo")
        c.add_metadata("Publisher", "AuraGo")
        c.add_metadata("Date", "2026-10-09")
        c.add_metadata("Description", "Synthetic test fixture written for AuraGo")
        first = None
        for path, art_title, paragraphs in articles:
            first = first or path
            c.add_item(Page(path, art_title, page_html(art_title, paragraphs)))
        for path, red_title, target in redirects:
            c.add_redirection(path, red_title, target, {Hint.FRONT_ARTICLE: True})
        c.set_mainpath(first)


def extract_indexes(zim_path, work, name):
    archive = Archive(zim_path)
    found = set()
    for i in range(archive.all_entry_count):
        entry = archive._get_entry_by_id(i)
        if entry.path in ("fulltext/xapian", "title/xapian") and not entry.is_redirect:
            kind = entry.path.split("/")[0]
            (work / f"{name}_{kind}.glass").write_bytes(bytes(entry.get_item().content))
            found.add(kind)
    if found != {"fulltext", "title"}:
        raise SystemExit(f"{zim_path}: missing Xapian indexes, found {sorted(found)}")
    return archive


def run_queries(archive, fulltext, suggest):
    searcher = Searcher(archive)
    sugg = SuggestionSearcher(archive)
    out = {"fulltext": {}, "suggest": {}}
    for q in fulltext:
        s = searcher.search(Query().set_query(q))
        out["fulltext"][q] = {
            "estimated": s.getEstimatedMatches(),
            "paths": list(s.getResults(0, RESULT_LIMIT)),
        }
    for q in suggest:
        s = sugg.suggest(q)
        out["suggest"][q] = {"paths": list(s.getResults(0, RESULT_LIMIT))}
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--zim-dir", required=True)
    ap.add_argument("--work", required=True)
    args = ap.parse_args()
    zim_dir = pathlib.Path(args.zim_dir)
    work = pathlib.Path(args.work)
    zim_dir.mkdir(parents=True, exist_ok=True)
    work.mkdir(parents=True, exist_ok=True)

    results = {"libzim": libzim.version.get_versions(), "fixtures": {}}
    jobs = []
    for name, fx in corpus.FIXTURES.items():
        jobs.append((name, fx["language"], fx["title"], fx["articles"], fx["redirects"], fx["fulltext"], fx["suggest"]))
    bulk = corpus.BULK
    jobs.append(("bulk", bulk["language"], bulk["title"], list(bulk_articles(bulk["count"])), [], bulk["fulltext"], bulk["suggest"]))

    for name, language, title, articles, redirects, fulltext, suggest in jobs:
        zim_path = zim_dir / f"fixture_{name}.zim"
        build(zim_path, language, title, articles, redirects)
        archive = extract_indexes(zim_path, work, name)
        results["fixtures"][name] = {"language": language, **run_queries(archive, fulltext, suggest)}
        print(f"built {zim_path} ({zim_path.stat().st_size} bytes)", file=sys.stderr)

    (work / "libzim_results.json").write_text(json.dumps(results, ensure_ascii=False, indent=1), encoding="utf-8")


if __name__ == "__main__":
    main()
