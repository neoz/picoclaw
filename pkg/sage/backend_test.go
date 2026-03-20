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
