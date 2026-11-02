package findit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

type itemInput struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	Category      string         `json:"category"`
	TemplateID    string         `json:"template_id"`
	Manufacturer  string         `json:"manufacturer"`
	PartNumber    string         `json:"part_number"`
	Unit          string         `json:"unit"`
	MinimumStock  string         `json:"minimum_stock"`
	PackageFamily string         `json:"package_family"`
	PinCount      int            `json:"pin_count"`
	PitchMM       *float64       `json:"pitch_mm"`
	LengthMM      *float64       `json:"length_mm"`
	WidthMM       *float64       `json:"width_mm"`
	HeightMM      *float64       `json:"height_mm"`
	PackageSize   string         `json:"package_size"`
	SizeSystem    string         `json:"size_system"`
	Tags          []string       `json:"tags"`
	DatasheetURLs []string       `json:"datasheet_urls"`
	ImageID       string         `json:"image_id"`
	IconID        string         `json:"icon_id"`
	TrackSerials  bool           `json:"track_serials"`
	Properties    map[string]any `json:"properties"`
}

type locationInput struct {
	ParentID    string `json:"parent_id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description"`
}

type projectLineInput struct {
	ID       string `json:"id"`
	ItemID   string `json:"item_id"`
	Quantity string `json:"quantity"`
	Note     string `json:"note"`
}

type projectInput struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Status      string             `json:"status"`
	Lines       []projectLineInput `json:"lines"`
}

type templateInput struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Category    string               `json:"category"`
	Fields      []PropertyDefinition `json:"fields"`
}

type stockInput struct {
	Kind             string `json:"kind"`
	ItemID           string `json:"item_id"`
	StockID          string `json:"stock_id"`
	LocationID       string `json:"location_id"`
	TargetLocationID string `json:"target_location_id"`
	Quantity         string `json:"quantity"`
	StockVersion     int64  `json:"stock_version,omitempty"`
	Lot              string `json:"lot"`
	Condition        string `json:"condition"`
	Serial           string `json:"serial"`
	Note             string `json:"note"`
}

type reservationInput struct {
	ProjectID string `json:"project_id"`
	LineID    string `json:"line_id"`
	StockID   string `json:"stock_id"`
	Quantity  string `json:"quantity"`
}

type reservationChangeInput struct {
	ReservationID string `json:"reservation_id"`
	Quantity      string `json:"quantity"`
}

type projectStatusInput struct {
	Status string `json:"status"`
}

func (s *Store) Execute(ctx context.Context, req CommandRequest) (any, error) {
	if s == nil || s.db == nil {
		return nil, domainError("unavailable", "FindIt is unavailable")
	}
	switch req.Operation {
	case "stock", "reserve", "release", "consume", "project_status":
		return s.executeIdempotent(ctx, req)
	case "create", "update", "archive":
		if req.RequestID != "" {
			return s.executeIdempotent(ctx, req)
		}
		if req.Operation == "create" {
			return s.create(ctx, req)
		}
		if req.Operation == "update" {
			return s.update(ctx, req)
		}
		return s.archive(ctx, req)
	default:
		return nil, domainError("invalid", "unsupported FindIt command")
	}
}

func (s *Store) create(ctx context.Context, req CommandRequest) (any, error) {
	switch req.Entity {
	case "items":
		var input itemInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.createItem(ctx, input)
	case "locations":
		var input locationInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.createLocation(ctx, input)
	case "projects":
		var input projectInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.createProject(ctx, input)
	case "templates":
		var input templateInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.createTemplate(ctx, input)
	default:
		return nil, domainError("invalid", "unsupported FindIt entity")
	}
}

func (s *Store) update(ctx context.Context, req CommandRequest) (any, error) {
	if req.ExpectedVersion <= 0 || validateID(req.ID, "record id", true) != nil {
		return nil, domainError("invalid", "update requires an id and expected_version")
	}
	switch req.Entity {
	case "items":
		var input itemInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.updateItem(ctx, req.ID, req.ExpectedVersion, input)
	case "locations":
		var input locationInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.updateLocation(ctx, req.ID, req.ExpectedVersion, input)
	case "projects":
		var input projectInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.updateProject(ctx, req.ID, req.ExpectedVersion, input)
	case "templates":
		var input templateInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return s.updateTemplate(ctx, req.ID, req.ExpectedVersion, input)
	default:
		return nil, domainError("invalid", "unsupported FindIt entity")
	}
}

func (s *Store) archive(ctx context.Context, req CommandRequest) (any, error) {
	if req.ExpectedVersion <= 0 || validateID(req.ID, "record id", true) != nil {
		return nil, domainError("invalid", "archive requires an id and expected_version")
	}
	switch req.Entity {
	case "items":
		return s.archiveItem(ctx, req.ID, req.ExpectedVersion)
	case "locations":
		return s.archiveLocation(ctx, req.ID, req.ExpectedVersion)
	case "projects":
		return s.archiveProject(ctx, req.ID, req.ExpectedVersion)
	case "templates":
		return s.archiveTemplate(ctx, req.ID, req.ExpectedVersion)
	default:
		return nil, domainError("invalid", "unsupported FindIt entity")
	}
}

func commandFingerprint(req CommandRequest) (string, error) {
	var payload any
	raw := req.Data
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return "", domainError("invalid", "command data is invalid")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return "", domainError("invalid", "command data contains extra values")
	}
	canonical, err := json.Marshal(struct {
		Operation string `json:"operation"`
		Entity    string `json:"entity"`
		ID        string `json:"id"`
		Version   int64  `json:"version"`
		Data      any    `json:"data"`
	}{req.Operation, req.Entity, req.ID, req.ExpectedVersion, payload})
	if err != nil {
		return "", domainError("invalid", "command data is invalid")
	}
	hash := sha256.Sum256(canonical)
	return hex.EncodeToString(hash[:]), nil
}

func (s *Store) executeIdempotent(ctx context.Context, req CommandRequest) (any, error) {
	if _, err := validateText(req.RequestID, "request_id", 128, true); err != nil {
		return nil, err
	}
	fingerprint, err := commandFingerprint(req)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt command", err)
	}
	defer tx.Rollback()
	var currentFingerprint, resultJSON string
	err = tx.QueryRowContext(ctx, `SELECT fingerprint,result_json FROM command_replays WHERE request_id=?`, req.RequestID).Scan(&currentFingerprint, &resultJSON)
	if err == nil {
		if currentFingerprint != fingerprint {
			return nil, ErrConflict
		}
		return json.RawMessage(resultJSON), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, wrapError("unavailable", "read FindIt command receipt", err)
	}
	result, err := executeCommandTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, wrapError("unavailable", "serialize FindIt command result", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO command_replays(request_id,fingerprint,result_json,created_at) VALUES(?,?,?,?)`, req.RequestID, fingerprint, string(encoded), nowString()); err != nil {
		return nil, mapWriteError(err, "record command receipt")
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "commit command")
	}
	return result, nil
}

func executeCommandTx(ctx context.Context, tx *sql.Tx, req CommandRequest) (any, error) {
	switch req.Operation {
	case "create":
		switch req.Entity {
		case "items":
			var input itemInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return createItemTx(ctx, tx, input)
		case "locations":
			var input locationInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return createLocationTx(ctx, tx, input)
		case "projects":
			var input projectInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return createProjectTx(ctx, tx, input)
		case "templates":
			var input templateInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return createTemplateTx(ctx, tx, input)
		default:
			return nil, domainError("invalid", "unsupported FindIt entity")
		}
	case "update":
		if req.ExpectedVersion <= 0 || validateID(req.ID, "record id", true) != nil {
			return nil, domainError("invalid", "update requires an id and expected_version")
		}
		switch req.Entity {
		case "items":
			var input itemInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return updateItemTxCommand(ctx, tx, req.ID, req.ExpectedVersion, input)
		case "locations":
			var input locationInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return updateLocationTxCommand(ctx, tx, req.ID, req.ExpectedVersion, input)
		case "projects":
			var input projectInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return updateProjectTxCommand(ctx, tx, req.ID, req.ExpectedVersion, input)
		case "templates":
			var input templateInput
			if err := decodeData(req.Data, &input); err != nil {
				return nil, err
			}
			return updateTemplateTxCommand(ctx, tx, req.ID, req.ExpectedVersion, input)
		default:
			return nil, domainError("invalid", "unsupported FindIt entity")
		}
	case "archive":
		if req.ExpectedVersion <= 0 || validateID(req.ID, "record id", true) != nil {
			return nil, domainError("invalid", "archive requires an id and expected_version")
		}
		switch req.Entity {
		case "items":
			return archiveItemTx(ctx, tx, req.ID, req.ExpectedVersion)
		case "locations":
			return archiveLocationTx(ctx, tx, req.ID, req.ExpectedVersion)
		case "projects":
			return archiveProjectTx(ctx, tx, req.ID, req.ExpectedVersion)
		case "templates":
			return archiveTemplateTx(ctx, tx, req.ID, req.ExpectedVersion)
		default:
			return nil, domainError("invalid", "unsupported FindIt entity")
		}
	case "stock":
		var input stockInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return applyStockCommand(ctx, tx, req.RequestID, input)
	case "reserve":
		var input reservationInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return reserveStock(ctx, tx, req.RequestID, input)
	case "release", "consume":
		var input reservationChangeInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return changeReservation(ctx, tx, req.RequestID, req.Operation, input)
	case "project_status":
		var input projectStatusInput
		if err := decodeData(req.Data, &input); err != nil {
			return nil, err
		}
		return updateProjectStatus(ctx, tx, req, input)
	default:
		return nil, domainError("invalid", "unsupported FindIt command")
	}
}
