---
name: memory-optimize
description: Optimize all stored memories and knowledge graph by deduplicating, merging, re-categorizing, condensing, and pruning low-value entries. Also clean up stale graph relations and orphaned entities. Use when asked to "optimize memory", "clean up memories", "consolidate memories", "deduplicate memories", "memory maintenance", "memory cleanup", "optimize graph", "clean knowledge graph", or when memory storage feels bloated or noisy.
---

# Memory Optimize

Scan all stored memories and knowledge graph, then optimize for quality, relevance, and efficiency.

This skill is tool-agnostic. Use whatever memory-related tools are available (search, store/save, delete/forget, list, etc.) to accomplish each step. Adapt parameter names and calling conventions to match the actual tools provided.

## Workflow

### 1. Discover Tools

Before starting, identify available memory-related tools. Look for tools that can:

- **Search/List** memories (retrieve existing entries)
- **Store/Save/Update** memories (create or modify entries)
- **Delete/Forget/Remove** memories (remove entries)
- **Graph operations** if available (entities, relations)

Note the actual tool names and their parameters. All subsequent steps should use these discovered tools.

### 2. Inventory

Retrieve all memories using the available search/list tools. If the tools support categories or filtering, query each category separately. Otherwise, retrieve all entries in batches.

Record:
- Total count of memories
- List of keys/identifiers
- Any metadata (categories, tags, timestamps, confidence scores, etc.)
- Content summaries for dedup analysis

### 3. Identify Issues

Scan inventory for:

**Duplicates**: Different keys with >80% content overlap. Group them.

**Mergeable**: Separate entries about the same topic/entity (e.g. multiple fragments about one person, same project tracked across dates).

**Miscategorized** (if categories exist):
- Permanent facts stored as ephemeral -> should be long-term/core
- Ephemeral info stored as permanent -> should be short-term/daily
- User preferences in wrong category

**Low-value**: Outdated plans, trivial info, stale context with no lasting value.

**Bloated**: Entries with excessive content that can be condensed without losing meaning.

**Low-confidence/Low-quality**: Entries with low confidence/relevance scores that haven't been accessed recently (if such metadata is available).

### 4. Graph Audit (if graph tools available)

Review knowledge graph for issues:

- **Stale relations**: Relations pointing to deleted memory keys
- **Orphaned entities**: Entities with no relations
- **Duplicate entities**: Same real-world entity with variant names (e.g. "John", "john", "John Doe")
- **Missing relations**: Memories mentioning entity connections that lack graph edges
- **Weak relations**: Relations that could be strengthened or consolidated

If no graph tools are available, skip this step.

### 5. Present Plan

Before making changes, present a summary:

```
## Memory Optimization Plan

**Memories**: {N} total ({breakdown by category if applicable})

### Memory Actions
- Merge: {N} groups ({key groups})
- Re-categorize: {N} entries ({key}: {old} -> {new})
- Condense: {N} entries ({keys})
- Delete: {N} low-value entries ({keys})

### Graph Actions (if applicable)
- Clean stale relations: {N} estimated
- Remove orphaned entities: {N} estimated
- Merge duplicate entities: {N} groups ({entity names})
- Add missing relations: {N} new edges

Proceed? (y/n)
```

Wait for user confirmation.

### 6. Execute

Apply in this order:

**6a. Graph cleanup first** (if applicable, prevents conflicts with memory changes):
1. For duplicate entities, pick canonical name and update affected memories
2. Add missing relations by updating relevant memories

**6b. Memory optimization**:
1. **Merge**: Combine content into best key. Preserve all metadata and relations from both entries. Delete redundant keys.
2. **Re-categorize**: Update entries with correct category/tags.
3. **Condense**: Update entries with tighter content. Keep all facts, remove filler/repetition. Preserve metadata.
4. **Delete**: Remove low-value entries.

**6c. Final sweep**:
- Verify no broken references remain
- Flag any remaining issues for awareness

### 7. Report

```
## Optimization Complete

### Memories
- Merged: {N} entries into {M}
- Re-categorized: {N}
- Condensed: {N} (saved ~{X}% content)
- Deleted: {N}
- Final count: {N} (was {old_N})

### Knowledge Graph (if applicable)
- Stale relations cleaned: {N}
- Orphaned entities removed: {N}
- Duplicate entities merged: {N}
- Missing relations added: {N}
```

## Rules

- Never delete or modify long-term/core entries without explicit user approval.
- Preserve all graph relations and metadata when merging entries.
- When condensing, keep all facts and entities intact. Only remove filler and repetition.
- When merging, prefer the more descriptive key name. Deduplicate content but keep all unique facts.
- When merging duplicate entities, prefer the most complete/formal name as canonical.
- If unsure whether an entry is low-value, keep it and flag for user review.
