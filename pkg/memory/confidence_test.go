package memory

import (
	"math"
	"testing"
	"time"
)

func TestComputeConfidenceCoreNeverDecays(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(-1, 0, 0) // 1 year ago

	conf := ComputeConfidence(1.0, created, now, 0, "core")
	if conf != 1.0 {
		t.Fatalf("core memory should never decay, got %f", conf)
	}
}

func TestComputeConfidenceConversationDecays(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -7) // 7 days ago (half-life)

	conf := ComputeConfidence(1.0, created, now, 0, "conversation")
	// At half-life, confidence should be ~0.5
	if math.Abs(conf-0.5) > 0.05 {
		t.Fatalf("conversation memory at half-life expected ~0.5, got %f", conf)
	}
}

func TestComputeConfidenceDailyDecays(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -30) // 30 days ago (half-life)

	conf := ComputeConfidence(1.0, created, now, 0, "daily")
	if math.Abs(conf-0.5) > 0.05 {
		t.Fatalf("daily memory at half-life expected ~0.5, got %f", conf)
	}
}

func TestComputeConfidenceCustomDecays(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -90) // 90 days ago (half-life)

	conf := ComputeConfidence(1.0, created, now, 0, "custom")
	if math.Abs(conf-0.5) > 0.05 {
		t.Fatalf("custom memory at half-life expected ~0.5, got %f", conf)
	}
}

func TestComputeConfidenceAccessBoost(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -7)

	confNoAccess := ComputeConfidence(1.0, created, now, 0, "conversation")
	confWithAccess := ComputeConfidence(1.0, created, now, 10, "conversation")

	if confWithAccess <= confNoAccess {
		t.Fatalf("access boost should increase confidence: without=%f, with=%f",
			confNoAccess, confWithAccess)
	}
}

func TestComputeConfidenceClampedToOne(t *testing.T) {
	now := time.Now().UTC()
	// Fresh core memory with many accesses - boost should not exceed 1.0
	conf := ComputeConfidence(1.0, now, now, 100, "core")
	if conf > 1.0 {
		t.Fatalf("confidence should be clamped to 1.0, got %f", conf)
	}
}

func TestComputeConfidenceNearZeroAfterLongTime(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(-1, 0, 0) // 1 year ago

	conf := ComputeConfidence(1.0, created, now, 0, "conversation")
	if conf > 0.01 {
		t.Fatalf("conversation memory after 1 year should be near zero, got %f", conf)
	}
}

func TestComputeConfidenceUnknownCategoryUsesCustomRate(t *testing.T) {
	now := time.Now().UTC()
	created := now.AddDate(0, 0, -90)

	confUnknown := ComputeConfidence(1.0, created, now, 0, "unknown_category")
	confCustom := ComputeConfidence(1.0, created, now, 0, "custom")

	if math.Abs(confUnknown-confCustom) > 0.001 {
		t.Fatalf("unknown category should use custom rate: unknown=%f, custom=%f",
			confUnknown, confCustom)
	}
}

func TestStoreAndGetConfidence(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact", "hello", "core", "")
	entry := db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry")
	}
	if entry.Confidence != 1.0 {
		t.Fatalf("expected confidence 1.0, got %f", entry.Confidence)
	}
	if entry.AccessCount != 0 {
		t.Fatalf("expected access_count 0, got %d", entry.AccessCount)
	}
}

func TestIncrementAccessCount(t *testing.T) {
	db := openTestDB(t)

	db.Store("fact", "hello", "core", "")
	entry := db.Get("fact")
	if entry == nil {
		t.Fatal("expected entry")
	}

	db.IncrementAccessCount([]int64{entry.ID})
	db.IncrementAccessCount([]int64{entry.ID})

	entry = db.Get("fact")
	if entry.AccessCount != 2 {
		t.Fatalf("expected access_count 2, got %d", entry.AccessCount)
	}
}

func TestSearchIncrementsAccessCount(t *testing.T) {
	db := openTestDB(t)

	db.Store("golang_lang", "Go is a programming language", "core", "")

	// Search should auto-increment
	results, err := db.Search("programming", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	entry := db.Get("golang_lang")
	if entry.AccessCount != 1 {
		t.Fatalf("expected access_count 1 after search, got %d", entry.AccessCount)
	}
}

func TestSearchReturnsDecayedConfidence(t *testing.T) {
	db := openTestDB(t)

	db.Store("recent_fact", "fresh information", "daily", "")

	results, err := db.Search("fresh", 10, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Fresh daily entry should have high confidence
	if results[0].DecayedConfidence < 0.9 {
		t.Fatalf("expected high decayed confidence for fresh entry, got %f",
			results[0].DecayedConfidence)
	}
}

func TestRetentionDeletesLowConfidence(t *testing.T) {
	db := openTestDB(t)

	db.Store("old_conv", "old conversation data", "conversation", "")

	// Backdate the entry to 60 days ago (well past 7-day half-life)
	old := time.Now().UTC().AddDate(0, 0, -60).Format(sqliteTimeFormat)
	db.db.Exec("UPDATE memories SET created_at = ?, updated_at = ? WHERE key = ?",
		old, old, "old_conv")

	deleted, err := db.RunRetention(map[string]int{"conversation": 90})
	if err != nil {
		t.Fatal(err)
	}

	// Should be deleted by confidence threshold even though TTL is 90 days
	// 60 days with conversation lambda (0.099): exp(-0.099*60) ~ 0.0027 < 0.01
	if deleted != 1 {
		t.Fatalf("expected 1 deleted (low confidence), got %d", deleted)
	}
}

func TestRetentionKeepsHighAccessEntries(t *testing.T) {
	db := openTestDB(t)

	db.Store("popular", "frequently accessed", "daily", "")

	// Backdate but give high access count
	old := time.Now().UTC().AddDate(0, 0, -25).Format(sqliteTimeFormat)
	db.db.Exec("UPDATE memories SET created_at = ?, updated_at = ?, access_count = 50 WHERE key = ?",
		old, old, "popular")

	deleted, err := db.RunRetention(map[string]int{"daily": 90})
	if err != nil {
		t.Fatal(err)
	}

	// 25 days with daily lambda (0.0231), access_count=50:
	// decay = exp(-0.0231*25) ~ 0.561
	// boost = 1 + 0.1*ln(51) ~ 1.393
	// conf = 1.0 * 0.561 * 1.393 ~ 0.782 >> 0.01
	if deleted != 0 {
		t.Fatalf("expected 0 deleted (high access keeps it alive), got %d", deleted)
	}
}
