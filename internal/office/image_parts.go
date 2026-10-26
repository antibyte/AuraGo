package office

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path"
	"strconv"
	"strings"
)

const officeImageLimit = 10 << 20
const officeImagePixels = 32_000_000
const officeImageRelationship = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"

func officeImageInfo(data []byte, width, height int) (string, string, int, int, error) {
	if len(data) == 0 || len(data) > officeImageLimit {
		return "", "", 0, 0, fmt.Errorf("image exceeds the 10 MiB limit")
	}
	config, kind, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || (kind != "png" && kind != "jpeg") || config.Width < 1 || config.Height < 1 || config.Width > officeImagePixels/config.Height {
		return "", "", 0, 0, fmt.Errorf("image must be a PNG or JPEG of at most 32 megapixels")
	}
	if _, decodedKind, err := image.Decode(bytes.NewReader(data)); err != nil || decodedKind != kind {
		return "", "", 0, 0, fmt.Errorf("image bytes are incomplete or invalid")
	}
	if width == 0 && height == 0 {
		width, height = config.Width, config.Height
		if width > 4096 || height > 4096 {
			return "", "", 0, 0, fmt.Errorf("specify insertion dimensions for images larger than 4096 pixels")
		}
	}
	if width < 1 || height < 1 || width > 4096 || height > 4096 {
		return "", "", 0, 0, fmt.Errorf("image insertion dimensions must be between 1 and 4096 pixels")
	}
	ext := ".png"
	if kind == "jpeg" {
		ext = ".jpeg"
	}
	return ext, "image/" + kind, width, height, nil
}

func officeRelationshipPart(part string) string {
	return path.Join(path.Dir(part), "_rels", path.Base(part)+".rels")
}

func relativeOfficePart(owner, target string) string {
	if path.Dir(owner) == "." {
		return target
	}
	from, to := strings.Split(path.Dir(owner), "/"), strings.Split(target, "/")
	common := 0
	for common < len(from) && common < len(to) && from[common] == to[common] {
		common++
	}
	return strings.Repeat("../", len(from)-common) + strings.Join(to[common:], "/")
}

func officeRelationshipIDs(raw []byte) (map[string]bool, error) {
	children, _, _, err := officeXMLChildren(raw)
	if err != nil {
		return nil, err
	}
	ids := map[string]bool{}
	for _, child := range children {
		id := xmlAttribute(raw[child.Start:child.End], "Id")
		if id == "" || ids[id] {
			return nil, fmt.Errorf("invalid or duplicate Office relationship identity")
		}
		ids[id] = true
	}
	return ids, nil
}

func removeOfficeChildByAttribute(raw []byte, attribute, value string) ([]byte, error) {
	children, _, _, err := officeXMLChildren(raw)
	if err != nil {
		return nil, err
	}
	for i := len(children) - 1; i >= 0; i-- {
		child := children[i]
		if xmlAttribute(raw[child.Start:child.End], attribute) == value {
			raw = append(append([]byte{}, raw[:child.Start]...), raw[child.End:]...)
		}
	}
	return raw, nil
}

func officeXMLReferences(raw []byte, id string) (bool, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if start, ok := token.(xml.StartElement); ok {
			for _, attribute := range start.Attr {
				if attribute.Value == id {
					return true, nil
				}
			}
		}
	}
}

func removeUnreferencedOfficeImage(parts map[string][]byte, drawing, relID, media string) error {
	referenced, err := officeXMLReferences(parts[drawing], relID)
	if err != nil || referenced {
		return err
	}
	relPart := officeRelationshipPart(drawing)
	parts[relPart], err = removeOfficeChildByAttribute(parts[relPart], "Id", relID)
	if err != nil {
		return err
	}
	for name, content := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		var relationships struct {
			Items []struct {
				Target string `xml:"Target,attr"`
				Mode   string `xml:"TargetMode,attr"`
			} `xml:"Relationship"`
		}
		if err := xml.Unmarshal(content, &relationships); err != nil {
			return err
		}
		owner := path.Join(path.Dir(path.Dir(name)), strings.TrimSuffix(path.Base(name), ".rels"))
		for _, relationship := range relationships.Items {
			if relationship.Mode != "External" && resolveOfficePart(owner, relationship.Target) == media {
				return nil
			}
		}
	}
	delete(parts, media)
	parts["[Content_Types].xml"], err = removeOfficeChildByAttribute(parts["[Content_Types].xml"], "PartName", "/"+media)
	return err
}

func nextOfficeDrawingID(raw []byte) (int, error) {
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	next := 1
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return next, nil
		}
		if err != nil {
			return 0, err
		}
		if start, ok := token.(xml.StartElement); ok && (start.Name.Local == "docPr" || start.Name.Local == "cNvPr") {
			for _, attribute := range start.Attr {
				if attribute.Name.Local == "id" {
					value, err := strconv.Atoi(attribute.Value)
					if err != nil || value < 0 || value >= 1<<30 {
						return 0, fmt.Errorf("invalid Office drawing identity")
					}
					if value >= next {
						next = value + 1
					}
				}
			}
		}
	}
}

func writeOfficeImageParts(parts map[string][]byte) ([]byte, error) {
	remaining := 128 << 20
	if len(parts) > 4096 {
		return nil, fmt.Errorf("Office package contains too many parts")
	}
	for name, content := range parts {
		if strings.HasSuffix(strings.ToLower(name), ".xml") && len(content) > 16<<20 {
			return nil, fmt.Errorf("Office XML part exceeds limit")
		}
		remaining -= len(content)
		if remaining < 0 {
			return nil, fmt.Errorf("Office expanded size exceeds limit")
		}
	}
	return writeOfficeParts(parts)
}

func appendOfficeChild(raw, child []byte, root string) ([]byte, error) {
	raw = expandEmptyOfficeRoot(raw)
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	depth, tail, roots := 0, -1, 0
	for {
		offset := int(decoder.InputOffset())
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("invalid Office XML: %w", err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots != 1 || value.Name.Local != root {
					return nil, fmt.Errorf("invalid Office XML root")
				}
				expected := map[string]string{
					"Relationships": "http://schemas.openxmlformats.org/package/2006/relationships",
					"Types":         "http://schemas.openxmlformats.org/package/2006/content-types",
					"wsDr":          "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing",
					"worksheet":     "http://schemas.openxmlformats.org/spreadsheetml/2006/main",
				}[root]
				if expected != "" && value.Name.Space != expected {
					return nil, fmt.Errorf("unsupported Office XML namespace")
				}
			}
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 {
				tail = offset
			}
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(value)) != 0 {
				return nil, fmt.Errorf("text outside Office XML root")
			}
		}
	}
	if tail < 0 {
		return nil, fmt.Errorf("Office XML root is missing")
	}
	return append(append(append([]byte{}, raw[:tail]...), child...), raw[tail:]...), nil
}

func addOfficeImagePart(parts map[string][]byte, owner, dir, ext, mime string, data []byte) (string, string, error) {
	media := ""
	for i := 1; i <= 4096; i++ {
		candidate := path.Join(dir, "aurago-image-"+strconv.Itoa(i)+ext)
		if _, exists := parts[candidate]; !exists {
			media = candidate
			break
		}
	}
	if media == "" || len(parts) > 4092 {
		return "", "", fmt.Errorf("Office package contains too many parts")
	}
	relPart := officeRelationshipPart(owner)
	relationships := parts[relPart]
	if len(relationships) == 0 {
		relationships = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`)
	}
	used, err := officeRelationshipIDs(relationships)
	if err != nil {
		return "", "", err
	}
	relID := ""
	for i := 1; i <= 4096; i++ {
		candidate := "rIdAuraGoImage" + strconv.Itoa(i)
		if _, exists := used[candidate]; !exists {
			relID = candidate
			break
		}
	}
	if relID == "" {
		return "", "", fmt.Errorf("Office drawing contains too many relationships")
	}
	target := relativeOfficePart(owner, media)
	child := []byte(`<Relationship xmlns="http://schemas.openxmlformats.org/package/2006/relationships" Id="` + relID + `" Type="` + officeImageRelationship + `" Target="` + xmlText(target) + `"/>`)
	updated, err := appendOfficeChild(relationships, child, "Relationships")
	if err != nil {
		return "", "", err
	}
	types, err := appendOfficeChild(parts["[Content_Types].xml"], []byte(`<Override xmlns="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/`+xmlText(media)+`" ContentType="`+mime+`"/>`), "Types")
	if err != nil {
		return "", "", err
	}
	parts[media], parts[relPart], parts["[Content_Types].xml"] = append([]byte{}, data...), updated, types
	return media, relID, nil
}
