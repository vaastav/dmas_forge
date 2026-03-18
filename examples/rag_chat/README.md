# RAG Chat Example

A single-agent chat application that showcases three different wiring-time configurations for the RAG agent, each demonstrating different capabilities of retrieval-augmented generation.

## Architecture

- **ChatAgentImpl** (`workflow/ChatAgent.go`) -- a minimal workflow that forwards messages to a `core.Agent`. Depending on the wiring, it can also index markdown files on startup and expose file-reading tools.
- **RAGAgent** -- wraps the base OpenAI agent with automatic retrieval and optional CRUD tools.
- **OpenAIKnowledgeBase** -- chunks documents, creates embeddings, and stores vectors in the configured vector store.
- **InMemoryVectorStore** -- stores embeddings in process and performs cosine-similarity search.
- **Corpus** (`workflow/data/*.md`) -- markdown files about sourdough baking used to demonstrate indexing strategies.

## Configurations

- **`automatic`**: All markdown files are indexed at startup, and relevant context is automatically retrieved and injected into the prompt before each query. The agent has no control over the knowledge base.

- **`agentic`**: The agent starts with an empty knowledge base and must explicitly call tools to read files, index documents, and search for relevant context. This demonstrates full agent autonomy over knowledge management.

### Configuration Details

- **AutoQuery**: Automatically retrieves relevant context before each query and injects it into the prompt
- **ToolExposure**: `NoTools` (agent cannot call RAG tools), `FullCRUD` (agent can search, index, and delete)

## Setup

Edit `wiring/example_model.json` with your OpenAI-compatible chat model, embeddings model, base URL, and API key:

```json
{
    "name": "gpt-5.4-nano",
    "url": "https://api.openai.com/v1",
    "key": "your-api-key-here",
    "embedding_model": "text-embedding-3-small"
}
```

## Build and Run

### Automatic

```bash
cd examples/rag_chat/wiring
go run main.go -w automatic -o build -modfile=./example_model.json
cd build/automatic
docker compose build && docker compose up -d
```

### Agentic

```bash
cd examples/rag_chat/wiring
go run main.go -w agentic -o build -modfile=./example_model.json
cd build/agentic
docker compose build && docker compose up -d
```

## Usage

### `automatic`

Pure retrieval augmentation - the agent receives relevant context automatically but cannot modify the knowledge base:

```bash
curl -s --get --data-urlencode "message=What hydration range does the guide recommend for everyday sourdough?" \
  http://localhost:12345/Chat | jq -r .Ret0

# Query outside knowledge base scope
curl -s --get --data-urlencode "message=What is the capital of France?" \
  http://localhost:12345/Chat | jq -r .Ret0
```

### `agentic`

The agent has full control - it must explicitly search, and can read, index, and delete documents:

```bash
curl -s --get --data-urlencode "message=First, list the available knowledge files. Then read the troubleshooting guide, index it, and tell me how to fix a dense loaf." \
  http://localhost:12345/Chat | jq -r .Ret0

curl -s --get --data-urlencode "message=Now search the knowledge base for dense loaf solutions." \
  http://localhost:12345/Chat | jq -r .Ret0

# Query outside knowledge base scope
curl -s --get --data-urlencode "message=What is the capital of France?" \
  http://localhost:12345/Chat | jq -r .Ret0
```

In agentic mode, the model can call `list_knowledge_files` and `read_knowledge_file`, then use `index_document`, `search_knowledge`, and `delete_document` to manage the knowledge base itself.

## Notes

- The vector store is in-process and resets when the container restarts.
- `agentic` requires the model to proactively use tools - smaller models may struggle with this autonomy.
