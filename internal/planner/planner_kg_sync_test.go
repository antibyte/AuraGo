package planner

import (
	"testing"
)

func mockKGHasEdge(kg *mockKG, source, target, relation string) bool {
	for _, edge := range kg.edges {
		if edge.source == source && edge.target == target && edge.relation == relation {
			return true
		}
	}
	return false
}

func TestSyncAppointmentKGRecordInvolvesEdges(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	apptID, _ := CreateAppointment(db, Appointment{
		Title:    "Team sync",
		DateTime: "2026-07-01T10:00:00Z",
	})
	if err := SetAppointmentContacts(db, apptID, []string{"contact-a", "contact-b"}); err != nil {
		t.Fatalf("SetAppointmentContacts: %v", err)
	}

	appt, err := GetAppointment(db, apptID)
	if err != nil {
		t.Fatalf("GetAppointment: %v", err)
	}
	contactIDs, err := GetAppointmentContactIDs(db, apptID)
	if err != nil {
		t.Fatalf("GetAppointmentContactIDs: %v", err)
	}

	kg := newMockKG()
	if err := SyncAppointmentKGRecord(kg, *appt, contactIDs, nil); err != nil {
		t.Fatalf("SyncAppointmentKGRecord: %v", err)
	}

	nodeID := "appointment_" + apptID
	if !mockKGHasEdge(kg, nodeID, "contact_contact-a", "involves") {
		t.Fatal("expected involves edge to contact-a")
	}
	if !mockKGHasEdge(kg, nodeID, "contact_contact-b", "involves") {
		t.Fatal("expected involves edge to contact-b")
	}
	for _, edge := range kg.edges {
		if edge.relation != "involves" {
			continue
		}
		if edge.props["source"] != PlannerKGSource {
			t.Fatalf("expected planner source on involves edge, got %#v", edge.props)
		}
	}
}

func TestSyncAppointmentKGRecordPrunesStaleInvolves(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	apptID, _ := CreateAppointment(db, Appointment{
		Title:    "Contact swap",
		DateTime: "2026-07-02T10:00:00Z",
	})
	if err := SetAppointmentContacts(db, apptID, []string{"keep-contact"}); err != nil {
		t.Fatalf("SetAppointmentContacts initial: %v", err)
	}

	appt, _ := GetAppointment(db, apptID)
	kg := newMockKG()
	contactIDs, _ := GetAppointmentContactIDs(db, apptID)
	if err := SyncAppointmentKGRecord(kg, *appt, contactIDs, nil); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	// Simulate a stale planner edge left from a previous sync.
	kg.edges = append(kg.edges, mockEdge{
		source:   "appointment_" + apptID,
		target:   "contact_stale-contact",
		relation: "involves",
		props:    plannerKGEdgeProps(),
	})

	if err := SetAppointmentContacts(db, apptID, []string{"keep-contact"}); err != nil {
		t.Fatalf("SetAppointmentContacts refresh: %v", err)
	}
	appt, _ = GetAppointment(db, apptID)
	contactIDs, _ = GetAppointmentContactIDs(db, apptID)
	if err := SyncAppointmentKGRecord(kg, *appt, contactIDs, nil); err != nil {
		t.Fatalf("resync: %v", err)
	}

	if mockKGHasEdge(kg, "appointment_"+apptID, "contact_stale-contact", "involves") {
		t.Fatal("expected stale involves edge pruned")
	}
	if !mockKGHasEdge(kg, "appointment_"+apptID, "contact_keep-contact", "involves") {
		t.Fatal("expected current involves edge kept")
	}
}

func TestSyncTodoKGRecordPartOfWorkspaceWhenNoItems(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	todoID, _ := CreateTodo(db, Todo{
		Title:    "Standalone task",
		Priority: "medium",
	})

	todo, err := GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}

	kg := newMockKG()
	tracker := NewKGSyncTracker()
	if err := SyncTodoKGRecord(kg, *todo, tracker); err != nil {
		t.Fatalf("SyncTodoKGRecord: %v", err)
	}

	if _, ok := tracker.ActivePlannerNodes[PlannerWorkspaceKGNodeID]; !ok {
		t.Fatal("expected planner workspace hub tracked for itemless todo")
	}
	if _, ok := tracker.ExpectedEdges[plannerKGEdgeKey("todo_"+todoID, PlannerWorkspaceKGNodeID, "part_of")]; !ok {
		t.Fatal("expected workspace part_of edge tracked")
	}

	hub, ok := kg.nodes[PlannerWorkspaceKGNodeID]
	if !ok {
		t.Fatal("expected planner workspace hub node")
	}
	if hub.props["type"] != "planner_hub" {
		t.Fatalf("expected planner_hub type on workspace node, got %q", hub.props["type"])
	}

	todoNodeID := "todo_" + todoID
	if !mockKGHasEdge(kg, todoNodeID, PlannerWorkspaceKGNodeID, "part_of") {
		t.Fatalf("expected part_of edge from %s to %s", todoNodeID, PlannerWorkspaceKGNodeID)
	}
}

func TestSyncTodoKGRecordRemovesWorkspaceLinkWhenItemsAdded(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	todoID, _ := CreateTodo(db, Todo{
		Title: "Growing checklist",
	})

	todo, err := GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}

	kg := newMockKG()
	if err := SyncTodoKGRecord(kg, *todo, nil); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	todoNodeID := "todo_" + todoID
	if !mockKGHasEdge(kg, todoNodeID, PlannerWorkspaceKGNodeID, "part_of") {
		t.Fatal("expected workspace part_of edge before items were added")
	}

	itemID, err := AddTodoItem(db, todoID, TodoItem{Title: "First step"})
	if err != nil {
		t.Fatalf("AddTodoItem: %v", err)
	}
	todo, err = GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("GetTodo after item add: %v", err)
	}

	if err := SyncTodoKGRecord(kg, *todo, nil); err != nil {
		t.Fatalf("resync with items: %v", err)
	}
	if mockKGHasEdge(kg, todoNodeID, PlannerWorkspaceKGNodeID, "part_of") {
		t.Fatal("expected workspace part_of edge removed after checklist items were added")
	}
	itemNodeID := PlannerTodoItemKGNodeID(todoNodeID, itemID)
	if !mockKGHasEdge(kg, itemNodeID, todoNodeID, "part_of") {
		t.Fatalf("expected checklist part_of edge from %s to %s", itemNodeID, todoNodeID)
	}
}

func TestSyncTodoKGRecordPartOfItems(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	todoID, _ := CreateTodo(db, Todo{
		Title:    "Launch checklist",
		Priority: "high",
		Items: []TodoItem{
			{Title: "Write docs"},
			{Title: "Ship build", IsDone: true},
		},
	})

	todo, err := GetTodo(db, todoID)
	if err != nil {
		t.Fatalf("GetTodo: %v", err)
	}
	if len(todo.Items) != 2 {
		t.Fatalf("expected 2 todo items, got %d", len(todo.Items))
	}

	kg := newMockKG()
	if err := SyncTodoKGRecord(kg, *todo, nil); err != nil {
		t.Fatalf("SyncTodoKGRecord: %v", err)
	}

	todoNodeID := "todo_" + todoID
	for _, item := range todo.Items {
		itemNodeID := PlannerTodoItemKGNodeID(todoNodeID, item.ID)
		node, ok := kg.nodes[itemNodeID]
		if !ok {
			t.Fatalf("expected item node %s", itemNodeID)
		}
		if node.props["type"] != "task_item" {
			t.Fatalf("expected task_item type on %s, got %q", itemNodeID, node.props["type"])
		}
		if !mockKGHasEdge(kg, itemNodeID, todoNodeID, "part_of") {
			t.Fatalf("expected part_of edge from %s to %s", itemNodeID, todoNodeID)
		}
	}
	if mockKGHasEdge(kg, todoNodeID, PlannerWorkspaceKGNodeID, "part_of") {
		t.Fatal("expected no workspace part_of edge when checklist items exist")
	}
}

func TestSyncTodoKGRecordPrunesRemovedItems(t *testing.T) {
	db := testDB(t)
	defer db.Close()

	todoID, _ := CreateTodo(db, Todo{
		Title: "Shrinking checklist",
		Items: []TodoItem{
			{Title: "Keep me"},
			{Title: "Remove me"},
		},
	})
	todo, _ := GetTodo(db, todoID)
	if len(todo.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(todo.Items))
	}
	removedItemID := todo.Items[1].ID
	keptItemID := todo.Items[0].ID

	kg := newMockKG()
	if err := SyncTodoKGRecord(kg, *todo, nil); err != nil {
		t.Fatalf("initial sync: %v", err)
	}

	todo.Items = todo.Items[:1]
	if err := SyncTodoKGRecord(kg, *todo, nil); err != nil {
		t.Fatalf("resync: %v", err)
	}

	todoNodeID := "todo_" + todoID
	removedNodeID := PlannerTodoItemKGNodeID(todoNodeID, removedItemID)
	keptNodeID := PlannerTodoItemKGNodeID(todoNodeID, keptItemID)

	if _, ok := kg.nodes[removedNodeID]; ok {
		t.Fatal("expected removed item node pruned")
	}
	if _, ok := kg.nodes[keptNodeID]; !ok {
		t.Fatal("expected kept item node to remain")
	}
	if mockKGHasEdge(kg, removedNodeID, todoNodeID, "part_of") {
		t.Fatal("expected stale part_of edge removed with node")
	}
}

func TestKGSyncTrackerRecordsNodesAndEdges(t *testing.T) {
	tracker := NewKGSyncTracker()
	a := Appointment{ID: "appt-1", Title: "Tracked", DateTime: "2026-08-01T10:00:00Z"}
	todo := Todo{
		ID:    "todo-1",
		Title: "Tracked todo",
		Items: []TodoItem{{ID: "item-1", Title: "Step"}},
	}
	kg := newMockKG()

	if err := SyncAppointmentKGRecord(kg, a, []string{"c1"}, tracker); err != nil {
		t.Fatalf("SyncAppointmentKGRecord: %v", err)
	}
	if err := SyncTodoKGRecord(kg, todo, tracker); err != nil {
		t.Fatalf("SyncTodoKGRecord: %v", err)
	}

	if _, ok := tracker.ActivePlannerNodes["appointment_appt-1"]; !ok {
		t.Fatal("expected appointment node tracked")
	}
	if _, ok := tracker.ActivePlannerNodes["todo_todo-1"]; !ok {
		t.Fatal("expected todo node tracked")
	}
	if _, ok := tracker.ActivePlannerNodes["todo_todo-1_item_item-1"]; !ok {
		t.Fatal("expected todo item node tracked")
	}

	if _, ok := tracker.ExpectedEdges[plannerKGEdgeKey("appointment_appt-1", "contact_c1", "involves")]; !ok {
		t.Fatal("expected involves edge tracked")
	}
	if _, ok := tracker.ExpectedEdges[plannerKGEdgeKey("todo_todo-1_item_item-1", "todo_todo-1", "part_of")]; !ok {
		t.Fatal("expected part_of edge tracked")
	}
}