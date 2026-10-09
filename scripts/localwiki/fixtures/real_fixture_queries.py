"""Record libzim's results on the real-world test ZIM (dev tooling only).

The ZIM (wikipedia_en_climate_change_mini_2024-06.zim, pinned by slice 1's
zimtest.RealFixture) is not committed; only these libzim results are. Runs in
the same python-libzim container as make_zims.py:

    python scripts/localwiki/fixtures/real_fixture_queries.py \
        --zim /real/wikipedia_en_climate_change_mini_2024-06.zim \
        --out internal/zim/xapian/testdata/real_fixture_libzim.json
"""
import argparse
import json
import pathlib

from libzim.reader import Archive
from libzim.search import Query, Searcher
from libzim.suggestion import SuggestionSearcher

FULLTEXT = ["climate change", "global warming", "carbon dioxide emissions", "ipcc", "the", "renewable energy", "greenhouse"]
SUGGEST = ["clim", "climate c", "global warming", "c", "gre", "paris agr", "ipcc", "kyoto"]
LIMIT = 20


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--zim", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()
    archive = Archive(args.zim)
    searcher = Searcher(archive)
    suggester = SuggestionSearcher(archive)
    out = {"zim": pathlib.Path(args.zim).name, "limit": LIMIT, "fulltext": {}, "suggest": {}}
    for q in FULLTEXT:
        s = searcher.search(Query().set_query(q))
        out["fulltext"][q] = {"estimated": s.getEstimatedMatches(), "paths": list(s.getResults(0, LIMIT))}
    for q in SUGGEST:
        out["suggest"][q] = {"paths": list(suggester.suggest(q).getResults(0, LIMIT))}
    pathlib.Path(args.out).write_text(json.dumps(out, ensure_ascii=False, indent=1, sort_keys=True) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
