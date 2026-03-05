# DMAS-Forge: A Framework for Transparent Deployment of AI Applications as Distributed Systems

Current Status and Features:

Agentic AI plugins:
+ openAI Agents: [ai_plugins/openai_plugin](ai_plugins/openai_plugin)
+ A2A support: [ai_plugins/a2a](ai_plugins/a2a); Currently we are building on [trpc-a2a-go](https://github.com/trpc-group/trpc-a2a-go) but we will most likely soon shift to [a2a-go](https://github.com/a2aproject/a2a-go) SDK by the official A2A project.
+ MCP support: [ai_plugins/mcp](ai_plugins/mcp); We build on the official [go mcp sdk](https://github.com/modelcontextprotocol/go-sdk/tree/main).
+ vLLM support: Coming Soon
+ kagent support: Coming Soon

Blueprint plugins:
+ Blueprint: [Plugins](https://github.com/Blueprint-uServices/blueprint/tree/main/plugins)

DMAS-Forge is still under construction and being built. APIs may change as the project evolves.

## Examples

Two example applications are provided in the [`examples/`](examples/) directory:

| Example | Description | Agents | Key features |
|---|---|---|---|
| [weather](examples/weather/) | Multi-agent weather report with disaster risk assessment | 2 (WeatherAgent + DisasterAgent) | Tool use, inter-agent communication, multiple wiring specs (HTTP, A2A, MCP) |
| [chat](examples/chat/) | Conversational agent with persistent memory | 1 (ChatAgent) | LLM-driven memory tools, decorator pattern, multi-round tool calls |

Each example has its own README with setup and usage instructions.

## How to Cite?

```
@inproceedings{cornacchia2025dmasforge,
  title        = {DMAS-Forge: A Framework for Transparent Deployment of AI Applications as Distributed Systems},
  author       = {Cornacchia, Alessandro and Anand, Vaastav and Bilal, Muhammad and Qazi, Zafar and Canini, Marco},
  booktitle    = {1st Workshop on Systems for Agentic AI (SAA '25)},
  year         = {2025},
  keywords     = {distributed systems, AI applications, framework, deployment},
}
```
