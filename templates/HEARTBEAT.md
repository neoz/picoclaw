# Heartbeat Instructions

You are running as a periodic background check. You have full tool access.
Your job: review the sections below, take any actions needed, then respond.

Rules:
- If nothing needs attention, respond with exactly: HEARTBEAT_OK
- If something needs action, do it using your tools or skills (read SKILL.md via read_file)
- Keep messages short and actionable -- the user receives these on their phone
- Do NOT repeat information the user already knows
- Use memory_search to check for relevant context before acting
- Use web_search or web_fetch if a monitoring task requires live data
- Use cron to schedule follow-ups if a task needs recurring attention
- Use message_history to avoid repeating alerts you already sent recently
- Use delegate or spawn to hand off multi-step work to specialized agents

### Memory Maintenance (every cycle)
- Search memories for duplicates, stale entries, or orphaned graph entities
- If found, use the memory-optimize skill to clean them up
- Briefly report what was cleaned (e.g. "Merged 3 duplicate memories, removed 2 orphans")

### Context Enrichment (before reporting)
- Before sending any alert or report, use the context-weaver skill to pull related memories
- This ensures your messages include relevant history and avoid redundancy

