package planner

import (
	"fmt"
	"strings"
)

const PlannerKGSource = "planner"

// KGSyncTracker records planner-managed nodes and edges during a batch sync pass.
type KGSyncTracker struct {
	ActivePlannerNodes map[string]struct{}
	ExpectedEdges      map[string]struct{}
}

func NewKGSyncTracker() *KGSyncTracker {
	return &KGSyncTracker{
		ActivePlannerNodes: make(map[string]struct{}),
		ExpectedEdges:      make(map[string]struct{}),
	}
}

func PlannerContactKGNodeID(contactID string) string {
	return "contact_" + strings.TrimSpace(contactID)
}

func PlannerTodoItemKGNodeID(todoKGNodeID, itemID string) string {
	return todoKGNodeID + "_item_" + strings.TrimSpace(itemID)
}

func plannerKGEdgeKey(source, target, relation string) string {
	return source + "\x00" + target + "\x00" + relation
}

func plannerKGEdgeProps() map[string]string {
	return map[string]string{"source": PlannerKGSource}
}

func (t *KGSyncTracker) markPlannerNode(id string) {
	id = strings.TrimSpace(id)
	if id == "" {
		return
	}
	t.ActivePlannerNodes[id] = struct{}{}
}

func (t *KGSyncTracker) expectEdge(source, target, relation string) {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	relation = strings.TrimSpace(relation)
	if source == "" || target == "" || relation == "" {
		return
	}
	t.ExpectedEdges[plannerKGEdgeKey(source, target, relation)] = struct{}{}
}

func appointmentKGProps(a Appointment) map[string]string {
	props := map[string]string{
		"type":   "event",
		"source": PlannerKGSource,
		"date":   a.DateTime,
		"status": a.Status,
	}
	if a.Description != "" {
		props["description"] = a.Description
	}
	return props
}

func todoKGProps(t Todo) map[string]string {
	props := map[string]string{
		"type":     "task",
		"source":   PlannerKGSource,
		"priority": t.Priority,
		"status":   t.Status,
		"progress": fmt.Sprintf("%d", t.ProgressPercent),
	}
	if t.DueDate != "" {
		props["due_date"] = t.DueDate
	}
	if t.Description != "" {
		props["description"] = t.Description
	}
	if t.RemindDaily {
		props["remind_daily"] = "true"
	}
	if t.ItemCount > 0 {
		props["item_count"] = fmt.Sprintf("%d", t.ItemCount)
		props["done_item_count"] = fmt.Sprintf("%d", t.DoneItemCount)
	}
	return props
}

// SyncAppointmentKGRecord syncs one appointment node and its planner involves edges.
func SyncAppointmentKGRecord(kg KnowledgeGraph, a Appointment, contactIDs []string, tracker *KGSyncTracker) error {
	if isNilKnowledgeGraph(kg) {
		return nil
	}
	if a.Status == "cancelled" {
		return nil
	}
	if a.KGNodeID == "" {
		a.KGNodeID = "appointment_" + a.ID
	}

	if err := kg.AddNode(a.KGNodeID, a.Title, appointmentKGProps(a)); err != nil {
		return fmt.Errorf("sync appointment node %s: %w", a.KGNodeID, err)
	}
	if tracker != nil {
		tracker.markPlannerNode(a.KGNodeID)
	}

	keepTargets := make(map[string]struct{}, len(contactIDs))
	edgeProps := plannerKGEdgeProps()
	for _, contactID := range contactIDs {
		contactID = strings.TrimSpace(contactID)
		if contactID == "" {
			continue
		}
		targetID := PlannerContactKGNodeID(contactID)
		keepTargets[targetID] = struct{}{}
		if err := kg.AddEdge(a.KGNodeID, targetID, "involves", edgeProps); err != nil {
			return fmt.Errorf("sync appointment involves edge %s->%s: %w", a.KGNodeID, targetID, err)
		}
		if tracker != nil {
			tracker.expectEdge(a.KGNodeID, targetID, "involves")
		}
	}

	if _, err := kg.PrunePlannerEdges(a.KGNodeID, "involves", keepTargets); err != nil {
		return fmt.Errorf("prune appointment involves edges for %s: %w", a.KGNodeID, err)
	}
	return nil
}

// SyncTodoKGRecord syncs one todo node and optional checklist item subgraph.
func SyncTodoKGRecord(kg KnowledgeGraph, t Todo, tracker *KGSyncTracker) error {
	if isNilKnowledgeGraph(kg) {
		return nil
	}
	if t.KGNodeID == "" {
		t.KGNodeID = "todo_" + t.ID
	}

	if err := kg.AddNode(t.KGNodeID, t.Title, todoKGProps(t)); err != nil {
		return fmt.Errorf("sync todo node %s: %w", t.KGNodeID, err)
	}
	if tracker != nil {
		tracker.markPlannerNode(t.KGNodeID)
	}

	if len(t.Items) == 0 {
		if _, err := kg.PrunePlannerNodesByPrefix(PlannerTodoItemKGNodeID(t.KGNodeID, ""), nil); err != nil {
			return fmt.Errorf("prune todo item nodes for %s: %w", t.KGNodeID, err)
		}
		return nil
	}

	keepItemNodes := make(map[string]struct{}, len(t.Items))
	edgeProps := plannerKGEdgeProps()
	for _, item := range t.Items {
		if strings.TrimSpace(item.ID) == "" {
			continue
		}
		itemNodeID := PlannerTodoItemKGNodeID(t.KGNodeID, item.ID)
		keepItemNodes[itemNodeID] = struct{}{}
		itemProps := map[string]string{
			"type":   "task_item",
			"source": PlannerKGSource,
		}
		if item.IsDone {
			itemProps["status"] = "done"
		} else {
			itemProps["status"] = "open"
		}
		if item.Description != "" {
			itemProps["description"] = item.Description
		}
		label := strings.TrimSpace(item.Title)
		if label == "" {
			label = "Checklist item"
		}
		if err := kg.AddNode(itemNodeID, label, itemProps); err != nil {
			return fmt.Errorf("sync todo item node %s: %w", itemNodeID, err)
		}
		if err := kg.AddEdge(itemNodeID, t.KGNodeID, "part_of", edgeProps); err != nil {
			return fmt.Errorf("sync todo part_of edge %s->%s: %w", itemNodeID, t.KGNodeID, err)
		}
		if tracker != nil {
			tracker.markPlannerNode(itemNodeID)
			tracker.expectEdge(itemNodeID, t.KGNodeID, "part_of")
		}
	}

	if _, err := kg.PrunePlannerNodesByPrefix(PlannerTodoItemKGNodeID(t.KGNodeID, ""), keepItemNodes); err != nil {
		return fmt.Errorf("prune todo item nodes for %s: %w", t.KGNodeID, err)
	}
	return nil
}