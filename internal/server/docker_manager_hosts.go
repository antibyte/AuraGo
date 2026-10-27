package server

import (
	"net/http"
	"strings"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerhost/transport"
)

// dmHostTestTimeout bounds a connection test; Registry.Test and TestDraft limit
// themselves to 30 s.
const dmHostTestTimeout = 35 * time.Second

// dockerHostView is the HostView of the API contract: the stored host plus
// derived flags. Secrets never appear, only whether one is set.
type dockerHostView struct {
	dockerhost.Host
	Builtin   bool                  `json:"builtin"`
	SecretSet bool                  `json:"secret_set"`
	Plaintext bool                  `json:"plaintext"`
	Status    dockerhost.HostStatus `json:"status"`
}

func (dm *dmRequest) hostView(host dockerhost.Host, status dockerhost.HostStatus) dockerHostView {
	return dockerHostView{
		Host:      host,
		Builtin:   host.ID == dockerhost.LocalHostID,
		SecretSet: dm.registry().SecretSet(host.ID),
		Plaintext: host.Kind == dockerhost.KindTCP,
		Status:    status,
	}
}

func (dm *dmRequest) hostViews(hosts []dockerhost.Host) []dockerHostView {
	views := make([]dockerHostView, 0, len(hosts))
	for _, host := range hosts {
		views = append(views, dm.hostView(host, dm.registry().Status(host.ID)))
	}
	return views
}

// dockerHostWrite is the HostWrite body of POST and PUT /hosts and of the draft
// test. The secret members are write-only; the registry keeps stored values for
// absent or blank ones. ID is read by the draft test only, to reuse the secrets
// of the host that is being edited.
type dockerHostWrite struct {
	ID             string                 `json:"id"`
	Name           string                 `json:"name"`
	Kind           string                 `json:"kind"`
	Address        string                 `json:"address"`
	Port           int                    `json:"port"`
	StacksDir      string                 `json:"stacks_dir"`
	HostSocketPath string                 `json:"host_socket_path"`
	Color          string                 `json:"color"`
	SSHUser        string                 `json:"ssh_user"`
	Sort           *int                   `json:"sort"`
	SSHSecret      *string                `json:"ssh_secret"`
	TLS            *transport.TLSMaterial `json:"tls"`
	ClearSecret    bool                   `json:"clear_secret"`
}

func (in dockerHostWrite) host() dockerhost.Host {
	host := dockerhost.Host{
		Name:           in.Name,
		Kind:           in.Kind,
		Address:        in.Address,
		Port:           in.Port,
		StacksDir:      in.StacksDir,
		HostSocketPath: in.HostSocketPath,
		Color:          in.Color,
		SSHUser:        in.SSHUser,
	}
	if in.Sort != nil {
		host.Sort = *in.Sort
	}
	return host
}

func (in dockerHostWrite) secrets() dockerhost.SecretInput {
	return dockerhost.SecretInput{SSHSecret: in.SSHSecret, TLS: in.TLS, Clear: in.ClearSecret}
}

// hostCollection serves GET and POST /hosts.
func (dm *dmRequest) hostCollection() {
	switch dm.r.Method {
	case http.MethodGet:
		dm.hostList()
	case http.MethodPost:
		dm.hostCreate()
	default:
		dm.allow(http.MethodGet, http.MethodPost)
	}
}

// hostList answers with the cached status of every host and starts a
// background test for stale ones, so the next poll shows fresh dots. It never
// waits for an unreachable host. The answer also carries docker_readonly.
func (dm *dmRequest) hostList() {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	hosts, err := dm.registry().Hosts(ctx)
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dm.s.dockerStatus.refreshStale(dm.registry(), hosts, time.Now())
	// docker_readonly is the switch the container mutations check (dm.readOnly, read per request), so the UI hides
	// start/stop/remove before the first refusal and shows them again once the admin lifts it.
	dmWriteOK(dm.w, http.StatusOK, dmFields{"hosts": dm.hostViews(hosts), "docker_readonly": dm.readOnly})
}

// hostCreate stores a host. The registry normalises and validates it (empty
// paths get the defaults, an empty color the next palette entry) and checks the
// secrets before anything is written.
func (dm *dmRequest) hostCreate() {
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	created, err := dm.registry().Create(ctx, in.host(), in.secrets())
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusCreated, dmFields{"host": dm.hostView(created, dm.registry().Status(created.ID))})
}

// hostItem serves PUT and DELETE /hosts/{id}.
func (dm *dmRequest) hostItem(id string) {
	switch dm.r.Method {
	case http.MethodPut:
		dm.hostUpdate(id)
	case http.MethodDelete:
		dm.hostDelete(id)
	default:
		dm.allow(http.MethodPut, http.MethodDelete)
	}
}

// hostUpdate replaces a host. Empty paths keep the stored values (the registry
// would reset them to the defaults), an absent sort keeps the stored one.
func (dm *dmRequest) hostUpdate(id string) {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	current, ok := dm.host(ctx, id)
	if !ok {
		return
	}
	if current.ID == dockerhost.LocalHostID {
		dm.hostFailure(dockerhost.ErrLocalHostFixed)
		return
	}
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	host := in.host()
	if strings.TrimSpace(host.StacksDir) == "" {
		host.StacksDir = current.StacksDir
	}
	if strings.TrimSpace(host.HostSocketPath) == "" {
		host.HostSocketPath = current.HostSocketPath
	}
	if in.Sort == nil {
		host.Sort = current.Sort
	}
	updated, err := dm.registry().Update(ctx, current.ID, host, in.secrets())
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host": dm.hostView(updated, dm.registry().Status(updated.ID))})
}

func (dm *dmRequest) hostDelete(id string) {
	ctx, cancel := dm.context(dmRegistryTimeout)
	defer cancel()
	if err := dm.registry().Delete(ctx, id); err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, nil)
}

// hostTest serves POST /hosts/{id}/test: it connects, caches the result and
// answers with the refreshed host. An unreachable host is still a 200; the
// failure is the test's result. The button is a deliberate attempt, so it lifts
// the pause that follows a rejected SSH login (Registry.TestNow); the background
// refresh keeps respecting it.
func (dm *dmRequest) hostTest(id string) {
	if !dm.allow(http.MethodPost) {
		return
	}
	ctx, cancel := dm.context(dmHostTestTimeout)
	defer cancel()
	host, ok := dm.host(ctx, id)
	if !ok {
		return
	}
	status := dm.registry().TestNow(ctx, host.ID)
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host": dm.hostView(host, status)})
}

// hostDraftTest serves POST /hosts/test: it tests an unsaved draft. A draft
// that names a stored host by id takes the secrets it leaves out from that
// host's vault entries (done by TestDraft), so "Test connection" works while
// editing without retyping a key. A draft that cannot be tested (an invalid
// field, or SSH/TLS credentials that are missing or may not be reused) is a 400
// invalid_host with the field, like a save; only a real connection outcome is a
// 200 host_status.
func (dm *dmRequest) hostDraftTest() {
	if !dm.allow(http.MethodPost) {
		return
	}
	var in dockerHostWrite
	if !dm.decode(&in) {
		return
	}
	ctx, cancel := dm.context(dmHostTestTimeout)
	defer cancel()
	host := in.host()
	host.ID = strings.TrimSpace(in.ID)
	status, err := dm.registry().TestDraft(ctx, host, in.secrets())
	if err != nil {
		dm.hostFailure(err)
		return
	}
	dmWriteOK(dm.w, http.StatusOK, dmFields{"host_status": status})
}
