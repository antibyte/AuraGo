package office

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"math"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

// EditorImage embeds its pixels; display and anchor dimensions use 96-DPI pixels.
type EditorImage struct {
	ID       string `json:"id"`
	Sheet    string `json:"sheet"`
	Anchor   string `json:"anchor"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	MIME     string `json:"mime"`
	Data     string `json:"data"`
	Alt      string `json:"alt"`
	Readonly bool   `json:"readonly,omitempty"`
}

type workbookPicture struct {
	image                 EditorImage
	drawing, media, relID string
	start, end            int
	raw                   []byte
}

type pictureAnchor struct {
	XMLName xml.Name
	From    struct {
		Col    int `xml:"col"`
		Row    int `xml:"row"`
		ColOff int `xml:"colOff"`
		RowOff int `xml:"rowOff"`
	} `xml:"from"`
	Ext struct {
		CX int64 `xml:"cx,attr"`
		CY int64 `xml:"cy,attr"`
	} `xml:"ext"`
	Picture struct {
		Properties struct {
			ID   int    `xml:"id,attr"`
			Name string `xml:"name,attr"`
			Alt  string `xml:"descr,attr"`
		} `xml:"nvPicPr>cNvPr"`
		Blip struct {
			ID string `xml:"embed,attr"`
		} `xml:"blipFill>blip"`
		Size struct {
			CX int64 `xml:"cx,attr"`
			CY int64 `xml:"cy,attr"`
		} `xml:"spPr>xfrm>ext"`
	} `xml:"pic"`
}

var editorImageID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func editableWorkbookPicture(raw []byte, anchor pictureAnchor) bool {
	if anchor.Picture.Properties.ID < 1 || anchor.Ext.CX < 9525 || anchor.Ext.CX > 4096*9525 || anchor.Ext.CY < 9525 || anchor.Ext.CY > 4096*9525 || len(anchor.Picture.Properties.Alt) > 2048 || math.Abs(float64(anchor.From.ColOff)) > 1000000*9525 || math.Abs(float64(anchor.From.RowOff)) > 1000000*9525 {
		return false
	}
	if anchor.Ext.CX != anchor.Picture.Size.CX || anchor.Ext.CY != anchor.Picture.Size.CY || anchor.Ext.CX%9525 != 0 || anchor.Ext.CY%9525 != 0 || anchor.From.ColOff%9525 != 0 || anchor.From.RowOff%9525 != 0 {
		return false
	}
	allowed := map[string]string{"oneCellAnchor": "", "from": "", "col": "", "row": "", "colOff": "", "rowOff": "", "ext": "cx cy", "pic": "", "nvPicPr": "", "cNvPr": "id name descr", "cNvPicPr": "", "picLocks": "noChangeAspect", "blipFill": "", "blip": "embed", "stretch": "", "fillRect": "", "spPr": "", "xfrm": "", "off": "x y", "prstGeom": "prst", "avLst": "", "clientData": ""}
	decoder := xml.NewDecoder(bytes.NewReader(raw))
	for {
		token, err := decoder.Token()
		if err != nil {
			return err == io.EOF
		}
		if start, ok := token.(xml.StartElement); ok {
			if start.Name.Space != "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" && start.Name.Space != "http://schemas.openxmlformats.org/drawingml/2006/main" {
				return false
			}
			attributes, ok := allowed[start.Name.Local]
			if !ok {
				return false
			}
			for _, attribute := range start.Attr {
				if attribute.Name.Local == "xmlns" || attribute.Name.Space == "xmlns" {
					continue
				}
				if !strings.Contains(" "+attributes+" ", " "+attribute.Name.Local+" ") {
					return false
				}
				if attribute.Name.Space != "" && attribute.Name.Space != "http://schemas.openxmlformats.org/officeDocument/2006/relationships" {
					return false
				}
				if start.Name.Local == "prstGeom" && attribute.Name.Local == "prst" && attribute.Value != "rect" {
					return false
				}
				if start.Name.Local == "off" && attribute.Value != "0" {
					return false
				}
				if start.Name.Local == "picLocks" && attribute.Value != "1" {
					return false
				}
			}
		}
	}
}

func readWorkbookPictures(parts map[string][]byte, book EditorWorkbook) ([]workbookPicture, bool, error) {
	result := []workbookPicture{}
	locked, total := false, 0
	for _, id := range book.SheetOrder {
		sheet := book.Sheets[id]
		sheetPart := sheetPartByName(parts, sheet.Name)
		var worksheet struct {
			Drawing struct {
				ID string `xml:"id,attr"`
			} `xml:"drawing"`
		}
		if err := xml.Unmarshal(parts[sheetPart], &worksheet); err != nil {
			return nil, false, err
		}
		if worksheet.Drawing.ID == "" {
			continue
		}
		drawing := resolveOfficePart(sheetPart, officeRelationships(parts[officeRelationshipPart(sheetPart)])[worksheet.Drawing.ID])
		if !strings.HasPrefix(drawing, "xl/drawings/") || len(parts[drawing]) == 0 {
			return nil, false, fmt.Errorf("invalid worksheet drawing")
		}
		children, _, _, err := officeXMLChildren(parts[drawing])
		if err != nil {
			return nil, false, err
		}
		rels := officeRelationships(parts[officeRelationshipPart(drawing)])
		for _, child := range children {
			raw := parts[drawing][child.Start:child.End]
			var anchor pictureAnchor
			if err := xml.Unmarshal(raw, &anchor); err != nil {
				return nil, false, err
			}
			if anchor.Picture.Blip.ID == "" {
				if !bytes.Contains(raw, []byte(":chart")) && !bytes.Contains(raw, []byte("<chart")) {
					locked = true
				}
				continue
			}
			if len(result) >= 256 {
				return nil, false, fmt.Errorf("workbook contains more than 256 images")
			}
			media := resolveOfficePart(drawing, rels[anchor.Picture.Blip.ID])
			pixels := parts[media]
			ownedID := strings.TrimPrefix(anchor.Picture.Properties.Name, "AuraGo image ")
			owned := ownedID != anchor.Picture.Properties.Name && editorImageID.MatchString(ownedID) && child.Name == "oneCellAnchor" && editableWorkbookPicture(raw, anchor)
			if !owned {
				ownedID = drawing + "#" + strconv.Itoa(anchor.Picture.Properties.ID)
			}
			value := EditorImage{ID: ownedID, Sheet: id, Anchor: "A1", Width: 1, Height: 1, Alt: anchor.Picture.Properties.Alt, Readonly: !owned}
			value.Anchor, err = excelize.CoordinatesToCellName(anchor.From.Col+1, anchor.From.Row+1)
			if err != nil {
				return nil, false, err
			}
			value.X, value.Y = anchor.From.ColOff/9525, anchor.From.RowOff/9525
			cx, cy := anchor.Ext.CX, anchor.Ext.CY
			if cx == 0 || cy == 0 {
				cx, cy = anchor.Picture.Size.CX, anchor.Picture.Size.CY
			}
			value.Width, value.Height = max(1, int(cx/9525)), max(1, int(cy/9525))
			config, kind, decodeErr := image.DecodeConfig(bytes.NewReader(pixels))
			if len(pixels) > 0 && len(pixels) <= officeImageLimit && total+len(pixels) <= 16<<20 && decodeErr == nil && (kind == "png" || kind == "jpeg") && config.Width > 0 && config.Height > 0 && config.Width <= officeImagePixels/config.Height {
				value.MIME, value.Data = "image/"+kind, base64.StdEncoding.EncodeToString(pixels)
				total += len(pixels)
			} else {
				value.Readonly = true
			}
			if value.Readonly {
				locked = true
			}
			result = append(result, workbookPicture{image: value, drawing: drawing, media: media, relID: anchor.Picture.Blip.ID, start: child.Start, end: child.End, raw: raw})
		}
	}
	return result, locked, nil
}

func applyEditorImageParts(original, updated map[string][]byte, before, after EditorWorkbook, images []EditorImage) error {
	pictures, _, err := readWorkbookPictures(original, before)
	if err != nil && len(original) > 0 {
		return err
	}
	previous := map[string]workbookPicture{}
	for _, picture := range pictures {
		if _, ok := previous[picture.image.ID]; ok {
			return fmt.Errorf("duplicate workbook image identity")
		}
		previous[picture.image.ID] = picture
	}
	if len(images) > 256 {
		return fmt.Errorf("too many workbook images")
	}
	next := map[string]EditorImage{}
	total := 0
	for _, value := range images {
		if value.ID == "" || len(value.ID) > 256 || next[value.ID].ID != "" || after.Sheets[value.Sheet] == nil {
			return fmt.Errorf("invalid workbook image identity or worksheet")
		}
		total += base64.StdEncoding.DecodedLen(len(value.Data))
		if total > 16<<20 {
			return fmt.Errorf("workbook images exceed the 16 MiB aggregate limit")
		}
		if old, ok := previous[value.ID]; ok && old.image.Readonly {
			if !reflect.DeepEqual(old.image, value) {
				return fmt.Errorf("this image is preserved and cannot be edited")
			}
			next[value.ID] = value
			continue
		}
		if value.Width < 1 || value.Width > 4096 || value.Height < 1 || value.Height > 4096 || math.Abs(float64(value.X)) > 1000000 || math.Abs(float64(value.Y)) > 1000000 || len(value.Alt) > 2048 {
			return fmt.Errorf("workbook image exceeds limits")
		}
		if col, row, e := excelize.CellNameToCoordinates(value.Anchor); e != nil || col < 1 || col > 16384 || row < 1 || row > 1048576 {
			return fmt.Errorf("invalid image cell anchor")
		}
		next[value.ID] = value
		if old, ok := previous[value.ID]; ok {
			if old.image.Readonly && !reflect.DeepEqual(old.image, value) {
				return fmt.Errorf("this image is preserved and cannot be edited")
			}
			if value.Data != old.image.Data || value.MIME != old.image.MIME || value.Alt != old.image.Alt || value.Sheet != old.image.Sheet || value.Readonly != old.image.Readonly {
				return fmt.Errorf("existing image content must remain unchanged")
			}
		} else if !editorImageID.MatchString(value.ID) || value.Readonly {
			return fmt.Errorf("invalid new image identity")
		}
	}
	type replacement struct {
		start, end int
		data       []byte
	}
	changes := map[string][]replacement{}
	currentPictures, _, err := readWorkbookPictures(updated, after)
	if err != nil {
		return err
	}
	current := map[string]workbookPicture{}
	for _, picture := range currentPictures {
		current[picture.image.ID] = picture
	}
	for _, old := range pictures {
		value, exists := next[old.image.ID]
		if !exists && old.image.Readonly {
			if after.Sheets[old.image.Sheet] != nil {
				continue
			}
			return fmt.Errorf("cannot remove a sheet containing a preserved image")
		}
		if exists && reflect.DeepEqual(old.image, value) {
			continue
		}
		if after.Sheets[old.image.Sheet] == nil {
			continue
		}
		present, ok := current[old.image.ID]
		if !ok {
			return fmt.Errorf("existing image drawing could not be preserved")
		}
		var raw []byte
		if exists {
			var parsed pictureAnchor
			if err = xml.Unmarshal(old.raw, &parsed); err != nil {
				return err
			}
			col, row, _ := excelize.CellNameToCoordinates(value.Anchor)
			raw = workbookImageAnchor(value.ID, parsed.Picture.Properties.ID, row-1, col-1, value.X, value.Y, value.Width, value.Height, value.Alt, old.relID)
		}
		changes[present.drawing] = append(changes[present.drawing], replacement{present.start, present.end, raw})
	}
	for drawing, list := range changes {
		raw := updated[drawing]
		sort.Slice(list, func(i, j int) bool { return list[i].start > list[j].start })
		for _, change := range list {
			raw = append(append(append([]byte{}, raw[:change.start]...), change.data...), raw[change.end:]...)
		}
		updated[drawing] = raw
	}
	for _, value := range images {
		if _, ok := previous[value.ID]; ok {
			continue
		}
		pixels, e := base64.StdEncoding.DecodeString(value.Data)
		if e != nil {
			return fmt.Errorf("invalid image bytes")
		}
		ext, mime, width, height, e := officeImageInfo(pixels, value.Width, value.Height)
		if e != nil {
			return e
		}
		if value.MIME != mime {
			return fmt.Errorf("image MIME does not match its bytes")
		}
		if err = appendWorkbookImage(updated, sheetPartByName(updated, after.Sheets[value.Sheet].Name), value.ID, value.Anchor, value.X, value.Y, width, height, value.Alt, pixels, ext, mime); err != nil {
			return err
		}
	}
	for _, old := range pictures {
		if _, exists := next[old.image.ID]; !exists && !old.image.Readonly && after.Sheets[old.image.Sheet] != nil {
			if err := removeUnreferencedOfficeImage(updated, old.drawing, old.relID, old.media); err != nil {
				return err
			}
		}
	}
	return nil
}

// WorkbookImageOptions uses 96-DPI pixels; Sheet is the displayed worksheet name.
type WorkbookImageOptions struct {
	Sheet  string
	Anchor string
	Width  int
	Height int
	Alt    string
}

// InsertWorkbookImage adds a picture without projecting or rebuilding the workbook.
// Callers must publish the returned package through their conditional-write path.
func InsertWorkbookImage(original, image []byte, options WorkbookImageOptions) ([]byte, error) {
	ext, mime, width, height, err := officeImageInfo(image, options.Width, options.Height)
	if err != nil {
		return nil, err
	}
	if len(options.Alt) > 2048 {
		return nil, fmt.Errorf("image alt text must not exceed 2048 bytes")
	}
	doc, err := DecodeEditorWorkbook(original)
	if err != nil {
		return nil, err
	}
	total := len(image)
	for _, existing := range doc.Images {
		total += base64.StdEncoding.DecodedLen(len(existing.Data))
	}
	if len(doc.Images) >= 256 || total > 16<<20 {
		return nil, fmt.Errorf("workbook image count or aggregate bytes exceed limits")
	}
	parts, err := readOfficeParts(original, "xl/workbook.xml")
	if err != nil {
		return nil, err
	}
	sheetPart := sheetPartByName(parts, options.Sheet)
	if sheetPart == "" {
		return nil, fmt.Errorf("image worksheet does not exist")
	}
	if err = appendWorkbookImage(parts, sheetPart, rand.Text(), options.Anchor, 0, 0, width, height, options.Alt, image, ext, mime); err != nil {
		return nil, err
	}
	return writeOfficeImageParts(parts)
}

func appendWorkbookImage(parts map[string][]byte, sheetPart, id, anchor string, x, y, width, height int, alt string, data []byte, ext, mime string) error {
	col, row, err := excelize.CellNameToCoordinates(anchor)
	if err != nil || col < 1 || col > 16384 || row < 1 || row > 1048576 {
		return fmt.Errorf("invalid image cell anchor")
	}
	var worksheet struct {
		Drawings []struct {
			ID string `xml:"id,attr"`
		} `xml:"drawing"`
	}
	if err = xml.Unmarshal(parts[sheetPart], &worksheet); err != nil || len(worksheet.Drawings) > 1 {
		return fmt.Errorf("unsupported worksheet drawing references")
	}
	drawingPart := ""
	if len(worksheet.Drawings) == 1 {
		rels := officeRelationships(parts[officeRelationshipPart(sheetPart)])
		drawingPart = resolveOfficePart(sheetPart, rels[worksheet.Drawings[0].ID])
		if !strings.HasPrefix(drawingPart, "xl/drawings/") || len(parts[drawingPart]) == 0 {
			return fmt.Errorf("worksheet drawing relationship is invalid")
		}
	} else {
		for i := 1; i <= 4096; i++ {
			candidate := "xl/drawings/aurago-drawing-" + strconv.Itoa(i) + ".xml"
			if _, exists := parts[candidate]; !exists {
				drawingPart = candidate
				break
			}
		}
		if drawingPart == "" {
			return fmt.Errorf("too many worksheet drawings")
		}
		relPart := officeRelationshipPart(sheetPart)
		rels := parts[relPart]
		if len(rels) == 0 {
			rels = []byte(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`)
		}
		used, err := officeRelationshipIDs(rels)
		if err != nil {
			return err
		}
		relID := ""
		for i := 1; i <= 4096; i++ {
			candidate := "rIdAuraGoDrawing" + strconv.Itoa(i)
			if _, exists := used[candidate]; !exists {
				relID = candidate
				break
			}
		}
		if relID == "" {
			return fmt.Errorf("too many worksheet relationships")
		}
		rels, err = appendOfficeChild(rels, []byte(`<Relationship xmlns="http://schemas.openxmlformats.org/package/2006/relationships" Id="`+relID+`" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/drawing" Target="`+xmlText(relativeOfficePart(sheetPart, drawingPart))+`"/>`), "Relationships")
		if err != nil {
			return err
		}
		sheet, err := appendOfficeChild(parts[sheetPart], []byte(`<drawing xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="`+relID+`"/>`), "worksheet")
		if err != nil {
			return err
		}
		sheet, err = orderedOfficeChildren(sheet)
		if err != nil {
			return err
		}
		types, err := appendOfficeChild(parts["[Content_Types].xml"], []byte(`<Override xmlns="http://schemas.openxmlformats.org/package/2006/content-types" PartName="/`+drawingPart+`" ContentType="application/vnd.openxmlformats-officedocument.drawing+xml"/>`), "Types")
		if err != nil {
			return err
		}
		parts[sheetPart], parts[relPart], parts["[Content_Types].xml"] = sheet, rels, types
		parts[drawingPart] = []byte(`<xdr:wsDr xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"></xdr:wsDr>`)
	}
	_, relID, err := addOfficeImagePart(parts, drawingPart, "xl/media", ext, mime, data)
	if err != nil {
		return err
	}
	nvID, err := nextOfficeDrawingID(parts[drawingPart])
	if err != nil {
		return err
	}
	anchorXML := workbookImageAnchor(id, nvID, row-1, col-1, x, y, width, height, alt, relID)
	parts[drawingPart], err = appendOfficeChild(parts[drawingPart], anchorXML, "wsDr")
	return err
}

func workbookImageAnchor(id string, nvID, row, col, x, y, width, height int, alt, relID string) []byte {
	cx, cy := strconv.Itoa(width*9525), strconv.Itoa(height*9525)
	return []byte(`<xdr:oneCellAnchor xmlns:xdr="http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><xdr:from><xdr:col>` + strconv.Itoa(col) + `</xdr:col><xdr:colOff>` + strconv.Itoa(x*9525) + `</xdr:colOff><xdr:row>` + strconv.Itoa(row) + `</xdr:row><xdr:rowOff>` + strconv.Itoa(y*9525) + `</xdr:rowOff></xdr:from><xdr:ext cx="` + cx + `" cy="` + cy + `"/><xdr:pic><xdr:nvPicPr><xdr:cNvPr id="` + strconv.Itoa(nvID) + `" name="AuraGo image ` + xmlText(id) + `" descr="` + xmlText(alt) + `"/><xdr:cNvPicPr><a:picLocks noChangeAspect="1"/></xdr:cNvPicPr></xdr:nvPicPr><xdr:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="` + relID + `"/><a:stretch><a:fillRect/></a:stretch></xdr:blipFill><xdr:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + cx + `" cy="` + cy + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></xdr:spPr></xdr:pic><xdr:clientData/></xdr:oneCellAnchor>`)
}
