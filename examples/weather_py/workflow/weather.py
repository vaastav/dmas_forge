from dmas_forge.core.agent import AgentCore
from dmas_forge.core.llm import LLMPort
from dmas_forge.core.tool import ToolPort
from dmas_forge.core.context import Context
from .disaster import DisasterAgent

class WeatherAgent(AgentCore):
    def __init__(self, llm: LLMPort, tools: ToolPort, disaster_agent: DisasterAgent, *, max_steps: int = 5) -> None:
        system_prompt = "Act as a weather analyst and prediction service. GIven a user query about weather in a given location, generate a weather report. Feel free to use the provided tools if necessary. "
        super().__init__(llm, tools, system_prompt=system_prompt, max_steps=max_steps)
        self.disaster_agent = disaster_agent

    def query(self, ctx: Context, q: str) -> str:
        res = self.run(q)
        return self.disaster_agent.query(ctx, res)