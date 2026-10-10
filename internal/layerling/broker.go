// Package layerling routes bounded CAD commands to a specific live browser editor.
package layerling

import (
	"context"
	"crypto/rand"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

const MaxArguments = 64 << 10
const MaxResult = 32 << 10

//go:embed contract.json
var contractJSON []byte

type Definition struct {
	Description string          `json:"description"`
	Schema      json.RawMessage `json:"schema"`
}

var contract = func() map[string]Definition {
	var v map[string]Definition
	if err := json.Unmarshal(contractJSON, &v); err != nil {
		panic(err)
	}
	return v
}()
var validators = func() map[string]*gojsonschema.Schema {
	v := map[string]*gojsonschema.Schema{}
	for k, d := range contract {
		s, e := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(d.Schema))
		if e != nil {
			panic(e)
		}
		v[k] = s
	}
	return v
}()

func Operations() []string {
	v := []string{"list_editors", "describe"}
	for k := range contract {
		v = append(v, k)
	}
	sort.Strings(v)
	return v
}
func ReadOnly(operation string) bool {
	switch operation {
	case "get_scene", "list_objects", "list_edges", "inspect_errors", "capture_image", "estimate_print", "list_custom_shapes", "list_reference_points":
		return true
	}
	return false
}
func Validate(operation string, args json.RawMessage) error {
	s := validators[operation]
	if s == nil {
		return errors.New("unknown Layerling operation")
	}
	if len(args) > MaxArguments {
		return errors.New("Layerling arguments exceed limit")
	}
	result, err := s.Validate(gojsonschema.NewBytesLoader(args))
	if err != nil {
		return fmt.Errorf("invalid Layerling arguments: %w", err)
	}
	if !result.Valid() {
		return fmt.Errorf("invalid Layerling arguments: %s", result.Errors()[0].String())
	}
	return nil
}

type Command struct {
	ID        string          `json:"id"`
	Operation string          `json:"operation"`
	Arguments json.RawMessage `json:"arguments"`
}
type Result struct {
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}
type Editor struct {
	ID              string `json:"editor_id"`
	WindowID        string `json:"window_id"`
	owner           string
	activeID        string
	activeOperation string
	activePath      string
	allow           func(bool) bool
	commands        chan Command
	results         chan Result
	done            chan struct{}
	gate            chan struct{}
	once            sync.Once
}
type Broker struct {
	mu      sync.Mutex
	editors map[string]*Editor
}

func NewBroker() *Broker { return &Broker{editors: map[string]*Editor{}} }
func newID() string {
	var v [24]byte
	if _, err := rand.Read(v[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(v[:])
}
func (b *Broker) Register(owner, windowID string, allow func(bool) bool) *Editor {
	e := &Editor{ID: newID(), WindowID: windowID, owner: owner, allow: allow, commands: make(chan Command), results: make(chan Result, 1), done: make(chan struct{}), gate: make(chan struct{}, 1)}
	b.mu.Lock()
	if len(b.editors) >= 64 {
		b.mu.Unlock()
		return nil
	}
	b.editors[e.ID] = e
	b.mu.Unlock()
	return e
}

// AuthorizeFile binds agent file I/O to the command still running in that editor.
func (b *Broker) AuthorizeFile(owner, id, command, path string, write bool) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.editors[id]
	return e.fileAllowed(owner, command, path, write)
}

func (e *Editor) fileAllowed(owner, command, path string, write bool) bool {
	if e == nil || e.owner != owner || command == "" || e.activeID != command || path == "" || e.activePath != path || !e.allow(write) {
		return false
	}
	if write {
		return e.activeOperation == "save_project" || e.activeOperation == "export_model"
	}
	return e.activeOperation == "open_project" || e.activeOperation == "import_model"
}

// PublishFile serializes a bounded file publication with disconnect/revocation.
func (b *Broker) PublishFile(owner, id, command, path string, publish func() error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	e := b.editors[id]
	if !e.fileAllowed(owner, command, path, true) {
		return errors.New("Layerling file command revoked")
	}
	return publish()
}
func (b *Broker) Remove(e *Editor) {
	b.mu.Lock()
	e.once.Do(func() { close(e.done) })
	delete(b.editors, e.ID)
	b.mu.Unlock()
}
func (e *Editor) Commands() <-chan Command { return e.commands }
func (e *Editor) Done() <-chan struct{}    { return e.done }
func (e *Editor) Reply(r Result) bool {
	select {
	case e.results <- r:
		return true
	default:
		return false
	}
}

type binding struct {
	broker *Broker
	owner  string
	valid  func() bool
}
type contextKey struct{}

func WithOwner(ctx context.Context, b *Broker, owner string, valid func() bool) context.Context {
	return context.WithValue(ctx, contextKey{}, binding{b, owner, valid})
}

// Execute has no process-global fallback: callers without a browser owner fail closed.
func Execute(ctx context.Context, operation, id string, args json.RawMessage) (json.RawMessage, error) {
	auth, ok := ctx.Value(contextKey{}).(binding)
	if !ok || auth.owner == "" || !auth.valid() {
		return nil, errors.New("Layerling requires an authorized Desktop chat session")
	}
	if operation == "describe" {
		var p struct {
			Operation string `json:"operation"`
		}
		if len(args) > 1024 || json.Unmarshal(args, &p) != nil {
			return nil, errors.New("invalid description request")
		}
		d, ok := contract[p.Operation]
		if !ok {
			return nil, errors.New("unknown Layerling operation")
		}
		return json.Marshal(d)
	}
	b := auth.broker
	b.mu.Lock()
	if operation == "list_editors" {
		list := []map[string]string{}
		for _, e := range b.editors {
			if e.owner == auth.owner && e.allow(false) {
				list = append(list, map[string]string{"editor_id": e.ID, "window_id": e.WindowID})
			}
		}
		b.mu.Unlock()
		return json.Marshal(list)
	}
	e := b.editors[id]
	b.mu.Unlock()
	if e == nil || e.owner != auth.owner {
		return nil, errors.New("select an authorized editor_id from list_editors")
	}
	if err := Validate(operation, args); err != nil {
		return nil, err
	}
	write := !ReadOnly(operation)
	if !e.allow(write) {
		return nil, errors.New("Layerling access denied")
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	select {
	case e.gate <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.done:
		return nil, errors.New("editor disconnected")
	}
	defer func() { <-e.gate }()
	if !auth.valid() || !e.allow(write) {
		return nil, errors.New("Layerling permission revoked")
	}
	command := Command{newID(), operation, args}
	var file struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(args, &file)
	b.mu.Lock()
	e.activeID = command.ID
	e.activeOperation = operation
	e.activePath = file.Path
	b.mu.Unlock()
	defer func() { b.mu.Lock(); e.activeID = ""; b.mu.Unlock() }()
	select {
	case e.commands <- command:
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-e.done:
		return nil, errors.New("editor disconnected")
	}
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case r := <-e.results:
			if r.ID != command.ID {
				b.Remove(e)
				return nil, errors.New("invalid editor response; connection revoked")
			}
			if !auth.valid() || !e.allow(write) {
				b.Remove(e)
				return nil, errors.New("Layerling permission revoked")
			}
			if !r.OK {
				return nil, errors.New("Layerling command failed: " + r.Error)
			}
			if len(r.Data) > MaxResult {
				return nil, errors.New("Layerling result exceeds context limit; request fewer objects")
			}
			return r.Data, nil
		case <-tick.C:
			if !auth.valid() || !e.allow(write) {
				b.Remove(e)
				return nil, errors.New("Layerling permission revoked; result uncertain, do not retry")
			}
		case <-ctx.Done():
			b.Remove(e)
			return nil, errors.New("Layerling command timed out; result uncertain, do not retry")
		case <-e.done:
			return nil, errors.New("editor disconnected; result uncertain, do not retry")
		}
	}
}
