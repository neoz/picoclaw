---
name: memory-optimize
description: Optimize all stored memories and knowledge graph by deduplicating, merging, re-categorizing, condensing, and pruning low-value entries. Also clean up stale graph relations and orphaned entities. Use when asked to "optimize memory", "clean up memories", "consolidate memories", "deduplicate memories", "memory maintenance", "memory cleanup", "optimize graph", "clean knowledge graph", or when memory storage feels bloated or noisy.
---

# Memory Optimize

Scan all stored memories and knowledge graph, then optimize for quality, relevance, and efficiency.

## Workflow

### 1. Inventory

Retrieve all memories by category using `memory_search` with empty query and `limit: 100`:

- `category: "core"`
- `category: "daily"`
- `category: "conversation"`
- `category: "custom"`

Record total count, list of keys, confidence scores, and update dates per category.

### 2. Identify Issues

Scan inventory for:

**Duplicates**: Different keys with >80% content overlap. Group them.

**Mergeable**: Separate entries about the same topic/entity (e.g. multiple fragments about one person, same project tracked across dates).

**Miscategorized**:
- Permanent facts in `daily`/`conversation` -> should be `core`
- Ephemeral info in `core` -> should be `daily` or `conversation`
- User preferences in `custom` -> should be `core`

**Low-value**: Outdated plans, trivial info, stale conversation context with no lasting value.

**Bloated**: Entries with excessive content that can be condensed without losing meaning.

**Low-confidence**: Entries with confidence below 30% that haven't been accessed recently.

### 3. Graph Audit

Review knowledge graph for issues:

- **Stale relations**: Relations pointing to deleted memory keys
- **Orphaned entities**: Entities with no relations
- **Duplicate entities**: Same real-world entity with variant names (e.g. "John", "john", "John Doe")
- **Missing relations**: Memories mentioning entity connections that lack graph edges
- **Weak relations**: Relations that could be strengthened or consolidated

Extract entity and relation info from memory content. For each memory mentioning entities, verify corresponding graph relations exist.

### 4. Present Plan

Before making changes, present a summary:

```
## Memory Optimization Plan

**Memories**: {N} total (core: {n}, daily: {n}, conversation: {n}, custom: {n})

### Memory Actions
- Merge: {N} groups ({key groups})
- Re-categorize: {N} entries ({key}: {old} -> {new})
- Condense: {N} entries ({keys})
- Delete: {N} low-value entries ({keys})

### Graph Actions
- Clean stale relations: {N} estimated
- Remove orphaned entities: {N} estimated
- Merge duplicate entities: {N} groups ({entity names})
- Add missing relations: {N} new edges

Proceed? (y/n)
```

Wait for user confirmation.

### 5. Execute

Apply in this order:

**5a. Graph cleanup first** (prevents conflicts with memory changes):
1. For duplicate entities, pick canonical name and re-store affected memories with updated `relations` parameter
2. Add missing relations by re-storing relevant memories with correct `relations` array

**5b. Memory optimization**:
1. **Merge**: Combine content into best key via `memory_store`. Preserve all `relations` from both entries. Delete redundant keys with `memory_forget`.
2. **Re-categorize**: `memory_store` same key+content with correct category. Include existing `relations`.
3. **Condense**: `memory_store` same key with tighter content. Keep all facts, remove filler/repetition. Preserve `relations`.
4. **Delete**: `memory_forget` low-value entries (graph relations auto-cleaned).

**5c. Final graph sweep**:
- Stale relations and orphaned entities are auto-cleaned by the system retention chain, but flag any remaining for awareness.

### 6. Report

```
## Optimization Complete

### Memories
- Merged: {N} entries into {M}
- Re-categorized: {N}
- Condensed: {N} (saved ~{X}% content)
- Deleted: {N}
- Final count: {N} (was {old_N})

### Knowledge Graph
- Stale relations cleaned: {N}
- Orphaned entities removed: {N}
- Duplicate entities merged: {N}
- Missing relations added: {N}
```

## Rules

- Never delete or modify `core` entries without explicit user approval.
- Preserve all knowledge graph relations when merging. Re-attach relations to the surviving key using the `relations` parameter in `memory_store`.
- When condensing, keep all facts and entities intact. Only remove filler and repetition.
- When merging, prefer the more descriptive key name. Deduplicate content but keep all unique facts.
- When merging duplicate entities, prefer the most complete/formal name as canonical.
- If unsure whether an entry is low-value, keep it and flag for user review.
