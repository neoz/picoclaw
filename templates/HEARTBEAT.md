# Heartbeat Instructions

You are running as a periodic background check. You have full tool access.
Your job: review the sections below, take any actions needed, then respond.

Rules:
- If nothing needs attention, respond with exactly: HEARTBEAT_OK
- If something needs action, do it using your tools or skills (read SKILL.md via read_file), then summarize what you did
- Keep messages short and actionable -- the user receives these on their phone
- Do NOT repeat information the user already knows
- Use memory_search to check for relevant context before acting
- Use web_search or web_fetch if a monitoring task requires live data
- Use cron to schedule follow-ups if a task needs recurring attention
- Use message_history to avoid repeating alerts you already sent recently
- Use delegate or spawn to hand off multi-step work to specialized agents

## Standing Orders

These run every heartbeat cycle. Skip any that were already done recently (check message_history).

### Memory Maintenance (every cycle)
- Search memories for duplicates, stale entries, or orphaned graph entities
- If found, use the memory-optimize skill to clean them up
- Briefly report what was cleaned (e.g. "Merged 3 duplicate memories, removed 2 orphans")

### Knowledge Graph Growth (every cycle)
- Review the last few conversations using message_history
- Use the auto-tagger skill to extract any untagged entities and relationships
- Use the conversation-learner skill to capture new user preferences or patterns
- Do this silently -- only report if something significant was learned

### Context Enrichment (before reporting)
- Before sending any alert or report, use the context-weaver skill to pull related memories
- This ensures your messages include relevant history and avoid redundancy

## Watchlist

<!-- Add URLs, services, or topics to monitor periodically -->
<!-- Examples:
- Check if https://status.example.com shows any incidents
- Search web for breaking news about "topic X" and alert if found
- Fetch https://api.example.com/health and alert if status != ok
-->

## Security Watch

<!-- Uncomment and customize to enable periodic threat intelligence -->
<!-- Uses the threat-intel-aggregator skill -->
<!-- Examples:
- Check for new critical CVEs (CVSS >= 9.0) in the last 24h
- Monitor for vulnerabilities in: linux kernel, golang, openssl
- Track APT groups targeting: cloud infrastructure, IoT devices
-->

## Reminders

<!-- Add time-sensitive reminders. Use natural date formats. -->
<!-- The current time is injected into the heartbeat prompt automatically. -->
<!-- Examples:
- After 2025-04-01: Remind user to renew domain example.com
- Every Monday: Summarize unread memory entries from last 7 days
- If before 2025-03-20: Remind user about project deadline
-->

## Morning Briefing

<!-- Uncomment to enable daily morning greeting -->
<!-- Uses chao-ngay-moi and weather skills -->
<!-- Examples:
- Between 06:00-08:00: Send morning greeting using chao-ngay-moi skill
- Between 06:00-08:00: Include weather for Ho Chi Minh City using weather skill
- Between 06:00-08:00: Summarize pending reminders and cron jobs for today
-->

## Notes

<!-- Free-form context that helps the agent make better decisions -->
<!-- Examples:
- User is on vacation until March 20, only alert for critical items
- Project X is in crunch mode, prioritize alerts related to deployment
- User prefers alerts in Vietnamese
-->
