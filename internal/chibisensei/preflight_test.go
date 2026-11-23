package chibisensei

import (
	"errors"
	"reflect"
	"sort"
	"testing"

	"aurago/internal/kaggle"
)

func okPreflightInput() PreflightInput {
	return PreflightInput{DataRev: 3, CheckPresent: true, CheckDataRev: 3, CheckReady: true,
		KaggleEnabled: true, AllowCreate: true, AllowRemoteExecute: true, AllowGPU: true, AllowInternet: true,
		AllowDownload: true, AllowDelete: true, AllowedMachineShapes: []string{"NvidiaTeslaT4"}, RemoteTimeoutMinutes: 120,
		MaxTransferMB: 2048, MaxStorageMB: 10240, MachineShape: "NvidiaTeslaT4", DeleteRemoteAfterRun: true,
		Network: true, Write: true, VaultAvailable: true, CredentialsStored: true, ConnectionTested: true,
		ConnectionOK: true, Account: "tester", QuotaRead: true, QuotaKnown: true, QuotaHours: 27.5, JobsRead: true,
		EstimateMinutes: 34, ArtifactsEstimatedMB: 650, LargestArtifactMB: 300, DiskFreeKnown: true,
		DiskFreeBytes: 50 << 30, StorageMaxGB: 20, ConsentRequired: true, ConsentRecorded: true,
		LicenseID: "gemma", LicenseVersion: "1.0"}
}

var preflightOrder = []string{"dataset_ready", "run_active", "kaggle_enabled", "kaggle_not_readonly", "kaggle_allow_create",
	"kaggle_allow_remote_execute", "kaggle_allow_gpu", "kaggle_allow_internet", "kaggle_allow_download",
	"kaggle_machine_shape", "kaggle_allow_delete", "network_access", "filesystem_write", "kaggle_vault",
	"kaggle_credentials", "kaggle_connection", "kaggle_timeout", "kaggle_quota", "kaggle_slot", "kaggle_transfer",
	"disk_space", "license_consent"}

func failing(p Preflight) []string {
	var out []string
	for _, it := range p.Items {
		if !it.OK {
			out = append(out, it.ID)
		}
	}
	sort.Strings(out)
	return out
}

func TestEvaluatePreflightAllOKAndOrder(t *testing.T) {
	p := EvaluatePreflight(okPreflightInput())
	if !p.Ready || len(failing(p)) != 0 {
		t.Fatalf("not ready: %v", failing(p))
	}
	var ids []string
	for _, it := range p.Items {
		ids = append(ids, it.ID)
		if it.MessageKey != "chibi_sensei.preflight."+it.ID {
			t.Errorf("%s key %s", it.ID, it.MessageKey)
		}
	}
	if !reflect.DeepEqual(ids, preflightOrder) {
		t.Fatalf("order = %v", ids)
	}
	if p.FixFlags() != nil {
		t.Fatalf("fix flags on a ready preflight: %v", p.FixFlags())
	}
}

func TestEvaluatePreflightCases(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*PreflightInput)
		fail  []string
		ready bool
	}{
		{"no check", func(in *PreflightInput) { in.CheckPresent = false }, []string{"dataset_ready"}, false},
		{"stale check", func(in *PreflightInput) { in.CheckDataRev = 2 }, []string{"dataset_ready"}, false},
		{"kaggle off", func(in *PreflightInput) { in.KaggleEnabled = false; in.ConnectionTested = false },
			[]string{"kaggle_connection", "kaggle_enabled"}, false},
		{"delete not granted", func(in *PreflightInput) { in.AllowDelete = false }, []string{"kaggle_allow_delete"}, true},
		{"quota unknown", func(in *PreflightInput) { in.QuotaKnown = false }, []string{"kaggle_quota"}, true},
		{"timeout short", func(in *PreflightInput) { in.RemoteTimeoutMinutes = 60 }, []string{"kaggle_timeout"}, true},
		{"active run", func(in *PreflightInput) { in.ActiveRunID = "r" }, []string{"run_active"}, false},
		{"read only", func(in *PreflightInput) { in.KaggleReadOnly = true }, []string{"kaggle_not_readonly", "kaggle_connection"}, false},
		{"no create", func(in *PreflightInput) { in.AllowCreate = false }, []string{"kaggle_allow_create", "kaggle_connection"}, false},
		{"no remote execute", func(in *PreflightInput) { in.AllowRemoteExecute = false }, []string{"kaggle_allow_remote_execute", "kaggle_connection"}, false},
		{"no gpu", func(in *PreflightInput) { in.AllowGPU = false }, []string{"kaggle_allow_gpu", "kaggle_connection"}, false},
		{"no internet", func(in *PreflightInput) { in.AllowInternet = false }, []string{"kaggle_allow_internet", "kaggle_connection"}, false},
		{"no download", func(in *PreflightInput) { in.AllowDownload = false }, []string{"kaggle_allow_download", "kaggle_connection"}, false},
		{"no shape", func(in *PreflightInput) { in.AllowedMachineShapes = nil }, []string{"kaggle_machine_shape", "kaggle_connection"}, false},
		{"delete not needed", func(in *PreflightInput) { in.AllowDelete = false; in.DeleteRemoteAfterRun = false }, nil, true},
		{"no network", func(in *PreflightInput) { in.Network = false }, []string{"network_access", "kaggle_connection"}, false},
		{"no write", func(in *PreflightInput) { in.Write = false }, []string{"filesystem_write", "kaggle_connection"}, false},
		{"no vault", func(in *PreflightInput) { in.VaultAvailable = false }, []string{"kaggle_vault", "kaggle_connection"}, false},
		{"no credentials", func(in *PreflightInput) { in.CredentialsStored = false }, []string{"kaggle_credentials", "kaggle_connection"}, false},
		{"connection failed", func(in *PreflightInput) { in.ConnectionOK = false }, []string{"kaggle_connection"}, false},
		{"quota low", func(in *PreflightInput) { in.QuotaHours = 0.5 }, []string{"kaggle_quota"}, false},
		{"quota not read", func(in *PreflightInput) { in.QuotaRead = false }, []string{"kaggle_quota"}, true},
		{"slot busy", func(in *PreflightInput) { in.ComputeJobs = 1; in.UnknownJobs = 1 }, []string{"kaggle_slot"}, false},
		{"jobs not read", func(in *PreflightInput) { in.JobsRead = false }, []string{"kaggle_slot"}, false},
		{"transfer cap", func(in *PreflightInput) { in.MaxTransferMB = 100 }, []string{"kaggle_transfer"}, false},
		{"kaggle storage full", func(in *PreflightInput) { in.KaggleStorageUsedBytes = 10240 << 20 }, []string{"kaggle_transfer"}, false},
		{"disk small", func(in *PreflightInput) { in.DiskFreeBytes = 100 << 20 }, []string{"disk_space"}, false},
		{"storage cap", func(in *PreflightInput) { in.StorageUsedBytes = 20 << 30 }, []string{"disk_space"}, false},
		{"disk unknown", func(in *PreflightInput) { in.DiskFreeKnown = false }, []string{"disk_space"}, false},
		{"no consent", func(in *PreflightInput) { in.ConsentRecorded = false }, []string{"license_consent"}, false},
		{"consent not required", func(in *PreflightInput) { in.ConsentRequired = false; in.ConsentRecorded = false }, nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := okPreflightInput()
			c.edit(&in)
			p := EvaluatePreflight(in)
			want := append([]string(nil), c.fail...)
			sort.Strings(want)
			if got := failing(p); !reflect.DeepEqual(got, want) || p.Ready != c.ready {
				t.Fatalf("failing %v ready %v, want %v %v", got, p.Ready, want, c.ready)
			}
		})
	}
}

func TestPreflightArgsAndFix(t *testing.T) {
	in := okPreflightInput()
	in.KaggleEnabled, in.AllowGPU, in.ConnectionTested = false, false, false
	p := EvaluatePreflight(in)
	if got := p.FixFlags(); !reflect.DeepEqual(got, []string{"kaggle.enabled", "kaggle.allow_gpu"}) {
		t.Fatalf("fix flags = %v", got)
	}
	if KaggleReachable(in) || !KaggleReachable(okPreflightInput()) {
		t.Fatal("KaggleReachable")
	}
	item := func(p Preflight, id string) PreflightItem {
		for _, it := range p.Items {
			if it.ID == id {
				return it
			}
		}
		t.Fatalf("no item %s", id)
		return PreflightItem{}
	}
	if got := item(p, "kaggle_connection").Args; !reflect.DeepEqual(got, map[string]any{"skipped": true}) {
		t.Fatalf("connection args = %v", got)
	}
	ok := EvaluatePreflight(okPreflightInput())
	if item(ok, "kaggle_credentials").Link != "/config#kaggle" {
		t.Fatal("credentials link")
	}
	if got := item(ok, "kaggle_timeout").Args; !reflect.DeepEqual(got, map[string]any{"minutes": 120, "needed": 66}) {
		t.Fatalf("timeout args = %v", got)
	}
	if got := item(ok, "kaggle_quota").Args; !reflect.DeepEqual(got, map[string]any{"hours": 27.5, "needed": 0.7}) {
		t.Fatalf("quota args = %v", got)
	}
}

func TestPreflightFlagPatch(t *testing.T) {
	got, err := PreflightFlagPatch([]string{"kaggle.enabled", "kaggle.read_only", "kaggle.allow_gpu", "kaggle.allowed_machine_shapes"},
		[]string{"NvidiaL4"}, "NvidiaTeslaT4")
	want := map[string]any{"kaggle": map[string]any{"enabled": true, "read_only": false, "allow_gpu": true,
		"allowed_machine_shapes": []any{"NvidiaL4", "NvidiaTeslaT4"}}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("patch = %v, %v", got, err)
	}
	got, err = PreflightFlagPatch([]string{"kaggle.allowed_machine_shapes"}, []string{"NvidiaTeslaT4", "NvidiaL4"}, "NvidiaTeslaT4")
	if err != nil || !reflect.DeepEqual(got["kaggle"].(map[string]any)["allowed_machine_shapes"], []any{"NvidiaTeslaT4", "NvidiaL4"}) {
		t.Fatalf("duplicate shape: %v, %v", got, err)
	}
	for name, args := range map[string]struct {
		flags []string
		shape string
	}{
		"unknown flag": {[]string{"kaggle.allow_tpu"}, "NvidiaTeslaT4"},
		"empty":        {nil, "NvidiaTeslaT4"},
		"bad shape":    {[]string{"kaggle.allowed_machine_shapes"}, "X"},
	} {
		if _, err := PreflightFlagPatch(args.flags, nil, args.shape); !errors.Is(err, ErrUnknownPreflightFlag) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	for _, f := range PreflightFlags {
		if _, err := PreflightFlagPatch([]string{f}, nil, "NvidiaTeslaT4"); err != nil {
			t.Errorf("flag %s: %v", f, err)
		}
	}
}

// The fixtures are PROVISIONAL until the proof run records a real answer.
func TestParseGPUQuota(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want float64
		ok   bool
	}{
		{"remaining hours", `{"gpu":{"remainingHours":12.5,"totalHours":30}}`, 12.5, true},
		{"duration seconds", `{"gpuQuotaRemaining":99000}`, 27.5, true},
		{"overused", `{"quotas":[{"type":"GPU","limitSeconds":108000,"usedSeconds":120000}]}`, 0, true},
		{"string seconds", `{"accelerators":[{"name":"NvidiaTeslaT4","remainingSeconds":"36000"}]}`, 10, true},
		{"minutes", `{"gpu":{"remainingMinutes":90}}`, 1.5, true},
		{"tpu only", `{"tpu":{"remaining":20}}`, 0, false},
		{"empty", `{}`, 0, false},
		{"not json", `not json`, 0, false},
		{"smallest wins", `{"quotas":[{"type":"GPU","remainingHours":5},{"type":"GPU","remainingHours":9}]}`, 5, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := ParseGPUQuota([]byte(c.in))
			if ok != c.ok || got != c.want {
				t.Fatalf("got %v %v, want %v %v", got, ok, c.want, c.ok)
			}
		})
	}
}

func TestNewKaggleRequestOperations(t *testing.T) {
	grants := map[string]string{"upload_dataset_file": "create", "create_dataset": "create", "save_kernel": "update",
		"download_kernel_output": "download", "delete_dataset": "delete", "delete_kernel": "delete"}
	for op, domain := range kaggleOpDomains {
		o, err := kaggle.Lookup(domain, op)
		if err != nil {
			t.Fatalf("%s/%s: %v", domain, op, err)
		}
		want := grants[op]
		if want == "" {
			want = "read"
		}
		if o.Grant != want {
			t.Errorf("%s grant = %q, want %q", op, o.Grant, want)
		}
	}
	if _, err := NewKaggleRequest("", "cancel_kernel_session", "", "", nil); err == nil {
		t.Fatal("an operation Chibi does not use was accepted")
	}
	id := KaggleRequestID("r", "s", 0)
	if len(id) != 32 || id == KaggleRequestID("r", "s", 1) {
		t.Fatalf("request id %q", id)
	}
}
