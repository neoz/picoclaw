---
name: threat-intel-aggregator
description: Aggregate and summarize cybersecurity threat intelligence. Triggers on "threat intel", "security news", "malware update", "CVE alert", "IOC", "vulnerability summary", "threat report", "cyber news", "tong hop threat intel", "tin bao mat". Searches multiple sources, deduplicates against memory, and delivers concise actionable summaries.
---

# Threat Intelligence Aggregator

Aggregate threat intelligence from multiple cybersecurity sources and deliver concise, actionable summaries.

## Sources

| Source | Focus | Search Query Pattern |
|--------|-------|---------------------|
| The Hacker News | Breaches, vulnerabilities | `site:thehackernews.com [topic] [year]` |
| BleepingComputer | Malware, ransomware | `site:bleepingcomputer.com [topic] [year]` |
| Mandiant / Google TAG | APT, incident response | `site:cloud.google.com/blog/topics/threat-intelligence [topic]` |
| NIST NVD | CVE database | `site:nvd.nist.gov CVE critical [year]` |
| CISA Alerts | US-CERT advisories | `site:cisa.gov alert [topic]` |
| TweetFeed | IOCs from infosec community | `site:tweetfeed.live [ioc-type]` |

## Workflow

### Step 1: Determine Scope

Parse user request to extract:

| Element | Default | Examples |
|---------|---------|---------|
| Time range | Last 24h | "hom nay", "tuan nay", "this week" |
| Topic filter | All | "ransomware", "CVE", "APT", "zero-day" |
| Region filter | Global | "Vietnam", "APAC", "US" |
| Severity filter | Critical+High | "critical only", "all severities" |

### Step 2: Search (parallel queries)

Run 3-4 `web_search` calls with targeted queries:

```
Query 1: "critical vulnerability CVE 2026" (count=5)
Query 2: "malware ransomware threat 2026 march" (count=5)
Query 3: "data breach cybersecurity news today" (count=5)
Query 4 (optional, topic-specific): "[user topic] cybersecurity 2026" (count=5)
```

### Step 3: Fetch Details

For top 3-5 most relevant results, use `web_fetch` to get article content:

```
web_fetch(url="[article_url]", maxChars=3000)
```

Extract from each article:
- Threat actor / malware name
- Target industry / region
- Attack vector / TTPs (MITRE ATT&CK if available)
- IOCs (IPs, domains, hashes)
- CVE IDs and CVSS scores
- Mitigation / patch info

### Step 4: Deduplicate

Check memory for previously reported items:

```
memory_search(query="threat-intel [threat_name_or_cve_id]")
```

Skip items already reported in recent runs. Only include new or updated threats.

### Step 5: Store and Report

Store new findings in memory for future deduplication:

```
memory_store(key="threat-intel-[date]", content="[summary of reported items]", category="threat-intel")
```

## Output Format

```
[!] THREAT INTEL SUMMARY - [YYYY-MM-DD]

--- HOT THREATS ---

1. [Threat Name] - [Critical/High/Medium]
   Target: [Industry/Region]
   Vector: [Attack method / TTP]
   IOCs: [Key indicators if available]
   Mitigation: [Patch/action]
   Source: [URL]

2. ...

--- NEW CVEs ---

- CVE-XXXX-XXXX: [Brief description] (CVSS: X.X) - [Patch status]
- ...

--- RECOMMENDATIONS ---

- [Actionable step 1]
- [Actionable step 2]
```

## Cron Usage

When running as a scheduled job:

1. Check `memory_search(query="threat-intel-last-run")` for last execution timestamp
2. Only search for content newer than last run
3. Store results: `memory_store(key="threat-intel-[date]", content="...", category="threat-intel")`
4. Update timestamp: `memory_store(key="threat-intel-last-run", content="[ISO timestamp]", category="threat-intel")`
5. Send summary to configured channel

## Query Patterns by Request Type

| User Request | Search Strategy |
|-------------|----------------|
| General daily summary | Broad search across all sources |
| Specific malware/APT | Targeted `"[name] malware analysis"` + `"[name] IOC"` |
| CVE lookup | `"CVE-XXXX-XXXX"` + NVD fetch |
| Industry-specific | `"[industry] cybersecurity threat [year]"` |
| IOC enrichment | `"[IP/domain/hash] threat intelligence"` |
