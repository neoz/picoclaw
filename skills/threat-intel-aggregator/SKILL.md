---
name: threat-intel-aggregator
description: Aggregate and summarize cybersecurity threat intelligence from multiple sources. Use when asked for "threat intel", "security news", "CVE alert", "vulnerability summary", "threat report", "cyber news", "IOC lookup", "malware update",  or any request about recent cybersecurity threats, breaches, or vulnerabilities. Also triggers on specific CVE lookups, IOC enrichment, or APT tracking requests.
---

# Threat Intelligence Aggregator

Aggregate threat intelligence from multiple sources into concise, actionable summaries.

## Workflow

### 1. Determine Scope

| Element | Default | Clarify if ambiguous |
|---------|---------|----------------------|
| Time range | Last 24h | "today" vs "this week" vs "this month" |
| Topic | All threats | ransomware, CVE, APT, zero-day, breach |
| Region | Global | specific country or region focus |
| Severity | Critical + High | include medium/low? |

### 2. Search (parallel)

Run 3-4 parallel `web_search` calls. Adapt queries to scope:

**General daily summary:**
- `"critical vulnerability CVE [current-year]"` (count=5)
- `"ransomware malware attack [current-year] [current-month]"` (count=5)
- `"data breach cybersecurity news [today's date or this week]"` (count=5)

**Topic-specific** (add as 4th query):
- APT: `"[APT name] threat analysis [year]"`
- CVE: `"CVE-XXXX-XXXX"` (exact ID)
- IOC: `"[IP/domain/hash] threat intelligence"`
- Industry: `"[industry] cybersecurity threat [year]"`
- Malware: `"[name] malware analysis IOC"`

**Source-targeted** (use `site:` when deeper coverage needed):

| Source | Focus | Pattern |
|--------|-------|---------|
| thehackernews.com | Breaches, vulns | `site:thehackernews.com [topic]` |
| bleepingcomputer.com | Malware, ransomware | `site:bleepingcomputer.com [topic]` |
| cloud.google.com/blog/topics/threat-intelligence | APT, IR | `site:cloud.google.com/blog/topics/threat-intelligence [topic]` |
| nvd.nist.gov | CVE database | `site:nvd.nist.gov [CVE-ID]` |
| cisa.gov | US-CERT advisories | `site:cisa.gov alert [topic]` |

### 3. Fetch and Extract

For top 3-5 results, `web_fetch(url=..., maxChars=3000)` and extract:

- **Identity**: Threat actor / malware name / CVE ID
- **Impact**: Target industry, region, scale
- **Technical**: Attack vector, TTPs (map to MITRE ATT&CK where possible)
- **Indicators**: IOCs (IPs, domains, hashes, file paths)
- **Severity**: CVSS score, exploitation status (in-the-wild, PoC, theoretical)
- **Response**: Patch availability, mitigations, workarounds

If an article lacks key fields, note as "unknown" rather than guessing.

### 4. Deduplicate

Check memory for previously reported items:

```
memory_search(query="threat-intel [threat_name or CVE-ID]", category="daily")
```

- Skip items already reported in recent runs
- Include items with significant updates (new patches, escalated severity, new IOCs)
- Flag updated items as "[UPDATED]" in the report

### 5. Store

Store new findings for future deduplication:

```
memory_store(
  key="threat-intel-[YYYY-MM-DD]",
  content="[list of threat names, CVE IDs, and one-line summaries]",
  category="daily",
  shared=true,
  relations=[
    {"source": "[threat_actor]", "relation": "exploits", "target": "[CVE-ID]"},
    {"source": "[threat_actor]", "relation": "targets", "target": "[industry/region]"},
    {"source": "[malware]", "relation": "used_by", "target": "[threat_actor]"}
  ]
)
```

Build knowledge graph relations to connect threat actors, malware families, CVEs, and targets across reports.

### 6. Report

```
[!] THREAT INTEL - [YYYY-MM-DD]

--- CRITICAL THREATS ---

1. [Threat Name] - [Critical/High]
   Actor: [Threat actor or "Unknown"]
   Target: [Industry / Region]
   Vector: [Attack method]
   CVEs: [IDs if applicable]
   IOCs: [Key indicators]
   Status: [Actively exploited / PoC / Patched]
   Action: [Patch / mitigate / monitor]
   Source: [URL]

--- CVE HIGHLIGHTS ---

| CVE | CVSS | Product | Exploited? | Patch |
|-----|------|---------|------------|-------|
| CVE-XXXX-XXXX | X.X | [product] | Yes/No | [link or status] |

--- RECOMMENDATIONS ---

Priority actions based on findings:
1. [Actionable step with specific target]
2. [Actionable step with specific target]
```

Adapt language to match user preference. Omit empty sections.

## Cron Usage

When running as a scheduled job:

1. Check last run: `memory_search(query="threat-intel-last-run")`
2. Search only for content newer than last run
3. Store results with `category: "daily"` (30-day retention)
4. Update timestamp: `memory_store(key="threat-intel-last-run", content="[ISO timestamp]", category="core")`
5. Send summary to configured channel
6. On failure (no search results, fetch errors), store partial results and note gaps
