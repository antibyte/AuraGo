package findit

import (
	"database/sql"
	"encoding/json"
	"errors"
)

const (
	SchemaVersion = 1
	quantityScale = int64(1_000_000)
	defaultLimit  = 50
	maxLimit      = 200
)

var (
	ErrInvalid     = &DomainError{Code: "invalid", Message: "invalid FindIt request"}
	ErrNotFound    = &DomainError{Code: "not_found", Message: "FindIt record not found"}
	ErrConflict    = &DomainError{Code: "conflict", Message: "FindIt record conflicts with current state"}
	ErrUnavailable = &DomainError{Code: "unavailable", Message: "FindIt is unavailable"}
)

// DomainError exposes a stable, non-sensitive error category to HTTP adapters.
type DomainError struct {
	Code    string            `json:"code"`
	Reason  string            `json:"reason,omitempty"`
	Field   string            `json:"field,omitempty"`
	Details map[string]string `json:"details,omitempty"`
	Message string            `json:"message"`
	cause   error             `json:"-"`
}

func (e *DomainError) Error() string {
	if e == nil {
		return "FindIt error"
	}
	return e.Message
}

func (e *DomainError) Unwrap() error { return e.cause }

func domainError(code, message string) error {
	return &DomainError{Code: code, Reason: code, Message: message}
}

func wrapError(code, message string, cause error) error {
	return &DomainError{Code: code, Reason: code, Message: message, cause: cause}
}

func detailedError(code, reason, field, message string, details map[string]string) error {
	return &DomainError{Code: code, Reason: reason, Field: field, Details: details, Message: message}
}

// ErrorDetail returns the safe structured domain error for HTTP or tool adapters.
func ErrorDetail(err error) *DomainError {
	var target *DomainError
	if errors.As(err, &target) {
		return target
	}
	return &DomainError{Code: "unavailable", Reason: "unavailable", Message: "FindIt is unavailable"}
}

// ErrorCode maps a domain failure to the stable code used by server adapters.
func ErrorCode(err error) string {
	var target *DomainError
	if errors.As(err, &target) && target.Code != "" {
		return target.Code
	}
	return "unavailable"
}

type QueryRequest struct {
	Operation     string           `json:"operation"`
	Entity        string           `json:"entity"`
	ID            string           `json:"id,omitempty"`
	Query         string           `json:"query,omitempty"`
	LocationID    string           `json:"location_id,omitempty"`
	TemplateID    string           `json:"template_id,omitempty"`
	Category      string           `json:"category,omitempty"`
	Package       string           `json:"package,omitempty"`
	AvailableOnly bool             `json:"available_only,omitempty"`
	LowStock      bool             `json:"low_stock,omitempty"`
	Filters       []PropertyFilter `json:"filters,omitempty"`
	Limit         int              `json:"limit,omitempty"`
	Offset        int              `json:"offset,omitempty"`
}

type PropertyFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type CommandRequest struct {
	Operation       string          `json:"operation"`
	Entity          string          `json:"entity,omitempty"`
	ID              string          `json:"id,omitempty"`
	ExpectedVersion int64           `json:"expected_version,omitempty"`
	RequestID       string          `json:"request_id,omitempty"`
	Data            json.RawMessage `json:"data"`
}

type SearchResult struct {
	Items  []any `json:"items"`
	Total  int   `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

type CategoryDefinition struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
}

type UnitDefinition struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Symbol string `json:"symbol"`
}

type PackageFamily struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type PropertyDefinition struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required,omitempty"`
	Unit     string   `json:"unit,omitempty"`
	Options  []string `json:"options,omitempty"`
	Min      *float64 `json:"min,omitempty"`
	Max      *float64 `json:"max,omitempty"`
}

type Template struct {
	ID          string               `json:"id"`
	IconID      string               `json:"icon_id"`
	Name        string               `json:"name"`
	Description string               `json:"description,omitempty"`
	Category    string               `json:"category"`
	BuiltIn     bool                 `json:"built_in"`
	Archived    bool                 `json:"archived,omitempty"`
	Version     int64                `json:"version"`
	Fields      []PropertyDefinition `json:"fields"`
}

type CatalogData struct {
	Version         int                  `json:"version"`
	Categories      []CategoryDefinition `json:"categories"`
	Units           []UnitDefinition     `json:"units"`
	PackageFamilies []PackageFamily      `json:"package_families"`
	Templates       []Template           `json:"templates"`
}

type Item struct {
	ID             string           `json:"id"`
	Version        int64            `json:"version"`
	Name           string           `json:"name"`
	Description    string           `json:"description,omitempty"`
	Category       string           `json:"category"`
	TemplateID     string           `json:"template_id"`
	Manufacturer   string           `json:"manufacturer,omitempty"`
	PartNumber     string           `json:"part_number,omitempty"`
	Unit           string           `json:"unit"`
	MinimumStock   string           `json:"minimum_stock"`
	PackageFamily  string           `json:"package_family,omitempty"`
	PinCount       int              `json:"pin_count,omitempty"`
	PitchMM        *float64         `json:"pitch_mm,omitempty"`
	LengthMM       *float64         `json:"length_mm,omitempty"`
	WidthMM        *float64         `json:"width_mm,omitempty"`
	HeightMM       *float64         `json:"height_mm,omitempty"`
	PackageSize    string           `json:"package_size,omitempty"`
	SizeSystem     string           `json:"size_system,omitempty"`
	Tags           []string         `json:"tags"`
	DatasheetURLs  []string         `json:"datasheet_urls"`
	ImageID        string           `json:"image_id,omitempty"`
	IconID         string           `json:"icon_id,omitempty"`
	TrackSerials   bool             `json:"track_serials"`
	Properties     map[string]any   `json:"properties"`
	Availability   Availability     `json:"availability"`
	StockLocations []StockLocation  `json:"stock_locations"`
	Movements      []MovementRecord `json:"movements,omitempty"`
	Archived       bool             `json:"archived,omitempty"`
	CreatedAt      string           `json:"created_at"`
	UpdatedAt      string           `json:"updated_at"`
}

type Location struct {
	ID          string   `json:"id"`
	Version     int64    `json:"version"`
	ParentID    string   `json:"parent_id,omitempty"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
	Path        []string `json:"path,omitempty"`
	Archived    bool     `json:"archived,omitempty"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

type Project struct {
	ID                  string        `json:"id"`
	Version             int64         `json:"version"`
	Name                string        `json:"name"`
	Description         string        `json:"description,omitempty"`
	Status              string        `json:"status"`
	BOMRevision         int64         `json:"bom_revision"`
	ReleasedRevision    int64         `json:"released_revision,omitempty"`
	Lines               []ProjectLine `json:"lines"`
	Reservations        []Reservation `json:"reservations,omitempty"`
	ReservationsHasMore bool          `json:"reservations_has_more,omitempty"`
	Archived            bool          `json:"archived,omitempty"`
	CreatedAt           string        `json:"created_at"`
	UpdatedAt           string        `json:"updated_at"`
}

type ProjectLine struct {
	ID         string `json:"id"`
	ItemID     string `json:"item_id"`
	ItemName   string `json:"item_name,omitempty"`
	Unit       string `json:"unit"`
	PartNumber string `json:"part_number,omitempty"`
	Quantity   string `json:"quantity"`
	Reserved   string `json:"reserved,omitempty"`
	Consumed   string `json:"consumed,omitempty"`
	Note       string `json:"note,omitempty"`
}

type Availability struct {
	OnHand    string `json:"on_hand"`
	Reserved  string `json:"reserved"`
	Available string `json:"available"`
}

type StockRecord struct {
	ID           string   `json:"id"`
	ItemID       string   `json:"item_id"`
	LocationID   string   `json:"location_id"`
	Location     string   `json:"location,omitempty"`
	LocationPath []string `json:"location_path"`
	Quantity     string   `json:"quantity"`
	Reserved     string   `json:"reserved"`
	Available    string   `json:"available"`
	Lot          string   `json:"lot,omitempty"`
	Condition    string   `json:"condition,omitempty"`
	Serial       string   `json:"serial,omitempty"`
	Version      int64    `json:"version"`
}

// StockLocation is a distinct location containing a positive on-hand quantity.
type StockLocation struct {
	ID   string   `json:"id"`
	Path []string `json:"path"`
}

type Reservation struct {
	ID           string   `json:"id"`
	ProjectID    string   `json:"project_id"`
	LineID       string   `json:"line_id"`
	StockID      string   `json:"stock_id"`
	LocationPath []string `json:"location_path"`
	Quantity     string   `json:"quantity"`
	Consumed     string   `json:"consumed"`
	Released     string   `json:"released"`
	Remaining    string   `json:"remaining"`
}

type MovementRecord struct {
	ID                 string   `json:"id"`
	RequestID          string   `json:"request_id"`
	Kind               string   `json:"kind"`
	ItemID             string   `json:"item_id"`
	StockID            string   `json:"stock_id"`
	SourceLocationID   string   `json:"source_location_id,omitempty"`
	SourceLocationPath []string `json:"source_location_path,omitempty"`
	TargetLocationID   string   `json:"target_location_id,omitempty"`
	TargetLocationPath []string `json:"target_location_path,omitempty"`
	Quantity           string   `json:"quantity"`
	ProjectID          string   `json:"project_id,omitempty"`
	ReservationID      string   `json:"reservation_id,omitempty"`
	Serial             string   `json:"serial,omitempty"`
	Lot                string   `json:"lot,omitempty"`
	Condition          string   `json:"condition,omitempty"`
	Note               string   `json:"note,omitempty"`
	CreatedAt          string   `json:"created_at"`
}

type StockCommandResult struct {
	MovementID    string      `json:"movement_id"`
	Kind          string      `json:"kind"`
	MovedQuantity string      `json:"moved_quantity"`
	Stock         StockRecord `json:"stock"`
}

type AvailabilityResult struct {
	ItemID    string        `json:"item_id"`
	Unit      string        `json:"unit"`
	OnHand    string        `json:"on_hand"`
	Reserved  string        `json:"reserved"`
	Available string        `json:"available"`
	Stocks    []StockRecord `json:"stocks"`
}

type ShortageLine struct {
	ProjectLineID string `json:"line_id"`
	ItemID        string `json:"item_id"`
	ItemName      string `json:"item_name"`
	Unit          string `json:"unit"`
	PartNumber    string `json:"part_number,omitempty"`
	Required      string `json:"required"`
	Reserved      string `json:"reserved"`
	Consumed      string `json:"consumed"`
	Available     string `json:"available"`
	Shortage      string `json:"shortage"`
}

type ShortagesResult struct {
	Project             Project        `json:"project"`
	Lines               []ShortageLine `json:"lines"`
	Reservations        []Reservation  `json:"reservations"`
	ReservationsHasMore bool           `json:"reservations_has_more,omitempty"`
}

type ImportRequest struct {
	Entity          string            `json:"entity"`
	CSV             string            `json:"csv"`
	Mapping         map[string]string `json:"mapping,omitempty"`
	ProjectID       string            `json:"project_id,omitempty"`
	ExpectedVersion int64             `json:"expected_version,omitempty"`
	RequestID       string            `json:"request_id,omitempty"`
	Resolutions     map[int]string    `json:"resolutions,omitempty"`
	SkipRows        []int             `json:"skip_rows,omitempty"`
}

type CSVRowError struct {
	Field   string `json:"field,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ItemCandidate struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Manufacturer string `json:"manufacturer,omitempty"`
	PartNumber   string `json:"part_number,omitempty"`
}

type ImportPreviewRow struct {
	Row            int             `json:"row"`
	Data           map[string]any  `json:"data"`
	Errors         []CSVRowError   `json:"errors"`
	Candidates     []ItemCandidate `json:"candidates,omitempty"`
	ResolvedItemID string          `json:"resolved_item_id,omitempty"`
	Skipped        bool            `json:"skipped,omitempty"`
}

type ImportPreview struct {
	Columns      []string           `json:"columns"`
	Rows         []ImportPreviewRow `json:"rows"`
	ValidCount   int                `json:"valid_count"`
	InvalidCount int                `json:"invalid_count"`
}

type ImportResult struct {
	Entity    string   `json:"entity"`
	Created   []string `json:"created"`
	Skipped   []int    `json:"skipped,omitempty"`
	ProjectID string   `json:"project_id,omitempty"`
	Version   int64    `json:"version,omitempty"`
}

// Store is the private, standalone SQLite store for FindIt.
type Store struct {
	db *sql.DB
}
