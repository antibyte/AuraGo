package i18n

import (
	"encoding/json"
	"testing"
)

func TestGetJSONForSectionFallsBackToEnglishPerKey(t *testing.T) {
	store := &Store{
		langJSON: map[string]string{
			"en": `{"common.ok":"OK","common.cancel":"Cancel","chat.send":"Send","chat.title":"Chat"}`,
			"de": `{"common.ok":"OK","chat.send":"Senden"}`,
		},
		langSectionJSON: map[string]string{
			"en:common": `{"common.ok":"OK","common.cancel":"Cancel"}`,
			"en:chat":   `{"chat.send":"Send","chat.title":"Chat"}`,
			"de:common": `{"common.ok":"OK"}`,
			"de:chat":   `{"chat.send":"Senden"}`,
		},
	}

	var got map[string]string
	if err := json.Unmarshal([]byte(store.GetJSONForSection("de", "chat")), &got); err != nil {
		t.Fatalf("unmarshal section JSON: %v", err)
	}
	want := map[string]string{
		"common.ok":     "OK",
		"common.cancel": "Cancel",
		"chat.send":     "Senden",
		"chat.title":    "Chat",
	}
	for key, value := range want {
		if got[key] != value {
			t.Errorf("translation %q = %q, want %q", key, got[key], value)
		}
	}
}
