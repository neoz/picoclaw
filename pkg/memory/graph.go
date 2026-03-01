package memory

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Entity represents a node in the knowledge graph.
type Entity struct {
	ID   int64
	Name string
	Type string // person, project, place, concept, thing
}

// Relation represents an edge between two entities.
type Relation struct {
	ID        int64
	SourceID  int64
	Relation  string
	TargetID  int64
	MemoryKey string
	Weight    float64
	CreatedAt time.Time
}

// GraphNode represents a node discovered during graph traversal.
type GraphNode struct {
	Entity    Entity
	Relations []Relation
	Depth     int
}

// UpsertEntity inserts an entity or returns the existing ID if the name already exists.
func (m *MemoryDB) UpsertEntity(name, entityType string) (int64, error) {
	if entityType == "" {
		entityType = "thing"
	}

	_, err := m.db.Exec(
		`INSERT INTO entities (name, type) VALUES (?, ?)
		 ON CONFLICT(name) DO UPDATE SET type = CASE
			WHEN excluded.type != 'thing' THEN excluded.type
			ELSE entities.type
		 END`,
		name, entityType,
	)
	if err != nil {
		return 0, fmt.Errorf("upsert entity: %w", err)
	}

	var id int64
	err = m.db.QueryRow("SELECT id FROM entities WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("get entity id: %w", err)
	}
	return id, nil
}

// AddRelation creates a relation between two entities (by name), auto-creating entities if needed.
func (m *MemoryDB) AddRelation(sourceName, relation, targetName, memoryKey string) error {
	sourceID, err := m.UpsertEntity(sourceName, "thing")
	if err != nil {
		return err
	}
	targetID, err := m.UpsertEntity(targetName, "thing")
	if err != nil {
		return err
	}

	_, err = m.db.Exec(
		`INSERT INTO relations (source_id, relation, target_id, memory_key)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(source_id, relation, target_id) DO UPDATE SET
			memory_key = excluded.memory_key`,
		sourceID, relation, targetID, memoryKey,
	)
	if err != nil {
		return fmt.Errorf("add relation: %w", err)
	}
	return nil
}

// RemoveRelationsByMemoryKey deletes all relations associated with a memory key.
func (m *MemoryDB) RemoveRelationsByMemoryKey(memoryKey string) error {
	_, err := m.db.Exec("DELETE FROM relations WHERE memory_key = ?", memoryKey)
	if err != nil {
		return fmt.Errorf("remove relations by key: %w", err)
	}
	return nil
}

// FindEntities returns entities whose names match any of the given names (case-insensitive).
func (m *MemoryDB) FindEntities(names []string) ([]Entity, error) {
	if len(names) == 0 {
		return nil, nil
	}

	placeholders := make([]string, len(names))
	args := make([]interface{}, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = strings.ToLower(n)
	}

	query := fmt.Sprintf(
		"SELECT id, name, type FROM entities WHERE LOWER(name) IN (%s)",
		strings.Join(placeholders, ","),
	)

	rows, err := m.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		var e Entity
		if err := rows.Scan(&e.ID, &e.Name, &e.Type); err != nil {
			continue
		}
		entities = append(entities, e)
	}
	if err := rows.Err(); err != nil {
		return entities, fmt.Errorf("find entities: %w", err)
	}
	return entities, nil
}

// AllEntityNames returns all entity names in the database (for matching against user messages).
func (m *MemoryDB) AllEntityNames() ([]string, error) {
	rows, err := m.db.Query("SELECT name FROM entities ORDER BY name")
	if err != nil {
		return nil, fmt.Errorf("list entity names: %w", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			continue
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return names, fmt.Errorf("list entity names: %w", err)
	}
	return names, nil
}

// WalkGraph performs BFS from seed entities up to maxHops hops, collecting up to maxNodes nodes.
func (m *MemoryDB) WalkGraph(entityNames []string, maxHops, maxNodes int) ([]GraphNode, error) {
	seeds, err := m.FindEntities(entityNames)
	if err != nil {
		return nil, err
	}
	if len(seeds) == 0 {
		return nil, nil
	}

	visited := make(map[int64]*GraphNode)
	var queue []struct {
		id    int64
		depth int
	}

	// Seed the BFS
	for _, s := range seeds {
		node := &GraphNode{Entity: s, Depth: 0}
		visited[s.ID] = node
		queue = append(queue, struct {
			id    int64
			depth int
		}{s.ID, 0})
	}

	for len(queue) > 0 && len(visited) < maxNodes {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxHops {
			continue
		}

		// Find relations where this entity is source or target
		rels, err := m.getRelationsForEntity(current.id)
		if err != nil {
			continue
		}

		visited[current.id].Relations = rels

		for _, rel := range rels {
			// Determine the neighbor (the other end of the relation)
			neighborID := rel.TargetID
			if rel.TargetID == current.id {
				neighborID = rel.SourceID
			}

			if _, seen := visited[neighborID]; seen {
				continue
			}
			if len(visited) >= maxNodes {
				break
			}

			// Fetch entity info for neighbor
			entity, err := m.getEntityByID(neighborID)
			if err != nil {
				continue
			}

			nextDepth := current.depth + 1
			node := &GraphNode{Entity: *entity, Depth: nextDepth}
			visited[neighborID] = node
			queue = append(queue, struct {
				id    int64
				depth int
			}{neighborID, nextDepth})
		}
	}

	// Collect results sorted by depth then entity ID for deterministic ordering
	result := make([]GraphNode, 0, len(visited))
	for _, node := range visited {
		result = append(result, *node)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Depth != result[j].Depth {
			return result[i].Depth < result[j].Depth
		}
		return result[i].Entity.ID < result[j].Entity.ID
	})
	return result, nil
}

// WalkGraphForOwner performs owner-scoped BFS. Only traverses relations whose
// memory_key links to a memory accessible by the given owner (shared or owned).
// When owner is empty, behaves identically to WalkGraph (no filtering).
func (m *MemoryDB) WalkGraphForOwner(entityNames []string, maxHops, maxNodes int, owner string) ([]GraphNode, error) {
	if owner == "" {
		return m.WalkGraph(entityNames, maxHops, maxNodes)
	}

	seeds, err := m.FindEntities(entityNames)
	if err != nil {
		return nil, err
	}
	if len(seeds) == 0 {
		return nil, nil
	}

	// Pre-load the set of memory keys accessible to this owner
	accessibleKeys, err := m.accessibleMemoryKeys(owner)
	if err != nil {
		return nil, err
	}

	visited := make(map[int64]*GraphNode)
	var queue []struct {
		id    int64
		depth int
	}

	for _, s := range seeds {
		node := &GraphNode{Entity: s, Depth: 0}
		visited[s.ID] = node
		queue = append(queue, struct {
			id    int64
			depth int
		}{s.ID, 0})
	}

	for len(queue) > 0 && len(visited) < maxNodes {
		current := queue[0]
		queue = queue[1:]

		if current.depth >= maxHops {
			continue
		}

		rels, err := m.getRelationsForEntity(current.id)
		if err != nil {
			continue
		}

		// Filter relations to only those accessible to the owner
		var filtered []Relation
		for _, rel := range rels {
			if rel.MemoryKey == "" || accessibleKeys[rel.MemoryKey] {
				filtered = append(filtered, rel)
			}
		}
		visited[current.id].Relations = filtered

		for _, rel := range filtered {
			neighborID := rel.TargetID
			if rel.TargetID == current.id {
				neighborID = rel.SourceID
			}

			if _, seen := visited[neighborID]; seen {
				continue
			}
			if len(visited) >= maxNodes {
				break
			}

			entity, err := m.getEntityByID(neighborID)
			if err != nil {
				continue
			}

			nextDepth := current.depth + 1
			node := &GraphNode{Entity: *entity, Depth: nextDepth}
			visited[neighborID] = node
			queue = append(queue, struct {
				id    int64
				depth int
			}{neighborID, nextDepth})
		}
	}

	result := make([]GraphNode, 0, len(visited))
	for _, node := range visited {
		result = append(result, *node)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Depth != result[j].Depth {
			return result[i].Depth < result[j].Depth
		}
		return result[i].Entity.ID < result[j].Entity.ID
	})
	return result, nil
}

// accessibleMemoryKeys returns the set of memory keys accessible to the given owner
// (shared entries + that owner's entries).
func (m *MemoryDB) accessibleMemoryKeys(owner string) (map[string]bool, error) {
	rows, err := m.db.Query("SELECT key FROM memories WHERE owner = '' OR owner = ?", owner)
	if err != nil {
		return nil, fmt.Errorf("accessible memory keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]bool)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			continue
		}
		keys[key] = true
	}
	if err := rows.Err(); err != nil {
		return keys, fmt.Errorf("accessible memory keys: %w", err)
	}
	return keys, nil
}

// CleanStaleRelations removes relations whose memory_key no longer exists in the memories table.
// Handles both NULL and empty-string memory_key values.
func (m *MemoryDB) CleanStaleRelations() (int, error) {
	result, err := m.db.Exec(`
		DELETE FROM relations
		WHERE memory_key IS NOT NULL AND memory_key != ''
		  AND memory_key NOT IN (SELECT key FROM memories)
	`)
	if err != nil {
		return 0, fmt.Errorf("clean stale relations: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

// CleanOrphanedEntities removes entities that have no relations.
func (m *MemoryDB) CleanOrphanedEntities() (int, error) {
	result, err := m.db.Exec(`
		DELETE FROM entities WHERE id NOT IN (
			SELECT source_id FROM relations
			UNION
			SELECT target_id FROM relations
		)
	`)
	if err != nil {
		return 0, fmt.Errorf("clean orphaned entities: %w", err)
	}
	rows, _ := result.RowsAffected()
	return int(rows), nil
}

func (m *MemoryDB) getRelationsForEntity(entityID int64) ([]Relation, error) {
	rows, err := m.db.Query(
		`SELECT id, source_id, relation, target_id, memory_key, weight, created_at
		 FROM relations WHERE source_id = ? OR target_id = ?`,
		entityID, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rels []Relation
	for rows.Next() {
		var r Relation
		var memKey *string
		var createdAt string
		if err := rows.Scan(&r.ID, &r.SourceID, &r.Relation, &r.TargetID, &memKey, &r.Weight, &createdAt); err != nil {
			continue
		}
		if memKey != nil {
			r.MemoryKey = *memKey
		}
		r.CreatedAt = parseTime(createdAt)
		rels = append(rels, r)
	}
	if err := rows.Err(); err != nil {
		return rels, err
	}
	return rels, nil
}

func (m *MemoryDB) getEntityByID(id int64) (*Entity, error) {
	var e Entity
	err := m.db.QueryRow("SELECT id, name, type FROM entities WHERE id = ?", id).
		Scan(&e.ID, &e.Name, &e.Type)
	if err != nil {
		return nil, err
	}
	return &e, nil
}
