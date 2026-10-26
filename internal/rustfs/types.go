package rustfs

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"aurago/internal/config"
	"aurago/internal/dockerutil"
)

const Image = "docker.io/rustfs/rustfs:1.0.4@sha256:ed9210d6411d77ffceb70d94b964a3cb7d225983368bc25bebe01e995293dcc2"
const Region = "us-east-1"
const stateKey = "rustfs_state"

type Action string

const (
	ActionInstall  Action = "install"
	ActionStart    Action = "start"
	ActionStop     Action = "stop"
	ActionRecreate Action = "recreate"
	ActionRemove   Action = "remove"
)

type Status struct {
	Enabled          bool      `json:"enabled"`
	Installed        bool      `json:"installed"`
	Provisioned      bool      `json:"provisioned"`
	Running          bool      `json:"running"`
	Ready            bool      `json:"ready"`
	AgentAccess      bool      `json:"agent_access"`
	State            string    `json:"state"`
	AccessState      string    `json:"access_state"`
	Reason           string    `json:"reason,omitempty"`
	Endpoint         string    `json:"endpoint,omitempty"`
	Bucket           string    `json:"bucket"`
	Image            string    `json:"image"`
	ContainerID      string    `json:"container_id,omitempty"`
	DataVolume       string    `json:"data_volume"`
	CredentialVolume string    `json:"credential_volume"`
	OperationID      string    `json:"operation_id,omitempty"`
	Generation       string    `json:"generation"`
	Updated          time.Time `json:"updated"`
}
type Preflight struct {
	DockerAvailable  bool   `json:"docker_available"`
	Supported        bool   `json:"supported"`
	MutationAllowed  bool   `json:"mutation_allowed"`
	Installed        bool   `json:"installed"`
	DataRetained     bool   `json:"data_retained"`
	Placement        string `json:"placement"`
	Endpoint         string `json:"endpoint,omitempty"`
	DataVolume       string `json:"data_volume"`
	CredentialVolume string `json:"credential_volume"`
	Reason           string `json:"reason,omitempty"`
}
type Operation struct {
	ID      string    `json:"id"`
	Action  Action    `json:"action"`
	State   string    `json:"state"`
	Phase   string    `json:"phase"`
	Error   string    `json:"error,omitempty"`
	Created time.Time `json:"created"`
	Updated time.Time `json:"updated"`
}
type codedError struct {
	code  string
	cause error
}

func (e *codedError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.code, e.cause)
	}
	return e.code
}
func (e *codedError) Unwrap() error          { return e.cause }
func failure(code string, cause error) error { return &codedError{code, cause} }
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}
	var e *codedError
	if errors.As(err, &e) {
		return e.code
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	return "rustfs_failed"
}

type secretStore interface {
	ReadSecret(string) (string, error)
	WriteSecret(string, string) error
}
type dockerEngine interface {
	DoJSON(context.Context, string, string, any, any) (int, error)
	HTTPClient() *http.Client
	Host() string
	CloseIdleConnections()
}
type resourceNames struct{ container, seed, data, credentials string }
type persistedState struct {
	InitialBucket      string `json:"initial_bucket"`
	Instance           string `json:"instance"`
	EngineID           string `json:"engine_id"`
	ContainerID        string `json:"container_id,omitempty"`
	DataCreated        string `json:"data_created,omitempty"`
	CredentialsCreated string `json:"credentials_created,omitempty"`
	Seeded             bool   `json:"seeded"`
	DesiredRunning     bool   `json:"desired_running"`
	Endpoint           string `json:"endpoint,omitempty"`
	Port               string `json:"port,omitempty"`
	RootAccess         string `json:"root_access"`
	RootSecret         string `json:"root_secret"`
	ParentAccess       string `json:"parent_access"`
	ParentSecret       string `json:"parent_secret"`
	ChildAccess        string `json:"child_access"`
	ChildSecret        string `json:"child_secret"`
	PolicyName         string `json:"policy_name"`
	Provisioned        bool   `json:"provisioned"`
	PolicyHash         string `json:"policy_hash,omitempty"`
	IAMPending         bool   `json:"iam_pending,omitempty"`
	PendingPolicyHash  string `json:"pending_policy_hash,omitempty"`
}
type job struct {
	generation uint64
	Operation
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

// A single runtime serializes mutations; receipts are intentionally process-local.
type managerState struct {
	mu            sync.Mutex
	cfg           *config.Config
	desired       context.Context
	desiredCancel context.CancelFunc
	generation    uint64
	fingerprint   string
	state         persistedState
	stateErr      error
	status        Status
	jobs          map[string]*job
	active        *job
}

func defaultNames() resourceNames {
	return resourceNames{dockerutil.RustFSContainerName, dockerutil.RustFSSeedContainerName, dockerutil.RustFSDataVolumeName, dockerutil.RustFSCredentialVolumeName}
}
