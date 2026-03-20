package memory

// SearchOptions provides extended search parameters.
// Backends implement what they support; unsupported fields are ignored.
type SearchOptions struct {
	Query         string  // Search query text
	Category      string  // Filter by category (core, daily, conversation, custom)
	Domain        string  // Filter by domain topic (e.g. profile, project, rules)
	MinConfidence float64 // Minimum confidence threshold 0.0-1.0 (0 = no filter)
	TimeRange     string  // Filter by recency: "today", "week", "month", "" (all)
	OwnerScope    string  // "shared", "private", "all" (default: "all")
	Tag           string  // Filter by memory tag
	Limit         int     // Max results (default: 20)
	Owner         string  // Current user (for visibility scoping)
}

// MemoryBackend defines the interface for memory storage backends.
// Both the local SQLite-based MemoryDB and remote backends (e.g. Sage)
// must implement this interface.
type MemoryBackend interface {
	Store(key, content, category, owner string) error
	Search(query string, limit int, owner string) ([]SearchResult, error)
	SearchByCategory(query, category string, limit int, owner string) ([]SearchResult, error)
	SearchWithOptions(opts SearchOptions) ([]SearchResult, error)
	DeleteAccessible(key, owner string) bool
	List(category string, limit int, owner string) ([]MemoryEntry, error)
	ListRecent(categories []string, days, limit int, owner string) ([]MemoryEntry, error)
	Get(key string) *MemoryEntry
	AllEntityNames() ([]string, error)
	WalkGraphForOwner(names []string, maxDepth, maxNodes int, owner string) ([]GraphNode, error)
	AddRelation(source, relation, target, memoryKey string) error
	RemoveRelationsByMemoryKey(key string) error
	SetTags(key string, tags []string) error
	SetDomain(key string, topic string) error
	Close() error
}

// Compile-time check: MemoryDB satisfies MemoryBackend.
var _ MemoryBackend = (*MemoryDB)(nil)
