package office

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Excelize owns chart updates; foreign pictures and drawing extensions stay verbatim.
func preserveEditorImageDrawings(original, updated map[string][]byte, chartsChanged bool) error {
	for name, raw := range original {
		if !strings.HasPrefix(name, "xl/drawings/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		if !chartsChanged || len(updated[name]) == 0 {
			updated[name] = raw
			if rel := officeRelationshipPart(name); len(original[rel]) > 0 {
				updated[rel] = original[rel]
			}
			continue
		}
		old, head, tail, err := officeXMLChildren(raw)
		if err != nil {
			return err
		}
		fresh, _, _, err := officeXMLChildren(updated[name])
		if err != nil {
			return err
		}
		isChart := func(data []byte) bool {
			var anchor struct {
				Chart struct {
					ID string `xml:"id,attr"`
				} `xml:"graphicFrame>graphic>graphicData>chart"`
			}
			return xml.Unmarshal(data, &anchor) == nil && anchor.Chart.ID != ""
		}
		var combined bytes.Buffer
		combined.Write(raw[:head])
		for _, child := range old {
			if childRaw := raw[child.Start:child.End]; !isChart(childRaw) {
				combined.Write(childRaw)
			}
		}
		for _, child := range fresh {
			if childRaw := updated[name][child.Start:child.End]; isChart(childRaw) {
				combined.Write(childRaw)
			}
		}
		combined.Write(raw[tail:])
		updated[name] = mergeXMLAttributes(updated[name], append([]byte{}, combined.Bytes()...), map[string]bool{})
		relPart := officeRelationshipPart(name)
		previous := expandEmptyOfficeRoot(original[relPart])
		if len(previous) == 0 {
			continue
		}
		oldRels, relHead, relTail, err := officeXMLChildren(previous)
		if err != nil {
			return err
		}
		newRels, _, _, err := officeXMLChildren(expandEmptyOfficeRoot(updated[relPart]))
		if err != nil {
			return err
		}
		combined.Reset()
		combined.Write(previous[:relHead])
		ids := map[string]bool{}
		appendRel := func(rel []byte) error {
			id := xmlAttribute(rel, "Id")
			if id == "" || ids[id] {
				return fmt.Errorf("drawing relationship identity could not be preserved")
			}
			ids[id] = true
			combined.Write(rel)
			return nil
		}
		for _, child := range oldRels {
			rel := previous[child.Start:child.End]
			if !strings.HasSuffix(xmlAttribute(rel, "Type"), "/chart") {
				if err := appendRel(rel); err != nil {
					return err
				}
			}
		}
		for _, child := range newRels {
			rel := updated[relPart][child.Start:child.End]
			if strings.HasSuffix(xmlAttribute(rel, "Type"), "/chart") {
				if err := appendRel(rel); err != nil {
					return err
				}
			}
		}
		combined.Write(previous[relTail:])
		updated[relPart] = combined.Bytes()
	}
	return nil
}
