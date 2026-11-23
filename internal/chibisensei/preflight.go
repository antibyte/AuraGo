package chibisensei

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	"aurago/internal/config"
)

// Preflight levels.
const (
	PreflightBlock = "block"
	PreflightWarn  = "warn"
)

const preflightKeyPrefix = "chibi_sensei.preflight."

// KaggleCredentialsLink is the Config UI anchor of the Kaggle credentials.
const KaggleCredentialsLink = "/config#kaggle"

// PreflightInput holds the plain values the server collects. EvaluatePreflight does no I/O.
type PreflightInput struct {
	// Data (store and check result).
	DataRev            int64  // the project's current data_rev
	CheckPresent       bool   // a check result exists
	CheckDataRev       int64  // data_rev of that result
	CheckReady         bool   // that result is ready
	ActiveRunID        string // the installation's non-terminal run; "" = none
	ActiveRunProjectID string

	// Kaggle configuration (kaggleSnapshot().Config).
	KaggleEnabled          bool
	KaggleReadOnly         bool
	AllowCreate            bool
	AllowRemoteExecute     bool
	AllowGPU               bool
	AllowInternet          bool
	AllowDownload          bool
	AllowDelete            bool
	AllowedMachineShapes   []string
	RemoteTimeoutMinutes   int
	MaxTransferMB          int
	MaxStorageMB           int
	KaggleStorageUsedBytes int64 // Service.Status()["storage_bytes"]

	// Chibi training configuration.
	MachineShape         string // chibi_sensei.training.machine_shape
	DeleteRemoteAfterRun bool   // chibi_sensei.training.delete_remote_after_run

	// Access (kaggleSnapshot().Access).
	Network bool
	Write   bool

	// Account.
	VaultAvailable    bool
	CredentialsStored bool
	ConnectionTested  bool // the server called Service.Test (it does so only when KaggleReachable is true)
	ConnectionOK      bool
	Account           string // the account Service.Test returned

	// Quota and compute slots.
	QuotaRead   bool    // the server read get_accelerator_quota_statistics
	QuotaKnown  bool    // ParseGPUQuota could tell the remaining hours
	QuotaHours  float64 // remaining GPU hours
	JobsRead    bool    // KaggleDriver.ActiveComputeJobs answered
	ComputeJobs int     // active compute jobs, unknown included
	UnknownJobs int     // of those, the jobs in status unknown

	// Plan and profile.
	EstimateMinutes      int // plan.estimate_minutes
	ArtifactsEstimatedMB int // plan.artifacts_estimated_mb
	LargestArtifactMB    int // profile.GGUF.EstimatedMB (the largest single download)

	// Local storage.
	DiskFreeKnown    bool
	DiskFreeBytes    int64 // free bytes on the volume of <data>/chibi_sensei
	StorageUsedBytes int64 // StorageBytes(dataDir) plus Store.SizeBytes()
	StorageMaxGB     int   // chibi_sensei.storage.max_gb

	// License.
	ConsentRequired bool
	ConsentRecorded bool // consent for this exact license version exists
	LicenseID       string
	LicenseVersion  string

	Admin bool // reported by the server next to the items; not used by EvaluatePreflight
}

// PreflightItem is one row of the preflight list.
type PreflightItem struct {
	ID         string         `json:"id"`
	Group      string         `json:"group"` // kaggle | account | quota | data | license | storage
	OK         bool           `json:"ok"`
	Level      string         `json:"level"` // block | warn
	MessageKey string         `json:"message_key"`
	Args       map[string]any `json:"args,omitempty"`
	Flag       string         `json:"flag,omitempty"`
	Link       string         `json:"link,omitempty"`
}

// Preflight is the evaluated list; Ready is true when every block item is OK.
type Preflight struct {
	Ready bool            `json:"ready"`
	Items []PreflightItem `json:"items"`
}

// PreflightFlags are the flag names POST /preflight/enable accepts.
var PreflightFlags = []string{"kaggle.enabled", "kaggle.read_only", "kaggle.allow_create", "kaggle.allow_remote_execute",
	"kaggle.allow_gpu", "kaggle.allow_internet", "kaggle.allow_download", "kaggle.allowed_machine_shapes", "kaggle.allow_delete"}

// ErrUnknownPreflightFlag refuses a flag outside PreflightFlags (the route answers 422).
var ErrUnknownPreflightFlag = errors.New("chibisensei: unknown preflight flag")

func preflightItem(id, group, level string, ok bool, flag string, args map[string]any) PreflightItem {
	return PreflightItem{ID: id, Group: group, OK: ok, Level: level, MessageKey: preflightKeyPrefix + id, Args: args, Flag: flag}
}

// preflightBase builds the items dataset_ready … kaggle_credentials.
func preflightBase(in PreflightInput) []PreflightItem {
	var active map[string]any
	if in.ActiveRunID != "" {
		active = map[string]any{"run_id": in.ActiveRunID, "project_id": in.ActiveRunProjectID}
	}
	cred := preflightItem("kaggle_credentials", "account", PreflightBlock, in.CredentialsStored, "", nil)
	cred.Link = KaggleCredentialsLink
	return []PreflightItem{
		preflightItem("dataset_ready", "data", PreflightBlock,
			in.CheckPresent && in.CheckDataRev == in.DataRev && in.CheckReady, "", nil),
		preflightItem("run_active", "data", PreflightBlock, in.ActiveRunID == "", "", active),
		preflightItem("kaggle_enabled", "kaggle", PreflightBlock, in.KaggleEnabled, "kaggle.enabled", nil),
		preflightItem("kaggle_not_readonly", "kaggle", PreflightBlock, !in.KaggleReadOnly, "kaggle.read_only", nil),
		preflightItem("kaggle_allow_create", "kaggle", PreflightBlock, in.AllowCreate, "kaggle.allow_create", nil),
		preflightItem("kaggle_allow_remote_execute", "kaggle", PreflightBlock, in.AllowRemoteExecute, "kaggle.allow_remote_execute", nil),
		preflightItem("kaggle_allow_gpu", "kaggle", PreflightBlock, in.AllowGPU, "kaggle.allow_gpu", nil),
		preflightItem("kaggle_allow_internet", "kaggle", PreflightBlock, in.AllowInternet, "kaggle.allow_internet", nil),
		preflightItem("kaggle_allow_download", "kaggle", PreflightBlock, in.AllowDownload, "kaggle.allow_download", nil),
		preflightItem("kaggle_machine_shape", "kaggle", PreflightBlock,
			in.MachineShape != "" && slices.Contains(in.AllowedMachineShapes, in.MachineShape),
			"kaggle.allowed_machine_shapes", map[string]any{"shape": in.MachineShape}),
		preflightItem("kaggle_allow_delete", "kaggle", PreflightWarn, in.AllowDelete || !in.DeleteRemoteAfterRun,
			"kaggle.allow_delete", nil),
		preflightItem("network_access", "account", PreflightBlock, in.Network, "", nil),
		preflightItem("filesystem_write", "account", PreflightBlock, in.Write, "", nil),
		preflightItem("kaggle_vault", "account", PreflightBlock, in.VaultAvailable, "", nil),
		cred,
	}
}

func reachable(items []PreflightItem) bool {
	for _, it := range items {
		if (it.Group == "kaggle" || it.Group == "account") && it.Level == PreflightBlock && !it.OK {
			return false
		}
	}
	return true
}

// KaggleReachable reports whether every blocking Kaggle and account item before kaggle_connection passes. The
// server calls Service.Test and reads the quota only then.
func KaggleReachable(in PreflightInput) bool { return reachable(preflightBase(in)) }

func round1(x float64) float64 { return math.Round(x*10) / 10 }

// EvaluatePreflight evaluates the contract's item table in its order.
func EvaluatePreflight(in PreflightInput) Preflight {
	items := preflightBase(in)
	canReach := reachable(items)

	conn := preflightItem("kaggle_connection", "account", PreflightBlock, canReach && in.ConnectionTested && in.ConnectionOK, "", nil)
	switch {
	case !canReach || !in.ConnectionTested:
		conn.Args = map[string]any{"skipped": true}
	case in.Account != "":
		conn.Args = map[string]any{"account": in.Account}
	}
	items = append(items, conn)

	needed := int(math.Ceil(1.5*float64(in.EstimateMinutes) + 15))
	items = append(items, preflightItem("kaggle_timeout", "quota", PreflightWarn, in.RemoteTimeoutMinutes >= needed, "",
		map[string]any{"minutes": in.RemoteTimeoutMinutes, "needed": needed}))

	var quota PreflightItem
	switch {
	case !in.QuotaRead:
		quota = preflightItem("kaggle_quota", "quota", PreflightWarn, false, "", map[string]any{"skipped": true})
	case !in.QuotaKnown:
		quota = preflightItem("kaggle_quota", "quota", PreflightWarn, false, "", map[string]any{"unknown": true})
	default:
		need := 1.2 * float64(in.EstimateMinutes) / 60
		quota = preflightItem("kaggle_quota", "quota", PreflightBlock, in.QuotaHours >= need, "",
			map[string]any{"hours": round1(in.QuotaHours), "needed": round1(need)})
	}
	items = append(items, quota)

	if !in.JobsRead {
		items = append(items, preflightItem("kaggle_slot", "quota", PreflightBlock, false, "", map[string]any{"skipped": true}))
	} else {
		items = append(items, preflightItem("kaggle_slot", "quota", PreflightBlock, in.ComputeJobs == 0, "",
			map[string]any{"jobs": in.ComputeJobs, "unknown": in.UnknownJobs}))
	}

	largest := int64(max(in.LargestArtifactMB, 0))
	kaggleFreeMB := max((int64(in.MaxStorageMB)<<20-in.KaggleStorageUsedBytes)>>20, 0)
	items = append(items, preflightItem("kaggle_transfer", "storage", PreflightBlock,
		int64(in.MaxTransferMB) >= largest && kaggleFreeMB >= largest, "",
		map[string]any{"needed_mb": largest, "max_transfer_mb": in.MaxTransferMB, "free_mb": kaggleFreeMB}))

	neededMB := int64(math.Ceil(1.2 * float64(max(in.ArtifactsEstimatedMB, 0))))
	capFree := max(int64(in.StorageMaxGB)<<30-in.StorageUsedBytes, 0)
	freeMB := int64(-1)
	if in.DiskFreeKnown {
		freeMB = in.DiskFreeBytes >> 20
	}
	items = append(items, preflightItem("disk_space", "storage", PreflightBlock,
		in.DiskFreeKnown && in.DiskFreeBytes >= neededMB<<20 && capFree >= neededMB<<20, "",
		map[string]any{"needed_mb": neededMB, "free_mb": freeMB, "cap_free_mb": capFree >> 20}))

	items = append(items, preflightItem("license_consent", "license", PreflightBlock,
		!in.ConsentRequired || in.ConsentRecorded, "",
		map[string]any{"license_id": in.LicenseID, "license_version": in.LicenseVersion}))

	ready := true
	for _, it := range items {
		if it.Level == PreflightBlock && !it.OK {
			ready = false
		}
	}
	return Preflight{Ready: ready, Items: items}
}

// FixFlags lists the flags of the failing items in item order without repeats; nil when none (B5 sends fix: null).
func (p Preflight) FixFlags() []string {
	var out []string
	for _, it := range p.Items {
		if !it.OK && it.Flag != "" && !slices.Contains(out, it.Flag) {
			out = append(out, it.Flag)
		}
	}
	return out
}

// PreflightFlagPatch maps flag names to one config patch shaped like config.yaml, for applyConfigPatch:
// kaggle.enabled → true, kaggle.read_only → false, kaggle.allow_* → true, kaggle.allowed_machine_shapes → the
// current list plus machineShape (deduplicated, current order kept, as []any). A flag outside PreflightFlags, an
// empty list, or an unknown machine shape is an error wrapping ErrUnknownPreflightFlag.
func PreflightFlagPatch(flags, currentShapes []string, machineShape string) (map[string]any, error) {
	if len(flags) == 0 {
		return nil, fmt.Errorf("%w: no flags", ErrUnknownPreflightFlag)
	}
	kaggle := map[string]any{}
	for _, f := range flags {
		switch f {
		case "kaggle.enabled":
			kaggle["enabled"] = true
		case "kaggle.read_only":
			kaggle["read_only"] = false
		case "kaggle.allow_create", "kaggle.allow_remote_execute", "kaggle.allow_gpu", "kaggle.allow_internet",
			"kaggle.allow_download", "kaggle.allow_delete":
			kaggle[strings.TrimPrefix(f, "kaggle.")] = true
		case "kaggle.allowed_machine_shapes":
			if !slices.Contains(config.KaggleMachineShapes, machineShape) {
				return nil, fmt.Errorf("%w: machine shape %q is not a known name", ErrUnknownPreflightFlag, machineShape)
			}
			list := []any{}
			seen := map[string]bool{}
			for _, s := range append(append([]string(nil), currentShapes...), machineShape) {
				s = strings.TrimSpace(s)
				if s != "" && !seen[s] {
					seen[s] = true
					list = append(list, s)
				}
			}
			kaggle["allowed_machine_shapes"] = list
		default:
			return nil, fmt.Errorf("%w: %s", ErrUnknownPreflightFlag, f)
		}
	}
	return map[string]any{"kaggle": kaggle}, nil
}
