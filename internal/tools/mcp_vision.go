package tools

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// Schema references must never fetch a URL or read a host file during argument selection.
type mcpOfflineSchemaLoader struct{ gojsonschema.JSONLoader }

func (mcpOfflineSchemaLoader) LoaderFactory() gojsonschema.JSONLoaderFactory {
	return mcpOfflineSchemaFactory{}
}

type mcpOfflineSchemaFactory struct{}

func (mcpOfflineSchemaFactory) New(string) gojsonschema.JSONLoader {
	return mcpBlockedSchemaLoader{gojsonschema.NewGoLoader(nil)}
}

type mcpBlockedSchemaLoader struct{ gojsonschema.JSONLoader }

func (mcpBlockedSchemaLoader) LoadJSON() (interface{}, error) {
	return nil, fmt.Errorf("external MCP schema references are unsupported")
}

func mcpImagePropertyPointer(schema map[string]interface{}, name string) string {
	if len(schema) == 0 {
		return ""
	}
	if props, _ := schema["properties"].(map[string]interface{}); len(props) > 0 {
		if _, ok := props[name]; ok {
			return "/properties/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(name)
		}
		return ""
	}
	for _, key := range []string{"inputSchema", "input_schema", "schema", "parameters"} {
		nested, _ := schema[key].(map[string]interface{})
		if pointer := mcpImagePropertyPointer(nested, name); pointer != "" {
			return "/" + key + pointer
		}
	}
	return ""
}

func mcpImageInputAccepts(schema map[string]interface{}, name, value string) bool {
	pointer := mcpImagePropertyPointer(schema, name)
	if pointer == "" {
		return false
	}
	loader := gojsonschema.NewSchemaLoader()
	loader.AutoDetect = false
	loader.Draft = gojsonschema.Draft7
	// Cache the original root so local references keep their document context.
	const document = "urn:aurago:mcp:vision-schema"
	if err := loader.AddSchema(document, mcpOfflineSchemaLoader{gojsonschema.NewGoLoader(schema)}); err != nil {
		return false
	}
	compiled, err := loader.Compile(mcpOfflineSchemaLoader{gojsonschema.NewReferenceLoader(document + "#" + pointer)})
	if err != nil {
		return false
	}
	result, err := compiled.Validate(gojsonschema.NewGoLoader(value))
	return err == nil && result.Valid()
}

func buildPreferredMCPVisionArgs(ctx context.Context, tool *MCPToolInfo, resolvedPath, prompt string) (map[string]interface{}, func(), error) {
	noop := func() {}
	if err := ctx.Err(); err != nil {
		return nil, noop, err
	}
	if tool.connection == nil {
		return nil, noop, fmt.Errorf("MCP vision connection runtime is unavailable")
	}
	input, err := rootedToolOpen(resolvedPath)
	if err != nil {
		return nil, noop, fmt.Errorf("open MCP vision image: %w", err)
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return nil, noop, fmt.Errorf("stat MCP vision image: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, noop, fmt.Errorf("MCP vision input must be a regular image file")
	}
	if info.Size() > maxGeneratedImageBytes {
		return nil, noop, fmt.Errorf("MCP vision image exceeds 50 MiB")
	}
	raw, err := io.ReadAll(io.LimitReader(input, maxGeneratedImageBytes+1))
	if err != nil {
		return nil, noop, fmt.Errorf("read MCP vision image: %w", err)
	}
	if int64(len(raw)) > maxGeneratedImageBytes {
		return nil, noop, fmt.Errorf("MCP vision image exceeds 50 MiB")
	}
	if err := ctx.Err(); err != nil {
		return nil, noop, err
	}
	mime := http.DetectContentType(raw)
	switch mime {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp":
	default:
		return nil, noop, fmt.Errorf("unsupported MCP vision image type %q", mime)
	}
	props := mcpInputProperties(tool)
	args := make(map[string]interface{})
	setFirstMatchingMCPArg(args, props, prompt, "prompt", "question", "query", "instruction", "text")
	setFirstMatchingMCPArg(args, props, filepath.Base(resolvedPath), "filename", "file_name", "fileName", "name")
	encoded := ""
	for _, group := range [][]string{
		{"image_base64", "imageBase64", "base64_image", "base64Image", "base64", "image_data", "imageData", "data"},
		{"image_url", "imageUrl", "url"},
	} {
		for _, name := range group {
			if _, ok := props[name]; ok {
				if encoded == "" {
					encoded = base64.StdEncoding.EncodeToString(raw)
				}
				value := encoded
				if name == "image_url" || name == "imageUrl" || name == "url" {
					value = "data:" + mime + ";base64," + encoded
				}
				if mcpImageInputAccepts(tool.InputSchema, name, value) {
					args[name] = value
					return args, noop, nil
				}
			}
		}
	}
	conn := tool.connection
	if conn.runtime != "local" && conn.runtime != "docker" {
		return nil, noop, fmt.Errorf("remote MCP vision tool %q requires a supported base64 or data-URL input; local paths cannot be sent", tool.Name)
	}
	for _, name := range []string{"file_path", "filePath", "path", "image_path", "imagePath", "image_file", "imageFile", "image_source", "imageSource", "local_path", "localPath", "input_path", "inputPath", "image"} {
		if _, ok := props[name]; ok {
			staged, cleanup, err := stageMCPVisionImage(conn, filepath.Base(resolvedPath), raw)
			if err != nil {
				return nil, noop, err
			}
			if !mcpImageInputAccepts(tool.InputSchema, name, staged) {
				cleanup()
				continue
			}
			args[name] = staged
			return args, cleanup, nil
		}
	}
	return nil, noop, fmt.Errorf("MCP vision tool %q has no supported image input (schema keys: %s)", tool.Name, summarizeMCPInputSchema(tool.InputSchema))
}

func stageMCPVisionImage(conn *mcpConn, filename string, raw []byte) (string, func(), error) {
	base := os.TempDir()
	if conn.runtime == "docker" {
		if conn.hostDir == "" || conn.contDir == "" {
			return "", nil, fmt.Errorf("MCP Docker vision workspace is unavailable")
		}
		base = conn.hostDir
	}
	root, err := os.OpenRoot(base)
	if err != nil {
		return "", nil, fmt.Errorf("open MCP vision staging root: %w", err)
	}
	dir := "aurago-mcp-vision-" + rand.Text()
	if err := root.Mkdir(dir, 0o700); err != nil {
		root.Close()
		return "", nil, fmt.Errorf("create MCP vision staging directory: %w", err)
	}
	cleanup := func() { _ = root.RemoveAll(dir); _ = root.Close() }
	rel := filepath.Join(dir, filename)
	f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err == nil {
		_, err = f.Write(raw)
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("stage MCP vision image: %w", err)
	}
	if conn.runtime == "docker" {
		return path.Join(conn.contDir, dir, filename), cleanup, nil
	}
	return filepath.Join(base, rel), cleanup, nil
}
