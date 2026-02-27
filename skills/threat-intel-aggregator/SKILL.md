---
name: threat-intel-aggregator
description: Aggregate and summarize threat intelligence news from cybersecurity sources like The Hacker News, Mandiant, CVE databases, and infosec community. Use when user requests threat intelligence summary, security news, malware updates, CVE alerts, or IOC aggregation. Can be scheduled via cron for daily/weekly summaries.
---

# Threat Intelligence Aggregator

Aggregate threat intelligence from multiple cybersecurity sources and deliver concise, actionable summaries.

## Data Sources

Search and aggregate from these primary sources:

1. **The Hacker News** (thehackernews.com) - Latest breaches, vulnerabilities
2. **Google Mandiant / M-Trends** - Incident response insights
3. **IBM X-Force Threat Intelligence**
4. **CVE Databases** - Recent critical CVEs
5. **TweetFeed** (tweetfeed.live) - IOCs from infosec community
6. **BleepingComputer** - Malware news and analysis

## Workflow

### For On-Demand Summary

1. Use `web_search` to find latest threat intel (last 24h-7 days)
2. Query for: "malware 2025", "CVE critical", "threat intelligence", "data breach"
3. Fetch top 3-5 most relevant articles
4. Summarize key points:
   - Threat actor / malware name
   - Target industry/region
   - Attack vector / TTPs
   - IOCs (if available)
   - Mitigation recommendations

### For Scheduled/Cron Usage

1. Store aggregated data in memory with category "threat-intel"
2. Compare with previous run to identify new threats
3. Format for Telegram group (use emojis, bullet points)
4. Include timestamp and source links

## Output Format

```
🔴 THREAT INTEL SUMMARY - [Date]

🔥 HOT THREATS
1. [Threat Name] - [Severity]
   📌 Target: [Industry/Region]
   🎯 Vector: [Attack method]
   ⚡ IOCs: [Key indicators]
   🔗 Source: [URL]

2. ...

🆕 NEW CVEs
- CVE-XXXX-XXXX: [Brief desc] (CVSS: X.X)

💡 QUICK TIPS
- [Actionable recommendation]
```

## Memory Management

When running via cron:
- Store last run timestamp in memory
- Store previously reported CVEs/threats to avoid duplicates
- Use memory_search with query "threat-intel" to check history

## Example Queries

User might ask:
- "Tổng hợp threat intel hôm nay"
- "Có tin tức gì về malware không?"
- "Latest CVE alerts"
- "IOC từ X/Twitter"
- "Security news this week"
