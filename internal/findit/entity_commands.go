package findit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

func (s *Store) createItem(ctx context.Context, input itemInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt item creation", err)
	}
	defer tx.Rollback()
	result, err := createItemTx(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "create item")
	}
	return result, nil
}

func createItemTx(ctx context.Context, tx *sql.Tx, input itemInput) (any, error) {
	item, properties, err := normalizeItem(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	item.ID, item.Version = newID(), 1
	now := nowString()
	if err := insertItemTx(ctx, tx, item, properties, now); err != nil {
		return nil, mapWriteError(err, "create item")
	}
	return getItem(ctx, tx, item.ID)
}

func (s *Store) updateItem(ctx context.Context, id string, expected int64, input itemInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt item update", err)
	}
	defer tx.Rollback()
	result, err := updateItemTxCommand(ctx, tx, id, expected, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "update item")
	}
	return result, nil
}

func updateItemTxCommand(ctx context.Context, tx *sql.Tx, id string, expected int64, input itemInput) (any, error) {
	current, err := getItemWithoutAvailability(ctx, tx, id)
	if err != nil {
		return nil, err
	}
	if current.Archived || current.Version != expected {
		return nil, ErrConflict
	}
	item, properties, err := normalizeItemForUpdate(ctx, tx, input, current.TemplateID)
	if err != nil {
		return nil, err
	}
	if current.Unit != item.Unit || current.TrackSerials != item.TrackSerials {
		var stockCount int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM stocks WHERE item_id=?`, id).Scan(&stockCount); err != nil {
			return nil, wrapError("unavailable", "check FindIt stock unit", err)
		}
		if stockCount > 0 {
			return nil, domainError("conflict", "unit and serial tracking cannot change after stock has been recorded")
		}
	}
	item.ID, item.Version = id, current.Version+1
	if err := updateItemTx(ctx, tx, item, properties, current.Version, nowString()); err != nil {
		return nil, mapWriteError(err, "update item")
	}
	return getItem(ctx, tx, id)
}

func normalizeItem(ctx context.Context, db queryDB, input itemInput) (Item, map[string]any, error) {
	return normalizeItemWithArchivedTemplate(ctx, db, input, "")
}

func normalizeItemForUpdate(ctx context.Context, db queryDB, input itemInput, currentTemplateID string) (Item, map[string]any, error) {
	if strings.TrimSpace(input.TemplateID) == "" {
		input.TemplateID = currentTemplateID
	}
	return normalizeItemWithArchivedTemplate(ctx, db, input, currentTemplateID)
}

func normalizeItemWithArchivedTemplate(ctx context.Context, db queryDB, input itemInput, currentTemplateID string) (Item, map[string]any, error) {
	name, err := validateText(input.Name, "name", 256, true)
	if err != nil {
		return Item{}, nil, err
	}
	description, err := validateText(input.Description, "description", 8192, false)
	if err != nil {
		return Item{}, nil, err
	}
	templateID := strings.TrimSpace(input.TemplateID)
	if templateID == "" {
		templateID = "generic"
	}
	var tmpl Template
	if templateID == currentTemplateID && currentTemplateID != "" {
		tmpl, err = getTemplateIncludingArchived(ctx, db, templateID)
	} else {
		tmpl, err = getTemplate(ctx, db, templateID)
	}
	if err != nil {
		return Item{}, nil, err
	}
	category := strings.TrimSpace(input.Category)
	if category == "" {
		category = tmpl.Category
	}
	if !validCategory(category) {
		return Item{}, nil, domainError("invalid", "item category is invalid")
	}
	unit := strings.TrimSpace(input.Unit)
	if unit == "" {
		unit = "piece"
	}
	if !validUnit(unit) {
		return Item{}, nil, domainError("invalid", "item unit is invalid")
	}
	minimum := strings.TrimSpace(input.MinimumStock)
	if minimum == "" {
		minimum = "0"
	}
	minimumMicro, err := ParseQuantity(minimum)
	if err != nil || unit == "piece" && minimumMicro%quantityScale != 0 {
		return Item{}, nil, domainError("invalid", "minimum_stock is invalid for the selected unit")
	}
	if input.TrackSerials && unit != "piece" {
		return Item{}, nil, domainError("invalid", "serial tracking requires piece units")
	}
	if input.PinCount < 0 || input.PinCount > 100_000 {
		return Item{}, nil, domainError("invalid", "pin_count is outside the supported range")
	}
	for _, dimension := range []*float64{input.PitchMM, input.LengthMM, input.WidthMM, input.HeightMM} {
		if err := validateDimension(dimension); err != nil {
			return Item{}, nil, err
		}
	}
	packageFamily, err := validateText(input.PackageFamily, "package family", 128, false)
	if err != nil {
		return Item{}, nil, err
	}
	packageSize, err := validateText(input.PackageSize, "package size", 64, false)
	if err != nil {
		return Item{}, nil, err
	}
	sizeSystem := strings.TrimSpace(strings.ToLower(input.SizeSystem))
	if sizeSystem != "" && sizeSystem != "metric" && sizeSystem != "imperial" {
		return Item{}, nil, domainError("invalid", "size_system must be metric or imperial")
	}
	if packageSize != "" && sizeSystem == "" {
		return Item{}, nil, domainError("invalid", "package_size requires size_system")
	}
	manufacturer, err := validateText(input.Manufacturer, "manufacturer", 256, false)
	if err != nil {
		return Item{}, nil, err
	}
	partNumber, err := validateText(input.PartNumber, "part number", 256, false)
	if err != nil {
		return Item{}, nil, err
	}
	tags, err := validateTags(input.Tags)
	if err != nil {
		return Item{}, nil, err
	}
	datasheets, err := validateURLs(input.DatasheetURLs)
	if err != nil {
		return Item{}, nil, err
	}
	imageID, err := validateAssetID(input.ImageID, "image_id")
	if err != nil {
		return Item{}, nil, err
	}
	iconID, err := validateAssetID(input.IconID, "icon_id")
	if err != nil {
		return Item{}, nil, err
	}
	properties, err := validateProperties(tmpl.Fields, input.Properties)
	if err != nil {
		return Item{}, nil, err
	}
	return Item{
		Name: name, Description: description, Category: category, TemplateID: templateID,
		Manufacturer: manufacturer, PartNumber: partNumber, Unit: unit, MinimumStock: FormatQuantity(minimumMicro),
		PackageFamily: packageFamily, PinCount: input.PinCount, PitchMM: input.PitchMM,
		LengthMM: input.LengthMM, WidthMM: input.WidthMM, HeightMM: input.HeightMM,
		PackageSize: packageSize, SizeSystem: sizeSystem, Tags: tags, DatasheetURLs: datasheets,
		ImageID: imageID, IconID: iconID, TrackSerials: input.TrackSerials, Properties: properties,
	}, properties, nil
}

func getTemplateIncludingArchived(ctx context.Context, db queryDB, id string) (Template, error) {
	var item Template
	var fields string
	var builtIn int
	err := db.QueryRowContext(ctx, `SELECT id,version,name,description,category,built_in,fields_json FROM templates WHERE id=?`, id).
		Scan(&item.ID, &item.Version, &item.Name, &item.Description, &item.Category, &builtIn, &fields)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	if err != nil {
		return Template{}, wrapError("unavailable", "read FindIt template", err)
	}
	item.BuiltIn = builtIn != 0
	item.IconID = item.ID
	if err := json.Unmarshal([]byte(fields), &item.Fields); err != nil {
		return Template{}, wrapError("unavailable", "decode FindIt template", err)
	}
	if item.Fields == nil {
		item.Fields = []PropertyDefinition{}
	}
	return item, nil
}

type queryDB interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func insertItemTx(ctx context.Context, tx *sql.Tx, item Item, properties map[string]any, now string) error {
	minimum, _ := ParseQuantity(item.MinimumStock)
	tags, _ := json.Marshal(item.Tags)
	datasheets, _ := json.Marshal(item.DatasheetURLs)
	props, _ := json.Marshal(properties)
	_, err := tx.ExecContext(ctx, `INSERT INTO items(id,version,name,description,category,template_id,manufacturer,part_number,unit,minimum_stock_micro,
		package_family,pin_count,pitch_mm,length_mm,width_mm,height_mm,package_size,size_system,tags_json,datasheet_urls_json,image_id,icon_id,track_serials,properties_json,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, item.ID, item.Version, item.Name, item.Description, item.Category,
		item.TemplateID, item.Manufacturer, item.PartNumber, item.Unit, minimum, item.PackageFamily, item.PinCount,
		item.PitchMM, item.LengthMM, item.WidthMM, item.HeightMM, item.PackageSize, item.SizeSystem, string(tags),
		string(datasheets), item.ImageID, item.IconID, boolInt(item.TrackSerials), string(props), now, now)
	if err != nil {
		return err
	}
	if err := writeProperties(ctx, tx, item.ID, properties); err != nil {
		return err
	}
	return refreshItemFTS(ctx, tx, item.ID)
}

func updateItemTx(ctx context.Context, tx *sql.Tx, item Item, properties map[string]any, expected int64, now string) error {
	minimum, _ := ParseQuantity(item.MinimumStock)
	tags, _ := json.Marshal(item.Tags)
	datasheets, _ := json.Marshal(item.DatasheetURLs)
	props, _ := json.Marshal(properties)
	result, err := tx.ExecContext(ctx, `UPDATE items SET version=?,name=?,description=?,category=?,template_id=?,manufacturer=?,part_number=?,unit=?,minimum_stock_micro=?,
		package_family=?,pin_count=?,pitch_mm=?,length_mm=?,width_mm=?,height_mm=?,package_size=?,size_system=?,tags_json=?,datasheet_urls_json=?,image_id=?,icon_id=?,track_serials=?,properties_json=?,updated_at=?
		WHERE id=? AND version=? AND archived=0`, item.Version, item.Name, item.Description, item.Category, item.TemplateID,
		item.Manufacturer, item.PartNumber, item.Unit, minimum, item.PackageFamily, item.PinCount, item.PitchMM,
		item.LengthMM, item.WidthMM, item.HeightMM, item.PackageSize, item.SizeSystem, string(tags), string(datasheets),
		item.ImageID, item.IconID, boolInt(item.TrackSerials), string(props), now, item.ID, expected)
	if err != nil {
		return err
	}
	changed, err := result.RowsAffected()
	if err != nil || changed != 1 {
		if err != nil {
			return err
		}
		return ErrConflict
	}
	if err := writeProperties(ctx, tx, item.ID, properties); err != nil {
		return err
	}
	return refreshItemFTS(ctx, tx, item.ID)
}

func writeProperties(ctx context.Context, tx *sql.Tx, itemID string, properties map[string]any) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM item_properties WHERE item_id=?`, itemID); err != nil {
		return err
	}
	keys := make([]string, 0, len(properties))
	for key := range properties {
		keys = append(keys, key)
	}
	for _, key := range keys {
		value := properties[key]
		var kind string
		var textValue any
		var numberValue any
		var boolValue any
		switch v := value.(type) {
		case dateValue:
			kind, textValue = "date", string(v)
		case string:
			kind, textValue = "text", v
		case json.Number:
			n, err := strconv.ParseFloat(string(v), 64)
			if err != nil || math.IsInf(n, 0) || math.IsNaN(n) {
				return domainError("invalid", "numeric property value is invalid")
			}
			kind, numberValue = "number", n
		case bool:
			kind, boolValue = "boolean", boolInt(v)
		default:
			return domainError("invalid", "property value is invalid")
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO item_properties(item_id,field_key,value_type,text_value,number_value,bool_value) VALUES(?,?,?,?,?,?)`,
			itemID, key, kind, textValue, numberValue, boolValue); err != nil {
			return err
		}
	}
	return nil
}

func refreshItemFTS(ctx context.Context, tx *sql.Tx, id string) error {
	item, err := getItemWithoutAvailability(ctx, tx, id)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM findit_items_fts WHERE item_id=?`, id); err != nil {
		return err
	}
	if item.Archived {
		return nil
	}
	properties := make([]string, 0, len(item.Properties)*2)
	for key, value := range item.Properties {
		properties = append(properties, key, fmt.Sprint(value))
		if num, err := parseNumeric(value); err == nil {
			properties = append(properties, engineeringAliases(num)...)
		}
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO findit_items_fts(item_id,name,description,manufacturer,part_number,category,package_family,tags,properties)
		VALUES(?,?,?,?,?,?,?,?,?)`, item.ID, item.Name, item.Description, item.Manufacturer, item.PartNumber,
		item.Category, item.PackageFamily, strings.Join(item.Tags, " "), strings.Join(properties, " "))
	return err
}

func (s *Store) createLocation(ctx context.Context, input locationInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt location creation", err)
	}
	defer tx.Rollback()
	result, err := createLocationTx(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "create location")
	}
	return result, nil
}

func createLocationTx(ctx context.Context, tx *sql.Tx, input locationInput) (any, error) {
	location, err := normalizeLocation(input)
	if err != nil {
		return nil, err
	}
	location.ID, location.Version = newID(), 1
	now := nowString()
	if err := validateLocationParent(ctx, tx, location.ID, location.ParentID); err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO locations(id,version,parent_id,name,kind,description,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)`,
		location.ID, location.Version, nullableText(location.ParentID), location.Name, location.Kind, location.Description, now, now)
	if err != nil {
		return nil, mapWriteError(err, "create location")
	}
	return getLocation(ctx, tx, location.ID)
}

func normalizeLocation(input locationInput) (Location, error) {
	name, err := validateText(input.Name, "location name", 128, true)
	if err != nil {
		return Location{}, err
	}
	kind := strings.TrimSpace(strings.ToLower(input.Kind))
	if kind == "" {
		kind = "box"
	}
	if !oneOf(kind, "site", "room", "cabinet", "shelf", "drawer", "box", "bin", "rack", "other") {
		return Location{}, domainError("invalid", "location kind is invalid")
	}
	parent := strings.TrimSpace(input.ParentID)
	if err := validateID(parent, "parent id", false); err != nil {
		return Location{}, err
	}
	description, err := validateText(input.Description, "description", 2048, false)
	if err != nil {
		return Location{}, err
	}
	return Location{Name: name, Kind: kind, ParentID: parent, Description: description}, nil
}

func validateLocationParent(ctx context.Context, tx *sql.Tx, id, parent string) error {
	if parent == "" {
		return nil
	}
	var found int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM locations WHERE id=? AND archived=0`, parent).Scan(&found); err != nil {
		return wrapError("unavailable", "check FindIt location parent", err)
	}
	if found != 1 || id == parent {
		return domainError("invalid", "location parent is missing or invalid")
	}
	if id != "" {
		var cycle int
		if err := tx.QueryRowContext(ctx, `WITH RECURSIVE ancestors(id,parent_id) AS (
			SELECT id,parent_id FROM locations WHERE id=? UNION ALL
			SELECT l.id,l.parent_id FROM locations l JOIN ancestors a ON l.id=a.parent_id
		) SELECT count(*) FROM ancestors WHERE id=?`, parent, id).Scan(&cycle); err != nil {
			return wrapError("unavailable", "check FindIt location hierarchy", err)
		}
		if cycle > 0 {
			return domainError("invalid", "location hierarchy cannot contain a cycle")
		}
	}
	return nil
}

func (s *Store) updateLocation(ctx context.Context, id string, expected int64, input locationInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt location update", err)
	}
	defer tx.Rollback()
	result, err := updateLocationTxCommand(ctx, tx, id, expected, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "update location")
	}
	return result, nil
}

func updateLocationTxCommand(ctx context.Context, tx *sql.Tx, id string, expected int64, input locationInput) (any, error) {
	location, err := normalizeLocation(input)
	if err != nil {
		return nil, err
	}
	var version int64
	var archived int
	if err := tx.QueryRowContext(ctx, `SELECT version,archived FROM locations WHERE id=?`, id).Scan(&version, &archived); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt location", err)
	}
	if archived != 0 || version != expected {
		return nil, ErrConflict
	}
	if err := validateLocationParent(ctx, tx, id, location.ParentID); err != nil {
		return nil, err
	}
	location.ID, location.Version = id, version+1
	_, err = tx.ExecContext(ctx, `UPDATE locations SET version=?,parent_id=?,name=?,kind=?,description=?,updated_at=? WHERE id=? AND version=? AND archived=0`,
		location.Version, nullableText(location.ParentID), location.Name, location.Kind, location.Description, nowString(), id, expected)
	if err != nil {
		return nil, mapWriteError(err, "update location")
	}
	return getLocation(ctx, tx, id)
}

func (s *Store) createProject(ctx context.Context, input projectInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt project creation", err)
	}
	defer tx.Rollback()
	result, err := createProjectTx(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "create project")
	}
	return result, nil
}

func createProjectTx(ctx context.Context, tx *sql.Tx, input projectInput) (any, error) {
	name, err := validateText(input.Name, "project name", 256, true)
	if err != nil {
		return nil, err
	}
	description, err := validateText(input.Description, "description", 8192, false)
	if err != nil {
		return nil, err
	}
	status := strings.TrimSpace(input.Status)
	if status == "" {
		status = "draft"
	}
	if status != "draft" {
		return nil, domainError("invalid", "new projects must start in draft status")
	}
	project := Project{ID: newID(), Version: 1, Name: name, Description: description, Status: "draft", BOMRevision: 1, Lines: []ProjectLine{}}
	now := nowString()
	if _, err := tx.ExecContext(ctx, `INSERT INTO projects(id,version,name,description,status,bom_revision,created_at,updated_at) VALUES(?,1,?,?,'draft',1,?,?)`,
		project.ID, project.Name, project.Description, now, now); err != nil {
		return nil, mapWriteError(err, "create project")
	}
	if err := replaceProjectLines(ctx, tx, project.ID, input.Lines); err != nil {
		return nil, err
	}
	return getProject(ctx, tx, project.ID)
}

func (s *Store) updateProject(ctx context.Context, id string, expected int64, input projectInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt project update", err)
	}
	defer tx.Rollback()
	result, err := updateProjectTxCommand(ctx, tx, id, expected, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "update project")
	}
	return result, nil
}

func updateProjectTxCommand(ctx context.Context, tx *sql.Tx, id string, expected int64, input projectInput) (any, error) {
	name, err := validateText(input.Name, "project name", 256, true)
	if err != nil {
		return nil, err
	}
	description, err := validateText(input.Description, "description", 8192, false)
	if err != nil {
		return nil, err
	}
	var version, revision int64
	var status string
	if err := tx.QueryRowContext(ctx, `SELECT version,bom_revision,status FROM projects WHERE id=? AND archived=0`, id).Scan(&version, &revision, &status); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt project", err)
	}
	if version != expected || status != "draft" {
		return nil, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE project_lines SET active=0 WHERE project_id=? AND active=1`, id); err != nil {
		return nil, wrapError("unavailable", "replace FindIt project lines", err)
	}
	if err := replaceProjectLines(ctx, tx, id, input.Lines); err != nil {
		return nil, err
	}
	revision++
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET version=?,name=?,description=?,bom_revision=?,updated_at=? WHERE id=? AND version=?`,
		version+1, name, description, revision, nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "update project")
	}
	return getProject(ctx, tx, id)
}

func replaceProjectLines(ctx context.Context, tx *sql.Tx, projectID string, lines []projectLineInput) error {
	if len(lines) > 1000 {
		return domainError("invalid", "project has too many BOM lines")
	}
	seenItems := make(map[string]bool, len(lines))
	seenLineIDs := make(map[string]bool, len(lines))
	for _, line := range lines {
		if err := validateID(line.ItemID, "item id", true); err != nil {
			return err
		}
		if seenItems[line.ItemID] {
			return domainError("invalid", "a project BOM cannot repeat an item")
		}
		seenItems[line.ItemID] = true
		qty, err := ParseQuantity(line.Quantity)
		if err != nil || qty <= 0 {
			return domainError("invalid", "BOM quantity must be positive")
		}
		item, err := getItemWithoutAvailability(ctx, tx, line.ItemID)
		if err != nil {
			return err
		}
		if item.Archived || item.Unit == "piece" && qty%quantityScale != 0 {
			return domainError("invalid", "BOM quantity is invalid for the item")
		}
		lineID := strings.TrimSpace(line.ID)
		if lineID == "" {
			lineID = newID()
		} else if err := validateID(lineID, "project line id", true); err != nil {
			return err
		}
		if seenLineIDs[lineID] {
			return domainError("invalid", "a project BOM cannot repeat a line id")
		}
		seenLineIDs[lineID] = true
		note, err := validateText(line.Note, "BOM note", 2048, false)
		if err != nil {
			return err
		}
		var existingProject string
		findErr := tx.QueryRowContext(ctx, `SELECT project_id FROM project_lines WHERE id=?`, lineID).Scan(&existingProject)
		if findErr == nil {
			if existingProject != projectID {
				return ErrConflict
			}
			if _, err := tx.ExecContext(ctx, `UPDATE project_lines SET item_id=?,quantity_micro=?,note=?,active=1 WHERE id=? AND project_id=?`, line.ItemID, qty, note, lineID, projectID); err != nil {
				return mapWriteError(err, "update project BOM line")
			}
			continue
		}
		if !errors.Is(findErr, sql.ErrNoRows) {
			return wrapError("unavailable", "read FindIt project line", findErr)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO project_lines(id,project_id,item_id,quantity_micro,note) VALUES(?,?,?,?,?)`, lineID, projectID, line.ItemID, qty, note); err != nil {
			return mapWriteError(err, "save project BOM line")
		}
	}
	return nil
}

func (s *Store) createTemplate(ctx context.Context, input templateInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt template creation", err)
	}
	defer tx.Rollback()
	result, err := createTemplateTx(ctx, tx, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "create template")
	}
	return result, nil
}

func createTemplateTx(ctx context.Context, tx *sql.Tx, input templateInput) (any, error) {
	tmpl, err := normalizeTemplate(input)
	if err != nil {
		return nil, err
	}
	tmpl.ID = "custom-" + newID()
	tmpl.Version, tmpl.BuiltIn = 1, false
	tmpl.IconID = tmpl.ID
	now := nowString()
	fields, _ := json.Marshal(tmpl.Fields)
	if _, err := tx.ExecContext(ctx, `INSERT INTO templates(id,version,name,description,category,built_in,fields_json,created_at,updated_at) VALUES(?,?,?,?,?,0,?,?,?)`,
		tmpl.ID, tmpl.Version, tmpl.Name, tmpl.Description, tmpl.Category, string(fields), now, now); err != nil {
		return nil, mapWriteError(err, "create template")
	}
	return getTemplate(ctx, tx, tmpl.ID)
}

func normalizeTemplate(input templateInput) (Template, error) {
	name, err := validateText(input.Name, "template name", 128, true)
	if err != nil {
		return Template{}, err
	}
	description, err := validateText(input.Description, "description", 2048, false)
	if err != nil {
		return Template{}, err
	}
	category := strings.TrimSpace(input.Category)
	if !validCategory(category) {
		return Template{}, domainError("invalid", "template category is invalid")
	}
	fields, err := validateTemplateFields(input.Fields)
	if err != nil {
		return Template{}, err
	}
	return Template{Name: name, Description: description, Category: category, Fields: fields}, nil
}

func (s *Store) updateTemplate(ctx context.Context, id string, expected int64, input templateInput) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt template update", err)
	}
	defer tx.Rollback()
	result, err := updateTemplateTxCommand(ctx, tx, id, expected, input)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "update template")
	}
	return result, nil
}

func updateTemplateTxCommand(ctx context.Context, tx *sql.Tx, id string, expected int64, input templateInput) (any, error) {
	tmpl, err := normalizeTemplate(input)
	if err != nil {
		return nil, err
	}
	var version int64
	var builtIn int
	var currentFields, currentCategory string
	if err := tx.QueryRowContext(ctx, `SELECT version,built_in,fields_json,category FROM templates WHERE id=? AND archived=0`, id).Scan(&version, &builtIn, &currentFields, &currentCategory); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt template", err)
	}
	if builtIn != 0 || version != expected {
		return nil, ErrConflict
	}
	newFields, _ := json.Marshal(tmpl.Fields)
	var used int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM items WHERE template_id=?`, id).Scan(&used); err != nil {
		return nil, wrapError("unavailable", "check FindIt template use", err)
	}
	if used > 0 && string(newFields) != currentFields {
		var oldDefinitions []PropertyDefinition
		if err := json.Unmarshal([]byte(currentFields), &oldDefinitions); err != nil {
			return nil, wrapError("unavailable", "decode FindIt template fields", err)
		}
		if !compatibleTemplateChange(oldDefinitions, tmpl.Fields) {
			return nil, domainError("conflict", "used templates only allow labels, descriptions, and additive optional fields")
		}
	}
	if used > 0 && tmpl.Category != currentCategory {
		return nil, domainError("conflict", "a used template cannot change category")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE templates SET version=?,name=?,description=?,category=?,fields_json=?,updated_at=? WHERE id=? AND version=? AND built_in=0`,
		version+1, tmpl.Name, tmpl.Description, tmpl.Category, string(newFields), nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "update template")
	}
	return getTemplate(ctx, tx, id)
}

func compatibleTemplateChange(previous, next []PropertyDefinition) bool {
	oldByKey := make(map[string]PropertyDefinition, len(previous))
	for _, field := range previous {
		oldByKey[field.Key] = field
	}
	newByKey := make(map[string]PropertyDefinition, len(next))
	for _, field := range next {
		newByKey[field.Key] = field
	}
	for _, oldField := range previous {
		newField, exists := newByKey[oldField.Key]
		if !exists || oldField.Type != newField.Type || oldField.Required != newField.Required || oldField.Unit != newField.Unit ||
			!sameOptionalFloat(oldField.Min, newField.Min) || !sameOptionalFloat(oldField.Max, newField.Max) {
			return false
		}
		if oldField.Type == "enum" {
			options := make(map[string]bool, len(newField.Options))
			for _, option := range newField.Options {
				options[option] = true
			}
			for _, option := range oldField.Options {
				if !options[option] {
					return false
				}
			}
		}
	}
	for _, field := range next {
		if _, existed := oldByKey[field.Key]; !existed && field.Required {
			return false
		}
	}
	return true
}

func sameOptionalFloat(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func (s *Store) archiveItem(ctx context.Context, id string, expected int64) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt item archive", err)
	}
	defer tx.Rollback()
	result, err := archiveItemTx(ctx, tx, id, expected)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "archive item")
	}
	return result, nil
}

func archiveItemTx(ctx context.Context, tx *sql.Tx, id string, expected int64) (any, error) {
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM items WHERE id=? AND archived=0`, id).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt item", err)
	}
	if version != expected {
		return nil, ErrConflict
	}
	var held int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE line_id IN (SELECT id FROM project_lines WHERE item_id=?)`, id).Scan(&held); err != nil {
		return nil, wrapError("unavailable", "check FindIt item reservations", err)
	}
	var onHand int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro),0) FROM stocks WHERE item_id=?`, id).Scan(&onHand); err != nil {
		return nil, wrapError("unavailable", "check FindIt item stock", err)
	}
	if held > 0 || onHand > 0 {
		return nil, domainError("conflict", "item with stock or reservations cannot be archived")
	}
	var activeBOMReferences int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM project_lines l JOIN projects p ON p.id=l.project_id
		WHERE l.item_id=? AND l.active=1 AND p.archived=0 AND p.status IN ('draft','released')`, id).Scan(&activeBOMReferences); err != nil {
		return nil, wrapError("unavailable", "check FindIt item project references", err)
	}
	if activeBOMReferences > 0 {
		return nil, domainError("conflict", "item referenced by a draft or released project BOM cannot be archived")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE items SET archived=1,version=version+1,updated_at=? WHERE id=? AND version=?`, nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "archive item")
	}
	if err := refreshItemFTS(ctx, tx, id); err != nil {
		return nil, wrapError("unavailable", "update FindIt search index", err)
	}
	return map[string]any{"id": id, "archived": true, "version": expected + 1}, nil
}

func (s *Store) archiveLocation(ctx context.Context, id string, expected int64) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt location archive", err)
	}
	defer tx.Rollback()
	result, err := archiveLocationTx(ctx, tx, id, expected)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "archive location")
	}
	return result, nil
}

func archiveLocationTx(ctx context.Context, tx *sql.Tx, id string, expected int64) (any, error) {
	var version int64
	if err := tx.QueryRowContext(ctx, `SELECT version FROM locations WHERE id=? AND archived=0`, id).Scan(&version); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt location", err)
	}
	if version != expected {
		return nil, ErrConflict
	}
	var used int
	if err := tx.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM locations WHERE parent_id=? AND archived=0) + (SELECT count(*) FROM stocks WHERE location_id=? AND quantity_micro>0)`, id, id).Scan(&used); err != nil {
		return nil, wrapError("unavailable", "check FindIt location use", err)
	}
	if used > 0 {
		return nil, domainError("conflict", "location with children or stock cannot be archived")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE locations SET archived=1,version=version+1,updated_at=? WHERE id=? AND version=?`, nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "archive location")
	}
	return map[string]any{"id": id, "archived": true, "version": expected + 1}, nil
}

func (s *Store) archiveProject(ctx context.Context, id string, expected int64) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt project archive", err)
	}
	defer tx.Rollback()
	result, err := archiveProjectTx(ctx, tx, id, expected)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "archive project")
	}
	return result, nil
}

func archiveProjectTx(ctx context.Context, tx *sql.Tx, id string, expected int64) (any, error) {
	var version int64
	var active int64
	if err := tx.QueryRowContext(ctx, `SELECT p.version,COALESCE((SELECT sum(quantity_micro-consumed_micro-released_micro) FROM reservations WHERE project_id=p.id),0) FROM projects p WHERE p.id=? AND p.archived=0`, id).Scan(&version, &active); errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	} else if err != nil {
		return nil, wrapError("unavailable", "read FindIt project", err)
	}
	if version != expected {
		return nil, ErrConflict
	}
	if active > 0 {
		return nil, domainError("conflict", "project with active reservations cannot be archived")
	}
	if _, err := tx.ExecContext(ctx, `UPDATE projects SET archived=1,version=version+1,updated_at=? WHERE id=? AND version=?`, nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "archive project")
	}
	return map[string]any{"id": id, "archived": true, "version": expected + 1}, nil
}

func (s *Store) archiveTemplate(ctx context.Context, id string, expected int64) (any, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt template archive", err)
	}
	defer tx.Rollback()
	result, err := archiveTemplateTx(ctx, tx, id, expected)
	if err != nil {
		return nil, err
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "archive template")
	}
	return result, nil
}

func archiveTemplateTx(ctx context.Context, tx *sql.Tx, id string, expected int64) (any, error) {
	var builtIn int
	var version int64
	err := tx.QueryRowContext(ctx, `SELECT version,built_in FROM templates WHERE id=? AND archived=0`, id).Scan(&version, &builtIn)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt template", err)
	}
	if builtIn != 0 || version != expected {
		return nil, ErrConflict
	}
	if _, err := tx.ExecContext(ctx, `UPDATE templates SET archived=1,version=version+1,updated_at=? WHERE id=? AND version=?`, nowString(), id, expected); err != nil {
		return nil, mapWriteError(err, "archive template")
	}
	return map[string]any{"id": id, "archived": true, "version": expected + 1}, nil
}
