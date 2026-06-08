package desktop

import (
	"bytes"
	"embed"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var nasscadExternalScriptPattern = regexp.MustCompile(`(?is)<script\b[^>]*\bsrc=["']([^"']+)["'][^>]*>\s*</script>`)

func isNasscadInlineSiblingAsset(src string) bool {
	src = strings.TrimSpace(src)
	if src == "" || strings.Contains(src, "://") || strings.HasPrefix(src, "//") || strings.HasPrefix(src, "/") {
		return false
	}
	if strings.Contains(src, "..") || strings.Contains(src, `\`) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(src))
	return ext == ".js" || ext == ".mjs"
}

func buildMonolithicNasscadHTML(indexHTML []byte, assets embed.FS, assetPrefix string) ([]byte, error) {
	if len(indexHTML) == 0 {
		return nil, fmt.Errorf("bundled nasscad index is empty")
	}
	assetPrefix = strings.TrimSuffix(strings.TrimSpace(assetPrefix), "/")
	var firstErr error
	out := nasscadExternalScriptPattern.ReplaceAllFunc(indexHTML, func(match []byte) []byte {
		parts := nasscadExternalScriptPattern.FindSubmatch(match)
		if len(parts) != 2 {
			return match
		}
		src := strings.TrimSpace(string(parts[1]))
		if !isNasscadInlineSiblingAsset(src) {
			return match
		}
		assetPath := assetPrefix + "/" + filepath.ToSlash(src)
		data, err := assets.ReadFile(assetPath)
		if err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("read bundled nasscad asset %s: %w", src, err)
			}
			return match
		}
		var buf bytes.Buffer
		buf.WriteString("<script>\n")
		buf.Write(data)
		buf.WriteString("\n</script>")
		return buf.Bytes()
	})
	if firstErr != nil {
		return out, firstErr
	}
	if !bytes.Contains(out, []byte("THREE")) {
		return out, fmt.Errorf("bundled nasscad html is missing inlined THREE runtime")
	}
	if !bytes.Contains(out, []byte("function nasLog")) {
		return out, fmt.Errorf("bundled nasscad html is missing inlined nasLog runtime")
	}
	return out, nil
}