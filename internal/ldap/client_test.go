package ldap

import (
	"errors"
	"testing"
	"time"

	ldappkg "github.com/go-ldap/ldap/v3"
)

type fakeLDAPConn struct {
	bindCalls    []struct{ username, password string }
	bindErrs     []error
	searchResult *ldappkg.SearchResult
	searchErr    error
	lastSearch   *ldappkg.SearchRequest
	lastAdd      *ldappkg.AddRequest
	addErr       error
	lastModify   *ldappkg.ModifyRequest
	modifyErr    error
	lastDelete   *ldappkg.DelRequest
	deleteErr    error
	timeout      time.Duration
}

func (f *fakeLDAPConn) Bind(username, password string) error {
	f.bindCalls = append(f.bindCalls, struct{ username, password string }{username: username, password: password})
	if len(f.bindErrs) == 0 {
		return nil
	}
	err := f.bindErrs[0]
	f.bindErrs = f.bindErrs[1:]
	return err
}

func (f *fakeLDAPConn) Search(searchRequest *ldappkg.SearchRequest) (*ldappkg.SearchResult, error) {
	f.lastSearch = searchRequest
	if f.searchErr != nil {
		return nil, f.searchErr
	}
	if f.searchResult != nil {
		return f.searchResult, nil
	}
	return &ldappkg.SearchResult{}, nil
}

func (f *fakeLDAPConn) Add(addRequest *ldappkg.AddRequest) error {
	f.lastAdd = addRequest
	return f.addErr
}

func (f *fakeLDAPConn) Modify(modifyRequest *ldappkg.ModifyRequest) error {
	f.lastModify = modifyRequest
	return f.modifyErr
}

func (f *fakeLDAPConn) Del(delRequest *ldappkg.DelRequest) error {
	f.lastDelete = delRequest
	return f.deleteErr
}

func (f *fakeLDAPConn) Close() error { return nil }

func (f *fakeLDAPConn) SetTimeout(timeout time.Duration) {
	f.timeout = timeout
}

func TestGetUserEscapesFilterUsingLibraryHelper(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{
		cfg: LDAPConfig{
			BaseDN:         "dc=example,dc=com",
			UserSearchBase: "ou=users,dc=example,dc=com",
		},
		conn: conn,
	}

	if _, err := client.GetUser(`john*)(|(uid=*))`); err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	want := "(|(uid=john\\2a\\29\\28|\\28uid=\\2a\\29\\29)(sAMAccountName=john\\2a\\29\\28|\\28uid=\\2a\\29\\29)(cn=john\\2a\\29\\28|\\28uid=\\2a\\29\\29))"
	if conn.lastSearch == nil || conn.lastSearch.Filter != want {
		t.Fatalf("filter = %q, want %q", conn.lastSearch.Filter, want)
	}
}

func TestListUsersUsesUserObjectFilter(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{
		cfg:  LDAPConfig{BaseDN: "dc=example,dc=com"},
		conn: conn,
	}

	if _, err := client.ListUsers(); err != nil {
		t.Fatalf("ListUsers() error = %v", err)
	}
	if conn.lastSearch.Filter != "(|(objectClass=person)(objectClass=user)(objectClass=inetOrgPerson))" {
		t.Fatalf("filter = %q", conn.lastSearch.Filter)
	}
}

func TestGetGroupUsesCNAndSAMAccountNameFilter(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{
		cfg:  LDAPConfig{BaseDN: "dc=example,dc=com"},
		conn: conn,
	}

	if _, err := client.GetGroup("Domain Users"); err != nil {
		t.Fatalf("GetGroup() error = %v", err)
	}
	if conn.lastSearch.Filter != "(|(cn=Domain Users)(sAMAccountName=Domain Users))" {
		t.Fatalf("filter = %q", conn.lastSearch.Filter)
	}
}

func TestListGroupsUsesGroupObjectFilter(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{
		cfg:  LDAPConfig{BaseDN: "dc=example,dc=com"},
		conn: conn,
	}

	if _, err := client.ListGroups(); err != nil {
		t.Fatalf("ListGroups() error = %v", err)
	}
	if conn.lastSearch.Filter != "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))" {
		t.Fatalf("filter = %q", conn.lastSearch.Filter)
	}
}

func TestAuthenticateReturnsFalseForInvalidCredentials(t *testing.T) {
	conn := &fakeLDAPConn{
		bindErrs: []error{&ldappkg.Error{ResultCode: ldappkg.LDAPResultInvalidCredentials}},
	}
	client := &Client{
		cfg:  LDAPConfig{BindDN: "cn=svc,dc=example,dc=com", BindPassword: "vault-secret"},
		conn: conn,
	}

	ok, err := client.Authenticate("cn=user,dc=example,dc=com", "bad-password")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if ok {
		t.Fatal("expected Authenticate() to return false for invalid credentials")
	}
}

func TestAuthenticateKeepsSuccessWhenRebindFails(t *testing.T) {
	conn := &fakeLDAPConn{
		bindErrs: []error{nil, errors.New("service account unavailable")},
	}
	client := &Client{
		cfg:  LDAPConfig{BindDN: "cn=svc,dc=example,dc=com", BindPassword: "vault-secret"},
		conn: conn,
	}

	ok, err := client.Authenticate("cn=user,dc=example,dc=com", "secret")
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if !ok {
		t.Fatal("expected Authenticate() to remain successful after rebind failure")
	}
	if len(conn.bindCalls) != 2 {
		t.Fatalf("bind calls = %d, want 2", len(conn.bindCalls))
	}
}

func TestAddEntryBuildsAddRequestFromAttributes(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{conn: conn}

	err := client.AddEntry("cn=jane,dc=example,dc=com", map[string][]string{
		"sn":          []string{"Doe"},
		"objectClass": []string{"top", "person"},
		"mail":        []string{""},
	})
	if err != nil {
		t.Fatalf("AddEntry() error = %v", err)
	}
	if conn.lastAdd == nil {
		t.Fatal("expected add request to be captured")
	}
	if conn.lastAdd.DN != "cn=jane,dc=example,dc=com" {
		t.Fatalf("DN = %q", conn.lastAdd.DN)
	}
	if len(conn.lastAdd.Attributes) != 2 {
		t.Fatalf("attributes = %#v, want 2 non-empty attributes", conn.lastAdd.Attributes)
	}
}

func TestModifyEntryReplacesAndDeletesAttributes(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{conn: conn}

	err := client.ModifyEntry("cn=jane,dc=example,dc=com", map[string][]string{
		"description":     []string{"Updated"},
		"telephoneNumber": []string{},
	})
	if err != nil {
		t.Fatalf("ModifyEntry() error = %v", err)
	}
	if conn.lastModify == nil {
		t.Fatal("expected modify request to be captured")
	}
	if len(conn.lastModify.Changes) != 2 {
		t.Fatalf("changes = %#v, want 2", conn.lastModify.Changes)
	}

	foundReplace := false
	foundDelete := false
	for _, change := range conn.lastModify.Changes {
		switch change.Modification.Type {
		case "description":
			foundReplace = true
		case "telephoneNumber":
			foundDelete = true
		}
	}
	if !foundReplace || !foundDelete {
		t.Fatalf("changes = %#v, want replace and delete entries", conn.lastModify.Changes)
	}
}

func TestDeleteEntryBuildsDeleteRequest(t *testing.T) {
	conn := &fakeLDAPConn{}
	client := &Client{conn: conn}

	if err := client.DeleteEntry("cn=jane,dc=example,dc=com"); err != nil {
		t.Fatalf("DeleteEntry() error = %v", err)
	}
	if conn.lastDelete == nil || conn.lastDelete.DN != "cn=jane,dc=example,dc=com" {
		t.Fatalf("delete request = %#v", conn.lastDelete)
	}
}
