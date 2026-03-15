---
name: conversation-learner
description: Passively extract user preferences, communication patterns, expertise areas, and personal context from conversations. Runs after meaningful conversations to build a richer understanding of each user over time. Triggers on "learn about me", "what do you know about me", "remember my preferences", or proactively after conversations containing personal information, preferences, or behavioral patterns worth remembering.
---

# Conversation Learner

After meaningful conversations, extract and store lasting insights about users to improve future interactions.

## What to Learn

| Category | Examples | Storage |
|----------|---------|---------|
| **Preferences** | Language, response length, formality, humor | `core`, per-user |
| **Expertise** | Technical skills, domain knowledge, experience level | `core`, per-user |
| **Role & context** | Job title, team, responsibilities | `core`, per-user |
| **Communication style** | Terse vs verbose, direct vs indirect, emoji usage | `core`, per-user |
| **Interests** | Topics they engage with most, recurring questions | `core`, per-user |
| **Relationships** | Who they work with, report to, collaborate with | `core`, shared |
| **Routines** | Active hours, regular tasks, recurring requests | `daily`, per-user |

## Workflow

### 1. Scan Recent Exchange

Review the conversation using `message_history(action="recent", limit=50)`.

Look for signals:

- **Explicit statements**: "I'm a backend developer", "I prefer Vietnamese"
- **Implicit patterns**: Always asks short questions (terse style), uses technical jargon (high expertise)
- **Corrections**: "Don't be so formal" (preference signal)
- **Repeated behaviors**: Always asks about the same project (interest signal)

### 2. Check Existing Profile

```
memory_search(query="user-profile [username]", limit=10)
```

Compare new observations against stored profile:
- **Confirms existing**: Skip (already known)
- **New insight**: Add to profile
- **Contradicts existing**: Update with newer information

### 3. Store Insights

One memory entry per user, updated over time:

```
memory_store(
  key="user-profile-[username]",
  content="ROLE: [role/title]\nEXPERTISE: [skills, level]\nLANGUAGE: [preferred language]\nSTYLE: [communication preferences]\nINTERESTS: [topics]\nNOTES: [other observations]",
  category="core",
  shared=false,
  relations=[
    {"source": "[username]", "relation": "works_on", "target": "[project]"},
    {"source": "[username]", "relation": "expert_in", "target": "[domain]"},
    {"source": "[username]", "relation": "belongs_to", "target": "[team/org]"}
  ]
)
```

For relationship discoveries (shared across users):
```
memory_store(
  key="graph-[person]-[person]-relation",
  content="[person A] and [person B] [relationship context]",
  category="core",
  shared=true,
  relations=[
    {"source": "[person A]", "relation": "works_with", "target": "[person B]"}
  ]
)
```

### 4. Learn Silently

Do NOT announce what was learned unless the user asks. The goal is for future responses to feel naturally more personalized.

When the user asks "what do you know about me":
```
memory_search(query="user-profile [username]", limit=5)
```

Present the stored profile in a friendly format.

## Learning Frequency

- **After every meaningful conversation** (5+ substantive exchanges)
- **Skip** trivial interactions (greetings, single-tool lookups)
- **Update, don't duplicate** -- always update `user-profile-[username]` rather than creating new entries

## Guidelines

- Only store information relevant to improving future interactions
- Never store sensitive data (passwords, financial details, private messages shared in confidence)
- Per-user data is `shared=false` by default -- only relationship data is `shared=true`
- Prefer updating existing profile entries over creating new ones
- Keep profiles concise -- under 150 words per user
- When unsure if something is worth learning, skip it. False memories are worse than missing ones.
- Observations need 2+ data points before storing (one-off mentions are noise)
