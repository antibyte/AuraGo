package memory

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

type correctionQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type correctionSnapshot struct {
	Absent   bool            `json:"absent,omitempty"`
	Value    string          `json:"value"`
	Revision string          `json:"revision,omitempty"`
	Meta     *MemoryMeta     `json:"meta,omitempty"`
	Node     *Node           `json:"node,omitempty"`
	Profile  *ProfileEntry   `json:"profile,omitempty"`
	Edge     *correctionEdge `json:"edge,omitempty"`
	Learning *Learning       `json:"learning,omitempty"`
}

func (c *MemoryCorrectionService) snapshot(ctx context.Context, q correctionQueryer, ref MemoryReference) (correctionSnapshot, error) {
	var s correctionSnapshot
	if err := c.checkAccess(ref); err != nil {
		return s, err
	}
	switch ref.Store {
	case "learning":
		l, err := readLearning(ctx, q, ref.ID)
		if err != nil {
			return s, err
		}
		if !c.LearningScope.Allows(l.Scope) || l.Protected {
			return s, fmt.Errorf("learning unavailable or protected")
		}
		s.Value, s.Revision, s.Learning = l.Statement, strconv.Itoa(l.Revision), &l
		s.Absent = l.Status == "archived" || l.Status == "superseded"
		return s, nil
	case "core":
		id, err := strconv.ParseInt(ref.ID, 10, 64)
		if err != nil || id <= 0 {
			return s, fmt.Errorf("core reference requires a numeric ID")
		}
		err = q.QueryRowContext(ctx, `SELECT fact, updated_at FROM core_memory WHERE id=?`, id).Scan(&s.Value, &s.Revision)
		if errors.Is(err, sql.ErrNoRows) {
			return correctionTombstone(ctx, q, ref)
		}
		return s, err
	case "profile":
		p := &ProfileEntry{}
		err := q.QueryRowContext(ctx, `SELECT category,key,value,confidence,source,updated_at,COALESCE(first_seen,updated_at) FROM user_profile WHERE category=? AND key=?`, ref.Category, ref.Key).Scan(&p.Category, &p.Key, &p.Value, &p.Confidence, &p.Source, &p.UpdatedAt, &p.FirstSeen)
		s.Value, s.Profile = p.Value, p
		if errors.Is(err, sql.ErrNoRows) {
			return correctionTombstone(ctx, q, ref)
		}
		return s, err
	case "ltm":
		if c.Vector == nil || !c.Vector.IsReady() || c.Vector.IsDisabled() {
			return s, fmt.Errorf("long-term memory unavailable")
		}
		meta := &MemoryMeta{}
		if err := scanMemoryMeta(q.QueryRowContext(ctx, `SELECT `+memoryMetaSelectColumns+` FROM memory_meta WHERE doc_id=?`, ref.ID), meta); err != nil {
			return s, err
		}
		if meta.Protected || meta.KeepForever {
			return s, fmt.Errorf("memory is protected or archived")
		}
		if IsMemoryArchived(*meta) {
			if _, err := correctionTombstone(ctx, q, ref); err != nil {
				return s, fmt.Errorf("memory is archived without a correction tombstone")
			}
			s.Absent = true
		}
		s.Meta = meta
		body, err := c.Vector.GetByIDFromCollection(ref.ID, "aurago_memories")
		if err != nil {
			return s, err
		}
		s.Value, s.Revision = correctionStoredFact(body, *meta), contentSHA256(body)
		return s, nil
	case "kg_node", "kg_edge":
		return c.Graph.correctionSnapshot(ctx, c.Graph.db, ref)
	}
	return s, fmt.Errorf("unsupported memory reference")
}

func correctionTombstone(ctx context.Context, q correctionQueryer, ref MemoryReference) (correctionSnapshot, error) {
	s := correctionSnapshot{Absent: true}
	err := q.QueryRowContext(ctx, `SELECT previous_value,correction_id FROM memory_correction_tombstones WHERE reference=?`, correctionReferenceKey(ref)).Scan(&s.Value, &s.Revision)
	return s, err
}

func (c *MemoryCorrectionService) validateChange(ctx context.Context, change MemoryCorrectionChange, snapshot correctionSnapshot, evidence []CorrectionEvidence) error {
	if snapshot.Value == "" && !(change.Reference.Store == "kg_node" && change.Operation == "replace") {
		return fmt.Errorf("correction target has no current value")
	}
	if snapshot.Absent && change.Operation != "replace" {
		return fmt.Errorf("fact is already archived; only a fresh evidenced replacement is allowed")
	}
	if snapshot.Absent && change.Reference.Store == "learning" {
		return fmt.Errorf("archived or superseded learning cannot be reactivated by a correction")
	}
	if snapshot.Edge != nil && snapshot.Edge.Status != string(KGClaimAccepted) && !(snapshot.Edge.Status == string(KGClaimRetracted) && change.Operation == "replace") {
		return fmt.Errorf("only active graph claims can be corrected")
	}
	if change.Operation != "replace" && change.Operation != "archive" {
		return fmt.Errorf("correction operation must be replace or archive")
	}
	if change.Operation == "archive" && change.Value != "" {
		return fmt.Errorf("archive cannot carry a replacement value")
	}
	if change.Operation == "replace" && (strings.TrimSpace(change.Value) == "" || len(change.Value) > 10000 || (change.Value == snapshot.Value && !snapshot.Absent)) {
		return fmt.Errorf("replacement must be nonempty, different and at most 10000 bytes")
	}
	userEvidence := false
	for _, e := range evidence {
		userEvidence = userEvidence || e.Kind == "user"
	}
	// Unknown natural-language facts are potentially personal. Tool-only authority
	// is restricted to existing technical provenance or typed machine properties.
	personal := true
	if snapshot.Meta != nil {
		personal = !correctionTechnicalSource(snapshot.Meta.SourceType)
	}
	if snapshot.Node != nil {
		switch snapshot.Node.Properties["type"] {
		case "device", "server", "service", "software", "container", "system", "hardware", "network":
			switch change.Reference.Field {
			case "os", "version", "cpu", "ram", "memory", "hostname", "architecture", "cores", "disk", "port", "protocol", "state", "image", "vendor", "model", "ip", "mac":
				personal = false
			}
		}
	}
	text := strings.ToLower(snapshot.Value + " " + change.Value + " " + change.Reference.Relation)
	for _, cue := range []string{"user", "prefers", "preference", "benutzer", "vorliebe", "bevorzug", "my ", "mein ", "identity", "identität", "likes", "lives_in", "speaks"} {
		personal = personal || strings.Contains(text, cue)
	}
	if personal && !userEvidence {
		return fmt.Errorf("personal facts require current user evidence")
	}
	if change.Operation == "archive" {
		return nil
	}
	switch change.Reference.Store {
	case "learning":
		if len(change.Value) > 2000 {
			return fmt.Errorf("learning replacement too long")
		}
	case "core":
		return ValidateCoreMemoryFact(change.Value)
	case "ltm":
		if _, ok := c.Vector.(correctionVectorWriter); !ok {
			return fmt.Errorf("vector backend cannot stage a correction safely")
		}
	case "profile":
		if snapshot.Absent {
			if err := checkCorrectionProfileCapacity(ctx, c.Memory.db, change.Reference.Category); err != nil {
				return err
			}
		}
		if !isValidProfileCategory(change.Reference.Category) || !isValidProfileKey(change.Reference.Key) || isRejectedProfileKey(change.Reference.Key) || normalizeProfileKey(change.Reference.Key) != change.Reference.Key || containsProfilePII(change.Value) || utf8.RuneCountInString(change.Value) > 200 {
			return fmt.Errorf("replacement violates profile policy")
		}
	case "kg_node":
		if utf8.RuneCountInString(change.Value) > knowledgeGraphPropertyValueLimit {
			return fmt.Errorf("graph property is too long")
		}
	case "kg_edge":
		if change.Value == change.Reference.Target && !snapshot.Absent {
			return fmt.Errorf("replacement edge requires a different target")
		}
		node, err := c.Graph.GetNode(change.Value)
		if err != nil {
			return err
		}
		if node == nil || node.Protected {
			return fmt.Errorf("replacement edge target must exist and be unprotected")
		}
		var count int
		if err := c.Graph.db.QueryRow(`SELECT COUNT(*) FROM kg_edges WHERE source=? AND target=? AND relation=? AND COALESCE(status,'accepted')='accepted'`, change.Reference.ID, change.Value, change.Reference.Relation).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("replacement edge already exists; resolve its claims explicitly")
		}
		replacementRef := change.Reference
		replacementRef.Target = change.Value
		if _, err := c.Graph.correctionSnapshot(ctx, c.Graph.db, replacementRef); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
	}
	return nil
}

// Preview resolves exact targets and preserves their full preconditions before any fact changes.
func (c *MemoryCorrectionService) Preview(ctx context.Context, sessionID, runID, reason string, changes []MemoryCorrectionChange, evidence []CorrectionEvidence) (*MemoryCorrection, error) {
	if c.Memory == nil {
		return nil, fmt.Errorf("memory unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	c.Memory.correctionMu.Lock()
	defer c.Memory.correctionMu.Unlock()
	if err := rejectDeletedChatSessionEvidence(ctx, c.Memory.db, sessionID); err != nil {
		return nil, err
	}
	correction, err := newCorrection(sessionID, runID, reason, evidence)
	if err != nil {
		return nil, err
	}
	if len(changes) == 0 || len(changes) > maxCorrectionItems {
		return nil, fmt.Errorf("provide between 1 and %d exact changes", maxCorrectionItems)
	}
	seen := make(map[string]MemoryCorrectionChange)
	for index := 0; index < len(changes); index++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		change := changes[index]
		if change.Reference.Store == "profile" && change.Operation == "replace" {
			change.Value = normalizeProfileValue(change.Value)
		}
		key := correctionReferenceKey(change.Reference)
		if previous, found := seen[key]; found {
			if previous.Operation != change.Operation || correctionIdentity(previous.Value) != correctionIdentity(change.Value) {
				return nil, fmt.Errorf("conflicting changes for one reference")
			}
			continue
		}
		seen[key] = change
		if len(seen) > maxCorrectionItems {
			return nil, fmt.Errorf("too many related facts; split the correction")
		}
		snapshot, err := c.snapshot(ctx, c.Memory.db, change.Reference)
		if err != nil {
			return nil, fmt.Errorf("read correction target %s: %w", key, err)
		}
		if err := c.validateChange(ctx, change, snapshot, evidence); err != nil {
			return nil, err
		}
		encoded, _ := json.Marshal(snapshot)
		item := MemoryCorrectionItem{MemoryCorrectionChange: change, Before: snapshot.Value, Snapshot: encoded, State: "pending"}
		if (change.Reference.Store == "ltm" || change.Reference.Store == "kg_edge") && change.Operation == "replace" {
			item.ReplacementID = correction.ID + "_" + strconv.Itoa(len(correction.Items))
		}
		correction.Items = append(correction.Items, item)
		copies, review, limited, err := c.relatedCopies(ctx, change, snapshot.Value)
		if err != nil {
			return nil, fmt.Errorf("check related memories: %w", err)
		}
		changes = append(changes, copies...)
		correction.Review = append(correction.Review, review...)
		correction.ReviewLimited = correction.ReviewLimited || limited
	}
	var unresolved []MemoryReference
	reviewSeen := make(map[string]bool)
	for _, ref := range correction.Review {
		key := correctionReferenceKey(ref)
		if _, ok := seen[key]; !ok && !reviewSeen[key] {
			unresolved = append(unresolved, ref)
			reviewSeen[key] = true
		}
	}
	correction.Review = unresolved
	if len(unresolved) > 0 {
		correction.Status = "needs_review"
	}
	if err := saveMemoryCorrection(ctx, c.Memory.db, correction); err != nil {
		return nil, err
	}
	return correction, nil
}

func (c *MemoryCorrectionService) relatedCopies(ctx context.Context, change MemoryCorrectionChange, old string) ([]MemoryCorrectionChange, []MemoryReference, bool, error) {
	var copies []MemoryCorrectionChange
	var review []MemoryReference
	limited := false
	add := func(ref MemoryReference, value string) {
		if correctionReferenceKey(ref) == correctionReferenceKey(change.Reference) {
			return
		}
		if correctionIdentity(value) == correctionIdentity(old) {
			// Equal values do not establish a shared subject, field or provenance.
			review = append(review, ref)
		}
	}
	facts, err := c.Memory.GetCoreMemoryFacts()
	if err != nil {
		return nil, nil, false, err
	}
	for _, fact := range facts {
		add(MemoryReference{Store: "core", ID: strconv.FormatInt(fact.ID, 10)}, fact.Fact)
	}
	profiles, err := c.Memory.GetProfileEntries("")
	if err != nil {
		return nil, nil, false, err
	}
	for _, profile := range profiles {
		add(MemoryReference{Store: "profile", Category: profile.Category, Key: profile.Key}, profile.Value)
	}
	if c.Vector != nil {
		if c.Vector.IsDisabled() || !c.Vector.IsReady() {
			return nil, nil, false, fmt.Errorf("cannot check related long-term memories while the store is unavailable")
		}
		_, ids, searchErr := c.Vector.SearchMemoriesOnly(old, maxCorrectionItems+1)
		if searchErr != nil {
			return nil, nil, false, searchErr
		}
		if len(ids) > maxCorrectionItems {
			limited = true
			ids = ids[:maxCorrectionItems]
		}
		for _, id := range ids {
			meta, err := c.Memory.GetMemoryMeta(id)
			if errors.Is(err, sql.ErrNoRows) {
				review = append(review, MemoryReference{Store: "ltm", ID: id})
				continue
			}
			if err != nil {
				return nil, nil, false, err
			}
			if IsMemoryArchived(meta) {
				continue
			}
			body, err := c.Vector.GetByIDFromCollection(id, "aurago_memories")
			if err != nil {
				return nil, nil, false, err
			}
			fact := correctionStoredFact(body, meta)
			add(MemoryReference{Store: "ltm", ID: id}, fact)
			if correctionIdentity(fact) != correctionIdentity(old) {
				review = append(review, MemoryReference{Store: "ltm", ID: id})
			}
		}
	}
	if c.Graph != nil {
		// Only the explicit core-memory ownership link authorizes automatic expansion.
		nodes, err := c.Graph.GetAllNodes(10001)
		if err != nil {
			return nil, nil, false, err
		}
		if len(nodes) > 10000 {
			limited = true
			nodes = nodes[:10000]
		}
		for _, node := range nodes {
			if err := ctx.Err(); err != nil {
				return nil, nil, false, err
			}
			if node.Properties["source"] == "core_memory" {
				ref := MemoryReference{Store: "kg_node", ID: node.ID, Field: "content"}
				if change.Reference.Store == "core" && node.ID == "core_fact_"+change.Reference.ID {
					copies = append(copies, MemoryCorrectionChange{Reference: ref, Operation: change.Operation, Value: change.Value})
				} else {
					add(ref, node.Properties["content"])
				}
				continue
			}
			add(MemoryReference{Store: "kg_node", ID: node.ID, Field: "label"}, node.Label)
			for field, value := range node.Properties {
				if correctionGraphFieldAllowed(field) {
					add(MemoryReference{Store: "kg_node", ID: node.ID, Field: field}, value)
				}
			}
		}
	}
	return copies, review, limited, nil
}
