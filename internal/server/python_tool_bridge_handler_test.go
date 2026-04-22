package server

import "testing"

func TestPythonToolBridgeBuildCatalogGroups_FiltersByAvailableTools(t *testing.T) {
	available := map[string]bool{
		"proxmox":          true,
		"docker":           true,
		"send_email":       true,
		"fetch_email":      true,
		"media_conversion": true,
		// list_email_accounts intentionally missing
		"execute_shell": true, // must never appear because not in curated groups
	}

	groups := pythonToolBridgeBuildCatalogGroups(available)
	if len(groups) == 0 {
		t.Fatal("expected at least one group")
	}

	foundProxmox := false
	foundEmail := false
	foundMediaConversion := false
	for _, g := range groups {
		if g.Key == "proxmox" {
			foundProxmox = true
			if len(g.Tools) != 1 || g.Tools[0] != "proxmox" {
				t.Fatalf("unexpected proxmox tools: %#v", g.Tools)
			}
		}
		if g.Key == "email" {
			foundEmail = true
			// list_email_accounts was not available, so it should not be included.
			if len(g.Tools) != 2 {
				t.Fatalf("expected 2 email tools, got %#v", g.Tools)
			}
		}
		if g.Key == "media_conversion" {
			foundMediaConversion = true
			if len(g.Tools) != 1 || g.Tools[0] != "media_conversion" {
				t.Fatalf("unexpected media conversion tools: %#v", g.Tools)
			}
		}
		if g.Key == "execution" || g.Key == "filesystem" {
			t.Fatalf("unexpected dangerous group in catalog: %q", g.Key)
		}
	}

	if !foundProxmox {
		t.Fatal("expected proxmox group")
	}
	if !foundEmail {
		t.Fatal("expected email group")
	}
	if !foundMediaConversion {
		t.Fatal("expected media_conversion group")
	}
}
