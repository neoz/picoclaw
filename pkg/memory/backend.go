package memory

// MemoryBackend defines the interface for memory storage backends.
// Both the local SQLite-based MemoryDB and remote backends (e.g. Sage)
// must implement this interface.
type MemoryBackend interface {
	Store(key, content, category, owner string) error
	Search(query string, limit int, owner string) ([]SearchResult, error)
	SearchByCategory(query, category string, limit int, owner string) ([]SearchResult, error)
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
