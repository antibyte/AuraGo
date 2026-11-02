package findit

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"unicode"
)

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func (s *Store) Query(ctx context.Context, req QueryRequest) (any, error) {
	if s == nil || s.db == nil {
		return nil, domainError("unavailable", "FindIt is unavailable")
	}
	return readSnapshot(ctx, s.db, func(tx *sql.Tx) (any, error) {
		return s.query(ctx, tx, req)
	})
}

func (s *Store) query(ctx context.Context, db queryer, req QueryRequest) (any, error) {
	switch req.Operation {
	case "search":
		return s.search(ctx, db, req)
	case "get":
		if err := validateID(req.ID, "record id", true); err != nil {
			return nil, err
		}
		switch req.Entity {
		case "items":
			return getItemDetail(ctx, db, req.ID)
		case "locations":
			return getLocation(ctx, db, req.ID)
		case "projects":
			return getProjectDetail(ctx, db, req.ID)
		case "templates":
			return getTemplateRecord(ctx, db, req.ID)
		default:
			return nil, domainError("invalid", "unsupported FindIt entity")
		}
	case "availability":
		if err := validateID(req.ID, "item id", true); err != nil {
			return nil, err
		}
		return availability(ctx, db, req.ID)
	case "shortages":
		if err := validateID(req.ID, "project id", true); err != nil {
			return nil, err
		}
		return projectShortages(ctx, db, req.ID)
	default:
		return nil, domainError("invalid", "unsupported FindIt query")
	}
}

func readSnapshot[T any](ctx context.Context, db *sql.DB, read func(*sql.Tx) (T, error)) (T, error) {
	var zero T
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return zero, wrapError("unavailable", "begin FindIt read snapshot", err)
	}
	defer tx.Rollback()
	return read(tx)
}

func (s *Store) search(ctx context.Context, db queryer, req QueryRequest) (any, error) {
	limit, offset, err := pagination(req.Limit, req.Offset)
	if err != nil {
		return nil, err
	}
	if len(req.Query) > 512 {
		return nil, domainError("invalid", "search query is too long")
	}
	switch req.Entity {
	case "items":
		return searchItems(ctx, db, req, limit, offset)
	case "locations":
		return searchLocations(ctx, db, req, limit, offset)
	case "projects":
		return searchProjects(ctx, db, req, limit, offset)
	case "templates":
		return searchTemplates(ctx, db, req, limit, offset)
	default:
		return nil, domainError("invalid", "unsupported FindIt entity")
	}
}

func pagination(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultLimit
	}
	if limit < 1 || limit > maxLimit || offset < 0 || offset > 1_000_000 {
		return 0, 0, domainError("invalid", "pagination is outside the supported range")
	}
	return limit, offset, nil
}

func searchItems(ctx context.Context, db queryer, req QueryRequest, limit, offset int) (SearchResult, error) {
	where := []string{"i.archived=0"}
	args := []any{}
	match := BuildMatch(req.Query)
	if prefix, requested, err := queryIDPrefix(req.Query); err != nil {
		return SearchResult{}, err
	} else if requested {
		where = append(where, "i.id>=? AND i.id<?")
		args = append(args, prefix, prefix+"\uffff")
	} else if match != "" {
		textClause := "i.id IN (SELECT f.item_id FROM findit_items_fts f WHERE findit_items_fts MATCH ?)"
		if quantityClause, quantityArgs, ok := quantitySearchClause(req.Query); ok {
			where = append(where, "("+textClause+" OR "+quantityClause+")")
			// Numeric properties are compared exactly, without case-folding SI
			// prefixes or trusting stale derived aliases in older FTS rows.
			args = append(args, "- properties : ("+match+")")
			args = append(args, quantityArgs...)
		} else {
			where = append(where, textClause)
			args = append(args, match)
		}
	}
	if req.LocationID != "" {
		if err := validateID(req.LocationID, "location id", true); err != nil {
			return SearchResult{}, err
		}
		where = append(where, `i.id IN (WITH RECURSIVE descendants(id) AS (
			SELECT id FROM locations WHERE id=? AND archived=0
			UNION ALL SELECT child.id FROM locations child JOIN descendants d ON child.parent_id=d.id WHERE child.archived=0
		) SELECT DISTINCT sl.item_id FROM stocks sl JOIN descendants d ON d.id=sl.location_id WHERE sl.quantity_micro>0)`)
		args = append(args, req.LocationID)
	}
	if req.TemplateID != "" {
		if err := validateID(req.TemplateID, "template id", true); err != nil {
			return SearchResult{}, err
		}
		where = append(where, "i.template_id=?")
		args = append(args, req.TemplateID)
	}
	if req.Category != "" {
		if !validCategory(req.Category) {
			return SearchResult{}, domainError("invalid", "item category is invalid")
		}
		where = append(where, "i.category=?")
		args = append(args, req.Category)
	}
	if req.Package != "" {
		if _, err := validateText(req.Package, "package family", 128, true); err != nil {
			return SearchResult{}, err
		}
		where = append(where, "i.package_family=?")
		args = append(args, req.Package)
	}
	if len(req.Filters) > 8 {
		return SearchResult{}, domainError("invalid", "at most eight typed property filters are allowed")
	}
	for _, filter := range req.Filters {
		clause, filterArgs, err := propertyFilter(ctx, db, req.TemplateID, filter)
		if err != nil {
			return SearchResult{}, err
		}
		where = append(where, clause)
		args = append(args, filterArgs...)
	}
	if req.AvailableOnly {
		where = append(where, `(COALESCE((SELECT sum(s.quantity_micro) FROM stocks s WHERE s.item_id=i.id),0)-COALESCE((SELECT sum(r.quantity_micro-r.consumed_micro-r.released_micro) FROM reservations r JOIN stocks rs ON rs.id=r.stock_id WHERE rs.item_id=i.id),0))>0`)
	}
	if req.LowStock {
		where = append(where, `i.minimum_stock_micro>0 AND (
			COALESCE((SELECT sum(s.quantity_micro) FROM stocks s WHERE s.item_id=i.id),0)-
			COALESCE((SELECT sum(r.quantity_micro-r.consumed_micro-r.released_micro) FROM reservations r JOIN stocks rs ON rs.id=r.stock_id WHERE rs.item_id=i.id),0)
		)<i.minimum_stock_micro`)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM items i WHERE `+clause, args...).Scan(&total); err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt items", err)
	}
	queryArgs := append(append([]any{}, args...), limit, offset)
	rows, err := db.QueryContext(ctx, `SELECT i.id FROM items i WHERE `+clause+` ORDER BY i.name COLLATE NOCASE,i.id LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt items", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "read FindIt item search", err)
	}
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		item, err := getItem(ctx, db, id)
		if err != nil {
			return SearchResult{}, err
		}
		items = append(items, item)
	}
	return SearchResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func propertyFilter(ctx context.Context, db queryer, templateID string, filter PropertyFilter) (string, []any, error) {
	if len(filter.Field) == 0 || len(filter.Field) > 64 || !fieldKeyPattern.MatchString(filter.Field) || !oneOf(filter.Operator, "eq", "ne", "contains", "gt", "gte", "lt", "lte") {
		return "", nil, domainError("invalid", "property filter is invalid")
	}
	fields, err := templateFieldsForFilter(ctx, db, templateID, filter.Field)
	if err != nil {
		return "", nil, err
	}
	if len(fields) == 0 {
		return "", nil, domainError("invalid", "property filter field is not defined")
	}
	typeName := fields[0].Type
	for _, field := range fields[1:] {
		if field.Type != typeName || (typeName == "number" && field.Unit != fields[0].Unit) {
			return "", nil, domainError("invalid", "property filter field has inconsistent types or units; select a template")
		}
	}
	operator := map[string]string{"eq": "=", "ne": "<>", "gt": ">", "gte": ">=", "lt": "<", "lte": "<="}[filter.Operator]
	switch typeName {
	case "number":
		if operator == "" {
			return "", nil, domainError("invalid", "numeric properties require a comparison operator")
		}
		number, err := parseNumeric(filter.Value)
		if err != nil {
			return "", nil, err
		}
		return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.field_key=? AND ip.value_type='number' AND ip.number_value ` + operator + ` ?)`, []any{filter.Field, number}, nil
	case "text", "enum":
		if filter.Operator != "eq" && filter.Operator != "ne" && filter.Operator != "contains" {
			return "", nil, domainError("invalid", "text properties support eq, ne, or contains")
		}
		value, ok := filter.Value.(string)
		if !ok || len(value) > 2048 {
			return "", nil, domainError("invalid", "text property filter value is invalid")
		}
		if typeName == "enum" && filter.Operator == "contains" {
			return "", nil, domainError("invalid", "enum properties do not support contains")
		}
		if filter.Operator == "contains" {
			value = "%" + escapeLike(value) + "%"
			return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.field_key=? AND ip.value_type IN ('text','enum') AND lower(ip.text_value) LIKE lower(?) ESCAPE '\')`, []any{filter.Field, value}, nil
		}
		return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.field_key=? AND ip.value_type IN ('text','enum') AND ip.text_value ` + operator + ` ?)`, []any{filter.Field, value}, nil
	case "date":
		if !oneOf(filter.Operator, "eq", "ne", "gt", "gte", "lt", "lte") {
			return "", nil, domainError("invalid", "date properties support comparison filters")
		}
		value, ok := filter.Value.(string)
		if !ok || !validISODate(value) {
			return "", nil, detailedError("invalid", "invalid_date", filter.Field, "date filters must use a real YYYY-MM-DD date", nil)
		}
		return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.field_key=? AND ip.value_type='date' AND ip.text_value ` + operator + ` ?)`, []any{filter.Field, value}, nil
	case "boolean":
		if filter.Operator != "eq" && filter.Operator != "ne" {
			return "", nil, domainError("invalid", "boolean properties support eq or ne")
		}
		value, ok := filter.Value.(bool)
		if !ok {
			return "", nil, domainError("invalid", "boolean property filter value is invalid")
		}
		return `EXISTS(SELECT 1 FROM item_properties ip WHERE ip.item_id=i.id AND ip.field_key=? AND ip.value_type='boolean' AND ip.bool_value ` + operator + ` ?)`, []any{filter.Field, boolInt(value)}, nil
	default:
		return "", nil, domainError("invalid", "property filter field type is unsupported")
	}
}

func templateFieldsForFilter(ctx context.Context, db queryer, templateID, key string) ([]PropertyDefinition, error) {
	query := `SELECT fields_json FROM templates WHERE (archived=0 OR EXISTS(SELECT 1 FROM items WHERE items.template_id=templates.id))`
	args := []any{}
	if templateID != "" {
		query += ` AND id=?`
		args = append(args, templateID)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt template fields", err)
	}
	defer rows.Close()
	fields := []PropertyDefinition{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, wrapError("unavailable", "read FindIt template fields", err)
		}
		var items []PropertyDefinition
		if err := json.Unmarshal([]byte(raw), &items); err != nil {
			return nil, wrapError("unavailable", "decode FindIt template fields", err)
		}
		for _, field := range items {
			if field.Key == key {
				fields = append(fields, field)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, wrapError("unavailable", "read FindIt template fields", err)
	}
	return fields, nil
}

// Printed labels use an explicit bounded UUID prefix, never SQL wildcards.
func queryIDPrefix(query string) (string, bool, error) {
	if !strings.HasPrefix(strings.ToLower(query), "id:") {
		return "", false, nil
	}
	prefix := strings.ToLower(strings.TrimSpace(query[3:]))
	if len(prefix) < 8 || len(prefix) > 36 || strings.ContainsAny(prefix, "_%*?[]") || strings.Trim(prefix, "0123456789abcdef-") != "" {
		return "", true, domainError("invalid", "record identifier prefix is invalid")
	}
	return prefix, true, nil
}

func searchLocations(ctx context.Context, db queryer, req QueryRequest, limit, offset int) (SearchResult, error) {
	if len(req.Filters) > 0 || req.AvailableOnly || req.LowStock || req.TemplateID != "" || req.Category != "" || req.Package != "" {
		return SearchResult{}, domainError("invalid", "location search does not support item filters")
	}
	where := []string{"archived=0"}
	args := []any{}
	if req.LocationID != "" {
		where = append(where, "parent_id=?")
		args = append(args, req.LocationID)
	}
	if prefix, requested, err := queryIDPrefix(req.Query); err != nil {
		return SearchResult{}, err
	} else if requested {
		where = append(where, "id>=? AND id<?")
		args = append(args, prefix, prefix+"\uffff")
	} else if term := strings.TrimSpace(req.Query); term != "" {
		where = append(where, `(lower(name) LIKE lower(?) ESCAPE '\' OR lower(description) LIKE lower(?) ESCAPE '\')`)
		pattern := "%" + escapeLike(term) + "%"
		args = append(args, pattern, pattern)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM locations WHERE `+clause, args...).Scan(&total); err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt locations", err)
	}
	args = append(append([]any{}, args...), limit, offset)
	rows, err := db.QueryContext(ctx, `SELECT id FROM locations WHERE `+clause+` ORDER BY name COLLATE NOCASE,id LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt locations", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "read FindIt location search", err)
	}
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		item, err := getLocation(ctx, db, id)
		if err != nil {
			return SearchResult{}, err
		}
		items = append(items, item)
	}
	return SearchResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func searchProjects(ctx context.Context, db queryer, req QueryRequest, limit, offset int) (SearchResult, error) {
	if len(req.Filters) > 0 || req.AvailableOnly || req.LowStock || req.TemplateID != "" || req.Category != "" || req.Package != "" || req.LocationID != "" {
		return SearchResult{}, domainError("invalid", "project search does not support item filters")
	}
	where := []string{"archived=0"}
	args := []any{}
	if term := strings.TrimSpace(req.Query); term != "" {
		where = append(where, `(lower(name) LIKE lower(?) ESCAPE '\' OR lower(description) LIKE lower(?) ESCAPE '\')`)
		pattern := "%" + escapeLike(term) + "%"
		args = append(args, pattern, pattern)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM projects WHERE `+clause, args...).Scan(&total); err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt projects", err)
	}
	args = append(append([]any{}, args...), limit, offset)
	rows, err := db.QueryContext(ctx, `SELECT id FROM projects WHERE `+clause+` ORDER BY name COLLATE NOCASE,id LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt projects", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "read FindIt project search", err)
	}
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		item, err := getProject(ctx, db, id)
		if err != nil {
			return SearchResult{}, err
		}
		items = append(items, item)
	}
	return SearchResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func searchTemplates(ctx context.Context, db queryer, req QueryRequest, limit, offset int) (SearchResult, error) {
	if len(req.Filters) > 0 || req.AvailableOnly || req.LowStock || req.TemplateID != "" || req.Category != "" || req.Package != "" || req.LocationID != "" {
		return SearchResult{}, domainError("invalid", "template search does not support item filters")
	}
	where := []string{"archived=0"}
	args := []any{}
	if term := strings.TrimSpace(req.Query); term != "" {
		where = append(where, `(lower(name) LIKE lower(?) ESCAPE '\' OR lower(description) LIKE lower(?) ESCAPE '\' OR lower(category) LIKE lower(?) ESCAPE '\')`)
		pattern := "%" + escapeLike(term) + "%"
		args = append(args, pattern, pattern, pattern)
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM templates WHERE `+clause, args...).Scan(&total); err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt templates", err)
	}
	args = append(append([]any{}, args...), limit, offset)
	rows, err := db.QueryContext(ctx, `SELECT id FROM templates WHERE `+clause+` ORDER BY name COLLATE NOCASE,id LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "search FindIt templates", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return SearchResult{}, wrapError("unavailable", "read FindIt template search", err)
	}
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		item, err := getTemplate(ctx, db, id)
		if err != nil {
			return SearchResult{}, err
		}
		items = append(items, item)
	}
	return SearchResult{Items: items, Total: total, Limit: limit, Offset: offset}, nil
}

func getItem(ctx context.Context, db queryer, id string) (Item, error) {
	item, err := getItemWithoutAvailability(ctx, db, id)
	if err != nil {
		return Item{}, err
	}
	available, err := availability(ctx, db, id)
	if err != nil {
		return Item{}, err
	}
	item.Availability = Availability{OnHand: available.OnHand, Reserved: available.Reserved, Available: available.Available}
	item.StockLocations = []StockLocation{}
	seen := make(map[string]bool)
	for _, stock := range available.Stocks {
		if stock.Quantity != "0" && !seen[stock.LocationID] {
			item.StockLocations = append(item.StockLocations, StockLocation{ID: stock.LocationID, Path: stock.LocationPath})
			seen[stock.LocationID] = true
		}
	}
	return item, nil
}

func getItemDetail(ctx context.Context, db queryer, id string) (Item, error) {
	item, err := getItem(ctx, db, id)
	if err != nil {
		return Item{}, err
	}
	item.Movements, err = getMovements(ctx, db, id, 100)
	if err != nil {
		return Item{}, err
	}
	return item, nil
}

func getMovements(ctx context.Context, db queryer, itemID string, limit int) ([]MovementRecord, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,request_id,kind,item_id,stock_id,source_location_id,target_location_id,quantity_micro,
		project_id,reservation_id,serial,lot,condition,note,created_at FROM stock_movements WHERE item_id=? ORDER BY created_at DESC,id DESC LIMIT ?`, itemID, limit)
	if err != nil {
		return nil, wrapError("unavailable", "read FindIt stock movements", err)
	}
	items := make([]MovementRecord, 0, limit)
	type paths struct{ source, target string }
	var locationIDs []paths
	for rows.Next() {
		var item MovementRecord
		var source, target, project, reservation sql.NullString
		var quantity int64
		if err := rows.Scan(&item.ID, &item.RequestID, &item.Kind, &item.ItemID, &item.StockID, &source, &target, &quantity,
			&project, &reservation, &item.Serial, &item.Lot, &item.Condition, &item.Note, &item.CreatedAt); err != nil {
			_ = rows.Close()
			return nil, wrapError("unavailable", "read FindIt stock movement", err)
		}
		item.SourceLocationID, item.TargetLocationID = source.String, target.String
		item.ProjectID, item.ReservationID = project.String, reservation.String
		item.Quantity = formatSignedQuantity(quantity)
		items = append(items, item)
		locationIDs = append(locationIDs, paths{source.String, target.String})
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, wrapError("unavailable", "read FindIt stock movements", err)
	}
	if err := rows.Close(); err != nil {
		return nil, wrapError("unavailable", "close FindIt stock movements", err)
	}
	for index, path := range locationIDs {
		if path.source != "" {
			location, err := getLocation(ctx, db, path.source)
			if err != nil {
				return nil, err
			}
			items[index].SourceLocationPath = location.Path
		}
		if path.target != "" {
			location, err := getLocation(ctx, db, path.target)
			if err != nil {
				return nil, err
			}
			items[index].TargetLocationPath = location.Path
		}
	}
	return items, nil
}

func getItemWithoutAvailability(ctx context.Context, db queryer, id string) (Item, error) {
	var item Item
	var minimum int64
	var tags, datasheets, properties string
	var pitch, length, width, height sql.NullFloat64
	var track, archived int
	err := db.QueryRowContext(ctx, `SELECT id,version,name,description,category,template_id,manufacturer,part_number,unit,minimum_stock_micro,package_family,pin_count,
		pitch_mm,length_mm,width_mm,height_mm,package_size,size_system,tags_json,datasheet_urls_json,image_id,icon_id,track_serials,properties_json,archived,created_at,updated_at
		FROM items WHERE id=?`, id).Scan(&item.ID, &item.Version, &item.Name, &item.Description, &item.Category, &item.TemplateID, &item.Manufacturer, &item.PartNumber,
		&item.Unit, &minimum, &item.PackageFamily, &item.PinCount, &pitch, &length, &width, &height, &item.PackageSize, &item.SizeSystem,
		&tags, &datasheets, &item.ImageID, &item.IconID, &track, &properties, &archived, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, wrapError("unavailable", "read FindIt item", err)
	}
	item.MinimumStock, item.TrackSerials, item.Archived = FormatQuantity(minimum), track != 0, archived != 0
	if pitch.Valid {
		item.PitchMM = &pitch.Float64
	}
	if length.Valid {
		item.LengthMM = &length.Float64
	}
	if width.Valid {
		item.WidthMM = &width.Float64
	}
	if height.Valid {
		item.HeightMM = &height.Float64
	}
	if err := json.Unmarshal([]byte(tags), &item.Tags); err != nil {
		return Item{}, wrapError("unavailable", "decode FindIt item tags", err)
	}
	if err := json.Unmarshal([]byte(datasheets), &item.DatasheetURLs); err != nil {
		return Item{}, wrapError("unavailable", "decode FindIt datasheet links", err)
	}
	decoder := json.NewDecoder(strings.NewReader(properties))
	decoder.UseNumber()
	if err := decoder.Decode(&item.Properties); err != nil {
		return Item{}, wrapError("unavailable", "decode FindIt item properties", err)
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if item.DatasheetURLs == nil {
		item.DatasheetURLs = []string{}
	}
	if item.Properties == nil {
		item.Properties = map[string]any{}
	}
	return item, nil
}

func getLocation(ctx context.Context, db queryer, id string) (Location, error) {
	var item Location
	var parent sql.NullString
	var archived int
	err := db.QueryRowContext(ctx, `SELECT id,version,parent_id,name,kind,description,archived,created_at,updated_at FROM locations WHERE id=?`, id).
		Scan(&item.ID, &item.Version, &parent, &item.Name, &item.Kind, &item.Description, &archived, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Location{}, ErrNotFound
	}
	if err != nil {
		return Location{}, wrapError("unavailable", "read FindIt location", err)
	}
	item.ParentID, item.Archived = parent.String, archived != 0
	item.Path = []string{item.Name}
	current := item.ParentID
	for depth := 0; current != "" && depth < 64; depth++ {
		var name string
		var next sql.NullString
		if err := db.QueryRowContext(ctx, `SELECT name,parent_id FROM locations WHERE id=?`, current).Scan(&name, &next); err != nil {
			return Location{}, wrapError("unavailable", "read FindIt location path", err)
		}
		item.Path = append([]string{name}, item.Path...)
		current = next.String
	}
	if current != "" {
		return Location{}, domainError("unavailable", "FindIt location hierarchy exceeds its depth limit")
	}
	return item, nil
}

func getProject(ctx context.Context, db queryer, id string) (Project, error) {
	var item Project
	var archived int
	err := db.QueryRowContext(ctx, `SELECT id,version,name,description,status,bom_revision,released_revision,archived,created_at,updated_at FROM projects WHERE id=?`, id).
		Scan(&item.ID, &item.Version, &item.Name, &item.Description, &item.Status, &item.BOMRevision, &item.ReleasedRevision, &archived, &item.CreatedAt, &item.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrNotFound
	}
	if err != nil {
		return Project{}, wrapError("unavailable", "read FindIt project", err)
	}
	item.Archived = archived != 0
	rows, err := db.QueryContext(ctx, `SELECT l.id,l.item_id,i.name,i.part_number,i.unit,l.quantity_micro,
		COALESCE((SELECT sum(r.quantity_micro-r.consumed_micro-r.released_micro) FROM reservations r WHERE r.line_id=l.id),0),
		COALESCE((SELECT sum(r.consumed_micro) FROM reservations r WHERE r.line_id=l.id),0),l.note
		FROM project_lines l JOIN items i ON i.id=l.item_id WHERE l.project_id=? AND l.active=1 ORDER BY i.name COLLATE NOCASE,l.id`, id)
	if err != nil {
		return Project{}, wrapError("unavailable", "read FindIt project BOM", err)
	}
	item.Lines = []ProjectLine{}
	for rows.Next() {
		var line ProjectLine
		var quantity, reserved, consumed int64
		if err := rows.Scan(&line.ID, &line.ItemID, &line.ItemName, &line.PartNumber, &line.Unit, &quantity, &reserved, &consumed, &line.Note); err != nil {
			_ = rows.Close()
			return Project{}, wrapError("unavailable", "read FindIt project BOM", err)
		}
		line.Quantity, line.Reserved, line.Consumed = FormatQuantity(quantity), FormatQuantity(reserved), FormatQuantity(consumed)
		item.Lines = append(item.Lines, line)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return Project{}, wrapError("unavailable", "read FindIt project BOM", err)
	}
	if err := rows.Close(); err != nil {
		return Project{}, wrapError("unavailable", "close FindIt project BOM", err)
	}
	return item, nil
}

func getProjectDetail(ctx context.Context, db queryer, id string) (Project, error) {
	project, err := getProject(ctx, db, id)
	if err != nil {
		return Project{}, err
	}
	project.Reservations, project.ReservationsHasMore, err = getProjectReservations(ctx, db, id, 100)
	if err != nil {
		return Project{}, err
	}
	return project, nil
}

func getProjectReservations(ctx context.Context, db queryer, projectID string, historyLimit int) ([]Reservation, bool, error) {
	type itemWithLocation struct {
		reservation Reservation
		locationID  string
	}
	items := make([]itemWithLocation, 0, historyLimit+16)
	read := func(query string, args ...any) error {
		rows, err := db.QueryContext(ctx, query, args...)
		if err != nil {
			return wrapError("unavailable", "read FindIt project reservations", err)
		}
		for rows.Next() {
			var item itemWithLocation
			var quantity, consumed, released int64
			if err := rows.Scan(&item.reservation.ID, &item.reservation.ProjectID, &item.reservation.LineID, &item.reservation.StockID,
				&quantity, &consumed, &released, &item.locationID); err != nil {
				_ = rows.Close()
				return wrapError("unavailable", "read FindIt project reservation", err)
			}
			item.reservation.Quantity = FormatQuantity(quantity)
			item.reservation.Consumed = FormatQuantity(consumed)
			item.reservation.Released = FormatQuantity(released)
			item.reservation.Remaining = FormatQuantity(quantity - consumed - released)
			items = append(items, item)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return wrapError("unavailable", "read FindIt project reservations", err)
		}
		if err := rows.Close(); err != nil {
			return wrapError("unavailable", "close FindIt project reservations", err)
		}
		return nil
	}
	activeQuery := `SELECT r.id,r.project_id,r.line_id,r.stock_id,r.quantity_micro,r.consumed_micro,r.released_micro,s.location_id
		FROM reservations r JOIN stocks s ON s.id=r.stock_id WHERE r.project_id=? AND r.quantity_micro>r.consumed_micro+r.released_micro ORDER BY r.created_at,r.id`
	if err := read(activeQuery, projectID); err != nil {
		return nil, false, err
	}
	var historyCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM reservations WHERE project_id=? AND quantity_micro<=consumed_micro+released_micro`, projectID).Scan(&historyCount); err != nil {
		return nil, false, wrapError("unavailable", "count FindIt project reservation history", err)
	}
	historyQuery := `SELECT r.id,r.project_id,r.line_id,r.stock_id,r.quantity_micro,r.consumed_micro,r.released_micro,s.location_id
		FROM reservations r JOIN stocks s ON s.id=r.stock_id WHERE r.project_id=? AND r.quantity_micro<=r.consumed_micro+r.released_micro ORDER BY r.created_at DESC,r.id DESC LIMIT ?`
	if err := read(historyQuery, projectID, historyLimit); err != nil {
		return nil, false, err
	}
	result := make([]Reservation, 0, len(items))
	paths := make(map[string][]string)
	for _, item := range items {
		path, ok := paths[item.locationID]
		if !ok {
			location, err := getLocation(ctx, db, item.locationID)
			if err != nil {
				return nil, false, err
			}
			path = location.Path
			paths[item.locationID] = path
		}
		item.reservation.LocationPath = path
		result = append(result, item.reservation)
	}
	return result, historyCount > historyLimit, nil
}

func getReservation(ctx context.Context, db queryer, id string) (Reservation, error) {
	var result Reservation
	var quantity, consumed, released int64
	var locationID string
	err := db.QueryRowContext(ctx, `SELECT r.id,r.project_id,r.line_id,r.stock_id,r.quantity_micro,r.consumed_micro,r.released_micro,s.location_id
		FROM reservations r JOIN stocks s ON s.id=r.stock_id WHERE r.id=?`, id).Scan(&result.ID, &result.ProjectID, &result.LineID, &result.StockID,
		&quantity, &consumed, &released, &locationID)
	if errors.Is(err, sql.ErrNoRows) {
		return Reservation{}, ErrNotFound
	}
	if err != nil {
		return Reservation{}, wrapError("unavailable", "read FindIt reservation", err)
	}
	location, err := getLocation(ctx, db, locationID)
	if err != nil {
		return Reservation{}, err
	}
	result.LocationPath = location.Path
	result.Quantity, result.Consumed, result.Released = FormatQuantity(quantity), FormatQuantity(consumed), FormatQuantity(released)
	result.Remaining = FormatQuantity(quantity - consumed - released)
	return result, nil
}

func getTemplate(ctx context.Context, db queryer, id string) (Template, error) {
	item, err := getTemplateRecord(ctx, db, id)
	if err == nil && item.Archived {
		return Template{}, ErrNotFound
	}
	return item, err
}

// Historical reads retain archived definitions; assignments use getTemplate.
func getTemplateRecord(ctx context.Context, db queryer, id string) (Template, error) {
	var item Template
	var fields string
	var built, archived int
	err := db.QueryRowContext(ctx, `SELECT id,version,name,description,category,built_in,fields_json,archived FROM templates WHERE id=?`, id).
		Scan(&item.ID, &item.Version, &item.Name, &item.Description, &item.Category, &built, &fields, &archived)
	if errors.Is(err, sql.ErrNoRows) {
		return Template{}, ErrNotFound
	}
	if err != nil {
		return Template{}, wrapError("unavailable", "read FindIt template", err)
	}
	item.BuiltIn = built != 0
	item.Archived = archived != 0
	item.IconID = item.ID
	if err := json.Unmarshal([]byte(fields), &item.Fields); err != nil {
		return Template{}, wrapError("unavailable", "decode FindIt template", err)
	}
	if item.Fields == nil {
		item.Fields = []PropertyDefinition{}
	}
	return item, nil
}

func availability(ctx context.Context, db queryer, itemID string) (AvailabilityResult, error) {
	item, err := getItemWithoutAvailability(ctx, db, itemID)
	if err != nil {
		return AvailabilityResult{}, err
	}
	var onHand, reserved int64
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro),0),COALESCE((SELECT sum(r.quantity_micro-r.consumed_micro-r.released_micro) FROM reservations r JOIN stocks rs ON rs.id=r.stock_id WHERE rs.item_id=?),0) FROM stocks WHERE item_id=?`, itemID, itemID).Scan(&onHand, &reserved); err != nil {
		return AvailabilityResult{}, wrapError("unavailable", "read FindIt availability", err)
	}
	result := AvailabilityResult{ItemID: itemID, Unit: item.Unit, OnHand: FormatQuantity(onHand), Reserved: FormatQuantity(reserved), Available: FormatQuantity(onHand - reserved), Stocks: []StockRecord{}}
	rows, err := db.QueryContext(ctx, `SELECT id FROM stocks WHERE item_id=? ORDER BY location_id,lot,serial,id`, itemID)
	if err != nil {
		return AvailabilityResult{}, wrapError("unavailable", "read FindIt stock locations", err)
	}
	ids, err := collectIDs(rows)
	if err != nil {
		return AvailabilityResult{}, wrapError("unavailable", "read FindIt stock locations", err)
	}
	for _, id := range ids {
		var row StockRecord
		var quantity int64
		var locationName string
		if err := db.QueryRowContext(ctx, `SELECT s.id,s.item_id,s.location_id,l.name,s.quantity_micro,s.lot,s.condition,s.serial,s.version FROM stocks s JOIN locations l ON l.id=s.location_id WHERE s.id=?`, id).
			Scan(&row.ID, &row.ItemID, &row.LocationID, &locationName, &quantity, &row.Lot, &row.Condition, &row.Serial, &row.Version); err != nil {
			return AvailabilityResult{}, wrapError("unavailable", "read FindIt stock location", err)
		}
		row.Location, row.Quantity = locationName, FormatQuantity(quantity)
		location, err := getLocation(ctx, db, row.LocationID)
		if err != nil {
			return AvailabilityResult{}, err
		}
		row.LocationPath = location.Path
		var held int64
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(sum(quantity_micro-consumed_micro-released_micro),0) FROM reservations WHERE stock_id=?`, id).Scan(&held); err != nil {
			return AvailabilityResult{}, wrapError("unavailable", "read FindIt reservations", err)
		}
		row.Reserved, row.Available = FormatQuantity(held), FormatQuantity(quantity-held)
		result.Stocks = append(result.Stocks, row)
	}
	return result, nil
}

func projectShortages(ctx context.Context, db queryer, projectID string) (ShortagesResult, error) {
	project, err := getProject(ctx, db, projectID)
	if err != nil {
		return ShortagesResult{}, err
	}
	reservations, reservationsHasMore, err := getProjectReservations(ctx, db, projectID, 100)
	if err != nil {
		return ShortagesResult{}, err
	}
	result := ShortagesResult{Project: project, Lines: []ShortageLine{}, Reservations: reservations, ReservationsHasMore: reservationsHasMore}
	for _, line := range project.Lines {
		required, _ := ParseQuantity(line.Quantity)
		reserved, _ := ParseQuantity(line.Reserved)
		consumed, _ := ParseQuantity(line.Consumed)
		var onHand, held int64
		if err := db.QueryRowContext(ctx, `SELECT COALESCE(sum(s.quantity_micro),0),COALESCE((SELECT sum(r.quantity_micro-r.consumed_micro-r.released_micro) FROM reservations r JOIN stocks rs ON rs.id=r.stock_id WHERE rs.item_id=?),0) FROM stocks s WHERE s.item_id=?`, line.ItemID, line.ItemID).Scan(&onHand, &held); err != nil {
			return ShortagesResult{}, wrapError("unavailable", "read FindIt project availability", err)
		}
		available := onHand - held
		if available < 0 {
			available = 0
		}
		shortage := required - reserved - consumed - available
		if shortage < 0 {
			shortage = 0
		}
		result.Lines = append(result.Lines, ShortageLine{ProjectLineID: line.ID, ItemID: line.ItemID, ItemName: line.ItemName, Unit: line.Unit, PartNumber: line.PartNumber, Required: line.Quantity, Reserved: line.Reserved, Consumed: line.Consumed, Available: FormatQuantity(available), Shortage: FormatQuantity(shortage)})
	}
	return result, nil
}

func formatSignedQuantity(value int64) string {
	if value >= 0 {
		return FormatQuantity(value)
	}
	return "-" + FormatQuantity(-value)
}

func collectIDs(rows *sql.Rows) ([]string, error) {
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func BuildMatch(text string) string {
	words := strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	if len(words) > 12 {
		words = words[:12]
	}
	terms := make([]string, 0, len(words))
	for _, word := range words {
		if word != "" {
			terms = append(terms, `"`+strings.ReplaceAll(word, `"`, `""`)+`"*`)
		}
	}
	return strings.Join(terms, " ")
}

func engineeringAliases(value float64) []string {
	if math.IsNaN(value) || math.IsInf(value, 0) || value == 0 {
		return nil
	}
	abs := math.Abs(value)
	units := []struct {
		scale  float64
		suffix string
	}{{1e12, "T"}, {1e9, "G"}, {1e6, "M"}, {1e3, "k"}, {1, ""}, {1e-3, "m"}, {1e-6, "u"}, {1e-9, "n"}, {1e-12, "p"}}
	for _, unit := range units {
		mantissa := abs / unit.scale
		if mantissa >= 1 && mantissa < 1000 {
			text := strconv.FormatFloat(mantissa, 'f', -1, 64)
			if value < 0 {
				text = "-" + text
			}
			aliases := []string{text + unit.suffix}
			if unit.suffix != "" && strings.Contains(text, ".") {
				parts := strings.SplitN(text, ".", 2)
				aliases = append(aliases, parts[0]+unit.suffix+parts[1])
			}
			aliases = append(aliases, strconv.FormatFloat(value, 'f', -1, 64))
			return aliases
		}
	}
	return []string{strconv.FormatFloat(value, 'f', -1, 64)}
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "%", `\%`)
	return strings.ReplaceAll(value, "_", `\_`)
}
