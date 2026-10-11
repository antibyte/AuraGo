package retrogames

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Search limits and sort orders.
const (
	MaxSearchLimit = 60
	maxQueryRunes  = 200
	maxSystemRunes = 60
	maxMatchTerms  = 8
	maxOffset      = 100000

	SortRelevance = "relevance"
	SortPopular   = "popular"
	SortTitle     = "title"
	SortYear      = "year"
)

// ErrInvalidQuery marks a search request with invalid parameters.
var ErrInvalidQuery = errors.New("retrogames: invalid query")

// Query describes one index search.
type Query struct {
	Text          string
	Platform      string
	System        string
	Decade        int
	Sort          string
	Offset        int
	Limit         int
	IncludeBroken bool
}

// SearchResult is one page of search hits.
type SearchResult struct {
	Games  []Game `json:"games"`
	Total  int    `json:"total"`
	Offset int    `json:"offset"`
	Limit  int    `json:"limit"`
}

// CountEntry is a named count (systems per platform).
type CountEntry struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Normalize validates q and fills defaults.
func (q Query) Normalize() (Query, error) {
	q.Text = strings.TrimSpace(q.Text)
	if utf8.RuneCountInString(q.Text) > maxQueryRunes {
		return q, fmt.Errorf("%w: text too long", ErrInvalidQuery)
	}
	q.Platform = strings.TrimSpace(q.Platform)
	if q.Platform != "" && !ValidPlatform(q.Platform) {
		return q, fmt.Errorf("%w: unknown platform", ErrInvalidQuery)
	}
	q.System = strings.TrimSpace(q.System)
	if utf8.RuneCountInString(q.System) > maxSystemRunes {
		return q, fmt.Errorf("%w: system too long", ErrInvalidQuery)
	}
	if q.Decade != 0 && (q.Decade < 1950 || q.Decade > 2020 || q.Decade%10 != 0) {
		return q, fmt.Errorf("%w: invalid decade", ErrInvalidQuery)
	}
	switch q.Sort {
	case "":
		q.Sort = SortRelevance
	case SortRelevance, SortPopular, SortTitle, SortYear:
	default:
		return q, fmt.Errorf("%w: unknown sort", ErrInvalidQuery)
	}
	if q.Offset < 0 || q.Offset > maxOffset {
		return q, fmt.Errorf("%w: invalid offset", ErrInvalidQuery)
	}
	if q.Limit <= 0 || q.Limit > MaxSearchLimit {
		q.Limit = MaxSearchLimit
	}
	return q, nil
}

// BuildMatch turns free text into a safe FTS5 query: every word (letters and
// digits only) becomes a quoted prefix term; terms are ANDed implicitly.
func BuildMatch(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	terms := make([]string, 0, maxMatchTerms)
	for _, w := range words {
		if len(terms) == maxMatchTerms {
			break
		}
		terms = append(terms, `"`+w+`"*`)
	}
	return strings.Join(terms, " ")
}

// Search runs q against the index.
func (st *Store) Search(ctx context.Context, q Query) (SearchResult, error) {
	q, err := q.Normalize()
	if err != nil {
		return SearchResult{}, err
	}
	match := BuildMatch(q.Text)
	from := "games g"
	var where []string
	var args []any
	if match != "" {
		from = "games_fts JOIN games g ON g.id = games_fts.rowid"
		where = append(where, "games_fts MATCH ?")
		args = append(args, match)
	}
	if q.Platform != "" {
		where = append(where, "g.platform = ?")
		args = append(args, q.Platform)
	}
	if q.System != "" {
		where = append(where, "g.system = ?")
		args = append(args, q.System)
	}
	if q.Decade != 0 {
		where = append(where, "g.year BETWEEN ? AND ?")
		args = append(args, q.Decade, q.Decade+9)
	}
	if !q.IncludeBroken {
		where = append(where, "NOT EXISTS (SELECT 1 FROM broken b WHERE b.identifier = g.identifier)")
	}
	whereSQL := ""
	if len(where) > 0 {
		whereSQL = " WHERE " + strings.Join(where, " AND ")
	}
	res := SearchResult{Games: []Game{}, Offset: q.Offset, Limit: q.Limit}
	if err := st.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+from+whereSQL, args...).Scan(&res.Total); err != nil {
		return SearchResult{}, fmt.Errorf("retrogames: count search: %w", err)
	}
	order, orderArgs := orderBy(q, match != "")
	selectArgs := append(append(append([]any{}, args...), orderArgs...), q.Limit, q.Offset)
	rows, err := st.db.QueryContext(ctx, "SELECT "+gameColumns+" FROM "+from+whereSQL+" ORDER BY "+order+" LIMIT ? OFFSET ?", selectArgs...)
	if err != nil {
		return SearchResult{}, fmt.Errorf("retrogames: search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		g, err := scanGame(rows)
		if err != nil {
			return SearchResult{}, fmt.Errorf("retrogames: scan search hit: %w", err)
		}
		res.Games = append(res.Games, g)
	}
	return res, rows.Err()
}

func orderBy(q Query, hasMatch bool) (string, []any) {
	switch q.Sort {
	case SortTitle:
		return "g.title COLLATE NOCASE, g.downloads DESC", nil
	case SortYear:
		return "g.year = 0, g.year, g.title COLLATE NOCASE", nil
	case SortPopular:
		return "g.downloads DESC, g.title COLLATE NOCASE", nil
	}
	if !hasMatch {
		return "g.downloads DESC, g.title COLLATE NOCASE", nil
	}
	exact := strings.ToLower(q.Text)
	return "lower(g.title) = ? DESC, bm25(games_fts, 10.0, 2.0, 1.0) - 2.0 * g.pop_rank, g.downloads DESC, g.title COLLATE NOCASE", []any{exact}
}

// PlatformCounts returns the number of indexed games per platform.
func (st *Store) PlatformCounts(ctx context.Context) (map[string]int, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT platform, COUNT(*) FROM games GROUP BY platform`)
	if err != nil {
		return nil, fmt.Errorf("retrogames: count platforms: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var platform string
		var n int
		if err := rows.Scan(&platform, &n); err != nil {
			return nil, fmt.Errorf("retrogames: scan platform count: %w", err)
		}
		out[platform] = n
	}
	return out, rows.Err()
}

// SystemCounts returns the systems of one platform, most games first.
func (st *Store) SystemCounts(ctx context.Context, platform string) ([]CountEntry, error) {
	rows, err := st.db.QueryContext(ctx, `SELECT system, COUNT(*) AS n FROM games WHERE platform = ? GROUP BY system ORDER BY n DESC, system`, platform)
	if err != nil {
		return nil, fmt.Errorf("retrogames: count systems: %w", err)
	}
	defer rows.Close()
	out := []CountEntry{}
	for rows.Next() {
		var e CountEntry
		if err := rows.Scan(&e.Name, &e.Count); err != nil {
			return nil, fmt.Errorf("retrogames: scan system count: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
