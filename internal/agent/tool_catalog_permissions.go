package agent

import (
	"aurago/internal/config"
	"aurago/internal/prompts"
	"reflect"
	"strings"
)

// Read-only metadata comes from existing integration settings. Runtime handlers
// remain responsible for their exact operation policy.
func catalogReadOnly(cfg *config.Config, entry *ToolCatalogEntry) bool {
	if cfg == nil {
		return false
	}
	name := prompts.ToolManualID(entry.Name)
	if entry.Name == "sql_query" || entry.Name == "manage_sql_connections" {
		name = "sql_connections"
	}
	root := reflect.ValueOf(cfg).Elem()
	for _, parent := range []reflect.Value{root, root.FieldByName("Tools")} {
		if !parent.IsValid() || parent.Kind() != reflect.Struct {
			continue
		}
		for i := 0; i < parent.NumField(); i++ {
			key := strings.Split(parent.Type().Field(i).Tag.Get("yaml"), ",")[0]
			if key == "" || key == "-" || (name != key && !strings.HasPrefix(entry.Name, key+"_")) {
				continue
			}
			value := parent.Field(i)
			if value.Kind() != reflect.Struct {
				continue
			}
			for _, field := range []string{"ReadOnly", "Readonly"} {
				v := value.FieldByName(field)
				if v.IsValid() && v.Kind() == reflect.Bool && v.Bool() {
					return true
				}
			}
		}
	}
	return false
}

func toolCatalogOperationField(entry *ToolCatalogEntry, field string) []string {
	props, _ := schemaParameters(entry.Schema)["properties"].(map[string]interface{})
	op, _ := props[field].(map[string]interface{})
	var result []string
	switch values := op["enum"].(type) {
	case []string:
		return values
	case []interface{}:
		for _, v := range values {
			if s, ok := v.(string); ok {
				result = append(result, s)
			}
		}
	}
	return result
}
