package localwiki

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	catalogCacheTTL     = 6 * time.Hour
	catalogBodyLimit    = 1 << 20
	smallFetchTimeout   = 30 * time.Second
	updateCheckInterval = 24 * time.Hour
	loopTick            = time.Hour
)

// Catalog returns the newest maxi and nopic edition Kiwix offers for an
// offered language. Results are cached for six hours.
func (m *Manager) Catalog(ctx context.Context, language string) (CatalogInfo, error) {
	return m.catalog(ctx, language, false)
}

func (m *Manager) catalog(ctx context.Context, language string, refresh bool) (CatalogInfo, error) {
	spec, ok := lookupLanguage(language)
	if !ok {
		return CatalogInfo{}, ErrUnknownLanguage
	}
	now := m.now()
	m.mu.Lock()
	cached, found := m.catalogCache[spec.Code]
	m.mu.Unlock()
	if found && !refresh && now.Sub(cached.fetched) < catalogCacheTTL {
		return cloneCatalogInfo(cached.info), nil
	}
	endpoint := m.catalogBase.JoinPath("catalog", "v2", "entries")
	endpoint.RawQuery = url.Values{"name": {"wikipedia_" + spec.Kiwix + "_all"}, "count": {"-1"}}.Encode()
	data, err := m.fetchSmall(ctx, endpoint.String(), catalogBodyLimit)
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("%w: %v", ErrCatalogUnreachable, err)
	}
	variants, err := parseOPDS(data, spec.Kiwix, endpoint)
	if err != nil {
		return CatalogInfo{}, fmt.Errorf("%w: %v", ErrCatalogUnreachable, err)
	}
	info := CatalogInfo{Language: spec.Code, Fulltext: fulltextSupported(spec), Variants: variants}
	m.mu.Lock()
	m.catalogCache[spec.Code] = catalogCacheEntry{info: cloneCatalogInfo(info), fetched: now}
	m.mu.Unlock()
	return info, nil
}

// fetchSmall GETs a catalog or .meta4 document over HTTPS with a deadline and
// a size limit.
func (m *Manager) fetchSmall(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	if _, err := requireHTTPS(rawURL); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, smallFetchTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, req.URL.Host)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("response from %s exceeds %d bytes", req.URL.Host, limit)
	}
	return data, nil
}

func cloneCatalogInfo(info CatalogInfo) CatalogInfo {
	out := info
	out.Variants = make(map[Variant]CatalogEdition, len(info.Variants))
	for variant, edition := range info.Variants {
		out.Variants[variant] = edition
	}
	return out
}

// CheckUpdate asks the catalog (bypassing the cache) for a newer edition of the
// installed language and variant and records it in the status. It never
// downloads anything.
func (m *Manager) CheckUpdate(ctx context.Context) error {
	m.mu.Lock()
	if m.state == nil || m.state.Edition == nil {
		m.mu.Unlock()
		return nil
	}
	installed := *m.state.Edition
	dir := m.activeDir
	m.mu.Unlock()
	info, err := m.catalog(ctx, installed.Language, true)
	if err != nil {
		return err
	}
	var update *UpdateInfo
	if latest, ok := info.Variants[installed.Variant]; ok && editionNewer(latest.Name, installed.Name) {
		update = &UpdateInfo{Date: latest.Date, Size: latest.Size}
	}
	m.mu.Lock()
	if m.activeDir != dir || m.state == nil || m.state.Edition == nil || m.state.Edition.Name != installed.Name {
		m.mu.Unlock()
		return nil
	}
	m.update = update
	m.state.LastUpdateCheck = m.now().UTC()
	m.mu.Unlock()
	if err := m.saveState(dir); err != nil {
		m.logger.Warn("[LocalWikipedia] state.json could not be updated", "error", err)
	}
	return nil
}

// maybeCheckUpdate runs the daily update check when it is enabled, an edition
// is installed, nothing is downloading and the last check is a day old.
func (m *Manager) maybeCheckUpdate(ctx context.Context) {
	m.mu.Lock()
	due := m.settings.Enabled && m.settings.UpdateCheck && !m.shuttingDown && m.op == nil &&
		m.state != nil && m.state.Edition != nil && m.now().Sub(m.state.LastUpdateCheck) >= updateCheckInterval
	m.mu.Unlock()
	if !due {
		return
	}
	if err := m.CheckUpdate(ctx); err != nil {
		m.logger.Info("[LocalWikipedia] Daily update check failed", "code", ErrorCode(err), "error", err)
	}
}
