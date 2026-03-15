---
name: plan-executor
description: Plan-first execution workflow for complex multi-step tasks. Use when a request requires 3+ tool calls, data aggregation across sources, multi-session queries, destructive or irreversible actions, or when the user explicitly asks for a plan. Do NOT use for simple single-tool queries, greetings, or straightforward lookups.
---

# Plan-First Execution

Structured workflow for complex multi-step requests: **ANALYZE > PLAN > CONFIRM > EXECUTE > REPORT**.

## When to Apply

**Use this workflow** for:
- Requests requiring 3+ sequential tool calls
- Cross-session or cross-source data aggregation
- Destructive or irreversible operations (memory deletion, bulk updates)
- Ambiguous requests needing clarification before execution

**Skip this workflow** for:
- Single tool calls with clear parameters
- Greetings, small talk, identity questions
- Straightforward lookups (weather, web search, file read)

## Step 1: ANALYZE

Extract from the user request:

| Element | Question |
|---------|----------|
| **Intent** | What does the user want to achieve? |
| **Target** | Who/what is the subject? |
| **Scope** | Which sessions, groups, or data sources? |
| **Time** | What time range? Resolve relative dates. |
| **Tools** | Which tools are needed and in what order? |
| **Risks** | Any destructive or irreversible actions? |

If any element is ambiguous, ask for clarification before proceeding to Step 2.

## Step 2: PLAN

Build a numbered step sequence. Each step specifies: tool name, parameters, expected output, and dependency on prior steps.

```
[Step 1] Resolve inputs
  - Tool: memory_search | query: "..." | Expected: session key or user ID

[Step 2] Fetch data (depends on Step 1)
  - Tool: session_messages | action: "recent" | session_key: from Step 1

[Step 3] Process and aggregate
  - Post-process: count, group, filter, compare

[Step 4] Format and report
```

Mark independent steps that can run in parallel.

## Step 3: CONFIRM

Present the plan concisely, highlighting:
- Number of steps and tools involved
- Data sources being accessed
- Any destructive actions (deletions, overwrites)

Wait for user approval. If user modifies the request, return to Step 2.

Skip confirmation when the user already provided all parameters clearly and no destructive actions are involved.

## Step 4: EXECUTE

Run steps in order (or parallel where marked). Handle failures:

| Situation | Action |
|-----------|--------|
| Tool returns error | Report error, try fallback if available |
| No results | Broaden scope (increase `days`, relax filters), report if still empty |
| Partial results | Continue with available data, note limitations |
| Mid-plan discovery | Adapt remaining steps, inform user if plan changes significantly |

## Step 5: REPORT

Deliver results matching the user's intent:
- Lead with the answer (number, summary, comparison)
- Include relevant context (time range, source, filters applied)
- Note any limitations or caveats
- Suggest follow-up actions if applicable

## Advanced Patterns

For conditional plans, parallel execution, cross-source aggregation, and fallback strategies, see [references/advanced-patterns.md](references/advanced-patterns.md)
