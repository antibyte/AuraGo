package server

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"

	"aurago/internal/homedash"
)

func (h *hdRequest) listEntities() {
	q := h.r.URL.Query()
	f := homedash.Filter{Source: homedash.SourceID(q.Get("source")), Kind: homedash.Kind(q.Get("kind")), Area: q.Get("area"), Query: q.Get("q")}
	switch {
	case f.Source != "" && !homedash.ValidSourceID(string(f.Source)):
		h.bad("source must be a source id")
		return
	case f.Kind != "" && !slices.Contains(homedash.AllKinds(), f.Kind):
		h.bad("kind must be a catalog kind")
		return
	case utf8.RuneCountInString(f.Query) > 100 || utf8.RuneCountInString(f.Area) > 80:
		h.bad("q or area is too long")
		return
	}
	var ok bool
	if f.Limit, ok = h.intParam("limit", 100, 1, 500); !ok {
		return
	}
	if f.Offset, ok = h.intParam("offset", 0, 0, 1<<30); !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	items, total, sources := h.svc.hub.Entities(ctx, f)
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"entities": homeDashNonNil(items), "total": total, "limit": f.Limit, "offset": f.Offset, "sources": homeDashNonNil(sources)})
}

func (h *hdRequest) getEntity() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	e, err := h.svc.hub.Entity(ctx, id)
	if err != nil {
		h.fail(err)
		return
	}
	homeDashWriteOK(h.w, http.StatusOK, map[string]any{"entity": e})
}

func (h *hdRequest) history() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	q := h.r.URL.Query()
	to := time.Now().UTC()
	if raw := q.Get("to"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			h.bad("to must be an RFC 3339 time")
			return
		}
		to = t
	}
	from := to.Add(-24 * time.Hour)
	if raw := q.Get("from"); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			h.bad("from must be an RFC 3339 time")
			return
		}
		from = t
	}
	points, ok := h.intParam("points", 120, 1, homedash.MaxHistoryPoints)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(h.r.Context(), homeDashCatalogWait)
	defer cancel()
	series, err := h.svc.hub.History(ctx, id, from, to, points)
	if err != nil {
		h.fail(err)
		return
	}
	fields := map[string]any{"id": id, "origin": series.Origin, "points": homeDashNonNil(series.Points)}
	if series.Origin == "memory" {
		fields["since"] = series.Since.UTC()
	}
	homeDashWriteOK(h.w, http.StatusOK, fields)
}

func (h *hdRequest) snapshot() {
	id, ok := h.entityID()
	if !ok {
		return
	}
	width, ok := h.intParam("w", 640, 1, 1920)
	if !ok {
		return
	}
	height, ok := h.intParam("h", 360, 1, 1080)
	if !ok {
		return
	}
	img, ct, err := h.svc.hub.Snapshot(h.r.Context(), id, width, height)
	if err != nil {
		h.fail(err)
		return
	}
	switch ct {
	case "image/jpeg", "image/png", "image/webp":
	default:
		homeDashWriteError(h.w, http.StatusBadGateway, homedash.CodeSourceError, "the source sent no image", nil)
		return
	}
	hdr := h.w.Header()
	hdr.Set("Content-Type", ct)
	hdr.Set("Content-Length", strconv.Itoa(len(img)))
	hdr.Set("X-Content-Type-Options", "nosniff")
	hdr.Set("Cache-Control", "no-store")
	h.w.WriteHeader(http.StatusOK)
	_, _ = h.w.Write(img)
}
