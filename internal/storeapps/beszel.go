package storeapps

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"aurago/internal/security"
)

var recordID = regexp.MustCompile(`^[a-zA-Z0-9]{1,32}$`)

func (c Connection) beszelLogin(ctx context.Context) (Connection, error) {
	var auth struct {
		Token  string `json:"token"`
		Record struct {
			CollectionName string `json:"collectionName"`
		} `json:"record"`
	}
	if c.Username == "" {
		return c, errors.New("Enter a normal Beszel user's email and password")
	}
	err := c.jsonRequest(ctx, "POST", "/api/collections/users/auth-with-password", map[string]string{"identity": c.Username, "password": c.Credential}, false, "", "", &auth)
	if err != nil {
		return c, err
	}
	if auth.Token == "" || auth.Record.CollectionName != "users" {
		return c, errors.New("A normal Beszel user is required; superusers are not accepted")
	}
	security.RegisterSensitive(auth.Token)
	c.Credential = auth.Token
	return c, nil
}

type BeszelSystem struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Updated string `json:"updated"`
	Stale   bool   `json:"stale"`
}

func metricStale(value string) bool {
	timestamp, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		timestamp, err = time.Parse("2006-01-02 15:04:05.999Z", value)
	}
	age := time.Since(timestamp)
	return err != nil || age > 5*time.Minute || age < -time.Minute
}

func (c Connection) BeszelSystems(ctx context.Context, offset, limit int) (any, error) {
	auth, err := c.beszelLogin(ctx)
	if err != nil {
		return nil, err
	}
	// PocketBase pages are 1-based; a fixed page width preserves arbitrary offsets.
	var page struct {
		Items      []BeszelSystem `json:"items"`
		TotalItems int            `json:"totalItems"`
	}
	pageNumber := offset/50 + 1
	err = auth.jsonRequest(ctx, "GET", "/api/collections/systems/records?perPage=50&page="+strconv.Itoa(pageNumber)+"&sort=name&fields=id,name,status,updated", nil, false, "Authorization", auth.Credential, &page)
	if err != nil {
		return nil, err
	}
	start := min(offset%50, len(page.Items))
	end := min(start+limit, len(page.Items))
	items := page.Items[start:end]
	for i := range items {
		items[i].Stale = metricStale(items[i].Updated)
	}
	return map[string]any{"systems": items, "has_more": offset+len(items) < page.TotalItems, "next_offset": offset + len(items)}, nil
}

func (c Connection) BeszelMetrics(ctx context.Context, id string, containers bool) (any, error) {
	if !recordID.MatchString(id) {
		return nil, errors.New("Invalid Beszel system ID")
	}
	auth, err := c.beszelLogin(ctx)
	if err != nil {
		return nil, err
	}
	collection := "system_stats"
	if containers {
		collection = "container_stats"
	}
	query := url.Values{"perPage": {"1"}, "sort": {"-created"}, "filter": {"system='" + id + "' && type='1m'"}, "fields": {"created,stats"}}
	type hostStats struct {
		CPU           float64   `json:"cpu"`
		MemoryPercent float64   `json:"mp"`
		MemoryGiB     float64   `json:"m"`
		MemoryUsedGiB float64   `json:"mu"`
		MemoryBytes   [4]uint64 `json:"mem"`
		DiskGiB       float64   `json:"d"`
		DiskUsedGiB   float64   `json:"du"`
		DiskPercent   float64   `json:"dp"`
		Load          []float64 `json:"la"`
		Bandwidth     [2]uint64 `json:"b"`
	}
	type containerStats struct {
		Name      string    `json:"n"`
		CPU       float64   `json:"c"`
		MemoryMiB float64   `json:"m"`
		Bandwidth [2]uint64 `json:"b"`
	}
	endpoint := "/api/collections/" + collection + "/records?" + query.Encode()
	if containers {
		var page struct {
			Items []struct {
				Created string           `json:"created"`
				Stats   []containerStats `json:"stats"`
			} `json:"items"`
		}
		if err = auth.jsonRequest(ctx, "GET", endpoint, nil, false, "Authorization", auth.Credential, &page); err != nil {
			return nil, err
		}
		if len(page.Items) == 0 {
			return map[string]any{"system_id": id, "stale": true, "containers": []any{}}, nil
		}
		row := page.Items[0]
		return map[string]any{"system_id": id, "timestamp": row.Created, "stale": metricStale(row.Created), "containers": row.Stats, "units": map[string]string{"c": "CPU percent", "m": "MiB", "b": "sent/received bytes"}}, nil
	}
	var page struct {
		Items []struct {
			Created string    `json:"created"`
			Stats   hostStats `json:"stats"`
		} `json:"items"`
	}
	if err = auth.jsonRequest(ctx, "GET", endpoint, nil, false, "Authorization", auth.Credential, &page); err != nil {
		return nil, err
	}
	if len(page.Items) == 0 {
		return map[string]any{"system_id": id, "stale": true, "metrics": nil}, nil
	}
	row := page.Items[0]
	return map[string]any{"system_id": id, "timestamp": row.Created, "stale": metricStale(row.Created), "metrics": row.Stats, "units": map[string]string{"cpu,mp,dp": "percent", "m,mu,d,du": "GiB", "mem": "total/used/cache/ARC bytes", "b": "sent/received bytes"}}, nil
}
