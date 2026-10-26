package server

import (
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	homepageAnnotationMaxItems    = 20
	homepageAnnotationMaxComment  = 1000
	homepageAnnotationMaxSelector = 300
	homepageAnnotationMaxText     = 120
	homepageAnnotationMaxAttr     = 200
	homepageAnnotationMaxAttrs    = 8
	homepageAnnotationMaxOverlaps = 5
	homepageAnnotationPromptBytes = 24000
)

// homepageAnnotationImageUnavailableNote replaces the image analysis when the
// fallback vision model cannot read the marked-places image.
const homepageAnnotationImageUnavailableNote = "(The marked-places image could not be analyzed; use the mark details from the Homepage Studio context.)"

// desktopChatAnnotations carries the marks Homepage Studio sends with a turn.
type desktopChatAnnotations struct {
	PagePath      string                  `json:"page_path"`
	ViewportWidth int                     `json:"viewport_width"`
	Items         []desktopChatAnnotation `json:"items"`
	// ImageOmitted is set by the server when the composed image cannot be used.
	ImageOmitted bool `json:"-"`
}

type desktopChatAnnotation struct {
	N        int                            `json:"n"`
	Kind     string                         `json:"kind"`
	Comment  string                         `json:"comment"`
	Selector string                         `json:"selector,omitempty"`
	Tag      string                         `json:"tag,omitempty"`
	Text     string                         `json:"text,omitempty"`
	Attrs    map[string]string              `json:"attrs,omitempty"`
	Box      []float64                      `json:"box,omitempty"`
	Elements []desktopChatAnnotationElement `json:"elements,omitempty"`
}

type desktopChatAnnotationElement struct {
	Selector string `json:"selector"`
	Tag      string `json:"tag"`
	Text     string `json:"text,omitempty"`
}

func (a *desktopChatAnnotations) hasItems() bool {
	return a != nil && len(a.Items) > 0
}

func clipAnnotationText(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= max {
		return value
	}
	runes := []rune(value)
	return string(runes[:max-1]) + "…"
}

// homepageAnnotationsPrompt renders the marks for the agent. Selectors, texts
// and attributes come from the rendered page and comments from the client, so
// the whole detail block is wrapped as untrusted external data.
func homepageAnnotationsPrompt(a *desktopChatAnnotations) string {
	if !a.hasItems() {
		return ""
	}
	var details strings.Builder
	fmt.Fprintf(&details, "page %s at %d px\n", clipAnnotationText(a.PagePath, homepageAnnotationMaxSelector), a.ViewportWidth)
	for i, item := range a.Items {
		if i >= homepageAnnotationMaxItems {
			break
		}
		kind := "area"
		if item.Kind == "element" {
			kind = "element"
		}
		fmt.Fprintf(&details, "#%d %s", item.N, kind)
		if kind == "element" {
			fmt.Fprintf(&details, " %s <%s>", clipAnnotationText(item.Selector, homepageAnnotationMaxSelector), clipAnnotationText(item.Tag, 20))
			if text := clipAnnotationText(item.Text, homepageAnnotationMaxText); text != "" {
				fmt.Fprintf(&details, " text %q", text)
			}
			if attrs := homepageAnnotationAttrs(item.Attrs); attrs != "" {
				details.WriteString(" " + attrs)
			}
		}
		if box := homepageAnnotationBox(item.Box); box != "" {
			details.WriteString(" box " + box)
		}
		details.WriteString("\n")
		if kind == "area" {
			for j, el := range item.Elements {
				if j >= homepageAnnotationMaxOverlaps {
					break
				}
				fmt.Fprintf(&details, "   overlaps %s <%s>", clipAnnotationText(el.Selector, homepageAnnotationMaxSelector), clipAnnotationText(el.Tag, 20))
				if text := clipAnnotationText(el.Text, homepageAnnotationMaxText); text != "" {
					fmt.Fprintf(&details, " text %q", text)
				}
				details.WriteString("\n")
			}
		}
		fmt.Fprintf(&details, "   comment: %s\n", clipAnnotationText(item.Comment, homepageAnnotationMaxComment))
	}

	var b strings.Builder
	b.WriteString("\n\nThe user marked places on a snapshot of the previewed page. The numbers match the \"Marks\" list in the user message")
	if a.ImageOmitted {
		b.WriteString("; the marked-places image could not be attached, so rely on the details below")
	} else {
		b.WriteString(" and the numbered crops in the attached image")
	}
	b.WriteString(". Selectors and texts were read from the rendered page; find the source with homepage_file search for the text, id or class before editing.\n")
	b.WriteString(desktopExternalData("homepage_annotations", details.String(), homepageAnnotationPromptBytes))
	return b.String()
}

func homepageAnnotationAttrs(attrs map[string]string) string {
	if len(attrs) == 0 {
		return ""
	}
	keys := make([]string, 0, len(attrs))
	for k := range attrs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for i, k := range keys {
		if i >= homepageAnnotationMaxAttrs {
			break
		}
		parts = append(parts, fmt.Sprintf("%s=%q", clipAnnotationText(k, 40), clipAnnotationText(attrs[k], homepageAnnotationMaxAttr)))
	}
	return strings.Join(parts, " ")
}

func homepageAnnotationBox(box []float64) string {
	if len(box) != 4 {
		return ""
	}
	return fmt.Sprintf("%.0f,%.0f %.0fx%.0f", box[0], box[1], box[2], box[3])
}
