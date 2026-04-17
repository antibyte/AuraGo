package ldap

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"sort"
	"strconv"
	"time"

	"aurago/internal/config"

	"github.com/go-ldap/ldap/v3"
)

type ldapConn interface {
	Bind(username, password string) error
	Search(searchRequest *ldap.SearchRequest) (*ldap.SearchResult, error)
	Add(addRequest *ldap.AddRequest) error
	Modify(modifyRequest *ldap.ModifyRequest) error
	Del(delRequest *ldap.DelRequest) error
	Close() error
	SetTimeout(timeout time.Duration)
}

type dialLDAPURLFunc func(addr string, opts ...ldap.DialOpt) (*ldap.Conn, error)

var dialLDAPURL dialLDAPURLFunc = ldap.DialURL

type Client struct {
	cfg    LDAPConfig
	conn   ldapConn
	closed bool
}

type LDAPConfig = config.LDAPConfig

type SearchEntry struct {
	DN         string
	Attributes map[string][]string
}

type SearchResult struct {
	Entries []SearchEntry
}

func NewClient(cfg LDAPConfig) *Client {
	return &Client{cfg: cfg}
}

func (c *Client) Connect() error {
	if c.closed {
		return fmt.Errorf("client is closed")
	}
	if c.conn != nil {
		return nil
	}

	address := net.JoinHostPort(c.cfg.Host, strconv.Itoa(c.cfg.Port))
	scheme := "ldap"
	if c.cfg.UseTLS {
		scheme = "ldaps"
	}

	dialer := &net.Dialer{}
	if c.cfg.ConnectTimeout > 0 {
		dialer.Timeout = time.Duration(c.cfg.ConnectTimeout) * time.Second
	}

	opts := []ldap.DialOpt{ldap.DialWithDialer(dialer)}
	if c.cfg.UseTLS {
		opts = append(opts, ldap.DialWithTLSConfig(&tls.Config{
			InsecureSkipVerify: c.cfg.InsecureSkipVerify,
		}))
	}

	conn, err := dialLDAPURL((&url.URL{Scheme: scheme, Host: address}).String(), opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}
	if c.cfg.RequestTimeout > 0 {
		conn.SetTimeout(time.Duration(c.cfg.RequestTimeout) * time.Second)
	}

	c.conn = conn
	return nil
}

func (c *Client) Bind() error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	if err := c.conn.Bind(c.cfg.BindDN, c.cfg.BindPassword); err != nil {
		return fmt.Errorf("bind failed: %w", err)
	}
	return nil
}

func (c *Client) ConnectAndBind() error {
	if err := c.Connect(); err != nil {
		return err
	}
	return c.Bind()
}

func (c *Client) Close() {
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
	c.closed = true
}

func (c *Client) Search(baseDN, filter string, attributes []string) (*SearchResult, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}

	timeLimit := c.cfg.RequestTimeout
	if timeLimit < 0 {
		timeLimit = 0
	}

	searchRequest := ldap.NewSearchRequest(
		baseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		timeLimit,
		false,
		filter,
		attributes,
		nil,
	)

	result, err := c.conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	entries := make([]SearchEntry, 0, len(result.Entries))
	for _, entry := range result.Entries {
		attrs := make(map[string][]string, len(entry.Attributes))
		for _, attr := range entry.Attributes {
			attrs[attr.Name] = append([]string(nil), attr.Values...)
		}
		entries = append(entries, SearchEntry{
			DN:         entry.DN,
			Attributes: attrs,
		})
	}

	return &SearchResult{Entries: entries}, nil
}

func (c *Client) Authenticate(userDN, password string) (bool, error) {
	if c.conn == nil {
		return false, fmt.Errorf("not connected")
	}

	err := c.conn.Bind(userDN, password)
	if err != nil {
		ldapErr, ok := err.(*ldap.Error)
		if ok && ldapErr.ResultCode == ldap.LDAPResultInvalidCredentials {
			return false, nil
		}
		return false, fmt.Errorf("authentication failed: %w", err)
	}

	if err := c.conn.Bind(c.cfg.BindDN, c.cfg.BindPassword); err != nil {
		slog.Warn("LDAP rebind after successful authentication failed", "error", err)
	}

	return true, nil
}

func (c *Client) GetUser(username string) (*SearchEntry, error) {
	baseDN := c.cfg.UserSearchBase
	if baseDN == "" {
		baseDN = c.cfg.BaseDN
	}

	escaped := ldap.EscapeFilter(username)
	filter := fmt.Sprintf("(|(uid=%s)(sAMAccountName=%s)(cn=%s))", escaped, escaped, escaped)

	result, err := c.Search(baseDN, filter, nil)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, nil
	}

	return &result.Entries[0], nil
}

func (c *Client) ListUsers() (*SearchResult, error) {
	baseDN := c.cfg.UserSearchBase
	if baseDN == "" {
		baseDN = c.cfg.BaseDN
	}

	filter := "(|(objectClass=person)(objectClass=user)(objectClass=inetOrgPerson))"
	return c.Search(baseDN, filter, nil)
}

func (c *Client) GetGroup(groupName string) (*SearchEntry, error) {
	baseDN := c.cfg.GroupSearchBase
	if baseDN == "" {
		baseDN = c.cfg.BaseDN
	}

	escaped := ldap.EscapeFilter(groupName)
	filter := fmt.Sprintf("(|(cn=%s)(sAMAccountName=%s))", escaped, escaped)

	result, err := c.Search(baseDN, filter, nil)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, nil
	}

	return &result.Entries[0], nil
}

func (c *Client) ListGroups() (*SearchResult, error) {
	baseDN := c.cfg.GroupSearchBase
	if baseDN == "" {
		baseDN = c.cfg.BaseDN
	}

	filter := "(|(objectClass=group)(objectClass=groupOfNames)(objectClass=groupOfUniqueNames))"
	return c.Search(baseDN, filter, nil)
}

func (c *Client) AddEntry(dn string, attributes map[string][]string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	if dn == "" {
		return fmt.Errorf("dn is required")
	}
	if len(attributes) == 0 {
		return fmt.Errorf("attributes are required")
	}

	req := ldap.NewAddRequest(dn, nil)
	keys := sortedAttributeKeys(attributes)
	added := 0
	for _, key := range keys {
		values := sanitizeAttributeValues(attributes[key])
		if len(values) == 0 {
			continue
		}
		req.Attribute(key, values)
		added++
	}
	if added == 0 {
		return fmt.Errorf("at least one non-empty attribute is required")
	}
	if err := c.conn.Add(req); err != nil {
		return fmt.Errorf("add failed: %w", err)
	}
	return nil
}

func (c *Client) ModifyEntry(dn string, changes map[string][]string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	if dn == "" {
		return fmt.Errorf("dn is required")
	}
	if len(changes) == 0 {
		return fmt.Errorf("changes are required")
	}

	req := ldap.NewModifyRequest(dn, nil)
	for _, key := range sortedAttributeKeys(changes) {
		values := sanitizeAttributeValues(changes[key])
		if len(values) == 0 {
			req.Delete(key, nil)
			continue
		}
		req.Replace(key, values)
	}
	if err := c.conn.Modify(req); err != nil {
		return fmt.Errorf("modify failed: %w", err)
	}
	return nil
}

func (c *Client) DeleteEntry(dn string) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	if dn == "" {
		return fmt.Errorf("dn is required")
	}
	if err := c.conn.Del(ldap.NewDelRequest(dn, nil)); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}

func (c *Client) TestConnection() error {
	if err := c.Connect(); err != nil {
		return err
	}
	defer c.Close()

	if err := c.Bind(); err != nil {
		return err
	}
	return nil
}

func sortedAttributeKeys(values map[string][]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sanitizeAttributeValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}
