from typing import List, Optional, Dict, Any
from . import models

class LLMPort:
    """
    Framework-agnostic LLM interface
    """
    def generate(self, *, messages: List[str], tools: Optional[List[Dict[str, Any]]] = None, system_prompt: Optional[str] = None) -> models.AgentStep:
        raise NotImplementedError