package bookshelf

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"path"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"crypto/sha256"
)

const (
	maxMetadataXMLBytes = 1 << 20
	maxFB2MetadataBytes = 64 << 20
)

// ExtractMetadata reads only bounded XML and image entries from EPUB, FB2,
// FB2.ZIP and CBZ files. It never contacts a metadata service.
func ExtractMetadata(ctx context.Context, reader io.ReaderAt, size int64, format string) (ExtractedMetadata, error) {
	if err := ctx.Err(); err != nil {
		return ExtractedMetadata{}, err
	}
	if reader == nil || size < 1 || size > 4<<30 {
		return ExtractedMetadata{}, ErrLimitExceeded
	}
	format = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(format), "."))
	result := ExtractedMetadata{Metadata: Metadata{Format: format}}
	switch format {
	case "epub":
		return extractEPUBMetadata(ctx, reader, size, result)
	case "fb2":
		parsed, err := extractFB2Metadata(ctx, io.NewSectionReader(reader, 0, size), size)
		parsed.Metadata.Format = format
		return parsed, err
	case "fb2.zip":
		archive, err := boundedZipReader(contextReaderAt{ctx: ctx, r: reader}, size)
		if err != nil {
			return result, err
		}
		for _, entry := range archive.File {
			if err := validateArchivePath(entry.Name); err != nil {
				return result, err
			}
			if strings.EqualFold(path.Ext(entry.Name), ".fb2") {
				if compressedTooLarge(entry) {
					return result, ErrLimitExceeded
				}
				body, err := entry.Open()
				if err != nil {
					return result, fmt.Errorf("open FB2 archive entry: %w", err)
				}
				parsed, parseErr := extractFB2Metadata(ctx, body, int64(entry.UncompressedSize64))
				if closeErr := body.Close(); parseErr == nil && closeErr != nil {
					parseErr = fmt.Errorf("close FB2 archive entry: %w", closeErr)
				}
				parsed.Metadata.Format = format
				return parsed, parseErr
			}
		}
		return result, ErrNotFound
	case "cbz":
		return extractCBZMetadata(ctx, reader, size, result)
	default:
		return result, ErrUnsupported
	}
}

// UpdateDiscoveredMetadata applies parsed source metadata without changing user
// overrides. Empty parsed fields preserve the last source-derived values.
func (s *Service) UpdateDiscoveredMetadata(ctx context.Context, bookID string, expectedRevision int64, metadata Metadata) (Book, error) {
	if err := s.checkOpen(); err != nil {
		return Book{}, err
	}
	if !validID(bookID) {
		return Book{}, ErrNotFound
	}
	if metadata.Format != "" && !validFormat(strings.ToLower(strings.TrimSpace(metadata.Format))) || len(metadata.Title) > 2048 || len(metadata.Author) > 512 || len(metadata.Series) > 512 || len(metadata.Description) > 32_768 || len(metadata.Tags) > 128 {
		return Book{}, fmt.Errorf("invalid discovered bookshelf metadata")
	}
	if !validateText(metadata.Title, 2048) || !validateText(metadata.Author, 512) || !validateText(metadata.Series, 512) || !validateText(metadata.Description, 32_768) {
		return Book{}, fmt.Errorf("invalid discovered bookshelf metadata")
	}
	for _, tag := range metadata.Tags {
		if tag == "" || !validateText(tag, 128) {
			return Book{}, fmt.Errorf("invalid discovered bookshelf metadata")
		}
	}
	metadata = normalizeMetadata(metadata)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Book{}, err
	}
	defer tx.Rollback()
	var title, author, series, description, rawTags, format string
	var revision int64
	if err := tx.QueryRowContext(ctx, `SELECT title,author,series,description,tags,format,revision FROM books WHERE id=?`, bookID).
		Scan(&title, &author, &series, &description, &rawTags, &format, &revision); errors.Is(err, sql.ErrNoRows) {
		return Book{}, ErrNotFound
	} else if err != nil {
		return Book{}, fmt.Errorf("read discovered bookshelf metadata: %w", err)
	}
	if revision != expectedRevision {
		return Book{}, ErrConflict
	}
	originalTitle, originalAuthor, originalSeries, originalDescription := title, author, series, description
	if metadata.Format != "" && metadata.Format != format {
		return Book{}, fmt.Errorf("discovered bookshelf format does not match catalog")
	}
	var tags []string
	_ = json.Unmarshal([]byte(rawTags), &tags)
	if metadata.Title != "" {
		title = metadata.Title
	}
	if metadata.Author != "" {
		author = metadata.Author
	}
	if metadata.Series != "" {
		series = metadata.Series
	}
	if metadata.Description != "" {
		description = metadata.Description
	}
	if len(metadata.Tags) > 0 {
		tags = metadata.Tags
	}
	encodedTags, err := json.Marshal(normalizeTags(tags))
	if err != nil {
		return Book{}, err
	}
	if title == "" || !validateText(title, 2048) || !validateText(author, 512) || !validateText(series, 512) || !validateText(description, 32_768) {
		return Book{}, fmt.Errorf("invalid discovered bookshelf metadata")
	}
	if title == originalTitle && author == originalAuthor && series == originalSeries && description == originalDescription && string(encodedTags) == rawTags {
		if err := tx.Commit(); err != nil {
			return Book{}, err
		}
		return s.GetBook(ctx, bookID)
	}
	res, err := tx.ExecContext(ctx, `UPDATE books SET title=?,author=?,series=?,description=?,tags=?,revision=revision+1 WHERE id=? AND revision=?`,
		title, author, series, description, string(encodedTags), bookID, expectedRevision)
	if err != nil {
		return Book{}, fmt.Errorf("update discovered bookshelf metadata: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Book{}, checkRevision(ctx, tx, bookID)
	}
	if err := tx.Commit(); err != nil {
		return Book{}, fmt.Errorf("commit discovered bookshelf metadata: %w", err)
	}
	return s.GetBook(ctx, bookID)
}

type metadataXMLNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr        `xml:",any,attr"`
	Text    string            `xml:",chardata"`
	Nodes   []metadataXMLNode `xml:",any"`
}

func extractEPUBMetadata(ctx context.Context, reader io.ReaderAt, size int64, result ExtractedMetadata) (ExtractedMetadata, error) {
	archive, err := boundedZipReader(contextReaderAt{ctx: ctx, r: reader}, size)
	if err != nil {
		return result, err
	}
	files := make(map[string]*zip.File, len(archive.File))
	for _, entry := range archive.File {
		if err := validateArchivePath(entry.Name); err != nil {
			return result, err
		}
		files[entry.Name] = entry
	}
	containerFile := files["META-INF/container.xml"]
	if containerFile == nil {
		return result, ErrNotFound
	}
	containerBody, err := openZipEntry(ctx, containerFile, maxMetadataXMLBytes)
	if err != nil {
		return result, err
	}
	container, err := parseXMLTree(containerBody, maxMetadataXMLBytes)
	if err != nil {
		return result, fmt.Errorf("parse EPUB container: %w", err)
	}
	var opfPath string
	for _, rootfile := range findNodes(container, "rootfile") {
		fullPath := attrLocal(rootfile, "full-path")
		mediaType := attrLocal(rootfile, "media-type")
		resolved, ok := resolveArchiveHref("", fullPath)
		if ok && mediaType == "application/oebps-package+xml" && files[resolved] != nil {
			opfPath = resolved
			break
		}
	}
	if opfPath == "" {
		return result, ErrNotFound
	}
	opfBody, err := openZipEntry(ctx, files[opfPath], maxMetadataXMLBytes)
	if err != nil {
		return result, err
	}
	packageDoc, err := parseXMLTree(opfBody, maxMetadataXMLBytes)
	if err != nil {
		return result, fmt.Errorf("parse EPUB package metadata: %w", err)
	}
	metadataNode := findNode(packageDoc, "metadata")
	manifest := findNode(packageDoc, "manifest")
	if metadataNode == nil {
		metadataNode = packageDoc
	}
	for _, node := range metadataNode.Nodes {
		switch strings.ToLower(node.XMLName.Local) {
		case "title":
			if result.Metadata.Title == "" {
				result.Metadata.Title = boundedText(&node, 2048)
			}
		case "creator":
			if result.Metadata.Author == "" {
				result.Metadata.Author = boundedText(&node, 512)
			}
		case "description":
			if result.Metadata.Description == "" {
				result.Metadata.Description = boundedText(&node, 32_768)
			}
		case "subject":
			if tag := boundedText(&node, 128); tag != "" && len(result.Metadata.Tags) < 128 {
				result.Metadata.Tags = append(result.Metadata.Tags, tag)
			}
		case "meta":
			property := strings.ToLower(attrLocal(&node, "property"))
			name := strings.ToLower(attrLocal(&node, "name"))
			if property == "belongs-to-collection" || name == "calibre:series" {
				if result.Metadata.Series == "" {
					result.Metadata.Series = boundedText(&node, 512)
					if result.Metadata.Series == "" {
						result.Metadata.Series = attrLocal(&node, "content")
					}
				}
			}
		}
	}
	result.Metadata.Tags = normalizeTags(result.Metadata.Tags)

	coverID := ""
	for _, node := range metadataNode.Nodes {
		if strings.EqualFold(node.XMLName.Local, "meta") && strings.EqualFold(attrLocal(&node, "name"), "cover") {
			coverID = attrLocal(&node, "content")
			break
		}
	}
	var coverHref string
	for _, item := range findNodes(manifest, "item") {
		id := attrLocal(item, "id")
		properties := strings.Fields(strings.ToLower(attrLocal(item, "properties")))
		isCover := id != "" && id == coverID
		for _, property := range properties {
			if property == "cover-image" {
				isCover = true
			}
		}
		mime := strings.ToLower(attrLocal(item, "media-type"))
		if isCover && (mime == "image/jpeg" || mime == "image/png") {
			coverHref = attrLocal(item, "href")
			break
		}
	}
	if coverHref != "" {
		coverPath, ok := resolveArchiveHref(path.Dir(opfPath), coverHref)
		if ok {
			if coverFile := files[coverPath]; coverFile != nil && coverFile.UncompressedSize64 <= maxCoverBytes {
				cover, readErr := openZipEntry(ctx, coverFile, maxCoverBytes)
				if readErr == nil {
					result.EmbeddedCover, result.CoverMIME = validatedCover(cover)
				}
			}
		}
	}
	return result, nil
}

func extractFB2Metadata(ctx context.Context, reader io.Reader, size int64) (ExtractedMetadata, error) {
	result := ExtractedMetadata{Metadata: Metadata{Format: "fb2"}}
	if size < 1 || size > 4<<30 {
		return result, ErrLimitExceeded
	}
	decoder := xml.NewDecoder(io.LimitReader(contextReader{ctx: ctx, r: reader}, maxFB2MetadataBytes))
	var description metadataXMLNode
	foundDescription := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF && foundDescription {
				break
			}
			return result, fmt.Errorf("parse FB2 metadata: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "description") {
			continue
		}
		if err := decodeElementBounded(decoder, start, &description, maxMetadataXMLBytes); err != nil {
			return result, fmt.Errorf("parse FB2 description: %w", err)
		}
		foundDescription = true
		break
	}
	if !foundDescription {
		return result, ErrNotFound
	}
	titleInfo := findNode(&description, "title-info")
	if titleInfo != nil {
		result.Metadata.Title = boundedText(findNode(titleInfo, "book-title"), 2048)
		if author := findNode(titleInfo, "author"); author != nil {
			result.Metadata.Author = fb2Author(author)
		}
		if annotation := findNode(titleInfo, "annotation"); annotation != nil {
			result.Metadata.Description = boundedText(annotation, 32_768)
		}
		for _, genre := range findNodes(titleInfo, "genre") {
			if len(result.Metadata.Tags) >= 128 {
				break
			}
			result.Metadata.Tags = append(result.Metadata.Tags, boundedText(genre, 128))
		}
		if sequence := findNode(titleInfo, "sequence"); sequence != nil {
			result.Metadata.Series = attrLocal(sequence, "name")
		}
	}
	result.Metadata.Tags = normalizeTags(result.Metadata.Tags)
	if size > maxFB2MetadataBytes {
		return result, nil // Metadata is up front; large bodies do not get scanned for covers.
	}
	coverID := ""
	if titleInfo != nil {
		if coverpage := findNode(titleInfo, "coverpage"); coverpage != nil {
			if imageNode := findNode(coverpage, "image"); imageNode != nil {
				coverID = strings.TrimPrefix(attrLocal(imageNode, "href"), "#")
			}
		}
	}
	if coverID == "" {
		return result, nil
	}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return result, nil
		}
		if err != nil {
			return result, fmt.Errorf("scan FB2 embedded cover: %w", err)
		}
		start, ok := token.(xml.StartElement)
		if !ok || !strings.EqualFold(start.Name.Local, "binary") || attrLocalElement(start, "id") != coverID {
			continue
		}
		var encoded strings.Builder
		for {
			token, err := decoder.Token()
			if err != nil {
				return result, fmt.Errorf("read FB2 embedded cover: %w", err)
			}
			switch value := token.(type) {
			case xml.CharData:
				if encoded.Len()+len(value) > 16<<20 {
					return result, ErrLimitExceeded
				}
				_, _ = encoded.Write(value)
			case xml.EndElement:
				if strings.EqualFold(value.Name.Local, "binary") {
					cover, err := io.ReadAll(io.LimitReader(base64.NewDecoder(base64.StdEncoding, strings.NewReader(stripXMLWhitespace(encoded.String()))), maxCoverBytes+1))
					if err == nil && len(cover) <= maxCoverBytes {
						result.EmbeddedCover, result.CoverMIME = validatedCover(cover)
					}
					return result, nil
				}
			}
		}
	}
}

func decodeElementBounded(decoder *xml.Decoder, start xml.StartElement, destination *metadataXMLNode, maxBytes int64) error {
	startOffset := decoder.InputOffset()
	root := metadataXMLNode{XMLName: start.Name, Attrs: append([]xml.Attr(nil), start.Attr...)}
	stack := []*metadataXMLNode{&root}
	builders := map[*metadataXMLNode]*strings.Builder{&root: {}}
	nodes := 1
	for len(stack) > 0 {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if decoder.InputOffset()-startOffset > maxBytes {
			return ErrLimitExceeded
		}
		switch value := token.(type) {
		case xml.StartElement:
			if len(stack) >= 128 || nodes >= 10_000 {
				return ErrLimitExceeded
			}
			parent := stack[len(stack)-1]
			parent.Nodes = append(parent.Nodes, metadataXMLNode{XMLName: value.Name, Attrs: append([]xml.Attr(nil), value.Attr...)})
			child := &parent.Nodes[len(parent.Nodes)-1]
			builders[child] = &strings.Builder{}
			stack = append(stack, child)
			nodes++
		case xml.CharData:
			current := stack[len(stack)-1]
			builder := builders[current]
			if int64(builder.Len()+len(value)) > maxBytes {
				return ErrLimitExceeded
			}
			_, _ = builder.Write(value)
		case xml.EndElement:
			current := stack[len(stack)-1]
			current.Text = builders[current].String()
			stack = stack[:len(stack)-1]
		}
	}
	*destination = root
	return nil
}

func parseXMLTree(data []byte, maxBytes int64) (*metadataXMLNode, error) {
	if int64(len(data)) > maxBytes {
		return nil, ErrLimitExceeded
	}
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var root metadataXMLNode
	found := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF && found {
				return &root, nil
			}
			return nil, err
		}
		if start, ok := token.(xml.StartElement); ok {
			if found {
				return nil, fmt.Errorf("multiple XML root elements")
			}
			if err := decodeElementBounded(decoder, start, &root, maxBytes); err != nil {
				return nil, err
			}
			found = true
			continue
		}
		if chars, ok := token.(xml.CharData); ok && strings.TrimSpace(string(chars)) != "" {
			return nil, fmt.Errorf("unexpected text outside XML root")
		}
	}
}

func stripXMLWhitespace(value string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, value)
}

func extractCBZMetadata(ctx context.Context, reader io.ReaderAt, size int64, result ExtractedMetadata) (ExtractedMetadata, error) {
	archive, err := boundedZipReader(reader, size)
	if err != nil {
		return result, err
	}
	var comicInfo *zip.File
	var images []*zip.File
	for _, entry := range archive.File {
		if err := validateArchivePath(entry.Name); err != nil {
			return result, err
		}
		if strings.EqualFold(path.Base(entry.Name), "ComicInfo.xml") {
			comicInfo = entry
		}
		switch strings.ToLower(path.Ext(entry.Name)) {
		case ".jpg", ".jpeg", ".png":
			images = append(images, entry)
		}
	}
	if comicInfo != nil && comicInfo.UncompressedSize64 <= maxMetadataXMLBytes {
		body, err := openZipEntry(ctx, comicInfo, maxMetadataXMLBytes)
		if err != nil {
			return result, err
		}
		info, err := parseXMLTree(body, maxMetadataXMLBytes)
		if err != nil {
			return result, fmt.Errorf("parse CBZ ComicInfo metadata: %w", err)
		}
		result.Metadata.Title = nodeChildText(info, "Title", 2048)
		result.Metadata.Series = nodeChildText(info, "Series", 512)
		result.Metadata.Author = nodeChildText(info, "Writer", 512)
		result.Metadata.Description = nodeChildText(info, "Summary", 32_768)
		if genre := nodeChildText(info, "Genre", 128); genre != "" {
			tags := strings.FieldsFunc(genre, func(r rune) bool { return r == ',' || r == ';' })
			if len(tags) > 128 {
				tags = tags[:128]
			}
			result.Metadata.Tags = normalizeTags(tags)
		}
	}
	if len(images) == 0 {
		return result, nil
	}
	sort.Slice(images, func(i, j int) bool {
		leftCover := strings.Contains(strings.ToLower(path.Base(images[i].Name)), "cover")
		rightCover := strings.Contains(strings.ToLower(path.Base(images[j].Name)), "cover")
		if leftCover != rightCover {
			return leftCover
		}
		return naturalLess(images[i].Name, images[j].Name)
	})
	attempts := 0
	for _, entry := range images {
		if entry.UncompressedSize64 > maxCoverBytes || compressedTooLarge(entry) {
			continue
		}
		attempts++
		if attempts > 16 {
			break
		}
		cover, err := openZipEntry(ctx, entry, maxCoverBytes)
		if err != nil {
			continue
		}
		if result.EmbeddedCover, result.CoverMIME = validatedCover(cover); len(result.EmbeddedCover) > 0 {
			break
		}
	}
	return result, nil
}

func openZipEntry(ctx context.Context, entry *zip.File, max int64) ([]byte, error) {
	if entry == nil || entry.UncompressedSize64 > uint64(max) || compressedTooLarge(entry) {
		return nil, ErrLimitExceeded
	}
	reader, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open bookshelf archive entry: %w", err)
	}
	body, readErr := io.ReadAll(io.LimitReader(contextReader{ctx: ctx, r: reader}, max+1))
	closeErr := reader.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read bookshelf archive entry: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close bookshelf archive entry: %w", closeErr)
	}
	if int64(len(body)) > max || uint64(len(body)) != entry.UncompressedSize64 {
		return nil, ErrLimitExceeded
	}
	return body, nil
}

func compressedTooLarge(entry *zip.File) bool {
	if entry.CompressedSize64 == 0 {
		return entry.UncompressedSize64 > 0
	}
	return entry.UncompressedSize64 > entry.CompressedSize64*200
}

func validateArchivePath(name string) error {
	clean := path.Clean(name)
	if name == "" || strings.ContainsAny(name, "\\\x00") || strings.HasPrefix(name, "/") || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return ErrInvalidPath
	}
	return nil
}

func resolveArchiveHref(base, href string) (string, bool) {
	parsed, err := url.Parse(href)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.RawQuery != "" || parsed.Path == "" {
		return "", false
	}
	decoded, err := url.PathUnescape(parsed.Path)
	if err != nil || strings.ContainsAny(decoded, "\\\x00") || strings.HasPrefix(decoded, "/") {
		return "", false
	}
	clean := path.Clean(path.Join(base, decoded))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", false
	}
	return clean, true
}

func findNode(node *metadataXMLNode, name string) *metadataXMLNode {
	if node == nil {
		return nil
	}
	for i := range node.Nodes {
		if strings.EqualFold(node.Nodes[i].XMLName.Local, name) {
			return &node.Nodes[i]
		}
	}
	for i := range node.Nodes {
		if found := findNode(&node.Nodes[i], name); found != nil {
			return found
		}
	}
	return nil
}

func findNodes(node *metadataXMLNode, name string) []*metadataXMLNode {
	var result []*metadataXMLNode
	if node == nil {
		return result
	}
	for i := range node.Nodes {
		if strings.EqualFold(node.Nodes[i].XMLName.Local, name) {
			result = append(result, &node.Nodes[i])
		}
		result = append(result, findNodes(&node.Nodes[i], name)...)
	}
	return result
}

func attrLocal(node *metadataXMLNode, name string) string {
	if node == nil {
		return ""
	}
	for _, attr := range node.Attrs {
		if strings.EqualFold(attr.Name.Local, name) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func attrLocalElement(element xml.StartElement, name string) string {
	for _, attr := range element.Attr {
		if strings.EqualFold(attr.Name.Local, name) {
			return strings.TrimSpace(attr.Value)
		}
	}
	return ""
}

func boundedText(node *metadataXMLNode, max int) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	var walk func(*metadataXMLNode)
	walk = func(current *metadataXMLNode) {
		if builder.Len() >= max {
			return
		}
		if current.Text != "" {
			remaining := max - builder.Len()
			if len(current.Text) > remaining {
				builder.WriteString(truncateUTF8(current.Text, remaining))
			} else {
				builder.WriteString(current.Text)
			}
		}
		for i := range current.Nodes {
			walk(&current.Nodes[i])
			if builder.Len() >= max {
				return
			}
		}
	}
	walk(node)
	return strings.TrimSpace(builder.String())
}

func truncateUTF8(value string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(value) <= max {
		return value
	}
	end := max
	for end > 0 && !utf8.RuneStart(value[end]) {
		end--
	}
	return value[:end]
}

func nodeChildText(node *metadataXMLNode, child string, max int) string {
	for _, candidate := range node.Nodes {
		if strings.EqualFold(candidate.XMLName.Local, child) {
			return boundedText(&candidate, max)
		}
	}
	return ""
}

func fb2Author(author *metadataXMLNode) string {
	parts := []string{nodeChildText(author, "first-name", 128), nodeChildText(author, "middle-name", 128), nodeChildText(author, "last-name", 128)}
	filtered := parts[:0]
	for _, part := range parts {
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	value := strings.TrimSpace(strings.Join(filtered, " "))
	if value == "" {
		value = nodeChildText(author, "nickname", 256)
	}
	return truncateUTF8(value, 512)
}

func validatedCover(data []byte) ([]byte, string) {
	if len(data) == 0 || len(data) > maxCoverBytes {
		return nil, ""
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width < 1 || config.Height < 1 || config.Width > 8192 || config.Height > 8192 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return nil, ""
	}
	if _, decodedFormat, err := image.Decode(bytes.NewReader(data)); err != nil || decodedFormat != format {
		return nil, ""
	}
	switch format {
	case "jpeg":
		return append([]byte(nil), data...), "image/jpeg"
	case "png":
		return append([]byte(nil), data...), "image/png"
	default:
		return nil, ""
	}
}

func naturalLess(left, right string) bool {
	a, b := []rune(strings.ToLower(left)), []rune(strings.ToLower(right))
	for i, j := 0, 0; i < len(a) && j < len(b); {
		if unicode.IsDigit(a[i]) && unicode.IsDigit(b[j]) {
			startA, startB := i, j
			for i < len(a) && unicode.IsDigit(a[i]) {
				i++
			}
			for j < len(b) && unicode.IsDigit(b[j]) {
				j++
			}
			numberA, numberB := strings.TrimLeft(string(a[startA:i]), "0"), strings.TrimLeft(string(b[startB:j]), "0")
			if len(numberA) != len(numberB) {
				return len(numberA) < len(numberB)
			}
			if numberA != numberB {
				return numberA < numberB
			}
			continue
		}
		if a[i] != b[j] {
			return a[i] < b[j]
		}
		i++
		j++
	}
	return len(a) < len(b)
}

// FallbackCover derives stable text and palette choices from the catalog ID.
func FallbackCover(book Book) CoverFallback {
	return CoverFallback{
		Title: book.Title, Author: book.Author,
		Monogram:   initials(book.Title, book.Author),
		ColorIndex: fallbackIndex(book.ID, 8), PatternIndex: fallbackIndex("pattern:"+book.ID, 4),
	}
}

func fallbackIndex(value string, count int) int {
	if count < 1 {
		return 0
	}
	digest := sha256.Sum256([]byte(value))
	return int(digest[0]) % count
}

func initials(title, author string) string {
	first := firstLetter(title)
	second := firstLetter(author)
	if second == "" {
		for _, word := range strings.Fields(title) {
			if initial := firstLetter(word); initial != "" && initial != first {
				second = initial
				break
			}
		}
	}
	if first == "" {
		first = "?"
	}
	return strings.ToUpper(first + second)
}

func firstLetter(value string) string {
	for _, r := range strings.TrimSpace(value) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			return string(r)
		}
	}
	return ""
}
