package findit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
)

const maxActiveReservationsPerProject = 5000

type stockState struct {
	ID           string
	ItemID       string
	LocationID   string
	Quantity     int64
	Lot          string
	Condition    string
	Serial       string
	Version      int64
	TrackSerials bool
	Unit         string
}

func applyStockCommand(ctx context.Context, tx *sql.Tx, requestID string, input stockInput) (any, error) {
	kind := strings.TrimSpace(strings.ToLower(input.Kind))
	if !oneOf(kind, "receive", "issue", "transfer", "adjust", "return") {
		return nil, domainError("invalid", "stock command kind is invalid")
	}
	if err := validateID(input.ItemID, "item id", true); err != nil {
		return nil, err
	}
	if input.StockID != "" {
		if err := validateID(input.StockID, "stock id", true); err != nil {
			return nil, err
		}
	}
	quantity, err := ParseQuantity(input.Quantity)
	if err != nil || kind != "adjust" && quantity <= 0 {
		return nil, domainError("invalid", "stock quantity must be positive")
	}
	locationID := strings.TrimSpace(input.LocationID)
	targetLocation := strings.TrimSpace(input.TargetLocationID)
	if locationID != "" {
		if err := validateID(locationID, "location id", true); err != nil {
			return nil, err
		}
	}
	if targetLocation != "" {
		if err := validateID(targetLocation, "target location id", true); err != nil {
			return nil, err
		}
	}
	lot, err := validateText(input.Lot, "lot", 128, false)
	if err != nil {
		return nil, err
	}
	condition := strings.TrimSpace(strings.ToLower(input.Condition))
	if condition == "" {
		condition = "new"
	}
	if !oneOf(condition, "new", "used", "refurbished", "damaged", "unknown") {
		return nil, domainError("invalid", "stock condition is invalid")
	}
	serial, err := validateText(input.Serial, "serial", 256, false)
	if err != nil {
		return nil, err
	}
	note, err := validateText(input.Note, "note", 2048, false)
	if err != nil {
		return nil, err
	}
	item, err := getItemWithoutAvailability(ctx, tx, input.ItemID)
	if err != nil {
		return nil, err
	}
	if item.Archived {
		return nil, domainError("conflict", "archived items cannot receive stock changes")
	}
	if item.TrackSerials {
		if item.Unit != "piece" || serial == "" && kind == "receive" || quantity != quantityScale && kind != "adjust" {
			return nil, domainError("invalid", "serial-tracked stock requires one piece and a serial number")
		}
		if kind == "adjust" && quantity != 0 && quantity != quantityScale {
			return nil, domainError("invalid", "serial-tracked stock quantity must be zero or one")
		}
	} else if serial != "" {
		return nil, domainError("invalid", "serial number requires serial tracking on the item")
	}
	if item.Unit == "piece" && quantity%quantityScale != 0 {
		return nil, domainError("invalid", "piece stock quantity must be a whole number")
	}
	if kind == "receive" {
		if locationID == "" {
			return nil, domainError("invalid", "receiving stock requires a location")
		}
		if err := requireActiveLocation(ctx, tx, locationID); err != nil {
			return nil, err
		}
	}
	var stock stockState
	var delta int64
	switch kind {
	case "receive":
		if input.StockID == "" {
			stock = stockState{ID: newID(), ItemID: input.ItemID, LocationID: locationID, Lot: lot, Condition: condition, Serial: serial, Version: 1, TrackSerials: item.TrackSerials, Unit: item.Unit}
			if err := createStockRow(ctx, tx, stock, quantity); err != nil {
				return nil, mapWriteError(err, "receive stock")
			}
			stock.Quantity = quantity
			delta = quantity
		} else {
			stock, err = loadStock(ctx, tx, input.StockID)
			if err != nil {
				return nil, err
			}
			if stock.ItemID != input.ItemID || stock.LocationID != locationID || stock.Lot != lot && lot != "" || stock.Condition != condition || stock.Serial != serial && serial != "" {
				return nil, ErrConflict
			}
			if stock.TrackSerials && stock.Quantity > 0 {
				return nil, domainError("conflict", "serial is already in stock; use return only after issue")
			}
			if err := addStock(ctx, tx, stock, quantity); err != nil {
				return nil, err
			}
			stock.Quantity += quantity
			delta = quantity
		}
	case "return":
		if input.StockID == "" {
			return nil, domainError("invalid", "return requires stock_id")
		}
		stock, err = loadStock(ctx, tx, input.StockID)
		if err != nil {
			return nil, err
		}
		if stock.ItemID != input.ItemID || stock.TrackSerials && stock.Quantity != 0 || stock.TrackSerials && serial != "" && stock.Serial != serial || !stock.TrackSerials && serial != "" {
			return nil, ErrConflict
		}
		if locationID != "" {
			if err := requireActiveLocation(ctx, tx, locationID); err != nil {
				return nil, err
			}
			if !stock.TrackSerials && locationID != stock.LocationID {
				return nil, domainError("conflict", "return location differs from the stock location; transfer stock separately")
			}
			stock.LocationID = locationID
		}
		if stock.TrackSerials && quantity != quantityScale || stock.Unit == "piece" && quantity%quantityScale != 0 {
			return nil, domainError("invalid", "returned stock quantity is invalid")
		}
		if err := setStock(ctx, tx, stock, stock.Quantity+quantity, stock.Version); err != nil {
			return nil, err
		}
		stock.Quantity += quantity
		delta = quantity
	case "issue":
		stock, err = requireStock(ctx, tx, input.StockID, input.ItemID)
		if err != nil {
			return nil, err
		}
		if stock.Unit == "piece" && quantity%quantityScale != 0 || stock.TrackSerials && quantity != quantityScale {
			return nil, domainError("invalid", "issued stock quantity is invalid")
		}
		if err := subtractAvailable(ctx, tx, stock, quantity); err != nil {
			return nil, err
		}
		stock.Quantity -= quantity
		delta = -quantity
	case "adjust":
		stock, err = loadStock(ctx, tx, input.StockID)
		if err != nil {
			return nil, err
		}
		if stock.ItemID != input.ItemID {
			return nil, ErrConflict
		}
		if stock.Unit == "piece" && quantity%quantityScale != 0 || stock.TrackSerials && quantity != 0 && quantity != quantityScale {
			return nil, domainError("invalid", "adjusted stock quantity is invalid")
		}
		delta = quantity - stock.Quantity
		if delta < 0 {
			var reserved int64
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, stock.ID).Scan(&reserved); err != nil {
				return nil, wrapError("unavailable", "read FindIt reservations", err)
			}
			if quantity < reserved {
				return nil, domainError("conflict", "adjustment would reduce stock below active reservations")
			}
		}
		if err := setStock(ctx, tx, stock, quantity, stock.Version); err != nil {
			return nil, err
		}
		stock.Quantity = quantity
	case "transfer":
		stock, err = requireStock(ctx, tx, input.StockID, input.ItemID)
		if err != nil {
			return nil, err
		}
		if targetLocation == "" || targetLocation == stock.LocationID {
			return nil, domainError("invalid", "transfer requires a different target location")
		}
		if err := requireActiveLocation(ctx, tx, targetLocation); err != nil {
			return nil, err
		}
		if stock.Unit == "piece" && quantity%quantityScale != 0 || stock.TrackSerials && quantity != quantityScale {
			return nil, domainError("invalid", "transferred stock quantity is invalid")
		}
		sourceLocation := stock.LocationID
		if stock.TrackSerials {
			var reserved int64
			if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, stock.ID).Scan(&reserved); err != nil {
				return nil, wrapError("unavailable", "read FindIt reservations", err)
			}
			if reserved > 0 {
				return nil, domainError("conflict", "reserved serial stock cannot be transferred")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE stocks SET location_id=?,version=version+1,updated_at=? WHERE id=? AND version=?`, targetLocation, nowString(), stock.ID, stock.Version); err != nil {
				return nil, mapWriteError(err, "transfer stock")
			}
			stock.LocationID, stock.Version = targetLocation, stock.Version+1
			delta = quantity
			if err := insertMovement(ctx, tx, requestID, kind, input.ItemID, stock.ID, sourceLocation, targetLocation, delta, "", "", stock.Serial, stock.Lot, stock.Condition, note); err != nil {
				return nil, err
			}
			return stockResult(ctx, tx, requestID, kind, quantity, stock)
		}
		if err := subtractAvailable(ctx, tx, stock, quantity); err != nil {
			return nil, err
		}
		stock.Quantity -= quantity
		target := stockState{ID: newID(), ItemID: stock.ItemID, LocationID: targetLocation, Lot: stock.Lot, Condition: stock.Condition, Serial: stock.Serial, Version: 1, TrackSerials: stock.TrackSerials, Unit: stock.Unit}
		var targetID string
		err = tx.QueryRowContext(ctx, `SELECT id FROM stocks WHERE item_id=? AND location_id=? AND lot=? AND condition=? AND serial_norm='' LIMIT 1`, stock.ItemID, targetLocation, stock.Lot, stock.Condition).Scan(&targetID)
		if errors.Is(err, sql.ErrNoRows) {
			if err := createStockRow(ctx, tx, target, quantity); err != nil {
				return nil, mapWriteError(err, "transfer stock")
			}
			targetID = target.ID
		} else if err != nil {
			return nil, wrapError("unavailable", "find FindIt transfer destination", err)
		} else {
			dest, err := loadStock(ctx, tx, targetID)
			if err != nil {
				return nil, err
			}
			if err := addStock(ctx, tx, dest, quantity); err != nil {
				return nil, err
			}
			dest.Quantity += quantity
		}
		delta = quantity
		if err := insertMovement(ctx, tx, requestID, kind, input.ItemID, stock.ID, sourceLocation, targetLocation, delta, "", "", "", lot, condition, note); err != nil {
			return nil, err
		}
		if targetID != "" {
			stock, err = loadStock(ctx, tx, targetID)
			if err != nil {
				return nil, err
			}
		}
		return stockResult(ctx, tx, requestID, kind, quantity, stock)
	}
	if kind != "transfer" {
		if err := insertMovement(ctx, tx, requestID, kind, input.ItemID, stock.ID, movementSource(kind, stock.LocationID), movementTarget(kind, stock.LocationID), delta, "", "", stock.Serial, stock.Lot, stock.Condition, note); err != nil {
			return nil, err
		}
	}
	return stockResult(ctx, tx, requestID, kind, absInt64(delta), stock)
}

func createStockRow(ctx context.Context, tx *sql.Tx, stock stockState, quantity int64) error {
	now := nowString()
	_, err := tx.ExecContext(ctx, `INSERT INTO stocks(id,item_id,location_id,quantity_micro,lot,condition,serial,serial_norm,note,version,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,1,?,?)`, stock.ID, stock.ItemID, stock.LocationID, quantity, stock.Lot, stock.Condition,
		stock.Serial, strings.ToLower(strings.TrimSpace(stock.Serial)), "", now, now)
	return err
}

func loadStock(ctx context.Context, tx *sql.Tx, id string) (stockState, error) {
	var stock stockState
	var track int
	err := tx.QueryRowContext(ctx, `SELECT st.id,st.item_id,st.location_id,st.quantity_micro,st.lot,st.condition,st.serial,st.version,i.track_serials,i.unit
		FROM stocks st JOIN items i ON i.id=st.item_id WHERE st.id=?`, id).Scan(&stock.ID, &stock.ItemID, &stock.LocationID, &stock.Quantity, &stock.Lot, &stock.Condition, &stock.Serial, &stock.Version, &track, &stock.Unit)
	if errors.Is(err, sql.ErrNoRows) {
		return stockState{}, ErrNotFound
	}
	if err != nil {
		return stockState{}, wrapError("unavailable", "read FindIt stock", err)
	}
	stock.TrackSerials = track != 0
	return stock, nil
}

func requireStock(ctx context.Context, tx *sql.Tx, stockID, itemID string) (stockState, error) {
	if err := validateID(stockID, "stock id", true); err != nil {
		return stockState{}, err
	}
	stock, err := loadStock(ctx, tx, stockID)
	if err != nil {
		return stockState{}, err
	}
	if stock.ItemID != itemID || stock.Quantity <= 0 {
		return stockState{}, ErrConflict
	}
	return stock, nil
}

func addStock(ctx context.Context, tx *sql.Tx, stock stockState, quantity int64) error {
	if quantity < 0 || stock.Quantity > int64(^uint64(0)>>1)-quantity {
		return domainError("invalid", "stock quantity is outside the supported range")
	}
	return setStock(ctx, tx, stock, stock.Quantity+quantity, stock.Version)
}

func setStock(ctx context.Context, tx *sql.Tx, stock stockState, quantity, expected int64) error {
	if quantity < 0 {
		return domainError("invalid", "stock cannot be negative")
	}
	result, err := tx.ExecContext(ctx, `UPDATE stocks SET quantity_micro=?,location_id=?,version=version+1,updated_at=? WHERE id=? AND version=?`, quantity, stock.LocationID, nowString(), stock.ID, expected)
	if err != nil {
		return mapWriteError(err, "update stock")
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return wrapError("unavailable", "count FindIt stock update", err)
	}
	if changed != 1 {
		return ErrConflict
	}
	stock.Version++
	return nil
}

func subtractAvailable(ctx context.Context, tx *sql.Tx, stock stockState, quantity int64) error {
	if quantity <= 0 {
		return invalidQuantity("stock quantity must be positive")
	}
	var reserved int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, stock.ID).Scan(&reserved); err != nil {
		return wrapError("unavailable", "read FindIt reservations", err)
	}
	available := stock.Quantity - reserved
	if available < quantity {
		return detailedError("conflict", "insufficient_available", "quantity", "requested quantity exceeds free stock", map[string]string{
			"available": FormatQuantity(available), "requested": FormatQuantity(quantity),
		})
	}
	if err := setStock(ctx, tx, stock, stock.Quantity-quantity, stock.Version); err != nil {
		return err
	}
	stock.Quantity -= quantity
	return nil
}

func insertMovement(ctx context.Context, tx *sql.Tx, requestID, kind, itemID, stockID, sourceID, targetID string, quantity int64, projectID, reservationID, serial, lot, condition, note string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO stock_movements(id,request_id,kind,item_id,stock_id,source_location_id,target_location_id,quantity_micro,project_id,reservation_id,serial,lot,condition,note,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, newID(), requestID, kind, itemID, stockID, nullableText(sourceID), nullableText(targetID), quantity,
		nullableText(projectID), nullableText(reservationID), serial, lot, condition, note, nowString())
	if err != nil {
		return mapWriteError(err, "record stock movement")
	}
	return nil
}

func stockResult(ctx context.Context, tx *sql.Tx, requestID, kind string, moved int64, stock stockState) (any, error) {
	record, err := stockRecord(ctx, tx, stock.ID)
	if err != nil {
		return nil, err
	}
	var movementID string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM stock_movements WHERE request_id=? ORDER BY rowid DESC LIMIT 1`, requestID).Scan(&movementID); err != nil {
		return nil, wrapError("unavailable", "read FindIt stock movement", err)
	}
	return StockCommandResult{MovementID: movementID, Kind: kind, MovedQuantity: FormatQuantity(moved), Stock: record}, nil
}

func stockRecord(ctx context.Context, tx *sql.Tx, id string) (StockRecord, error) {
	var result StockRecord
	var quantity int64
	var location string
	if err := tx.QueryRowContext(ctx, `SELECT s.id,s.item_id,s.location_id,l.name,s.quantity_micro,s.lot,s.condition,s.serial,s.version
		FROM stocks s JOIN locations l ON l.id=s.location_id WHERE s.id=?`, id).Scan(&result.ID, &result.ItemID, &result.LocationID, &location, &quantity, &result.Lot, &result.Condition, &result.Serial, &result.Version); errors.Is(err, sql.ErrNoRows) {
		return StockRecord{}, ErrNotFound
	} else if err != nil {
		return StockRecord{}, wrapError("unavailable", "read FindIt stock result", err)
	}
	result.Location = location
	result.Quantity = FormatQuantity(quantity)
	var reserved int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, id).Scan(&reserved); err != nil {
		return StockRecord{}, wrapError("unavailable", "read FindIt stock reservations", err)
	}
	result.Reserved = FormatQuantity(reserved)
	result.Available = FormatQuantity(quantity - reserved)
	return result, nil
}

func requireActiveLocation(ctx context.Context, tx *sql.Tx, id string) error {
	var exists int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM locations WHERE id=? AND archived=0`, id).Scan(&exists); err != nil {
		return wrapError("unavailable", "read FindIt location", err)
	}
	if exists == 0 {
		return ErrNotFound
	}
	return nil
}

func reserveStock(ctx context.Context, tx *sql.Tx, requestID string, input reservationInput) (any, error) {
	for field, value := range map[string]string{"project id": input.ProjectID, "BOM line id": input.LineID, "stock id": input.StockID} {
		if err := validateID(value, field, true); err != nil {
			return nil, err
		}
	}
	quantity, err := ParseQuantity(input.Quantity)
	if err != nil || quantity <= 0 {
		return nil, domainError("invalid", "reservation quantity must be positive")
	}
	stock, err := loadStock(ctx, tx, input.StockID)
	if err != nil {
		return nil, err
	}
	var lineItem string
	var required int64
	if err := tx.QueryRowContext(ctx, `SELECT item_id,quantity_micro FROM project_lines WHERE id=? AND project_id=? AND active=1`, input.LineID, input.ProjectID).Scan(&lineItem, &required); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt project line", err)
	}
	if lineItem != stock.ItemID || stock.Quantity <= 0 {
		return nil, domainError("invalid", "reservation item does not match the project line")
	}
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM projects WHERE id=? AND archived=0`, input.ProjectID).Scan(&status); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt project", err)
	}
	if status != "released" {
		return nil, domainError("conflict", "reservations require a released project")
	}
	item, err := getItemWithoutAvailability(ctx, tx, stock.ItemID)
	if err != nil {
		return nil, err
	}
	if item.Unit == "piece" && quantity%quantityScale != 0 || stock.TrackSerials && quantity != quantityScale {
		return nil, domainError("invalid", "reservation quantity is invalid for the item")
	}
	var lineHeld, consumed int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0),COALESCE(sum(consumed_micro),0) FROM reservations WHERE line_id=?`, input.LineID).Scan(&lineHeld, &consumed); err != nil {
		return nil, wrapError("unavailable", "read FindIt reservations", err)
	}
	remainingNeed := required - consumed - lineHeld
	if quantity > remainingNeed {
		return nil, detailedError("conflict", "reservation_exceeds_need", "quantity", "reservation exceeds the remaining BOM need", map[string]string{
			"remaining_need": FormatQuantity(remainingNeed), "requested": FormatQuantity(quantity),
		})
	}
	var stockHeld int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, input.StockID).Scan(&stockHeld); err != nil {
		return nil, wrapError("unavailable", "read FindIt stock reservations", err)
	}
	available := stock.Quantity - stockHeld
	if quantity > available {
		return nil, detailedError("conflict", "insufficient_available", "quantity", "requested quantity exceeds free stock", map[string]string{
			"available": FormatQuantity(available), "requested": FormatQuantity(quantity),
		})
	}
	var activeCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM reservations WHERE project_id=? AND quantity_micro>consumed_micro+released_micro`, input.ProjectID).Scan(&activeCount); err != nil {
		return nil, wrapError("unavailable", "count FindIt project reservations", err)
	}
	if activeCount >= maxActiveReservationsPerProject {
		return nil, domainError("conflict", "project has reached the active reservation limit")
	}
	reservation := Reservation{ID: newID(), ProjectID: input.ProjectID, LineID: input.LineID, StockID: input.StockID,
		Quantity: FormatQuantity(quantity), Consumed: "0", Released: "0", Remaining: FormatQuantity(quantity)}
	if _, err := tx.ExecContext(ctx, `INSERT INTO reservations(id,project_id,line_id,stock_id,quantity_micro,created_at) VALUES(?,?,?,?,?,?)`, reservation.ID, input.ProjectID, input.LineID, input.StockID, quantity, nowString()); err != nil {
		return nil, mapWriteError(err, "reserve stock")
	}
	if err := insertMovement(ctx, tx, requestID, "reserve", stock.ItemID, stock.ID, "", stock.LocationID, quantity, input.ProjectID, reservation.ID, stock.Serial, stock.Lot, stock.Condition, ""); err != nil {
		return nil, err
	}
	return getReservation(ctx, tx, reservation.ID)
}

func changeReservation(ctx context.Context, tx *sql.Tx, requestID, operation string, input reservationChangeInput) (any, error) {
	if err := validateID(input.ReservationID, "reservation id", true); err != nil {
		return nil, err
	}
	quantity, err := ParseQuantity(input.Quantity)
	if err != nil || quantity <= 0 {
		return nil, domainError("invalid", "reservation change quantity must be positive")
	}
	var result Reservation
	var quantityMicro, consumed, released int64
	var stockID, projectID, lineID, itemID, locationID, serial, lot, condition string
	var track int
	err = tx.QueryRowContext(ctx, `SELECT r.id,r.project_id,r.line_id,r.stock_id,r.quantity_micro,r.consumed_micro,r.released_micro,
		s.item_id,s.location_id,s.serial,s.lot,s.condition,i.track_serials,i.unit
		FROM reservations r JOIN stocks s ON s.id=r.stock_id JOIN items i ON i.id=s.item_id WHERE r.id=?`, input.ReservationID).
		Scan(&result.ID, &projectID, &lineID, &stockID, &quantityMicro, &consumed, &released, &itemID, &locationID, &serial, &lot, &condition, &track, new(string))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt reservation", err)
	}
	remaining := quantityMicro - consumed - released
	if quantity > remaining {
		return nil, domainError("conflict", "reservation change exceeds the remaining quantity")
	}
	var unit string
	if err := tx.QueryRowContext(ctx, `SELECT unit FROM items WHERE id=?`, itemID).Scan(&unit); err != nil {
		return nil, wrapError("unavailable", "read FindIt item unit", err)
	}
	if unit == "piece" && quantity%quantityScale != 0 || track != 0 && quantity != quantityScale {
		return nil, domainError("invalid", "reservation change quantity is invalid for the item")
	}
	if operation == "consume" {
		var projectStatus string
		if err := tx.QueryRowContext(ctx, `SELECT status FROM projects WHERE id=?`, projectID).Scan(&projectStatus); err != nil {
			return nil, wrapError("unavailable", "read FindIt project status", err)
		}
		if projectStatus != "released" {
			return nil, domainError("conflict", "consumption requires a released project")
		}
		stock, err := loadStock(ctx, tx, stockID)
		if err != nil {
			return nil, err
		}
		if stock.Quantity < quantity {
			return nil, domainError("conflict", "insufficient physical stock")
		}
		if err := setStock(ctx, tx, stock, stock.Quantity-quantity, stock.Version); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET consumed_micro=consumed_micro+? WHERE id=?`, quantity, input.ReservationID); err != nil {
			return nil, mapWriteError(err, "consume reservation")
		}
		if err := insertMovement(ctx, tx, requestID, "consume", itemID, stockID, locationID, "", -quantity, projectID, input.ReservationID, serial, lot, condition, ""); err != nil {
			return nil, err
		}
		consumed += quantity
	} else {
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET released_micro=released_micro+? WHERE id=?`, quantity, input.ReservationID); err != nil {
			return nil, mapWriteError(err, "release reservation")
		}
		if err := insertMovement(ctx, tx, requestID, "release", itemID, stockID, "", locationID, quantity, projectID, input.ReservationID, serial, lot, condition, ""); err != nil {
			return nil, err
		}
		released += quantity
	}
	return getReservation(ctx, tx, input.ReservationID)
}

func updateProjectStatus(ctx context.Context, tx *sql.Tx, req CommandRequest, input projectStatusInput) (any, error) {
	if req.Entity != "projects" || validateID(req.ID, "project id", true) != nil || req.ExpectedVersion <= 0 {
		return nil, domainError("invalid", "project_status requires a project id and expected_version")
	}
	wanted := strings.TrimSpace(strings.ToLower(input.Status))
	if !oneOf(wanted, "draft", "released", "cancelled", "completed") {
		return nil, domainError("invalid", "project status is invalid")
	}
	project, err := getProject(ctx, tx, req.ID)
	if err != nil {
		return nil, err
	}
	if project.Version != req.ExpectedVersion {
		return nil, ErrConflict
	}
	if project.Status == wanted {
		return project, nil
	}
	now := nowString()
	switch {
	case project.Status == "draft" && wanted == "released":
		if len(project.Lines) == 0 {
			return nil, domainError("invalid", "a project needs BOM lines before release")
		}
		var archivedBOMItems int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM project_lines l JOIN items i ON i.id=l.item_id
			WHERE l.project_id=? AND l.active=1 AND i.archived=1`, req.ID).Scan(&archivedBOMItems); err != nil {
			return nil, wrapError("unavailable", "check FindIt project BOM items", err)
		}
		if archivedBOMItems > 0 {
			return nil, domainError("conflict", "a project BOM references an archived item")
		}
		lines, err := json.Marshal(project.Lines)
		if err != nil {
			return nil, wrapError("unavailable", "serialize FindIt BOM revision", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_releases(project_id,revision,lines_json,released_at) VALUES(?,?,?,?)`, req.ID, project.BOMRevision, string(lines), now); err != nil {
			return nil, mapWriteError(err, "release project BOM")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='released',released_revision=?,version=version+1,updated_at=? WHERE id=? AND version=?`, project.BOMRevision, now, req.ID, req.ExpectedVersion); err != nil {
			return nil, mapWriteError(err, "release project")
		}
	case project.Status == "released" && wanted == "draft":
		var active, consumed int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0),COALESCE(sum(consumed_micro),0) FROM reservations WHERE project_id=?`, req.ID).Scan(&active, &consumed); err != nil {
			return nil, wrapError("unavailable", "check FindIt project reservations", err)
		}
		if active > 0 || consumed > 0 {
			return nil, domainError("conflict", "a project with active reservations or consumption cannot reopen its BOM")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='draft',bom_revision=MAX(bom_revision,released_revision)+1,version=version+1,updated_at=? WHERE id=? AND version=?`, now, req.ID, req.ExpectedVersion); err != nil {
			return nil, mapWriteError(err, "reopen project")
		}
	case (project.Status == "draft" || project.Status == "released") && wanted == "cancelled":
		if err := releaseProjectReservations(ctx, tx, req.RequestID, req.ID); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='cancelled',version=version+1,updated_at=? WHERE id=? AND version=?`, now, req.ID, req.ExpectedVersion); err != nil {
			return nil, mapWriteError(err, "cancel project")
		}
	case project.Status == "released" && wanted == "completed":
		var active int64
		if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE project_id=?`, req.ID).Scan(&active); err != nil {
			return nil, wrapError("unavailable", "check FindIt project reservations", err)
		}
		if active > 0 {
			return nil, domainError("conflict", "release or consume reservations before completing the project")
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET status='completed',version=version+1,updated_at=? WHERE id=? AND version=?`, now, req.ID, req.ExpectedVersion); err != nil {
			return nil, mapWriteError(err, "complete project")
		}
	default:
		return nil, domainError("conflict", "project status transition is not allowed")
	}
	return getProject(ctx, tx, req.ID)
}

func releaseProjectReservations(ctx context.Context, tx *sql.Tx, requestID, projectID string) error {
	rows, err := tx.QueryContext(ctx, `SELECT r.id,r.stock_id,r.line_id,r.quantity_micro,r.consumed_micro,r.released_micro,s.item_id,s.location_id,s.serial,s.lot,s.condition
		FROM reservations r JOIN stocks s ON s.id=r.stock_id WHERE r.project_id=? AND r.quantity_micro>r.consumed_micro+r.released_micro`, projectID)
	if err != nil {
		return wrapError("unavailable", "read FindIt reservations", err)
	}
	type pending struct {
		id, stockID, lineID, itemID, locationID, serial, lot, condition string
		quantity, consumed, released                                    int64
	}
	var items []pending
	for rows.Next() {
		var item pending
		if err := rows.Scan(&item.id, &item.stockID, &item.lineID, &item.quantity, &item.consumed, &item.released, &item.itemID, &item.locationID, &item.serial, &item.lot, &item.condition); err != nil {
			_ = rows.Close()
			return wrapError("unavailable", "read FindIt reservation", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return wrapError("unavailable", "read FindIt reservations", err)
	}
	if err := rows.Close(); err != nil {
		return wrapError("unavailable", "close FindIt reservation query", err)
	}
	for _, item := range items {
		remaining := item.quantity - item.consumed - item.released
		if _, err := tx.ExecContext(ctx, `UPDATE reservations SET released_micro=released_micro+? WHERE id=?`, remaining, item.id); err != nil {
			return mapWriteError(err, "release project reservation")
		}
		if err := insertMovement(ctx, tx, requestID, "release", item.itemID, item.stockID, "", item.locationID, remaining, projectID, item.id, item.serial, item.lot, item.condition, "project cancelled"); err != nil {
			return err
		}
	}
	return nil
}

func movementSource(kind, location string) string {
	if kind == "issue" || kind == "adjust" {
		return location
	}
	return ""
}

func movementTarget(kind, location string) string {
	if kind == "receive" || kind == "return" {
		return location
	}
	return ""
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
