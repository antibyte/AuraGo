package kaggle

import (
	"aurago/internal/config"
	"testing"
)

func TestKagglePublicDownloadDoesNotRequireOwnerWriteGrant(t *testing.T) {
	c := config.DefaultKaggleConfig()
	c.Enabled = true
	c.AllowDownload = true
	a := Access{Network: true, Write: true}
	op, _ := Lookup("datasets", "download_dataset")
	p := map[string]any{"ownerSlug": "public-author", "datasetSlug": "example"}
	if err := CheckPolicy(c, a, op, p, "my-account"); err != nil {
		t.Fatal(err)
	}
	c.ReadOnly = false
	c.AllowDelete = true
	op, _ = Lookup("datasets", "delete_dataset")
	if err := CheckPolicy(c, a, op, p, "my-account"); err == nil {
		t.Fatal("foreign deletion allowed")
	}
}
