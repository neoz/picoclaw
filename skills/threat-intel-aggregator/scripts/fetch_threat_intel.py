#!/usr/bin/env python3
"""
Threat Intelligence Fetcher Script
Simple script to fetch and format threat intelligence data
"""

import json
import sys
from datetime import datetime

def format_threat_summary(threats, cves, date_str=None):
    """Format threat intel into Telegram-friendly message"""
    if not date_str:
        date_str = datetime.now().strftime("%Y-%m-%d")
    
    output = f"🔴 **THREAT INTEL SUMMARY - {date_str}**\n\n"
    
    # Hot Threats Section
    output += "🔥 **HOT THREATS**\n"
    for i, threat in enumerate(threats[:5], 1):
        output += f"\n{i}. **{threat.get('name', 'Unknown')}** - {threat.get('severity', 'N/A')}\n"
        output += f"   📌 Target: {threat.get('target', 'N/A')}\n"
        output += f"   🎯 Vector: {threat.get('vector', 'N/A')}\n"
        if threat.get('iocs'):
            output += f"   ⚡ IOCs: {threat['iocs']}\n"
        output += f"   🔗 Source: {threat.get('url', 'N/A')}\n"
    
    # CVEs Section
    if cves:
        output += "\n🆕 **NEW CVEs**\n"
        for cve in cves[:5]:
            output += f"- **{cve.get('id', 'N/A')}**: {cve.get('desc', 'N/A')[:80]}... (CVSS: {cve.get('cvss', 'N/A')})\n"
    
    output += "\n💡 *Stay safe, update your systems!* 🛡️"
    return output

def main():
    """Main entry point"""
    # This is a template - actual implementation would fetch from APIs
    # For now, it just demonstrates the format
    
    sample_threats = [
        {
            "name": "Example Malware",
            "severity": "High",
            "target": "Financial Sector",
            "vector": "Phishing",
            "iocs": "example.com, 192.168.1.1",
            "url": "https://example.com/threat"
        }
    ]
    
    sample_cves = [
        {
            "id": "CVE-2025-1234",
            "desc": "Remote code execution vulnerability in example software",
            "cvss": "9.8"
        }
    ]
    
    if len(sys.argv) > 1 and sys.argv[1] == "--sample":
        print(format_threat_summary(sample_threats, sample_cves))
    else:
        print("Usage: python fetch_threat_intel.py --sample")
        print("This is a template script. The agent uses web_search/web_fetch directly.")

if __name__ == "__main__":
    main()
