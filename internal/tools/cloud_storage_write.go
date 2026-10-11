package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"path"
	"strings"
)

// Put creates a file unless an observed version explicitly authorizes replacement.
func (c CloudStorage) Put(ctx context.Context, loc StorageLocation, body io.Reader, version string) (StorageEntry, error) {
	if loc.Provider == "gog" {
		return c.Gog.Put(ctx, loc, body, version)
	}
	if loc.Path == "" {
		return StorageEntry{}, fmt.Errorf("cannot write storage root")
	}
	headers := map[string]string{"Content-Type": "application/octet-stream"}
	method := "PUT"
	route := ""
	if loc.Provider == "webdav" {
		if version == "" {
			headers["If-None-Match"] = "*"
		} else {
			headers["If-Match"] = version
		}
	} else {
		parent := loc
		parent.Path = path.Dir(loc.Path)
		if parent.Path == "." {
			parent.Path = ""
		}
		route = koofrStorageRoute(parent, "put", true) + "&filename=" + url.QueryEscape(path.Base(loc.Path)) + "&info=true&autorename=false"
		if version != "" {
			route += "&overwriteIfHash=" + url.QueryEscape(version)
		}
		var prefix bytes.Buffer
		writer := multipart.NewWriter(&prefix)
		if _, err := writer.CreateFormFile("file", path.Base(loc.Path)); err != nil {
			return StorageEntry{}, err
		}
		headers["Content-Type"] = writer.FormDataContentType()
		start := append([]byte(nil), prefix.Bytes()...)
		prefix.Reset()
		if err := writer.Close(); err != nil {
			return StorageEntry{}, err
		}
		body = io.MultiReader(bytes.NewReader(start), body, bytes.NewReader(prefix.Bytes()))
		method = "POST"
	}
	_, err := readStorageResponse(c.request(ctx, loc, method, route, body, headers))
	if err != nil {
		return StorageEntry{}, err
	}
	return c.Stat(ctx, loc)
}

func (c CloudStorage) Mkdir(ctx context.Context, loc StorageLocation) error {
	if loc.Provider == "gog" {
		return c.Gog.Mkdir(ctx, loc)
	}
	if loc.Path == "" {
		return fmt.Errorf("cannot create storage root")
	}
	if loc.Provider == "webdav" {
		_, err := readStorageResponse(c.request(ctx, loc, "MKCOL", "", nil, nil))
		return err
	}
	parent := loc
	parent.Path = path.Dir(loc.Path)
	if parent.Path == "." {
		parent.Path = ""
	}
	body, _ := json.Marshal(map[string]string{"name": path.Base(loc.Path)})
	_, err := readStorageResponse(c.request(ctx, parent, "POST", koofrStorageRoute(parent, "folder", false), bytes.NewReader(body), map[string]string{"Content-Type": "application/json"}))
	return err
}

// Remove can conditionally delete a copied source; unsafe cleanup retains it.
func (c CloudStorage) Remove(ctx context.Context, loc StorageLocation, version string, onlyEmpty bool) error {
	if loc.Provider == "gog" {
		return ErrStorageSourceRetained
	}
	if loc.Path == "" {
		return fmt.Errorf("cannot delete storage root")
	}
	headers := map[string]string{}
	route := koofrStorageRoute(loc, "remove", false)
	if loc.Provider == "webdav" {
		// WebDAV has no atomic remove-if-empty; a concurrent new child must survive.
		if onlyEmpty {
			return ErrStorageSourceRetained
		}
		if version != "" {
			headers["If-Match"] = version
		}
	} else {
		if version != "" {
			route += "&removeIfHash=" + url.QueryEscape(version)
		}
		if onlyEmpty {
			route += "&removeIfEmpty="
		}
	}
	_, err := readStorageResponse(c.request(ctx, loc, "DELETE", route, nil, headers))
	return err
}

func (c CloudStorage) Move(ctx context.Context, from, to StorageLocation, version string) error {
	if from.Provider == "gog" && to.Provider == "gog" && from.AccountID == to.AccountID {
		return c.Gog.Move(ctx, from, to, version)
	}
	if from.Provider != to.Provider || from.MountID != to.MountID || from.Path == "" || to.Path == "" {
		return fmt.Errorf("invalid storage move")
	}
	if err := ValidateStorageLocation(to); err != nil {
		return err
	}
	if from.Path == to.Path || strings.HasPrefix(to.Path, from.Path+"/") {
		return fmt.Errorf("destination is inside source")
	}
	if _, err := c.Stat(ctx, to); err != ErrStorageNotFound {
		if err == nil {
			return ErrStorageConflict
		}
		return err
	}
	if from.Provider == "webdav" {
		dest, err := webdavURL(c.WebDAV, to.Path)
		if err != nil {
			return err
		}
		headers := map[string]string{"Destination": dest, "Overwrite": "F"}
		if version != "" {
			headers["If-Match"] = version
		}
		_, err = readStorageResponse(c.request(ctx, from, "MOVE", "", nil, headers))
		return err
	}
	body, _ := json.Marshal(map[string]string{"toMountId": to.MountID, "toPath": "/" + to.Path})
	_, err := readStorageResponse(c.request(ctx, from, "PUT", koofrStorageRoute(from, "move", false), bytes.NewReader(body), map[string]string{"Content-Type": "application/json"}))
	return err
}
