package prompts

func CoordinatorPrompt() string {
	return `You are a financial analysis orchestrator.

You have access to these tools:
1. run_research_quality_controller
2. run_financial_analyst
3. run_report_writer

Rules:
- The target company and mode are provided in the user message.
- In sanity mode, skip run_financial_analyst and complete the workflow with minimal latency.
- In full mode, complete a professional stock-analysis workflow.
- Start with research_quality_controller to gather verified research.
- Use financial_analyst once the research is strong enough to support analysis, but only in full mode.
- Use report_writer to produce the final report from the best available research and analysis.
- Keep the final textual response brief; the primary deliverable is the generated report content.`
}

func CoordinatorTask(company string, sanityMode bool) string {
	if sanityMode {
		return `Create a quick sanity-check stock snapshot for ` + company + `:

1. Use 'run_research_quality_controller' to gather:
   - today's stock price, change %, and volume vs average
   - latest EPS + revenue actual vs estimate
   - 2 timely headlines with URLs
   - valuation metrics such as P/E and market cap

2. Pass the verified notes to 'run_report_writer' so it produces a concise markdown snapshot.

The goal is to produce trustworthy data with minimal latency. Skip deep dives, but include precise figures and citations when available.`
	}

	return `Create a high-quality stock analysis report for ` + company + ` by following these steps:

1. Use 'run_research_quality_controller' to gather high-quality financial data about ` + company + `, including:
   - current stock price and recent movement
   - latest quarterly earnings results and performance vs expectations
   - recent news and developments
   - key metrics and valuation context

2. Use 'run_financial_analyst' to analyze the research data and identify the key investment insights.

3. Use 'run_report_writer' to create a comprehensive, fact-based stock report.

The final report should be professional, balanced, and grounded in the verified research.`
}
