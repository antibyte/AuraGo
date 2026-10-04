package ldap

import (
	"fmt"
	"time"

	"github.com/go-ldap/ldap/v3"
)

func searchLDAPPages(conn ldapConn, request *ldap.SearchRequest, timeoutSeconds int) (*ldap.SearchResult, error) {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 30
	}
	budget := time.Duration(timeoutSeconds) * time.Second
	deadline := time.Now().Add(budget)
	defer conn.SetTimeout(budget)
	paging := ldap.NewControlPaging(500)
	request.Controls = append(request.Controls, paging)
	all := &ldap.SearchResult{}
	seen := map[string]bool{}
	for page := 0; page < 200; page++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("LDAP search deadline exceeded")
		}
		conn.SetTimeout(remaining)
		request.TimeLimit = int((remaining + time.Second - 1) / time.Second)
		result, err := conn.Search(request)
		if err != nil {
			return nil, fmt.Errorf("LDAP paged search failed: %w", err)
		}
		if result == nil {
			return nil, fmt.Errorf("LDAP search returned no result")
		}
		if len(result.Entries) > 100000-len(all.Entries) {
			return nil, fmt.Errorf("LDAP search exceeds 100000 entries")
		}
		all.Entries = append(all.Entries, result.Entries...)
		control := ldap.FindControl(result.Controls, ldap.ControlTypePaging)
		if control == nil {
			return all, nil
		}
		next, ok := control.(*ldap.ControlPaging)
		if !ok {
			return nil, fmt.Errorf("invalid LDAP paging response")
		}
		if len(next.Cookie) == 0 {
			return all, nil
		}
		cookie := string(next.Cookie)
		if seen[cookie] {
			return nil, fmt.Errorf("LDAP server repeated a paging cookie")
		}
		seen[cookie] = true
		paging.SetCookie(append([]byte(nil), next.Cookie...))
	}
	return nil, fmt.Errorf("LDAP search exceeds 200 pages")
}
