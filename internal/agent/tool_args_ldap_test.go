package agent

import "testing"

func TestDecodeLDAPArgsUsesExtendedFields(t *testing.T) {
	tc := ToolCall{
		Action: "ldap",
		Params: map[string]interface{}{
			"operation":  "update_user",
			"base_dn":    "dc=example,dc=com",
			"filter":     "(objectClass=person)",
			"username":   "jane.doe",
			"group_name": "engineering",
			"user_dn":    "cn=jane.doe,dc=example,dc=com",
			"dn":         "cn=jane.doe,dc=example,dc=com",
			"password":   "secret",
			"attributes": []interface{}{"cn", "mail"},
			"entry_attributes": map[string]interface{}{
				"objectClass": []interface{}{"top", "person"},
				"cn":          "jane.doe",
			},
			"changes": map[string]interface{}{
				"description":     []interface{}{"Updated"},
				"telephoneNumber": []interface{}{},
			},
		},
	}

	req := decodeLDAPArgs(tc)
	if req.Operation != "update_user" {
		t.Fatalf("Operation = %q, want update_user", req.Operation)
	}
	if req.BaseDN != "dc=example,dc=com" || req.Filter != "(objectClass=person)" {
		t.Fatalf("unexpected base/filter decode: %+v", req)
	}
	if req.Username != "jane.doe" || req.GroupName != "engineering" {
		t.Fatalf("unexpected identity decode: %+v", req)
	}
	if req.UserDN != "cn=jane.doe,dc=example,dc=com" || req.DN != "cn=jane.doe,dc=example,dc=com" {
		t.Fatalf("unexpected DN decode: %+v", req)
	}
	if req.Password != "secret" {
		t.Fatalf("Password = %q, want secret", req.Password)
	}
	if len(req.Attributes) != 2 || req.Attributes[0] != "cn" || req.Attributes[1] != "mail" {
		t.Fatalf("Attributes = %#v", req.Attributes)
	}
	if len(req.EntryAttributes["objectClass"]) != 2 || req.EntryAttributes["cn"][0] != "jane.doe" {
		t.Fatalf("EntryAttributes = %#v", req.EntryAttributes)
	}
	if len(req.Changes["description"]) != 1 || req.Changes["description"][0] != "Updated" {
		t.Fatalf("Changes = %#v", req.Changes)
	}
	if req.Changes["telephoneNumber"] != nil && len(req.Changes["telephoneNumber"]) != 0 {
		t.Fatalf("telephoneNumber delete marker = %#v, want empty/nil slice", req.Changes["telephoneNumber"])
	}
}
