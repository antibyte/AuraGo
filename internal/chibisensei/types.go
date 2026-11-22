package chibisensei

import (
	"errors"
	"strings"
)

// Message is one chat message of a row.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// Steps is the state of the five project steps.
type Steps struct {
	Goal     string `json:"goal"`
	Data     string `json:"data"`
	Training string `json:"training"`
	Test     string `json:"test"`
	Use      string `json:"use"`
}

// ProjectSummary is the list form of a project.
type ProjectSummary struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Recipe    string `json:"recipe"`
	TaskKind  string `json:"task_kind"`
	ProfileID string `json:"profile_id"`
	Rows      int    `json:"rows"`
	DataRev   int64  `json:"data_rev"`
	Rev       int64  `json:"rev"`
	Updated   string `json:"updated"`
	Archived  bool   `json:"archived"`
	Steps     Steps  `json:"steps"`
}

// Overrides are the user's training hyper-parameter overrides.
type Overrides struct {
	Method       string  `json:"method,omitempty"`
	Epochs       int     `json:"epochs,omitempty"`
	LearningRate float64 `json:"learning_rate,omitempty"`
	MaxSeqLen    int     `json:"max_seq_len,omitempty"`
	LoRAR        int     `json:"lora_r,omitempty"`
	LoRAAlpha    int     `json:"lora_alpha,omitempty"`
}

// Suggestion is a profile suggestion with a translatable reason.
type Suggestion struct {
	ID        string `json:"id"`
	ReasonKey string `json:"reason_key"`
}

// Project is the full project object.
type Project struct {
	ProjectSummary
	Labels           []string    `json:"labels"`
	GoalText         string      `json:"goal_text"`
	Preset           string      `json:"preset"`
	Overrides        Overrides   `json:"overrides"`
	SplitSeed        int64       `json:"split_seed"`
	Created          string      `json:"created"`
	SuggestedProfile *Suggestion `json:"suggested_profile"` // filled by the HTTP layer
}

// ProjectInput is the body of project creation.
type ProjectInput struct {
	Name      string   `json:"name"`
	Recipe    string   `json:"recipe"`
	TaskKind  string   `json:"task_kind"`
	Labels    []string `json:"labels"`
	GoalText  string   `json:"goal_text"`
	ProfileID string   `json:"profile_id"`
}

// ProjectPatch is the body of a project update; nil fields stay unchanged.
type ProjectPatch struct {
	Name      *string    `json:"name"`
	Recipe    *string    `json:"recipe"`
	TaskKind  *string    `json:"task_kind"`
	GoalText  *string    `json:"goal_text"`
	ProfileID *string    `json:"profile_id"`
	Preset    *string    `json:"preset"`
	Labels    *[]string  `json:"labels"`
	Overrides *Overrides `json:"overrides"`
}

// Row is one dataset row.
type Row struct {
	ID          int64     `json:"id"`
	Seq         int64     `json:"seq"`
	Kind        string    `json:"kind"`
	Messages    []Message `json:"messages"`
	Text        *string   `json:"text"`
	Label       string    `json:"label"`
	Split       string    `json:"split"`
	Source      string    `json:"source"`
	SourceRef   string    `json:"source_ref"`
	Tags        []string  `json:"tags"`
	EstTokens   int       `json:"est_tokens"`
	Flags       []string  `json:"flags"`
	Rev         int64     `json:"rev"`
	BatchID     string    `json:"-"`
	ContentHash string    `json:"-"`
	PromptHash  string    `json:"-"`
}

// RowInput is one row to add or to replace.
type RowInput struct {
	Messages  []Message `json:"messages,omitempty"`
	Text      *string   `json:"text,omitempty"`
	Split     string    `json:"split,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	SourceRef string    `json:"-"`
}

// RowFilter selects rows; all set fields must match.
type RowFilter struct {
	Q      string `json:"q"`
	Split  string `json:"split"`
	Source string `json:"source"`
	Flag   string `json:"flag"`
	Label  string `json:"label"`
	Batch  string `json:"batch"`
}

// Select addresses rows by ids, by filter, or both (intersection).
type Select struct {
	IDs    []int64    `json:"ids,omitempty"`
	Filter *RowFilter `json:"filter,omitempty"`
}

// RowQuery is a filtered, sorted page request. Sort is seq|est_tokens, Dir asc|desc.
type RowQuery struct {
	RowFilter
	Offset, Limit int
	Sort, Dir     string
}

// RowPage is one page of rows.
type RowPage struct {
	Rows    []Row `json:"rows"`
	Total   int   `json:"total"`
	Offset  int   `json:"offset"`
	Limit   int   `json:"limit"`
	DataRev int64 `json:"data_rev"`
}

// Batch is one history entry.
type Batch struct {
	BatchID string `json:"batch_id"`
	Kind    string `json:"kind"`
	Rows    int    `json:"rows"`
	Created string `json:"created"`
}

// Issue is one validation finding the API reports with a 422. B1 only needs this type for the
// server error writer; B2 extends this file with the remaining API types.
type Issue struct {
	Field      string `json:"field,omitempty"`
	Line       int    `json:"line,omitempty"`
	MessageKey string `json:"message_key"`
	Message    string `json:"message"`
}

// ValidationError carries the issues behind a 422.
type ValidationError struct{ Issues []Issue }

func (e *ValidationError) Error() string {
	msg := ""
	if len(e.Issues) > 0 {
		msg = e.Issues[0].Message
	}
	return "chibisensei: validation failed: " + msg
}

var (
	ErrNotFound      = errors.New("chibisensei: not found")
	ErrConflict      = errors.New("chibisensei: revision conflict")
	ErrDatasetFull   = errors.New("chibisensei: dataset full")
	ErrNotArchived   = errors.New("chibisensei: project is not archived")
	ErrAlreadyUndone = errors.New("chibisensei: batch already undone")
	ErrJobActive     = errors.New("chibisensei: a job of this kind is active")
	ErrDataChanged   = errors.New("chibisensei: data changed during the computation")
	ErrUndoConflict  = errors.New("chibisensei: a later change touched the rows of this batch")
)

// Flag is a row flag; flags are stored as a bit mask in dataset_rows.flags.
type Flag int64

const (
	FlagStructure Flag = 1 << iota
	FlagShortAnswer
	FlagDuplicate
	FlagConflict
	FlagTooLong
	FlagInvalidJSON
	FlagRareLabel
)

var flagNames = [...]string{"structure", "short_answer", "duplicate", "conflict", "too_long", "invalid_json", "rare_label"}

var checkFlags = map[string]Flag{"structure": FlagStructure, "short_answer": FlagShortAnswer, "duplicates": FlagDuplicate,
	"conflicts": FlagConflict, "too_long": FlagTooLong, "json_valid": FlagInvalidJSON, "label_balance": FlagRareLabel}

// rowLocalFlags are set on every write; the others only by RunChecks.
const rowLocalFlags = FlagStructure | FlagShortAnswer | FlagInvalidJSON

// Names lists the set bits in flagNames order; never nil.
func (f Flag) Names() []string {
	out := []string{}
	for i, n := range flagNames {
		if f&(Flag(1)<<uint(i)) != 0 {
			out = append(out, n)
		}
	}
	return out
}

// ParseFlag accepts a flag name or a check ID.
func ParseFlag(s string) (Flag, bool) {
	s = strings.TrimSpace(s)
	for i, n := range flagNames {
		if n == s {
			return Flag(1) << uint(i), true
		}
	}
	f, ok := checkFlags[s]
	return f, ok
}
