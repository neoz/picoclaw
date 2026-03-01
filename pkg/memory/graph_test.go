package memory

import (
	"testing"
	"time"
)

func openTestDB(t *testing.T) *MemoryDB {
	t.Helper()
	db, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestUpsertEntity(t *testing.T) {
	db := openTestDB(t)

	id1, err := db.UpsertEntity("Alice", "person")
	if err != nil {
		t.Fatal(err)
	}
	if id1 == 0 {
		t.Fatal("expected non-zero entity ID")
	}

	// Same name should return same ID
	id2, err := db.UpsertEntity("Alice", "person")
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Fatalf("upsert should return same ID: got %d and %d", id1, id2)
	}
}

func TestUpsertEntityDefaultType(t *testing.T) {
	db := openTestDB(t)

	id, err := db.UpsertEntity("Something", "")
	if err != nil {
		t.Fatal(err)
	}

	e, err := db.getEntityByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != "thing" {
		t.Fatalf("expected default type 'thing', got %q", e.Type)
	}
}

func TestUpsertEntityTypePreservation(t *testing.T) {
	db := openTestDB(t)

	// First insert with specific type
	db.UpsertEntity("Alice", "person")

	// Upsert with "thing" should NOT overwrite "person"
	id, err := db.UpsertEntity("Alice", "thing")
	if err != nil {
		t.Fatal(err)
	}
	e, err := db.getEntityByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != "person" {
		t.Fatalf("expected preserved type 'person', got %q", e.Type)
	}

	// Upsert with a different specific type SHOULD overwrite
	id, err = db.UpsertEntity("Alice", "concept")
	if err != nil {
		t.Fatal(err)
	}
	e, err = db.getEntityByID(id)
	if err != nil {
		t.Fatal(err)
	}
	if e.Type != "concept" {
		t.Fatalf("expected updated type 'concept', got %q", e.Type)
	}
}

func TestAddRelation(t *testing.T) {
	db := openTestDB(t)

	err := db.AddRelation("Alice", "works_on", "PicoClaw", "team_alice")
	if err != nil {
		t.Fatal(err)
	}

	// Verify entities were auto-created
	entities, err := db.FindEntities([]string{"Alice", "PicoClaw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities, got %d", len(entities))
	}
}

func TestAddRelationUpsert(t *testing.T) {
	db := openTestDB(t)

	// Add relation with one memory key
	err := db.AddRelation("Alice", "works_on", "PicoClaw", "key1")
	if err != nil {
		t.Fatal(err)
	}

	// Same triple with different key should update, not duplicate
	err = db.AddRelation("Alice", "works_on", "PicoClaw", "key2")
	if err != nil {
		t.Fatal(err)
	}

	// Check there's only one relation
	aliceID, _ := db.UpsertEntity("Alice", "")
	rels, err := db.getRelationsForEntity(aliceID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation after upsert, got %d", len(rels))
	}
	if rels[0].MemoryKey != "key2" {
		t.Fatalf("expected updated memory_key 'key2', got %q", rels[0].MemoryKey)
	}
}

func TestRemoveRelationsByMemoryKey(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("Alice", "works_on", "PicoClaw", "team_info")
	db.AddRelation("Alice", "knows", "Bob", "team_info")
	db.AddRelation("Bob", "lives_in", "Tokyo", "bob_location")

	err := db.RemoveRelationsByMemoryKey("team_info")
	if err != nil {
		t.Fatal(err)
	}

	// Alice should have no relations left (both were team_info)
	aliceID, _ := db.UpsertEntity("Alice", "")
	rels, _ := db.getRelationsForEntity(aliceID)
	if len(rels) != 0 {
		t.Fatalf("expected 0 relations for Alice, got %d", len(rels))
	}

	// Bob's other relation should survive
	bobID, _ := db.UpsertEntity("Bob", "")
	rels, _ = db.getRelationsForEntity(bobID)
	if len(rels) != 1 {
		t.Fatalf("expected 1 relation for Bob, got %d", len(rels))
	}
}

func TestFindEntitiesCaseInsensitive(t *testing.T) {
	db := openTestDB(t)

	db.UpsertEntity("Alice", "person")
	db.UpsertEntity("PicoClaw", "project")

	entities, err := db.FindEntities([]string{"alice", "picoclaw"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entities) != 2 {
		t.Fatalf("expected 2 entities (case-insensitive), got %d", len(entities))
	}
}

func TestFindEntitiesEmpty(t *testing.T) {
	db := openTestDB(t)

	entities, err := db.FindEntities(nil)
	if err != nil {
		t.Fatal(err)
	}
	if entities != nil {
		t.Fatalf("expected nil for empty input, got %v", entities)
	}
}

func TestAllEntityNames(t *testing.T) {
	db := openTestDB(t)

	db.UpsertEntity("Alice", "person")
	db.UpsertEntity("Bob", "person")
	db.UpsertEntity("PicoClaw", "project")

	names, err := db.AllEntityNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	// Should be sorted alphabetically
	if names[0] != "Alice" || names[1] != "Bob" || names[2] != "PicoClaw" {
		t.Fatalf("unexpected order: %v", names)
	}
}

func TestAllEntityNamesEmpty(t *testing.T) {
	db := openTestDB(t)

	names, err := db.AllEntityNames()
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 0 {
		t.Fatalf("expected 0 names on empty DB, got %d", len(names))
	}
}

func TestWalkGraphSingleHop(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")
	db.AddRelation("Bob", "works_on", "PicoClaw", "m2")

	nodes, err := db.WalkGraph([]string{"Alice"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}

	// Alice (seed) + PicoClaw (1 hop)
	if len(nodes) < 2 {
		t.Fatalf("expected at least 2 nodes, got %d", len(nodes))
	}

	nameSet := nodeNames(nodes)
	if !nameSet["Alice"] {
		t.Fatal("expected Alice in results")
	}
	if !nameSet["PicoClaw"] {
		t.Fatal("expected PicoClaw in results")
	}
}

func TestWalkGraphTwoHops(t *testing.T) {
	db := openTestDB(t)

	// Alice -> PicoClaw -> Bob (2 hops from Alice to Bob)
	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")
	db.AddRelation("Bob", "works_on", "PicoClaw", "m2")

	nodes, err := db.WalkGraph([]string{"Alice"}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := nodeNames(nodes)
	if !nameSet["Alice"] {
		t.Fatal("expected Alice")
	}
	if !nameSet["PicoClaw"] {
		t.Fatal("expected PicoClaw")
	}
	if !nameSet["Bob"] {
		t.Fatal("expected Bob at 2 hops")
	}
}

func TestWalkGraphMaxHopsLimit(t *testing.T) {
	db := openTestDB(t)

	// Chain: A -> B -> C -> D
	db.AddRelation("A", "knows", "B", "m1")
	db.AddRelation("B", "knows", "C", "m2")
	db.AddRelation("C", "knows", "D", "m3")

	// maxHops=1: should only reach A and B
	nodes, err := db.WalkGraph([]string{"A"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := nodeNames(nodes)
	if !nameSet["A"] || !nameSet["B"] {
		t.Fatal("expected A and B")
	}
	if nameSet["C"] || nameSet["D"] {
		t.Fatal("C and D should not be reachable at 1 hop")
	}
}

func TestWalkGraphMaxNodesLimit(t *testing.T) {
	db := openTestDB(t)

	// Star: Center connected to many nodes
	for i := 0; i < 10; i++ {
		db.AddRelation("Center", "connects", nodeName(i), "m")
	}

	// maxNodes=3: should stop early
	nodes, err := db.WalkGraph([]string{"Center"}, 1, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) > 3 {
		t.Fatalf("expected at most 3 nodes, got %d", len(nodes))
	}
}

func TestWalkGraphNoSeeds(t *testing.T) {
	db := openTestDB(t)

	nodes, err := db.WalkGraph([]string{"NonExistent"}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}
	if nodes != nil {
		t.Fatalf("expected nil for non-existent seed, got %v", nodes)
	}
}

func TestWalkGraphDepthTracking(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("A", "knows", "B", "m1")
	db.AddRelation("B", "knows", "C", "m2")

	nodes, err := db.WalkGraph([]string{"A"}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}

	depthMap := make(map[string]int)
	for _, n := range nodes {
		depthMap[n.Entity.Name] = n.Depth
	}

	if depthMap["A"] != 0 {
		t.Fatalf("expected A at depth 0, got %d", depthMap["A"])
	}
	if depthMap["B"] != 1 {
		t.Fatalf("expected B at depth 1, got %d", depthMap["B"])
	}
	if depthMap["C"] != 2 {
		t.Fatalf("expected C at depth 2, got %d", depthMap["C"])
	}
}

func TestWalkGraphRelationsIncluded(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")

	nodes, err := db.WalkGraph([]string{"Alice"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}

	// The seed node (Alice) should have relations populated
	var aliceNode *GraphNode
	for i := range nodes {
		if nodes[i].Entity.Name == "Alice" {
			aliceNode = &nodes[i]
			break
		}
	}

	if aliceNode == nil {
		t.Fatal("Alice not found in results")
	}
	if len(aliceNode.Relations) == 0 {
		t.Fatal("expected Alice to have relations populated")
	}
	if aliceNode.Relations[0].MemoryKey != "m1" {
		t.Fatalf("expected memory_key 'm1', got %q", aliceNode.Relations[0].MemoryKey)
	}
}

func TestCleanOrphanedEntities(t *testing.T) {
	db := openTestDB(t)

	// Create entities with a relation
	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")

	// Create an orphaned entity (no relations)
	db.UpsertEntity("Orphan", "thing")

	cleaned, err := db.CleanOrphanedEntities()
	if err != nil {
		t.Fatal(err)
	}
	if cleaned != 1 {
		t.Fatalf("expected 1 orphan cleaned, got %d", cleaned)
	}

	// Orphan should be gone
	entities, _ := db.FindEntities([]string{"Orphan"})
	if len(entities) != 0 {
		t.Fatal("orphaned entity should have been deleted")
	}

	// Connected entities should survive
	entities, _ = db.FindEntities([]string{"Alice", "PicoClaw"})
	if len(entities) != 2 {
		t.Fatalf("expected 2 connected entities to survive, got %d", len(entities))
	}
}

func TestCleanOrphanedEntitiesNone(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")

	cleaned, err := db.CleanOrphanedEntities()
	if err != nil {
		t.Fatal(err)
	}
	if cleaned != 0 {
		t.Fatalf("expected 0 orphans, got %d", cleaned)
	}
}

func TestCleanOrphanedAfterRelationRemoval(t *testing.T) {
	db := openTestDB(t)

	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")

	// Remove the relation - both entities become orphans
	db.RemoveRelationsByMemoryKey("m1")

	cleaned, err := db.CleanOrphanedEntities()
	if err != nil {
		t.Fatal(err)
	}
	if cleaned != 2 {
		t.Fatalf("expected 2 orphans cleaned, got %d", cleaned)
	}
}

func TestWalkGraphBidirectional(t *testing.T) {
	db := openTestDB(t)

	// Relation: Alice -> PicoClaw
	// Walking from PicoClaw should also find Alice (reverse traversal)
	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")

	nodes, err := db.WalkGraph([]string{"PicoClaw"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := nodeNames(nodes)
	if !nameSet["PicoClaw"] {
		t.Fatal("expected PicoClaw")
	}
	if !nameSet["Alice"] {
		t.Fatal("expected Alice via reverse traversal")
	}
}

func TestWalkGraphMultipleSeeds(t *testing.T) {
	db := openTestDB(t)

	// Two disconnected subgraphs
	db.AddRelation("Alice", "works_on", "PicoClaw", "m1")
	db.AddRelation("Charlie", "lives_in", "Tokyo", "m2")

	nodes, err := db.WalkGraph([]string{"Alice", "Charlie"}, 1, 10)
	if err != nil {
		t.Fatal(err)
	}

	nameSet := nodeNames(nodes)
	if !nameSet["Alice"] || !nameSet["PicoClaw"] {
		t.Fatal("expected Alice's subgraph")
	}
	if !nameSet["Charlie"] || !nameSet["Tokyo"] {
		t.Fatal("expected Charlie's subgraph")
	}
}

func TestRetentionCleansOrphans(t *testing.T) {
	db := openTestDB(t)

	// Store a memory with relations
	db.Store("temp_fact", "Alice is here", "conversation", "")
	db.AddRelation("Alice", "mentioned_in", "TempChat", "temp_fact")

	// Backdate the memory so retention will pick it up (older than 1 day)
	old := time.Now().UTC().AddDate(0, 0, -5).Format(sqliteTimeFormat)
	db.db.Exec("UPDATE memories SET updated_at = ? WHERE key = ?", old, "temp_fact")

	// Run retention with 1-day conversation retention
	deleted, err := db.RunRetention(map[string]int{"conversation": 1})
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected 1 deleted memory, got %d", deleted)
	}

	// Orphaned entities should be cleaned
	names, _ := db.AllEntityNames()
	if len(names) != 0 {
		t.Fatalf("expected 0 entities after orphan cleanup, got %d: %v", len(names), names)
	}
}

// === Fix #5: Owner-scoped graph traversal ===

func TestWalkGraphForOwnerFiltersPrivate(t *testing.T) {
	db := openTestDB(t)

	// Shared memory with relation
	db.Store("team_info", "Alice works on PicoClaw", "core", "")
	db.AddRelation("Alice", "works_on", "PicoClaw", "team_info")

	// Private memory owned by bob with relation
	db.Store("bob_secret", "Bob has a secret project", "core", "bob")
	db.AddRelation("Bob", "works_on", "SecretProject", "bob_secret")

	// Alice should see shared graph but NOT bob's private relations
	nodes, err := db.WalkGraphForOwner([]string{"Alice"}, 2, 20, "alice")
	if err != nil {
		t.Fatal(err)
	}

	nameSet := nodeNames(nodes)
	if !nameSet["Alice"] {
		t.Fatal("expected Alice")
	}
	if !nameSet["PicoClaw"] {
		t.Fatal("expected PicoClaw (via shared memory)")
	}
	if nameSet["SecretProject"] {
		t.Fatal("SecretProject should NOT be reachable for alice (bob's private memory)")
	}
}

func TestWalkGraphForOwnerEmptyOwnerUnfiltered(t *testing.T) {
	db := openTestDB(t)

	db.Store("m1", "fact 1", "core", "bob")
	db.AddRelation("A", "knows", "B", "m1")

	// Empty owner = no filtering (admin mode)
	nodes, err := db.WalkGraphForOwner([]string{"A"}, 1, 10, "")
	if err != nil {
		t.Fatal(err)
	}
	nameSet := nodeNames(nodes)
	if !nameSet["A"] || !nameSet["B"] {
		t.Fatal("expected both A and B with empty owner (no filtering)")
	}
}

func TestWalkGraphForOwnerSharedMemories(t *testing.T) {
	db := openTestDB(t)

	// Shared memory is accessible to everyone
	db.Store("shared_fact", "shared info", "core", "")
	db.AddRelation("X", "links_to", "Y", "shared_fact")

	nodes, err := db.WalkGraphForOwner([]string{"X"}, 1, 10, "anyone")
	if err != nil {
		t.Fatal(err)
	}
	nameSet := nodeNames(nodes)
	if !nameSet["X"] || !nameSet["Y"] {
		t.Fatal("expected both X and Y (shared memory accessible to all)")
	}
}

func TestWalkGraphForOwnerRelationWithoutMemoryKey(t *testing.T) {
	db := openTestDB(t)

	// Relation with empty memory_key should always be traversable
	db.AddRelation("A", "knows", "B", "")

	nodes, err := db.WalkGraphForOwner([]string{"A"}, 1, 10, "alice")
	if err != nil {
		t.Fatal(err)
	}
	nameSet := nodeNames(nodes)
	if !nameSet["A"] || !nameSet["B"] {
		t.Fatal("relations without memory_key should be traversable by anyone")
	}
}

// === Fix #6: CleanStaleRelations handles empty string memory_key ===

func TestCleanStaleRelationsEmptyStringKey(t *testing.T) {
	db := openTestDB(t)

	// Relation with empty string memory_key (not NULL)
	db.AddRelation("A", "knows", "B", "")
	// Relation with a valid memory_key that exists
	db.Store("valid_key", "content", "core", "")
	db.AddRelation("C", "uses", "D", "valid_key")
	// Relation with a stale memory_key (no matching memory)
	db.AddRelation("E", "sees", "F", "nonexistent_key")

	cleaned, err := db.CleanStaleRelations()
	if err != nil {
		t.Fatal(err)
	}

	// Should clean the stale key but NOT the empty string key
	if cleaned != 1 {
		t.Fatalf("expected 1 stale relation cleaned, got %d", cleaned)
	}

	// Empty string relation should survive
	aID, _ := db.UpsertEntity("A", "")
	rels, _ := db.getRelationsForEntity(aID)
	if len(rels) != 1 {
		t.Fatalf("expected empty-key relation to survive, got %d relations", len(rels))
	}

	// Valid key relation should survive
	cID, _ := db.UpsertEntity("C", "")
	rels, _ = db.getRelationsForEntity(cID)
	if len(rels) != 1 {
		t.Fatalf("expected valid-key relation to survive, got %d relations", len(rels))
	}
}

func TestCleanStaleRelationsNullKey(t *testing.T) {
	db := openTestDB(t)

	// Insert a relation with actual NULL memory_key via raw SQL
	srcID, _ := db.UpsertEntity("P", "thing")
	tgtID, _ := db.UpsertEntity("Q", "thing")
	db.db.Exec("INSERT INTO relations (source_id, relation, target_id, memory_key) VALUES (?, ?, ?, NULL)",
		srcID, "links", tgtID)

	cleaned, err := db.CleanStaleRelations()
	if err != nil {
		t.Fatal(err)
	}
	// NULL key relations should not be cleaned
	if cleaned != 0 {
		t.Fatalf("expected 0 cleaned (NULL key should be preserved), got %d", cleaned)
	}
}

// === Fix #10: Deterministic WalkGraph ordering ===

func TestWalkGraphDeterministicOrder(t *testing.T) {
	db := openTestDB(t)

	// Build a graph with multiple nodes at various depths
	db.AddRelation("Root", "connects", "Child1", "m1")
	db.AddRelation("Root", "connects", "Child2", "m2")
	db.AddRelation("Root", "connects", "Child3", "m3")
	db.AddRelation("Child1", "connects", "Grandchild1", "m4")

	// Run walk multiple times - order should be consistent
	var firstOrder []string
	for i := 0; i < 5; i++ {
		nodes, err := db.WalkGraph([]string{"Root"}, 2, 20)
		if err != nil {
			t.Fatal(err)
		}

		var names []string
		for _, n := range nodes {
			names = append(names, n.Entity.Name)
		}

		if i == 0 {
			firstOrder = names
		} else {
			if len(names) != len(firstOrder) {
				t.Fatalf("run %d: different node count: %v vs %v", i, names, firstOrder)
			}
			for j := range names {
				if names[j] != firstOrder[j] {
					t.Fatalf("run %d: non-deterministic order at position %d: %v vs %v", i, j, names, firstOrder)
				}
			}
		}
	}
}

func TestWalkGraphDepthOrderedFirst(t *testing.T) {
	db := openTestDB(t)

	// Chain: A -> B -> C
	db.AddRelation("A", "knows", "B", "m1")
	db.AddRelation("B", "knows", "C", "m2")

	nodes, err := db.WalkGraph([]string{"A"}, 2, 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	// Depth 0 should come before depth 1 which should come before depth 2
	if nodes[0].Depth != 0 {
		t.Fatalf("first node should be depth 0, got %d", nodes[0].Depth)
	}
	if nodes[1].Depth != 1 {
		t.Fatalf("second node should be depth 1, got %d", nodes[1].Depth)
	}
	if nodes[2].Depth != 2 {
		t.Fatalf("third node should be depth 2, got %d", nodes[2].Depth)
	}
}

// helpers

func nodeNames(nodes []GraphNode) map[string]bool {
	m := make(map[string]bool)
	for _, n := range nodes {
		m[n.Entity.Name] = true
	}
	return m
}

func nodeName(i int) string {
	return string(rune('N')) + string(rune('0'+i))
}
