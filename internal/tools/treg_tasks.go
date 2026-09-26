package tools

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/security"
)

type tregAsync struct {
	IDFrom string `json:"id_from"`
	Poll   struct {
		Endpoint string `json:"endpoint"`
		Param    struct {
			In   string `json:"in"`
			Name string `json:"name"`
		} `json:"param"`
		URLFrom  string   `json:"url_from"`
		URLHosts []string `json:"url_hosts"`
	} `json:"poll"`
	Status struct {
		Path          string   `json:"path"`
		Success       []string `json:"success"`
		Failure       []string `json:"failure"`
		BilledFailure []string `json:"billed_failure"`
	} `json:"status"`
	Result struct {
		Path       string `json:"path"`
		Fetch      string `json:"fetch"`
		FetchParam struct {
			Name      string `json:"name"`
			ValueFrom string `json:"value_from"`
		} `json:"fetch_param"`
	} `json:"result"`
	Interval int `json:"interval"`
}

type tregTask struct {
	mu            sync.Mutex
	owner         [32]byte
	expires       time.Time
	endpoint      TregEndpoint
	operation     string
	rule          tregAsync
	pollPath      string
	query         url.Values
	pollContract  *TregEndpoint
	fetchContract *TregEndpoint
	result        TregResult
}

// ponytail: process-local 256-entry/24-hour continuation store; receipt IDs in
// tool history remain usable after restart. No billing DB or background poller.
var tregTasks = struct {
	sync.Mutex
	items map[string]*tregTask
}{items: make(map[string]*tregTask)}

func (c *TregClient) taskOwner() [32]byte {
	return sha256.Sum256([]byte(c.token + "\x00" + c.SessionID))
}

func tregPath(value any, path string) any {
	path = strings.ReplaceAll(strings.ReplaceAll(path, "[", "."), "]", "")
	for _, key := range strings.Split(strings.TrimPrefix(path, "$."), ".") {
		if key == "" || key == "$" {
			continue
		}
		switch node := value.(type) {
		case map[string]any:
			value = node[key]
		case []any:
			i, err := strconv.Atoi(key)
			if err != nil || i < 0 || i >= len(node) {
				return nil
			}
			value = node[i]
		default:
			return nil
		}
	}
	return value
}

func (c *TregClient) acceptTask(ctx context.Context, ep *TregEndpoint, operation string, headers http.Header, result *TregResult) error {
	rule := ep.Async
	if h := headers.Get("X-Treg-Async"); h != "" {
		if len(h) > 16384 {
			return fmt.Errorf("oversized async descriptor")
		}
		var described tregAsync
		if err := json.Unmarshal([]byte(h), &described); err != nil {
			return fmt.Errorf("invalid async descriptor: %w", err)
		}
		// The provider may supply a descriptor in the response, but it cannot
		// replace a different contract already advertised by the catalog.
		if rule != nil {
			a, _ := json.Marshal(rule)
			b, _ := json.Marshal(described)
			if string(a) != string(b) {
				return fmt.Errorf("async contract changed")
			}
		} else {
			rule = &described
		}
	}
	if rule == nil {
		if result.Status == "pending" {
			result.ReservedMicro = result.HeaderCostMicro
			result.Message = "Accepted without a supported continuation descriptor; inspect call accounting. Do not resubmit."
			return nil
		}
		result.ChargedMicro = result.HeaderCostMicro
		if media := tregMediaPayload(result.Data, ep.Platform); media != nil {
			if err := c.collectMedia(ctx, media, result, ep, operation); err != nil {
				return err
			}
		}
		return nil
	}
	task := &tregTask{owner: c.taskOwner(), expires: time.Now().Add(24 * time.Hour), endpoint: TregEndpoint{ID: ep.ID, Method: ep.Method, Path: ep.Path}, operation: operation, rule: *rule, query: url.Values{}}
	if rule.Poll.Endpoint != "" {
		if !config.ValidTregEndpointID(rule.Poll.Endpoint) || rule.Poll.Param.Name == "" {
			return fmt.Errorf("invalid polling endpoint descriptor")
		}
		id := tregPath(result.Data, rule.IDFrom)
		values, err := tregValues(id)
		if err != nil || len(values) != 1 {
			return fmt.Errorf("missing async task ID")
		}
		task.pollPath = "/call/" + rule.Poll.Endpoint
		task.query.Set(rule.Poll.Param.Name, values[0])
		poll, _, err := c.Endpoint(ctx, rule.Poll.Endpoint)
		if err != nil {
			return err
		}
		if poll.Method != "GET" {
			return fmt.Errorf("polling endpoint is not a read-only HTTP method")
		}
		task.pollContract = &poll
	} else if rule.Poll.URLFrom != "" {
		raw, _ := tregPath(result.Data, rule.Poll.URLFrom).(string)
		u, err := url.Parse(raw)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || u.Port() != "" || !slices.Contains(rule.Poll.URLHosts, u.Hostname()) {
			return fmt.Errorf("untrusted async polling URL")
		}
		if _, err := security.NewStrictPublicHTTPClientForURL(raw, 30*time.Second); err != nil {
			return fmt.Errorf("polling destination rejected: %w", err)
		}
		// Only a server-returned URL on the descriptor's exact hosts can use
		// treg's URL proxy. There is no model-supplied URL parameter.
		task.pollPath = "/call/" + u.Scheme + "://" + u.Host + u.EscapedPath()
		task.query = u.Query()
	} else {
		return fmt.Errorf("missing polling descriptor")
	}
	if rule.Status.Path == "" || len(rule.Status.Success) == 0 || len(rule.Status.Failure) == 0 {
		return fmt.Errorf("incomplete async status descriptor")
	}
	if rule.Result.Fetch != "" {
		fetch, _, err := c.Endpoint(ctx, rule.Result.Fetch)
		if err != nil {
			return err
		}
		if fetch.Method != "GET" {
			return fmt.Errorf("media fetch must use a read-only GET endpoint")
		}
		task.fetchContract = &fetch
	}
	result.Status = "pending"
	result.ReservedMicro = result.HeaderCostMicro
	result.RetryAfter = max(1, rule.Interval)
	result.Message = "Accepted; generation is not complete. Use this continuation for one bounded status check."
	ref := tregRandomID()
	tregTasks.Lock()
	defer tregTasks.Unlock()
	for key, t := range tregTasks.items {
		if time.Now().After(t.expires) {
			delete(tregTasks.items, key)
		}
	}
	if len(tregTasks.items) >= 256 {
		return fmt.Errorf("continuation capacity reached; use the treg call ID")
	}
	result.Continuation = ref
	task.result = *result
	task.result.Data = nil
	tregTasks.items[ref] = task
	return nil
}

func (c *TregClient) Tasks() []TregResult {
	tregTasks.Lock()
	var own []*tregTask
	for _, task := range tregTasks.items {
		if task.owner == c.taskOwner() && time.Now().Before(task.expires) {
			own = append(own, task)
		}
	}
	tregTasks.Unlock()
	results := []TregResult{}
	for _, task := range own {
		task.mu.Lock()
		if _, err := c.grant(task.endpoint.ID, task.operation, &task.endpoint); err == nil {
			r := task.result
			r.Data = nil
			results = append(results, r)
		}
		task.mu.Unlock()
	}
	return results
}

func (c *TregClient) Poll(ctx context.Context, ref string) (*TregResult, error) {
	tregTasks.Lock()
	task := tregTasks.items[ref]
	tregTasks.Unlock()
	if task == nil || task.owner != c.taskOwner() || time.Now().After(task.expires) {
		return nil, &TregError{Status: "policy_denied", Text: "unknown, expired or foreign continuation; use the treg call ID after restart"}
	}
	task.mu.Lock()
	defer task.mu.Unlock()
	if _, err := c.grant(task.endpoint.ID, task.operation, &task.endpoint); err != nil {
		return nil, err
	}
	current, _, err := c.Endpoint(ctx, task.endpoint.ID)
	if err != nil {
		return nil, err
	}
	if _, err := c.grant(current.ID, task.operation, &current); err != nil {
		return nil, err
	}
	if task.result.Status == "success" || task.result.Status == "error" {
		copy := task.result
		return &copy, nil
	}
	if task.rule.Poll.Endpoint != "" {
		poll, _, err := c.Endpoint(ctx, task.rule.Poll.Endpoint)
		if err != nil {
			return nil, err
		}
		if poll.Method != "GET" {
			return nil, &TregError{Status: "policy_denied", Text: "continuation requires a catalog-declared read-only HTTP method"}
		}
		if task.pollContract != nil && (poll.Method != task.pollContract.Method || poll.Path != task.pollContract.Path) {
			return nil, &TregError{Status: "policy_denied", Text: "polling contract changed"}
		}
		task.pollContract = &poll
		name := task.rule.Poll.Param.Name
		if poll.Input.PathParams[name] == nil && poll.Input.QueryParams[name] == nil {
			return nil, fmt.Errorf("polling parameter is not declared")
		}
	}
	resp, err := c.request(ctx, "GET", task.pollPath, task.query, nil, "", tregRandomID(), &task.endpoint, task.operation)
	if err != nil {
		return nil, err
	}
	next := task.result
	next.Data, next.Media = nil, nil
	var probe TregResult
	if err := c.readCallResponse(resp, &probe); err != nil {
		return nil, err
	}
	next.Data = probe.Data
	if probe.Status != "success" && probe.Status != "pending" {
		next.Message = fmt.Sprintf("Status check HTTP %d did not confirm task completion; no resubmission was made", probe.HTTPStatus)
		return &next, nil
	}
	status, _ := tregPath(probe.Data, task.rule.Status.Path).(string)
	switch {
	case slices.Contains(task.rule.Status.Failure, status), slices.Contains(task.rule.Status.BilledFailure, status):
		next.Status, next.Message = "error", "Provider task failed; query call accounting for any refund."
	case slices.Contains(task.rule.Status.Success, status):
		next.Status, next.Message = "success", "Provider task completed; final billing is available through call accounting."
		if task.rule.Result.Fetch != "" {
			if err := c.fetchTaskResult(ctx, task, probe.Data, &next); err != nil {
				next.Status, next.Message = "pending", "Task completed, but its media could not be stored: "+err.Error()
			}
		} else if path := task.rule.Result.Path; path != "" {
			if err := c.collectMedia(ctx, tregPath(probe.Data, path), &next, &task.endpoint, task.operation); err != nil {
				next.Status, next.Message = "pending", "Task completed, but media download failed: "+err.Error()
			}
		}
	default:
		next.Status, next.Message = "pending", "Provider task is still pending; use the continuation for another status check."
	}
	task.result = next
	task.result.Data = nil
	return &next, nil
}

func (c *TregClient) collectMedia(ctx context.Context, value any, result *TregResult, endpoint *TregEndpoint, operation string) error {
	values := []any{value}
	if list, ok := value.([]any); ok {
		values = list
	}
	if len(values) > 8 {
		return fmt.Errorf("at most eight media resources per result")
	}
	for _, value := range values {
		if _, err := c.grant(endpoint.ID, operation, endpoint); err != nil {
			return err
		}
		item, err := c.resultMedia(ctx, value)
		if err != nil {
			return err
		}
		result.Media = append(result.Media, item)
	}
	return nil
}

func (c *TregClient) fetchTaskResult(ctx context.Context, task *tregTask, data any, result *TregResult) error {
	rule := task.rule.Result
	ep, _, err := c.Endpoint(ctx, rule.Fetch)
	if err != nil {
		return err
	}
	if ep.Method != "GET" {
		return fmt.Errorf("media fetch must use a read-only GET endpoint")
	}
	if task.fetchContract == nil || ep.Method != task.fetchContract.Method || ep.Path != task.fetchContract.Path {
		return &TregError{Status: "policy_denied", Text: "media fetch contract changed"}
	}
	path := rule.FetchParam.ValueFrom
	values, err := tregValues(tregPath(data, path))
	if err != nil || len(values) != 1 || rule.FetchParam.Name == "" {
		return fmt.Errorf("invalid result fetch parameter")
	}
	if ep.Input.PathParams[rule.FetchParam.Name] == nil && ep.Input.QueryParams[rule.FetchParam.Name] == nil {
		return fmt.Errorf("fetch parameter is not declared")
	}
	resp, err := c.request(ctx, "GET", "/call/"+ep.ID, url.Values{rule.FetchParam.Name: values}, nil, "", tregRandomID(), &task.endpoint, task.operation)
	if err != nil {
		return err
	}
	var fetched TregResult
	if err := c.readCallResponse(resp, &fetched); err != nil {
		return err
	}
	if fetched.Status != "success" {
		return fmt.Errorf("result fetch HTTP %d", fetched.HTTPStatus)
	}
	if len(fetched.Media) == 0 {
		media := tregMediaPayload(fetched.Data, "video-gen")
		if media == nil {
			return fmt.Errorf("fetch returned no explicit media resource")
		}
		if err := c.collectMedia(ctx, media, &fetched, &task.endpoint, task.operation); err != nil {
			return err
		}
	}
	result.Media = append(result.Media, fetched.Media...)
	return nil
}
