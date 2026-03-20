---
name: context-weaver
description: Enrich responses by actively pulling related memories, knowledge graph connections, and past conversation context before answering. Use on any non-trivial question or task where historical context would improve the response. Triggers implicitly when a topic has prior history in memory or the knowledge graph. Also triggers explicitly on "what do you know about X", "remind me about X", "context on X", "background on X", "history of X".
---

# Context Weaver

Before responding to a non-trivial request, actively gather related context from all available sources to produce a more informed, personalized response.

This skill is tool-agnostic. Use whatever memory, search, and message history tools are available. Adapt parameter names and calling conventions to match the actual tools provided.

## When to Weave

**Always weave** for:
- Questions about people, projects, or topics that may have stored history
- Tasks that build on prior work or decisions
- Requests where user context (preferences, role, expertise) would shape the answer
- Follow-ups to multi-session conversations
- Anything referencing past events, decisions, or agreements

**Skip weaving** for:
- Simple greetings and small talk
- Self-contained questions with no history (math, definitions, general knowledge)
- Requests where the user provides all needed context inline

## Workflow

### 1. Discover Tools

Identify available tools for:
- **Memory search/list**: retrieve stored memories by keyword, category, or topic
- **Memory store/update**: create or modify entries (for the optional update step)
- **Message history**: search or retrieve recent/past conversation messages
- **Graph operations**: if available, walk entity relationships

Note: some systems auto-inject graph context and FTS-matched memories into the prompt. This skill handles *deliberate deeper gathering* beyond what is automatic.

### 2. Extract Entities and Keywords

From the user's message, identify:
- **Named entities**: people, usernames, project names, team names
- **Technical terms**: tools, languages, frameworks, services
- **Topic keywords**: the core subject of the request
- **Temporal references**: "last week", "that bug from March", "the migration"

### 3. Gather Context (parallel where possible)

Run targeted searches -- aim for 2-4 queries, not exhaustive scans:

**a. Primary topic search**: Search memories for the main entity or topic keyword.

**b. User profile**: Search for the requesting user's profile/preferences if the response should be tailored (e.g., language, detail level, role-specific framing).

**c. Behavioral rules**: Search for any correction rules or conventions that apply to the topic or response style.

**d. Conversation history**: If the topic spans sessions or the user references past discussion, search message history for relevant keywords.

**e. Related entities**: If initial results reveal connected entities not in the original message, do one follow-up search for those.

### 4. Synthesize

From gathered context, extract:

- **Known facts**: What is already stored about this topic?
- **User preferences**: How should the response be shaped for this user?
- **Prior decisions**: What was decided before that constrains the current answer?
- **Entity relationships**: How do the people/projects/tools connect?
- **Contradictions**: Does stored context conflict with the current request?
- **Gaps**: What is missing, outdated, or uncertain?

### 5. Apply

Integrate context into the response naturally:

- Reference prior knowledge concisely ("As decided in March, ..." / "Since you prefer ...")
- Apply user preferences silently (language, format, detail level, formality)
- Flag contradictions between the current request and stored context -- ask for clarification rather than assuming
- Note when stored context may be outdated ("This was stored on [date], may need re-checking")
- Adapt explanation depth to user's known expertise level

Do NOT dump raw memory results. Synthesize into natural, relevant context.

### 6. Update (optional)

If the conversation reveals new facts that correct or extend stored context, update the relevant memory entry as a side effect. This keeps the knowledge base current without a separate maintenance step.

## Context Priority

When context is abundant, prioritize:

1. **Correction rules / conventions** (highest -- avoid known mistakes)
2. **User-specific preferences and profile**
3. **Prior decisions and agreements** on the topic
4. **Direct entity matches** from knowledge graph or memory
5. **Related topic matches** from broader memory search
6. **Past conversation snippets** (lowest -- most likely stale)

## Rules

- Context weaving should be invisible -- responses just feel smarter and more informed
- Do not mention "I searched my memory" unless the user explicitly asks what you know
- Keep weaving lightweight -- 2-4 targeted searches, not exhaustive scans
- If auto-injected context (graph walk, FTS) already covers the topic, skip redundant manual searches
- When context contradicts the user's current statement, ask rather than assume
- Prefer recent information over older stored context when they conflict
