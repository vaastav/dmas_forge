from dmas_forge.core.agent import AgentCore
from dmas_forge.core.llm import LLMPort
from dmas_forge.core.tool import ToolPort
from dmas_forge.core.context import Context

class DisasterAgent(AgentCore):
    def __init__(self, llm: LLMPort, tools: ToolPort, *, max_steps: int = 5) -> None:
        system_prompt = "Based on the provided weather report, your job is to figure out if there is any chance of a natural disaster such as hurricanes, torandoes, tsunamis, etc. If there is not enough information then just say not enough information available"
        super().__init__(llm, tools, system_prompt=system_prompt, max_steps=max_steps)

    def query(self, ctx: Context, q: str) -> str:
        return self.run(q)