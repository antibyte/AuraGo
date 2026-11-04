package flows

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"aurago/internal/security"
)

const (
	maxScenariosPerFlow     = 20
	maxScenarioBytes        = MaxStoredOutputBytes
	maxScenarioStorageBytes = 16 << 20
)

var (
	ErrScenarioNotFound   = errors.New("scenario not found")
	ErrScenarioConflict   = errors.New("the scenario was changed elsewhere")
	ErrScenarioStale      = errors.New("the scenario does not match the current draft")
	ErrScenarioInvalid    = errors.New("invalid simulation scenario")
	ErrScenarioUnreadable = errors.New("the encrypted scenario cannot be read")
	ErrScenarioSensitive  = errors.New("the scenario contains sensitive data")
	ErrScenarioLimit      = errors.New("the scenario storage limit was reached")
	scenarioIDPattern     = regexp.MustCompile(`^sc_[a-f0-9]{24}$`)
)

// ScenarioKeyProvider holds the scenario-only encryption key in the Vault.
// create is true only while the database write lock proves no scenario exists.
type ScenarioKeyProvider interface {
	ScenarioKey(context.Context, bool) ([]byte, error)
}

// Scenario is a private, simulation-only set of sample inputs and fixture outputs.
type Scenario struct {
	ID            string                       `json:"id"`
	Name          string                       `json:"name"`
	Version       int                          `json:"version"`
	DraftRevision int                          `json:"draft_revision"`
	TriggerNode   string                       `json:"trigger_node"`
	TriggerData   map[string]any               `json:"trigger_data"`
	OnlyNode      string                       `json:"only_node,omitempty"`
	At            time.Time                    `json:"at"`
	Fixtures      map[string]SimulationFixture `json:"fixtures"`
	Assertions    []ScenarioAssertion          `json:"assertions,omitempty"`
	Stale         bool                         `json:"stale"`
}

type ScenarioAssertion struct {
	NodeID string          `json:"node_id"`
	Output json.RawMessage `json:"output,omitempty"`
	Ports  *[]string       `json:"ports,omitempty"`
}

type ScenarioAssertionResult struct {
	NodeID string `json:"node_id"`
	Passed bool   `json:"passed"`
	Code   string `json:"code,omitempty"`
}

type ScenarioSimulationResult struct {
	*SimulationResult
	Assertions []ScenarioAssertionResult `json:"assertions"`
}

// ScenarioSummary excludes trigger and fixture data from the collection response.
type ScenarioSummary struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Version       int    `json:"version"`
	DraftRevision int    `json:"draft_revision"`
	Stale         bool   `json:"stale"`
}

type scenarioPayload struct {
	Scenario     Scenario          `json:"scenario"`
	Fingerprints map[string]string `json:"fingerprints"`
}

func scenarioAAD(flowID, scenarioID string) []byte {
	const domain = "aurago.flows.scenario.v1\x00"
	aad := make([]byte, len(domain)+4+len(flowID)+4+len(scenarioID))
	copy(aad, domain)
	at := len(domain)
	binary.BigEndian.PutUint32(aad[at:], uint32(len(flowID)))
	at += 4
	at += copy(aad[at:], flowID)
	binary.BigEndian.PutUint32(aad[at:], uint32(len(scenarioID)))
	at += 4
	copy(aad[at:], scenarioID)
	return aad
}

func scenarioAEAD(key []byte) (cipher.AEAD, error) {
	if len(key) != 32 {
		return nil, ErrScenarioInvalid
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, ErrScenarioInvalid
	}
	return cipher.NewGCM(block)
}

func sealScenario(key []byte, flowID, id string, payload scenarioPayload) ([]byte, error) {
	aead, err := scenarioAEAD(key)
	if err != nil {
		return nil, err
	}
	plain, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrScenarioInvalid
	}
	defer clear(plain)
	if len(plain)+aead.NonceSize()+aead.Overhead() > maxScenarioBytes {
		return nil, ErrTestDataTooLarge
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("create scenario nonce: %w", err)
	}
	return aead.Seal(nonce, nonce, plain, scenarioAAD(flowID, id)), nil
}

func openScenario(key []byte, flowID, id string, sealed []byte) (scenarioPayload, error) {
	var payload scenarioPayload
	aead, err := scenarioAEAD(key)
	if err != nil || len(sealed) > maxScenarioBytes {
		return payload, ErrScenarioUnreadable
	}
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return payload, ErrScenarioUnreadable
	}
	plain, err := aead.Open(nil, sealed[:aead.NonceSize()], sealed[aead.NonceSize():], scenarioAAD(flowID, id))
	if err != nil {
		return payload, ErrScenarioUnreadable
	}
	defer clear(plain)
	if json.Unmarshal(plain, &payload) != nil || payload.Scenario.ID != id || payload.Scenario.Version <= 0 || payload.Fingerprints == nil {
		return scenarioPayload{}, ErrScenarioUnreadable
	}
	return payload, nil
}

func scenarioFingerprints(doc *Flow, scenario Scenario) (map[string]string, error) {
	ids := map[string]bool{scenario.TriggerNode: true}
	if scenario.OnlyNode != "" {
		ids[scenario.OnlyNode] = true
	}
	for id := range scenario.Fixtures {
		ids[id] = true
	}
	for _, assertion := range scenario.Assertions {
		ids[assertion.NodeID] = true
	}
	fingerprints := make(map[string]string, len(ids))
	for id := range ids {
		n := doc.NodeByID(id)
		if n == nil {
			return nil, ErrScenarioInvalid
		}
		settings := n.Settings
		settings.Notes = ""
		encoded, err := json.Marshal(struct {
			Key      string         `json:"key"`
			Type     string         `json:"type"`
			Version  int            `json:"version"`
			Params   map[string]any `json:"params"`
			Settings NodeSettings   `json:"settings"`
		}{n.Key, n.Type, n.TypeVersion, n.Params, settings})
		if err != nil {
			return nil, ErrScenarioInvalid
		}
		sum := sha256.Sum256(encoded)
		fingerprints[id] = hex.EncodeToString(sum[:])
	}
	type graphNode struct {
		ID       string         `json:"id"`
		Key      string         `json:"key"`
		Type     string         `json:"type"`
		Version  int            `json:"version"`
		Params   map[string]any `json:"params"`
		Settings NodeSettings   `json:"settings"`
		Parent   string         `json:"parent,omitempty"`
	}
	type graphEdge struct {
		Source PortRef `json:"source"`
		Target PortRef `json:"target"`
	}
	nodes := make([]graphNode, 0, len(doc.Nodes))
	for _, node := range doc.Nodes {
		settings := node.Settings
		settings.Notes = ""
		nodes = append(nodes, graphNode{node.ID, node.Key, node.Type, node.TypeVersion, node.Params, settings, node.Parent})
	}
	edges := make([]graphEdge, 0, len(doc.Edges))
	for _, edge := range doc.Edges {
		edges = append(edges, graphEdge{edge.Source, edge.Target})
	}
	encoded, err := json.Marshal(struct {
		Settings FlowSettings `json:"settings"`
		Nodes    []graphNode  `json:"nodes"`
		Edges    []graphEdge  `json:"edges"`
	}{doc.Settings, nodes, edges})
	if err != nil {
		return nil, ErrScenarioInvalid
	}
	sum := sha256.Sum256(encoded)
	fingerprints["__graph__"] = hex.EncodeToString(sum[:])
	return fingerprints, nil
}

func scenarioContainsSensitive(value any) bool {
	stack := []any{value}
	for len(stack) > 0 {
		item := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch v := item.(type) {
		case string:
			if security.ScrubSecrets(v) != v || security.RedactSensitiveInfo(v) != v {
				return true
			}
		case []any:
			stack = append(stack, v...)
		case map[string]any:
			for key, val := range v {
				lower := strings.ToLower(strings.TrimSpace(key))
				credential := lower == "token" || lower == "authorization" || lower == "cookie" || lower == "password" || lower == "secret" ||
					strings.HasSuffix(lower, "_api_key") || strings.HasSuffix(lower, "_password") || strings.HasSuffix(lower, "_secret") ||
					strings.HasSuffix(lower, "_access_token") || strings.HasSuffix(lower, "_auth_token")
				if security.ScrubSecrets(key) != key || credential && val != nil && val != "" {
					return true
				}
				stack = append(stack, val)
			}
		}
	}
	return false
}

func scenarioNewID() (string, error) {
	var raw [12]byte
	if _, err := io.ReadFull(rand.Reader, raw[:]); err != nil {
		return "", err
	}
	return "sc_" + hex.EncodeToString(raw[:]), nil
}

func scenarioDraft(ctx context.Context, tx *sql.Tx, flowID string) (*Flow, int, error) {
	var raw string
	var revision int
	err := tx.QueryRowContext(ctx, `SELECT draft_json, draft_revision FROM flows WHERE id = ?`, flowID).Scan(&raw, &revision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, 0, ErrNotFound
	}
	if err != nil {
		return nil, 0, err
	}
	doc, err := ParseFlow([]byte(raw))
	return doc, revision, err
}

func scenarioKey(ctx context.Context, keys ScenarioKeyProvider, create bool) ([]byte, error) {
	if keys == nil {
		return nil, ErrScenarioInvalid
	}
	key, err := keys.ScenarioKey(ctx, create)
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		clear(key)
		return nil, ErrScenarioInvalid
	}
	return key, nil
}

// SaveScenario stores a new scenario (empty ID, expected version zero) or replaces the
// exact version a client read. The write lock protects quotas and first-key creation.
func (s *Service) SaveScenario(ctx context.Context, flowID string, baseDraftRevision, expectedVersion int, scenario Scenario, keys ScenarioKeyProvider) (Scenario, error) {
	scenario.Name = strings.TrimSpace(scenario.Name)
	if scenario.Name == "" || utf8.RuneCountInString(scenario.Name) > 80 || baseDraftRevision <= 0 || expectedVersion < 0 {
		return Scenario{}, ErrScenarioInvalid
	}
	creating := scenario.ID == ""
	if creating {
		if expectedVersion != 0 {
			return Scenario{}, ErrScenarioConflict
		}
		id, err := scenarioNewID()
		if err != nil {
			return Scenario{}, err
		}
		scenario.ID = id
	} else if !scenarioIDPattern.MatchString(scenario.ID) || expectedVersion <= 0 {
		return Scenario{}, ErrScenarioInvalid
	}
	encoded, err := json.Marshal(scenario)
	if err != nil || len(encoded) > maxScenarioBytes {
		return Scenario{}, ErrTestDataTooLarge
	}
	var normalized Scenario
	if json.Unmarshal(encoded, &normalized) != nil {
		return Scenario{}, ErrScenarioInvalid
	}
	var raw any
	if json.Unmarshal(encoded, &raw) != nil || scenarioContainsSensitive(raw) || security.ScrubSecrets(string(encoded)) != string(encoded) {
		return Scenario{}, ErrScenarioSensitive
	}
	scenario = normalized
	if scenario.TriggerData == nil {
		scenario.TriggerData = map[string]any{}
	}
	if scenario.At.IsZero() {
		scenario.At = s.now().UTC()
	}
	if err := rejectRedactedTestData(scenario.TriggerData); err != nil {
		return Scenario{}, err
	}
	for _, fixture := range scenario.Fixtures {
		if err := rejectRedactedTestData(fixture.Output); err != nil {
			return Scenario{}, err
		}
	}
	if len(scenario.Assertions) > MaxNodes {
		return Scenario{}, ErrScenarioInvalid
	}
	seenAssertions := make(map[string]bool, len(scenario.Assertions))
	for _, assertion := range scenario.Assertions {
		if assertion.NodeID == "" || seenAssertions[assertion.NodeID] || len(assertion.Output) == 0 && assertion.Ports == nil {
			return Scenario{}, ErrScenarioInvalid
		}
		seenAssertions[assertion.NodeID] = true
		if len(assertion.Output) > 0 {
			var expected any
			if json.Unmarshal(assertion.Output, &expected) != nil {
				return Scenario{}, ErrScenarioInvalid
			}
			if err := rejectRedactedTestData(map[string]any{"value": expected}); err != nil {
				return Scenario{}, err
			}
		}
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return Scenario{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value = value WHERE key = 'schema_version'`); err != nil {
		return Scenario{}, err
	}
	doc, revision, err := scenarioDraft(ctx, tx, flowID)
	if err != nil {
		return Scenario{}, err
	}
	if revision != baseDraftRevision {
		return Scenario{}, ErrRevisionConflict
	}
	reg := NewRegistry()
	if err := RegisterCatalog(reg, nil); err != nil {
		return Scenario{}, err
	}
	trigger := doc.NodeByID(scenario.TriggerNode)
	if trigger == nil || trigger.Settings.Disabled {
		return Scenario{}, ErrNoTrigger
	}
	def, ok := reg.Lookup(trigger.Type)
	if !ok || !def.Trigger {
		return Scenario{}, ErrNoTrigger
	}
	if err := validateSimulationFixtures(doc, reg, scenario.Fixtures); err != nil {
		return Scenario{}, err
	}
	for _, assertion := range scenario.Assertions {
		node := doc.NodeByID(assertion.NodeID)
		if node == nil {
			return Scenario{}, ErrScenarioInvalid
		}
		def, ok := reg.Lookup(node.Type)
		if !ok {
			return Scenario{}, ErrScenarioInvalid
		}
		if assertion.Ports != nil {
			var ports []string
			if catchPanic(func() { ports = def.OutputPorts(node) }) != nil {
				return Scenario{}, ErrScenarioInvalid
			}
			for _, port := range *assertion.Ports {
				if port == PortError || !containsString(ports, port) {
					return Scenario{}, ErrScenarioInvalid
				}
			}
		}
	}
	fingerprints, err := scenarioFingerprints(doc, scenario)
	if err != nil {
		return Scenario{}, err
	}
	var count, allBytes, flowCount int
	if err := tx.QueryRowContext(ctx, `SELECT count(*), COALESCE(sum(length(ciphertext)), 0) FROM flow_test_scenarios`).Scan(&count, &allBytes); err != nil {
		return Scenario{}, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM flow_test_scenarios WHERE flow_id = ?`, flowID).Scan(&flowCount); err != nil {
		return Scenario{}, err
	}
	if creating && flowCount >= maxScenariosPerFlow {
		return Scenario{}, ErrScenarioLimit
	}
	key, err := scenarioKey(ctx, keys, count == 0)
	if err != nil {
		return Scenario{}, err
	}
	defer clear(key)
	var previous []byte
	if !creating {
		err := tx.QueryRowContext(ctx, `SELECT ciphertext FROM flow_test_scenarios WHERE flow_id = ? AND id = ?`, flowID, scenario.ID).Scan(&previous)
		if errors.Is(err, sql.ErrNoRows) {
			return Scenario{}, ErrScenarioNotFound
		}
		if err != nil {
			return Scenario{}, err
		}
		old, err := openScenario(key, flowID, scenario.ID, previous)
		if err != nil {
			return Scenario{}, err
		}
		if old.Scenario.Version != expectedVersion {
			return Scenario{}, ErrScenarioConflict
		}
	}
	scenario.Version = expectedVersion + 1
	scenario.DraftRevision = revision
	scenario.Stale = false
	sealed, err := sealScenario(key, flowID, scenario.ID, scenarioPayload{Scenario: scenario, Fingerprints: fingerprints})
	if err != nil {
		return Scenario{}, err
	}
	if allBytes-len(previous)+len(sealed) > maxScenarioStorageBytes {
		return Scenario{}, ErrScenarioLimit
	}
	if creating {
		_, err = tx.ExecContext(ctx, `INSERT INTO flow_test_scenarios (flow_id,id,ciphertext,updated_at) VALUES (?,?,?,?)`, flowID, scenario.ID, sealed, formatTime(s.now()))
	} else {
		_, err = tx.ExecContext(ctx, `UPDATE flow_test_scenarios SET ciphertext=?, updated_at=? WHERE flow_id=? AND id=?`, sealed, formatTime(s.now()), flowID, scenario.ID)
	}
	if err != nil {
		return Scenario{}, err
	}
	if err := tx.Commit(); err != nil {
		return Scenario{}, err
	}
	return scenario, nil
}

// ListScenarios decrypts a flow's bounded scenarios and marks incompatible fixtures.
func (s *Service) ListScenarios(ctx context.Context, flowID string, keys ScenarioKeyProvider) ([]ScenarioSummary, error) {
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	doc, _, err := scenarioDraft(ctx, tx, flowID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,ciphertext FROM flow_test_scenarios WHERE flow_id=? ORDER BY updated_at DESC,id`, flowID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ScenarioSummary{}
	var key []byte
	defer func() { clear(key) }()
	for rows.Next() {
		if len(list) >= maxScenariosPerFlow {
			return nil, ErrScenarioLimit
		}
		var id string
		var sealed []byte
		if err := rows.Scan(&id, &sealed); err != nil {
			return nil, err
		}
		if !scenarioIDPattern.MatchString(id) {
			return nil, ErrScenarioInvalid
		}
		if key == nil {
			key, err = scenarioKey(ctx, keys, false)
			if err != nil {
				return nil, err
			}
		}
		payload, err := openScenario(key, flowID, id, sealed)
		if err != nil {
			return nil, err
		}
		actual, err := scenarioFingerprints(doc, payload.Scenario)
		payload.Scenario.Stale = err != nil || !equalScenarioFingerprints(actual, payload.Fingerprints)
		item := payload.Scenario
		list = append(list, ScenarioSummary{ID: item.ID, Name: item.Name, Version: item.Version,
			DraftRevision: item.DraftRevision, Stale: item.Stale})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

func equalScenarioFingerprints(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func (s *Service) GetScenario(ctx context.Context, flowID, id string, keys ScenarioKeyProvider) (Scenario, error) {
	scenario, _, err := s.getScenarioAtDraftRevision(ctx, flowID, id, keys)
	return scenario, err
}

// getScenarioAtDraftRevision binds compatibility to the draft snapshot read in
// the same transaction; callers must fence later execution to this revision.
func (s *Service) getScenarioAtDraftRevision(ctx context.Context, flowID, id string, keys ScenarioKeyProvider) (Scenario, int, error) {
	if !scenarioIDPattern.MatchString(id) {
		return Scenario{}, 0, ErrScenarioNotFound
	}
	tx, err := s.store.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Scenario{}, 0, err
	}
	defer tx.Rollback()
	doc, revision, err := scenarioDraft(ctx, tx, flowID)
	if err != nil {
		return Scenario{}, 0, err
	}
	var sealed []byte
	err = tx.QueryRowContext(ctx, `SELECT ciphertext FROM flow_test_scenarios WHERE flow_id=? AND id=?`, flowID, id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return Scenario{}, 0, ErrScenarioNotFound
	}
	if err != nil {
		return Scenario{}, 0, err
	}
	key, err := scenarioKey(ctx, keys, false)
	if err != nil {
		return Scenario{}, 0, err
	}
	defer clear(key)
	payload, err := openScenario(key, flowID, id, sealed)
	if err != nil {
		return Scenario{}, 0, err
	}
	actual, err := scenarioFingerprints(doc, payload.Scenario)
	payload.Scenario.Stale = err != nil || !equalScenarioFingerprints(actual, payload.Fingerprints)
	return payload.Scenario, revision, nil
}

// SimulateScenario runs the explicitly selected, compatible snapshot in the isolated engine.
func (s *Service) SimulateScenario(ctx context.Context, flowID, id string, baseDraftRevision, expectedVersion int, keys ScenarioKeyProvider) (*ScenarioSimulationResult, error) {
	if baseDraftRevision <= 0 || expectedVersion <= 0 {
		return nil, ErrScenarioInvalid
	}
	scenario, observedRevision, err := s.getScenarioAtDraftRevision(ctx, flowID, id, keys)
	if err != nil {
		return nil, err
	}
	return s.simulateLoadedScenario(ctx, flowID, baseDraftRevision, expectedVersion, observedRevision, scenario)
}

func (s *Service) simulateLoadedScenario(ctx context.Context, flowID string, baseDraftRevision, expectedVersion, observedRevision int, scenario Scenario) (*ScenarioSimulationResult, error) {
	if observedRevision != baseDraftRevision {
		return nil, ErrRevisionConflict
	}
	if scenario.Version != expectedVersion {
		return nil, ErrScenarioConflict
	}
	if scenario.Stale {
		return nil, ErrScenarioStale
	}
	result, err := s.Simulate(ctx, flowID, SimulationRequest{TriggerNode: scenario.TriggerNode, TriggerData: scenario.TriggerData,
		OnlyNode: scenario.OnlyNode, BaseRevision: baseDraftRevision, Fixtures: scenario.Fixtures, At: scenario.At})
	if err != nil {
		return nil, err
	}
	checks := make([]ScenarioAssertionResult, 0, len(scenario.Assertions))
	for _, assertion := range scenario.Assertions {
		check := ScenarioAssertionResult{NodeID: assertion.NodeID}
		var step *StepRecord
		for i := range result.Steps {
			if result.Steps[i].NodeID == assertion.NodeID {
				step = &result.Steps[i]
			}
		}
		switch {
		case step == nil || step.Status != StepSuccess:
			check.Code = "step_unavailable"
		case step.OutputTruncated:
			check.Code = "output_truncated"
		default:
			if len(assertion.Output) > 0 {
				var expected, actual any
				encoded, err := json.Marshal(step.Output)
				if err != nil || json.Unmarshal(encoded, &actual) != nil || json.Unmarshal(assertion.Output, &expected) != nil || !reflect.DeepEqual(expected, actual) {
					check.Code = "output_mismatch"
				}
			}
			if check.Code == "" && assertion.Ports != nil && !slices.Equal(*assertion.Ports, step.Ports) {
				check.Code = "ports_mismatch"
			}
			check.Passed = check.Code == ""
		}
		checks = append(checks, check)
	}
	return &ScenarioSimulationResult{SimulationResult: result, Assertions: checks}, nil
}

func (s *Service) DeleteScenario(ctx context.Context, flowID, id string, expectedVersion int, keys ScenarioKeyProvider) error {
	if !scenarioIDPattern.MatchString(id) || expectedVersion <= 0 {
		return ErrScenarioInvalid
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		return err
	}
	if _, _, err := scenarioDraft(ctx, tx, flowID); err != nil {
		return err
	}
	var sealed []byte
	err = tx.QueryRowContext(ctx, `SELECT ciphertext FROM flow_test_scenarios WHERE flow_id=? AND id=?`, flowID, id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrScenarioNotFound
	}
	if err != nil {
		return err
	}
	key, err := scenarioKey(ctx, keys, false)
	if err != nil {
		return err
	}
	defer clear(key)
	payload, err := openScenario(key, flowID, id, sealed)
	if err != nil {
		return err
	}
	if payload.Scenario.Version != expectedVersion {
		return ErrScenarioConflict
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM flow_test_scenarios WHERE flow_id=? AND id=?`, flowID, id); err != nil {
		return err
	}
	return tx.Commit()
}

// RenameScenario changes only the name, preserving the encrypted input snapshot.
func (s *Service) RenameScenario(ctx context.Context, flowID, id string, expectedVersion int, name string, keys ScenarioKeyProvider) (ScenarioSummary, error) {
	name = strings.TrimSpace(name)
	if !scenarioIDPattern.MatchString(id) || expectedVersion <= 0 || name == "" || utf8.RuneCountInString(name) > 80 {
		return ScenarioSummary{}, ErrScenarioInvalid
	}
	if scenarioContainsSensitive(name) {
		return ScenarioSummary{}, ErrScenarioSensitive
	}
	tx, err := s.store.db.BeginTx(ctx, nil)
	if err != nil {
		return ScenarioSummary{}, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE flows_meta SET value=value WHERE key='schema_version'`); err != nil {
		return ScenarioSummary{}, err
	}
	doc, _, err := scenarioDraft(ctx, tx, flowID)
	if err != nil {
		return ScenarioSummary{}, err
	}
	var sealed []byte
	err = tx.QueryRowContext(ctx, `SELECT ciphertext FROM flow_test_scenarios WHERE flow_id=? AND id=?`, flowID, id).Scan(&sealed)
	if errors.Is(err, sql.ErrNoRows) {
		return ScenarioSummary{}, ErrScenarioNotFound
	}
	if err != nil {
		return ScenarioSummary{}, err
	}
	key, err := scenarioKey(ctx, keys, false)
	if err != nil {
		return ScenarioSummary{}, err
	}
	defer clear(key)
	payload, err := openScenario(key, flowID, id, sealed)
	if err != nil {
		return ScenarioSummary{}, err
	}
	if payload.Scenario.Version != expectedVersion {
		return ScenarioSummary{}, ErrScenarioConflict
	}
	payload.Scenario.Name = name
	payload.Scenario.Version++
	payload.Scenario.Stale = false
	replacement, err := sealScenario(key, flowID, id, payload)
	if err != nil {
		return ScenarioSummary{}, err
	}
	var total int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(sum(length(ciphertext)),0) FROM flow_test_scenarios`).Scan(&total); err != nil {
		return ScenarioSummary{}, err
	}
	if total-len(sealed)+len(replacement) > maxScenarioStorageBytes {
		return ScenarioSummary{}, ErrScenarioLimit
	}
	if _, err := tx.ExecContext(ctx, `UPDATE flow_test_scenarios SET ciphertext=?,updated_at=? WHERE flow_id=? AND id=?`, replacement, formatTime(s.now()), flowID, id); err != nil {
		return ScenarioSummary{}, err
	}
	if err := tx.Commit(); err != nil {
		return ScenarioSummary{}, err
	}
	actual, err := scenarioFingerprints(doc, payload.Scenario)
	item := payload.Scenario
	return ScenarioSummary{ID: id, Name: name, Version: item.Version, DraftRevision: item.DraftRevision,
		Stale: err != nil || !equalScenarioFingerprints(actual, payload.Fingerprints)}, nil
}
