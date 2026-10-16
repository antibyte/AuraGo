package mailer

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const maxMailHTMLBytes = 2 << 20

// SanitizeHTML keeps mail structure and inert formatting. Rendering still needs
// an isolated frame and an explicit remote-image decision in the client.
func SanitizeHTML(input string) (string, error) {
	if len(input) > maxMailHTMLBytes {
		return "", fmt.Errorf("%w: HTML body exceeds size limit", ErrInvalid)
	}
	nodes, err := html.ParseFragment(strings.NewReader(input), &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return "", fmt.Errorf("parse mail HTML: %w", err)
	}
	root := &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	count := 0
	var copyNode func(*html.Node, *html.Node, int) error
	copyNode = func(src, dest *html.Node, depth int) error {
		count++
		if count > 100000 || depth > 64 {
			return fmt.Errorf("%w: HTML structure exceeds limit", ErrInvalid)
		}
		switch src.Type {
		case html.TextNode:
			dest.AppendChild(&html.Node{Type: html.TextNode, Data: src.Data})
			return nil
		case html.ElementNode:
			if mailHTMLBlocked[src.Data] {
				return nil
			}
			if mailHTMLTags[src.Data] {
				next := &html.Node{Type: html.ElementNode, Data: src.Data, DataAtom: src.DataAtom}
				for _, a := range src.Attr {
					if a.Namespace == "" && safeMailAttribute(src.Data, a) {
						next.Attr = append(next.Attr, a)
					}
				}
				if src.Data == "a" {
					next.Attr = append(next.Attr, html.Attribute{Key: "rel", Val: "noopener noreferrer"}, html.Attribute{Key: "target", Val: "_blank"})
				}
				dest.AppendChild(next)
				dest = next
			}
		default:
			return nil
		}
		for child := src.FirstChild; child != nil; child = child.NextSibling {
			if err := copyNode(child, dest, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, node := range nodes {
		if err := copyNode(node, root, 0); err != nil {
			return "", err
		}
	}
	var out bytes.Buffer
	for child := root.FirstChild; child != nil; child = child.NextSibling {
		if err := html.Render(&out, child); err != nil {
			return "", fmt.Errorf("render mail HTML: %w", err)
		}
	}
	if out.Len() > maxMailHTMLBytes {
		return "", fmt.Errorf("%w: sanitized HTML exceeds size limit", ErrInvalid)
	}
	return out.String(), nil
}

var mailHTMLTags = map[string]bool{
	"a": true, "abbr": true, "b": true, "bdi": true, "bdo": true, "blockquote": true, "br": true, "caption": true, "code": true, "col": true, "colgroup": true, "dd": true,
	"del": true, "div": true, "dl": true, "dt": true, "em": true, "font": true, "h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "hr": true,
	"i": true, "img": true, "ins": true, "li": true, "ol": true, "p": true, "pre": true, "q": true, "s": true, "small": true, "span": true, "strong": true, "sub": true, "sup": true,
	"table": true, "tbody": true, "td": true, "tfoot": true, "th": true, "thead": true, "tr": true, "u": true, "ul": true,
}
var mailHTMLBlocked = map[string]bool{
	"script": true, "style": true, "iframe": true, "frame": true, "frameset": true, "object": true, "embed": true, "applet": true, "svg": true, "math": true, "template": true,
	"form": true, "input": true, "button": true, "textarea": true, "select": true, "option": true, "link": true, "meta": true, "base": true, "audio": true, "video": true, "source": true,
}

func safeMailAttribute(tag string, a html.Attribute) bool {
	if len(a.Val) > 2048 || strings.ContainsRune(a.Val, 0) {
		return false
	}
	switch a.Key {
	case "alt", "title":
		return true
	case "dir":
		return a.Val == "ltr" || a.Val == "rtl" || a.Val == "auto"
	case "lang":
		for _, r := range a.Val {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '-') {
				return false
			}
		}
		return len(a.Val) <= 40
	case "href":
		return tag == "a" && safeMailURL(a.Val, false)
	case "src":
		return tag == "img" && safeMailURL(a.Val, true)
	case "width", "height", "colspan", "rowspan", "cellpadding", "cellspacing", "border":
		if tag != "img" && tag != "table" && tag != "td" && tag != "th" && tag != "col" && tag != "colgroup" {
			return false
		}
		value := strings.TrimSuffix(a.Val, "%")
		n, err := strconv.Atoi(value)
		return err == nil && n >= 0 && n <= 4096
	case "align":
		return a.Val == "left" || a.Val == "right" || a.Val == "center" || a.Val == "justify"
	case "valign":
		return a.Val == "top" || a.Val == "middle" || a.Val == "bottom"
	case "color", "bgcolor":
		return len(a.Val) == 7 && a.Val[0] == '#' && strings.Trim(a.Val[1:], "0123456789abcdefABCDEF") == ""
	case "size":
		n, err := strconv.Atoi(a.Val)
		return tag == "font" && err == nil && n >= 1 && n <= 7
	case "start":
		_, err := strconv.Atoi(a.Val)
		return tag == "ol" && err == nil
	}
	return false
}

func safeMailURL(raw string, image bool) bool {
	for _, r := range raw {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "https", "http":
		return u.Hostname() != ""
	case "cid":
		return image && u.Opaque != "" && len(u.Opaque) <= 256 && !strings.ContainsAny(u.Opaque, "<>\"'")
	case "mailto":
		return !image && u.Opaque != "" && u.Fragment == ""
	}
	return false
}
