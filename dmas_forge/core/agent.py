from typing import List, Dict, Any
from .llm import LLMPort
from .tool import ToolPort
from .models import AgentStep

class AgentCore:
    """
    Framework-agnostic agent
    """
    def __init__(self, llm: LLMPort, tools: ToolPort, *, role: str = "generic", system_prompt: str = "", max_steps: int = 5) -> None:
        self.llm = llm
        self.tools = tools
        self.role = role
        self.system_prompt = system_prompt

    def step(self, messages: List[str]) -> AgentStep:
        return self.llm.generate(messages=messages, tools=self.tools.schema(), system_prompt=self._compose_system_prompt())

    def run(self, user_input: str) -> str:
        messages = [user_input]

        for _ in range(self.max_steps):
            step = self.step(messages)
            if step.content:
                return step.content

            if step.tool_call:
                result = self.execute_tool(step.tool_call.name, step.tool_call.arguments)
                messages.append(f"Tool result: {result}")

        raise Exception("Max steps reached")

    def execute_tool(self, name: str, args: Dict[str, Any]) -> Any:
        return self.tools.execute(name, args)

    def _compose_system_prompt(self) -> str:
        parts = []
        if self.system_prompt:
            parts.append(self.system_prompt)
        if self.role:
            parts.append(f"Role: {self.role}")
        return "\n".join(parts)