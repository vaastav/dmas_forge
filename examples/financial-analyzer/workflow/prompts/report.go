package prompts

import (
	"fmt"
	"time"
)

func ReportPrompt(company string, sanityMode bool) string {
	reportDate := time.Now().Format("January 02, 2006 at 3:04 PM MST")
	if sanityMode {
		return sanityModePrompt(company, reportDate)
	}
	return fullModePrompt(company, reportDate)
}

func sanityModePrompt(company, reportDate string) string {
	return fmt.Sprintf(`Create a concise, sanity-check markdown snapshot.

The user message provides the target company plus the verified research and optional analyst notes.

# %s Quick Financial Snapshot
**Report Date:** %s
**Mode:** Sanity Check

## Market Pulse
- Price + intraday change
- Volume vs average
- 52-week range position

## Earnings Pulse
- Latest EPS actual vs estimate
- Latest revenue actual vs estimate
- YOY growth callout

## Headlines to Watch
- The 2 most important recent items with source + impact

## Key Metrics & Takeaways
- P/E, market cap, or other notable ratios
- 2 brief bullets on bullish/concern items

Rules:
- Return markdown only.
- Keep the entire document under 400 words.
- Use only facts supported by the supplied research.
- End with a one-sentence overall assessment and confidence level.`, company, reportDate)
}

func fullModePrompt(company, reportDate string) string {
	return fmt.Sprintf(`Create a comprehensive, institutional-quality financial report.

The user message provides the target company, verified research, and analyst output.

# %s - Comprehensive Financial Analysis
**Report Date:** %s
**Analyst:** AI Financial Research Team

## Executive Summary
**Current Price:** [exact figure from research]
**Market Cap:** [exact figure from research]
**Investment Thesis:** [2-3 sentence summary]
**Recommendation:** [balanced assessment with confidence]

## Current Market Performance
### Trading Metrics
- Stock Price
- Trading Volume
- 52-Week Range
- Current Position in Range
- Market Capitalization

### Technical Analysis
[Brief market-performance interpretation grounded in the research]

## Financial Performance
### Latest Quarterly Results
- EPS actual vs estimate
- Revenue actual vs estimate
- Year-over-year growth
- Quarter

### Key Financial Metrics
- P/E ratio
- Valuation context

## Recent Developments
### Market-Moving News (Last 7 Days)
[3-5 sourced items]

### Analyst Activity
[Upgrades, downgrades, price targets, or note if unavailable]

## Investment Analysis
### Bull Case - Key Strengths
1. ...
2. ...
3. ...

### Bear Case - Key Concerns
1. ...
2. ...
3. ...

### Valuation Assessment
[Current valuation and context]

## Risk Factors
### Company-Specific Risks
- ...

### Market & Sector Risks
- ...

## Investment Conclusion
### Summary Assessment
[Balanced summary]

### Overall Recommendation
[Rationale + confidence]

### Price Target/Fair Value
[Only if supported by the supplied research]

## Data Sources & Methodology
### Sources Used
[List cited sources from the supplied research]

### Data Quality Notes
[Any limitations or assumptions]

### Report Disclaimers
*This report is for informational purposes only and should not be considered as personalized investment advice.*

Rules:
- Return markdown only.
- Use exact figures and dates from the supplied research.
- Do not invent unsupported claims or sources.
- Keep the tone professional and objective.`, company, reportDate)
}
