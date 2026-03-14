---
name: plan-executor
description: Plan-first execution workflow for agents. Use when user requests require multi-step tool calls, complex queries, or when the agent needs to present a plan before execution. Triggers on requests involving message counting, data aggregation, multi-tool workflows, or when user explicitly asks for planning. The skill enforces analyze > plan > confirm > execute > report workflow.
---

# Plan-First Execution Skill

ALL user requests MUST go through this workflow. No exceptions.

## Core Workflow

```
ANALYZE > PLAN > CONFIRM > EXECUTE > REPORT
```

## Step 1: ANALYZE

Extract key elements from user request:

| Element | Description | Example |
|---------|-------------|---------|
| **Intent** | What user wants | "Count messages" |
| **Target** | Who/what is the subject | "ch3r0k33r0s3" |
| **Scope** | Where to search | "Reaonline group" |
| **Time** | When | "today" |
| **Action** | What tools needed | "session_messages" |

## Step 2: PLAN

Create execution plan with tool sequence:

```
Plan for "tong tin nhan cua ch3r0k33r0s3 trong ngay hom nay":

[Step 1] Resolve username > user_id
  - Check memory for user mapping
  - If not found, use sender_name filter

[Step 2] Determine session
  - Current session OR
  - Named group (lookup session_key in memory)

[Step 3] Call session_messages
  - action: "recent"
  - days: 1
  - sender_name: "ch3r0k33r0s3"
  - session_key: "telegram:-1003269096966"

[Step 4] Count and format results
```

## Step 3: CONFIRM

### Complexity Classification

| Complexity | CONFIRM Required? | Description |
|------------|-------------------|-------------|
| **Simple** | Skip | Greetings, small talk, no-tool requests |
| **Medium** | Skip | Single tool call, straightforward queries |
| **Complex** | Required | Multi-step, data aggregation, destructive actions |

### Simple (skip confirmation)

1. Greetings ("hello", "hi", "chao buoi sang")
2. Small talk ("khoe khong", "dang lam gi")
3. Identity questions ("ten gi", "la ai")
4. Help requests ("co the lam gi", "giup gi duoc")

### Medium (skip confirmation)

1. Single tool calls (weather, web search, file read)
2. Straightforward queries with clear scope
3. Non-destructive operations

### Complex (confirm first)

Present plan to user for approval:

> Em se:
> 1. Lay tin nhan tu group **Reaonline**
> 2. Loc theo user **@ch3r0k33r0s3**
> 3. Dem tong so tin nhan trong **hom nay**
>
> Anh dong y khong a?

### Confirmation Responses

- User says "ok", "dong y", "yes", "y", "di", "lam di" -> EXECUTE
- User modifies request -> Re-PLAN and CONFIRM again
- Already confirmed in same session -> May skip for related requests

## Step 4: EXECUTE

Execute tools in sequence, handle errors:

- If tool fails -> report error, suggest alternative
- If no results -> explain why, suggest adjustment
- If partial results -> note limitations

## Step 5: REPORT

Format results clearly, include context:
```
"Trong hom nay, @ch3r0k33r0s3 da gui 15 tin nhan trong group Reaonline."
```



## Progressive Disclosure

For complex multi-step plans, see [references/advanced-patterns.md](references/advanced-patterns.md)
