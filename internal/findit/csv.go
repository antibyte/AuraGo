package findit

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	maxCSVBytes = 10 << 20
	maxCSVRows  = 25_000
)

var itemCSVFields = []string{
	"name", "description", "category", "template_id", "manufacturer", "part_number", "unit", "minimum_stock",
	"package_family", "pin_count", "pitch_mm", "length_mm", "width_mm", "height_mm", "package_size", "size_system",
	"tags", "datasheet_urls", "image_id", "icon_id", "track_serials", "properties",
}

var projectLineCSVFields = []string{"item_id", "manufacturer", "part_number", "quantity", "note"}

// PreviewCSV validates a CSV import without changing the database.
func (s *Store) PreviewCSV(ctx context.Context, req ImportRequest) (ImportPreview, error) {
	if s == nil || s.db == nil {
		return ImportPreview{}, ErrUnavailable
	}
	return readSnapshot(ctx, s.db, func(tx *sql.Tx) (ImportPreview, error) {
		return previewCSV(ctx, tx, req)
	})
}

func previewCSV(ctx context.Context, db queryDB, req ImportRequest) (ImportPreview, error) {
	parsed, err := parseCSV(req.CSV)
	if err != nil {
		return ImportPreview{}, err
	}
	allowed := itemCSVFields
	if req.Entity == "project_lines" {
		allowed = projectLineCSVFields
		if err := validateID(req.ProjectID, "project id", true); err != nil || req.ExpectedVersion <= 0 {
			return ImportPreview{}, domainError("invalid", "project line import requires project_id and expected_version")
		}
		project, err := getProject(ctx, db, req.ProjectID)
		if err != nil {
			return ImportPreview{}, err
		}
		if project.Status != "draft" || project.Version != req.ExpectedVersion {
			return ImportPreview{}, ErrConflict
		}
	} else if req.Entity != "items" {
		return ImportPreview{}, domainError("invalid", "CSV import entity must be items or project_lines")
	}
	mapping, err := normalizeCSVMapping(req.Mapping, parsed.headers, allowed)
	if err != nil {
		return ImportPreview{}, err
	}
	if len(parsed.records) > maxCSVRows {
		return ImportPreview{}, domainError("invalid", "CSV import contains too many rows")
	}
	knownRows := make(map[int]struct{}, len(parsed.records))
	for i := range parsed.records {
		knownRows[i+2] = struct{}{}
	}
	for _, row := range req.SkipRows {
		if _, exists := knownRows[row]; !exists {
			return ImportPreview{}, domainError("invalid", "skip_rows references an unknown CSV row")
		}
	}
	for row := range req.Resolutions {
		if _, exists := knownRows[row]; !exists {
			return ImportPreview{}, domainError("invalid", "resolutions references an unknown CSV row")
		}
	}

	preview := ImportPreview{Columns: append([]string{}, parsed.headers...), Rows: make([]ImportPreviewRow, 0, len(parsed.records))}
	seenParts := make(map[string]int)
	seenProjectItems := make(map[string]int)
	for index, record := range parsed.records {
		if err := ctx.Err(); err != nil {
			return ImportPreview{}, wrapError("unavailable", "preview FindIt CSV", err)
		}
		rowNumber := index + 2
		row := ImportPreviewRow{Row: rowNumber, Data: map[string]any{}, Errors: []CSVRowError{}}
		if len(record) != len(parsed.headers) {
			row.Errors = append(row.Errors, CSVRowError{Code: "column_count", Message: "CSV row has a different number of columns than the header"})
		}
		for canonical, header := range mapping {
			column := parsed.headerIndex[strings.ToLower(header)]
			if column < len(record) {
				row.Data[canonical] = strings.TrimSpace(record[column])
			} else {
				row.Data[canonical] = ""
			}
		}
		if req.Entity == "items" {
			input, err := itemInputFromCSV(row.Data)
			if err != nil {
				row.Errors = append(row.Errors, CSVRowError{Code: "invalid_data", Message: err.Error()})
			} else {
				item, _, err := normalizeItem(ctx, db, input)
				if err != nil {
					row.Errors = append(row.Errors, CSVRowError{Code: ErrorCode(err), Message: err.Error()})
				} else if item.PartNumber != "" {
					key := duplicatePartKey(item.Manufacturer, item.PartNumber)
					var existingID string
					err := db.QueryRowContext(ctx, `SELECT id FROM items WHERE archived=0 AND lower(trim(manufacturer))=? AND lower(trim(part_number))=?`, strings.ToLower(item.Manufacturer), strings.ToLower(item.PartNumber)).Scan(&existingID)
					if err == nil {
						row.Candidates = []ItemCandidate{{ID: existingID, Name: item.Name, Manufacturer: item.Manufacturer, PartNumber: item.PartNumber}}
						row.Errors = append(row.Errors, CSVRowError{Field: "part_number", Code: "duplicate_item", Message: "This manufacturer and part number already exists; mark this row to skip"})
					} else if !errors.Is(err, sql.ErrNoRows) {
						return ImportPreview{}, wrapError("unavailable", "check FindIt CSV duplicates", err)
					} else if first, duplicate := seenParts[key]; duplicate {
						row.Errors = append(row.Errors, CSVRowError{Field: "part_number", Code: "duplicate_item", Message: fmt.Sprintf("This part duplicates CSV row %d; mark this row to skip", first)})
					} else {
						seenParts[key] = rowNumber
					}
				}
			}
			if containsRow(req.SkipRows, rowNumber) {
				if hasRowError(row.Errors, "duplicate_item") {
					row.Errors = removeRowError(row.Errors, "duplicate_item")
					row.Skipped = true
				} else {
					row.Errors = append(row.Errors, CSVRowError{Code: "invalid_skip", Message: "Only duplicate item rows can be skipped"})
				}
			}
		} else {
			quantity, err := ParseQuantity(stringValue(row.Data["quantity"]))
			if err != nil || quantity <= 0 {
				row.Errors = append(row.Errors, CSVRowError{Field: "quantity", Code: "invalid_quantity", Message: "Quantity must be a positive decimal string"})
			}
			resolvedID, candidates, err := resolveCSVItem(ctx, db, row.Data, req.Resolutions[rowNumber])
			if err != nil {
				row.Errors = append(row.Errors, CSVRowError{Field: "item_id", Code: ErrorCode(err), Message: err.Error()})
			} else if resolvedID == "" {
				row.Candidates = candidates
				code, message := "item_not_found", "No matching item was found; choose an item for this row"
				if len(candidates) > 1 {
					code, message = "ambiguous_item", "Several matching items were found; choose one for this row"
				}
				row.Errors = append(row.Errors, CSVRowError{Field: "item_id", Code: code, Message: message})
			} else {
				row.ResolvedItemID = resolvedID
				if quantity, err := ParseQuantity(stringValue(row.Data["quantity"])); err == nil && quantity > 0 {
					item, err := getItemWithoutAvailability(ctx, db, resolvedID)
					if err != nil {
						return ImportPreview{}, err
					}
					if item.Unit == "piece" && quantity%quantityScale != 0 {
						row.Errors = append(row.Errors, CSVRowError{Field: "quantity", Code: "invalid_quantity", Message: "Piece quantities must be whole numbers"})
					}
				}
				if first, duplicate := seenProjectItems[resolvedID]; duplicate {
					row.Errors = append(row.Errors, CSVRowError{Field: "item_id", Code: "duplicate_item", Message: fmt.Sprintf("This BOM already contains the item from CSV row %d", first)})
				} else {
					seenProjectItems[resolvedID] = rowNumber
				}
			}
			if containsRow(req.SkipRows, rowNumber) {
				if hasRowError(row.Errors, "duplicate_item") {
					row.Errors = removeRowError(row.Errors, "duplicate_item")
					row.Skipped = true
				} else {
					row.Errors = append(row.Errors, CSVRowError{Code: "invalid_skip", Message: "Only duplicate BOM item rows can be skipped"})
				}
			}
		}
		if len(row.Errors) == 0 {
			if !row.Skipped {
				preview.ValidCount++
			}
		} else {
			preview.InvalidCount++
		}
		preview.Rows = append(preview.Rows, row)
	}
	if req.Entity == "project_lines" && preview.ValidCount == 0 {
		return ImportPreview{}, domainError("invalid", "project BOM import needs at least one row that will be applied")
	}
	if req.Entity == "project_lines" && preview.ValidCount > 1000 {
		return ImportPreview{}, domainError("invalid", "project BOM import contains more than 1000 lines")
	}
	return preview, nil
}

// ImportCSV atomically creates items or replaces a draft project's BOM lines.
func (s *Store) ImportCSV(ctx context.Context, req ImportRequest) (any, error) {
	if s == nil || s.db == nil {
		return nil, ErrUnavailable
	}
	if _, err := validateText(req.RequestID, "request_id", 128, true); err != nil {
		return nil, err
	}
	fingerprint, err := importFingerprint(req)
	if err != nil {
		return nil, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, wrapError("unavailable", "begin FindIt CSV import", err)
	}
	defer tx.Rollback()
	var oldFingerprint, oldResult string
	err = tx.QueryRowContext(ctx, `SELECT fingerprint,result_json FROM command_replays WHERE request_id=?`, req.RequestID).Scan(&oldFingerprint, &oldResult)
	if err == nil {
		if oldFingerprint != fingerprint {
			return nil, ErrConflict
		}
		return json.RawMessage(oldResult), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, wrapError("unavailable", "read FindIt CSV receipt", err)
	}
	preview, err := previewCSV(ctx, tx, req)
	if err != nil {
		return nil, err
	}
	if preview.InvalidCount > 0 {
		return nil, domainError("invalid", "CSV preview contains unresolved errors")
	}
	result := ImportResult{Entity: req.Entity, Created: []string{}, Skipped: []int{}}
	if req.Entity == "items" {
		for _, row := range preview.Rows {
			if row.Skipped {
				result.Skipped = append(result.Skipped, row.Row)
				continue
			}
			input, err := itemInputFromCSV(row.Data)
			if err != nil {
				return nil, err
			}
			item, properties, err := normalizeItem(ctx, tx, input)
			if err != nil {
				return nil, err
			}
			item.ID, item.Version = newID(), 1
			if err := insertItemTx(ctx, tx, item, properties, nowString()); err != nil {
				return nil, mapWriteError(err, "import item")
			}
			result.Created = append(result.Created, item.ID)
		}
	} else {
		project, err := getProject(ctx, tx, req.ProjectID)
		if err != nil {
			return nil, err
		}
		if project.Version != req.ExpectedVersion || project.Status != "draft" {
			return nil, ErrConflict
		}
		lines := make([]projectLineInput, 0, len(preview.Rows))
		for _, row := range preview.Rows {
			if row.Skipped {
				result.Skipped = append(result.Skipped, row.Row)
				continue
			}
			lines = append(lines, projectLineInput{ItemID: row.ResolvedItemID, Quantity: stringValue(row.Data["quantity"]), Note: stringValue(row.Data["note"])})
		}
		if _, err := tx.ExecContext(ctx, `UPDATE project_lines SET active=0 WHERE project_id=? AND active=1`, req.ProjectID); err != nil {
			return nil, wrapError("unavailable", "replace FindIt project BOM", err)
		}
		if err := replaceProjectLines(ctx, tx, req.ProjectID, lines); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE projects SET version=version+1,bom_revision=bom_revision+1,updated_at=? WHERE id=? AND version=? AND status='draft'`, nowString(), req.ProjectID, req.ExpectedVersion); err != nil {
			return nil, mapWriteError(err, "update project BOM")
		}
		result.ProjectID, result.Version = req.ProjectID, project.Version+1
	}
	encoded, err := json.Marshal(result)
	if err != nil {
		return nil, wrapError("unavailable", "serialize FindIt CSV result", err)
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO command_replays(request_id,fingerprint,result_json,created_at) VALUES(?,?,?,?)`, req.RequestID, fingerprint, string(encoded), nowString()); err != nil {
		return nil, mapWriteError(err, "record CSV receipt")
	}
	if err := commit(ctx, tx); err != nil {
		return nil, mapWriteError(err, "commit CSV import")
	}
	return result, nil
}

// ExportCSV writes a portable CSV snapshot for items, stock rows, or a project's BOM.
func (s *Store) ExportCSV(ctx context.Context, entity, projectID string) ([]byte, error) {
	if s == nil || s.db == nil {
		return nil, ErrUnavailable
	}
	return readSnapshot(ctx, s.db, func(tx *sql.Tx) ([]byte, error) {
		return exportCSV(ctx, tx, entity, projectID)
	})
}

func exportCSV(ctx context.Context, db queryDB, entity, projectID string) ([]byte, error) {
	var buffer strings.Builder
	writer := csv.NewWriter(&buffer)
	var header []string
	var rows [][]string
	switch entity {
	case "items":
		header = append([]string{}, itemCSVFields...)
		ids, err := collectIDsFromQuery(ctx, db, `SELECT id FROM items WHERE archived=0 ORDER BY name COLLATE NOCASE,id`)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			item, err := getItemWithoutAvailability(ctx, db, id)
			if err != nil {
				return nil, err
			}
			rows = append(rows, itemCSVRecord(item))
		}
	case "stocks":
		header = []string{"stock_id", "item_id", "location_id", "location_path", "quantity", "lot", "condition", "serial", "note", "version"}
		ids, err := collectIDsFromQuery(ctx, db, `SELECT id FROM stocks ORDER BY item_id,location_id,lot,serial,id`)
		if err != nil {
			return nil, err
		}
		for _, id := range ids {
			var row StockRecord
			var note string
			var quantity int64
			if err := db.QueryRowContext(ctx, `SELECT id,item_id,location_id,quantity_micro,lot,condition,serial,note,version FROM stocks WHERE id=?`, id).
				Scan(&row.ID, &row.ItemID, &row.LocationID, &quantity, &row.Lot, &row.Condition, &row.Serial, &note, &row.Version); err != nil {
				return nil, wrapError("unavailable", "read FindIt stock export", err)
			}
			location, err := getLocation(ctx, db, row.LocationID)
			if err != nil {
				return nil, err
			}
			rows = append(rows, []string{row.ID, row.ItemID, row.LocationID, strings.Join(location.Path, " / "), FormatQuantity(quantity), row.Lot, row.Condition, row.Serial, note, strconv.FormatInt(row.Version, 10)})
		}
	case "project_lines":
		if err := validateID(projectID, "project id", true); err != nil {
			return nil, err
		}
		project, err := getProject(ctx, db, projectID)
		if err != nil {
			return nil, err
		}
		_ = project
		header = append([]string{}, projectLineCSVFields...)
		lines, err := getProjectExportLines(ctx, db, projectID)
		if err != nil {
			return nil, err
		}
		rows = lines
	default:
		return nil, domainError("invalid", "CSV export entity must be items, stocks, or project_lines")
	}
	if err := writer.Write(header); err != nil {
		return nil, wrapError("unavailable", "write FindIt CSV header", err)
	}
	for _, row := range rows {
		if err := writer.Write(row); err != nil {
			return nil, wrapError("unavailable", "write FindIt CSV row", err)
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, wrapError("unavailable", "finish FindIt CSV export", err)
	}
	return []byte(buffer.String()), nil
}

type parsedCSV struct {
	headers     []string
	headerIndex map[string]int
	records     [][]string
}

func parseCSV(content string) (parsedCSV, error) {
	if len(content) == 0 || len(content) > maxCSVBytes || !utf8.ValidString(content) {
		return parsedCSV{}, domainError("invalid", "CSV must be non-empty valid UTF-8 and no larger than 10 MiB")
	}
	content = strings.TrimPrefix(content, "\uFEFF")
	parseWith := func(delimiter rune) (parsedCSV, error) {
		reader := csv.NewReader(strings.NewReader(content))
		reader.Comma = delimiter
		reader.FieldsPerRecord = -1
		reader.TrimLeadingSpace = true
		all, err := reader.ReadAll()
		if err != nil {
			return parsedCSV{}, domainError("invalid", "CSV syntax is invalid")
		}
		if len(all) < 2 || len(all[0]) == 0 {
			return parsedCSV{}, domainError("invalid", "CSV needs a header and at least one data row")
		}
		headers := make([]string, len(all[0]))
		index := make(map[string]int, len(all[0]))
		for i, value := range all[0] {
			value = strings.TrimSpace(value)
			if value == "" {
				return parsedCSV{}, domainError("invalid", "CSV header names cannot be blank")
			}
			key := strings.ToLower(value)
			if _, exists := index[key]; exists {
				return parsedCSV{}, domainError("invalid", "CSV header names must be unique")
			}
			headers[i] = value
			index[key] = i
		}
		return parsedCSV{headers: headers, headerIndex: index, records: all[1:]}, nil
	}
	return parseWith(detectCSVDelimiter(content))
}

func detectCSVDelimiter(content string) rune {
	line := strings.SplitN(content, "\n", 2)[0]
	quoted := false
	commas, semicolons := 0, 0
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			if quoted && i+1 < len(line) && line[i+1] == '"' {
				i++
			} else {
				quoted = !quoted
			}
		case ',':
			if !quoted {
				commas++
			}
		case ';':
			if !quoted {
				semicolons++
			}
		}
	}
	if semicolons > commas {
		return ';'
	}
	return ','
}

func normalizeCSVMapping(mapping map[string]string, headers []string, allowed []string) (map[string]string, error) {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, field := range allowed {
		allowedSet[field] = struct{}{}
	}
	index := make(map[string]string, len(headers))
	for _, header := range headers {
		index[strings.ToLower(header)] = header
	}
	result := make(map[string]string, len(allowed))
	for field, header := range mapping {
		if _, ok := allowedSet[field]; !ok {
			return nil, domainError("invalid", "CSV mapping contains an unsupported field")
		}
		canonicalHeader, ok := index[strings.ToLower(strings.TrimSpace(header))]
		if !ok {
			return nil, domainError("invalid", "CSV mapping references an unknown header")
		}
		result[field] = canonicalHeader
	}
	if mapping == nil {
		for _, field := range allowed {
			if header, ok := index[field]; ok {
				result[field] = header
			}
		}
	}
	return result, nil
}

func itemInputFromCSV(data map[string]any) (itemInput, error) {
	input := itemInput{
		Name: stringValue(data["name"]), Description: stringValue(data["description"]), Category: stringValue(data["category"]),
		TemplateID: stringValue(data["template_id"]), Manufacturer: stringValue(data["manufacturer"]), PartNumber: stringValue(data["part_number"]),
		Unit: stringValue(data["unit"]), MinimumStock: stringValue(data["minimum_stock"]), PackageFamily: stringValue(data["package_family"]),
		PackageSize: stringValue(data["package_size"]), SizeSystem: stringValue(data["size_system"]), ImageID: stringValue(data["image_id"]), IconID: stringValue(data["icon_id"]),
	}
	if value := stringValue(data["pin_count"]); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			return itemInput{}, domainError("invalid", "pin_count must be an integer")
		}
		input.PinCount = parsed
	}
	for key, target := range map[string]**float64{"pitch_mm": &input.PitchMM, "length_mm": &input.LengthMM, "width_mm": &input.WidthMM, "height_mm": &input.HeightMM} {
		value := stringValue(data[key])
		if value == "" {
			continue
		}
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) {
			return itemInput{}, domainError("invalid", key+" must be a finite number")
		}
		*target = &parsed
	}
	if value := stringValue(data["track_serials"]); value != "" {
		parsed, err := strconv.ParseBool(strings.ToLower(value))
		if err != nil {
			return itemInput{}, domainError("invalid", "track_serials must be true or false")
		}
		input.TrackSerials = parsed
	}
	if value := stringValue(data["tags"]); value != "" {
		if err := decodeCSVJSON(value, &input.Tags); err != nil {
			return itemInput{}, domainError("invalid", "tags must be a JSON string array")
		}
	}
	if value := stringValue(data["datasheet_urls"]); value != "" {
		if err := decodeCSVJSON(value, &input.DatasheetURLs); err != nil {
			return itemInput{}, domainError("invalid", "datasheet_urls must be a JSON string array")
		}
	}
	if value := stringValue(data["properties"]); value != "" {
		if err := decodeCSVJSON(value, &input.Properties); err != nil {
			return itemInput{}, domainError("invalid", "properties must be a JSON object")
		}
	}
	if input.Tags == nil {
		input.Tags = []string{}
	}
	if input.DatasheetURLs == nil {
		input.DatasheetURLs = []string{}
	}
	if input.Properties == nil {
		input.Properties = map[string]any{}
	}
	return input, nil
}

func decodeCSVJSON(value string, target any) error {
	decoder := json.NewDecoder(strings.NewReader(value))
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("extra JSON data")
	}
	return nil
}

func resolveCSVItem(ctx context.Context, db queryDB, data map[string]any, selectedID string) (string, []ItemCandidate, error) {
	if selectedID != "" {
		if err := validateID(selectedID, "resolved item id", true); err != nil {
			return "", nil, err
		}
		item, err := getItemWithoutAvailability(ctx, db, selectedID)
		if err != nil {
			return "", nil, err
		}
		if item.Archived {
			return "", nil, domainError("invalid", "resolved item is archived")
		}
		return item.ID, []ItemCandidate{{ID: item.ID, Name: item.Name, Manufacturer: item.Manufacturer, PartNumber: item.PartNumber}}, nil
	}
	itemID := strings.TrimSpace(stringValue(data["item_id"]))
	manufacturer := strings.TrimSpace(stringValue(data["manufacturer"]))
	partNumber := strings.TrimSpace(stringValue(data["part_number"]))
	if itemID != "" {
		if err := validateID(itemID, "item id", true); err != nil {
			return "", nil, err
		}
		item, err := getItemWithoutAvailability(ctx, db, itemID)
		if err != nil {
			return "", nil, err
		}
		if item.Archived {
			return "", nil, domainError("invalid", "project lines cannot reference archived items")
		}
		return item.ID, []ItemCandidate{{ID: item.ID, Name: item.Name, Manufacturer: item.Manufacturer, PartNumber: item.PartNumber}}, nil
	}
	if manufacturer == "" || partNumber == "" {
		return "", nil, nil
	}
	rows, err := db.QueryContext(ctx, `SELECT id,name,manufacturer,part_number FROM items WHERE archived=0 AND lower(trim(manufacturer))=? AND lower(trim(part_number))=? ORDER BY id LIMIT 2`, strings.ToLower(manufacturer), strings.ToLower(partNumber))
	if err != nil {
		return "", nil, wrapError("unavailable", "resolve FindIt CSV project item", err)
	}
	var candidates []ItemCandidate
	for rows.Next() {
		var item ItemCandidate
		if err := rows.Scan(&item.ID, &item.Name, &item.Manufacturer, &item.PartNumber); err != nil {
			_ = rows.Close()
			return "", nil, wrapError("unavailable", "read FindIt CSV project item", err)
		}
		candidates = append(candidates, item)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return "", nil, wrapError("unavailable", "read FindIt CSV project items", err)
	}
	if err := rows.Close(); err != nil {
		return "", nil, wrapError("unavailable", "close FindIt CSV project items", err)
	}
	if len(candidates) == 1 {
		return candidates[0].ID, candidates, nil
	}
	return "", candidates, nil
}

func importFingerprint(req ImportRequest) (string, error) {
	encoded, err := json.Marshal(req)
	if err != nil {
		return "", wrapError("invalid", "encode FindIt CSV import", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), nil
}

func itemCSVRecord(item Item) []string {
	return []string{
		item.Name, item.Description, item.Category, item.TemplateID, item.Manufacturer, item.PartNumber, item.Unit, item.MinimumStock,
		item.PackageFamily, strconv.Itoa(item.PinCount), optionalFloat(item.PitchMM), optionalFloat(item.LengthMM), optionalFloat(item.WidthMM), optionalFloat(item.HeightMM),
		item.PackageSize, item.SizeSystem, jsonString(item.Tags), jsonString(item.DatasheetURLs), item.ImageID, item.IconID,
		strconv.FormatBool(item.TrackSerials), jsonString(item.Properties),
	}
}

func optionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}

func jsonString(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(encoded)
}

func collectIDsFromQuery(ctx context.Context, db queryDB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt export", err)
	}
	return collectIDs(rows)
}

func getProjectExportLines(ctx context.Context, db queryDB, projectID string) ([][]string, error) {
	rows, err := db.QueryContext(ctx, `SELECT l.item_id,l.quantity_micro,l.note,i.manufacturer,i.part_number FROM project_lines l JOIN items i ON i.id=l.item_id
		WHERE l.project_id=? AND l.active=1 ORDER BY i.name COLLATE NOCASE,l.id`, projectID)
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt project BOM export", err)
	}
	var result [][]string
	for rows.Next() {
		var itemID, note, manufacturer, partNumber string
		var quantity int64
		if err := rows.Scan(&itemID, &quantity, &note, &manufacturer, &partNumber); err != nil {
			_ = rows.Close()
			return nil, wrapError("unavailable", "read FindIt project BOM export", err)
		}
		result = append(result, []string{itemID, manufacturer, partNumber, FormatQuantity(quantity), note})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, wrapError("unavailable", "read FindIt project BOM export", err)
	}
	if err := rows.Close(); err != nil {
		return nil, wrapError("unavailable", "close FindIt project BOM export", err)
	}
	return result, nil
}

func duplicatePartKey(manufacturer, partNumber string) string {
	return strings.ToLower(strings.TrimSpace(manufacturer)) + "\x00" + strings.ToLower(strings.TrimSpace(partNumber))
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	text, _ := value.(string)
	return text
}

func containsRow(values []int, row int) bool {
	for _, value := range values {
		if value == row {
			return true
		}
	}
	return false
}

func hasRowError(values []CSVRowError, code string) bool {
	for _, value := range values {
		if value.Code == code {
			return true
		}
	}
	return false
}

func removeRowError(values []CSVRowError, code string) []CSVRowError {
	out := values[:0]
	for _, value := range values {
		if value.Code != code {
			out = append(out, value)
		}
	}
	return out
}
