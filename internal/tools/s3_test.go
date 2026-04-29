package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestS3ReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := S3Config{AccessKey: "test-access", SecretKey: "test-secret", ReadOnly: true}

	for name, got := range map[string]string{
		"upload": ExecuteS3(cfg, "upload", "bucket", "key", "local.txt", "", "", ""),
		"delete": ExecuteS3(cfg, "delete", "bucket", "key", "", "", "", ""),
		"copy":   ExecuteS3(cfg, "copy", "bucket", "key", "", "", "bucket", "copy-key"),
		"move":   ExecuteS3(cfg, "move", "bucket", "key", "", "", "bucket", "move-key"),
	} {
		t.Run(name, func(t *testing.T) {
			var result s3Result
			if err := json.Unmarshal([]byte(got), &result); err != nil {
				t.Fatalf("decode result: %v\nraw=%s", err, got)
			}
			if result.Status != "error" || !strings.Contains(result.Message, "read-only mode") {
				t.Fatalf("response = %+v, want read-only denial", result)
			}
		})
	}
}
