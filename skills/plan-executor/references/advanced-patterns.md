# Advanced Planning Patterns

## Conditional Plans (Fallback Chains)

When the preferred approach may fail, define fallbacks:

```
Plan A (preferred):
  - session_messages with sender_id (exact match)

If no results -> Plan B (relaxed):
  - session_messages with sender_name (partial match)

If still no results -> Plan C (broadened):
  - Increase days parameter or remove sender filter
  - Report: "No messages found with these criteria"
```

## Parallel Execution

When steps are independent, mark them for parallel execution:

```
[Step 1a] Fetch user A's messages (independent)
[Step 1b] Fetch user B's messages (independent)
[Step 2]  Compare results (depends on 1a + 1b)
```

## Cross-Source Aggregation

Combine data from multiple sessions or tools:

```
[Step 1] List all available sessions
  - session_messages(action="list")

[Step 2] For each relevant session (parallel):
  - session_messages(action="recent", session_key=..., sender_name=..., days=N)

[Step 3] Aggregate
  - Per-source breakdown + total
  - Handle sessions with zero results gracefully
```

## Time-Series Analysis

Group results by time buckets:

```
[Step 1] Fetch messages with broad time range
[Step 2] Group by hour/day from timestamps
[Step 3] Report as distribution or trend
```

## Plan Adaptation

When mid-execution discovery changes the plan:

```
Original Step 3: Fetch from session "telegram:group-abc"
Discovery: Session not found in memory
Adapted Step 3: List sessions -> find closest match -> confirm with user -> fetch
```

Always inform the user when adapting: "Session not found by that name. Found 'group-xyz' instead -- using that."

## Input Validation

Before executing, verify:
- All required parameters have values (no placeholders)
- Session/user references resolve to real entities
- Time ranges are valid and reasonable
- Tool choice is optimal for the task

If validation fails, ask for clarification rather than guessing.

## Common Pitfalls

1. **Don't assume sessions** - Always verify which group/session via memory or listing
2. **Don't assume time** - Clarify "today" vs "last 24h" vs "this calendar day"
3. **Don't assume user format** - @username vs display name vs sender_id
4. **Don't ignore pagination** - Large result sets may be truncated; note the limit
5. **Don't re-fetch** - If data from a prior step is reusable, reference it instead of calling the tool again
