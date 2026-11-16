package storeapps

import (
	"context"
	"errors"
	"net/url"
	"sort"
)

type NodeREDInject struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}
type NodeREDFlow struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Injects []NodeREDInject `json:"injects"`
}

func (c Connection) NodeREDProbe(ctx context.Context) error {
	var auth struct {
		Type string `json:"type"`
	}
	if err := c.request(ctx, "GET", "/auth/login", nil, false, &auth); err != nil {
		return err
	}
	if auth.Type == "" {
		return errors.New("Enable Node-RED admin authentication before connecting an access token")
	}
	_, err := c.NodeREDFlows(ctx)
	return err
}

func (c Connection) NodeREDFlows(ctx context.Context) ([]NodeREDFlow, error) {
	var nodes []struct {
		ID, Type, Label, Name, Z string
		Disabled                 bool `json:"disabled"`
		D                        bool `json:"d"`
	}
	if err := c.request(ctx, "GET", "/flows", nil, false, &nodes); err != nil {
		return nil, err
	}
	flows := map[string]*NodeREDFlow{}
	for _, node := range nodes {
		if node.Type == "tab" && !node.Disabled {
			flows[node.ID] = &NodeREDFlow{ID: node.ID, Name: node.Label, Injects: []NodeREDInject{}}
		}
	}
	for _, node := range nodes {
		if node.Type != "inject" || node.D || !c.Allowed(node.ID) {
			continue
		}
		if flow := flows[node.Z]; flow != nil {
			flow.Injects = append(flow.Injects, NodeREDInject{ID: node.ID, Name: node.Name})
		}
	}
	result := make([]NodeREDFlow, 0, len(flows))
	for _, flow := range flows {
		sort.Slice(flow.Injects, func(i, j int) bool { return flow.Injects[i].ID < flow.Injects[j].ID })
		result = append(result, *flow)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result, nil
}

func (c Connection) NodeREDTrigger(ctx context.Context, id string) (any, error) {
	if !c.Allowed(id) {
		return nil, errors.New("Inject node is not administratively allowed")
	}
	flows, err := c.NodeREDFlows(ctx)
	if err != nil {
		return nil, err
	}
	found := false
	for _, flow := range flows {
		for _, inject := range flow.Injects {
			if inject.ID == id {
				found = true
			}
		}
	}
	if !found {
		return nil, errors.New("Selected Inject node is missing or disabled")
	}
	if err := c.request(ctx, "POST", "/inject/"+url.PathEscape(id), nil, true, nil); err != nil {
		return nil, err
	}
	return map[string]any{"status": "accepted", "node_id": id, "completed": false}, nil
}
