---
name: auto-tagger
description: Automatically extract entities and relationships from conversations to build the knowledge graph. Use when asked to "tag this conversation", "extract entities", "build knowledge graph", "auto tag", "map relationships", "who knows who", "what's related to X". Also use proactively during summarization or when conversations contain rich entity information (people, projects, tools, decisions, locations) that should be captured for future recall.
---

# Auto Tagger

Extract entities and relationships from conversation messages and persist them to the knowledge graph via `memory_store` with `relations`.

## Entity Types

| Type | What to look for |
|------|------------------|
| **person** | Names, usernames, @mentions, pronouns resolved to names |
| **project** | Project names, repo names, product names |
| **tool** | Software, frameworks, libraries, services mentioned |
| **org** | Companies, teams, departments |
| **topic** | Recurring discussion themes, technical domains |
| **place** | Locations, servers, environments (prod, staging) |

## Relationship Types

Common relation verbs (keep consistent across tags):

| Relation | Usage |
|----------|-------|
| `works_on` | person -> project/tool |
| `maintains` | person -> project/tool |
| `uses` | person/project -> tool |
| `belongs_to` | person -> org/team |
| `knows` | person -> person |
| `depends_on` | project -> project/tool |
| `deployed_on` | project -> place |
| `interested_in` | person -> topic |
| `decided` | person -> topic (for decisions) |
| `mentioned_in` | entity -> session context |

Use these exact verbs when possible. Create new ones only when none above fits.

## Workflow

### 1. Gather Context

Retrieve recent messages to scan:

- Use `message_history` with `action: "recent"`, `limit: 100` for current session
- Or `session_messages` for a specific session if user specifies

### 2. Extract

Scan messages and identify:

- **Named entities**: People, projects, tools, orgs, places
- **Relationships**: Who works on what, who knows whom, what depends on what
- **Key facts**: Decisions, preferences, responsibilities

Minimum confidence: only tag entities mentioned 2+ times or in meaningful context (not just passing mentions).

### 3. Deduplicate

Before storing, check existing graph:

```
memory_search(query="[entity name]", limit=5)
```

- If entity already exists with same relations, skip
- If entity exists but new relations found, update
- Normalize names: prefer full names over nicknames, consistent casing

### 4. Store

Store each meaningful cluster as one memory entry with relations:

```
memory_store(
  key="graph-[primary-entity]-[context]",
  content="[concise fact: who/what and their role/relationship]",
  category="core",
  shared=true,
  relations=[
    {"source": "Alice", "relation": "works_on", "target": "PicoClaw"},
    {"source": "Alice", "relation": "uses", "target": "Go"},
    {"source": "PicoClaw", "relation": "depends_on", "target": "SQLite"}
  ]
)
```

**Key naming**: `graph-[main-entity]-[brief-context]` (e.g., `graph-alice-role`, `graph-picoclaw-stack`)

**Category selection**:
- `core` for stable facts (roles, team membership, tech stack)
- `daily` for time-bound observations (current focus, recent activity)

### 5. Report

Summarize what was tagged:

```
Tagged {N} entities with {M} relationships:

Entities: Alice (person), PicoClaw (project), Go (tool)
Relations:
  Alice --works_on--> PicoClaw
  Alice --uses--> Go
  PicoClaw --depends_on--> SQLite

New: {count} | Updated: {count} | Skipped (existing): {count}
```

## Guidelines

- Prefer fewer high-quality tags over many weak ones
- Do not tag greetings, filler, or small talk
- Resolve pronouns to names when context is clear, skip when ambiguous
- When tagging from group chats, attribute relationships to specific people, not "the group"
- Entity names must be 3+ characters (shorter names cause false matches in graph context lookups)
- Keep `content` in `memory_store` concise -- the graph relations carry the structure, content carries the context
