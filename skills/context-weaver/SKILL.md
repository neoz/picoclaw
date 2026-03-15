---
name: context-weaver
description: Enrich responses by actively pulling related memories, knowledge graph connections, and past conversation context before answering. Use on any non-trivial question or task where historical context would improve the response. Triggers implicitly -- the agent should weave context whenever a topic has prior history in memory or the knowledge graph. Also triggers explicitly on "what do you know about X", "remind me about X", "context on X", "background on X".
---

# Context Weaver

Before responding to a non-trivial request, actively gather related context from all available sources to produce a more informed response.

## When to Weave

**Always weave** for:
- Questions about people, projects, or topics that may have history
- Tasks that build on prior work or decisions
- Requests where user context (preferences, role, past interactions) matters
- Follow-ups to multi-session conversations

**Skip weaving** for:
- Simple greetings and small talk
- Self-contained questions with no history (weather, math, definitions)
- Requests where the user provides all needed context

## Workflow

### 1. Identify Entities

From the user's message, extract:
- People names and usernames
- Project/product names
- Technical terms and tools
- Topic keywords

### 2. Gather (parallel where possible)

Run these searches to build context:

**Memory search** -- direct keyword matches:
```
memory_search(query="[primary topic/entity]", limit=10)
```

**Graph walk** -- the system does this automatically via `buildGraphMemoryContext`, but for deeper context on specific entities:
```
memory_search(query="[related entity from graph]", limit=5)
```

**Correction rules** -- check if any behavioral rules apply:
```
memory_search(query="correction-rule [relevant behavior]", limit=5)
```

**Past conversations** -- if topic spans sessions:
```
message_history(action="search", query="[topic keyword]", limit=20)
```

### 3. Synthesize

From gathered context, identify:

- **Relevant facts**: What does the bot already know about this topic?
- **User preferences**: How does this user prefer responses on this topic?
- **History**: What has been discussed, decided, or done before?
- **Relationships**: How do the entities connect?
- **Gaps**: What is missing or outdated?

### 4. Apply

Integrate context into the response naturally:

- Reference prior knowledge without restating everything ("As discussed last week, ...")
- Apply user preferences silently (language, format, detail level)
- Flag contradictions between current request and stored context
- Note when stored context may be outdated

Do NOT dump raw memory results into the response. Synthesize into natural, relevant context.

### 5. Update (optional)

If the current conversation reveals new facts that update stored context:
```
memory_store(
  key="[existing key or new key]",
  content="[updated fact]",
  category="core",
  relations=[...]
)
```

Keep the knowledge base current as a side effect of answering.

## Context Priority

When context is abundant, prioritize by:

1. Correction rules (highest -- avoid known mistakes)
2. User-specific preferences and history
3. Direct entity matches from knowledge graph
4. Related topic matches from memory search
5. Past conversation snippets (lowest -- most likely stale)

## Guidelines

- Context weaving should be invisible to the user -- responses just feel smarter
- Do not mention "I searched my memory" unless the user explicitly asks what you know
- If context contradicts the user's current statement, ask for clarification rather than assuming
- Keep weaving lightweight -- 2-3 targeted searches, not exhaustive scans
- Trust graph context (auto-injected) for entity relationships; use manual search for deeper dives
