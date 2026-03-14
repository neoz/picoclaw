# Advanced Planning Patterns

## Multi-Step Aggregation Plans

### Pattern: Count Across Multiple Groups

```
User: "tổng tin nhắn của @userX trong tất cả group"

Plan:
1. memory_search: find all sessions user participates in
2. For each session:
   - session_messages(action="recent", days=N, sender_name="userX", session_key=...)
3. Aggregate results
4. Report: per-group breakdown + total
```

### Pattern: Time-Series Analysis

```
User: "thống kê tin nhắn theo giờ trong ngày"

Plan:
1. Fetch messages (broad time range)
2. Group by hour (post-processing)
3. Generate histogram/report
```

## Conditional Plans

### With Fallbacks

```
Plan A (preferred):
  - session_messages with sender_id (exact match)
  
If no results:
  Plan B (fallback):
  - session_messages with sender_name (partial match)
  
If still no results:
  Plan C (broaden):
  - Increase days parameter
  - Or search without sender filter
```

### With Validation

```
Step 1: Validate inputs
  - Is username valid format?
  - Is date range reasonable?
  - Does session exist?

Step 2: If valid → execute
Step 3: If invalid → ask for clarification
```

## Parallel Execution Plans

When tools are independent:

```
User: "so sánh tin nhắn của A và B trong hôm nay"

Plan:
1. Fetch A's messages (independent)
2. Fetch B's messages (independent)
3. Wait for both
4. Compare and report
```

## Caching Strategy

For repeated queries:

```
Before execution:
  - Check if similar query was made recently
  - If yes AND data hasn't changed → return cached
  - If no OR data changed → execute and cache
```

## Template Responses

### Confirmation Templates

```
"Em sẽ [action] [target] trong [scope] [time]. Đúng không anh? 🙏"

Examples:
- "Em sẽ đếm tin nhắn của @ch3r0k33r0s3 trong group Reaonline hôm nay. Đúng không anh? 🙏"
- "Em sẽ tìm tin nhắn về 'xăng dầu' trong 7 ngày qua. Đúng không anh? 🙏"
```

### Result Templates

```
"Tìm thấy [count] tin nhắn của [user] trong [scope] [time]:"

Chi tiết:
| Thởi gian | Nội dung |
|-----------|----------|
| [time] | [preview] |
```

## Common Pitfalls

1. **Don't assume session** - Always confirm which group/session
2. **Don't assume time** - "hôm nay" vs "24h qua" vs "ngày hôm nay"
3. **Don't assume user format** - @username vs name vs "anh"
4. **Don't forget pagination** - Large result sets need handling

## Testing Plans

Before executing, verify:
- [ ] All parameters have values
- [ ] Session/user exists
- [ ] Time range is valid
- [ ] Tool choice is optimal