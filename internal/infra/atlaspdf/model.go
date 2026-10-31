package atlaspdf

import (
	"errors"
	"sort"
	"strings"

	"aurago/internal/infra"
)

// ErrTooLarge is returned when the snapshot exceeds the renderable bounds.
var ErrTooLarge = errors.New("atlaspdf: snapshot too large to render")

const (
	maxNodes = 5000
	maxEdges = 10000
)

// model is the sanitised graph the renderer works on: unique nodes, edges with
// both ends present, valid parents and the guest-to-host folding.
type model struct {
	nodes    []infra.Node
	byID     map[string]int
	edges    []infra.Edge
	parent   map[string]string
	hostOf   map[string]string
	guestsOf map[string][]string
}

var hostedKinds = map[infra.NodeKind]bool{infra.NodeVM: true, infra.NodeContainer: true, infra.NodeService: true}

func isPlace(k infra.NodeKind) bool {
	return k == infra.NodeSite || k == infra.NodeRoom || k == infra.NodeRack || k == infra.NodeGroup
}

func newModel(s infra.Snapshot) (*model, error) {
	if len(s.Nodes) > maxNodes || len(s.Edges) > maxEdges {
		return nil, ErrTooLarge
	}
	m := &model{byID: map[string]int{}, parent: map[string]string{}, hostOf: map[string]string{}, guestsOf: map[string][]string{}}
	for _, n := range s.Nodes {
		if n.ID == "" {
			continue
		}
		if _, dup := m.byID[n.ID]; dup {
			continue
		}
		m.byID[n.ID] = len(m.nodes)
		m.nodes = append(m.nodes, n)
	}
	for _, e := range s.Edges {
		_, okFrom := m.byID[e.From]
		_, okTo := m.byID[e.To]
		if okFrom && okTo && e.From != e.To {
			m.edges = append(m.edges, e)
		}
	}
	for _, n := range m.nodes {
		if n.ParentID != "" && n.ParentID != n.ID {
			if _, ok := m.byID[n.ParentID]; ok {
				m.parent[n.ID] = n.ParentID
			}
		}
	}
	m.foldGuests()
	return m, nil
}

func (m *model) node(id string) *infra.Node {
	if i, ok := m.byID[id]; ok {
		return &m.nodes[i]
	}
	return nil
}

func (m *model) name(id string) string {
	if n := m.node(id); n != nil {
		return n.Name
	}
	return ""
}

// foldGuests ports guestIndex: a vm, container or service belongs to its
// host_id or else the smallest runs_on target. Hosting cycles are broken in
// favour of the smaller id.
func (m *model) foldGuests() {
	runsOn := map[string][]string{}
	for _, e := range m.edges {
		if e.Kind == infra.EdgeRunsOn {
			runsOn[e.From] = append(runsOn[e.From], e.To)
		}
	}
	for _, n := range m.nodes {
		if !hostedKinds[n.Kind] {
			continue
		}
		host := ""
		if _, ok := m.byID[n.HostID]; ok && n.HostID != n.ID {
			host = n.HostID
		} else if targets := runsOn[n.ID]; len(targets) > 0 {
			sort.Strings(targets)
			host = targets[0]
		}
		if host != "" {
			m.hostOf[n.ID] = host
		}
	}
	ids := make([]string, 0, len(m.hostOf))
	for id := range m.hostOf {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		for {
			cycle := m.hostCycle(id)
			if len(cycle) == 0 {
				break
			}
			sort.Strings(cycle)
			delete(m.hostOf, cycle[len(cycle)-1])
		}
	}
	for guest, host := range m.hostOf {
		m.guestsOf[host] = append(m.guestsOf[host], guest)
	}
	for host := range m.guestsOf {
		sort.Strings(m.guestsOf[host])
	}
}

// hostCycle returns the members of the hosting cycle reachable from id, if any.
func (m *model) hostCycle(id string) []string {
	seen := map[string]int{}
	var path []string
	for cur := id; cur != ""; cur = m.hostOf[cur] {
		if at, ok := seen[cur]; ok {
			return append([]string(nil), path[at:]...)
		}
		seen[cur] = len(path)
		path = append(path, cur)
	}
	return nil
}

// rootHost follows hostOf to the first node that is not itself a guest.
func (m *model) rootHost(id string) string {
	seen := map[string]bool{}
	for {
		host, ok := m.hostOf[id]
		if !ok || seen[id] {
			return id
		}
		seen[id] = true
		id = host
	}
}

// allGuests lists every direct and nested guest of host, ordered by kind then name.
func (m *model) allGuests(host string) []string {
	var out []string
	seen := map[string]bool{host: true}
	stack := []string{host}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, g := range m.guestsOf[cur] {
			if !seen[g] {
				seen[g] = true
				out = append(out, g)
				stack = append(stack, g)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return m.lessAsset(out[i], out[j]) })
	return out
}

var kindOrder = map[infra.NodeKind]int{
	infra.NodeNetwork: 0, infra.NodeDevice: 1, infra.NodeVM: 2, infra.NodeContainer: 3, infra.NodeService: 4,
	infra.NodeSite: 5, infra.NodeRoom: 6, infra.NodeRack: 7, infra.NodeGroup: 8,
}

func (m *model) lessAsset(a, b string) bool {
	na, nb := m.node(a), m.node(b)
	if ka, kb := guestOrder(na.Kind), guestOrder(nb.Kind); ka != kb {
		return ka < kb
	}
	if la, lb := strings.ToLower(na.Name), strings.ToLower(nb.Name); la != lb {
		return la < lb
	}
	return a < b
}

func guestOrder(k infra.NodeKind) int {
	switch k {
	case infra.NodeVM:
		return 0
	case infra.NodeContainer:
		return 1
	case infra.NodeService:
		return 2
	}
	return 3
}

// placePath is the names of the place chain above id, outermost first.
func (m *model) placePath(id string) []string {
	var path []string
	seen := map[string]bool{id: true}
	for p := m.parent[id]; p != "" && !seen[p]; p = m.parent[p] {
		seen[p] = true
		if n := m.node(p); n != nil {
			path = append([]string{n.Name}, path...)
		}
	}
	return path
}

// connections lists the edge indexes shown as connections: every edge except a
// runs_on that only repeats the guest nesting.
func (m *model) connections() []int {
	var out []int
	for i, e := range m.edges {
		if e.Kind == infra.EdgeRunsOn && m.hostOf[e.From] == e.To {
			continue
		}
		out = append(out, i)
	}
	return out
}

// assets counts the nodes that are not places.
func (m *model) assets() int {
	n := 0
	for _, node := range m.nodes {
		if !isPlace(node.Kind) {
			n++
		}
	}
	return n
}

// corpus is all node and edge text that will be printed, used to pick fonts.
func (m *model) corpus() string {
	var b strings.Builder
	add := func(s ...string) {
		for _, v := range s {
			b.WriteString(v)
			b.WriteByte(' ')
		}
	}
	for _, n := range m.nodes {
		p := n.Profile
		add(n.Name, p.Role, p.Vendor, p.Model, p.OS, p.CPU, p.RAM, p.RackPosition)
	}
	for _, e := range m.edges {
		add(e.Label, e.FromPort, e.ToPort, e.Speed)
	}
	return b.String()
}

// sanitise rewrites every printed string through clean and the monospaced
// fields through mono.
func (m *model) sanitise(clean, mono func(string) string) {
	for i := range m.nodes {
		n := &m.nodes[i]
		n.Name = clean(n.Name)
		p := &n.Profile
		p.Role, p.Vendor, p.Model, p.OS = clean(p.Role), clean(p.Vendor), clean(p.Model), clean(p.OS)
		p.CPU, p.RAM, p.RackPosition = clean(p.CPU), clean(p.RAM), clean(p.RackPosition)
		p.IPAddress, p.CIDR, p.MACAddress = mono(p.IPAddress), mono(p.CIDR), mono(p.MACAddress)
		if n.Name == "" {
			n.Name = "?"
		}
	}
	for i := range m.edges {
		e := &m.edges[i]
		e.Label, e.FromPort, e.ToPort, e.Speed = clean(e.Label), clean(e.FromPort), clean(e.ToPort), clean(e.Speed)
	}
}
