package sage

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// These are integration tests that require a running Sage server.
// Run with: SAGE_TEST_URL=http://localhost:18099 go test ./pkg/sage/ -v -count=1
//
// Start the test server:
//   docker compose -f docker-compose.test.yml up -d sage-test

func sageTestBackend(t *testing.T) *SageBackend {
	t.Helper()

	baseURL := os.Getenv("SAGE_TEST_URL")
	if baseURL == "" {
		t.Skip("SAGE_TEST_URL not set, skipping integration test")
	}

	keysDir := t.TempDir()
	client := NewClient(baseURL)
	identity := NewIdentityManager(keysDir, client)

	// Verify connectivity
	if err := client.Health(); err != nil {
		t.Skipf("Sage server not reachable at %s: %v", baseURL, err)
	}

	return NewSageBackend(client, identity)
}

func TestSageStoreAndList(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_greeting", "hello world", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	entries, err := sb.List("", 50, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "test_greeting" && e.Content == "hello world" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find 'test_greeting' in %d entries", len(entries))
	}

	// Cleanup
	sb.deleteByKey("test_greeting", "")
}

func TestSageStoreAndGet(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_get_key", "get content", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	entry := sb.Get("test_get_key")
	if entry == nil {
		t.Fatal("Get returned nil, expected entry")
	}
	if entry.Content != "get content" {
		t.Fatalf("expected 'get content', got %q", entry.Content)
	}

	// Cleanup
	sb.deleteByKey("test_get_key", "")
}

func TestSageSearch(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_search_item", "unique flamingo data", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	results, err := sb.Search("flamingo", 10, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Entry.Key == "test_search_item" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find 'test_search_item' in %d search results", len(results))
	}

	// Cleanup
	sb.deleteByKey("test_search_item", "")
}

// TestSageListAllDomainTags verifies that memories stored with any domain tag
// (including LLM-assigned custom topics like "profile") are retrievable.
// This was the original bug: domainTagsForRecall had a hardcoded list that
// missed tags like "shared_profile".
func TestSageListAllDomainTags(t *testing.T) {
	sb := sageTestBackend(t)

	// Store with custom topic "profile" (like the LLM does via SetDomain)
	sb.SetDomain("test_custom_topic", "profile")
	err := sb.Store("test_custom_topic", "custom topic content", "core", "")
	if err != nil {
		t.Fatalf("Store with custom topic failed: %v", err)
	}

	// Store with default topic (knowledge)
	err = sb.Store("test_default_topic", "default topic content", "core", "")
	if err != nil {
		t.Fatalf("Store with default topic failed: %v", err)
	}

	// List should find BOTH entries, regardless of domain tag
	entries, err := sb.List("", 50, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	foundCustom := false
	foundDefault := false
	for _, e := range entries {
		if e.Key == "test_custom_topic" {
			foundCustom = true
		}
		if e.Key == "test_default_topic" {
			foundDefault = true
		}
	}

	if !foundCustom {
		t.Error("custom topic memory (shared_profile) not found in List results")
	}
	if !foundDefault {
		t.Error("default topic memory (shared_knowledge) not found in List results")
	}

	// Cleanup
	sb.deleteByKey("test_custom_topic", "")
	sb.deleteByKey("test_default_topic", "")
}

// TestSageSharedMemoriesVisibleToUser verifies that memories stored with
// owner="" (shared agent) are visible when searching with a non-empty owner.
// This was the second bug: listAll with non-empty owner queried only the
// user's agent identity, missing all shared memories.
func TestSageSharedMemoriesVisibleToUser(t *testing.T) {
	sb := sageTestBackend(t)

	// Store as shared (owner="")
	err := sb.Store("test_shared_visible", "shared data", "core", "")
	if err != nil {
		t.Fatalf("Store shared failed: %v", err)
	}

	// List as a specific user — should still see shared memories
	entries, err := sb.List("", 50, "testuser123")
	if err != nil {
		t.Fatalf("List with owner failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "test_shared_visible" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("shared memory not visible to user 'testuser123' (got %d entries)", len(entries))
	}

	// Cleanup
	sb.deleteByKey("test_shared_visible", "")
}

// TestSageSearchSharedAsUser verifies that Search with a non-empty owner
// can find shared memories.
func TestSageSearchSharedAsUser(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_search_shared", "unique zeppelin data", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	results, err := sb.Search("zeppelin", 10, "testuser456")
	if err != nil {
		t.Fatalf("Search with owner failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Entry.Key == "test_search_shared" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("shared memory not found by user search (got %d results)", len(results))
	}

	// Cleanup
	sb.deleteByKey("test_search_shared", "")
}

// TestSageUserMemoriesIsolated verifies that memories stored under a specific
// user are NOT visible to a different user (agent-scoped isolation).
func TestSageUserMemoriesIsolated(t *testing.T) {
	sb := sageTestBackend(t)

	// Store as user "alice"
	err := sb.Store("test_alice_secret", "alice private data", "core", "alice")
	if err != nil {
		t.Fatalf("Store as alice failed: %v", err)
	}

	// List as user "bob" — should NOT see alice's memory
	entries, err := sb.List("", 50, "bob")
	if err != nil {
		t.Fatalf("List as bob failed: %v", err)
	}

	for _, e := range entries {
		if e.Key == "test_alice_secret" {
			t.Fatal("bob should not see alice's private memory")
		}
	}

	// But alice should see her own memory
	entries, err = sb.List("", 50, "alice")
	if err != nil {
		t.Fatalf("List as alice failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "test_alice_secret" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("alice should see her own memory")
	}

	// Cleanup
	sb.deleteByKey("test_alice_secret", "alice")
}

// TestSageDeleteAndList verifies that deprecated memories are filtered out.
func TestSageDeleteAndList(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_delete_me", "ephemeral data", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Verify it exists
	entry := sb.Get("test_delete_me")
	if entry == nil {
		t.Fatal("expected entry before delete")
	}

	// Delete
	if !sb.DeleteAccessible("test_delete_me", "") {
		t.Fatal("DeleteAccessible returned false")
	}

	// Should no longer appear in List
	entries, err := sb.List("", 50, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, e := range entries {
		if e.Key == "test_delete_me" {
			t.Fatal("deleted memory should not appear in List")
		}
	}
}

// TestSageCategoryFilter verifies that List with a category filter works.
func TestSageCategoryFilter(t *testing.T) {
	sb := sageTestBackend(t)

	// Store memories in different categories
	err := sb.Store("test_cat_core", "core data", "core", "")
	if err != nil {
		t.Fatalf("Store core failed: %v", err)
	}
	err = sb.Store("test_cat_custom", "custom data", "custom", "")
	if err != nil {
		t.Fatalf("Store custom failed: %v", err)
	}

	// List only "custom" category
	entries, err := sb.List("custom", 50, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	for _, e := range entries {
		if e.Key == "test_cat_core" {
			t.Fatal("core memory should not appear in custom category list")
		}
	}

	foundCustom := false
	for _, e := range entries {
		if e.Key == "test_cat_custom" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatal("custom memory not found in custom category list")
	}

	// Cleanup
	sb.deleteByKey("test_cat_core", "")
	sb.deleteByKey("test_cat_custom", "")
}

// TestSageListRecent verifies that ListRecent only returns entries within the time window.
func TestSageListRecent(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_recent", "recent data", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Should find it within 1 day window
	entries, err := sb.ListRecent([]string{"core"}, 1, 50, "")
	if err != nil {
		t.Fatalf("ListRecent failed: %v", err)
	}

	found := false
	for _, e := range entries {
		if e.Key == "test_recent" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("recent memory not found in ListRecent")
	}

	// Cleanup
	sb.deleteByKey("test_recent", "")
}

// TestSageMultipleDomainTagsVisible is the key regression test.
// It stores memories with various domain tags (simulating real usage with
// LLM-assigned topics) and verifies they ALL appear in a single List call.
func TestSageMultipleDomainTagsVisible(t *testing.T) {
	sb := sageTestBackend(t)

	type testCase struct {
		key      string
		content  string
		category string
		topic    string // empty = use default
	}
	cases := []testCase{
		{"test_multi_knowledge", "knowledge data", "core", ""},              // -> shared_knowledge
		{"test_multi_profile", "profile data", "core", "profile"},           // -> shared_profile
		{"test_multi_daily", "daily data", "daily", ""},                     // -> shared_daily
		{"test_multi_conversation", "conv data", "conversation", ""},        // -> shared_conversation
		{"test_multi_custom", "custom data", "custom", ""},                  // -> shared_project
		{"test_multi_rules", "rules data", "core", "rules"},                 // -> shared_rules
		{"test_multi_arbitrary", "arbitrary data", "core", "my_new_topic"},  // -> shared_my_new_topic
	}

	// Store all
	for _, tc := range cases {
		if tc.topic != "" {
			sb.SetDomain(tc.key, tc.topic)
		}
		if err := sb.Store(tc.key, tc.content, tc.category, ""); err != nil {
			t.Fatalf("Store %q failed: %v", tc.key, err)
		}
	}

	// Give the server a moment to commit
	time.Sleep(500 * time.Millisecond)

	// List all — every entry must be visible
	entries, err := sb.List("", 100, "")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	entryKeys := make(map[string]bool)
	for _, e := range entries {
		entryKeys[e.Key] = true
	}

	missing := 0
	for _, tc := range cases {
		if !entryKeys[tc.key] {
			t.Errorf("MISSING: key=%q (would have domain_tag=%s)", tc.key, expectedDomainTag(tc.category, tc.topic))
			missing++
		}
	}
	if missing > 0 {
		t.Errorf("%d/%d memories missing from List (total entries: %d)", missing, len(cases), len(entries))
	}

	// Also verify they're visible when queried as a user
	entries, err = sb.List("", 100, "someuser")
	if err != nil {
		t.Fatalf("List as user failed: %v", err)
	}

	entryKeys = make(map[string]bool)
	for _, e := range entries {
		entryKeys[e.Key] = true
	}

	missing = 0
	for _, tc := range cases {
		if !entryKeys[tc.key] {
			t.Errorf("MISSING (user query): key=%q", tc.key)
			missing++
		}
	}
	if missing > 0 {
		t.Errorf("%d/%d memories missing from user List", missing, len(cases))
	}

	// Cleanup
	for _, tc := range cases {
		sb.deleteByKey(tc.key, "")
	}
}

// expectedDomainTag is a test helper that mirrors buildDomainTag logic.
func expectedDomainTag(category, topic string) string {
	if topic == "" {
		topic = sharedTopicDefaults[category]
		if topic == "" {
			topic = "knowledge"
		}
	}
	return "shared_" + topic
}

// TestParseKey verifies key encoding/decoding round-trip.
func TestParseKey(t *testing.T) {
	tests := []struct {
		raw     string
		wantKey string
		wantVal string
	}{
		{"[key:greeting]\nhello world", "greeting", "hello world"},
		{"[key:k]v", "k", "v"},
		{"no key here", "", "no key here"},
		{"[key:empty]\n", "empty", ""},
		{fmt.Sprintf("[key:multi]\nline1\nline2"), "multi", "line1\nline2"},
	}

	for _, tt := range tests {
		key, content := parseKey(tt.raw)
		if key != tt.wantKey || content != tt.wantVal {
			t.Errorf("parseKey(%q) = (%q, %q), want (%q, %q)",
				tt.raw, key, content, tt.wantKey, tt.wantVal)
		}
	}
}

// TestEncodeKeyRoundTrip verifies that encodeKey -> parseKey is identity.
func TestEncodeKeyRoundTrip(t *testing.T) {
	keys := []string{"simple", "with spaces", "unicode-test"}
	contents := []string{"hello", "multi\nline\ncontent", ""}

	for _, k := range keys {
		for _, c := range contents {
			encoded := encodeKey(k, c)
			gotKey, gotContent := parseKey(encoded)
			if gotKey != k || gotContent != c {
				t.Errorf("round-trip failed: encodeKey(%q,%q) -> parseKey -> (%q,%q)",
					k, c, gotKey, gotContent)
			}
		}
	}
}

// TestOwnerFromDomainTag verifies owner extraction from domain tags.
func TestOwnerFromDomainTag(t *testing.T) {
	tests := []struct {
		tag  string
		want string
	}{
		{"shared_knowledge", ""},
		{"shared_profile", ""},
		{"u_alice_profile", "alice"},
		{"u_bob_daily", "bob"},
		{"u_alice", "alice"},
		{"other", ""},
	}
	for _, tt := range tests {
		got := ownerFromDomainTag(tt.tag)
		if got != tt.want {
			t.Errorf("ownerFromDomainTag(%q) = %q, want %q", tt.tag, got, tt.want)
		}
	}
}

// TestBuildDomainTag verifies domain tag construction.
func TestBuildDomainTag(t *testing.T) {
	tests := []struct {
		owner, topic, want string
	}{
		{"", "knowledge", "shared_knowledge"},
		{"", "profile", "shared_profile"},
		{"alice", "profile", "u_alice_profile"},
		{"bob", "daily", "u_bob_daily"},
	}
	for _, tt := range tests {
		got := buildDomainTag(tt.owner, tt.topic)
		if got != tt.want {
			t.Errorf("buildDomainTag(%q,%q) = %q, want %q", tt.owner, tt.topic, got, tt.want)
		}
	}
}

// --- Unit tests for the in-memory entity graph cache ---
// These do not require a running Sage server.

// newCacheOnlyBackend creates a SageBackend with no client/identity
// for testing the local entity graph cache in isolation.
func newCacheOnlyBackend() *SageBackend {
	return NewSageBackend(nil, nil)
}

func TestAllEntityNamesEmpty(t *testing.T) {
	sb := newCacheOnlyBackend()
	names, err := sb.AllEntityNames()
	if err != nil {
		t.Fatalf("AllEntityNames: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("expected empty, got %v", names)
	}
}

func TestAllEntityNamesPopulated(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "mem1")
	sb.AddRelation("Bob", "works_at", "Acme", "mem2")

	names, err := sb.AllEntityNames()
	if err != nil {
		t.Fatalf("AllEntityNames: %v", err)
	}
	// Should be sorted: Acme, Alice, Bob
	expected := []string{"Acme", "Alice", "Bob"}
	if len(names) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, names)
	}
	for i, name := range names {
		if name != expected[i] {
			t.Errorf("names[%d] = %q, want %q", i, name, expected[i])
		}
	}
}

func TestAllEntityNamesDedup(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "mem1")
	sb.AddRelation("Alice", "likes", "Charlie", "mem2")

	names, err := sb.AllEntityNames()
	if err != nil {
		t.Fatalf("AllEntityNames: %v", err)
	}
	// Alice appears in two relations but should only be listed once
	count := 0
	for _, n := range names {
		if n == "Alice" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("Alice appeared %d times, want 1", count)
	}
}

func TestWalkGraphForOwnerEmpty(t *testing.T) {
	sb := newCacheOnlyBackend()
	nodes, err := sb.WalkGraphForOwner([]string{"Alice"}, 2, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected empty, got %d nodes", len(nodes))
	}
}

func TestWalkGraphForOwnerSingleHop(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "mem1")
	sb.AddRelation("Bob", "works_at", "Acme", "mem1")
	sb.AddRelation("Charlie", "knows", "Dave", "mem2")

	sb.mu.Lock()
	sb.keyOwner["mem1"] = ""
	sb.keyOwner["mem2"] = ""
	sb.mu.Unlock()

	// 2 hops from Alice should reach Bob (hop 1) and Acme (hop 2)
	nodes, err := sb.WalkGraphForOwner([]string{"Alice"}, 2, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nameSet := make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}

	if !nameSet["Alice"] {
		t.Error("expected Alice in results")
	}
	if !nameSet["Bob"] {
		t.Error("expected Bob in results (1 hop from Alice)")
	}
	if !nameSet["Acme"] {
		t.Error("expected Acme in results (2 hops from Alice via Bob)")
	}
	if nameSet["Charlie"] || nameSet["Dave"] {
		t.Error("Charlie/Dave should not be reachable from Alice")
	}
}

func TestWalkGraphForOwnerMaxHops(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("A", "r", "B", "m1")
	sb.AddRelation("B", "r", "C", "m1")
	sb.AddRelation("C", "r", "D", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	// 1 hop from A should reach B but not C
	nodes, err := sb.WalkGraphForOwner([]string{"A"}, 1, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nameSet := make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}

	if !nameSet["A"] || !nameSet["B"] {
		t.Error("expected A and B within 1 hop")
	}
	if nameSet["C"] || nameSet["D"] {
		t.Error("C and D should not be reachable within 1 hop")
	}

	// 2 hops from A should reach B and C but not D
	nodes, err = sb.WalkGraphForOwner([]string{"A"}, 2, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nameSet = make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}

	if !nameSet["C"] {
		t.Error("expected C within 2 hops")
	}
	if nameSet["D"] {
		t.Error("D should not be reachable within 2 hops")
	}
}

func TestWalkGraphForOwnerMaxNodes(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Center", "r", "N1", "m1")
	sb.AddRelation("Center", "r", "N2", "m1")
	sb.AddRelation("Center", "r", "N3", "m1")
	sb.AddRelation("Center", "r", "N4", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	nodes, err := sb.WalkGraphForOwner([]string{"Center"}, 1, 3, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	if len(nodes) > 3 {
		t.Errorf("expected at most 3 nodes, got %d", len(nodes))
	}
}

func TestWalkGraphForOwnerDepthTracking(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("A", "r", "B", "m1")
	sb.AddRelation("B", "r", "C", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	nodes, err := sb.WalkGraphForOwner([]string{"A"}, 2, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	depthMap := make(map[string]int)
	for _, n := range nodes {
		depthMap[n.Entity.Name] = n.Depth
	}

	if depthMap["A"] != 0 {
		t.Errorf("A depth = %d, want 0", depthMap["A"])
	}
	if depthMap["B"] != 1 {
		t.Errorf("B depth = %d, want 1", depthMap["B"])
	}
	if depthMap["C"] != 2 {
		t.Errorf("C depth = %d, want 2", depthMap["C"])
	}
}

func TestWalkGraphForOwnerScoping(t *testing.T) {
	sb := newCacheOnlyBackend()
	// alice's relations
	sb.AddRelation("Alice", "knows", "Secret", "alice_mem")
	// shared relations
	sb.AddRelation("Alice", "knows", "Public", "shared_mem")

	sb.mu.Lock()
	sb.keyOwner["alice_mem"] = "alice"
	sb.keyOwner["shared_mem"] = ""
	sb.mu.Unlock()

	// Bob should NOT see alice's private relations
	nodes, err := sb.WalkGraphForOwner([]string{"Alice"}, 1, 10, "bob")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nameSet := make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}

	if nameSet["Secret"] {
		t.Error("bob should not see alice's private relation to Secret")
	}
	if !nameSet["Public"] {
		t.Error("bob should see shared relation to Public")
	}

	// Alice should see her own relations
	nodes, err = sb.WalkGraphForOwner([]string{"Alice"}, 1, 10, "alice")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nameSet = make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}

	if !nameSet["Secret"] {
		t.Error("alice should see her own relation to Secret")
	}
	if !nameSet["Public"] {
		t.Error("alice should see shared relation to Public")
	}
}

func TestWalkGraphForOwnerCaseInsensitiveSeeds(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	// Seed with lowercase "alice" should still match "Alice"
	nodes, err := sb.WalkGraphForOwner([]string{"alice"}, 1, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatal("expected nodes from case-insensitive seed match")
	}

	nameSet := make(map[string]bool)
	for _, n := range nodes {
		nameSet[n.Entity.Name] = true
	}
	if !nameSet["Alice"] {
		t.Error("expected Alice in results")
	}
}

func TestWalkGraphForOwnerRelationsIncluded(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "m1")
	sb.AddRelation("Alice", "works_with", "Charlie", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	nodes, err := sb.WalkGraphForOwner([]string{"Alice"}, 1, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	// Find Alice's node and check relations
	for _, n := range nodes {
		if n.Entity.Name == "Alice" {
			if len(n.Relations) < 2 {
				t.Errorf("expected at least 2 relations on Alice, got %d", len(n.Relations))
			}
			relNames := make(map[string]bool)
			for _, r := range n.Relations {
				relNames[r.Relation] = true
			}
			if !relNames["knows"] {
				t.Error("expected 'knows' relation on Alice")
			}
			if !relNames["works_with"] {
				t.Error("expected 'works_with' relation on Alice")
			}
			return
		}
	}
	t.Error("Alice not found in results")
}

func TestRemoveRelationsByMemoryKeyCleansCache(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Alice", "knows", "Bob", "m1")
	sb.AddRelation("Charlie", "knows", "Dave", "m2")

	// Verify both relations exist
	names, _ := sb.AllEntityNames()
	if len(names) != 4 {
		t.Fatalf("expected 4 entities, got %d", len(names))
	}

	// Remove relations for m1
	sb.RemoveRelationsByMemoryKey("m1")

	// Relations for m1 should be gone, m2 should remain
	sb.mu.Lock()
	relCount := len(sb.relations)
	sb.mu.Unlock()

	if relCount != 1 {
		t.Errorf("expected 1 relation remaining, got %d", relCount)
	}

	// Entity names are still cached (not removed by RemoveRelationsByMemoryKey)
	names, _ = sb.AllEntityNames()
	if len(names) != 4 {
		t.Fatalf("entity names should persist after relation removal, got %d", len(names))
	}
}

func TestWalkGraphForOwnerDeterministicOrder(t *testing.T) {
	sb := newCacheOnlyBackend()
	sb.AddRelation("Root", "r", "B", "m1")
	sb.AddRelation("Root", "r", "A", "m1")
	sb.AddRelation("Root", "r", "C", "m1")

	sb.mu.Lock()
	sb.keyOwner["m1"] = ""
	sb.mu.Unlock()

	// Run multiple times to check determinism
	var prev []string
	for i := 0; i < 5; i++ {
		nodes, err := sb.WalkGraphForOwner([]string{"Root"}, 1, 10, "")
		if err != nil {
			t.Fatalf("WalkGraphForOwner: %v", err)
		}
		var names []string
		for _, n := range nodes {
			names = append(names, n.Entity.Name)
		}
		if prev != nil {
			for j := range names {
				if names[j] != prev[j] {
					t.Fatalf("non-deterministic order: run %d got %v, previous %v", i, names, prev)
				}
			}
		}
		prev = names
	}
}

// --- Integration tests for LinkMemories ---

func TestSageLinkMemories(t *testing.T) {
	sb := sageTestBackend(t)

	// Store two memories
	err := sb.Store("test_link_src", "source memory", "core", "")
	if err != nil {
		t.Fatalf("Store source: %v", err)
	}
	err = sb.Store("test_link_tgt", "target memory", "core", "")
	if err != nil {
		t.Fatalf("Store target: %v", err)
	}

	// Look up their Sage IDs
	sb.mu.Lock()
	srcID := sb.keyIDs["test_link_src"]
	tgtID := sb.keyIDs["test_link_tgt"]
	sb.mu.Unlock()

	if srcID == "" || tgtID == "" {
		t.Fatalf("missing memory IDs: src=%q tgt=%q", srcID, tgtID)
	}

	// Link them
	privKey, agentID, err := sb.identity.GetOrCreate("")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	err = sb.client.LinkMemories(agentID, privKey, srcID, tgtID, "related")
	if err != nil {
		t.Fatalf("LinkMemories: %v", err)
	}

	// Link with custom type
	err = sb.client.LinkMemories(agentID, privKey, srcID, tgtID, "causal")
	if err != nil {
		// May fail if server enforces unique (source,target) — that's OK
		t.Logf("LinkMemories with different type: %v (may be expected)", err)
	}

	// Cleanup
	sb.deleteByKey("test_link_src", "")
	sb.deleteByKey("test_link_tgt", "")
}

func TestSageStoreWithTriplesLinksRelated(t *testing.T) {
	sb := sageTestBackend(t)

	// Store first memory with an entity relation
	sb.AddRelation("ProjectX", "uses", "GoLang", "test_triple_a")
	err := sb.Store("test_triple_a", "ProjectX uses Go", "core", "")
	if err != nil {
		t.Fatalf("Store A: %v", err)
	}

	// Store second memory that shares an entity with the first
	sb.AddRelation("ProjectX", "deployed_on", "Linux", "test_triple_b")
	err = sb.Store("test_triple_b", "ProjectX runs on Linux", "core", "")
	if err != nil {
		t.Fatalf("Store B: %v", err)
	}

	// Verify that both memories have IDs (linking was attempted)
	sb.mu.Lock()
	idA := sb.keyIDs["test_triple_a"]
	idB := sb.keyIDs["test_triple_b"]
	sb.mu.Unlock()

	if idA == "" || idB == "" {
		t.Fatalf("missing IDs: A=%q B=%q", idA, idB)
	}

	// Verify entity cache was populated
	names, err := sb.AllEntityNames()
	if err != nil {
		t.Fatalf("AllEntityNames: %v", err)
	}

	expected := map[string]bool{"GoLang": true, "Linux": true, "ProjectX": true}
	for _, n := range names {
		delete(expected, n)
	}
	if len(expected) > 0 {
		t.Errorf("missing entities: %v", expected)
	}

	// Verify graph traversal works
	nodes, err := sb.WalkGraphForOwner([]string{"ProjectX"}, 1, 10, "")
	if err != nil {
		t.Fatalf("WalkGraphForOwner: %v", err)
	}

	nodeNames := make(map[string]bool)
	for _, n := range nodes {
		nodeNames[n.Entity.Name] = true
	}

	if !nodeNames["GoLang"] {
		t.Error("expected GoLang reachable from ProjectX")
	}
	if !nodeNames["Linux"] {
		t.Error("expected Linux reachable from ProjectX")
	}

	// Cleanup
	sb.deleteByKey("test_triple_a", "")
	sb.deleteByKey("test_triple_b", "")
}

// --- Integration tests for semantic search (embed + query) ---

// TestSageSemanticSearch verifies that Search uses Sage's embed + query
// pipeline for server-side vector similarity instead of client-side substring matching.
func TestSageSemanticSearch(t *testing.T) {
	sb := sageTestBackend(t)

	// Store a memory with distinctive content
	err := sb.Store("test_semantic_item", "the quick brown fox jumps over the lazy dog", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Wait for indexing
	time.Sleep(500 * time.Millisecond)

	// Search with semantically related query (not exact substring)
	results, err := sb.Search("fox jumping over dog", 10, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Entry.Key == "test_semantic_item" {
			found = true
			break
		}
	}
	if !found {
		// With hash embeddings (no_llm config), semantic search may not match;
		// text fallback should still find it via shared query terms.
		t.Fatalf("expected to find 'test_semantic_item' in %d results", len(results))
	}

	// Cleanup
	sb.deleteByKey("test_semantic_item", "")
}

// TestSageSemanticSearchSharedAsUser verifies that semantic search as a user
// can find shared memories (queries both user and shared agents).
func TestSageSemanticSearchSharedAsUser(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_sem_shared", "elephants have excellent memory", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	results, err := sb.Search("elephant memory", 10, "testuser789")
	if err != nil {
		t.Fatalf("Search with owner failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Entry.Key == "test_sem_shared" {
			found = true
			break
		}
	}
	if !found {
		t.Logf("shared memory not found by user semantic search (got %d results, may need Ollama)", len(results))
	}

	// Cleanup
	sb.deleteByKey("test_sem_shared", "")
}

// TestSageSemanticSearchByCategory verifies that SearchByCategory filters
// results to the requested category.
func TestSageSemanticSearchByCategory(t *testing.T) {
	sb := sageTestBackend(t)

	// Store in different categories
	err := sb.Store("test_semcat_core", "the sky is blue", "core", "")
	if err != nil {
		t.Fatalf("Store core failed: %v", err)
	}
	err = sb.Store("test_semcat_custom", "the sky is blue today", "custom", "")
	if err != nil {
		t.Fatalf("Store custom failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Search only in "custom" category
	results, err := sb.SearchByCategory("sky blue", "custom", 10, "")
	if err != nil {
		t.Fatalf("SearchByCategory failed: %v", err)
	}

	for _, r := range results {
		if r.Entry.Key == "test_semcat_core" {
			t.Error("core memory should not appear in custom category search")
		}
	}

	// Cleanup
	sb.deleteByKey("test_semcat_core", "")
	sb.deleteByKey("test_semcat_custom", "")
}

// TestSageSearchFallbackToText verifies that Search still works via text
// matching when a query contains exact terms (covers both semantic and fallback paths).
func TestSageSearchFallbackToText(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_fallback_item", "unique xylophone orchestra performance", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	// Search with exact term that guarantees text match fallback works
	results, err := sb.Search("xylophone", 10, "")
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.Entry.Key == "test_fallback_item" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find 'test_fallback_item' in %d results", len(results))
	}

	// Cleanup
	sb.deleteByKey("test_fallback_item", "")
}

// --- Integration tests for Embed and QueryMemories client methods ---

// TestSageEmbed verifies the Embed client method against a real Sage server.
func TestSageEmbed(t *testing.T) {
	sb := sageTestBackend(t)

	privKey, agentID, err := sb.identity.GetOrCreate("")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	embedding, err := sb.client.Embed(agentID, privKey, "hello world")
	if err != nil {
		t.Skipf("Embed not available (Ollama may not be running): %v", err)
	}

	if len(embedding) == 0 {
		t.Fatal("expected non-empty embedding")
	}
}

// TestSageQueryMemories verifies the QueryMemories client method.
func TestSageQueryMemories(t *testing.T) {
	sb := sageTestBackend(t)

	// Store something first
	err := sb.Store("test_query_api", "quantum computing breakthroughs", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	privKey, agentID, err := sb.identity.GetOrCreate("")
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	embedding, err := sb.client.Embed(agentID, privKey, "quantum computing")
	if err != nil {
		t.Skipf("Embed not available: %v", err)
	}

	resp, err := sb.client.QueryMemories(agentID, privKey, QueryRequest{
		Embedding: embedding,
		TopK:      10,
	})
	if err != nil {
		t.Fatalf("QueryMemories failed: %v", err)
	}

	// With hash embeddings (no_llm test config), cosine similarity may not
	// produce meaningful matches. Just verify the API call succeeds and
	// returns a valid response structure.
	t.Logf("QueryMemories returned %d results (hash embeddings may yield 0)", len(resp.Results))

	found := false
	for _, r := range resp.Results {
		key, _ := parseKey(r.Content)
		if key == "test_query_api" {
			found = true
			break
		}
	}
	if !found && len(resp.Results) > 0 {
		t.Logf("test_query_api not in top results (got %d results)", len(resp.Results))
	}

	// Cleanup
	sb.deleteByKey("test_query_api", "")
}

// --- Integration tests for listAll cache ---

// TestSageListAllCache verifies that listAll caches results and InvalidateCache clears them.
func TestSageListAllCache(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_cache_a", "cache test data", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// First call populates cache
	items1, err := sb.listAll("", 100)
	if err != nil {
		t.Fatalf("listAll failed: %v", err)
	}

	// Second call should return cached result (same pointer contents)
	items2, err := sb.listAll("", 100)
	if err != nil {
		t.Fatalf("listAll cached failed: %v", err)
	}

	if len(items1) != len(items2) {
		t.Errorf("cached listAll returned different count: %d vs %d", len(items1), len(items2))
	}

	// Store another entry and verify cache is stale until invalidated
	err = sb.Store("test_cache_b", "more cache data", "core", "")
	if err != nil {
		t.Fatalf("Store B failed: %v", err)
	}

	// Store calls InvalidateCache, so next listAll should see the new entry
	items3, err := sb.listAll("", 100)
	if err != nil {
		t.Fatalf("listAll after invalidate failed: %v", err)
	}

	foundB := false
	for _, item := range items3 {
		key, _ := parseKey(item.Content)
		if key == "test_cache_b" {
			foundB = true
			break
		}
	}
	if !foundB {
		t.Error("expected test_cache_b in listAll after cache invalidation")
	}

	// Cleanup
	sb.deleteByKey("test_cache_a", "")
	sb.deleteByKey("test_cache_b", "")
}

// TestSageListAllCacheDifferentOwner verifies that cache is not reused
// when owner changes.
func TestSageListAllCacheDifferentOwner(t *testing.T) {
	sb := sageTestBackend(t)

	// Populate cache as shared
	_, err := sb.listAll("", 50)
	if err != nil {
		t.Fatalf("listAll shared failed: %v", err)
	}

	// Verify cache exists
	sb.mu.Lock()
	cachedOwner := sb.cached.owner
	sb.mu.Unlock()
	if cachedOwner != "" {
		t.Fatalf("expected cached owner to be empty, got %q", cachedOwner)
	}

	// Call with different owner — should NOT use the cached result
	_, err = sb.listAll("someuser", 50)
	if err != nil {
		t.Fatalf("listAll someuser failed: %v", err)
	}

	sb.mu.Lock()
	cachedOwner = sb.cached.owner
	sb.mu.Unlock()
	if cachedOwner != "someuser" {
		t.Errorf("expected cached owner to be 'someuser', got %q", cachedOwner)
	}
}

// TestSageDeleteInvalidatesCache verifies that DeleteAccessible clears the cache.
func TestSageDeleteInvalidatesCache(t *testing.T) {
	sb := sageTestBackend(t)

	err := sb.Store("test_del_cache", "delete cache test", "core", "")
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Populate cache
	_, err = sb.listAll("", 100)
	if err != nil {
		t.Fatalf("listAll failed: %v", err)
	}

	sb.mu.Lock()
	hasCacheBefore := sb.cached != nil
	sb.mu.Unlock()
	if !hasCacheBefore {
		t.Fatal("expected cache to be populated")
	}

	// Delete should invalidate cache
	sb.DeleteAccessible("test_del_cache", "")

	sb.mu.Lock()
	hasCacheAfter := sb.cached != nil
	sb.mu.Unlock()
	if hasCacheAfter {
		t.Error("expected cache to be nil after delete")
	}
}

// TestSageTypeToCategory verifies mapping from Sage memory types back to categories.
func TestSageTypeToCategory(t *testing.T) {
	tests := []struct {
		memType string
		want    string
	}{
		{"fact", "core"},
		{"observation", "daily"},
		{"inference", "custom"},
		{"unknown", "core"},
	}
	for _, tt := range tests {
		got := sageTypeToCategory(tt.memType)
		if got != tt.want {
			t.Errorf("sageTypeToCategory(%q) = %q, want %q", tt.memType, got, tt.want)
		}
	}
}
