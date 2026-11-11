package flows

import (
	"encoding/base64"
	"errors"
	"strings"
)

var (
	// ErrCredentialGone: the credential is no longer at that place of the draft.
	ErrCredentialGone = errors.New("the credential is no longer in the flow")
	// ErrCredentialNotMovable: the finding exists, but only an http.request header, query value
	// or user info with a free auth_secret can move to the vault.
	ErrCredentialNotMovable = errors.New("this credential cannot be moved to the vault")
)

// MoveCredential rewrites doc (a copy the caller owns) so that the credential at (nodeID,
// location, path) is read from the vault entry secretName instead: header -> auth_in=header,
// auth_header, auth_prefix; url_query -> auth_in=query, auth_query; url_userinfo -> Basic
// authorisation. Every other movable finding with the identical literal and location in the
// same document gets the same rewrite (moved lists the node ids, in document order);
// notMovable lists the ones that cannot move (their node has an auth_secret already, or the
// value mixes a template). value is what the caller stores in the vault; it is never returned
// to a browser.
func MoveCredential(doc *Flow, nodeID, location, path, secretName string) (moved []string, notMovable []CredentialFinding, value string, err error) {
	if doc == nil {
		return nil, nil, "", ErrCredentialGone
	}
	findings := FindLiteralCredentials(doc)
	var target *CredentialFinding
	for i := range findings {
		if f := &findings[i]; f.NodeID == nodeID && f.Location == location && f.Path == path {
			target = f
			break
		}
	}
	if target == nil {
		return nil, nil, "", ErrCredentialGone
	}
	if !target.Movable {
		return nil, nil, "", ErrCredentialNotMovable
	}
	secret, prefix := credentialSecret(*target)
	if strings.TrimSpace(secret) == "" {
		return nil, nil, "", ErrCredentialNotMovable
	}
	seen := map[string]bool{}
	for _, f := range findings {
		if f.Location != target.Location || f.Value != target.Value {
			continue
		}
		node := doc.NodeByID(f.NodeID)
		// The slot is checked again: an earlier rewrite of this loop can have taken it.
		if !f.Movable || node == nil || !isEmptyValue(node.Params["auth_secret"]) {
			notMovable = append(notMovable, f)
			continue
		}
		rewriteCredential(node, f, secretName, prefix)
		if !seen[node.ID] {
			seen[node.ID] = true
			moved = append(moved, node.ID)
		}
	}
	return moved, notMovable, secret, nil
}

// credentialSecret derives what goes into the vault and the header prefix from a finding.
func credentialSecret(f CredentialFinding) (secret, prefix string) {
	switch f.Location {
	case credHeader:
		for _, scheme := range []string{"Bearer ", "Basic "} {
			if len(f.Value) > len(scheme) && strings.EqualFold(f.Value[:len(scheme)], scheme) {
				return strings.TrimSpace(f.Value[len(scheme):]), f.Value[:len(scheme)]
			}
		}
		return strings.TrimSpace(f.Value), ""
	case credUserinfo:
		return base64.StdEncoding.EncodeToString([]byte(f.Value)), "Basic "
	}
	return f.Value, ""
}

// rewriteURLParam applies fn to the first web address word of a text parameter.
func rewriteURLParam(v any, fn func(u *credentialURL)) any {
	s, ok := v.(string)
	if !ok {
		return v
	}
	tokens := credentialWords.FindAllString(s, -1)
	for i, tok := range tokens {
		if u, isURL := splitCredentialURL(tok); isURL {
			fn(&u)
			tokens[i] = u.String()
			break
		}
	}
	return strings.Join(tokens, "")
}

func rewriteCredential(n *Node, f CredentialFinding, secretName, prefix string) {
	n.Params["auth_secret"] = secretName
	switch f.Location {
	case credHeader:
		if headers, ok := n.Params["headers"].(map[string]any); ok {
			for k := range headers {
				if strings.TrimSpace(k) == f.Path {
					delete(headers, k)
				}
			}
		}
		n.Params["auth_in"], n.Params["auth_header"], n.Params["auth_prefix"] = "header", f.Path, prefix
	case credQuery:
		n.Params["url"] = rewriteURLParam(n.Params["url"], func(u *credentialURL) {
			var keep []string
			for _, pair := range strings.Split(u.query, "&") {
				key, val, _ := strings.Cut(pair, "=")
				if unescapeQuery(key) == f.Path && unescapeQuery(val) == f.Value {
					continue
				}
				keep = append(keep, pair)
			}
			u.query, u.hasQuery = strings.Join(keep, "&"), len(keep) > 0
		})
		n.Params["auth_in"], n.Params["auth_query"] = "query", f.Path
	case credUserinfo:
		n.Params["url"] = rewriteURLParam(n.Params["url"], func(u *credentialURL) { u.userinfo, u.hasUser = "", false })
		n.Params["auth_in"], n.Params["auth_header"], n.Params["auth_prefix"] = "header", "Authorization", "Basic "
	}
}
