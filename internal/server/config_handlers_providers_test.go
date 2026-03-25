package server

import (
	"reflect"
	"testing"

	"aurago/internal/config"
)

func TestBuildSchemaIncludesRocketChatAuthTokenAsSensitive(t *testing.T) {
	schema := buildSchema(reflect.TypeOf(config.Config{}), "")

	var rocketchat *SchemaField
	for i := range schema {
		if schema[i].YAMLKey == "rocketchat" {
			rocketchat = &schema[i]
			break
		}
	}
	if rocketchat == nil {
		t.Fatal("rocketchat section not found in schema")
	}

	for _, field := range rocketchat.Children {
		if field.YAMLKey != "auth_token" {
			continue
		}
		if field.Key != "rocketchat.auth_token" {
			t.Fatalf("unexpected field key: %s", field.Key)
		}
		if !field.Sensitive {
			t.Fatal("expected rocketchat.auth_token to be marked sensitive")
		}
		return
	}

	t.Fatal("rocketchat.auth_token field not found in schema")
}
