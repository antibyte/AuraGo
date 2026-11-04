package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"aurago/internal/memory"
)

func identityOrExpiryTable(table string) bool {
	switch table {
	case "memory_document_identities", "memory_archive_identities", "memory_archive_cleanup", "memory_expiry":
		return true
	}
	return false
}

func validateIdentityRows(group mergeGroup) error {
	if len(group.Before.Rows["memory_archive_cleanup"]) > 0 {
		return fmt.Errorf("archive cleanup is pending; manual review required")
	}
	if len(group.Before.Documents) == 0 {
		return fmt.Errorf("document identity is unavailable")
	}
	doc := group.Before.Documents[0]
	concept, body, complete := memory.AnalysisDocumentParts(doc.Content)
	identity, valid := memory.AnalysisMemoryIdentity(concept, body, doc.Metadata["domain"])
	if !complete || !valid {
		return fmt.Errorf("document identity is unproven")
	}
	for _, table := range []string{"memory_document_identities", "memory_archive_identities"} {
		for _, row := range group.Before.Rows[table] {
			if !contains(group.IDs, text(row, "doc_id")) || text(row, "identity") != identity {
				return fmt.Errorf("conflicting document identity in %s", table)
			}
		}
	}
	// A historical archive blocks automatic resurrection even if its old ID is removed.
	if len(group.Before.Rows["memory_archive_identities"]) > 0 {
		var canonical record
		for _, row := range group.Before.Rows["memory_meta"] {
			if text(row, "doc_id") == group.Canonical {
				canonical = row
			}
		}
		if !archived(canonical) {
			return fmt.Errorf("archive identity conflicts with active canonical document")
		}
	}
	expiries := map[string]time.Time{}
	for _, row := range group.Before.Rows["memory_expiry"] {
		id, value := text(row, "doc_id"), text(row, "expires_at")
		at, err := time.Parse("2006-01-02 15:04:05", value)
		if err != nil || !contains(group.IDs, id) {
			return fmt.Errorf("invalid memory expiry evidence")
		}
		expiries[id] = at
	}
	for _, id := range group.IDs {
		if !expiries[id].Equal(expiries[group.Canonical]) {
			return fmt.Errorf("different memory expiry rules; manual review required")
		}
	}
	return nil
}

func transferIdentityRows(ctx context.Context, tx *sql.Tx, group mergeGroup) error {
	if rows := group.Before.Rows["memory_document_identities"]; len(rows) > 0 {
		_, err := tx.ExecContext(ctx, `INSERT INTO memory_document_identities(doc_id,identity) VALUES(?,?) ON CONFLICT(doc_id) DO NOTHING`, group.Canonical, text(rows[0], "identity"))
		return err
	}
	return nil
}
