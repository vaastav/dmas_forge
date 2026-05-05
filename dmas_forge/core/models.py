from dataclasses import dataclass
from typing import Optional, Dict, Any, List

@dataclass
class ToolCall:
    name: str
    arguments: Dict[str, Any]
    call_id: Optional[str] = None

@dataclass
class AgentStep:
    content: Optional[str] = None
    tool_call: Optional[ToolCall] = None
    