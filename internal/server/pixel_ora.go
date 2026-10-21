package server

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
)

// OpenRaster (.ora) project files are ZIP archives: a stored "mimetype" entry first, a "stack.xml" layer stack, the
// layer bitmaps as PNG files under data/, plus mergedimage.png and Thumbnails/thumbnail.png. The Pixel editor writes
// them on the client; this file decides whether an uploaded archive may be stored. Nothing is extracted to disk, the
// archive is only read in memory and rejected when anything about it is unexpected.
const (
	pixelORAMimetype       = "image/openraster"
	pixelORANamespace      = "urn:aurago:openraster"
	pixelORAMaxEntries     = 128
	pixelORAMaxXMLBytes    = 1 << 20
	pixelORAMaxTotalBytes  = 512 << 20
	pixelORAMaxTotalPixels = 1 << 30
	pixelORAMaxSide        = 16384
	pixelORAMaxNameLength  = 200
	pixelORAMaxXMLDepth    = 32
	pixelORAMaxXMLElements = 20000
	pixelORAMaxLayers      = 256
	pixelORAMaxDirBytes    = 1 << 20

	pixelZIPEndSignature   = 0x06054b50
	pixelZIPEndRecordBytes = 22
)

var (
	errPixelORAInvalid  = errors.New("the project file is not a valid OpenRaster archive")
	errPixelORATooLarge = errors.New("the project file exceeds the size limit")
)

// pixelValidateSavePayload validates the bytes of a /api/pixel/save request and returns their format name: "png",
// "jpeg" or "ora". A .ora path or an explicit "ora" format selects the OpenRaster rules, everything else the plain
// image rules of pixelValidateImage.
func pixelValidateSavePayload(data []byte, limit int64, savePath, requestedFormat string) (string, error) {
	if strings.EqualFold(path.Ext(savePath), ".ora") || strings.EqualFold(strings.TrimSpace(requestedFormat), "ora") {
		if err := pixelValidateORA(data, limit); err != nil {
			return "", err
		}
		return "ora", nil
	}
	return pixelValidateImage(data, limit)
}

// pixelValidateORA checks an OpenRaster archive: size, ZIP structure, entry names and kinds, declared and actual
// sizes (zip bomb), stack.xml, and every PNG it contains.
func pixelValidateORA(data []byte, limit int64) error {
	if int64(len(data)) > limit {
		return errPixelORATooLarge
	}
	if err := pixelORACheckDirectory(data); err != nil {
		return err
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return errPixelORAInvalid
	}
	return pixelORAValidateArchive(reader, limit)
}

// pixelORACheckDirectory inspects the end-of-central-directory record before archive/zip parses the directory, so a
// crafted entry count or directory size cannot make the reader allocate a huge file table. The writer never adds an
// archive comment, ZIP64 records or leading data, so the directory must end exactly where the end record starts.
func pixelORACheckDirectory(data []byte) error {
	if len(data) < pixelZIPEndRecordBytes {
		return errPixelORAInvalid
	}
	end := data[len(data)-pixelZIPEndRecordBytes:]
	if binary.LittleEndian.Uint32(end[0:4]) != pixelZIPEndSignature || binary.LittleEndian.Uint16(end[20:22]) != 0 {
		return errPixelORAInvalid
	}
	diskNumber, directoryDisk := binary.LittleEndian.Uint16(end[4:6]), binary.LittleEndian.Uint16(end[6:8])
	entriesOnDisk, entries := binary.LittleEndian.Uint16(end[8:10]), binary.LittleEndian.Uint16(end[10:12])
	directorySize, directoryOffset := binary.LittleEndian.Uint32(end[12:16]), binary.LittleEndian.Uint32(end[16:20])
	if diskNumber != 0 || directoryDisk != 0 || entriesOnDisk != entries || entries == 0 {
		return errPixelORAInvalid
	}
	if entries > pixelORAMaxEntries {
		return fmt.Errorf("the project file has too many entries (limit %d)", pixelORAMaxEntries)
	}
	if directorySize > pixelORAMaxDirBytes || uint64(directoryOffset)+uint64(directorySize) != uint64(len(data)-pixelZIPEndRecordBytes) {
		return errPixelORAInvalid
	}
	return nil
}

// pixelORACleanEntryName accepts only plain relative names: no empty, "." or ".." segments, no absolute paths, no
// backslashes, colons (drive letters, streams) or control characters.
func pixelORACleanEntryName(name string) error {
	trimmed := strings.TrimSuffix(name, "/")
	if trimmed == "" || len(name) > pixelORAMaxNameLength || !utf8.ValidString(name) {
		return fmt.Errorf("the project file has an invalid entry name")
	}
	if strings.HasPrefix(name, "/") || strings.ContainsAny(name, "\\:") {
		return fmt.Errorf("the project file has an unsafe entry name")
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return fmt.Errorf("the project file has an invalid entry name")
		}
	}
	for _, part := range strings.Split(trimmed, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("the project file has an unsafe entry name")
		}
	}
	return nil
}

// pixelORAEntryKind classifies the entries the editor writes; everything else is refused so an .ora cannot be used
// to smuggle arbitrary payloads into the workspace.
func pixelORAEntryKind(name string) string {
	switch name {
	case "mimetype":
		return "mimetype"
	case "stack.xml":
		return "stack"
	case "mergedimage.png", "Thumbnails/thumbnail.png":
		return "png"
	case "data/", "Thumbnails/":
		return "dir"
	}
	if rest, ok := strings.CutPrefix(name, "data/"); ok && len(rest) > len(".png") && strings.HasSuffix(rest, ".png") && !strings.Contains(rest, "/") {
		return "png"
	}
	return ""
}

type pixelORAStack struct {
	width, height int
	refs          []string
}

func pixelORAValidateArchive(reader *zip.Reader, limit int64) error {
	files := reader.File
	if len(files) == 0 {
		return errPixelORAInvalid
	}
	if len(files) > pixelORAMaxEntries {
		return fmt.Errorf("the project file has too many entries (limit %d)", pixelORAMaxEntries)
	}

	// The mimetype entry must be the very first thing in the file: stored, with no extra field.
	first := files[0]
	if first.Name != "mimetype" || first.Method != zip.Store || first.Flags&1 != 0 {
		return fmt.Errorf("the project file must start with a stored mimetype entry")
	}
	if offset, err := first.DataOffset(); err != nil || offset != int64(30+len("mimetype")) {
		return fmt.Errorf("the project file must start with a stored mimetype entry")
	}

	// Pass one looks at the directory only: names, methods and the sizes the archive claims.
	seen := make(map[string]bool, len(files))
	kinds := make([]string, len(files))
	var declaredTotal uint64
	for i, f := range files {
		if err := pixelORACleanEntryName(f.Name); err != nil {
			return err
		}
		lower := strings.ToLower(f.Name)
		if seen[lower] {
			return fmt.Errorf("the project file has duplicate entries")
		}
		seen[lower] = true
		kind := pixelORAEntryKind(f.Name)
		if kind == "" {
			return fmt.Errorf("the project file contains an unexpected entry")
		}
		kinds[i] = kind
		if f.Flags&1 != 0 {
			return fmt.Errorf("encrypted project files are not supported")
		}
		if f.Method != zip.Store && f.Method != zip.Deflate {
			return fmt.Errorf("the project file uses an unsupported compression method")
		}
		if kind == "dir" {
			if f.UncompressedSize64 != 0 {
				return errPixelORAInvalid
			}
			continue
		}
		var entryLimit uint64
		switch kind {
		case "mimetype":
			entryLimit = uint64(len(pixelORAMimetype))
		case "stack":
			entryLimit = pixelORAMaxXMLBytes
		default:
			entryLimit = uint64(limit)
		}
		if f.UncompressedSize64 > entryLimit {
			if kind == "stack" {
				return fmt.Errorf("stack.xml is too large")
			}
			return errPixelORATooLarge
		}
		declaredTotal += f.UncompressedSize64
		if declaredTotal > pixelORAMaxTotalBytes {
			return errPixelORATooLarge
		}
	}

	// Pass two reads the entries. The declared size is only a claim: never read more than it, and require the
	// entry to end exactly there, so a tiny header cannot hide a huge payload.
	var actualTotal uint64
	var totalPixels int64
	var stack *pixelORAStack
	var mimetypeOK bool
	pngs := make(map[string][2]int)
	for i, f := range files {
		kind := kinds[i]
		if kind == "dir" {
			continue
		}
		content, err := pixelORAReadEntry(f)
		if err != nil {
			return err
		}
		actualTotal += uint64(len(content))
		if actualTotal > pixelORAMaxTotalBytes {
			return errPixelORATooLarge
		}
		switch kind {
		case "mimetype":
			if string(content) != pixelORAMimetype {
				return fmt.Errorf("the mimetype entry is not %s", pixelORAMimetype)
			}
			mimetypeOK = true
		case "stack":
			parsed, err := pixelORAParseStack(content)
			if err != nil {
				return err
			}
			stack = parsed
		case "png":
			cfg, format, err := image.DecodeConfig(bytes.NewReader(content))
			if err != nil || format != "png" || cfg.Width <= 0 || cfg.Height <= 0 {
				return fmt.Errorf("%s is not a valid PNG image", f.Name)
			}
			totalPixels += int64(cfg.Width) * int64(cfg.Height)
			if totalPixels > pixelORAMaxTotalPixels {
				return fmt.Errorf("the project images are too large")
			}
			if _, err := pixelValidateImage(content, limit); err != nil {
				return fmt.Errorf("%s: %w", f.Name, err)
			}
			pngs[f.Name] = [2]int{cfg.Width, cfg.Height}
		}
	}
	if !mimetypeOK || stack == nil {
		return fmt.Errorf("the project file has no stack.xml")
	}
	for _, ref := range stack.refs {
		if _, ok := pngs[ref]; !ok {
			return fmt.Errorf("stack.xml references a missing image")
		}
	}
	for name, size := range pngs {
		if size[0] > stack.width || size[1] > stack.height {
			return fmt.Errorf("%s is larger than the image", name)
		}
	}
	return nil
}

func pixelORAReadEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, errPixelORAInvalid
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, int64(f.UncompressedSize64)+1))
	if err != nil {
		return nil, fmt.Errorf("the project file entry %q is corrupt", f.Name)
	}
	if uint64(len(data)) != f.UncompressedSize64 {
		return nil, fmt.Errorf("the project file entry %q does not match its declared size", f.Name)
	}
	return data, nil
}

// pixelORAParseStack parses stack.xml and returns the image size and every file the layer stack refers to (layer
// src plus the AuraGo mask extension). The XML must be well-formed UTF-8 without a DOCTYPE, with bounded depth.
func pixelORAParseStack(content []byte) (*pixelORAStack, error) {
	bad := func(reason string) (*pixelORAStack, error) {
		return nil, fmt.Errorf("stack.xml is invalid: %s", reason)
	}
	decoder := xml.NewDecoder(bytes.NewReader(content))
	decoder.Strict = true
	result := &pixelORAStack{}
	depth, elements, layers := 0, 0, 0
	rootSeen := false
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return bad("not well-formed XML")
		}
		switch t := token.(type) {
		case xml.StartElement:
			elements++
			depth++
			if elements > pixelORAMaxXMLElements || depth > pixelORAMaxXMLDepth {
				return bad("too complex")
			}
			if depth == 1 {
				if rootSeen || t.Name.Local != "image" {
					return bad("the root element must be <image>")
				}
				rootSeen = true
				width, height, err := pixelORAImageSize(t.Attr)
				if err != nil {
					return bad(err.Error())
				}
				result.width, result.height = width, height
			}
			if t.Name.Local == "layer" {
				layers++
				if layers > pixelORAMaxLayers {
					return bad("too many layers")
				}
				if err := pixelORACollectLayer(t.Attr, result); err != nil {
					return bad(err.Error())
				}
			}
		case xml.EndElement:
			depth--
		case xml.Directive:
			return bad("DOCTYPE declarations are not allowed")
		case xml.CharData:
			if depth == 0 && len(bytes.TrimSpace(t)) > 0 {
				return bad("text outside the root element")
			}
		}
	}
	if !rootSeen || depth != 0 {
		return bad("no <image> element")
	}
	return result, nil
}

func pixelORAImageSize(attrs []xml.Attr) (int, int, error) {
	var width, height int
	for _, attr := range attrs {
		switch attr.Name.Local {
		case "w":
			width, _ = strconv.Atoi(strings.TrimSpace(attr.Value))
		case "h":
			height, _ = strconv.Atoi(strings.TrimSpace(attr.Value))
		}
	}
	if width <= 0 || height <= 0 || width > pixelORAMaxSide || height > pixelORAMaxSide || int64(width)*int64(height) > pixelMaxPixels {
		return 0, 0, errors.New("the image size is missing or out of range")
	}
	return width, height, nil
}

func pixelORACollectLayer(attrs []xml.Attr, result *pixelORAStack) error {
	srcSeen := false
	for _, attr := range attrs {
		value := strings.TrimSpace(attr.Value)
		switch {
		case attr.Name.Local == "src" && attr.Name.Space == "":
			srcSeen = true
			if pixelORAEntryKind(value) != "png" {
				return errors.New("a layer refers to an unsupported file")
			}
			result.refs = append(result.refs, value)
		case attr.Name.Local == "mask" && (attr.Name.Space == "aurago" || attr.Name.Space == pixelORANamespace):
			if pixelORAEntryKind(value) != "png" {
				return errors.New("a layer mask refers to an unsupported file")
			}
			result.refs = append(result.refs, value)
		case (attr.Name.Local == "x" || attr.Name.Local == "y") && attr.Name.Space == "":
			offset, err := strconv.Atoi(value)
			if err != nil || offset < -(1<<20) || offset > 1<<20 {
				return errors.New("a layer offset is out of range")
			}
		case attr.Name.Local == "opacity" && attr.Name.Space == "":
			opacity, err := strconv.ParseFloat(value, 64)
			if err != nil || math.IsNaN(opacity) || opacity < 0 || opacity > 1 {
				return errors.New("a layer opacity is out of range")
			}
		case attr.Name.Local == "composite-op" && attr.Name.Space == "":
			if len(value) > 64 {
				return errors.New("a layer blend mode is too long")
			}
		}
	}
	if !srcSeen {
		return errors.New("a layer has no src")
	}
	return nil
}
