---
name: messages-analysis
description: Analyze message history to build user personality profiles. Use when asked to analyze a user's personality, communication style, behavior patterns, interests, or emotional tendencies from chat history. Triggers on requests like "analyze personality", "what kind of person is X", "user profile analysis", "communication style analysis", "who is X based on messages".
---

# Messages Analysis

Analyze message history to produce a structured personality and communication profile for a user.

## Workflow

### 1. Gather Messages

Retrieve message history using available tools in this order:

- **Current session**: Use `message_history` with `action: "recent"`, `limit: 500`, `days: 1`
- **Cross-session (if needed)**: Use `session_messages` with `action: "list"` to find sessions, then `action: "recent"` with `days: 1`, `limit: 500` on relevant sessions
- **Filter by user**: Set `sender_name` to target a specific user when analyzing one person

If insufficient data (< 20 messages in the last 24h), inform the user and provide only a partial analysis with a disclaimer.

### 2. Analyze Dimensions

Evaluate messages across these dimensions:

**Communication Style**
- Formality level (casual/neutral/formal)
- Message length tendency (terse/moderate/verbose)
- Language patterns (multilingual usage, code-switching, slang, abbreviations)
- Emoji/sticker usage frequency and type

**Personality Indicators**
- Openness: curiosity, topic diversity, willingness to explore new ideas
- Conscientiousness: detail orientation, follow-through on tasks, structured vs freeform
- Extraversion: initiation frequency, group engagement, enthusiasm markers
- Agreeableness: supportive language, conflict style, collaborative vs competitive
- Emotional stability: tone consistency, reaction patterns under stress

**Interests & Topics**
- Top recurring topics (rank by frequency)
- Technical vs non-technical ratio
- Questions asked vs statements made (curiosity index)

**Behavioral Patterns**
- Active hours (time-of-day patterns from timestamps)
- Response latency tendency (quick responder vs delayed)
- Conversation initiation vs response ratio
- Topic transitions (abrupt vs gradual)

**Emotional Tone**
- Dominant sentiment (positive/neutral/negative)
- Humor frequency and style
- Frustration/excitement markers
- Supportiveness toward others

### 3. Output Format

Present analysis as a structured profile:

```
## User Profile: {name}

**Based on**: {N} messages in the last 24 hours

### Communication Style
- Formality: {level} | Length: {tendency} | Language: {patterns}

### Personality Summary
{2-3 sentence narrative synthesis}

### Big Five Indicators
- Openness: {Low/Medium/High} - {brief evidence}
- Conscientiousness: {Low/Medium/High} - {brief evidence}
- Extraversion: {Low/Medium/High} - {brief evidence}
- Agreeableness: {Low/Medium/High} - {brief evidence}
- Emotional Stability: {Low/Medium/High} - {brief evidence}

### Top Interests
1. {topic} ({frequency indicator})
2. {topic} ({frequency indicator})
3. {topic} ({frequency indicator})

### Behavioral Patterns
- Active hours: {pattern}
- Initiation ratio: {initiator/responder/balanced}
- Curiosity index: {low/medium/high}

### Emotional Tone
- Dominant: {tone} | Humor: {frequency} | Style: {description}

### Notable Traits
- {unique observation 1}
- {unique observation 2}
```

### 4. Store Results (Optional)

If the user wants to persist the analysis, store it using `memory_store`:
- Key: `personality-profile:{username}`
- Category: `custom` (90-day retention)
- Content: the full profile output

## Guidelines

- Never fabricate traits without message evidence. Cite specific message patterns.
- Mark low-confidence assessments with "(limited data)" qualifier.
- Respect privacy: if analyzing someone other than the requester, note this is based only on public group messages.
- Adapt output language to match the user's language preference.
- For group chats, offer to compare multiple users if relevant.
