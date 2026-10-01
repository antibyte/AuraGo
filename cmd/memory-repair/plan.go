package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"aurago/internal/memory"
	chromem "github.com/philippgille/chromem-go"
)

type record map[string]any
type snapshot struct {
	Documents []chromem.Document  `json:"documents"`
	Rows      map[string][]record `json:"rows"`
}
type mergeGroup struct {
	Canonical string   `json:"canonical"`
	IDs       []string `json:"ids"`
	Before    snapshot `json:"before"`
	Hash      string   `json:"hash"`
}
type reviewItem struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}
type mergePlan struct {
	Version    int          `json:"version"`
	Database   string       `json:"database"`
	Vectors    string       `json:"vectors"`
	InstallDir string       `json:"install_dir"`
	Groups     []mergeGroup `json:"groups"`
	Review     []reviewItem `json:"review_required"`
}
type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func fingerprint(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:])
}
func text(row record, key string) string { value, _ := row[key].(string); return value }
func number(row record, key string) int64 {
	switch n := row[key].(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case json.Number:
		value, _ := n.Int64()
		return value
	}
	return 0
}
func rowsFor(ctx context.Context, db queryer, query string, args ...any) ([]record, error) {
	// CAST datetime columns to avoid the driver's implicit time.Time conversion;
	// repair evidence must retain the exact on-disk text, including its format.
	if rest, star := strings.CutPrefix(query, "SELECT * FROM "); star {
		table, tail, _ := strings.Cut(rest, " ")
		_, known := referenceColumns[table]
		if known || table == "episodic_memories" {
			schema, err := rowsFor(ctx, db, `PRAGMA table_info(`+table+`)`)
			if err != nil {
				return nil, err
			}
			var fields []string
			for _, column := range schema {
				name := `"` + strings.ReplaceAll(text(column, "name"), `"`, `""`) + `"`
				if strings.Contains(strings.ToUpper(text(column, "type")), "DATE") {
					fields = append(fields, `CAST(`+name+` AS TEXT) AS `+name)
				} else {
					fields = append(fields, name)
				}
			}
			query = `SELECT ` + strings.Join(fields, ",") + ` FROM ` + table + " " + tail
		}
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var result []record
	for rows.Next() {
		values := make([]any, len(columns))
		pointers := make([]any, len(columns))
		for i := range values {
			pointers[i] = &values[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return nil, err
		}
		row := record{}
		for i, key := range columns {
			row[key] = values[i]
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Slice(result, func(i, j int) bool { return fingerprint(result[i]) < fingerprint(result[j]) })
	return result, nil
}

var referenceColumns = map[string][]string{
	"memory_meta": {"doc_id"}, "memory_usage_log": {"memory_id"},
	"memory_curation_events": {"doc_id"}, "memory_extraction_sources": {"doc_id"},
	"memory_conflicts":    {"doc_id_left", "doc_id_right", "winning_doc_id", "superseded_doc_id"},
	"file_embedding_docs": {"doc_id"}, "memory_maintenance_failures": {"target_id"},
}

const journalPrefix = "memory_analysis_merge."

func sqlSnapshot(ctx context.Context, db queryer, ids []string) (map[string][]record, error) {
	result := map[string][]record{}
	tables, err := rowsFor(ctx, db, `SELECT name FROM sqlite_master WHERE type='table' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	exists := map[string]bool{}
	for _, table := range tables {
		exists[text(table, "name")] = true
	}
	for table, columns := range referenceColumns {
		if !exists[table] {
			if table == "memory_extraction_sources" {
				return nil, fmt.Errorf("source migration must be completed by the upgraded agent first")
			}
			continue
		}
		var clauses []string
		var args []any
		for _, column := range columns {
			placeholders := make([]string, len(ids))
			for i, id := range ids {
				placeholders[i] = "?"
				args = append(args, id)
			}
			clauses = append(clauses, column+" IN ("+strings.Join(placeholders, ",")+")")
		}
		result[table], err = rowsFor(ctx, db, `SELECT * FROM `+table+` WHERE `+strings.Join(clauses, " OR "), args...)
		if err != nil {
			return nil, fmt.Errorf("read %s evidence: %w", table, err)
		}
	}
	if exists["episodic_memories"] {
		all, err := rowsFor(ctx, db, `SELECT * FROM episodic_memories`)
		if err != nil {
			return nil, err
		}
		for _, row := range all {
			var refs []string
			if err := json.Unmarshal([]byte(text(row, "related_doc_ids")), &refs); err != nil {
				return nil, fmt.Errorf("unparseable episodic document references: %w", err)
			}
			for _, ref := range refs {
				if contains(ids, ref) {
					result["episodic_memories"] = append(result["episodic_memories"], row)
					break
				}
			}
		}
	}
	states, err := rowsFor(ctx, db, `SELECT * FROM memory_maintenance_meta`)
	if err != nil {
		return nil, err
	}
	for _, row := range states {
		if strings.HasPrefix(text(row, "key"), journalPrefix) {
			continue
		}
		for _, id := range ids {
			if strings.Contains(text(row, "value"), id) {
				result["memory_maintenance_meta"] = append(result["memory_maintenance_meta"], row)
				break
			}
		}
	}
	// Unknown document-reference columns cannot silently lose references.
	for _, table := range tables {
		name := text(table, "name")
		if _, known := referenceColumns[name]; known || name == "episodic_memories" || name == "memory_maintenance_meta" {
			continue
		}
		columns, err := rowsFor(ctx, db, `PRAGMA table_info("`+strings.ReplaceAll(name, `"`, `""`)+`")`)
		if err != nil {
			return nil, err
		}
		for _, column := range columns {
			key := text(column, "name")
			if !strings.Contains(key, "doc_id") && key != "memory_id" && key != "target_id" {
				continue
			}
			for _, id := range ids {
				found, err := rowsFor(ctx, db, `SELECT * FROM "`+strings.ReplaceAll(name, `"`, `""`)+`" WHERE instr("`+strings.ReplaceAll(key, `"`, `""`)+`",?)>0`, id)
				if err != nil {
					return nil, err
				}
				if len(found) > 0 {
					return nil, fmt.Errorf("unknown reference in %s.%s", name, key)
				}
			}
		}
	}
	return result, nil
}
func contains(ids []string, id string) bool {
	for _, candidate := range ids {
		if id == candidate {
			return true
		}
	}
	return false
}
func mapped(group mergeGroup, id string) string {
	if contains(group.IDs, id) {
		return group.Canonical
	}
	return id
}
func archived(row record) bool {
	return text(row, "verification_status") == "archived" || text(row, "archived_at") != ""
}
func humanEvent(row record) bool {
	switch text(row, "actor") {
	case "system", "auto_curation", "nightly_memory_budget", "maintenance", "memory_analysis", "operator_repair":
		return false
	}
	return number(row, "dry_run") == 0
}

func validateMetadata(row record) error {
	for _, key := range []string{"doc_id", "verification_status", "source_type", "archived_reason", "review_note"} {
		if _, ok := row[key].(string); !ok {
			return fmt.Errorf("metadata field %s is incomplete", key)
		}
	}
	if text(row, "doc_id") == "" || text(row, "source_type") == "" {
		return fmt.Errorf("metadata identity or provenance is missing")
	}
	switch text(row, "verification_status") {
	case "unverified", "confirmed", "contradicted", "archived":
	default:
		return fmt.Errorf("unknown verification status")
	}
	for _, key := range []string{"access_count", "useful_count", "useless_count", "protected", "keep_forever"} {
		var value int64
		var err error
		switch n := row[key].(type) {
		case int64:
			value = n
		case json.Number:
			value, err = n.Int64()
		default:
			return fmt.Errorf("metadata field %s is not an integer", key)
		}
		if err != nil || value < 0 || ((key == "protected" || key == "keep_forever") && value > 1) {
			return fmt.Errorf("invalid metadata field %s", key)
		}
	}
	for _, key := range []string{"extraction_confidence", "source_reliability"} {
		var value float64
		var err error
		switch n := row[key].(type) {
		case float64:
			value = n
		case int64:
			value = float64(n)
		case json.Number:
			value, err = n.Float64()
		default:
			return fmt.Errorf("metadata field %s is not numeric", key)
		}
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 || value > 1 {
			return fmt.Errorf("invalid metadata field %s", key)
		}
	}
	for _, key := range []string{"last_accessed", "last_event_at", "last_effectiveness_at", "archived_at", "last_reviewed_at"} {
		if row[key] == nil && key != "last_accessed" && key != "last_event_at" {
			continue
		}
		if _, err := parseTime(text(row, key)); err != nil {
			return fmt.Errorf("metadata field %s: %w", key, err)
		}
	}
	if text(row, "verification_status") == "archived" && text(row, "archived_at") == "" {
		return fmt.Errorf("archive metadata repair is pending")
	}
	return nil
}

func selectCanonical(group mergeGroup) (string, error) {
	metas := group.Before.Rows["memory_meta"]
	if len(metas) != len(group.IDs) {
		return "", fmt.Errorf("incomplete metadata")
	}
	human := map[string]bool{}
	lastReview := map[string]string{}
	for _, meta := range metas {
		id := text(meta, "doc_id")
		if !contains(group.IDs, id) {
			return "", fmt.Errorf("metadata identity does not match vectors")
		}
		if err := validateMetadata(meta); err != nil {
			return "", err
		}
		reviewed, curated, err := verifiedCuration(group, meta)
		if err != nil {
			return "", err
		}
		lastReview[id], human[id] = reviewed, curated
	}
	var protected []string
	var effectiveArchive bool
	var humanStatus string
	for _, meta := range metas {
		id := text(meta, "doc_id")
		if number(meta, "protected") != 0 || number(meta, "keep_forever") != 0 {
			protected = append(protected, id)
		}
		effectiveArchive = effectiveArchive || archived(meta)
		if archived(meta) && text(meta, "verification_status") != "archived" {
			return "", fmt.Errorf("archive metadata repair is pending")
		}
		if text(meta, "last_reviewed_at") != "" && lastReview[id] == "" && human[id] {
			return "", fmt.Errorf("curation history is incomplete")
		}
		if human[id] {
			if text(meta, "source_type") == "memory_analysis" {
				return "", fmt.Errorf("curated provenance still needs metadata repair")
			}
			status := text(meta, "verification_status")
			if humanStatus != "" && humanStatus != status {
				return "", fmt.Errorf("human curation disagrees")
			}
			humanStatus = status
		}
	}
	if len(protected) > 1 {
		return "", fmt.Errorf("multiple protected or permanent IDs")
	}
	if effectiveArchive && humanStatus != "" && humanStatus != "archived" {
		return "", fmt.Errorf("archive conflicts with human curation")
	}
	if len(protected) == 1 {
		for _, meta := range metas {
			if text(meta, "doc_id") == protected[0] && effectiveArchive && !archived(meta) {
				return "", fmt.Errorf("protected canonical ID conflicts with an effective archive")
			}
			if human[text(meta, "doc_id")] && (!human[protected[0]] || lastReview[text(meta, "doc_id")] > lastReview[protected[0]]) {
				return "", fmt.Errorf("protected canonical ID cannot preserve the latest human curation")
			}
		}
		return protected[0], nil
	}
	var candidates []record
	for _, meta := range metas {
		if !effectiveArchive || archived(meta) {
			candidates = append(candidates, meta)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := text(candidates[i], "doc_id"), text(candidates[j], "doc_id")
		if human[left] != human[right] {
			return human[left]
		}
		if lastReview[left] != lastReview[right] {
			return lastReview[left] > lastReview[right]
		}
		return left < right
	})
	return text(candidates[0], "doc_id"), nil
}

func validateReferences(group mergeGroup) error {
	if _, err := mergeActivity(group); err != nil {
		return err
	}
	if _, err := sourceRows(group); err != nil {
		return err
	}
	if len(group.Before.Rows["file_embedding_docs"]) > 0 {
		return fmt.Errorf("indexing references outside the analysis store")
	}
	keys := map[string]bool{}
	for _, row := range group.Before.Rows["memory_conflicts"] {
		left, right := mapped(group, text(row, "doc_id_left")), mapped(group, text(row, "doc_id_right"))
		if left == right {
			return fmt.Errorf("conflict history would collapse onto one ID")
		}
		if left > right {
			left, right = right, left
		}
		key := fingerprint([]string{left, right, text(row, "conflict_key")})
		if keys[key] {
			return fmt.Errorf("conflict histories collide")
		}
		keys[key] = true
		for _, column := range []string{"winning_doc_id", "superseded_doc_id"} {
			id := text(row, column)
			if id != "" && id != text(row, "doc_id_left") && id != text(row, "doc_id_right") {
				return fmt.Errorf("conflict outcome has an unclear document reference")
			}
		}
	}
	for _, row := range group.Before.Rows["memory_maintenance_failures"] {
		if action := text(row, "action"); action != "memory_conflict_scan" && action != "canonical_repair" {
			return fmt.Errorf("unknown maintenance reference %s", action)
		}
		switch row["failure_count"].(type) {
		case int64, json.Number:
		default:
			return fmt.Errorf("maintenance failure counter is incomplete")
		}
		if number(row, "failure_count") < 0 {
			return fmt.Errorf("maintenance failure counter is invalid")
		}
		for _, key := range []string{"first_failed_at", "last_failed_at"} {
			if _, err := parseTime(text(row, key)); err != nil {
				return err
			}
		}
	}
	for _, row := range group.Before.Rows["memory_maintenance_meta"] {
		if text(row, "key") != "memory_conflict_scan.cursor" && text(row, "key") != "canonical_repair.v1.cursor" {
			return fmt.Errorf("unknown maintenance state reference %s", text(row, "key"))
		}
		if !contains(group.IDs, text(row, "value")) {
			return fmt.Errorf("ambiguous maintenance cursor")
		}
	}
	return nil
}

func buildPlan(ctx context.Context, db *sql.DB, vectors *chromem.Collection) (mergePlan, error) {
	plan := mergePlan{Version: 1}
	states, err := rowsFor(ctx, db, `SELECT key,value FROM memory_maintenance_meta WHERE key LIKE ?`, journalPrefix+"%")
	if err != nil {
		return plan, err
	}
	for _, row := range states {
		var state mergeJournal
		if err := json.Unmarshal([]byte(text(row, "value")), &state); err != nil || state.State != "done" {
			plan.Review = append(plan.Review, reviewItem{Reason: "unfinished merge journal; resume its original saved plan: " + text(row, "key")})
		}
	}
	metas, err := rowsFor(ctx, db, `SELECT * FROM memory_meta`)
	if err != nil {
		return plan, err
	}
	groups := map[string][]chromem.Document{}
	for _, meta := range metas {
		id := text(meta, "doc_id")
		doc, err := vectors.GetByID(ctx, id)
		if err != nil {
			plan.Review = append(plan.Review, reviewItem{[]string{id}, "vector content is unavailable"})
			continue
		}
		concept, body, complete := memory.AnalysisDocumentParts(doc.Content)
		if !complete {
			if text(meta, "source_type") == "memory_analysis" {
				plan.Review = append(plan.Review, reviewItem{[]string{id}, "analysis envelope is incomplete or unknown"})
			}
			continue
		}
		if doc.Metadata["chunk_index"] != "" || len(doc.Content) > 4000 {
			plan.Review = append(plan.Review, reviewItem{[]string{id}, "chunked or incomplete analysis document"})
			continue
		}
		identity, _ := memory.AnalysisMemoryIdentity(concept, body, doc.Metadata["domain"])
		groups[identity] = append(groups[identity], doc)
	}
	var identities []string
	for identity := range groups {
		identities = append(identities, identity)
	}
	sort.Strings(identities)
	for _, identity := range identities {
		docs := groups[identity]
		if len(docs) < 2 {
			continue
		}
		sort.Slice(docs, func(i, j int) bool { return docs[i].ID < docs[j].ID })
		group := mergeGroup{Before: snapshot{Documents: docs}}
		for _, doc := range docs {
			group.IDs = append(group.IDs, doc.ID)
		}
		group.Before.Rows, err = sqlSnapshot(ctx, db, group.IDs)
		if err == nil {
			group.Canonical, err = selectCanonical(group)
		}
		if err == nil {
			err = validateReferences(group)
		}
		if err != nil {
			plan.Review = append(plan.Review, reviewItem{group.IDs, err.Error()})
			continue
		}
		group.Hash = fingerprint(group.Before)
		if group.Hash == "" {
			plan.Review = append(plan.Review, reviewItem{group.IDs, "evidence cannot be serialized"})
			continue
		}
		plan.Groups = append(plan.Groups, group)
	}
	return plan, nil
}

func parseTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid activity timestamp %q", value)
}
