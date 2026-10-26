package rustfs

import (
	"encoding/json"
	"sort"
	"strings"

	"aurago/internal/config"
)

type policyStatement struct {
	Effect    string                         `json:"Effect"`
	Action    []string                       `json:"Action"`
	Resource  []string                       `json:"Resource,omitempty"`
	Condition map[string]map[string][]string `json:"Condition,omitempty"`
}
type policyDocument struct {
	Version   string            `json:"Version"`
	Statement []policyStatement `json:"Statement"`
}

func intersectPermissions(a, b config.RustFSPermissions) config.RustFSPermissions {
	out := config.RustFSPermissions{AllowRead: a.AllowRead && b.AllowRead, AllowWrite: a.AllowWrite && b.AllowWrite, AllowDelete: a.AllowDelete && b.AllowDelete}
	for _, left := range a.AllowedLocations {
		found := map[string]bool{}
		for _, right := range b.AllowedLocations {
			if left.Bucket != right.Bucket {
				continue
			}
			for _, lp := range left.Prefixes {
				for _, rp := range right.Prefixes {
					switch {
					case strings.HasPrefix(lp, rp):
						found[lp] = true
					case strings.HasPrefix(rp, lp):
						found[rp] = true
					}
				}
			}
		}
		if len(found) > 0 {
			var prefixes []string
			for prefix := range found {
				prefixes = append(prefixes, prefix)
			}
			sort.Strings(prefixes)
			out.AllowedLocations = append(out.AllowedLocations, config.RustFSLocation{Bucket: left.Bucket, Prefixes: prefixes})
		}
	}
	sort.Slice(out.AllowedLocations, func(i, j int) bool { return out.AllowedLocations[i].Bucket < out.AllowedLocations[j].Bucket })
	return out
}
func effectivePermissions(cfg *config.Config) config.RustFSPermissions {
	p := clonePermissions(cfg.RustFS.Permissions)
	if !cfg.RustFS.AgentAccess || !cfg.RustFS.Enabled || !cfg.S3.Enabled || cfg.S3.Provider != config.S3ProviderRustFS {
		p.AllowRead = false
		p.AllowWrite = false
		p.AllowDelete = false
	}
	if cfg.S3.ReadOnly {
		p.AllowWrite = false
		p.AllowDelete = false
	}
	return p
}
func clonePermissions(p config.RustFSPermissions) config.RustFSPermissions {
	out := p
	out.AllowedLocations = nil
	for _, l := range p.AllowedLocations {
		out.AllowedLocations = append(out.AllowedLocations, config.RustFSLocation{Bucket: l.Bucket, Prefixes: append([]string(nil), l.Prefixes...)})
	}
	return out
}
func generatePolicy(p config.RustFSPermissions) []byte {
	doc := policyDocument{Version: "2012-10-17", Statement: []policyStatement{
		{Effect: "Deny", Action: []string{"admin:*"}},
		{Effect: "Deny", Action: []string{"sts:*"}},
	}}
	for _, location := range p.AllowedLocations {
		if len(location.Prefixes) == 0 || !p.AllowRead && !p.AllowWrite && !p.AllowDelete {
			continue
		}
		arn := "arn:aws:s3:::" + location.Bucket
		prefixes := make([]string, 0, len(location.Prefixes))
		objects := make([]string, 0, len(location.Prefixes))
		for _, prefix := range location.Prefixes {
			prefixes = append(prefixes, prefix+"*")
			objects = append(objects, arn+"/"+prefix+"*")
		}
		if p.AllowRead {
			doc.Statement = append(doc.Statement, policyStatement{Effect: "Allow", Action: []string{"s3:GetBucketLocation"}, Resource: []string{arn}})
		}
		if p.AllowRead {
			doc.Statement = append(doc.Statement,
				policyStatement{Effect: "Allow", Action: []string{"s3:ListBucket"}, Resource: []string{arn}, Condition: map[string]map[string][]string{"StringLike": {"s3:prefix": prefixes}}},
				policyStatement{Effect: "Allow", Action: []string{"s3:GetObject"}, Resource: objects})
		}
		if p.AllowWrite {
			doc.Statement = append(doc.Statement, policyStatement{Effect: "Allow", Action: []string{"s3:PutObject", "s3:AbortMultipartUpload", "s3:ListMultipartUploadParts"}, Resource: objects})
		}
		if p.AllowDelete {
			doc.Statement = append(doc.Statement, policyStatement{Effect: "Allow", Action: []string{"s3:DeleteObject"}, Resource: objects})
		}
	}
	data, _ := json.Marshal(doc)
	return data
}
func hasDataPermissions(p config.RustFSPermissions) bool {
	if !p.AllowRead && !p.AllowWrite && !p.AllowDelete {
		return false
	}
	for _, l := range p.AllowedLocations {
		if len(l.Prefixes) > 0 {
			return true
		}
	}
	return false
}

// RustFS serializes a one-value condition as a scalar, while policy requests may use arrays.
func normalizePolicyConditions(value any) {
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	if conditions, ok := object["Condition"].(map[string]any); ok {
		for _, operator := range conditions {
			if fields, ok := operator.(map[string]any); ok {
				for key, item := range fields {
					if scalar, ok := item.(string); ok {
						fields[key] = []any{scalar}
					}
				}
			}
		}
	}
	for _, child := range object {
		switch typed := child.(type) {
		case map[string]any:
			normalizePolicyConditions(typed)
		case []any:
			for _, entry := range typed {
				normalizePolicyConditions(entry)
			}
		}
	}
}

func policyHash(data []byte) string {
	var value any
	if json.Unmarshal(data, &value) != nil {
		return ""
	}
	normalizePolicyConditions(value)
	return hashValues(value)
}
