package storeapps

import (
	"context"
	"crypto/rand"
	"errors"
	"strings"
	"unicode/utf8"
)

const oliveTinAPI = "/api/olivetin.api.v1.OliveTinApiService/"

type OliveTinArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type OliveTinArgumentDefinition struct {
	Name    string `json:"name"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Default string `json:"defaultValue,omitempty"`
	Choices []struct {
		Value string `json:"value"`
		Title string `json:"title"`
	} `json:"choices,omitempty"`
}
type OliveTinAction struct {
	ID         string                       `json:"bindingId"`
	Title      string                       `json:"title"`
	CanExecute bool                         `json:"canExec"`
	Arguments  []OliveTinArgumentDefinition `json:"arguments"`
}

func (c Connection) OliveTinProbe(ctx context.Context) error {
	var init struct {
		User          string `json:"authenticatedUser"`
		LoginRequired bool   `json:"loginRequired"`
		Version       string `json:"currentVersion"`
	}
	if err := c.request(ctx, "POST", oliveTinAPI+"Init", struct{}{}, false, &init); err != nil {
		return err
	}
	if init.LoginRequired || init.User == "" || init.Version == "" {
		return errors.New("OliveTin needs a dedicated authenticated local user and a compatible API")
	}
	return nil
}

func (c Connection) oliveTinAction(ctx context.Context, id string) (OliveTinAction, error) {
	var result struct {
		Action OliveTinAction `json:"action"`
	}
	if !c.Allowed(id) {
		return result.Action, errors.New("OliveTin action is not administratively allowed")
	}
	if err := c.request(ctx, "POST", oliveTinAPI+"GetActionBinding", map[string]string{"bindingId": id}, false, &result); err != nil {
		return result.Action, err
	}
	if result.Action.ID != id || len(result.Action.Arguments) > 64 {
		return result.Action, errors.New("OliveTin action definition is unavailable or incompatible")
	}
	return result.Action, nil
}

func (c Connection) OliveTinActions(ctx context.Context, offset, limit int) (any, error) {
	if offset < 0 || limit < 1 || limit > 50 {
		return nil, errors.New("Use offset >= 0 and limit between 1 and 50")
	}
	items := []OliveTinAction{}
	end := min(len(c.AllowedIDs), offset+limit)
	for i := offset; i < end; i++ {
		action, err := c.oliveTinAction(ctx, c.AllowedIDs[i])
		if err != nil {
			return nil, err
		}
		for j := range action.Arguments {
			action.Arguments[j].Default = ""
		}
		items = append(items, action)
	}
	return map[string]any{"actions": items, "has_more": end < len(c.AllowedIDs), "next_offset": end}, nil
}

func (c Connection) OliveTinExecute(ctx context.Context, id string, args []OliveTinArgument) (any, error) {
	action, err := c.oliveTinAction(ctx, id)
	if err != nil {
		return nil, err
	}
	if !action.CanExecute {
		return nil, errors.New("OliveTin denies execution for this user")
	}
	if len(args) > 64 {
		return nil, errors.New("Too many action arguments")
	}
	values := map[string]string{}
	for _, arg := range args {
		if _, duplicate := values[arg.Name]; duplicate || arg.Name == "" || len(arg.Value) > 4096 || !utf8.ValidString(arg.Value) || strings.ContainsRune(arg.Value, 0) {
			return nil, errors.New("Invalid action argument")
		}
		values[arg.Name] = arg.Value
	}
	validated := make([]OliveTinArgument, 0, len(action.Arguments))
	for _, definition := range action.Arguments {
		value, exists := values[definition.Name]
		if !exists {
			value = definition.Default
		}
		delete(values, definition.Name)
		if len(definition.Choices) > 0 {
			found := false
			for _, choice := range definition.Choices {
				if choice.Value == value {
					found = true
				}
			}
			if !found {
				return nil, errors.New("Action argument is not one of the configured choices")
			}
		}
		var check struct {
			Valid bool `json:"valid"`
		}
		body := map[string]string{"bindingId": id, "argumentName": definition.Name, "type": definition.Type, "value": value}
		if err := c.request(ctx, "POST", oliveTinAPI+"ValidateArgumentType", body, false, &check); err != nil {
			return nil, err
		}
		if !check.Valid {
			return nil, errors.New("OliveTin rejected an action argument")
		}
		validated = append(validated, OliveTinArgument{definition.Name, value})
	}
	if len(values) != 0 {
		return nil, errors.New("Unknown action argument")
	}
	tracking := rand.Text()
	var result struct {
		ID string `json:"executionTrackingId"`
	}
	err = c.request(ctx, "POST", oliveTinAPI+"StartAction", map[string]any{"bindingId": id, "arguments": validated, "uniqueTrackingId": tracking}, true, &result)
	if errors.Is(err, ErrUncertain) {
		return map[string]any{"status": "unknown", "uncertain": true, "action_id": id, "execution_id": tracking, "message": ErrUncertain.Error()}, nil
	}
	if err != nil {
		return nil, err
	}
	if result.ID == "" {
		return nil, ErrUncertain
	}
	return map[string]any{"status": "accepted", "action_id": id, "execution_id": result.ID}, nil
}

func (c Connection) OliveTinStatus(ctx context.Context, id, execution string) (any, error) {
	if !c.Allowed(id) || execution == "" || len(execution) > 256 {
		return nil, errors.New("Select an allowed action and its execution ID")
	}
	var result struct {
		Log struct {
			ID         string `json:"bindingId"`
			Execution  string `json:"executionTrackingId"`
			Started    bool   `json:"executionStarted"`
			Finished   bool   `json:"executionFinished"`
			Blocked    bool   `json:"blocked"`
			Queued     bool   `json:"queued"`
			TimedOut   bool   `json:"timedOut"`
			ExitCode   int    `json:"exitCode"`
			StartedAt  string `json:"datetimeStarted"`
			FinishedAt string `json:"datetimeFinished"`
			Output     string `json:"output"`
		} `json:"logEntry"`
	}
	if err := c.request(ctx, "POST", oliveTinAPI+"ExecutionStatus", map[string]string{"actionId": id, "executionTrackingId": execution}, false, &result); err != nil {
		return nil, err
	}
	if result.Log.ID != id || result.Log.Execution != execution {
		return nil, errors.New("Execution does not belong to the selected action")
	}
	if len(result.Log.Output) > 16384 {
		result.Log.Output = string([]rune(result.Log.Output[:16384])) + "\n[output truncated]"
	}
	return result.Log, nil
}
