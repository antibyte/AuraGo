package gamemaker

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestModelDescriptionKeepsExampleBeforeLargeMetadata(t *testing.T) {
	s := newTestService(t)
	detail, err := s.DescribeAsset(ModelPackID, "fps-arms-modern", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	end := strings.Index(text, "releaseAsset")
	if !strings.HasPrefix(text, `{"example":`) || end < 0 || end > 1400 {
		t.Fatal("bounded tool output hides the complete loading/disposal example")
	}
}
