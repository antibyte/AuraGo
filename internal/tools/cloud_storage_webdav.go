package tools

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"aurago/internal/security"
)

func (c CloudStorage) davEntries(ctx context.Context, loc StorageLocation, depth string) ([]StorageEntry, error) {
	body := `<?xml version="1.0"?><d:propfind xmlns:d="DAV:"><d:prop><d:displayname/><d:getcontentlength/><d:getcontenttype/><d:getlastmodified/><d:resourcetype/><d:getetag/></d:prop></d:propfind>`
	data, err := readStorageResponse(c.request(ctx, loc, "PROPFIND", "", strings.NewReader(body), map[string]string{"Depth": depth, "Content-Type": "application/xml"}))
	if err != nil {
		return nil, err
	}
	var result struct {
		XMLName   xml.Name `xml:"multistatus"`
		Responses []struct {
			Href  string `xml:"href"`
			Props []struct {
				Status string `xml:"status"`
				Prop   struct {
					Size        string `xml:"getcontentlength"`
					ContentType string `xml:"getcontenttype"`
					Modified    string `xml:"getlastmodified"`
					ETag        string `xml:"getetag"`
					Resource    struct {
						Collection *struct{} `xml:"collection"`
					} `xml:"resourcetype"`
				} `xml:"prop"`
			} `xml:"propstat"`
		} `xml:"response"`
	}
	if err = xml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode WebDAV metadata: %w", err)
	}
	base, err := url.Parse(strings.TrimRight(c.WebDAV.URL, "/") + "/")
	if err != nil {
		return nil, err
	}
	requestURL, _ := webdavURL(c.WebDAV, loc.Path)
	request, _ := url.Parse(requestURL)
	if depth == "1" {
		request.Path = strings.TrimRight(request.Path, "/") + "/"
		request.RawPath = ""
	}
	root := strings.TrimRight(base.Path, "/")
	entries := make([]StorageEntry, 0)
	seen := make(map[string]bool)
	if len(result.Responses) > 10001 {
		return nil, fmt.Errorf("storage listing exceeds 10000 entries")
	}
	for _, response := range result.Responses {
		href, e := url.Parse(response.Href)
		if e != nil || href.RawQuery != "" || href.Fragment != "" || href.User != nil || strings.TrimSpace(response.Href) == "" {
			return nil, fmt.Errorf("invalid WebDAV href")
		}
		escaped := strings.ToLower(href.EscapedPath())
		if strings.Contains(escaped, "%2f") || strings.Contains(escaped, "%5c") {
			return nil, fmt.Errorf("WebDAV href contains an encoded path separator")
		}
		resolved := request.ResolveReference(href)
		if !security.SameHTTPOrigin(base, resolved) {
			return nil, fmt.Errorf("WebDAV href leaves configured origin")
		}
		p := strings.TrimRight(resolved.Path, "/")
		if p != root && !strings.HasPrefix(p, root+"/") {
			return nil, fmt.Errorf("WebDAV href leaves configured root")
		}
		rel := strings.TrimPrefix(strings.TrimPrefix(p, root), "/")
		if err := ValidateStorageLocation(StorageLocation{Provider: "webdav", Path: rel}); err != nil {
			return nil, err
		}
		if depth == "1" && rel == loc.Path {
			continue
		}
		if depth == "0" && rel != loc.Path {
			return nil, fmt.Errorf("WebDAV metadata names a different path")
		}
		parent := path.Dir(rel)
		if parent == "." {
			parent = ""
		}
		if depth == "1" && parent != loc.Path {
			return nil, fmt.Errorf("WebDAV listing contains a non-child path")
		}
		if seen[rel] {
			return nil, fmt.Errorf("duplicate WebDAV entry")
		}
		seen[rel] = true
		entry := StorageEntry{Name: path.Base(rel), Path: rel, Type: "file", ReadOnly: c.WebDAV.ReadOnly}
		valid := false
		for _, properties := range response.Props {
			status := strings.Fields(properties.Status)
			if len(status) < 2 || status[1] != "200" {
				continue
			}
			valid = true
			prop := properties.Prop
			if prop.Resource.Collection != nil {
				entry.Type = "directory"
			}
			if prop.Size != "" {
				entry.Size, e = strconv.ParseInt(prop.Size, 10, 64)
				if e != nil || entry.Size < 0 {
					return nil, fmt.Errorf("invalid WebDAV file size")
				}
			}
			if prop.ContentType != "" {
				entry.ContentType = prop.ContentType
			}
			if prop.Modified != "" {
				if modified, e := time.Parse(time.RFC1123, prop.Modified); e == nil {
					entry.Modified = modified.UTC().Format(time.RFC3339)
				}
			}
			if len(prop.ETag) >= 2 && strings.HasPrefix(prop.ETag, "\"") && strings.HasSuffix(prop.ETag, "\"") && !strings.ContainsAny(prop.ETag[1:len(prop.ETag)-1], "\"\r\n") {
				entry.Version = prop.ETag
			}
		}
		if !valid {
			return nil, fmt.Errorf("WebDAV entry has no successful properties")
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
