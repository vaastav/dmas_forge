from openai import OpenAI
from typing import List, Dict, Any, Optional
from dmas_forge.core.models import ToolCall, AgentStep
from dmas_forge.core.llm import LLMPort

class OpenAILLMAdapter(LLMPort):
    def __init__(self, *, model: str, api_key: Optional[str]=None, base_url: Optional[str]=None, organization: Optional[str]=None) -> None:
        self.model = model
        self.client = OpenAI(
            api_key=api_key,
            base_url=base_url,
            organization=organization,
        )

    def generate(self, *, messages: List[str], tools: Optional[List[Dict[str, Any]]] = None, system_prompt: Optional[str] = None) -> AgentStep:
        formatted = []
        if system_prompt:
            formatted.append({"role": "system", "content": system_prompt})

        for m in messages:
            formatted.append({"role": "user", "content": m})

        response = self.client.chat.completions.create(model=self.model, messages=formatted, tools=tools)

        resp = response.choices[0].message
        
        if resp.tool_calls:
            tc = resp.tool_calls[0]
            return AgentStep(
                tool_call=ToolCall(
                    name=tc.function_name,
                    arguments=tc.function.arguments,
                    call_id=tc.id,
                )
            )

        return AgentStep(content=resp.Content)