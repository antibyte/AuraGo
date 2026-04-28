package audit

import (
	"errors"
	"fmt"
)

var (
	ErrExternalDataTemporary    = errors.New("external data temporary failure")
	ErrExternalDataRevokedToken = errors.New("external data revoked token")
)

type ExternalSyncStatus string

const (
	ExternalSyncComplete  ExternalSyncStatus = "complete"
	ExternalSyncPartial   ExternalSyncStatus = "partial"
	ExternalSyncRevoked   ExternalSyncStatus = "revoked"
	ExternalSyncOversized ExternalSyncStatus = "oversized"
	ExternalSyncError     ExternalSyncStatus = "error"
)

type ExternalDataItem[T any] struct {
	ID    string
	Value T
}

type ExternalDataPage[T any] struct {
	Items      []ExternalDataItem[T]
	NextCursor string
}

type ExternalDataSyncContract[T any] struct {
	MaxItems  int
	SeenIDs   map[string]bool
	FetchPage func(cursor string) (ExternalDataPage[T], error)
	Upsert    func(ExternalDataItem[T]) error
}

type ExternalDataSyncStatus struct {
	Status            ExternalSyncStatus
	Message           string
	Committed         int
	DuplicatesSkipped int
	NextCursor        string
	Retryable         bool
}

func RunExternalDataSyncContract[T any](contract ExternalDataSyncContract[T]) ExternalDataSyncStatus {
	if contract.FetchPage == nil {
		return ExternalDataSyncStatus{Status: ExternalSyncError, Message: "fetch page callback is required"}
	}
	if contract.Upsert == nil {
		return ExternalDataSyncStatus{Status: ExternalSyncError, Message: "upsert callback is required"}
	}
	seen := make(map[string]bool, len(contract.SeenIDs))
	for id, ok := range contract.SeenIDs {
		seen[id] = ok
	}

	status := ExternalDataSyncStatus{Status: ExternalSyncComplete}
	cursor := ""
	for {
		page, err := contract.FetchPage(cursor)
		if err != nil {
			status.NextCursor = cursor
			switch {
			case errors.Is(err, ErrExternalDataRevokedToken):
				status.Status = ExternalSyncRevoked
				status.Message = "external data credential was revoked"
				status.Retryable = false
			case errors.Is(err, ErrExternalDataTemporary):
				status.Status = ExternalSyncPartial
				status.Message = "external data sync stopped after a retryable page failure"
				status.Retryable = true
			default:
				status.Status = ExternalSyncError
				status.Message = err.Error()
				status.Retryable = false
			}
			return status
		}

		for _, item := range page.Items {
			if item.ID == "" {
				status.Status = ExternalSyncError
				status.Message = "external data item id is required"
				status.NextCursor = cursor
				return status
			}
			if seen[item.ID] {
				status.DuplicatesSkipped++
				continue
			}
			if contract.MaxItems > 0 && status.Committed >= contract.MaxItems {
				status.Status = ExternalSyncOversized
				status.Message = fmt.Sprintf("external data sync exceeded max_items=%d", contract.MaxItems)
				status.NextCursor = cursor
				return status
			}
			if err := contract.Upsert(item); err != nil {
				status.Status = ExternalSyncPartial
				status.Message = err.Error()
				status.NextCursor = cursor
				status.Retryable = true
				return status
			}
			seen[item.ID] = true
			status.Committed++
		}

		if page.NextCursor == "" {
			status.NextCursor = ""
			return status
		}
		cursor = page.NextCursor
	}
}

type ExternalDataSyncBoundary struct {
	Name         string
	Scenario     string
	TestCoverage string
}

func ExternalDataSyncContractManifest() []ExternalDataSyncBoundary {
	return []ExternalDataSyncBoundary{
		{
			Name:         "page-one-success-page-two-failure",
			Scenario:     "page 1 commits, page 2 fails retryably, and the status carries the failed cursor for resume",
			TestCoverage: "internal/audit/external_data_sync_contract.go and audit_manifests_test.go",
		},
		{
			Name:         "retry-skips-duplicates",
			Scenario:     "retrying from a partial sync skips already committed IDs and only upserts new items",
			TestCoverage: "internal/audit/audit_manifests_test.go",
		},
		{
			Name:         "revoked-token-stops-without-commit",
			Scenario:     "revoked credentials stop the sync as non-retryable and do not call upsert",
			TestCoverage: "internal/audit/audit_manifests_test.go",
		},
		{
			Name:         "oversized-result-stops-with-status",
			Scenario:     "oversized result sets stop at max_items with an explicit user-visible status",
			TestCoverage: "internal/audit/audit_manifests_test.go",
		},
	}
}
