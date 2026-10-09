package localwiki

import (
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const meta4BodyLimit = 8 << 20

// zimFileNamePattern is the only file name an edition may have on disk.
var zimFileNamePattern = regexp.MustCompile(`^wikipedia_[a-z]{2,3}_all_(maxi|nopic)_\d{4}-\d{2}[a-z]?\.zim$`)

type meta4File struct {
	FileName string
	Size     int64
	SHA256   string
	URLs     []string
}

type metalinkDoc struct {
	Files []metalinkFile `xml:"file"`
}

type metalinkFile struct {
	Name   string `xml:"name,attr"`
	Size   int64  `xml:"size"`
	Hashes []struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	} `xml:"hash"`
	URLs []struct {
		Priority int    `xml:"priority,attr"`
		Value    string `xml:",chardata"`
	} `xml:"url"`
}

// parseMeta4 reads the RFC 5854 metalink Kiwix publishes for an edition: exact
// size, SHA-256 and the mirror URLs (HTTPS only, ascending priority).
func parseMeta4(data []byte, wantFile string) (meta4File, error) {
	var doc metalinkDoc
	if err := xml.Unmarshal(data, &doc); err != nil {
		return meta4File{}, fmt.Errorf("parse meta4: %w", err)
	}
	for _, file := range doc.Files {
		name := strings.TrimSpace(file.Name)
		if name != wantFile {
			continue
		}
		if !zimFileNamePattern.MatchString(name) {
			return meta4File{}, fmt.Errorf("meta4 names an unexpected file %q", name)
		}
		if file.Size <= 0 {
			return meta4File{}, fmt.Errorf("meta4 for %s has no size", name)
		}
		out := meta4File{FileName: name, Size: file.Size}
		for _, hash := range file.Hashes {
			if !strings.EqualFold(strings.TrimSpace(hash.Type), "sha-256") {
				continue
			}
			value := strings.ToLower(strings.TrimSpace(hash.Value))
			if decoded, err := hex.DecodeString(value); err == nil && len(decoded) == 32 {
				out.SHA256 = value
			}
		}
		if out.SHA256 == "" {
			return meta4File{}, fmt.Errorf("meta4 for %s has no SHA-256", name)
		}
		type mirror struct {
			priority int
			url      string
		}
		var mirrors []mirror
		for _, candidate := range file.URLs {
			value := strings.TrimSpace(candidate.Value)
			if _, err := requireHTTPS(value); err != nil {
				continue
			}
			priority := candidate.Priority
			if priority <= 0 {
				priority = 1 << 20
			}
			mirrors = append(mirrors, mirror{priority: priority, url: value})
		}
		sort.SliceStable(mirrors, func(i, j int) bool { return mirrors[i].priority < mirrors[j].priority })
		for _, m := range mirrors {
			out.URLs = append(out.URLs, m.url)
		}
		return out, nil
	}
	return meta4File{}, fmt.Errorf("meta4 does not describe %s", wantFile)
}
