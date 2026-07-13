package manus

import "fmt"

// OutcomeUnknownError means a mutating request was sent but its final remote
// outcome could not be established. Retrying such a request is unsafe.
type OutcomeUnknownError struct {
	Operation string
	Err       error
}

func (e *OutcomeUnknownError) Error() string {
	if e == nil {
		return "Manus mutation outcome is unknown"
	}
	return fmt.Sprintf("Manus mutation %s outcome is unknown: %v", e.Operation, e.Err)
}

func (e *OutcomeUnknownError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// RemoteAppliedError means Manus confirmed a mutation but AuraGo could not
// persist the corresponding local ledger state.
type RemoteAppliedError struct {
	Operation string
	TaskID    string
	TaskURL   string
	Err       error
}

func (e *RemoteAppliedError) Error() string {
	if e == nil {
		return "Manus mutation was applied but local persistence failed"
	}
	return fmt.Sprintf("Manus mutation %s was applied remotely but local persistence failed: %v", e.Operation, e.Err)
}

func (e *RemoteAppliedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
