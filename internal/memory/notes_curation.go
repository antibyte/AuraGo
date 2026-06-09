package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	defaultNotesCurationMaxActions       = 100
	defaultNotesStaleOpenAge             = 90 * 24 * time.Hour
	defaultNotesOverdueStaleAge          = 30 * 24 * time.Hour
	defaultNotesOverdueInactivityAge     = 30 * 24 * time.Hour
	noteCurationActionArchive            = "archive"
	noteCurationActionReview             = "review"
	noteCurationReasonStaleOpen          = "stale open low-priority note"
	noteCurationReasonOverdueLowTouch    = "overdue low-priority note"
	noteCurationReasonHighPriorityReview = "high-priority stale note requires manual review"
)

type NotesCurationOptions struct {
	Now        time.Time
	MaxActions int
}

type NoteCurationAction struct {
	NoteID          int64  `json:"note_id"`
	Action          string `json:"action"`
	Reason          string `json:"reason"`
	Title           string `json:"title"`
	Category        string `json:"category"`
	Priority        int    `json:"priority"`
	DaysSinceUpdate int    `json:"days_since_update"`
	DaysOverdue     int    `json:"days_overdue"`
	CurrentDone     bool   `json:"current_done"`
	CurrentArchived bool   `json:"current_archived"`
}

type NotesCurationPlan struct {
	GeneratedAt         string               `json:"generated_at"`
	AutoArchive         []NoteCurationAction `json:"auto_archive"`
	ReviewRequired      []NoteCurationAction `json:"review_required"`
	AutoArchiveCount    int                  `json:"auto_archive_count"`
	ReviewRequiredCount int                  `json:"review_required_count"`
}

func (s *SQLiteMemory) BuildNotesCurationPlan(opts NotesCurationOptions) (NotesCurationPlan, error) {
	if s == nil {
		return NotesCurationPlan{}, nil
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	maxActions := opts.MaxActions
	if maxActions <= 0 {
		maxActions = defaultNotesCurationMaxActions
	}

	notes, err := s.ListNotesWithOptions(NotesListOptions{DoneFilter: 0})
	if err != nil {
		return NotesCurationPlan{}, err
	}
	plan := NotesCurationPlan{GeneratedAt: now.Format(time.RFC3339)}
	for _, note := range notes {
		if note.Done || note.Archived {
			continue
		}
		updatedAt := parseMemoryMetaTime(note.UpdatedAt)
		if updatedAt.IsZero() {
			updatedAt = parseMemoryMetaTime(note.CreatedAt)
		}
		daysSinceUpdate := 0
		if !updatedAt.IsZero() {
			daysSinceUpdate = int(now.Sub(updatedAt).Hours() / 24)
			if daysSinceUpdate < 0 {
				daysSinceUpdate = 0
			}
		}
		daysOverdue := noteDaysOverdue(note.DueDate, now)
		action := NoteCurationAction{
			NoteID:          note.ID,
			Title:           note.Title,
			Category:        note.Category,
			Priority:        note.Priority,
			DaysSinceUpdate: daysSinceUpdate,
			DaysOverdue:     daysOverdue,
			CurrentDone:     note.Done,
			CurrentArchived: note.Archived,
		}

		staleOpen := note.DueDate == "" && time.Duration(daysSinceUpdate)*24*time.Hour >= defaultNotesStaleOpenAge
		staleOverdue := daysOverdue >= int(defaultNotesOverdueStaleAge.Hours()/24) &&
			time.Duration(daysSinceUpdate)*24*time.Hour >= defaultNotesOverdueInactivityAge
		if note.Priority <= 2 && (staleOpen || staleOverdue) {
			action.Action = noteCurationActionArchive
			if staleOverdue {
				action.Reason = noteCurationReasonOverdueLowTouch
			} else {
				action.Reason = noteCurationReasonStaleOpen
			}
			plan.AutoArchive = append(plan.AutoArchive, action)
			continue
		}
		if note.Priority >= 3 && (staleOpen || staleOverdue) {
			action.Action = noteCurationActionReview
			action.Reason = noteCurationReasonHighPriorityReview
			plan.ReviewRequired = append(plan.ReviewRequired, action)
		}
	}

	sortNoteCurationActions(plan.AutoArchive)
	sortNoteCurationActions(plan.ReviewRequired)
	if len(plan.AutoArchive) > maxActions {
		plan.AutoArchive = plan.AutoArchive[:maxActions]
	}
	if len(plan.ReviewRequired) > maxActions {
		plan.ReviewRequired = plan.ReviewRequired[:maxActions]
	}
	plan.AutoArchiveCount = len(plan.AutoArchive)
	plan.ReviewRequiredCount = len(plan.ReviewRequired)
	return plan, nil
}

func (s *SQLiteMemory) ApplyNoteCurationAction(action NoteCurationAction, actor string, dryRun bool) error {
	if s == nil || action.NoteID <= 0 {
		return nil
	}
	normalizedAction := strings.ToLower(strings.TrimSpace(action.Action))
	if normalizedAction == "" {
		normalizedAction = noteCurationActionArchive
	}
	if normalizedAction != noteCurationActionArchive {
		return fmt.Errorf("unsupported note curation action %q", action.Action)
	}
	if dryRun {
		return nil
	}
	reason := strings.TrimSpace(action.Reason)
	if reason == "" {
		reason = noteCurationReasonStaleOpen
	}
	if strings.TrimSpace(actor) != "" {
		reason = reason + " (" + strings.TrimSpace(actor) + ")"
	}
	return s.ArchiveNote(action.NoteID, reason)
}

func noteDaysOverdue(dueDate string, now time.Time) int {
	dueDate = strings.TrimSpace(dueDate)
	if dueDate == "" {
		return 0
	}
	layouts := []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, dueDate)
		if err != nil {
			continue
		}
		days := int(now.Sub(parsed).Hours() / 24)
		if days < 0 {
			return 0
		}
		return days
	}
	return 0
}

func sortNoteCurationActions(actions []NoteCurationAction) {
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].DaysSinceUpdate == actions[j].DaysSinceUpdate {
			return actions[i].NoteID < actions[j].NoteID
		}
		return actions[i].DaysSinceUpdate > actions[j].DaysSinceUpdate
	})
}
