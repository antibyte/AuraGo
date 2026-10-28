package scraper

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/atom"
)

var hiddenStyle = regexp.MustCompile(`(?i)(?:display:none|visibility:(?:hidden|collapse)|opacity:0(?:\.0+)?(?:[;!]|$)|font-size:0(?:\.0+)?(?:px|em|rem|%|pt)?(?:[;!]|$)|(?:^|;)color:(?:transparent|rgba\([^)]*,0(?:\.0+)?\)|[a-z]+\([^)]*/0(?:\.0+)?\))|(?:left|top|text-indent):-[0-9]{3,}(?:px|em)|clip:rect\((?:0(?:px)?[,]?){4}\)|clip-path:inset\((?:50|100)%\)|transform:scale\(0\))`)
var zeroDimension = regexp.MustCompile(`(?:^|;)(?:height|width):0(?:px|em|rem)?(?:[;!]|$)`)
var farPosition = regexp.MustCompile(`(?:^|;)(?:left|top):[0-9]{5,}px`)
var collapsedPanelStyle = regexp.MustCompile(`(?:^|;)(?:display:none|visibility:hidden)(?:!important)?(?:;|$)`)

// CleanHiddenHTML is shared by article, selector and crawler extraction. It operates
// on a parsed copy; accessible panels and noscript content remain readable.
func CleanHiddenHTML(raw string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(raw))
	if err != nil {
		return raw
	}
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil {
			return
		}
		if n.Data == "noscript" {
			n.Data = "div"
			n.DataAtom = atom.Div
			return
		}
		role, _ := s.Attr("role")
		_, labelled := s.Attr("aria-labelledby")
		style, _ := s.Attr("style")
		style = strings.ToLower(strings.Join(strings.Fields(style), ""))
		_, hidden := s.Attr("hidden")
		aria, _ := s.Attr("aria-hidden")
		if role == "region" || role == "tabpanel" || (labelled && s.Is("div,section")) {
			hidden, aria = false, ""
			style = collapsedPanelStyle.ReplaceAllString(style, ";")
			s.RemoveAttr("hidden").RemoveAttr("aria-hidden").SetAttr("style", style)
		}
		classes, _ := s.Attr("class")
		screenReader := false
		for _, c := range strings.Fields(classes) {
			if c == "sr-only" || c == "visually-hidden" || c == "screen-reader-only" || c == "screen-reader-text" {
				screenReader = true
			}
		}
		shortLabel := screenReader && utf8.RuneCountInString(strings.TrimSpace(s.Text())) <= 40
		if !shortLabel && (hidden || strings.EqualFold(aria, "true") || hiddenStyle.MatchString(style) || (zeroDimension.MatchString(style) && (strings.Contains(style, "overflow:hidden") || strings.Contains(style, "overflow:clip"))) || (farPosition.MatchString(style) && (strings.Contains(style, "position:absolute") || strings.Contains(style, "position:fixed"))) || screenReader) {
			s.Remove()
		}
	})
	clean, err := doc.Html()
	if err != nil {
		return raw
	}
	return clean
}
