package office

import (
	"fmt"
	"strconv"
)

// DocumentImageOptions uses 96-DPI pixels for the physical insertion dimensions.
type DocumentImageOptions struct {
	Anchor string
	Width  int
	Height int
	Alt    string
}

// InsertDOCXImage appends an embedded image before the final section properties.
// Callers must publish the returned package through their conditional-write path.
func InsertDOCXImage(original, image []byte, options DocumentImageOptions) ([]byte, error) {
	if options.Anchor != "end" || len(options.Alt) > 2048 {
		return nil, fmt.Errorf("DOCX image anchor must be end and alt text must not exceed 2048 bytes")
	}
	ext, mime, width, height, err := officeImageInfo(image, options.Width, options.Height)
	if err != nil {
		return nil, err
	}
	if err = ValidateDOCX(original); err != nil {
		return nil, err
	}
	parts, err := ReadDOCXParts(original)
	if err != nil {
		return nil, err
	}
	document := parts["word/document.xml"]
	children, _, _, err := officeXMLChildren(document)
	if err != nil {
		return nil, err
	}
	bodyStart, bodyEnd := -1, -1
	for _, child := range children {
		if child.Name == "body" {
			if bodyStart >= 0 {
				return nil, fmt.Errorf("DOCX has multiple document bodies")
			}
			bodyStart, bodyEnd = child.Start, child.End
		}
	}
	if bodyStart < 0 {
		return nil, fmt.Errorf("DOCX document body is missing")
	}
	body := expandEmptyOfficeRoot(document[bodyStart:bodyEnd])
	blocks, _, insert, err := officeXMLChildren(body)
	if err != nil {
		return nil, err
	}
	for index, block := range blocks {
		if block.Name == "sectPr" {
			if index != len(blocks)-1 {
				return nil, fmt.Errorf("DOCX has unsupported section placement")
			}
			insert = block.Start
		}
	}
	if insert < 1 {
		return nil, fmt.Errorf("DOCX document body is invalid")
	}
	nextID, err := nextOfficeDrawingID(document)
	if err != nil {
		return nil, err
	}
	_, relID, err := addOfficeImagePart(parts, "word/document.xml", "word/media", ext, mime, image)
	if err != nil {
		return nil, err
	}
	id, cx, cy := strconv.Itoa(nextID), strconv.Itoa(width*9525), strconv.Itoa(height*9525)
	paragraph := []byte(`<w:p xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:r><w:drawing><wp:inline xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" distT="0" distB="0" distL="0" distR="0"><wp:extent cx="` + cx + `" cy="` + cy + `"/><wp:docPr id="` + id + `" name="AuraGo image ` + id + `" descr="` + xmlText(options.Alt) + `"/><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="` + id + `" name="AuraGo image ` + id + `"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:embed="` + relID + `"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="` + cx + `" cy="` + cy + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`)
	body = append(append(append([]byte{}, body[:insert]...), paragraph...), body[insert:]...)
	parts["word/document.xml"] = append(append(append([]byte{}, document[:bodyStart]...), body...), document[bodyEnd:]...)
	return writeOfficeImageParts(parts)
}
