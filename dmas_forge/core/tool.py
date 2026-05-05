from typing import List, Dict, Any

class ToolPort:
    """
    Tool registry + executor
    """

    def schema(self) -> List[Dict[str, Any]]:
        """
        Raise OpenAI-style tool schemas
        """
        raise NotImplementedError

    def execute(self, name: str, arguments: Dict[str, Any]) -> Any:
        """
        Execute tool by name
        """
        raise NotImplementedError