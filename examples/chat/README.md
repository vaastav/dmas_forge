# Chat Agent Example

A single-agent application with persistent memory. The agent remembers information across requests using LLM-driven memory tools -- the LLM autonomously decides what to store and recall. Note that this example doesn't have sessions, so the agent doesn't see any previous messages in the conversation. It relies entirely on memory tools for remembering information across requests.

## Architecture

The application has one workflow service (`ChatAgent`) and two infrastructure components wired in at deploy time:

- **ChatAgentImpl** (`workflow/ChatAgent.go`) -- a minimal workflow that forwards messages to a `core.Agent`. It has no knowledge of memory.
- **MemoryAgent** (decorator) -- transparently wraps the base agent with four memory tools (`store_memory`, `recall_memory`, `delete_memory`, `list_memories`). The LLM sees these tools alongside its system prompt and decides when to use them.
- **InMemoryStore** -- a thread-safe in-memory key-value store that persists across requests within the same process.

The workflow code never references memory. Whether memory is enabled is purely a wiring decision in `wiring/specs/default.go`:

```go
memStore := memory_plugin.MemoryStore[*memory.InMemoryStore](spec, "chat_memory")
baseAgent := openai_plugin.OpenAILLMAgent(spec, "agent_base", model_url, model_key, model_name)
agent := memory_plugin.MemoryAgent(spec, "agent", baseAgent, memStore)
chatService := workflow.Service[wf.ChatAgent](spec, "chat_service", agent)
```

The `MemoryStore` function is generic and accepts any `core.Memory` implementation. For example, if you built a custom Redis-backed store, you would use it as:

```go
memStore := memory_plugin.MemoryStore[*redis.RedisMemory](spec, "chat_memory")
```

In this example, however, we use the built-in `memory.InMemoryStore` for simplicity. 

## Setup

Edit `wiring/example_model.json` with your API key, model name, and URL:

```json
{
    "name": "gpt-3.5-turbo",
    "url": "https://api.openai.com/v1",
    "key": "your-api-key-here"
}
```

## Build and Run

### With Memory

```bash
cd examples/chat/wiring
go run main.go -w memory -o build -modfile=./example_model.json
cd build/memory
docker compose build && docker compose up -d
```

### Without Memory

```bash
cd examples/chat/wiring
go run main.go -w no_memory -o build -modfile=./example_model.json
cd build/no_memory
docker compose build && docker compose up -d
```

## Usage

```bash
# Ask something the agent doesn't know yet
curl -s 'http://localhost:12345/Chat?message=What%20is%20my%20name?' | jq -r .Ret0
# -> "I don't have that information..."

# Tell it something and ask it to remember
curl -s 'http://localhost:12345/Chat?message=My%20name%20is%20Abdo.%20Remember%20it.' | jq -r .Ret0
# -> "Got it! I'll remember your name, Abdo."

# Ask again -- the agent recalls from its memory store
curl -s 'http://localhost:12345/Chat?message=What%20is%20my%20name?' | jq -r .Ret0
# -> "Your name is Abdo."
```

Behind the scenes, the LLM calls `store_memory` to persist facts and `list_memories`/`recall_memory` to retrieve them. The tool-call loop in the OpenAI agent handles multi-round tool use, so the LLM can list keys, then recall a specific value, all within a single request.

## Notes

- Memory is in-process and not persisted to disk. Restarting the container resets all memories.
- The quality of memory usage depends on the model. More capable models (e.g. gpt-4) are more reliable at proactively using memory tools.
- This example doesn't have sessions, so the model doesn't see any previous messages in the conversation.
