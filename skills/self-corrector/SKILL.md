---
name: self-corrector
description: Learn from user corrections and negative feedback to avoid repeating mistakes. Activate when the user corrects, rejects, or expresses dissatisfaction with a response ("no not that", "wrong", "I said X not Y", "stop doing that", "don't", "that's not what I meant", "sai roi", "khong phai", "dung lam vay"). Extract the correction pattern and store as a behavioral rule for future reference.
---

# Self Corrector

Extract correction patterns from user feedback and store as behavioral rules that persist across sessions.

## Detection Triggers

Watch for signals in user messages:

| Signal | Examples |
|--------|---------|
| Direct correction | "no, I meant...", "not X, use Y instead" |
| Rejection | "wrong", "that's not right", "sai roi" |
| Frustration | "I already told you", "again?", "why do you keep..." |
| Preference override | "don't do X", "always use Y", "stop doing Z" |
| Style correction | "too long", "be more concise", "speak Vietnamese" |

## Workflow

### 1. Extract Correction

From the user's correction message and the bot's preceding response, identify:

- **Rule**: What the bot should do differently (concise imperative)
- **Context**: When this rule applies (specific user, channel, topic, or general)
- **Bad behavior**: What the bot did wrong (for pattern matching)
- **User**: Who gave the correction

### 2. Check Existing Rules

```
memory_search(query="correction-rule [topic/behavior]", limit=5)
```

- If a similar rule exists, update it with the new instance (reinforces confidence)
- If contradicts an existing rule, flag the conflict for user clarification

### 3. Store Rule

```
memory_store(
  key="correction-rule-[behavior-slug]",
  content="RULE: [imperative statement]\nCONTEXT: [when to apply]\nBAD: [what not to do]\nFROM: [user] on [date]\nEXAMPLE: [the actual correction instance]",
  category="core",
  shared=false,
  relations=[
    {"source": "[user]", "relation": "prefers", "target": "[behavior/preference]"}
  ]
)
```

- Use `shared=false` for user-specific preferences
- Use `shared=true` for corrections about factual errors or general behavior

### 4. Acknowledge

Brief confirmation without over-apologizing:
```
"Got it, [concise restatement of the rule]. Will remember."
```

## Rule Application

Before responding, the agent should check relevant correction rules via graph context (automatic) or targeted search. Rules inform:

- Response format and length
- Language and tone
- Tool selection and parameters
- Information to include or exclude

## Key Naming

Format: `correction-rule-[behavior]`

Examples:
- `correction-rule-response-language`
- `correction-rule-message-format`
- `correction-rule-tool-preference`
- `correction-rule-alice-greeting-style`

## Guidelines

- One rule per correction -- keep atomic and searchable
- Include the concrete example that triggered the rule
- Do not store trivial one-off corrections (typo fixes, misheard words)
- When a user contradicts their own prior rule, update the old rule rather than creating a conflict
- Per-user rules take precedence over general rules
