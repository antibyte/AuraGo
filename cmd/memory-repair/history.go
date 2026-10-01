package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
)

func sameRecords(left, right []record) bool {
	if len(left) == 0 && len(right) == 0 {
		return true
	}
	return fingerprint(left) == fingerprint(right)
}
func curationFields(row record) record {
	result := record{}
	for key, value := range row {
		switch key {
		case "access_count", "last_accessed", "last_event_at", "useful_count", "useless_count", "last_effectiveness_at":
			continue
		}
		result[key] = value
	}
	return result
}

func verifiedCuration(group mergeGroup, meta record) (string, bool, error) {
	id := text(meta, "doc_id")
	var events, conflicts []record
	for _, row := range group.Before.Rows["memory_curation_events"] {
		if text(row, "doc_id") == id && number(row, "dry_run") == 0 {
			events = append(events, row)
		}
	}
	for _, row := range group.Before.Rows["memory_conflicts"] {
		if text(row, "doc_id_left") == id || text(row, "doc_id_right") == id {
			conflicts = append(conflicts, row)
		}
	}
	sort.Slice(events, func(i, j int) bool { return number(events[i], "id") < number(events[j], "id") })
	sort.Slice(conflicts, func(i, j int) bool { return number(conflicts[i], "id") < number(conflicts[j], "id") })
	state := meta
	latest := true
	for len(events) > 0 && text(events[len(events)-1], "action") == "metadata_repair" {
		event := events[len(events)-1]
		events = events[:len(events)-1]
		var audit struct {
			Version      int      `json:"version"`
			Before       record   `json:"before"`
			After        record   `json:"after"`
			Changes      record   `json:"changes"`
			Remaining    []string `json:"remaining"`
			Evidence     string   `json:"evidence"`
			EvidenceHash string   `json:"evidence_hash"`
		}
		var proof struct {
			Events    []record `json:"events"`
			Conflicts []record `json:"conflicts"`
		}
		auditErr := json.Unmarshal([]byte(text(event, "reason")), &audit)
		proofErr := json.Unmarshal([]byte(audit.Evidence), &proof)
		hash := sha256.Sum256([]byte(audit.Evidence))
		if auditErr != nil || proofErr != nil || hex.EncodeToString(hash[:]) != audit.EvidenceHash || audit.Version != 2 || text(event, "actor") != "operator_repair" || len(audit.Changes) == 0 ||
			text(audit.Before, "doc_id") != id || text(audit.After, "last_event_at") != text(event, "timestamp") ||
			text(audit.Before, "verification_status") != text(event, "previous_status") || text(audit.After, "verification_status") != text(event, "new_status") ||
			fingerprint(curationFields(state)) != fingerprint(curationFields(audit.After)) ||
			!sameRecords(events, proof.Events) || !sameRecords(conflicts, proof.Conflicts) {
			return "", false, fmt.Errorf("metadata repair evidence chain is unproven")
		}
		if state == nil || (latest && len(audit.Remaining) > 0) {
			return "", false, fmt.Errorf("metadata repair remains incomplete")
		}
		latest = false
		for key := range audit.Changes {
			switch key {
			case "verification_status", "archived_at", "archived_reason", "source_type", "source_reliability", "extraction_confidence":
			default:
				return "", false, fmt.Errorf("metadata repair changed an unsupported field")
			}
		}
		if len(audit.Before) != len(audit.After) {
			return "", false, fmt.Errorf("metadata repair fields are incomplete")
		}
		for key, before := range audit.Before {
			if key == "last_event_at" {
				continue
			}
			after := before
			if value, changed := audit.Changes[key]; changed {
				after = value
			}
			if fingerprint(after) != fingerprint(audit.After[key]) {
				return "", false, fmt.Errorf("metadata repair transition is unproven")
			}
		}
		state = audit.Before
	}
	var human bool
	var lastStatus string
	for _, event := range events {
		if text(event, "action") == "metadata_repair" {
			return "", false, fmt.Errorf("later curation requires review of the repair chain")
		}
		human = human || humanEvent(event)
		if text(event, "previous_status") != text(event, "new_status") {
			lastStatus = text(event, "new_status")
		}
	}
	if lastStatus != "" && lastStatus != text(meta, "verification_status") {
		return "", false, fmt.Errorf("curation status needs metadata repair")
	}
	reviewed := text(meta, "last_reviewed_at")
	if reviewed == "" {
		if human {
			return "", false, fmt.Errorf("human review timestamp is missing")
		}
		return "", false, nil
	}
	if len(events) == 0 || reviewed != text(events[len(events)-1], "timestamp") {
		return "", false, fmt.Errorf("curation history is incomplete")
	}
	at, err := parseTime(reviewed)
	if err != nil {
		return "", false, err
	}
	return at.UTC().Format("2006-01-02T15:04:05.000000000Z"), human, nil
}
