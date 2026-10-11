package config

import "testing"

func TestGogAccountValidation(t *testing.T) {
	for _, id := range []string{"../secret", "a/b", "A", "", "a:other"} {
		if ValidateGogConfig(GogConfig{Accounts: []GogAccount{{ID: id}}}) == nil {
			t.Fatalf("accepted %q", id)
		}
	}
	if err := ValidateGogConfig(GogConfig{Accounts: []GogAccount{{ID: "personal", Services: map[string]GogGrants{"drive": {Read: true}}}}}); err != nil {
		t.Fatal(err)
	}
	if ValidateGogConfig(GogConfig{Accounts: []GogAccount{{ID: "a"}, {ID: "a"}}}) == nil {
		t.Fatal("duplicate account")
	}
}
